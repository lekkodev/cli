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
	"github.com/lekkodev/cli/pkg/gen/proto/go-connect/lekko/bff/v1beta1/bffv1beta1connect"
	bffv1beta1 "github.com/lekkodev/cli/pkg/gen/proto/go/lekko/bff/v1beta1"
	"github.com/lekkodev/cli/pkg/gh"
	"github.com/lekkodev/cli/pkg/lekko"
	"github.com/lekkodev/cli/pkg/logging"
	"github.com/lekkodev/cli/pkg/secrets"
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
	lekkoBFFClient  bffv1beta1connect.BFFServiceClient
	lekkoAuthClient bffv1beta1connect.AuthServiceClient
}

// Returns an OAuth object, responsible for managing oauth on the local FS.
// This is meant to be used by the cli on the user's local filesystem.
func NewOAuth(bff bffv1beta1connect.BFFServiceClient) *OAuth {
	return &OAuth{
		lekkoBFFClient:  bff,
		lekkoAuthClient: bffv1beta1connect.NewAuthServiceClient(http.DefaultClient, lekko.URL),
	}
}

// Login will attempt to read any existing lekko and github credentials
// from disk. If either of those credentials don't exist, or are expired,
// we will reinitiate oauath with that provider.
func (a *OAuth) Login(ctx context.Context, ws secrets.WriteSecrets) error {
	defer a.Status(ctx, true, ws)
	if err := a.loginLekko(ctx, ws); err != nil {
		return errors.Wrap(err, "login lekko")
	}
	if err := a.loginGithub(ctx, ws); err != nil {
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
func (a *OAuth) Logout(ctx context.Context, provider string, ws secrets.WriteSecrets) error {
	if !(provider == "lekko" || provider == "github") {
		return fmt.Errorf("provider must be one of 'lekko' or 'github'")
	}
	fmt.Printf("Logging out of %s...\n", provider)
	if provider == "lekko" {
		ws.SetLekkoUsername("")
		ws.SetLekkoTeam("")
		ws.SetLekkoToken("")
	}
	if provider == "github" {
		ws.SetGithubToken("")
		ws.SetGithubUser("")
	}
	a.Status(ctx, true, ws)
	return nil
}

func (a *OAuth) Register(ctx context.Context, username, password string) error {
	resp, err := a.lekkoAuthClient.RegisterUser(ctx, connect.NewRequest(&bffv1beta1.RegisterUserRequest{
		Username: username,
		Password: password,
	}))
	if err != nil {
		return errors.Wrap(err, "register user")
	}
	if resp.Msg.GetAccountExisted() {
		return errors.Errorf("Account with user '%s' already exists", username)
	}
	return nil
}

func (a *OAuth) Tokens(ctx context.Context, rs secrets.ReadSecrets) []string {
	return []string{
		rs.GetLekkoToken(),
		rs.GetGithubToken(),
	}
}

// Status reads existing credentials and prints them out in stdout.
func (a *OAuth) Status(ctx context.Context, skipAuthCheck bool, rs secrets.ReadSecrets) {
	var lekkoAuthErr, ghAuthErr error
	if !skipAuthCheck {
		fmt.Println("Checking authentication status...")
	}
	lekkoAuthErr, ghAuthErr = a.authStatus(ctx, skipAuthCheck, rs)

	authStatus := func(err error) string {
		if err == nil {
			return fmt.Sprintf("Authenticated %s", logging.Green("✔"))
		}
		return fmt.Sprintf("Unauthenticated %s | %s", logging.Red("✖"), err.Error())
	}

	var teamSuffix string
	if len(rs.GetLekkoTeam()) > 0 {
		teamSuffix = fmt.Sprintf(", using team %s", logging.Bold(rs.GetLekkoTeam()))
	}

	lines := []string{
		fmt.Sprintf("%s %s", logging.Bold("lekko.com"), authStatus(lekkoAuthErr)),
	}
	if lekkoAuthErr == nil {
		lines = append(lines, fmt.Sprintf("  Logged in to Lekko as %s%s", logging.Bold(rs.GetLekkoUsername()), teamSuffix))
	}
	lines = append(lines, fmt.Sprintf("  Token: %s", maskToken(rs.GetLekkoToken())))
	lines = append(lines, fmt.Sprintf("%s %s", logging.Bold("github.com"), authStatus(ghAuthErr)))
	if ghAuthErr == nil {
		lines = append(lines, fmt.Sprintf("  Logged in to GitHub as %s", logging.Bold(rs.GetGithubUser())))
	}
	lines = append(lines, fmt.Sprintf("  Token: %s", maskToken(rs.GetGithubToken())))

	fmt.Println(strings.Join(lines, "\n"))
}

func (a *OAuth) authStatus(ctx context.Context, skipAuthCheck bool, rs secrets.ReadSecrets) (lekkoAuthErr error, ghAuthErr error) {
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		if rs.HasLekkoToken() {
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
		if rs.HasGithubToken() {
			if !skipAuthCheck {
				if err := a.checkGithubAuth(ctx, rs); err != nil {
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

func (a *OAuth) loginLekko(ctx context.Context, ws secrets.WriteSecrets) error {
	if ws.HasLekkoToken() {
		_, err := a.checkLekkoAuth(ctx)
		if err == nil {
			return nil
		}
		log.Printf("Existing lekko token auth: %v\n", err)
	}
	fmt.Println("Initiating OAuth for Lekko")
	authCreds, err := NewDeviceFlow(lekko.URL).Authorize(ctx)
	if err != nil {
		return errors.Wrap(err, "lekko oauth authorize")
	}
	ws.SetLekkoToken(authCreds.Token)
	if authCreds.Team != nil {
		ws.SetLekkoTeam(*authCreds.Team)
	}
	username, err := a.checkLekkoAuth(ctx)
	if err != nil {
		return errors.Wrap(err, "check lekko auth after login")
	}
	ws.SetLekkoUsername(username)
	return nil
}

func (a *OAuth) loginGithub(ctx context.Context, ws secrets.WriteSecrets) error {
	if ws.HasGithubToken() {
		err := a.checkGithubAuth(ctx, ws)
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
	ws.SetGithubToken(token.Token)
	login, err := a.getGithubUserLogin(ctx, ws)
	if err != nil {
		return err
	}
	ws.SetGithubUser(login)
	return nil
}

func (a *OAuth) checkLekkoAuth(ctx context.Context) (username string, err error) {
	req := connect.NewRequest(&bffv1beta1.GetUserLoggedInInfoRequest{})
	resp, err := a.lekkoBFFClient.GetUserLoggedInInfo(ctx, req)
	if err != nil {
		return "", errors.Wrap(err, "check lekko auth")
	}
	return resp.Msg.GetUsername(), nil
}

func (a *OAuth) getGithubUserLogin(ctx context.Context, rs secrets.ReadSecrets) (string, error) {
	ghCli := gh.NewGithubClientFromToken(ctx, rs.GetGithubToken())
	user, err := ghCli.GetUser(ctx)
	if err != nil {
		return "", errors.Wrap(err, "check auth")
	}
	return user.GetLogin(), nil
}

func (a *OAuth) checkGithubAuth(ctx context.Context, rs secrets.ReadSecrets) error {
	_, err := a.getGithubUserLogin(ctx, rs)
	return err
}
