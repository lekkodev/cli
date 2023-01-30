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
	"github.com/lekkodev/cli/pkg/metadata"
	"github.com/pkg/errors"
)

const (
	// The client ID is public knowledge, so this is safe to commit in version control.
	lekkoGHAppClientID string = "Iv1.031cf53c3284be35"
	// Lekko CLI client ID
	LekkoClientID string = "test"
)

type AuthFS struct {
	Secrets metadata.Secrets
}

// Returns an AuthFS object, responsible for managing authentication on the local FS.
// This is meant to be used by the cli on the user's local filesystem.
func NewAuthFS() *AuthFS {
	return &AuthFS{
		Secrets: metadata.NewSecretsOrFail(),
	}
}

func (a *AuthFS) Close() {
	if err := a.Secrets.Close(); err != nil {
		log.Printf("error closing secrets: %v\n", err)
	}
}

// Login will attempt to read any existing github credentials from disk. If unavailable,
// it will initiate oauth with github.
func (a *AuthFS) Login(ctx context.Context) error {
	defer a.Status(ctx)
	if a.Secrets.HasGithubToken() {
		if err := a.CheckGithubAuth(ctx); err == nil {
			return nil
		} else {
			log.Printf("Existing gh token expired: %v\n", err)
		}
	}
	flow := &ghauth.Flow{
		Host:     ghauth.GitHubHost("https://github.com"),
		ClientID: lekkoGHAppClientID,
		Scopes:   []string{"user", "repo"},
	}
	token, err := flow.DetectFlow()
	if err != nil {
		return errors.Wrap(err, "gh oauth flow")
	}
	a.Secrets.SetGithubToken(token.Token)
	login, email, err := a.GetGithubUserLogin(ctx)
	if err != nil {
		return err
	}
	a.Secrets.SetGithubUser(login)
	a.Secrets.SetGithubEmail(email)
	return nil
}

func maskToken(token string) string {
	var ret []string
	for range token {
		ret = append(ret, "*")
	}
	return strings.Join(ret, "")
}

func (a *AuthFS) Logout(ctx context.Context) error {
	a.Secrets.SetGithubToken("")
	a.Secrets.SetGithubUser("")
	a.Secrets.SetGithubEmail("")
	a.Status(ctx)
	return nil
}

func (a *AuthFS) Status(ctx context.Context) {
	status := "Logged In"
	if !a.Secrets.HasGithubToken() {
		status = "Logged out"
	}
	if err := a.CheckGithubAuth(ctx); err != nil {
		status = fmt.Sprintf("Auth Failed: %v", err)
	}
	fmt.Printf(
		"Github Authentication Status: %s\n\tUser: %s\n\tEmail: %s\n\tToken: %s\n",
		status,
		a.Secrets.GetGithubUser(),
		a.Secrets.GetGithubEmail(),
		maskToken(a.Secrets.GetGithubToken()),
	)
}

func (a *AuthFS) GetGithubUserLogin(ctx context.Context) (string, string, error) {
	ghCli := NewGithubClientFromToken(ctx, a.Secrets.GetGithubToken())
	user, err := ghCli.GetUser(ctx)
	if err != nil {
		return "", "", errors.Wrap(err, "check auth")
	}
	// Note: user.Email is only set if the user has allowed their email
	// address to be publically available on github. So it may not be populated.
	// If it is there, we will use it as part of the commit message.
	return user.GetLogin(), user.GetEmail(), nil
}

func (a *AuthFS) CheckGithubAuth(ctx context.Context) error {
	if _, _, err := a.GetGithubUserLogin(ctx); err != nil {
		return err
	}
	return nil
}
