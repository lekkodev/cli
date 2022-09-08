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
	"log"
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
func (cr *ConfigRepo) Login(ctx context.Context) error {
	defer cr.Status(ctx)
	if cr.Secrets.HasGithubToken() {
		if err := cr.CheckGithubAuth(ctx); err == nil {
			return nil
		} else {
			log.Printf("Existing gh token expired: %v\n", err)
		}
	}
	flow := &ghauth.Flow{
		Host:     ghauth.GitHubHost("https://github.com"),
		ClientID: lekkoGHAppClientID,
	}
	token, err := flow.DetectFlow()
	if err != nil {
		return errors.Wrap(err, "gh oauth flow")
	}
	cr.Secrets.SetGithubToken(token.Token)
	cr.mkGhCli(ctx)
	if err := cr.CheckGithubAuth(ctx); err != nil {
		return err
	}
	user, resp, err := cr.ghCli.Users.Get(ctx, "")
	if err != nil {
		return fmt.Errorf("ghCli user %v: %v", resp.Status, err)
	}
	cr.Secrets.SetGithubUser(user.GetLogin())
	return nil
}

func maskToken(token string) string {
	var ret []string
	for range token {
		ret = append(ret, "*")
	}
	return strings.Join(ret, "")
}

func (cr *ConfigRepo) Logout(ctx context.Context) error {
	cr.Secrets.SetGithubToken("")
	cr.Secrets.SetGithubUser("")
	cr.Status(ctx)
	return nil
}

func (cr *ConfigRepo) Status(ctx context.Context) {
	status := "Logged In"
	if !cr.Secrets.HasGithubToken() {
		status = "Logged out"
	}
	if err := cr.CheckGithubAuth(ctx); err != nil {
		status = fmt.Sprintf("Auth Failed: %v", err)
	}
	fmt.Printf(
		"Github Authentication Status: %s\n\tToken: %s\n\tUser: %s\n",
		status,
		maskToken(cr.Secrets.GetGithubToken()),
		cr.Secrets.GetGithubUser(),
	)
}
