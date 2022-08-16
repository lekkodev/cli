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

package generate

import (
	"path/filepath"

	"github.com/lekkodev/cli/pkg/feature"
	"github.com/lekkodev/cli/pkg/fs"
	"github.com/lekkodev/cli/pkg/metadata"
)

// Compiles each namespace.
func Compile(rootPath string) error {
	_, nsNameToNsMDs, err := metadata.ParseFullConfigRepoMetadataStrict(rootPath, fs.LocalProvider())
	if err != nil {
		return err
	}
	for ns, nsMD := range nsNameToNsMDs {
		featureFiles, err := feature.GroupFeatureFiles(filepath.Join(rootPath, ns), nsMD, fs.LocalProvider())
		if err != nil {
			return err
		}
		for range featureFiles {
			// TODO.
		}
	}
	return nil
}
