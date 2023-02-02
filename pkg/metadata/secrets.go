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

package metadata

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sync"

	"github.com/pkg/errors"
)

const AuthenticateGituhubMessage = "User is not authenticated.\nRun 'lekko auth login' to authenticate with GitHub."

// Secrets holds all the user-specific information that needs to exist for the cli
// to work, but should not live in a shared config repo. For instance, it holds
// the github auth token. The secrets are backed by the filesystem under the user's home
// directory, so these secrets don't need to be fetched as part of every cli command.
type Secrets interface {
	GetLekkoUsername() string
	SetLekkoUsername(username string)
	GetLekkoToken() string
	SetLekkoToken(token string)
	HasLekkoToken() bool
	GetLekkoTeam() string
	SetLekkoTeam(team string)
	GetGithubToken() string
	SetGithubToken(token string)
	GetGithubUser() string
	SetGithubUser(user string)
	HasGithubToken() bool
	Close() error
}

type secrets struct {
	LekkoUsername string `json:"lekko_username,omitempty" yaml:"lekko_username,omitempty"`
	LekkoToken    string `json:"lekko_token,omitempty" yaml:"lekko_token,omitempty"`
	LekkoTeam     string `json:"lekko_team,omitempty" yaml:"lekko_team,omitempty"`
	GithubUser    string `json:"github_user,omitempty" yaml:"github_user,omitempty"`
	GithubToken   string `json:"github_token,omitempty" yaml:"github_token,omitempty"`
	GithubEmail   string `json:"github_email,omitempty" yaml:"github_email,omitempty"`

	homeDir      string
	changed      bool
	sync.RWMutex `json:"-" yaml:"-"`
}

// Instantiates new secrets, attempting to read from local disk if available.
// NOTE: always defer *Secrets.Close after calling this method.
func NewSecrets(homeDir string) *secrets {
	s := &secrets{homeDir: homeDir}
	if err := s.ReadOrCreate(); err != nil {
		log.Printf("failed to read secrets: %v\n", err)
	}
	return s
}

func NewSecretsOrFail() *secrets {
	hd, err := os.UserHomeDir()
	if err != nil {
		log.Fatalf("user home directory: %v", err)
	}
	return NewSecrets(hd)
}

func NewSecretsOrError() (*secrets, error) {
	hd, err := os.UserHomeDir()
	if err != nil {
		return nil, errors.Wrap(err, "user home directory")
	}
	s := &secrets{homeDir: hd}
	if err := s.ReadOrCreate(); err != nil {
		return nil, errors.Wrap(err, "failed to read secrets")
	}
	return s, nil
}

func (s *secrets) ReadOrCreate() error {
	s.Lock()
	defer s.Unlock()
	bytes, err := os.ReadFile(s.filename())
	if err != nil {
		if os.IsNotExist(err) {
			if err := s.create(); err != nil {
				return errors.Wrap(err, "create")
			}
		} else {
			return errors.Wrap(err, "read file")
		}
	}
	if err := UnmarshalYAMLStrict(bytes, s); err != nil {
		return fmt.Errorf("unmarshal secrets from file %s: %w", s.filename(), err)
	}
	return nil
}

func (s *secrets) Close() error {
	s.Lock()
	defer s.Unlock()
	if !s.changed {
		return nil
	}
	return s.create()
}

func (s *secrets) create() error {
	bytes, err := MarshalYAML(s)
	if err != nil {
		return errors.Wrap(err, "failed to marshal secrets")
	}
	if err := os.MkdirAll(filepath.Dir(s.filename()), 0755); err != nil {
		return errors.Wrap(err, "failed to mkdir")
	}
	if err := os.WriteFile(s.filename(), bytes, 0600); err != nil {
		return errors.Wrap(err, "failed to write secrets file")
	}
	return nil
}

func (s *secrets) GetGithubToken() string {
	s.RLock()
	defer s.RUnlock()
	return s.GithubToken
}

func (s *secrets) SetGithubToken(token string) {
	s.Lock()
	defer s.Unlock()
	s.changed = true
	s.GithubToken = token
}

func (s *secrets) GetGithubUser() string {
	s.RLock()
	defer s.RUnlock()
	return s.GithubUser
}

func (s *secrets) SetGithubUser(user string) {
	s.Lock()
	defer s.Unlock()
	s.changed = true
	s.GithubUser = user
}

func (s *secrets) GetLekkoUsername() string {
	s.RLock()
	defer s.RUnlock()
	return s.LekkoUsername
}

func (s *secrets) SetLekkoUsername(username string) {
	s.Lock()
	defer s.Unlock()
	s.changed = true
	s.LekkoUsername = username
}

func (s *secrets) GetLekkoToken() string {
	s.RLock()
	defer s.RUnlock()
	return s.LekkoToken
}

func (s *secrets) SetLekkoToken(token string) {
	s.Lock()
	defer s.Unlock()
	s.changed = true
	s.LekkoToken = token
}

func (s *secrets) HasLekkoToken() bool {
	return len(s.GetLekkoToken()) > 0
}

func (s *secrets) GetLekkoTeam() string {
	s.RLock()
	defer s.RUnlock()
	return s.LekkoTeam
}

func (s *secrets) SetLekkoTeam(team string) {
	s.Lock()
	defer s.Unlock()
	s.changed = true
	s.LekkoTeam = team
}

func (s *secrets) HasGithubToken() bool {
	return len(s.GetGithubToken()) > 0
}

/*
	Methods that implement repo.AuthProvider. For now, we only
	support Github, but in the future, we may support multiple
	providers.
*/

func (s *secrets) GetToken() string {
	return s.GetGithubToken()
}

func (s *secrets) GetUsername() string {
	return s.GetGithubUser()
}

func (s *secrets) filename() string {
	return filepath.Join(s.homeDir, ".config", "lekko", "secrets.yaml")
}
