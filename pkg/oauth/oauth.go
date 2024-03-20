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
	"time"

	bffv1beta1connect "buf.build/gen/go/lekkodev/cli/bufbuild/connect-go/lekko/bff/v1beta1/bffv1beta1connect"
	bffv1beta1 "buf.build/gen/go/lekkodev/cli/protocolbuffers/go/lekko/bff/v1beta1"
	"github.com/briandowns/spinner"
	connect_go "github.com/bufbuild/connect-go"
	ghauth "github.com/cli/oauth"
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

func maskToken(token, prefix string) string {
	if len(token) == 0 {
		return ""
	}
	if !strings.HasPrefix(token, prefix) {
		return "**********"
	}
	return prefix + token[len(prefix):min(len(token), len(prefix)+4)] + "**********"
}

// Logout implicitly expires the relevant credentials by
// deleting them. TODO: explore explicitly expiring these
// credentials with each provider.
func (a *OAuth) Logout(ctx context.Context, provider string, ws secrets.WriteSecrets, noStatus bool) error {
	if !(provider == "lekko" || provider == "github") {
		return fmt.Errorf("provider must be one of 'lekko' or 'github'")
	}
	fmt.Printf("Logging out of %s...\n", provider)
	if provider == "lekko" {
		ws.SetLekkoUsername("")
		ws.SetLekkoTeam("")
		ws.SetLekkoToken("")
		ws.SetLekkoAPIKey("")
	}
	if provider == "github" {
		ws.SetGithubToken("")
		ws.SetGithubUser("")
	}
	if !noStatus {
		a.Status(ctx, true, ws)
	}
	return nil
}

type ErrUserAlreadyExists struct {
	username string
}

func (e ErrUserAlreadyExists) Error() string {
	return fmt.Sprintf("Account with user '%s' already exists", e.username)
}

func (a *OAuth) PreRegister(ctx context.Context, username string, ws secrets.WriteSecrets) error {
	s := spinner.New(spinner.CharSets[14], 100*time.Millisecond)
	s.Suffix = " Generating OAuth code..."
	s.Start()
	dcResp, err := a.lekkoAuthClient.GetDeviceCode(ctx, connect_go.NewRequest(&bffv1beta1.GetDeviceCodeRequest{
		ClientId: LekkoClientID,
	}))
	s.Stop()
	if err != nil {
		return errors.Wrap(err, "Failed to get device code")
	}
	s.Suffix = " Connecting to Lekko..."
	s.Start()
	_, err = a.lekkoAuthClient.PreRegisterUser(ctx, connect_go.NewRequest(&bffv1beta1.PreRegisterUserRequest{
		Username: username,
		// We're passing in user code just because that's what backend handles better at the moment
		DeviceCode: dcResp.Msg.UserCode,
	}))
	s.Stop()
	// TODO: Better error handling for different error types
	if err != nil {
		return errors.Wrap(err, "Failed pre-registration step for user")
	}
	// Wait for user to finish registration step, which will make an access token available
	// TODO: Handle cancellations gracefully
	s.Suffix = " Check your email address for a sign-up link. Waiting for registration to complete, do not close this session..."
	s.Start()
	df := NewDeviceFlow(lekko.URL)
	creds, err := df.pollToken(ctx, dcResp.Msg.DeviceCode, time.Second*time.Duration(dcResp.Msg.IntervalS))
	s.Stop()
	if err != nil {
		return errors.Wrap(err, "Failed to login to Lekko")
	}
	ws.SetLekkoUsername(username)
	ws.SetLekkoToken(creds.Token)

	fmt.Printf("Sign-up complete! You are now logged into Lekko as %s.\n", logging.Bold(username))
	return nil
}

func (a *OAuth) Register(ctx context.Context, username, password, confirmPassword string) error {
	registerResp, err := a.lekkoAuthClient.RegisterUser(ctx, connect_go.NewRequest(&bffv1beta1.RegisterUserRequest{
		Username:        username,
		Password:        password,
		ConfirmPassword: confirmPassword,
	}))
	if err != nil {
		return errors.Wrap(err, "register user")
	}
	if registerResp.Msg.GetAccountExisted() {
		return ErrUserAlreadyExists{username: username}
	}
	return nil
}

func (a *OAuth) ConfirmUser(ctx context.Context, username, code string) error {
	_, err := a.lekkoAuthClient.ConfirmUser(ctx, connect_go.NewRequest(&bffv1beta1.ConfirmUserRequest{
		Username: username,
		Code:     code,
	}))
	if err != nil {
		return errors.Wrap(err, "confirm user")
	}
	return nil
}

func (a *OAuth) Tokens(ctx context.Context, rs secrets.ReadSecrets) []string {
	return []string{
		rs.GetLekkoToken(),
		rs.GetGithubToken(),
	}
}

func (a *OAuth) ForgotPassword(ctx context.Context, email string) error {
	_, err := a.lekkoAuthClient.ForgotPassword(ctx, connect_go.NewRequest(
		&bffv1beta1.ForgotPasswordRequest{Username: email}),
	)
	return err
}

func (a *OAuth) ConfirmForgotPassword(
	ctx context.Context, email string, newPassword string, confirmNewPassword string, verificationCode string,
) error {
	_, err := a.lekkoAuthClient.ConfirmForgotPassword(ctx, connect_go.NewRequest(
		&bffv1beta1.ConfirmForgotPasswordRequest{
			Username:           email,
			NewPassword:        newPassword,
			ConfirmNewPassword: confirmNewPassword,
			VerificationCode:   verificationCode,
		}),
	)
	return err
}

func (a *OAuth) ResendVerification(ctx context.Context, email string) error {
	_, err := a.lekkoAuthClient.ResendVerificationCode(ctx, connect_go.NewRequest(
		&bffv1beta1.ResendVerificationCodeRequest{Username: email}),
	)
	return err
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
	lines = append(lines, fmt.Sprintf("  Token: %s", maskToken(rs.GetLekkoToken(), "lekko_oauth_")))
	lines = append(lines, fmt.Sprintf("  API Key: %s", maskToken(rs.GetLekkoAPIKey(), "lekko_")))
	lines = append(lines, fmt.Sprintf("%s %s", logging.Bold("github.com"), authStatus(ghAuthErr)))
	if ghAuthErr == nil {
		lines = append(lines, fmt.Sprintf("  Logged in to GitHub as %s", logging.Bold(rs.GetGithubUser())))
	}
	lines = append(lines, fmt.Sprintf("  Token: %s", maskToken(rs.GetGithubToken(), "ghu_")))

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
	req := connect_go.NewRequest(&bffv1beta1.GetUserLoggedInInfoRequest{})
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
