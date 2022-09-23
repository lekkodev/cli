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
	"path/filepath"

	"github.com/pkg/errors"
	"google.golang.org/protobuf/reflect/protoregistry"

	"github.com/lekkodev/cli/pkg/feature"
	"github.com/lekkodev/cli/pkg/fs"
	"github.com/lekkodev/cli/pkg/metadata"
	"github.com/lekkodev/cli/pkg/star"
	"github.com/lekkodev/cli/pkg/verify"
)

// Compiles each namespace.
// TODO: compilation should not happen destructively (right now compilation will overwrite
// existing compiled output whether or not compilation was successful). Ideally, we write
// compiled output to a tmp location, compare the tmp output and the existing compiled flag
// to make sure the update is backwards compatible and that existing feature flags are not
// renamed, etc. Only then should we replace existing compiled output with new compiled output.
func Compile(rootPath, namespaceOverride, featureOverride string) error {
	ctx := context.TODO()
	cw := fs.LocalConfigWriter()
	rootMD, nsNameToNsMDs, err := metadata.ParseFullConfigRepoMetadataStrict(ctx, rootPath, cw)
	if err != nil {
		return err
	}
	registry, err := star.ReBuildDynamicTypeRegistry(filepath.Join(rootPath, rootMD.ProtoDirectory), cw)
	if err != nil {
		return errors.Wrap(err, "failed to build dynamic proto registry")
	}
	if namespaceOverride != "" {
		nsMD, ok := nsNameToNsMDs[namespaceOverride]
		if !ok {
			return fmt.Errorf("namespace %s not found", namespaceOverride)
		}
		return compileNamespace(ctx, rootPath, cw, cw, rootMD, registry, nsMD, featureOverride)
	}
	for _, nsMD := range nsNameToNsMDs {
		if err := compileNamespace(ctx, rootPath, cw, cw, rootMD, registry, nsMD, ""); err != nil {
			return err
		}
	}
	// Finally, run a sanity check to make sure we compiled everything correctly
	if err := verify.Verify(rootPath); err != nil {
		return errors.Wrap(err, "internal compilation error")
	}
	return nil
}

func compileNamespace(
	ctx context.Context,
	rootPath string,
	provider fs.Provider,
	cw fs.ConfigWriter,
	rootMD *metadata.RootConfigRepoMetadata,
	registry *protoregistry.Types,
	nsMD *metadata.NamespaceConfigRepoMetadata,
	featureOverride string,
) error {
	if _, ok := map[string]struct{}{"v1beta3": {}}[nsMD.Version]; !ok {
		fmt.Printf("Skipping namespace %s since version %s doesn't conform to compilation\n", nsMD.Name, nsMD.Version)
		return nil
	}

	pathToNamespace := filepath.Join(rootPath, nsMD.Name)
	featureFiles, err := feature.GroupFeatureFiles(
		ctx,
		pathToNamespace,
		cw,
	)
	if err != nil {
		return errors.Wrap(err, "group feature files")
	}
	if featureOverride != "" {
		for _, ff := range featureFiles {
			ff := ff
			if ff.Name == featureOverride {
				return compileFeature(ctx, rootPath, cw, rootMD, registry, nsMD, pathToNamespace, &ff)
			}
		}
		return fmt.Errorf("feature %s not found", featureOverride)
	}
	for _, ff := range featureFiles {
		ff := ff
		if err := compileFeature(ctx, rootPath, cw, rootMD, registry, nsMD, pathToNamespace, &ff); err != nil {
			return err
		}
	}
	return nil
}

func compileFeature(
	ctx context.Context,
	rootPath string,
	cw fs.ConfigWriter,
	rootMD *metadata.RootConfigRepoMetadata,
	registry *protoregistry.Types,
	nsMD *metadata.NamespaceConfigRepoMetadata,
	pathToNamespace string,
	ff *feature.FeatureFile,
) error {
	compiler := star.NewCompiler(registry, ff, cw)
	f, err := compiler.Compile()
	if err != nil {
		return errors.Wrap(err, "compile")
	}
	if err := compiler.Persist(f); err != nil {
		return errors.Wrap(err, "persist")
	}
	return nil
}
