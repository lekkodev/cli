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

package sync

import (
	"fmt"
	"os/exec"
	"strings"

	"github.com/pkg/errors"
)

func BisyncTS(lekkoPath, repoPath string) error {
	tsSyncCmd := exec.Command("npx", "lekko-repo-sync", "--lekko-dir", lekkoPath, "--repo-path", repoPath)
	output, err := tsSyncCmd.CombinedOutput()
	outputStr := strings.TrimSpace(string(output))
	if len(outputStr) > 0 {
		fmt.Println(outputStr)
	}
	if err != nil {
		return errors.Wrap(err, "Lekko Typescript tools not found, please make sure that you are inside a node project and have up to date Lekko packages.")
	}
	return nil
}
