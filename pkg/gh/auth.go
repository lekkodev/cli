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
	"strings"

	ghauth "github.com/cli/oauth"
	"github.com/pkg/errors"
)

const (
	// The client ID is public knowledge, so this is safe to commit in version control.
	lekkoGHAppClientID string = "Iv1.031cf53c3284be35"
)

// Login will attempt to read any existing github credentials from disk. If unavailable,
// it will initiate oauth with github.
func (cr *ConfigRepo) Login() error {
	defer cr.Status()
	ghToken := cr.secrets.GetGithubToken()
	if ghToken != "" {
		return nil
	}
	flow := &ghauth.Flow{
		Host:     ghauth.GitHubHost("https://github.com"),
		ClientID: lekkoGHAppClientID,
	}
	token, err := flow.DetectFlow()
	if err != nil {
		return errors.Wrap(err, "gh oauth flow")
	}
	cr.secrets.SetGithubToken(token.Token)
	ctx := context.Background()
	cr.authenticateGithub(ctx)
	user, resp, err := cr.ghCli.Users.Get(ctx, "")
	if err != nil {
		return fmt.Errorf("ghCli user %v: %v", resp.Status, err)
	}
	cr.secrets.SetGithubUser(user.GetLogin())
	return nil
}

func maskToken(token string) string {
	var ret []string
	for range token {
		ret = append(ret, "*")
	}
	return strings.Join(ret, "")
}

func (cr *ConfigRepo) Logout() error {
	cr.secrets.SetGithubToken("")
	cr.secrets.SetGithubUser("")
	cr.Status()
	return nil
}

func (cr *ConfigRepo) Status() {
	status := "Logged In"
	if cr.secrets.GetGithubToken() == "" {
		status = "Logged out"
	}
	fmt.Printf(
		"Github Authentication Status: %s\n\tToken: %s\n\tUser: %s\n",
		status,
		maskToken(cr.secrets.GetGithubToken()),
		cr.secrets.GetGithubUser(),
	)
}
