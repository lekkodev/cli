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

// This package governs the specifics of a feature, like what individual
// files make up a feature.
package feature

import (
	"io/ioutil"
	"strings"

	"github.com/lekkodev/cli/pkg/metadata"
)

// FeatureFile is a parsed feature from an on desk representation.
type FeatureFile struct {
	Name string
	// Filename of the featureName.build.star file.
	BuilderFileName string
	// Filename of the featureName.validate.star file.
	// This is optional.
	ValidatorFileName string
	// Filename of an featureName.proto file.
	// This is optional.
	ProtoFileName string
}

// This groups feature files in a way that is
// governed by the namespace metadata.
func GroupFeatureFiles(pathToNamespace string, nsMD *metadata.NamespaceConfigRepoMetadata) ([]FeatureFile, error) {
	if nsMD.Version == "v1beta2" {
		files, err := ioutil.ReadDir(pathToNamespace)
		if err != nil {
			return nil, err
		}
		var ret []FeatureFile
		for _, file := range files {
			if file.IsDir() {
				continue
			}
			if strings.HasSuffix(file.Name(), ".star") {
				ret = append(ret, FeatureFile{
					Name:            strings.Split(file.Name(), ".")[0],
					BuilderFileName: file.Name(),
				})
			}
		}
		return ret, nil
	}
	return nil, nil
}
