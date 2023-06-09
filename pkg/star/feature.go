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
	"fmt"

	rulesv1beta3 "buf.build/gen/go/lekkodev/cli/protocolbuffers/go/lekko/rules/v1beta3"
	"github.com/lekkodev/cli/pkg/feature"
	"github.com/lekkodev/rules/pkg/parser"

	"github.com/pkg/errors"
	"github.com/stripe/skycfg/go/protomodule"
	"go.starlark.net/lib/json"
	"go.starlark.net/starlark"
	"go.starlark.net/starlarkstruct"
	"go.starlark.net/starlarktest"
	"google.golang.org/protobuf/reflect/protoregistry"
	"google.golang.org/protobuf/types/known/structpb"
)

const (
	FeatureConstructor   starlark.String = "feature"
	FeatureVariableName  string          = "result"
	DefaultValueAttrName string          = "default"
	DescriptionAttrName  string          = "description"
	RulesAttrName        string          = "rules"
	validatorAttrName    string          = "validator"
	unitTestsAttrName    string          = "tests"
)

var (
	allowedAttrNamesMap map[string]struct{} = map[string]struct{}{
		DefaultValueAttrName: {},
		DescriptionAttrName:  {},
		RulesAttrName:        {},
		validatorAttrName:    {},
		unitTestsAttrName:    {},
	}
)

func allowedAttrName(name string) bool {
	_, ok := allowedAttrNamesMap[name]
	return ok
}

func allowedAttrNames() []string {
	var ret []string
	for name := range allowedAttrNamesMap {
		ret = append(ret, name)
	}
	return ret
}

func makeFeature(_ *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	if len(args) > 0 {
		return nil, fmt.Errorf("feature: unexpected positional arguments")
	}
	return starlarkstruct.FromKeywords(FeatureConstructor, kwargs), nil
}

type Builder interface {
	Build() (*feature.CompiledFeature, error)
}

type featureBuilder struct {
	featureName string
	globals     starlark.StringDict
	validator   starlark.Callable
	nv          feature.NamespaceVersion
	registry    *protoregistry.Types
}

func newFeatureBuilder(featureName string, globals starlark.StringDict, nv feature.NamespaceVersion, registry *protoregistry.Types) Builder {
	return &featureBuilder{
		featureName: featureName,
		globals:     globals,
		nv:          nv,
		registry:    registry,
	}
}

func (fb *featureBuilder) Build() (*feature.CompiledFeature, error) {
	resultVal, ok := fb.globals[FeatureVariableName]
	if !ok {
		return nil, fmt.Errorf("required variable %s is not found", FeatureVariableName)
	}
	featureVal, ok := resultVal.(*starlarkstruct.Struct)
	if !ok {
		return nil, fmt.Errorf("expecting variable of type %s, instead got %T", FeatureConstructor.GoString(), featureVal)
	}
	if err := fb.validateFeature(featureVal); err != nil {
		return nil, errors.Wrap(err, "validate feature")
	}
	var err error
	fb.validator, err = fb.getValidator(featureVal)
	if err != nil {
		return nil, errors.Wrap(err, "get validator")
	}
	defaultVal, err := featureVal.Attr(DefaultValueAttrName)
	if err != nil {
		return nil, errors.Wrap(err, "default attribute")
	}
	f, err := fb.init(defaultVal)
	if err != nil {
		return nil, errors.Wrap(err, "initialize feature")
	}
	f.Key = fb.featureName
	f.Description, err = fb.getDescription(featureVal)
	if err != nil {
		return nil, errors.Wrap(err, "description")
	}

	ruleVals, err := fb.addRules(f, featureVal)
	if err != nil {
		return nil, errors.Wrap(err, "add rules")
	}

	if err := fb.addUnitTests(f, featureVal); err != nil {
		return nil, errors.Wrap(err, "add unit tests")
	}

	// Run user validation
	var validatorResults []*feature.ValidatorResult
	if fb.validator != nil {
		validatorResults = append(validatorResults, fb.validate(feature.ValidatorResultTypeDefault, 0, defaultVal))
		for idx, rv := range ruleVals {
			validatorResults = append(validatorResults, fb.validate(feature.ValidatorResultTypeRule, idx, rv))
		}
	}

	// Run unit tests
	results, err := f.RunUnitTests()
	if err != nil {
		return nil, errors.Wrap(err, "run unit tests")
	}

	return &feature.CompiledFeature{
		Feature:          f,
		TestResults:      results,
		ValidatorResults: validatorResults,
	}, nil
}

func (fb *featureBuilder) getValidator(featureVal *starlarkstruct.Struct) (starlark.Callable, error) {
	validator, err := featureVal.Attr(validatorAttrName)
	if err != nil {
		//lint:ignore nilerr no validator provided
		return nil, nil
	}
	validatorCallable, ok := validator.(starlark.Callable)
	if !ok {
		return nil, fmt.Errorf("type error: received %s of type %T, expected %T", validatorAttrName, validatorCallable, starlark.Function{})
	}
	return validatorCallable, nil
}

func (fb *featureBuilder) validateFeature(featureVal *starlarkstruct.Struct) error {
	for _, attr := range featureVal.AttrNames() {
		if !allowedAttrName(attr) {
			return fmt.Errorf("result attribute name %s not supported. use one of: %v", attr, allowedAttrNames())
		}
	}
	return nil
}

// idx indicates which part of the feature we are validating. 0 for default, 1-n for each subsequent rule.
func (fb *featureBuilder) validate(t feature.ValidatorResultType, index int, value starlark.Value) *feature.ValidatorResult {
	if fb.validator == nil {
		return nil
	}
	vr := feature.NewValidatorResult(t, index, value.String())
	var err error
	thread := &starlark.Thread{Name: "validate", Load: load}
	reporter := &validatorReporter{}
	starlarktest.SetReporter(thread, reporter)
	args := starlark.Tuple([]starlark.Value{value})
	_, err = starlark.Call(thread, fb.validator, args, nil)
	if err != nil {
		return vr.WithError(err)
	}
	return vr.WithError(reporter.toErr())
}

func (fb *featureBuilder) init(defaultVal starlark.Value) (*feature.Feature, error) {
	// check if this is a proto type
	message, ok := protomodule.AsProtoMessage(defaultVal)
	if ok {
		return feature.NewProtoFeature(message), nil
	}
	// check if this is a supported primitive type
	switch typedVal := defaultVal.(type) {
	case starlark.Bool:
		return feature.NewBoolFeature(bool(typedVal)), nil
	case starlark.String:
		return feature.NewStringFeature(typedVal.GoString()), nil
	case starlark.Int:
		intVal, ok := typedVal.Int64()
		if !ok {
			return nil, errors.Errorf("int value '%s' not representable as int64", typedVal.String())
		}
		return feature.NewIntFeature(intVal), nil
	case starlark.Float:
		return feature.NewFloatFeature(float64(typedVal)), nil
	case *starlark.Dict:
		encoded, err := fb.extractJSON(defaultVal)
		if err != nil {
			return nil, errors.Wrap(err, "extract json dict")
		}
		return feature.NewEncodedJSONFeature(encoded)
	case *starlark.List:
		encoded, err := fb.extractJSON(defaultVal)
		if err != nil {
			return nil, errors.Wrap(err, "extract json list")
		}
		return feature.NewEncodedJSONFeature(encoded)
	default:
		return nil, fmt.Errorf("received default value with unsupported type %T", typedVal)
	}
}

func (fb *featureBuilder) extractJSON(jsonVal starlark.Value) ([]byte, error) {
	encodeMethodVal, err := json.Module.Attr("encode")
	if err != nil {
		return nil, errors.Wrap(err, "failed to find encode method in json module")
	}
	encodeMethodBuiltin, ok := encodeMethodVal.(*starlark.Builtin)
	if !ok {
		return nil, fmt.Errorf("encode method value of type %T expected, found %T instead", encodeMethodBuiltin, encodeMethodVal)
	}
	thread := &starlark.Thread{Name: "json encode"}
	encodedVal, err := encodeMethodBuiltin.CallInternal(thread, starlark.Tuple{jsonVal}, nil)
	if err != nil {
		return nil, errors.Wrap(err, "encode builtin")
	}
	encodedStr, ok := encodedVal.(starlark.String)
	if !ok {
		return nil, fmt.Errorf("encoded value of type %T expected, found %T instead", encodedStr, encodedVal)
	}
	return []byte(encodedStr), nil
}

func (fb *featureBuilder) getDescription(featureVal *starlarkstruct.Struct) (string, error) {
	descriptionVal, err := featureVal.Attr(DescriptionAttrName)
	if err != nil {
		return "", errors.Wrap(err, "default attribute")
	}
	dsc, ok := descriptionVal.(starlark.String)
	if !ok {
		return "", fmt.Errorf("description must be a string (got a %s)", descriptionVal.Type())
	}
	return dsc.GoString(), nil
}

func (fb *featureBuilder) addRules(f *feature.Feature, featureVal *starlarkstruct.Struct) ([]starlark.Value, error) {
	rulesVal, err := featureVal.Attr(RulesAttrName)
	if err != nil {
		//lint:ignore nilerr Attr returns nil, err when not present, which is terrible.
		return nil, nil
	}
	seq, ok := rulesVal.(starlark.Sequence)
	if !ok {
		return nil, fmt.Errorf("rules: did not get back a starlark sequence: %v", rulesVal)
	}
	it := seq.Iterate()
	defer it.Done()
	var val starlark.Value
	var i int
	var ruleVals []starlark.Value
	for it.Next(&val) {
		if val == starlark.None {
			return nil, fmt.Errorf("type error: [%v] %v", val.Type(), val.String())
		}
		tuple, ok := val.(starlark.Tuple)
		if !ok {
			return nil, fmt.Errorf("type error: expecting tuple, got %v", val.Type())
		}
		if tuple.Len() != 2 {
			return nil, fmt.Errorf("expecting tuple of length 2, got length %d: %v", tuple.Len(), tuple)
		}
		conditionStr, ok := tuple.Index(0).(starlark.String)
		if !ok {
			return nil, fmt.Errorf("type error: expecting string, got %v: %v", tuple.Index(0).Type(), tuple.Index(0))
		}
		if conditionStr.GoString() == "" {
			return nil, fmt.Errorf("expecting valid ruleslang, got %s", conditionStr.GoString())
		}
		ruleASTv3, err := parseRulesLangV3(conditionStr.GoString())
		if err != nil {
			return nil, err
		}
		ruleVal := tuple.Index(1)
		ruleVals = append(ruleVals, ruleVal)
		switch f.FeatureType {
		case feature.FeatureTypeProto:
			message, ok := protomodule.AsProtoMessage(ruleVal)
			if !ok {
				return nil, typeError(f.FeatureType, i, ruleVal)
			}
			f.Rules = append(f.Rules, &feature.Rule{
				Condition:      conditionStr.GoString(),
				ConditionASTV3: ruleASTv3,
				Value:          message,
			})
		case feature.FeatureTypeBool:
			typedRuleVal, ok := ruleVal.(starlark.Bool)
			if !ok {
				return nil, typeError(f.FeatureType, i, ruleVal)
			}
			if err := f.AddBoolRule(conditionStr.GoString(), ruleASTv3, bool(typedRuleVal)); err != nil {
				return nil, err
			}
		case feature.FeatureTypeString:
			typedRuleVal, ok := ruleVal.(starlark.String)
			if !ok {
				return nil, typeError(f.FeatureType, i, ruleVal)
			}
			if err := f.AddStringRule(conditionStr.GoString(), ruleASTv3, typedRuleVal.GoString()); err != nil {
				return nil, err
			}
		case feature.FeatureTypeInt:
			typedRuleVal, ok := ruleVal.(starlark.Int)
			if !ok {
				return nil, typeError(f.FeatureType, i, ruleVal)
			}
			intVal, ok := typedRuleVal.Int64()
			if !ok {
				return nil, errors.Wrapf(typeError(f.FeatureType, i, ruleVal), "%T not representable as int64", typedRuleVal)
			}
			if err := f.AddIntRule(conditionStr.GoString(), ruleASTv3, intVal); err != nil {
				return nil, err
			}
		case feature.FeatureTypeFloat:
			typedRuleVal, ok := ruleVal.(starlark.Float)
			if !ok {
				return nil, typeError(f.FeatureType, i, ruleVal)
			}
			if err := f.AddFloatRule(conditionStr.GoString(), ruleASTv3, float64(typedRuleVal)); err != nil {
				return nil, err
			}
		case feature.FeatureTypeJSON:
			encoded, err := fb.extractJSON(ruleVal)
			if err != nil {
				return nil, errors.Wrap(err, typeError(f.FeatureType, i, ruleVal).Error())
			}
			structVal := &structpb.Value{}
			if err := structVal.UnmarshalJSON(encoded); err != nil {
				return nil, errors.Wrapf(err, "failed to unmarshal encoded json '%s'", string(encoded))
			}
			if err := f.AddJSONRule(conditionStr.GoString(), ruleASTv3, structVal); err != nil {
				return nil, errors.Wrap(err, "failed to add json rule")
			}
		default:
			return nil, fmt.Errorf("unsupported type %s for rule #%d", f.FeatureType, i)
		}
		i++
	}

	return ruleVals, nil
}

func (fb *featureBuilder) addUnitTests(f *feature.Feature, featureVal *starlarkstruct.Struct) error {
	testVal, err := featureVal.Attr(unitTestsAttrName)
	if err != nil {
		//lint:ignore nilerr Attr returns nil, err when not present, which is terrible.
		return nil
	}
	seq, ok := testVal.(starlark.Sequence)
	if !ok {
		return fmt.Errorf("tests: did not get back a starlark sequence: %v", testVal)
	}
	it := seq.Iterate()
	defer it.Done()
	var val starlark.Value
	var i int
	for it.Next(&val) {
		if val == starlark.None {
			return fmt.Errorf("type error: [%v] %v", val.Type(), val.String())
		}
		tuple, ok := val.(starlark.Tuple)
		if !ok {
			return fmt.Errorf("type error: expecting tuple, got %v", val.Type())
		}
		if tuple.Len() != 2 {
			return fmt.Errorf("expecting tuple of length 2, got length %d: %v", tuple.Len(), tuple)
		}
		starCtx, ok := tuple.Index(0).(*starlark.Dict)
		if !ok {
			return fmt.Errorf("invalid first type of tuple %T, should be a starlark dict to build a context object", tuple.Index(0))
		}

		goCtx, err := translateContext(starCtx)
		if err != nil {
			return errors.Wrap(err, "error translating starlark context for unit test")
		}

		testFunc, ok := tuple.Index(1).(starlark.Callable)
		if ok {
			f.UnitTests = append(f.UnitTests, feature.NewCallableUnitTest(goCtx, fb.registry, testFunc, starCtx.String(), f.FeatureType))
			i++
			continue
		}
		expectedVal := tuple.Index(1)
		if vr := fb.validate(feature.ValidatorResultTypeTest, i, expectedVal); vr != nil && vr.Error != nil {
			return errors.Wrap(vr.Error, "test value validate")
		}
		switch f.FeatureType {
		case feature.FeatureTypeProto:
			message, ok := protomodule.AsProtoMessage(expectedVal)
			if !ok {
				return typeError(f.FeatureType, i, expectedVal)
			}
			f.UnitTests = append(f.UnitTests, feature.NewValueUnitTest(goCtx, message, starCtx.String(), expectedVal.String()))
		case feature.FeatureTypeBool:
			typedUnitTestVal, ok := expectedVal.(starlark.Bool)
			if !ok {
				return typeError(f.FeatureType, i, expectedVal)
			}
			f.UnitTests = append(f.UnitTests, feature.NewValueUnitTest(goCtx, bool(typedUnitTestVal), starCtx.String(), expectedVal.String()))
		case feature.FeatureTypeString:
			typedUnitTestVal, ok := expectedVal.(starlark.String)
			if !ok {
				return typeError(f.FeatureType, i, expectedVal)
			}
			f.UnitTests = append(f.UnitTests, feature.NewValueUnitTest(goCtx, typedUnitTestVal.GoString(), starCtx.String(), expectedVal.String()))
		case feature.FeatureTypeInt:
			typedUnitTestVal, ok := expectedVal.(starlark.Int)
			if !ok {
				return typeError(f.FeatureType, i, expectedVal)
			}
			intVal, ok := typedUnitTestVal.Int64()
			if !ok {
				return errors.Wrapf(typeError(f.FeatureType, i, expectedVal), "%T not representable as int64", intVal)
			}
			f.UnitTests = append(f.UnitTests, feature.NewValueUnitTest(goCtx, intVal, starCtx.String(), expectedVal.String()))
		case feature.FeatureTypeFloat:
			typedUnitTestVal, ok := expectedVal.(starlark.Float)
			if !ok {
				return typeError(f.FeatureType, i, expectedVal)
			}
			f.UnitTests = append(f.UnitTests, feature.NewValueUnitTest(goCtx, float64(typedUnitTestVal), starCtx.String(), expectedVal.String()))
		case feature.FeatureTypeJSON:
			encoded, err := fb.extractJSON(expectedVal)
			if err != nil {
				return errors.Wrap(err, typeError(f.FeatureType, i, expectedVal).Error())
			}
			structVal := &structpb.Value{}
			if err := structVal.UnmarshalJSON(encoded); err != nil {
				return errors.Wrapf(err, "failed to unmarshal encoded json '%s'", string(encoded))
			}
			if err := f.AddJSONUnitTest(goCtx, structVal, starCtx.String(), expectedVal.String()); err != nil {
				return errors.Wrap(err, "failed to add json unit test")
			}
		default:
			return fmt.Errorf("unsupported type %s for unit test #%d", f.FeatureType, i)
		}
		i++
	}

	return nil
}

// translateContext takes a starlark native context and builds a generic map[string]interface{} from it.
func translateContext(dict *starlark.Dict) (map[string]interface{}, error) {
	m := make(map[string]interface{})
	for _, k := range dict.Keys() {
		val, _, err := dict.Get(k)
		if err != nil {
			return nil, errors.Wrap(err, "transforming context")
		}
		var res interface{}
		switch v := val.(type) {
		case starlark.Bool:
			res = interface{}(v.Truth())
		case starlark.String:
			s := v.GoString()
			res = interface{}(s)
		case starlark.Int:
			// Starlark uses math.Big under the hood, so technically
			// we could get a very big number that doesn't fit in an i64.
			// For now just return an error.
			i, ok := v.Int64()
			if !ok {
				return nil, fmt.Errorf("context key %s would have overflowed", k)
			}
			res = interface{}(i)
		case starlark.Float:
			res = interface{}(float64(v))
		case *starlark.Dict:
			nestedMap, err := translateContext(v)
			if err != nil {
				return nil, err
			}
			res = interface{}(nestedMap)
		default:
			return nil, fmt.Errorf("unsupported context value %T for context key %s", v, k)
		}
		str, ok := k.(starlark.String)
		if !ok {
			return nil, fmt.Errorf("invalid context key type %T for context key %v", k, k)
		}
		m[str.GoString()] = res
	}
	return m, nil
}

func typeError(expectedType feature.FeatureType, ruleIdx int, value starlark.Value) error {
	return fmt.Errorf("expecting %s for rule idx #%d, instead got %T", starType(expectedType), ruleIdx, value)
}

func starType(ft feature.FeatureType) string {
	switch ft {
	case feature.FeatureTypeProto:
		return "protoMessage"
	case feature.FeatureTypeBool:
		return fmt.Sprintf("%T", starlark.False)
	case feature.FeatureTypeString:
		return fmt.Sprintf("%T", starlark.String(""))
	case feature.FeatureTypeInt:
		return fmt.Sprintf("%T", starlark.MakeInt64(0))
	case feature.FeatureTypeFloat:
		return fmt.Sprintf("%T", starlark.Float(0))
	default:
		return "unknown"
	}
}

type validatorReporter struct {
	args   []interface{}
	failed bool
}

func (vr *validatorReporter) Error(args ...interface{}) {
	vr.args = append(vr.args, args...)
	vr.failed = true
}

func (vr *validatorReporter) hasError() bool {
	return vr.failed
}

func (vr *validatorReporter) toErr() error {
	if !vr.hasError() {
		return nil
	}
	return fmt.Errorf("%v", vr.args...)
}

func parseRulesLangV3(ruleslang string) (*rulesv1beta3.Rule, error) {
	ret, err := parser.BuildASTV3(ruleslang)
	if err != nil {
		return nil, errors.Wrapf(err, "build rules ast v3 with ruleslang '%s'", ruleslang)
	}
	return ret, nil
}
