// Copyright 2020-2022 Buf Technologies, Inc.
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

package verify

import (
	"os"
	"path/filepath"

	"github.com/lekkodev/cli/pkg/config"
	"github.com/lekkodev/cli/pkg/encoding"
)

// Verifies that a configuration from a root is properly formatted.
func Verify(rootPath string) error {
	_, nsNameToNsConfigs, err := config.ParseFullConfigStrict(rootPath)
	if err != nil {
		return err
	}
	for ns, nsConfig := range nsNameToNsConfigs {
		files, err := os.ReadDir(filepath.Join(rootPath, ns))
		if err != nil {
			return err
		}
		for _, file := range files {
			if file.Name() == config.DefaultNamespaceConfigFileName {
				// Do not parse the yaml config.
				continue
			}

			contents, err := os.ReadFile(filepath.Join(rootPath, ns, file.Name()))
			if err != nil {
				return err
			}
			if _, err := encoding.ParseFeature(contents, nsConfig.Version); err != nil {
				return err
			}
		}
	}
	return nil
}
