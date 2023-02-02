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
	"sync"

	"github.com/bufbuild/connect-go"
	ghauth "github.com/cli/oauth"
	"github.com/lekkodev/cli/logging"
	"github.com/lekkodev/cli/pkg/gen/proto/go-connect/lekko/bff/v1beta1/bffv1beta1connect"
	bffv1beta1 "github.com/lekkodev/cli/pkg/gen/proto/go/lekko/bff/v1beta1"
	"github.com/lekkodev/cli/pkg/gh"
	"github.com/lekkodev/cli/pkg/metadata"
	"github.com/pkg/errors"
)

const (
	// The client ID is public knowledge, so this is safe to commit in version control.
	lekkoGHAppClientID string = "Iv1.031cf53c3284be35"
)

var errNoToken error = fmt.Errorf("no token")

// OAuth is responsible for managing all of the authentication
// credentials and settings on the CLI.
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
		lekkoBFFClient:  bffv1beta1connect.NewBFFServiceClient(http.DefaultClient, LekkoURL),
		lekkoAuthClient: bffv1beta1connect.NewAuthServiceClient(http.DefaultClient, LekkoURL),
	}
}

func (a *OAuth) Close() {
	if err := a.Secrets.Close(); err != nil {
		log.Printf("error closing secrets: %v\n", err)
	}
}

// Login will attempt to read any existing lekko and github credentials
// from disk. If either of those credentials don't exist, or are expired,
// we will reinitiate oauath with that provider.
func (a *OAuth) Login(ctx context.Context) error {
	defer a.Status(ctx, true)
	if err := a.loginLekko(ctx); err != nil {
		return errors.Wrap(err, "login lekko")
	}
	if err := a.loginGithub(ctx); err != nil {
		return errors.Wrap(err, "login github")
	}
	return nil
}

func maskToken(token string) string {
	if len(token) == 0 {
		return ""
	}
	parts := strings.Split(token, "_")
	parts[len(parts)-1] = "**********"
	return strings.Join(parts, "_")
}

// Logout implicitly expires the relevant credentials by
// deleting them. TODO: explore explicitly expiring these
// credentials with each provider.
func (a *OAuth) Logout(ctx context.Context) error {
	fmt.Println("Logging out...")
	a.Secrets.SetLekkoUsername("")
	a.Secrets.SetLekkoTeam("")
	a.Secrets.SetLekkoToken("")
	a.Secrets.SetGithubToken("")
	a.Secrets.SetGithubUser("")
	a.Status(ctx, true)
	return nil
}

// Status reads existing credentials and prints them out in stdout.
func (a *OAuth) Status(ctx context.Context, skipAuthCheck bool) {
	var lekkoAuthErr, ghAuthErr error
	if !skipAuthCheck {
		fmt.Println("Checking authentication status...")
	}
	lekkoAuthErr, ghAuthErr = a.authStatus(ctx, skipAuthCheck)

	authStatus := func(err error) string {
		if err == nil {
			return fmt.Sprintf("Authenticated %s✔%s", logging.Green, logging.Reset)
		}
		return fmt.Sprintf("Unauthenticated %s✖%s | %s", logging.Red, logging.Reset, err.Error())
	}

	var teamSuffix string
	if len(a.Secrets.GetLekkoTeam()) > 0 {
		teamSuffix = fmt.Sprintf(", using team %s", logging.Bold+a.Secrets.GetLekkoTeam()+logging.Reset)
	}

	lines := []string{
		fmt.Sprintf(logging.Bold+"lekko.com"+logging.Reset+" %s", authStatus(lekkoAuthErr)),
	}
	if lekkoAuthErr == nil {
		lines = append(lines, fmt.Sprintf("  Logged in to Lekko as %s%s", logging.Bold+a.Secrets.GetLekkoUsername()+logging.Reset, teamSuffix))
	}
	lines = append(lines, fmt.Sprintf("  Token: %s", maskToken(a.Secrets.GetLekkoToken())))
	lines = append(lines, fmt.Sprintf(logging.Bold+"github.com"+logging.Reset+" %s", authStatus(ghAuthErr)))
	if ghAuthErr == nil {
		lines = append(lines, fmt.Sprintf("  Logged in to GitHub as %s", logging.Bold+a.Secrets.GetGithubUser()+logging.Reset))
	}
	lines = append(lines, fmt.Sprintf("  Token: %s", maskToken(a.Secrets.GetGithubToken())))

	fmt.Println(strings.Join(lines, "\n"))
}

func (a *OAuth) authStatus(ctx context.Context, skipAuthCheck bool) (lekkoAuthErr error, ghAuthErr error) {
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		if a.Secrets.HasLekkoToken() {
			if !skipAuthCheck {
				if _, err := a.checkLekkoAuth(ctx); err != nil {
					lekkoAuthErr = err
				}
			}
		} else {
			lekkoAuthErr = errNoToken
		}
	}()
	wg.Add(1)
	go func() {
		defer wg.Done()
		if a.Secrets.HasGithubToken() {
			if !skipAuthCheck {
				if err := a.checkGithubAuth(ctx); err != nil {
					ghAuthErr = err
				}
			}
		} else {
			ghAuthErr = errNoToken
		}
	}()
	wg.Wait()
	return
}

func (a *OAuth) loginLekko(ctx context.Context) error {
	if a.Secrets.HasLekkoToken() {
		_, err := a.checkLekkoAuth(ctx)
		if err == nil {
			return nil
		}
		log.Printf("Existing lekko token auth: %v\n", err)
	}
	fmt.Println("Initiating OAuth for Lekko")
	authCreds, err := NewDeviceFlow(LekkoURL).Authorize(ctx)
	if err != nil {
		return errors.Wrap(err, "lekko oauth authorize")
	}
	a.Secrets.SetLekkoToken(authCreds.Token)
	if authCreds.Team != nil {
		a.Secrets.SetLekkoTeam(*authCreds.Team)
	}
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
	fmt.Println("Initiating OAuth for GitHub")
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
	login, err := a.getGithubUserLogin(ctx)
	if err != nil {
		return err
	}
	a.Secrets.SetGithubUser(login)
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
		req.Header().Set(AuthorizationHeaderKey, fmt.Sprintf("Bearer %s", a.Secrets.GetLekkoToken()))
		if lekkoTeam := a.Secrets.GetLekkoTeam(); len(lekkoTeam) > 0 {
			req.Header().Set(LekkoTeamHeaderKey, lekkoTeam)
		}
	}
	if a.Secrets.HasGithubToken() {
		req.Header().Set(GithubOAuthHeaderKey, a.Secrets.GetGithubToken())
		if ghUser := a.Secrets.GetGithubUser(); len(ghUser) > 0 {
			req.Header().Set(GithubUserHeaderKey, ghUser)
		}
	}
}

func (a *OAuth) getGithubUserLogin(ctx context.Context) (string, error) {
	ghCli := gh.NewGithubClientFromToken(ctx, a.Secrets.GetGithubToken())
	user, err := ghCli.GetUser(ctx)
	if err != nil {
		return "", errors.Wrap(err, "check auth")
	}
	return user.GetLogin(), nil
}

func (a *OAuth) checkGithubAuth(ctx context.Context) error {
	_, err := a.getGithubUserLogin(ctx)
	return err
}
