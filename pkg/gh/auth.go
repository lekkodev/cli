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

	ghauth "github.com/cli/oauth"
	"github.com/pkg/errors"
)

func (cr *ConfigRepo) Auth() error {
	flow := &ghauth.Flow{
		Host:     ghauth.GitHubHost("https://github.com"),
		ClientID: "82ce82fd4d8f764f1977",
		Scopes:   []string{"repo"},
	}
	token, err := flow.DetectFlow()
	if err != nil {
		return errors.Wrap(err, "gh oauth flow")
	}
	fmt.Printf("received token: \n\t%v\n", *token)
	return nil
}
