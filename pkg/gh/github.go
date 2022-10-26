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
	"io/ioutil"
	"net/http"

	"github.com/go-git/go-git/v5"
	"github.com/google/go-github/v47/github"
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
	if err != nil && resp.StatusCode != http.StatusNotFound { // some other error occurred
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
		body, _ := ioutil.ReadAll(resp.Body)
		return "", errors.Wrapf(err, "create from template [%s], %s", resp.Status, string(body))
	}
	return repo.GetCloneURL(), nil
}
