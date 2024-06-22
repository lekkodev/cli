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

package gitcli

import (
	"bytes"
	"os/exec"

	"github.com/pkg/errors"
)

func Clone(url, path string) error {
	cmd := exec.Command("git", "clone", url, path)
	var buf bytes.Buffer
	cmd.Stdout = &buf
	cmd.Stderr = &buf
	if err := cmd.Run(); err != nil {
		return errors.Wrap(err, buf.String())
	}
	return nil
}

func Pull(path string) error {
	cmd := exec.Command("git", "pull")
	cmd.Dir = path
	var buf bytes.Buffer
	cmd.Stdout = &buf
	cmd.Stderr = &buf
	if err := cmd.Run(); err != nil {
		return errors.Wrap(err, buf.String())
	}
	return nil
}

func Push(path string) ([]byte, error) {
	cmd := exec.Command("git", "push")
	cmd.Dir = path
	return cmd.CombinedOutput()
}
