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

	"github.com/google/go-github/v47/github"
	"golang.org/x/oauth2"
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

func (gc *GithubClient) GetUserLogin(ctx context.Context) (string, error) {
	user, resp, err := gc.Users.Get(ctx, "")
	if err != nil {
		return "", fmt.Errorf("get user failed [%v]: %w", resp, err)
	}
	return user.GetLogin(), nil
}
