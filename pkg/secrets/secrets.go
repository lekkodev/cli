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

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/lekkodev/cli/pkg/metadata"
	"github.com/pkg/errors"
	"github.com/zalando/go-keyring"
	"gopkg.in/yaml.v2"
)

// WriteSecrets holds all the user-specific information that needs to exist for the cli
// to work, but should not live in a shared config repo. For instance, it holds
// the github auth token. The secrets are backed by the filesystem under the user's home
// directory, so these secrets don't need to be fetched as part of every cli command.
type WriteSecrets interface {
	ReadSecrets
	SetLekkoUsername(username string)
	SetLekkoToken(token string)
	SetLekkoTeam(team string)
	SetGithubToken(token string)
	SetGithubUser(user string)
	SetLekkoAPIKey(apikey string)
}

type ReadSecrets interface {
	GetLekkoUsername() string
	GetLekkoToken() string
	HasLekkoToken() bool
	GetLekkoTeam() string
	HasLekkoAPIKey() bool
	GetLekkoAPIKey() string
	GetGithubToken() string
	GetGithubUser() string
	HasGithubToken() bool
	GetUsername() string
	GetToken() string
}

type secrets struct {
	LekkoUsername string `json:"lekko_username,omitempty" yaml:"lekko_username,omitempty"`
	LekkoToken    string `json:"lekko_token,omitempty" yaml:"lekko_token,omitempty"`
	LekkoTeam     string `json:"lekko_team,omitempty" yaml:"lekko_team,omitempty"`
	LekkoAPIKey   string `json:"lekko_api_key,omitempty" yaml:"lekko_api_key,omitempty"`
	LekkoRepoPath string `json:"lekko_repo_path,omitempty" yaml:"lekko_repo_path,omitempty"`
	GithubUser    string `json:"github_user,omitempty" yaml:"github_user,omitempty"`
	GithubToken   string `json:"github_token,omitempty" yaml:"github_token,omitempty"`

	homeDir      string
	changed      bool
	sync.RWMutex `json:"-" yaml:"-"`
}

func NewSecretsOrFail(opts ...Option) ReadSecrets {
	rs, err := newSecrets()
	if err != nil {
		log.Fatalf("secrets: %v", err)
	}
	for _, opt := range opts {
		if err := opt.applyToSecrets(rs); err != nil {
			log.Fatalf("secrets: %v", err)
		}
	}
	return rs
}

func NewSecretsFromEnv() ReadSecrets {
	return &secrets{
		LekkoUsername: os.Getenv("LEKKO_USERNAME"),
		LekkoToken:    os.Getenv("LEKKO_TOKEN"),
		LekkoTeam:     os.Getenv("LEKKO_TEAM"),
		LekkoAPIKey:   os.Getenv("LEKKO_API_PATH"),
	}
}

func WithWriteSecrets(f func(WriteSecrets) error, opts ...Option) error {
	s, err := newSecrets()
	if err != nil {
		return err
	}
	for _, opt := range opts {
		if err := opt.applyToSecrets(s); err != nil {
			log.Fatalf("secrets: %v", err)
		}
	}
	if err := f(s); err != nil {
		return err
	}
	if !s.changed {
		return nil
	}
	s.Lock()
	defer s.Unlock()
	return s.save()
}

func newSecrets() (*secrets, error) {
	hd, err := os.UserHomeDir()
	if err != nil {
		return nil, errors.Wrap(err, "user home directory")
	}
	s := &secrets{homeDir: hd}
	s.load()
	return s, nil
}

func (s *secrets) readFromFile() error {
	bytes, err := os.ReadFile(s.filename())
	if err != nil {
		return errors.Wrap(err, "read file")
	}
	if err := metadata.UnmarshalYAMLStrict(bytes, s); err != nil {
		return fmt.Errorf("unmarshal secrets from file %s: %w", s.filename(), err)
	}
	return nil
}

func (s *secrets) readFromKeyring() error {
	secretsYaml, err := keyring.Get("lekko", "secrets.yaml")
	if err != nil {
		return err
	}
	err = yaml.NewDecoder(strings.NewReader(secretsYaml)).Decode(s)
	if err != nil {
		return errors.Wrap(err, "decode yaml")
	}
	return nil
}

func (s *secrets) load() {
	s.Lock()
	defer s.Unlock()
	// read from keyring first
	err := s.readFromKeyring()
	// on failure, try reading from file
	if err != nil {
		err = s.readFromFile()
		// err != nil means that there are no Lekko secrets on this device,
		// so we start with empty secrets.
		// Otherwise we migrate the secrets to keyring.
		if err == nil {
			// save to keyring
			err = s.save()
			// migrated to keyring, we don't need the file anymore
			if err == nil {
				os.Remove(s.filename())
			}
		}
	}
}

func (s *secrets) save() error {
	var buffer strings.Builder
	if err := yaml.NewEncoder(&buffer).Encode(s); err != nil {
		return errors.Wrap(err, "failed to encode secrets")
	}
	if err := keyring.Set("lekko", "secrets.yaml", buffer.String()); err != nil {
		return errors.Wrap(err, "failed to set secrets in keyring")
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

func (s *secrets) HasLekkoAPIKey() bool {
	return len(s.GetLekkoAPIKey()) > 0
}

func (s *secrets) GetLekkoAPIKey() string {
	s.RLock()
	defer s.RUnlock()
	return s.LekkoAPIKey
}

func (s *secrets) SetLekkoAPIKey(apikey string) {
	s.Lock()
	defer s.Unlock()
	s.changed = true
	s.LekkoAPIKey = apikey
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
