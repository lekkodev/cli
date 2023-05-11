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
	"math"
	"reflect"
	"strconv"
	"strings"

	"github.com/bazelbuild/buildtools/build"
	butils "github.com/bazelbuild/buildtools/buildifier/utils"
	"github.com/lekkodev/cli/pkg/feature"
	"github.com/lekkodev/rules/pkg/parser"
	"github.com/pkg/errors"
	"go.starlark.net/starlark"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/structpb"
)

var (
	ErrUnsupportedStaticParsing = errors.New("unsupported static parsing")
)

// Walker provides methods to statically read and manipulate .star files
// that hold lekko features.
type Walker interface {
	Build() (*feature.Feature, error)
	// Note: this method will perform mutations based on the V3 ruleslang
	// AST if it exists. If not, it will fall back to the ruleString
	// provided in the feature.
	Mutate(f *feature.Feature) ([]byte, error)
	// Returns the formatted bytes.
	Format() ([]byte, error)
}

type walker struct {
	filename  string
	starBytes []byte
}

func NewWalker(filename string, starBytes []byte) *walker {
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

func (w *walker) Format() ([]byte, error) {
	f, err := w.Build()
	if err != nil {
		return nil, errors.Wrap(err, "build")
	}
	return w.Mutate(f)
}

func (w *walker) genAST() (*build.File, error) {
	parser := butils.GetParser(InputTypeAuto)
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
			return nil, "", errors.Wrapf(ErrUnsupportedStaticParsing, "unknown identifier %s", t.Name)
		}
	case *build.LiteralExpr:
		if strings.Contains(t.Token, ".") {
			if f, err := strconv.ParseFloat(t.Token, 64); err == nil {
				return f, feature.FeatureTypeFloat, nil
			}
		} else {
			if i, err := strconv.ParseInt(t.Token, 10, 64); err == nil {
				return i, feature.FeatureTypeInt, nil
			}
		}
	case *build.StringExpr:
		return t.Value, feature.FeatureTypeString, nil
	case *build.ListExpr:
		var elemVals []interface{}
		for _, expr := range t.List {
			expr := expr
			elemVal, _, err := w.extractValue(&expr)
			if err != nil {
				return nil, "", errors.Wrap(err, "extract list elem value")
			}
			elemVals = append(elemVals, elemVal)
		}
		return elemVals, feature.FeatureTypeJSON, nil
	case *build.DictExpr:
		keyVals := make(map[string]interface{})
		for _, kvExpr := range t.List {
			kvExpr := kvExpr
			keyExpr, ok := kvExpr.Key.(*build.StringExpr)
			if !ok {
				return nil, "", errors.Errorf("json structs must have keys of type string, not %T", kvExpr.Key)
			}
			key := keyExpr.Value
			vVar, _, err := w.extractValue(&kvExpr.Value)
			if err != nil {
				return nil, "", errors.Wrap(err, "extract struct elem value")
			}
			keyVals[key] = vVar
		}
		return keyVals, feature.FeatureTypeJSON, nil
	case *build.CallExpr:
		// try to statically parse the protobuf
		proto, err := CallExprToProto(t)
		if err != nil {
			return nil, "", errors.Wrapf(ErrUnsupportedStaticParsing, "unknown expression: %v %e", v, err)
		}
		return proto, feature.FeatureTypeProto, nil
	}
	return nil, "", errors.Wrapf(ErrUnsupportedStaticParsing, "type %T", v)
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
			rulesLang := r.conditionV.Value
			ast, err := parser.BuildAST(rulesLang)
			if err != nil {
				return errors.Wrapf(err, "build ast for rule '%s'", rulesLang)
			}
			astNew, err := parser.BuildASTV3(rulesLang)
			if err != nil {
				return errors.Wrapf(err, "build ast for rule '%s'", rulesLang)
			}
			rule := &feature.Rule{
				Condition:      rulesLang,
				ConditionAST:   ast,
				ConditionASTV3: astNew,
			}
			goVal, featureType, err := w.extractValue(&r.v)
			if err != nil {
				return fmt.Errorf("rule #%d: extract value: %w", i, err)
			}
			switch featureType {
			case feature.FeatureTypeBool:
				rule.Value = goVal
			case feature.FeatureTypeFloat:
				rule.Value = goVal
			case feature.FeatureTypeInt:
				rule.Value = goVal
			case feature.FeatureTypeString:
				rule.Value = goVal
			case feature.FeatureTypeProto:
				rule.Value = goVal
			case feature.FeatureTypeJSON:
				structVal, err := structpb.NewValue(goVal)
				if err != nil {
					return errors.Wrapf(err, "rule #%d: structpb new value with type %T", i, goVal)
				}
				rule.Value = structVal
			default:
				return errors.Wrapf(ErrUnsupportedStaticParsing, "rule #%d: feature type %s", i, featureType)
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
		case feature.FeatureTypeFloat:
			floatVal, ok := goVal.(float64)
			if !ok {
				return fmt.Errorf("expected float64, got %T", goVal)
			}
			floatFeature := feature.NewFloatFeature(floatVal)
			*f = *floatFeature
		case feature.FeatureTypeInt:
			intVal, ok := goVal.(int64)
			if !ok {
				return fmt.Errorf("expected int64, got %T", goVal)
			}
			intFeature := feature.NewIntFeature(intVal)
			*f = *intFeature
		case feature.FeatureTypeString:
			stringVal, ok := goVal.(string)
			if !ok {
				return fmt.Errorf("expected string, got %T", goVal)
			}
			stringFeature := feature.NewStringFeature(stringVal)
			*f = *stringFeature
		case feature.FeatureTypeJSON:
			structVal, err := structpb.NewValue(goVal)
			if err != nil {
				return errors.Wrap(err, "structpb new value")
			}
			jsonFeature := feature.NewJSONFeature(structVal)
			if err != nil {
				return errors.Wrap(err, "new json feature")
			}
			*f = *jsonFeature
		case feature.FeatureTypeProto:
			proto, ok := goVal.(proto.Message)
			if !ok {
				return fmt.Errorf("expected proto, got %T", goVal)
			}
			protoFeature := feature.NewProtoFeature(proto)
			*f = *protoFeature
		default:
			return errors.Wrapf(ErrUnsupportedStaticParsing, "feature type %s", featureType)
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
	case float64:
		return &build.LiteralExpr{
			Token: starlark.Float(goValType).String(),
		}, nil
	case int64:
		return &build.LiteralExpr{
			Token: strconv.FormatInt(goValType, 10),
		}, nil
	case string:
		return &build.StringExpr{
			Value: goValType,
		}, nil
	case *structpb.Value:
		return w.genValue(goValType.GetKind())
	case *structpb.Value_ListValue:
		listExpr := &build.ListExpr{
			ForceMultiLine: true,
		}
		for _, listElem := range goValType.ListValue.GetValues() {
			expr, err := w.genValue(listElem)
			if err != nil {
				return nil, errors.Wrap(err, "gen value list elem")
			}
			listExpr.List = append(listExpr.List, expr)
		}
		return listExpr, nil
	case *structpb.Value_StructValue:
		dictExpr := &build.DictExpr{
			ForceMultiLine: true,
		}
		for key, value := range goValType.StructValue.Fields {
			valExpr, err := w.genValue(value)
			if err != nil {
				return nil, errors.Wrap(err, "gen value dict elem")
			}
			dictExpr.List = append(dictExpr.List, &build.KeyValueExpr{
				Key:   &build.StringExpr{Value: key},
				Value: valExpr,
			})
		}
		return dictExpr, nil
	case *structpb.Value_BoolValue:
		return w.genValue(goValType.BoolValue)
	case *structpb.Value_StringValue:
		return w.genValue(goValType.StringValue)
	case *structpb.Value_NumberValue:
		if goValType.NumberValue == math.Trunc(goValType.NumberValue) {
			return w.genValue(int64(goValType.NumberValue))
		}
		return w.genValue(goValType.NumberValue)
	case proto.Message: // statically mutating protobuf
		return ProtoToStatic("pb", goValType)
	default:
		return nil, errors.Wrapf(ErrUnsupportedStaticParsing, "go val type %T", goVal)
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
		goVal, featureType, err := w.extractValue(v)
		if err != nil {
			return errors.Wrap(err, "extract default value")
		}
		if featureType != f.FeatureType {
			return errors.Wrapf(err, "cannot mutate star type %T with feature type %v", goVal, featureType)
		}
		if featureType != feature.FeatureTypeJSON {
			starValType := reflect.TypeOf(goVal)
			featureValType := reflect.TypeOf(f.Value)
			if starValType.Name() != featureValType.Name() {
				return errors.Errorf("cannot mutate go star type %T with feature type %T", goVal, f.Value)
			}
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
			newRuleString := r.Condition
			if r.ConditionASTV3 != nil {
				newRuleString, err = parser.RuleToString(r.ConditionASTV3)
				if err != nil {
					return errors.Wrap(err, "error attempting to parse v3 rule ast to string")
				}
			}
			newRule := rule{
				conditionV: &build.StringExpr{
					Value: newRuleString,
				},
				v: gen,
			}
			newRules = append(newRules, newRule)
		}
		rulesW.rules = newRules
		return nil
	}
}
