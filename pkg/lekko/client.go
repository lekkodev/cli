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

package lekko

import (
	"context"
	"fmt"
	"net/http"

	bffv1beta1connect "buf.build/gen/go/lekkodev/cli/bufbuild/connect-go/lekko/bff/v1beta1/bffv1beta1connect"
	connect_go "github.com/bufbuild/connect-go"
)

const (
	// Relevant headers used for auth with lekko.
	LekkoTeamHeaderKey     string = "X-Lekko-Team"
	GithubOAuthHeaderKey   string = "X-Github-Token"
	GithubUserHeaderKey    string = "X-Github-User"
	AuthorizationHeaderKey string = "Authorization"
)

var URL string

type AuthCredentials interface {
	GetLekkoUsername() string
	GetLekkoToken() string
	HasLekkoToken() bool
	GetLekkoTeam() string
	GetGithubToken() string
	GetGithubUser() string
	HasGithubToken() bool
}

func NewBFFClient(creds AuthCredentials) bffv1beta1connect.BFFServiceClient {
	interceptor := NewUserAuthInterceptor(creds)
	return bffv1beta1connect.NewBFFServiceClient(http.DefaultClient, URL, connect_go.WithInterceptors(interceptor))
}

func NewUserAuthInterceptor(a AuthCredentials) connect_go.UnaryInterceptorFunc {
	interceptor := func(next connect_go.UnaryFunc) connect_go.UnaryFunc {
		return connect_go.UnaryFunc(func(
			ctx context.Context,
			req connect_go.AnyRequest,
		) (connect_go.AnyResponse, error) {
			if a.HasLekkoToken() {
				req.Header().Set(AuthorizationHeaderKey, fmt.Sprintf("Bearer %s", a.GetLekkoToken()))
				if lekkoTeam := a.GetLekkoTeam(); len(lekkoTeam) > 0 {
					req.Header().Set(LekkoTeamHeaderKey, lekkoTeam)
				}
			}
			if a.HasGithubToken() {
				req.Header().Set(GithubOAuthHeaderKey, a.GetGithubToken())
				if ghUser := a.GetGithubUser(); len(ghUser) > 0 {
					req.Header().Set(GithubUserHeaderKey, ghUser)
				}
			}
			return next(ctx, req)
		})
	}
	return connect_go.UnaryInterceptorFunc(interceptor)
}
