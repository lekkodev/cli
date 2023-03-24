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
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoregistry"
	"google.golang.org/protobuf/types/known/anypb"
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

func (c *compiler) fieldsModule(ctx context.Context, tmpl *featurev1beta1.FeatureTemplate) (*starlarkstruct.Module, error) {
	msg, err := anypb.UnmarshalNew(tmpl.Tree.Value, proto.UnmarshalOptions{
		Resolver: c.registry,
	})
	if err != nil {
		return nil, errors.Wrap(err, "anypb unmarshal")
	}
	starProtoValue, err := protomodule.NewMessage(msg)
	if err != nil {
		return nil, errors.Wrap(err, "protomodule new msg")
	}

	// members := make(map[string]starlark.Value)
	// for key, anyField := range tmpl.Tree.Fields {
	// 	msg, err := anypb.UnmarshalNew(anyField, proto.UnmarshalOptions{
	// 		Resolver: c.registry,
	// 	})
	// 	if err != nil {
	// 		return nil, errors.Wrap(err, "anypb unmarshal")
	// 	}

	// 	bv, ok := msg.(*wrapperspb.BoolValue)

	// 	b := wrapperspb.BoolValue{}
	// 	if err := anyField.UnmarshalTo(&b); err == nil {
	// 		fmt.Println("is bool val, ", ok, bv)
	// 		members[key] = starlark.Bool(b.Value)
	// 		continue
	// 	}

	// 	starProto, err := protomodule.NewMessage(msg)
	// 	if err != nil {
	// 		return nil, err
	// 	}
	// 	members[key] = starProto
	// }
	// sv := wrapperspb.String("blah")
	// msg, err := protomodule.NewMessage(sv)
	// if err != nil {
	// 	log.Fatal(err)
	// }

	return &starlarkstruct.Module{
		Name: "template",
		Members: map[string]starlark.Value{
			"mask": starProtoValue,
		},
		// Members: map[string]starlark.Value{
		// 	"deploy_everywhere": starlark.True,
		// 	"pods":              starlark.MakeInt(4),
		// 	// "env":               msg,
		// 	"str": msg,
		// },
	}, nil
}

func (c *compiler) Compile(ctx context.Context, nv feature.NamespaceVersion) (*feature.CompiledFeature, error) {
	// if nv == feature.NamespaceVersionV2 {
	// 	tmpl, err := c.featureTemplate(ctx)
	// 	if err != nil {
	// 		return nil, errors.Wrap(err, "feature template")
	// 	}
	// 	f, err := templateToFeature(tmpl)
	// 	if err != nil {
	// 		return nil, errors.Wrap(err, "template to feature")
	// 	}
	// 	return &feature.CompiledFeature{
	// 		Feature:          f,
	// 		TestResults:      nil,
	// 		ValidatorResults: nil,
	// 	}, nil
	// }
	// Execute the starlark file to retrieve its contents (globals)

	// testing merge behavior
	bf, bt := false, true
	bva := &featurev1beta1.TestOptional{
		A: bf,
		B: &bt,
	}
	bvb := &featurev1beta1.TestOptional{
		A: bt,
		B: &bf,
	}
	proto.Merge(bva, bvb)
	fmt.Println(bva.String())

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
	tmplBytes, err := c.cw.GetFileContents(ctx, c.ff.RootPath(fmt.Sprintf("%s.json", c.ff.Name)))
	if err != nil {
		return nil, errors.Wrap(err, "read json file")
	}
	tmpl := &featurev1beta1.FeatureTemplate{}
	err = protojson.UnmarshalOptions{Resolver: c.registry}.Unmarshal(tmplBytes, tmpl)
	if err != nil {
		return nil, errors.Wrap(err, "protojson unmarshal")
	}
	fieldsMod, err := c.fieldsModule(ctx, tmpl)
	if err != nil {
		return nil, errors.Wrap(err, "fields mod")
	}
	globals, err := starlark.ExecFile(thread, c.ff.RootPath(c.ff.StarlarkFileName), moduleSource, starlark.StringDict{
		"proto":    protoModule,
		"assert":   assertModule,
		"feature":  starlark.NewBuiltin("feature", makeFeature),
		"template": fieldsMod,
	})
	// fmt.Println("Globals", globals["value"])
	if err != nil {
		return nil, errors.Wrap(err, "starlark execfile")
	}
	fmt.Println("globals value", globals["value"])
	message, ok := protomodule.AsProtoMessage(globals["value"])
	if ok {
		fmt.Printf("Proto value: %v\n", message)

		any, err := feature.ValToAny(message, feature.FeatureTypeProto)
		if err != nil {
			fmt.Printf("err: %v\n", err)
		} else {
			fmt.Printf("ANy: %v\n", any.String())
		}
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

func (c *compiler) featureTemplate(ctx context.Context) (*featurev1beta1.FeatureTemplate, error) {
	tmpl := &featurev1beta1.FeatureTemplate{}
	bytes, err := c.cw.GetFileContents(ctx, c.ff.TemplateFileName)
	if err != nil {
		return nil, errors.Wrap(err, "get template file contents")
	}
	err = protojson.UnmarshalOptions{}.Unmarshal(bytes, tmpl)
	if err != nil {
		return nil, errors.Wrap(err, "protojson unmarshal")
	}
	return tmpl, nil
}

func templateToFeature(tmpl *featurev1beta1.FeatureTemplate) (*feature.Feature, error) {
	ret := &feature.Feature{
		Key:         tmpl.Key,
		Description: tmpl.Description,
		Value:       nil,
		FeatureType: "",
		Rules:       []*feature.Rule{},
		UnitTests:   []*feature.UnitTest{},
	}
	return ret, nil
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
