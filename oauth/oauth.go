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

package oauth

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"strings"

	"github.com/bufbuild/connect-go"
	ghauth "github.com/cli/oauth"
	"github.com/lekkodev/cli/pkg/gen/proto/go-connect/lekko/bff/v1beta1/bffv1beta1connect"
	bffv1beta1 "github.com/lekkodev/cli/pkg/gen/proto/go/lekko/bff/v1beta1"
	"github.com/lekkodev/cli/pkg/gh"
	"github.com/lekkodev/cli/pkg/metadata"
	"github.com/pkg/errors"
)

const (
	// The client ID is public knowledge, so this is safe to commit in version control.
	lekkoGHAppClientID string = "Iv1.031cf53c3284be35"
	// lekkoURL string = "https://prod.api.lekko.dev"
	lekkoURL string = "http://localhost:8080"
)

type OAuth struct {
	Secrets         metadata.Secrets
	lekkoBFFClient  bffv1beta1connect.BFFServiceClient
	lekkoAuthClient bffv1beta1connect.AuthServiceClient
}

// Returns an OAuth object, responsible for managing oauth on the local FS.
// This is meant to be used by the cli on the user's local filesystem.
func NewOAuth() *OAuth {
	return &OAuth{
		Secrets:         metadata.NewSecretsOrFail(),
		lekkoBFFClient:  bffv1beta1connect.NewBFFServiceClient(http.DefaultClient, lekkoURL),
		lekkoAuthClient: bffv1beta1connect.NewAuthServiceClient(http.DefaultClient, lekkoURL),
	}
}

func (a *OAuth) Close() {
	if err := a.Secrets.Close(); err != nil {
		log.Printf("error closing secrets: %v\n", err)
	}
}

// Login will attempt to read any existing github credentials from disk. If unavailable,
// it will initiate oauth with github.
func (a *OAuth) Login(ctx context.Context) error {
	defer a.Status(ctx)
	if err := a.loginLekko(ctx); err != nil {
		return errors.Wrap(err, "login lekko")
	}
	if err := a.loginGithub(ctx); err != nil {
		return errors.Wrap(err, "login github")
	}
	return nil
}

func maskToken(token string) string {
	var ret []string
	for range token {
		ret = append(ret, "*")
	}
	return strings.Join(ret, "")
}

func (a *OAuth) Logout(ctx context.Context) error {
	a.Secrets.SetGithubToken("")
	a.Secrets.SetGithubUser("")
	a.Secrets.SetGithubEmail("")
	a.Status(ctx)
	return nil
}

func (a *OAuth) Status(ctx context.Context) {
	status := "Logged In"
	if !a.Secrets.HasGithubToken() {
		status = "Logged out"
	}
	if err := a.checkGithubAuth(ctx); err != nil {
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

func (a *OAuth) loginLekko(ctx context.Context) error {
	if a.Secrets.HasLekkoToken() {
		_, err := a.checkLekkoAuth(ctx)
		if err == nil {
			return nil
		}
		log.Printf("Existing lekko token auth: %v\n", err)
	}
	authCreds, err := NewDeviceFlow(lekkoURL).Authorize(ctx)
	if err != nil {
		return errors.Wrap(err, "lekko oauth authorize")
	}
	a.Secrets.SetLekkoToken(authCreds.Token)
	a.Secrets.SetLekkoTeam(*authCreds.Team)
	username, err := a.checkLekkoAuth(ctx)
	if err != nil {
		return errors.Wrap(err, "check lekko auth after login")
	}
	a.Secrets.SetLekkoUsername(username)
	return nil
}

func (a *OAuth) loginGithub(ctx context.Context) error {
	if a.Secrets.HasGithubToken() {
		err := a.checkGithubAuth(ctx)
		if err == nil {
			return nil
		}
		log.Printf("Existing gh token expired: %v\n", err)
	}
	flow := &ghauth.Flow{
		ClientID: lekkoGHAppClientID,
		Scopes:   []string{"user", "repo"},
	}
	token, err := flow.DetectFlow()
	if err != nil {
		return errors.Wrap(err, "gh oauth flow")
	}
	a.Secrets.SetGithubToken(token.Token)
	login, email, err := a.getGithubUserLogin(ctx)
	if err != nil {
		return err
	}
	a.Secrets.SetGithubUser(login)
	a.Secrets.SetGithubEmail(email)
	return nil
}

func (a *OAuth) checkLekkoAuth(ctx context.Context) (username string, err error) {
	req := connect.NewRequest(&bffv1beta1.GetUserLoggedInInfoRequest{})
	a.setLekkoHeaders(req)
	resp, err := a.lekkoBFFClient.GetUserLoggedInInfo(ctx, req)
	if err != nil {
		return "", errors.Wrap(err, "check lekko auth")
	}
	return resp.Msg.GetUsername(), nil
}

func (a *OAuth) setLekkoHeaders(req connect.AnyRequest) {
	if a.Secrets.HasLekkoToken() {
		req.Header().Set("Authorization", fmt.Sprintf("Bearer %s", a.Secrets.GetLekkoToken()))
		if lekkoTeam := a.Secrets.GetLekkoTeam(); len(lekkoTeam) > 0 {
			req.Header().Set("X-Lekko-Team", a.Secrets.GetLekkoTeam())
		}
	}
}

func (a *OAuth) getGithubUserLogin(ctx context.Context) (string, string, error) {
	ghCli := gh.NewGithubClientFromToken(ctx, a.Secrets.GetGithubToken())
	user, err := ghCli.GetUser(ctx)
	if err != nil {
		return "", "", errors.Wrap(err, "check auth")
	}
	// Note: user.Email is only set if the user has allowed their email
	// address to be publically available on github. So it may not be populated.
	// If it is there, we will use it as part of the commit message.
	return user.GetLogin(), user.GetEmail(), nil
}

func (a *OAuth) checkGithubAuth(ctx context.Context) error {
	if _, _, err := a.getGithubUserLogin(ctx); err != nil {
		return err
	}
	return nil
}
