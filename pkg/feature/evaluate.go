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
	"context"
	"fmt"
	"strings"

	"google.golang.org/protobuf/types/known/anypb"
	"google.golang.org/protobuf/types/known/structpb"
	"google.golang.org/protobuf/types/known/wrapperspb"

	"github.com/lekkodev/cli/pkg/fs"
	featurev1beta1 "github.com/lekkodev/cli/pkg/gen/proto/go/lekko/feature/v1beta1"
	rulesv1beta1 "github.com/lekkodev/cli/pkg/gen/proto/go/lekko/rules/v1beta1"
	"github.com/lekkodev/cli/pkg/metadata"
	"github.com/lekkodev/cli/pkg/rules"
	"github.com/pkg/errors"
)

type EvaluableFeature interface {
	// Evaluate Feature returns a protobuf.Any.
	// For user defined protos, we shouldn't attempt to Unmarshal
	// this unless we know the type. For primitive types, we can
	// safely unmarshal into BoolValue, StringValue, etc.
	Evaluate(evalContext map[string]interface{}) (*anypb.Any, error)
}

func NewV1Beta1(f *rulesv1beta1.Feature) EvaluableFeature {
	return &v1beta1{f}
}

type v1beta1 struct {
	*rulesv1beta1.Feature
}

func (v1beta1 *v1beta1) Evaluate(evalContext map[string]interface{}) (*anypb.Any, error) {
	ctxMap, err := rules.ContextHelper(evalContext)
	if err != nil {
		return nil, err
	}
	protobufVal, err := rules.EvaluateFeatureV1Beta1(v1beta1.Feature, ctxMap)
	if err != nil {
		return nil, err
	}
	switch v := protobufVal.GetKind().(type) {
	case *structpb.Value_NumberValue:
		return anypb.New(&wrapperspb.DoubleValue{Value: v.NumberValue})
	case *structpb.Value_StringValue:
		return anypb.New(&wrapperspb.StringValue{Value: v.StringValue})
	case *structpb.Value_BoolValue:
		return anypb.New(&wrapperspb.BoolValue{Value: v.BoolValue})
	case *structpb.Value_ListValue:
		return nil, fmt.Errorf("invalid list value: %v", v)
	case *structpb.Value_StructValue:
		// If we ever wanted to support complex types in v1beta1 we would need to change this.
		return nil, fmt.Errorf("invalid struct value: %v", v)
	default:
		return nil, fmt.Errorf("invalid unknown type: %v", v)
	}
}

type v1beta2 struct {
	*featurev1beta1.Feature
}

func NewV1Beta2(f *featurev1beta1.Feature) EvaluableFeature {
	return &v1beta2{f}
}

// TODO: pre-compute the ruleslang tree so that we:
// 1) error on verify time if things aren't valid.
// 2) pre-compute antlr trees.
func (v1beta2 *v1beta2) Evaluate(evalContext map[string]interface{}) (*anypb.Any, error) {
	return rules.EvaluateFeatureV1Beta2(v1beta2.Tree, evalContext)
}

// FeatureFile is a parsed feature from an on desk representation.
// This is intended to remain stable across feature versions.
// For now, v1beta1 just has a CompiledJSONFileName and
// v1beta2 has other files.
type FeatureFile struct {
	Name string
	// Filename of the featureName.star file.
	StarlarkFileName string
	// Filename of an featureName.proto file.
	// This is optional.
	ProtoFileName string
	// Filename of a compiled .json file.
	CompiledJSONFileName string
	// Filename of a compiled .proto.bin file.
	CompiledProtoBinFileName string
}

// This groups feature files in a way that is
// governed by the namespace metadata.
// TODO naming conventions.
func GroupFeatureFiles(
	ctx context.Context,
	pathToNamespace string,
	nsMD *metadata.NamespaceConfigRepoMetadata,
	fsProvider fs.Provider,
	validate bool,
) ([]FeatureFile, error) {
	featureToFile := make(map[string]FeatureFile)
	files, err := fsProvider.GetDirContents(ctx, pathToNamespace)
	if err != nil {
		return nil, err
	}

	for _, file := range files {
		if strings.HasSuffix(file.Name, ".json") {
			featureName := strings.TrimSuffix(file.Name, ".json")
			f, ok := featureToFile[featureName]
			if !ok {
				featureToFile[featureName] = FeatureFile{Name: featureName, CompiledJSONFileName: file.Name}
			} else {
				f.CompiledJSONFileName = file.Name
				featureToFile[featureName] = f
			}
		}
		if strings.HasSuffix(file.Name, ".star") {
			featureName := strings.TrimSuffix(file.Name, ".star")
			f, ok := featureToFile[featureName]
			if !ok {
				featureToFile[featureName] = FeatureFile{Name: featureName, StarlarkFileName: file.Name}
			} else {
				f.StarlarkFileName = file.Name
				featureToFile[featureName] = f
			}
		}
		if strings.HasSuffix(file.Name, ".proto") {
			featureName := strings.TrimSuffix(file.Name, ".proto")
			f, ok := featureToFile[featureName]
			if !ok {
				featureToFile[featureName] = FeatureFile{Name: featureName, ProtoFileName: file.Name}
			} else {
				f.ProtoFileName = file.Name
				featureToFile[featureName] = f
			}
		}
		if strings.HasSuffix(file.Name, ".proto.bin") {
			featureName := strings.TrimSuffix(file.Name, ".proto.bin")
			f, ok := featureToFile[featureName]
			if !ok {
				featureToFile[featureName] = FeatureFile{Name: featureName, CompiledProtoBinFileName: file.Name}
			} else {
				f.CompiledProtoBinFileName = file.Name
				featureToFile[featureName] = f
			}
		}
	}

	featureFiles := make([]FeatureFile, len(featureToFile))
	// Compliance checks for each version.
	i := 0
	for _, feature := range featureToFile {
		featureFiles[i] = feature
		i = i + 1
		if validate {
			if err := ComplianceCheck(feature, nsMD); err != nil {
				return nil, errors.Wrap(err, "feature file compliance check")
			}
		}
	}
	return featureFiles, nil
}

func ComplianceCheck(f FeatureFile, nsMD *metadata.NamespaceConfigRepoMetadata) error {
	switch nsMD.Version {
	case "v1beta1":
		if len(f.CompiledJSONFileName) == 0 {
			return fmt.Errorf("empty compiled JSON for feature: %s", f.Name)
		}
	case "v1beta2":
		if len(f.CompiledJSONFileName) == 0 {
			return fmt.Errorf("empty compiled JSON for feature: %s", f.Name)
		}
		if len(f.CompiledProtoBinFileName) == 0 {
			return fmt.Errorf("empty compiled proto for feature: %s", f.Name)
		}
		if len(f.StarlarkFileName) == 0 {
			return fmt.Errorf("empty starlark file for feature: %s", f.Name)
		}
	}
	return nil
}
