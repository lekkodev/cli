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

package secrets

import "errors"

// Options to specify when reading secrets.
type Option interface {
	applyToSecrets(ReadSecrets) error
}

// Assert that secrets contain a lekko token
func RequireLekkoToken() Option {
	return &require{
		apply: requireLekkoToken,
	}
}

// Assert that secrets contain a lekko team
func RequireLekkoTeam() Option {
	return &require{
		apply: requireLekkoTeam,
	}
}

// Assert that secrets contain a github token
func RequireGithubToken() Option {
	return &require{
		apply: requireGithubToken,
	}
}

// Assert that secrets contain all Lekko credentials
func RequireLekko() Option {
	return &require{
		apply: func(rs ReadSecrets) error {
			if err := requireLekkoToken(rs); err != nil {
				return err
			}
			if err := requireLekkoTeam(rs); err != nil {
				return err
			}
			if err := requireLekkoUser(rs); err != nil {
				return err
			}
			return nil
		},
	}
}

// Assert that secrets contain all Github credentials
func RequireGithub() Option {
	return &require{
		apply: func(rs ReadSecrets) error {
			if err := requireGithubToken(rs); err != nil {
				return err
			}
			if err := requireGithubUser(rs); err != nil {
				return err
			}
			return nil
		},
	}
}

func requireLekkoToken(rs ReadSecrets) error {
	if !rs.HasLekkoToken() {
		return errors.New("no lekko token, please run `lekko auth login`")
	}
	return nil
}

func requireLekkoTeam(rs ReadSecrets) error {
	if len(rs.GetLekkoTeam()) == 0 {
		return errors.New("no lekko team, please run `lekko team switch`")
	}
	return nil
}

func requireLekkoUser(rs ReadSecrets) error {
	if len(rs.GetLekkoUsername()) == 0 {
		return errors.New("no lekko email, please run `lekko auth login`")
	}
	return nil
}

func requireGithubToken(rs ReadSecrets) error {
	if !rs.HasGithubToken() {
		return errors.New("no github token, please run `lekko auth login`")
	}
	return nil
}

func requireGithubUser(rs ReadSecrets) error {
	if len(rs.GetGithubUser()) == 0 {
		return errors.New("no github user, please run `lekko auth login`")
	}
	return nil
}

type require struct {
	apply func(rs ReadSecrets) error
}

func (r *require) applyToSecrets(rs ReadSecrets) error {
	return r.apply(rs)
}
