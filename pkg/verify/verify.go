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

package verify

import (
	"context"
	"fmt"
	"path/filepath"

	"github.com/lekkodev/cli/pkg/encoding"
	"github.com/lekkodev/cli/pkg/feature"
	"github.com/lekkodev/cli/pkg/fs"
	"github.com/lekkodev/cli/pkg/metadata"
	"github.com/lekkodev/cli/pkg/star"
	"github.com/pkg/errors"
)

// Verifies that a configuration from a root is properly formatted.
// TODO: do even more validation including compilation.
func Verify(rootPath string) error {
	rootMD, nsNameToNsMDs, err := metadata.ParseFullConfigRepoMetadataStrict(context.TODO(), rootPath, fs.LocalProvider())
	if err != nil {
		return errors.Wrap(err, "parse full config repo metadata")
	}
	registry, err := star.BuildDynamicTypeRegistryFromFile(filepath.Join(rootPath, rootMD.ProtoDirectory))
	if err != nil {
		return errors.Wrap(err, "failed to build dynamic proto registry")
	}

	for ns, nsMD := range nsNameToNsMDs {
		groupedFeatures, err := feature.GroupFeatureFiles(context.Background(), filepath.Join(rootPath, ns), nsMD, fs.LocalProvider(), true)
		if err != nil {
			return errors.Wrap(err, "group feature files")
		}
		for _, ff := range groupedFeatures {
			if err := ff.Verify(); err != nil {
				return errors.Wrap(err, fmt.Sprintf("verify ns %s", ns))
			}
			if _, err := encoding.ParseFeature(rootPath, ff, nsMD, fs.LocalProvider()); err != nil {
				return errors.Wrap(err, "parse feature")
			}
			// TODO: share this code between verify and compile, could easily diverge.
			if nsMD.Version == metadata.LatestNamespaceVersion {
				// if we compile, then do so.
				compiler := star.NewCompiler(
					registry,
					rootMD.ProtoDirectory,
					filepath.Join(rootPath, ns, ff.StarlarkFileName),
					ff.Name,
				)
				f, err := compiler.Compile()
				if err != nil {
					return errors.Wrap(err, "compile")
				}
				if len(f.UnitTests) > 0 {
					fmt.Printf("running %d unit tests for feature %s/%s: ", len(f.UnitTests), ns, ff.Name)
					if err := f.RunUnitTests(registry); err != nil {
						fmt.Printf("FAIL: %v\n", err)
					} else {
						fmt.Println("PASS")
					}
				}
			}
		}
	}
	// lint protos
	if err := star.Lint(filepath.Join(rootPath, rootMD.ProtoDirectory)); err != nil {
		return errors.Wrap(err, "lint protos")
	}
	return nil
}
