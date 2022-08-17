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
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/lekkodev/cli/pkg/feature"
	"github.com/lekkodev/cli/pkg/fs"
	"github.com/lekkodev/cli/pkg/metadata"
	"github.com/lekkodev/cli/pkg/star"
	"github.com/pkg/errors"
	"google.golang.org/protobuf/encoding/protojson"
)

// Compiles each namespace.
// TODO: compilation should not happen destructively (right now compilation will overwrite
// existing compiled output whether or not compilation was successful). Ideally, we write
// compiled output to a tmp location, compare the tmp output and the existing compiled flag
// to make sure the update is backwards compatible and that existing feature flags are not
// renamed, etc. Only then should we replace existing compiled output with new compiled output.
func Compile(rootPath string) error {
	rootMD, nsNameToNsMDs, err := metadata.ParseFullConfigRepoMetadataStrict(rootPath, fs.LocalProvider())
	if err != nil {
		return err
	}
	for ns, nsMD := range nsNameToNsMDs {
		pathToNamespace := filepath.Join(rootPath, ns)
		featureFiles, err := feature.GroupFeatureFiles(context.Background(), pathToNamespace, nsMD, fs.LocalProvider())
		if err != nil {
			return err
		}
		for _, ff := range featureFiles {
			result, err := star.Compile(
				rootMD.ProtoDirectory,
				filepath.Join(rootPath, ns, ff.StarlarkFileName),
				ff.Name,
			)
			if err != nil {
				return err
			}

			// Create the json file
			bytes, err := protojson.MarshalOptions{Multiline: true}.Marshal(result)
			if err != nil {
				return errors.Wrap(err, "failed to marshal proto to json")
			}
			jsonFile := filepath.Join(pathToNamespace, fmt.Sprintf("%s.json", ff.Name))
			if err := os.WriteFile(jsonFile, bytes, 0600); err != nil {
				return errors.Wrap(err, "failed to write file")
			}
		}
	}
	return nil
}
