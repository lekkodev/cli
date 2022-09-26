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
	"fmt"

	"github.com/bazelbuild/buildtools/build"
	butils "github.com/bazelbuild/buildtools/buildifier/utils"
	"github.com/lekkodev/cli/pkg/feature"
	"github.com/lekkodev/cli/pkg/star"
	"github.com/pkg/errors"
)

// Walker provides methods to statically read and manipulate .star files
// that hold lekko features.
type Walker interface {
	Build() (*feature.Feature, error)
	Mutate(f *feature.Feature) ([]byte, error)
}

type walker struct {
	filename  string
	starBytes []byte
}

func NewWalker(filename string, starBytes []byte) Walker {
	return &walker{
		filename:  filename,
		starBytes: starBytes,
	}
}

func (w *walker) Build() (*feature.Feature, error) {
	ast, err := w.genAST()
	if err != nil {
		return nil, errors.Wrap(err, "gen ast")
	}
	ret := &feature.Feature{}
	t := newTraverser(ast).
		withDefaultFn(w.buildDefaultFn(ret)).
		withDescriptionFn(w.buildDescriptionFn(ret)).
		withRulesFn(w.buildRulesFn(ret))

	if err := t.traverse(); err != nil {
		return nil, errors.Wrap(err, "traverse")
	}
	return ret, nil
}

func (w *walker) Mutate(f *feature.Feature) ([]byte, error) {
	ast, err := w.genAST()
	if err != nil {
		return nil, errors.Wrap(err, "gen ast")
	}
	t := newTraverser(ast).
		withDefaultFn(w.mutateDefaultFn(f)).
		withDescriptionFn(w.mutateDescriptionFn(f)).
		withRulesFn(w.mutateRulesFn(f))

	if err := t.traverse(); err != nil {
		return nil, errors.Wrap(err, "traverse")
	}
	return t.format(), nil
}

func (w *walker) genAST() (*build.File, error) {
	parser := butils.GetParser(star.InputTypeAuto)
	file, err := parser(w.filename, w.starBytes)
	if err != nil {
		return nil, errors.Wrap(err, "parse")
	}

	return file, nil
}

/** Methods helpful for building a go-native representation of a feature **/

func (w *walker) extractValue(vPtr *build.Expr) (interface{}, feature.FeatureType, error) {
	if vPtr == nil {
		return nil, "", fmt.Errorf("received nil value")
	}
	v := *vPtr
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

func (w *walker) buildDescriptionFn(f *feature.Feature) descriptionFn {
	return func(v *build.StringExpr) error {
		f.Description = v.Value
		return nil
	}
}

func (w *walker) buildRulesFn(f *feature.Feature) rulesFn {
	return func(rulesW *rulesWrapper) error {
		for i, r := range rulesW.rules {
			rule := &feature.Rule{
				Condition: r.conditionV.Value,
			}
			goVal, featureType, err := w.extractValue(&r.v)
			if err != nil {
				return fmt.Errorf("rule #%d: extract value: %w", i, err)
			}
			switch featureType {
			case feature.FeatureTypeBool:
				rule.Value = goVal
			default:
				return fmt.Errorf("rule #%d: unsupported feature type %s", i, featureType)
			}
			f.Rules = append(f.Rules, rule)
		}
		return nil
	}
}

func (w *walker) buildDefaultFn(f *feature.Feature) defaultFn {
	return func(v *build.Expr) error {
		goVal, featureType, err := w.extractValue(v)
		if err != nil {
			return err
		}
		switch featureType {
		case feature.FeatureTypeBool:
			boolVal, ok := goVal.(bool)
			if !ok {
				return fmt.Errorf("expected bool, got %T", goVal)
			}
			boolFeature := feature.NewBoolFeature(boolVal)
			*f = *boolFeature
		default:
			return fmt.Errorf("unsupported feature type %s", featureType)
		}
		return nil
	}
}

/** Methods helpful for mutating the underlying AST. **/

func (w *walker) genValue(goVal interface{}) (build.Expr, error) {
	switch goValType := goVal.(type) {
	case bool:
		identExpr := &build.Ident{}
		identExpr.Name = "False"
		if goValType {
			identExpr.Name = "True"
		}
		return identExpr, nil
	default:
		return nil, fmt.Errorf("unsupported feature type %T", goValType)
	}
}

func (w *walker) mutateValue(v *build.Expr, goVal interface{}) error {
	gen, err := w.genValue(goVal)
	if err != nil {
		return errors.Wrap(err, "gen value")
	}
	*v = gen
	return nil
}

func (w *walker) mutateDefaultFn(f *feature.Feature) defaultFn {
	return func(v *build.Expr) error {
		_, featureType, err := w.extractValue(v)
		if err != nil {
			return errors.Wrap(err, "extract default value")
		}
		if featureType != f.FeatureType {
			return errors.Wrapf(err, "cannot mutate star type %T with feature type %v", v, featureType)
		}
		if err := w.mutateValue(v, f.Value); err != nil {
			return errors.Wrap(err, "mutate default feature value")
		}
		return nil
	}
}

func (w *walker) mutateDescriptionFn(f *feature.Feature) descriptionFn {
	return func(v *build.StringExpr) error {
		v.Value = f.Description
		return nil
	}
}

func (w *walker) mutateRulesFn(f *feature.Feature) rulesFn {
	return func(rulesW *rulesWrapper) error {
		var newRules []rule
		for i, r := range f.Rules {
			gen, err := w.genValue(r.Value)
			if err != nil {
				return errors.Wrapf(err, "rule %d: gen value", i)
			}
			newRule := rule{
				conditionV: &build.StringExpr{
					Value: r.Condition,
				},
				v: gen,
			}
			newRules = append(newRules, newRule)
		}
		rulesW.rules = newRules
		return nil
	}
}
