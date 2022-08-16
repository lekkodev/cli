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
	"log"
	"path/filepath"

	"github.com/lekkodev/cli/pkg/feature"
	"github.com/lekkodev/cli/pkg/metadata"
	"github.com/lekkodev/cli/pkg/star"
)

// Compiles each namespace.
func Compile(rootPath string) error {
	_, nsNameToNsMDs, err := metadata.ParseFullConfigRepoMetadataStrict(rootPath, metadata.LocalProvider())
	if err != nil {
		return err
	}
	log.Printf("ns %v\n", nsNameToNsMDs)
	for ns, nsMD := range nsNameToNsMDs {
		featureFiles, err := feature.GroupFeatureFiles(filepath.Join(rootPath, ns), nsMD)
		if err != nil {
			log.Printf("ffiles err %v\n", err)
			return err
		}
		for _, ff := range featureFiles {
			result, err := star.Compile(ff.Name, filepath.Join(rootPath, ns, ff.BuilderFileName))
			if err != nil {
				return err
			}
			log.Printf("Got proto result: \n%s\n", result.String())
		}
	}
	return nil
}
