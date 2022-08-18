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
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/lekkodev/cli/pkg/feature"
	"github.com/lekkodev/cli/pkg/fs"
	"github.com/lekkodev/cli/pkg/metadata"
	"github.com/lekkodev/cli/pkg/star"
	"github.com/pkg/errors"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
)

// Compiles each namespace.
// TODO: compilation should not happen destructively (right now compilation will overwrite
// existing compiled output whether or not compilation was successful). Ideally, we write
// compiled output to a tmp location, compare the tmp output and the existing compiled flag
// to make sure the update is backwards compatible and that existing feature flags are not
// renamed, etc. Only then should we replace existing compiled output with new compiled output.
func Compile(rootPath string) error {
	rootMD, nsNameToNsMDs, err := metadata.ParseFullConfigRepoMetadataStrict(context.TODO(), rootPath, fs.LocalProvider())
	if err != nil {
		return err
	}
	registry, err := star.BuildDynamicTypeRegistry(rootMD.ProtoDirectory)
	if err != nil {
		return errors.Wrap(err, "failed to build dynamic proto registry")
	}
	for ns, nsMD := range nsNameToNsMDs {
		if nsMD.Version != "v1beta2" {
			fmt.Printf("Skipping namespace %s since version %s doesn't conform to compilation\n", ns, nsMD.Version)
			continue
		}

		pathToNamespace := filepath.Join(rootPath, ns)
		featureFiles, err := feature.GroupFeatureFiles(context.Background(), pathToNamespace, nsMD, fs.LocalProvider())
		if err != nil {
			return errors.Wrap(err, "group feature files")
		}
		for _, ff := range featureFiles {
			compiler := star.NewCompiler(
				registry,
				rootMD.ProtoDirectory,
				filepath.Join(rootPath, ns, ff.StarlarkFileName),
				ff.Name,
			)
			f, err := compiler.Compile()
			if err != nil {
				return err
			}

			fProto, err := f.ToProto()
			if err != nil {
				return errors.Wrap(err, "feature to proto")
			}

			// Create the json file
			jBytes, err := protojson.MarshalOptions{
				Resolver: registry,
			}.Marshal(fProto)
			if err != nil {
				return errors.Wrap(err, "failed to marshal proto to json")
			}
			indentedJBytes := bytes.NewBuffer(nil)
			// encoding/json provides a deterministic serialization output, ensuring
			// that indentation always uses the same number of characters.
			if err := json.Indent(indentedJBytes, jBytes, "", "  "); err != nil {
				return errors.Wrap(err, "failed to indent json")
			}
			jsonFile := filepath.Join(pathToNamespace, fmt.Sprintf("%s.json", ff.Name))
			if err := os.WriteFile(jsonFile, indentedJBytes.Bytes(), 0600); err != nil {
				return errors.Wrap(err, "failed to write file")
			}
			// Create the proto file
			pBytes, err := proto.MarshalOptions{Deterministic: true}.Marshal(fProto)
			if err != nil {
				return errors.Wrap(err, "failed to marshal to proto")
			}
			protoBinFile := filepath.Join(pathToNamespace, fmt.Sprintf("%s.proto.bin", ff.Name))
			if err := os.WriteFile(protoBinFile, pBytes, 0600); err != nil {
				return errors.Wrap(err, "failed to write file")
			}
		}
	}
	return nil
}
