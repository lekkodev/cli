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

package static

import (
	"context"
	"fmt"
	"io/ioutil"
	"path/filepath"
	"strings"

	"github.com/bazelbuild/buildtools/build"
	butils "github.com/bazelbuild/buildtools/buildifier/utils"
	"github.com/lekkodev/cli/pkg/feature"
	"github.com/lekkodev/cli/pkg/fs"
	"github.com/lekkodev/cli/pkg/metadata"
	"github.com/lekkodev/cli/pkg/star"
	"github.com/pkg/errors"
)

// Parse reads the star file at the given path and performs
// static parsing, converting the feature to our go-native model.
func Parse(root, filename string) error {
	parser := butils.GetParser(star.InputTypeAuto)
	data, err := ioutil.ReadFile(filename)
	if err != nil {
		return errors.Wrap(err, "read file")
	}
	file, err := parser(filename, data)
	if err != nil {
		return errors.Wrap(err, "parse")
	}
	sb := NewStaticBuilder(file)
	f, err := sb.Build()
	if err != nil {
		return err
	}
	f.Key = strings.Split(filepath.Base(filename), ".")[0]
	rootMD, _, err := metadata.ParseFullConfigRepoMetadataStrict(context.Background(), root, fs.LocalProvider())
	if err != nil {
		return errors.Wrap(err, "parse root metadata")
	}
	registry, err := star.BuildDynamicTypeRegistry(rootMD.ProtoDirectory)
	if err != nil {
		return errors.Wrap(err, "build dynamic type registry")
	}
	// Print json to stdout
	f.PrintJSON(registry)
	// Rewrite the bytes to the starfile path, based on the parse AST.
	// This is just an illustration, but in the future we could modify
	// the AST and use the following code to write it out.
	bytes := sb.format()
	if err := ioutil.WriteFile(filename, bytes, 0600); err != nil {
		return errors.Wrap(err, "failed to write file")
	}
	return nil
}

type StaticBuilder struct {
	file *build.File
	f    *feature.Feature
	t    *traverser
}

func NewStaticBuilder(file *build.File) *StaticBuilder {
	b := &StaticBuilder{
		file: file,
		f:    &feature.Feature{},
	}
	b.t = newTraverser(b.file).
		withDefaultFn(b.initFeature).
		withDescriptionFn(b.description).
		withRulesFn(b.parseRules)
	return b
}

func (b *StaticBuilder) Build() (*feature.Feature, error) {
	if err := b.t.traverse(); err != nil {
		return nil, errors.Wrap(err, "traverse")
	}
	return b.f, nil
}

func (b *StaticBuilder) format() []byte {
	return b.t.format()
}

func (b *StaticBuilder) description(v *build.StringExpr) error {
	b.f.Description = v.Value
	return nil
}

func (b *StaticBuilder) parseRules(rules []rule) error {
	for i, r := range rules {
		rule := &feature.Rule{
			Condition: r.conditionV.Value,
		}
		goVal, featureType, err := b.extractFeatureValue(r.v)
		if err != nil {
			return fmt.Errorf("rule #%d: extract value: %w", i, err)
		}
		switch featureType {
		case feature.FeatureTypeBool:
			rule.Value = goVal
		default:
			return fmt.Errorf("rule #%d: unsupported feature type %s", i, featureType)
		}
		b.f.Rules = append(b.f.Rules, rule)
	}
	return nil
}

func (b *StaticBuilder) extractFeatureValue(v build.Expr) (interface{}, feature.FeatureType, error) {
	switch t := v.(type) {
	case *build.Ident:
		switch t.Name {
		case "True":
			return true, feature.FeatureTypeBool, nil
		case "False":
			return false, feature.FeatureTypeBool, nil
		default:
			return nil, "", fmt.Errorf("unsupported identifier name %s", t.Name)
		}
	default:
		return nil, "", fmt.Errorf("unsupported type %T", v)
	}
}

func (b *StaticBuilder) initFeature(v build.Expr) error {
	goVal, featureType, err := b.extractFeatureValue(v)
	if err != nil {
		return err
	}
	switch featureType {
	case feature.FeatureTypeBool:
		boolVal, ok := goVal.(bool)
		if !ok {
			return fmt.Errorf("expected bool, got %T", goVal)
		}
		b.f = feature.NewBoolFeature(boolVal)
	default:
		return fmt.Errorf("unsupported feature type %s", featureType)
	}
	return nil
}
