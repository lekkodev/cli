// Copyright 2022 Lekko Technologies, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package gh

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/go-git/go-git/v5"
	"github.com/google/go-github/v52/github"
	"github.com/pkg/errors"
	"github.com/shurcooL/githubv4"
	"golang.org/x/oauth2"
)

const (
	templateOwner      = "lekkodev"
	templateRepo       = "template"
	defaultDescription = "A home for dynamic configuration"
)

// A simple wrapper around the github client, exposing additional functionality
// relevant to lekko.
type GithubClient struct {
	*github.Client
	Graphql *githubv4.Client
}

func NewGithubClient(h *http.Client) *GithubClient {
	return &GithubClient{
		Client:  github.NewClient(h),
		Graphql: githubv4.NewClient(h),
	}
}

func NewGithubClientFromToken(ctx context.Context, token string) *GithubClient {
	ts := oauth2.StaticTokenSource(&oauth2.Token{AccessToken: token})
	tc := oauth2.NewClient(ctx, ts)
	return NewGithubClient(tc)
}

func (gc *GithubClient) GetUser(ctx context.Context) (*github.User, error) {
	user, _, err := gc.Users.Get(ctx, "")
	if err != nil {
		return nil, errors.New(SanitizedErrorMessage(err))
	}
	return user, nil
}

func (gc *GithubClient) GetUserEmails(ctx context.Context) ([]*github.UserEmail, error) {
	emails, _, err := gc.Users.ListEmails(ctx, nil)
	if err != nil {
		return nil, errors.New(SanitizedErrorMessage(err))
	}
	return emails, nil
}

// Init will create a new github repo based off of lekko's config repo template.
func (gc *GithubClient) Init(ctx context.Context, owner, repoName string, private bool) (string, error) {
	repo, resp, err := gc.Repositories.Get(ctx, owner, repoName)
	if err == nil {
		return repo.GetCloneURL(), errors.Wrapf(git.ErrRepositoryAlreadyExists, "repo: %s", repo.GetCloneURL())
	}
	if resp != nil && resp.StatusCode != http.StatusNotFound { // some other error occurred
		return "", errors.New(SanitizedErrorMessage(err))
	}
	description := defaultDescription
	repo, _, err = gc.Repositories.CreateFromTemplate(ctx, templateOwner, templateRepo, &github.TemplateRepoRequest{
		Name:        &repoName,
		Owner:       &owner,
		Description: &description,
		Private:     &private,
	})
	if err != nil {
		return "", errors.New(SanitizedErrorMessage(err))
	}
	return repo.GetCloneURL(), nil
}

func (gc *GithubClient) CreateRepo(ctx context.Context, owner, repoName string, description string, private bool) (*github.Repository, error) {
	repo, resp, err := gc.Repositories.Get(ctx, owner, repoName)
	if err == nil {
		return repo, errors.Wrapf(git.ErrRepositoryAlreadyExists, "repo: %s", repo.GetCloneURL())
	}
	if resp != nil && resp.StatusCode != http.StatusNotFound { // some other error occurred
		return nil, errors.New(SanitizedErrorMessage(err))
	}
	if len(description) == 0 {
		description = defaultDescription
	}
	repo, _, err = gc.Repositories.Create(ctx, owner, &github.Repository{
		Name:        &repoName,
		Private:     &private,
		Description: &description,
	})
	if err != nil {
		return nil, errors.New(SanitizedErrorMessage(err))
	}
	return repo, nil
}

// GetUserOrganizations uses an authenticated call to get the users private and public organizations
func (gc *GithubClient) GetUserOrganizations(ctx context.Context) ([]*github.Organization, error) {
	// TODO: may need pagination if user is in more than 100 orgs
	orgs, _, err := gc.Organizations.List(ctx, "", &github.ListOptions{PerPage: 100})
	if err != nil {
		return nil, errors.New(SanitizedErrorMessage(err))
	}
	return orgs, nil
}

// GetAuthenticatedRepos gets all repos for a user. Passing the empty string will list
// repositories for the authenticated user.
func (gc *GithubClient) GetAllUserRepositories(ctx context.Context, username string) ([]*github.Repository, error) {
	var allRepos []*github.Repository
	opts := &github.RepositoryListOptions{
		ListOptions: github.ListOptions{PerPage: 100},
	}
	for {
		repos, resp, err := gc.Repositories.List(ctx, username, opts)
		if err != nil {
			return nil, errors.New(SanitizedErrorMessage(err))
		}
		allRepos = append(allRepos, repos...)
		if resp.NextPage == 0 {
			break
		}
		opts.Page = resp.NextPage
	}
	return allRepos, nil
}

// GetAllUserInstallations gets all installations for a user
func (gc *GithubClient) GetAllUserInstallations(ctx context.Context, filterSuspended bool) ([]*github.Installation, error) {
	var allInstalls []*github.Installation
	opts := &github.ListOptions{PerPage: 100}
	for {
		installs, resp, err := gc.Apps.ListUserInstallations(ctx, opts)
		if err != nil {
			return nil, errors.New(SanitizedErrorMessage(err))
		}
		for _, install := range installs {
			if filterSuspended && install.SuspendedAt != nil {
				continue
			}
			allInstalls = append(allInstalls, install)
		}
		if resp.NextPage == 0 {
			break
		}
		opts.Page = resp.NextPage
	}
	return allInstalls, nil
}

func ParseOwnerRepo(githubURL string) (string, string, error) {
	// TODO: make this exhaustive and check the git standard for remotes, or do it the URL way.
	// ssh url
	if strings.Contains(githubURL, "@") {
		parts := strings.Split(strings.Split(githubURL, ":")[1], "/")
		if len(parts) != 2 {
			return "", "", fmt.Errorf("unknown ssh github repo format: %s", githubURL)
		}
		owner, repo := parts[0], parts[1]
		if idx := strings.Index(repo, ".git"); idx >= 0 {
			repo = repo[0:idx]
		}
		return owner, repo, nil
	}
	// https url
	parts := strings.Split(githubURL, "/")
	n := len(parts)
	if n < 3 {
		return "", "", fmt.Errorf("invalid github repo url %s", githubURL)
	}
	owner, repo := parts[n-2], parts[n-1]
	if idx := strings.Index(repo, ".git"); idx >= 0 {
		repo = repo[0:idx]
	}
	return owner, repo, nil
}

// Parses GitHub's errors for cleaner error messages
func SanitizedErrorMessage(err error) string {
	var gErrResp *github.ErrorResponse
	var gerr *github.Error
	var uerr *url.Error
	if errors.As(err, &gErrResp) {
		var errMsgs []string
		for _, gerr := range gErrResp.Errors {
			errMsgs = append(errMsgs, gerr.Message)
		}
		errReasons := strings.Join(errMsgs, ",")
		if len(errReasons) > 0 {
			return errReasons
		} else {
			return fmt.Sprintf("%d %s", gErrResp.Response.StatusCode, gErrResp.Message)
		}
	} else if errors.As(err, &gerr) {
		return gerr.Message
	} else if errors.As(err, &uerr) {
		return uerr.Unwrap().Error()
	} else {
		return err.Error()
	}
}
