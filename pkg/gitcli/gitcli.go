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
	"os"
	"os/exec"

	"github.com/pkg/errors"
)

func disablePrompts(cmd *exec.Cmd) {
	cmd.Env = append(os.Environ(), "GIT_TERMINAL_PROMPT=0")
}

func handleAuthFailures(output []byte, err error) ([]byte, error) {
	if err == nil {
		return output, nil
	}
	if bytes.Contains(output, []byte("Repository not found")) {
		return output, errors.New("Repository not found. Make sure that the repository exists and you have access to it.")
	}
	if bytes.Contains(output, []byte("terminal prompts disabled")) {
		return output, errors.New("Failed to read GitHub credentials, please follow https://docs.github.com/en/authentication/keeping-your-account-and-data-secure/about-authentication-to-github#authenticating-with-the-command-line to authenticate with GitHub.")
	}
	return output, err
}

func Clone(url, path string) ([]byte, error) {
	cmd := exec.Command("git", "clone", url, path)
	disablePrompts(cmd)
	return handleAuthFailures(cmd.CombinedOutput())
}

func Pull(path string) ([]byte, error) {
	cmd := exec.Command("git", "pull")
	disablePrompts(cmd)
	cmd.Dir = path
	return handleAuthFailures(cmd.CombinedOutput())
}

func Push(path string) ([]byte, error) {
	cmd := exec.Command("git", "push")
	disablePrompts(cmd)
	cmd.Dir = path
	return handleAuthFailures(cmd.CombinedOutput())
}
