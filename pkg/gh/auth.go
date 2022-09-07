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
	"fmt"
	"strings"

	ghauth "github.com/cli/oauth"
	"github.com/pkg/errors"
)

const (
	lekkoGHAppClientID string = "Iv1.031cf53c3284be35"
	// lekkoOAuthClientID string = "82ce82fd4d8f764f1977"
)

func (cr *ConfigRepo) Auth() error {
	ghToken := cr.secrets.GetGithubToken()
	if ghToken != "" {
		fmt.Printf("Found existing gh token: %v\n", maskToken(ghToken))
		return nil
	}
	flow := &ghauth.Flow{
		Host:     ghauth.GitHubHost("https://github.com"),
		ClientID: lekkoGHAppClientID,
		Scopes:   []string{"repo", "user"},
	}
	token, err := flow.DetectFlow()
	if err != nil {
		return errors.Wrap(err, "gh oauth flow")
	}
	cr.secrets.SetGithubToken(token.Token)
	return nil
}

func maskToken(token string) string {
	var ret []string
	for range token {
		ret = append(ret, "*")
	}
	return strings.Join(ret, "")
}
