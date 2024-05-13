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

package dotlekko

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/pkg/errors"
	"gopkg.in/yaml.v3"
)

// Parsed schema for .lekko config/lock file in code repos.
// Allows the Lekko CLI and build tools to be able to fetch/generate correct information.
type DotLekko struct {
	// v1
	Version string `json:"version" yaml:"version"`
	// <owner>/<name>
	Repository string `json:"repository" yaml:"repository,omitempty"`
	// Where lekko/, the directory where config functions are managed, is in the project
	LekkoPath string `json:"lekkoPath" yaml:"lekko_path"`
	// The latest pushed commit SHA in the config repo from the perspective of the code repo.
	// Allows stable regeneration and correct 3-way merges.
	// This field is generated/managed by the CLI and should not be modified by hand.
	LockSHA string `json:"lockSHA" yaml:"lock_sha,omitempty"`

	repoOwner string
	repoName  string
	path      string
}

func NewDotLekko(lekkoPath string) *DotLekko {
	return &DotLekko{
		Version:   "v1",
		LekkoPath: lekkoPath,
		path:      ".lekko",
	}
}

// Looks for .lekko or .lekko.(yaml|yml) in the working directory
// Returns the parsed configuration object and the path to the file.
func ReadDotLekko(codeRepoPath string) (*DotLekko, error) {
	var bareMissing, yamlMissing, ymlMissing bool
	var path string
	barePath := filepath.Join(codeRepoPath, ".lekko")
	yamlPath := filepath.Join(codeRepoPath, ".lekko.yaml")
	ymlPath := filepath.Join(codeRepoPath, ".lekko.yml")

	if _, err := os.Stat(barePath); err != nil {
		bareMissing = true
	}
	if _, err := os.Stat(yamlPath); err != nil {
		yamlMissing = true
	}
	if _, err := os.Stat(ymlPath); err != nil {
		ymlMissing = true
	}
	if bareMissing && yamlMissing && ymlMissing {
		return nil, errors.New("missing Lekko configuration file: .lekko or .lekko.(yaml|yml) not found")
	}
	if !bareMissing {
		path = barePath
	} else if !yamlMissing {
		path = yamlPath
	} else {
		path = ymlPath
	}
	f, err := os.Open(path)
	if err != nil {
		return nil, errors.Wrapf(err, "open %s", path)
	}
	defer f.Close()

	dot, err := ParseDotLekko(f)
	if err != nil {
		return nil, errors.Wrapf(err, "parse %s", path)
	}
	dot.path = path
	return dot, nil
}

func ParseDotLekko(r io.Reader) (*DotLekko, error) {
	yd := yaml.NewDecoder(r)
	yd.KnownFields(true)

	dot := &DotLekko{}
	if err := yd.Decode(dot); err != nil {
		return nil, errors.Wrap(err, "yaml decode")
	}
	if dot.Version != "v1" {
		return nil, fmt.Errorf("invalid .lekko version: %s", dot.Version)
	}
	// It's okay for config repository to be not set (e.g. testing before remote repo is set up)
	// but if it's set, it should be validly formatted
	if len(dot.Repository) > 0 {
		cr := strings.Split(dot.Repository, "/")
		if len(cr) != 2 {
			return nil, errors.New("invalid config repository format, must be like '<owner>/<name>'")
		}
		dot.repoOwner = cr[0]
		dot.repoName = cr[1]
	}
	if len(dot.LekkoPath) == 0 {
		return nil, errors.New("missing lekko_path")
	}
	if strings.Contains(dot.LekkoPath, "~") {
		return nil, errors.New("lekko_path must be relative to project root")
	}
	// Resolve relative, etc.
	dot.LekkoPath = filepath.Clean(dot.LekkoPath)

	return dot, nil
}

func (d *DotLekko) GetRepoInfo() (string, string) {
	return d.repoOwner, d.repoName
}

// Get path where the dotlekko object was read from
func (d *DotLekko) GetPath() string {
	return d.path
}

// Writes back the contents of the dotlekko object to the path it was read from
func (d *DotLekko) WriteBack() error {
	// TODO: Figure out how to do comments - e.g. don't touch lock_sha field
	f, err := os.Create(d.path)
	if err != nil {
		return errors.Wrapf(err, "open %s to write back", d.path)
	}
	defer f.Close()

	ye := yaml.NewEncoder(f)
	if err := ye.Encode(d); err != nil {
		return errors.Wrap(err, "marshal yaml")
	}
	return nil
}
