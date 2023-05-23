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
	"io"
	"net/http"
	"strings"

	"github.com/go-git/go-git/v5"
	"github.com/google/go-github/v52/github"
	"github.com/pkg/errors"
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
}

func NewGithubClient(h *http.Client) *GithubClient {
	return &GithubClient{
		Client: github.NewClient(h),
	}
}

func NewGithubClientFromToken(ctx context.Context, token string) *GithubClient {
	ts := oauth2.StaticTokenSource(&oauth2.Token{AccessToken: token})
	tc := oauth2.NewClient(ctx, ts)
	return &GithubClient{
		Client: github.NewClient(tc),
	}
}

func (gc *GithubClient) GetUser(ctx context.Context) (*github.User, error) {
	user, resp, err := gc.Users.Get(ctx, "")
	if err != nil {
		return nil, fmt.Errorf("get user failed [%v]: %w", resp, err)
	}
	return user, nil
}

// Init will create a new github repo based off of lekko's config repo template.
func (gc *GithubClient) Init(ctx context.Context, owner, repoName string, private bool) (string, error) {
	repo, resp, err := gc.Repositories.Get(ctx, owner, repoName)
	if err == nil {
		return repo.GetCloneURL(), errors.Wrapf(git.ErrRepositoryAlreadyExists, "repo: %s", repo.GetCloneURL())
	}
	if err != nil && resp != nil && resp.StatusCode != http.StatusNotFound { // some other error occurred
		return "", err
	}
	description := defaultDescription
	repo, resp, err = gc.Repositories.CreateFromTemplate(ctx, templateOwner, templateRepo, &github.TemplateRepoRequest{
		Name:        &repoName,
		Owner:       &owner,
		Description: &description,
		Private:     &private,
	})
	if err != nil {
		body, _ := io.ReadAll(resp.Body)
		return "", errors.Wrapf(err, "create from template [%s], %s", resp.Status, string(body))
	}
	return repo.GetCloneURL(), nil
}

func (gc *GithubClient) CreateRepo(ctx context.Context, owner, repoName string, private bool) (*github.Repository, error) {
	repo, resp, err := gc.Repositories.Get(ctx, owner, repoName)
	if err == nil {
		return repo, errors.Wrapf(git.ErrRepositoryAlreadyExists, "repo: %s", repo.GetCloneURL())
	}
	if err != nil && resp != nil && resp.StatusCode != http.StatusNotFound { // some other error occurred
		return nil, err
	}
	description := defaultDescription
	repo, resp, err = gc.Repositories.Create(ctx, owner, &github.Repository{
		Name:        &repoName,
		Private:     &private,
		Description: &description,
	})
	if err != nil {
		var body []byte
		var status string
		if resp != nil {
			body, _ = io.ReadAll(resp.Body)
			status = resp.Status
		}
		return nil, errors.Wrapf(err, "create repo [%s]: %s", status, string(body))
	}
	return repo, nil
}

// GetUserOrganizations uses an authenticated call to get the users private and public organizations
func (gc *GithubClient) GetUserOrganizations(ctx context.Context) ([]string, error) {
	// TODO: may need pagination if user is in more than 100 orgs
	orgs, _, err := gc.Organizations.List(ctx, "", nil)
	if err != nil {
		return nil, err
	}
	var result = make([]string, 0, len(orgs))
	for _, org := range orgs {
		if org.Login != nil {
			result = append(result, *org.Login)
		}
	}
	return result, nil
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
			return nil, err
		}
		allRepos = append(allRepos, repos...)
		if resp.NextPage == 0 {
			break
		}
		opts.Page = resp.NextPage
	}
	return allRepos, nil
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
