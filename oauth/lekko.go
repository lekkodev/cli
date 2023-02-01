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
	"bufio"
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"

	"github.com/bufbuild/connect-go"
	"github.com/cenkalti/backoff/v4"
	"github.com/cli/browser"
	"github.com/lekkodev/cli/pkg/gen/proto/go-connect/lekko/bff/v1beta1/bffv1beta1connect"
	bffv1beta1 "github.com/lekkodev/cli/pkg/gen/proto/go/lekko/bff/v1beta1"
	"github.com/pkg/errors"
)

const (
	// Lekko CLI client ID. Used for oauth with lekko.
	LekkoClientID string = "v0.303976a05d96c02eee5b1a75a3923815d82599b0"
	grantType     string = "urn:ietf:params:oauth:grant-type:device_code"
)

// DeviceFlow initiates the OAuth 2.0 device authorization
// flow with Lekko.
type DeviceFlow struct {
	lekkoAuthClient bffv1beta1connect.AuthServiceClient
}

func NewDeviceFlow(lekkoURL string) *DeviceFlow {
	return &DeviceFlow{
		lekkoAuthClient: bffv1beta1connect.NewAuthServiceClient(http.DefaultClient, lekkoURL),
	}
}

type AuthCredentials struct {
	// OAuth token to be used in rpcs to Lekko.
	Token string
	// Optional team that the user is a member of and is currently operating on
	Team *string
}

func (f *DeviceFlow) Authorize(ctx context.Context) (*AuthCredentials, error) {
	resp, err := f.lekkoAuthClient.GetDeviceCode(ctx, connect.NewRequest(&bffv1beta1.GetDeviceCodeRequest{
		ClientId: LekkoClientID,
	}))
	if err != nil {
		return nil, errors.Wrap(err, "get device code")
	}
	// open browser
	fmt.Printf("First, copy your one-time code: %s\n", resp.Msg.UserCode)
	fmt.Print("Then press [Enter] to continue in the web browser... ")
	_ = waitForEnter(os.Stdin)
	if err := browser.OpenURL(resp.Msg.GetVerificationUriComplete()); err != nil {
		return nil, errors.Wrapf(err, "error opening the web browser at url %s", resp.Msg.GetVerificationUriComplete())
	}
	// poll
	return f.pollToken(ctx, resp.Msg.GetDeviceCode(), time.Second*time.Duration(resp.Msg.GetIntervalS()))
}

func (f *DeviceFlow) pollToken(ctx context.Context, deviceCode string, interval time.Duration) (*AuthCredentials, error) {
	var ret *AuthCredentials

	operation := func() error {
		resp, err := f.lekkoAuthClient.GetAccessToken(ctx, connect.NewRequest(&bffv1beta1.GetAccessTokenRequest{
			GrantType:  grantType,
			DeviceCode: deviceCode,
			ClientId:   LekkoClientID,
		}))
		if err != nil {
			return backoff.Permanent(errors.Wrap(err, "get access token"))
		}
		if len(resp.Msg.GetAccessToken()) == 0 {
			return fmt.Errorf("no access token yet, retry")
		}
		ret := &AuthCredentials{
			Token: resp.Msg.GetAccessToken(),
		}
		if len(resp.Msg.GetTeamName()) > 0 {
			ret.Team = &resp.Msg.TeamName
		}
		return nil
	}

	return ret, backoff.Retry(operation, backoff.NewConstantBackOff(interval))
}

func waitForEnter(r io.Reader) error {
	scanner := bufio.NewScanner(r)
	scanner.Scan()
	return scanner.Err()
}
