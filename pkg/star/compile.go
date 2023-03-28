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

package star

import (
	"context"
	"fmt"
	"path/filepath"

	"github.com/lekkodev/cli/pkg/feature"
	"github.com/lekkodev/cli/pkg/fs"
	featurev1beta1 "github.com/lekkodev/cli/pkg/gen/proto/go/lekko/feature/v1beta1"
	"github.com/lekkodev/cli/pkg/metadata"
	"github.com/pkg/errors"
	"github.com/stripe/skycfg/go/protomodule"
	"go.starlark.net/starlark"
	"go.starlark.net/starlarkstruct"
	"go.starlark.net/starlarktest"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoregistry"
)

type Compiler interface {
	Compile(context.Context, feature.NamespaceVersion) (*feature.CompiledFeature, error)
	Persist(ctx context.Context, f *feature.Feature, nv feature.NamespaceVersion, ignoreBackwardsCompatibility, dryRun bool) (persisted bool, diffExists bool, err error)
}

type compiler struct {
	registry *protoregistry.Types
	ff       *feature.FeatureFile
	cw       fs.ConfigWriter
}

// Compile takes the following parameters:
// protoDir: path to the proto directory that stores all user-defined proto files
// starfilePath: path to the .star file that defines this feature flag
// featureName: human-readable name of this feature flag. Also matches the starfile name.
func NewCompiler(registry *protoregistry.Types, ff *feature.FeatureFile, cw fs.ConfigWriter) Compiler {
	return &compiler{
		registry: registry,
		ff:       ff,
		cw:       cw,
	}
}

func (c *compiler) Compile(ctx context.Context, nv feature.NamespaceVersion) (*feature.CompiledFeature, error) {
	// Execute the starlark file to retrieve its contents (globals)
	thread := &starlark.Thread{
		Name: "compile",
	}
	moduleSource, err := c.cw.GetFileContents(ctx, c.ff.RootPath(c.ff.StarlarkFileName))
	if err != nil {
		return nil, errors.Wrap(err, "read starfile")
	}
	protoModule := protomodule.NewModule(c.registry)
	assertModule, err := newAssertModule()
	if err != nil {
		return nil, errors.Wrap(err, "new assert module")
	}
	globals, err := starlark.ExecFile(thread, c.ff.RootPath(c.ff.StarlarkFileName), moduleSource, starlark.StringDict{
		"proto":   protoModule,
		"assert":  assertModule,
		"feature": starlark.NewBuiltin("feature", makeFeature),
	})
	if err != nil {
		return nil, errors.Wrap(err, "starlark execfile")
	}
	cf, err := newFeatureBuilder(c.ff.Name, globals, nv).Build()
	if err != nil {
		return nil, errors.Wrap(err, "build")
	}
	return cf, nil
}

func (c *compiler) Persist(ctx context.Context, f *feature.Feature, nv feature.NamespaceVersion, ignoreBackwardsCompatibility, dryRun bool) (bool, bool, error) {
	fProto, err := f.ToProto()
	if err != nil {
		return false, false, errors.Wrap(err, "feature to proto")
	}
	jsonGenPath := filepath.Join(c.ff.NamespaceName, metadata.GenFolderPathJSON)
	protoGenPath := filepath.Join(c.ff.NamespaceName, metadata.GenFolderPathProto)
	protoBinFile := filepath.Join(protoGenPath, fmt.Sprintf("%s.proto.bin", c.ff.Name))
	diffExists, err := compareExistingProto(ctx, protoBinFile, fProto, c.cw)
	if err != nil && !ignoreBackwardsCompatibility { // exit due to backwards incompatibility
		return false, false, errors.Wrap(err, "comparing with existing proto")
	}
	if !diffExists || dryRun {
		// skipping i/o
		return false, diffExists, nil
	}

	// Create the json file
	jBytes, err := feature.ProtoToJSON(fProto, c.registry)
	if err != nil {
		return false, diffExists, errors.Wrap(err, "proto to json")
	}
	jsonFile := filepath.Join(jsonGenPath, fmt.Sprintf("%s.json", c.ff.Name))
	if err := c.cw.MkdirAll(jsonGenPath, 0775); err != nil {
		return false, diffExists, errors.Wrap(err, "failed to make gen json directory")
	}
	if err := c.cw.WriteFile(jsonFile, jBytes, 0600); err != nil {
		return false, diffExists, errors.Wrap(err, "failed to write file")
	}
	// Create the proto file
	pBytes, err := proto.MarshalOptions{Deterministic: true}.Marshal(fProto)
	if err != nil {
		return false, diffExists, errors.Wrap(err, "failed to marshal to proto")
	}
	if err := c.cw.MkdirAll(protoGenPath, 0775); err != nil {
		return false, diffExists, errors.Wrap(err, "failed to make gen proto directory")
	}
	if err := c.cw.WriteFile(protoBinFile, pBytes, 0600); err != nil {
		return false, diffExists, errors.Wrap(err, "failed to write file")
	}
	return true, diffExists, nil
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

func load(thread *starlark.Thread, module string) (starlark.StringDict, error) {
	if module == "assert.star" {
		return starlarktest.LoadAssertModule()
	}
	return nil, fmt.Errorf("load not implemented for %s", module)
}

func newAssertModule() (*starlarkstruct.Module, error) {
	sd, err := starlarktest.LoadAssertModule()
	if err != nil {
		return nil, errors.Wrap(err, "load assert module")
	}
	assertVal, ok := sd["assert"]
	if !ok {
		return nil, fmt.Errorf("could not find assert value in keys %v", sd.Keys())
	}
	assertModule, ok := assertVal.(*starlarkstruct.Module)
	if !ok {
		return nil, fmt.Errorf("assertVal incorrect type %T", assertVal)
	}
	return assertModule, nil
}
