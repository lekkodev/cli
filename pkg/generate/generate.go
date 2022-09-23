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
	"log"
	"path/filepath"

	"github.com/pkg/errors"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoregistry"

	"github.com/lekkodev/cli/pkg/feature"
	"github.com/lekkodev/cli/pkg/fs"
	featurev1beta1 "github.com/lekkodev/cli/pkg/gen/proto/go/lekko/feature/v1beta1"
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
	provider := fs.LocalProvider()
	rootMD, nsNameToNsMDs, err := metadata.ParseFullConfigRepoMetadataStrict(ctx, rootPath, provider)
	if err != nil {
		return err
	}
	registry, err := star.BuildDynamicTypeRegistryFromFile(filepath.Join(rootPath, rootMD.ProtoDirectory))
	if err != nil {
		return errors.Wrap(err, "failed to build dynamic proto registry")
	}
	if namespaceOverride != "" {
		nsMD, ok := nsNameToNsMDs[namespaceOverride]
		if !ok {
			return fmt.Errorf("namespace %s not found", namespaceOverride)
		}
		return compileNamespace(ctx, rootPath, provider, cw, rootMD, registry, nsMD, featureOverride)
	}
	for _, nsMD := range nsNameToNsMDs {
		if err := compileNamespace(ctx, rootPath, provider, cw, rootMD, registry, nsMD, ""); err != nil {
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
		fs.LocalProvider(),
	)
	if err != nil {
		return errors.Wrap(err, "group feature files")
	}
	if featureOverride != "" {
		for _, ff := range featureFiles {
			if ff.Name == featureOverride {
				return compileFeature(ctx, rootPath, provider, cw, rootMD, registry, nsMD, pathToNamespace, ff)
			}
		}
		return fmt.Errorf("feature %s not found", featureOverride)
	}
	for _, ff := range featureFiles {
		if err := compileFeature(ctx, rootPath, provider, cw, rootMD, registry, nsMD, pathToNamespace, ff); err != nil {
			return err
		}
	}
	return nil
}

func compileFeature(
	ctx context.Context,
	rootPath string,
	provider fs.Provider,
	cw fs.ConfigWriter,
	rootMD *metadata.RootConfigRepoMetadata,
	registry *protoregistry.Types,
	nsMD *metadata.NamespaceConfigRepoMetadata,
	pathToNamespace string,
	ff feature.FeatureFile,
) error {
	compiler := star.NewCompiler(
		registry,
		rootMD.ProtoDirectory,
		filepath.Join(rootPath, nsMD.Name, ff.StarlarkFileName),
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
	protoGenPath, jsonGenPath := pathToNamespace, pathToNamespace
	if nsMD.Version == metadata.LatestNamespaceVersion {
		jsonGenPath = filepath.Join(pathToNamespace, metadata.GenFolderPathJSON)
		protoGenPath = filepath.Join(pathToNamespace, metadata.GenFolderPathProto)
	}
	protoBinFile := filepath.Join(protoGenPath, fmt.Sprintf("%s.proto.bin", ff.Name))
	diffExists, err := compareExistingProto(ctx, protoBinFile, fProto, provider)
	if err != nil {
		return errors.Wrap(err, "comparing with existing proto")
	}
	if !diffExists {
		// skipping i/o as no diff exists
		return nil
	}

	// Create the json file
	jBytes, err := feature.ProtoToJSON(fProto, registry)
	if err != nil {
		return errors.Wrap(err, "proto to json")
	}
	jsonFile := filepath.Join(jsonGenPath, fmt.Sprintf("%s.json", ff.Name))
	if err := cw.MkdirAll(jsonGenPath, 0775); err != nil {
		return errors.Wrap(err, "failed to make gen json directory")
	}
	if err := cw.WriteFile(jsonFile, jBytes, 0600); err != nil {
		return errors.Wrap(err, "failed to write file")
	}
	// Create the proto file
	pBytes, err := proto.MarshalOptions{Deterministic: true}.Marshal(fProto)
	if err != nil {
		return errors.Wrap(err, "failed to marshal to proto")
	}
	if err := cw.MkdirAll(protoGenPath, 0775); err != nil {
		return errors.Wrap(err, "failed to make gen proto directory")
	}
	if err := cw.WriteFile(protoBinFile, pBytes, 0600); err != nil {
		return errors.Wrap(err, "failed to write file")
	}
	log.Printf("Generated diff for %s/%s\n", nsMD.Name, ff.Name)
	return nil
}

// returns true if there is an actual semantic difference between the existing compiled proto,
// and the new proto we have on hand.
func compareExistingProto(ctx context.Context, existingProtoFilePath string, newProto *featurev1beta1.Feature, provider fs.Provider) (bool, error) {
	bytes, err := provider.GetFileContents(ctx, existingProtoFilePath)
	if err != nil {
		if provider.IsNotExist(err) {
			return true, nil
		}
		return false, errors.Wrap(err, "read existing proto file")
	}
	existingProto := &featurev1beta1.Feature{}
	if err := proto.Unmarshal(bytes, existingProto); err != nil {
		return false, errors.Wrap(err, fmt.Sprintf("failed to unmarshal existing proto at path %s", existingProtoFilePath))
	}
	if existingProto.GetKey() != newProto.GetKey() {
		return false, fmt.Errorf("cannot change key of feature flag: old %s, new %s", existingProto.GetKey(), newProto.GetKey())
	}
	if existingProto.GetTree().GetDefault().GetTypeUrl() != newProto.GetTree().GetDefault().GetTypeUrl() {
		return false, fmt.Errorf(
			"cannot change feature flag type: old %s, new %s",
			existingProto.GetTree().GetDefault().GetTypeUrl(),
			newProto.GetTree().GetDefault().GetTypeUrl(),
		)
	}
	return !proto.Equal(existingProto, newProto), nil
}
