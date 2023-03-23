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
	"path/filepath"
	"strings"

	"google.golang.org/protobuf/types/known/anypb"

	"github.com/lekkodev/cli/pkg/fs"
	featurev1beta1 "github.com/lekkodev/cli/pkg/gen/proto/go/lekko/feature/v1beta1"
	rulesv1beta2 "github.com/lekkodev/cli/pkg/gen/proto/go/lekko/rules/v1beta2"
	rulesv1beta3 "github.com/lekkodev/cli/pkg/gen/proto/go/lekko/rules/v1beta3"
	"github.com/lekkodev/cli/pkg/metadata"
	"github.com/lekkodev/cli/pkg/rules"
	"github.com/pkg/errors"
)

type EvaluableFeature interface {
	// Evaluate Feature returns a protobuf.Any.
	// For user defined protos, we shouldn't attempt to Unmarshal
	// this unless we know the type. For primitive types, we can
	// safely unmarshal into BoolValue, StringValue, etc.
	Evaluate(evalContext map[string]interface{}) (*anypb.Any, ResultPath, error)
	// Returns the feature type (bool, string, json, proto, etc)
	// or "" if the type is not supported
	Type() FeatureType
}

// Stores the path of the tree node that returned the final value
// after successful evaluation. See the readme for an illustration.
type ResultPath []int

type v1beta3 struct {
	*featurev1beta1.Feature
}

func NewV1Beta3(f *featurev1beta1.Feature) EvaluableFeature {
	return &v1beta3{f}
}

func (v1b3 *v1beta3) Type() FeatureType {
	return FeatureTypeFromProto(v1b3.GetType())
}

func (v1b3 *v1beta3) Evaluate(evalContext map[string]interface{}) (*anypb.Any, ResultPath, error) {
	return v1b3.evaluate(evalContext)
}

func (v1b3 *v1beta3) evaluate(context map[string]interface{}) (*anypb.Any, []int, error) {
	for i, constraint := range v1b3.GetTree().GetConstraints() {
		childVal, childPasses, childPath, err := v1b3.traverse(constraint, context)
		if err != nil {
			return nil, []int{}, err
		}
		if childPasses {
			if childVal != nil {
				return childVal, append([]int{i}, childPath...), nil
			}
			break
		}
	}
	return v1b3.GetTree().Default, []int{}, nil
}

func (v1b3 *v1beta3) traverse(constraint *featurev1beta1.Constraint, context map[string]interface{}) (*anypb.Any, bool, []int, error) {
	passes, err := v1b3.evaluateRule(constraint.GetRuleAst(), constraint.GetRuleAstNew(), context)
	if err != nil {
		return nil, false, []int{}, errors.Wrap(err, "processing")
	}
	if !passes {
		// If the rule fails, we avoid further traversal
		return nil, passes, []int{}, nil
	}
	// rule passed
	retVal := constraint.Value // may be null
	for i, child := range constraint.GetConstraints() {
		childVal, childPasses, childPath, err := v1b3.traverse(child, context)
		if err != nil {
			return nil, false, []int{}, errors.Wrapf(err, "traverse %d", i)
		}
		if childPasses {
			// We may stop iterating. But first, remember the traversed
			// value if it exists
			if childVal != nil {
				return childVal, passes, append([]int{i}, childPath...), nil
			}
			break
		}
		// Child evaluation did not pass, continue iterating
	}
	return retVal, passes, []int{}, nil
}

func (v1b3 *v1beta3) evaluateRule(rule *rulesv1beta2.Rule, ruleV3 *rulesv1beta3.Rule, context map[string]interface{}) (bool, error) {
	// evaluate using the new rule AST if it exists.
	if ruleV3 != nil {
		passes, err := rules.NewV1Beta3(ruleV3).EvaluateRule(context)
		if err != nil {
			return false, errors.Wrap(err, "evaluating rule v3")
		}
		return passes, nil
	}
	if rule == nil {
		// empty rule evaluates to 'true'
		return true, nil
	}
	passes, err := rules.NewV1Beta2(rule).EvaluateRule(context)
	if err != nil {
		return false, errors.Wrap(err, "evaluating rule")
	}
	return passes, nil
}

// FeatureFile is a parsed feature from an on desk representation.
// This is intended to remain stable across feature versions.
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
	// name of the namespace directory
	NamespaceName string
}

type FeatureContents struct {
	File *FeatureFile

	Star  []byte
	JSON  []byte
	Proto []byte
	SHA   string
}

func (ff FeatureFile) Verify() error {
	if ff.Name == "" {
		return fmt.Errorf("feature file has no name")
	}
	if ff.StarlarkFileName == "" {
		return fmt.Errorf("feature file %s has no .star file", ff.Name)
	}
	if ff.CompiledJSONFileName == "" {
		return fmt.Errorf("feature file %s has no .json file", ff.Name)
	}
	if ff.CompiledProtoBinFileName == "" {
		return fmt.Errorf("feature file %s has no .proto.bin file", ff.Name)
	}
	return nil
}

func (ff FeatureFile) RootPath(filename string) string {
	return filepath.Join(ff.NamespaceName, filename)
}

func NewFeatureFile(nsName, featureName string) FeatureFile {
	return FeatureFile{
		Name:                     featureName,
		NamespaceName:            nsName,
		StarlarkFileName:         fmt.Sprintf("%s.star", featureName),
		CompiledJSONFileName:     filepath.Join(metadata.GenFolderPathJSON, fmt.Sprintf("%s.json", featureName)),
		CompiledProtoBinFileName: filepath.Join(metadata.GenFolderPathProto, fmt.Sprintf("%s.proto.bin", featureName)),
	}
}

func walkNamespace(ctx context.Context, nsName, path, nsRelativePath string, featureToFile map[string]FeatureFile, fsProvider fs.Provider) error {
	files, err := fsProvider.GetDirContents(ctx, path)
	if err != nil {
		return errors.Wrap(err, fmt.Sprintf("get dir contents for %s", path))
	}
	for _, file := range files {
		if strings.HasSuffix(file.Name, ".json") {
			featureName := strings.TrimSuffix(file.Name, ".json")
			f, ok := featureToFile[featureName]
			if !ok {
				featureToFile[featureName] = FeatureFile{Name: featureName, CompiledJSONFileName: filepath.Join(nsRelativePath, file.Name), NamespaceName: nsName}
			} else {
				f.CompiledJSONFileName = filepath.Join(nsRelativePath, file.Name)
				featureToFile[featureName] = f
			}
		} else if strings.HasSuffix(file.Name, ".star") {
			featureName := strings.TrimSuffix(file.Name, ".star")
			f, ok := featureToFile[featureName]
			if !ok {
				featureToFile[featureName] = FeatureFile{Name: featureName, StarlarkFileName: filepath.Join(nsRelativePath, file.Name), NamespaceName: nsName}
			} else {
				f.StarlarkFileName = filepath.Join(nsRelativePath, file.Name)
				featureToFile[featureName] = f
			}
		} else if strings.HasSuffix(file.Name, ".proto") {
			featureName := strings.TrimSuffix(file.Name, ".proto")
			f, ok := featureToFile[featureName]
			if !ok {
				featureToFile[featureName] = FeatureFile{Name: featureName, ProtoFileName: filepath.Join(nsRelativePath, file.Name), NamespaceName: nsName}
			} else {
				f.ProtoFileName = filepath.Join(nsRelativePath, file.Name)
				featureToFile[featureName] = f
			}
		} else if strings.HasSuffix(file.Name, ".proto.bin") {
			featureName := strings.TrimSuffix(file.Name, ".proto.bin")
			f, ok := featureToFile[featureName]
			if !ok {
				featureToFile[featureName] = FeatureFile{Name: featureName, CompiledProtoBinFileName: filepath.Join(nsRelativePath, file.Name), NamespaceName: nsName}
			} else {
				f.CompiledProtoBinFileName = filepath.Join(nsRelativePath, file.Name)
				featureToFile[featureName] = f
			}
		} else if file.IsDir {
			if err := walkNamespace(ctx, nsName, file.Path, filepath.Join(nsRelativePath, file.Name), featureToFile, fsProvider); err != nil {
				return errors.Wrap(err, "walkNamespace")
			}
		}
	}
	return nil
}

// This groups feature files in a way that is
// governed by the namespace metadata.
// TODO naming conventions.
func GroupFeatureFiles(
	ctx context.Context,
	pathToNamespace string,
	fsProvider fs.Provider,
) ([]FeatureFile, error) {
	featureToFile := make(map[string]FeatureFile)
	if err := walkNamespace(ctx, filepath.Base(pathToNamespace), pathToNamespace, "", featureToFile, fsProvider); err != nil {
		return nil, errors.Wrap(err, "walk namespace")
	}
	featureFiles := make([]FeatureFile, len(featureToFile))
	i := 0
	for _, feature := range featureToFile {
		featureFiles[i] = feature
		i = i + 1
	}
	return featureFiles, nil
}

func ComplianceCheck(f FeatureFile, nsMD *metadata.NamespaceConfigRepoMetadata) error {
	switch nsMD.Version {
	case "v1beta5":
		fallthrough
	case "v1beta4":
		fallthrough
	case "v1beta3":
		if len(f.CompiledJSONFileName) == 0 {
			return fmt.Errorf("empty compiled JSON for feature: %s", f.Name)
		}
		if len(f.CompiledProtoBinFileName) == 0 {
			return fmt.Errorf("empty compiled proto for feature: %s", f.Name)
		}
		if len(f.StarlarkFileName) == 0 {
			return fmt.Errorf("empty starlark file for feature: %s", f.Name)
		}
	default:
		return fmt.Errorf("unknown namespace version %s", nsMD.Version)
	}
	return nil
}

func ParseFeaturePath(featurePath string) (namespaceName string, featureName string, err error) {
	splits := strings.SplitN(featurePath, "/", 2)
	if len(splits) == 1 {
		return splits[0], "", nil
	}
	if len(splits) == 2 {
		return splits[0], splits[1], nil
	}
	return "", "", fmt.Errorf("invalid featurepath: %s, should be of format namespace[/feature]", featurePath)
}
