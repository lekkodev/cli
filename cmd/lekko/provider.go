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

package main

import "github.com/pkg/errors"

type provider string

const (
	providerLekko  provider = "lekko"
	providerGithub provider = "github"
)

func (p *provider) String() string {
	return string(*p)
}

func (p *provider) Set(v string) error {
	switch v {
	case string(providerLekko), string(providerGithub):
		*p = provider(v)
	default:
		return errors.New(`must be one of "lekko" or "github"`)
	}
	return nil
}

func (p *provider) Type() string {
	return "provider"
}
