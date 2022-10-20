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

	"github.com/lekkodev/cli/pkg/feature"
	"github.com/pkg/errors"
	"github.com/stripe/skycfg/go/protomodule"
	"go.starlark.net/lib/json"
	"go.starlark.net/starlark"
	"go.starlark.net/starlarkstruct"
	"go.starlark.net/starlarktest"
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
	allowedAttrNames map[string]struct{} = map[string]struct{}{
		DefaultValueAttrName: {},
		DescriptionAttrName:  {},
		RulesAttrName:        {},
		validatorAttrName:    {},
		unitTestsAttrName:    {},
	}
)

func makeFeature(_ *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	if len(args) > 0 {
		return nil, fmt.Errorf("feature: unexpected positional arguments")
	}
	return starlarkstruct.FromKeywords(FeatureConstructor, kwargs), nil
}

type Builder interface {
	Build() (*feature.Feature, error)
}

type featureBuilder struct {
	globals   starlark.StringDict
	validator starlark.Callable
}

func newFeatureBuilder(globals starlark.StringDict) Builder {
	return &featureBuilder{
		globals: globals,
	}
}

func (fb *featureBuilder) Build() (*feature.Feature, error) {
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
	f, err := fb.init(featureVal)
	if err != nil {
		return nil, errors.Wrap(err, "initialize feature")
	}
	f.Description, err = fb.getDescription(featureVal)
	if err != nil {
		return nil, errors.Wrap(err, "description")
	}

	if err := fb.addRules(f, featureVal); err != nil {
		return nil, errors.Wrap(err, "add rules")
	}

	if err := fb.addUnitTests(f, featureVal); err != nil {
		return nil, errors.Wrap(err, "add unit tests")
	}
	// TODO run unit tests
	return f, nil
}

func (fb *featureBuilder) getValidator(featureVal *starlarkstruct.Struct) (starlark.Callable, error) {
	validator, err := featureVal.Attr(validatorAttrName)
	if err != nil {
		// no validator provided
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
		if _, ok := allowedAttrNames[attr]; !ok {
			return fmt.Errorf("result attribute name %s not supported. use one of: %v", attr, featureVal.AttrNames())
		}
	}
	return nil
}

func (fb *featureBuilder) validate(value starlark.Value) error {
	if fb.validator == nil {
		return nil
	}
	var err error
	thread := &starlark.Thread{Name: "validate", Load: load}
	vr := &validatorReporter{}
	starlarktest.SetReporter(thread, vr)
	args := starlark.Tuple([]starlark.Value{value})
	_, err = starlark.Call(thread, fb.validator, args, nil)
	if err != nil {
		return errors.Wrap(err, "validator")
	}
	return vr.toErr()
}

func (fb *featureBuilder) init(featureVal *starlarkstruct.Struct) (*feature.Feature, error) {
	defaultVal, err := featureVal.Attr(DefaultValueAttrName)
	if err != nil {
		return nil, errors.Wrap(err, "default attribute")
	}
	if err := fb.validate(defaultVal); err != nil {
		return nil, errors.Wrap(err, "default value validate")
	}
	// check if this is a complex type
	message, ok := protomodule.AsProtoMessage(defaultVal)
	if ok {
		return feature.NewComplexFeature(message), nil
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

func (fb *featureBuilder) addRules(f *feature.Feature, featureVal *starlarkstruct.Struct) error {
	rulesVal, err := featureVal.Attr(RulesAttrName)
	if err != nil {
		// Attr returns nil, err when not present, which is terrible.
		return nil
	}
	seq, ok := rulesVal.(starlark.Sequence)
	if !ok {
		return fmt.Errorf("rules: did not get back a starlark sequence: %v", rulesVal)
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
		conditionStr, ok := tuple.Index(0).(starlark.String)
		if !ok {
			return fmt.Errorf("type error: expecting string, got %v: %v", tuple.Index(0).Type(), tuple.Index(0))
		}
		if conditionStr.GoString() == "" {
			return fmt.Errorf("expecting valid ruleslang, got %s", conditionStr.GoString())
		}
		ruleVal := tuple.Index(1)
		if err := fb.validate(ruleVal); err != nil {
			return errors.Wrap(err, fmt.Sprintf("rule #%d value validate", i))
		}
		switch f.FeatureType {
		case feature.FeatureTypeProto:
			message, ok := protomodule.AsProtoMessage(ruleVal)
			if !ok {
				return typeError(f.FeatureType, i, ruleVal)
			}
			f.Rules = append(f.Rules, &feature.Rule{
				Condition: conditionStr.GoString(),
				Value:     message,
			})
		case feature.FeatureTypeBool:
			typedRuleVal, ok := ruleVal.(starlark.Bool)
			if !ok {
				return typeError(f.FeatureType, i, ruleVal)
			}
			f.Rules = append(f.Rules, &feature.Rule{
				Condition: conditionStr.GoString(),
				Value:     bool(typedRuleVal),
			})
		case feature.FeatureTypeString:
			typedRuleVal, ok := ruleVal.(starlark.String)
			if !ok {
				return typeError(f.FeatureType, i, ruleVal)
			}
			f.Rules = append(f.Rules, &feature.Rule{
				Condition: conditionStr.GoString(),
				Value:     typedRuleVal.GoString(),
			})
		case feature.FeatureTypeInt:
			typedRuleVal, ok := ruleVal.(starlark.Int)
			if !ok {
				return typeError(f.FeatureType, i, ruleVal)
			}
			intVal, ok := typedRuleVal.Int64()
			if !ok {
				return errors.Wrapf(typeError(f.FeatureType, i, ruleVal), "%T not representable as int64", typedRuleVal)
			}
			f.Rules = append(f.Rules, &feature.Rule{
				Condition: conditionStr.GoString(),
				Value:     intVal,
			})
		case feature.FeatureTypeFloat:
			typedRuleVal, ok := ruleVal.(starlark.Float)
			if !ok {
				return typeError(f.FeatureType, i, ruleVal)
			}
			f.Rules = append(f.Rules, &feature.Rule{
				Condition: conditionStr.GoString(),
				Value:     float64(typedRuleVal),
			})
		case feature.FeatureTypeJSON:
			encoded, err := fb.extractJSON(ruleVal)
			if err != nil {
				return errors.Wrap(err, typeError(f.FeatureType, i, ruleVal).Error())
			}
			if err := f.AddJSONRule(conditionStr.GoString(), encoded); err != nil {
				return errors.Wrap(err, "failed to add json rule")
			}
		default:
			return fmt.Errorf("unsupported type %s for rule #%d", f.FeatureType, i)
		}
		i++
	}

	return nil
}

func (fb *featureBuilder) addUnitTests(f *feature.Feature, featureVal *starlarkstruct.Struct) error {
	testVal, err := featureVal.Attr(unitTestsAttrName)
	if err != nil {
		// Attr returns nil, err when not present, which is terrible.
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
		contextMap, ok := tuple.Index(0).(*starlark.Dict)
		if !ok {
			return fmt.Errorf("invalid first type of tuple %T, should be a starlark dict to build a context object", tuple.Index(0))
		}

		translatedContextMap, err := translateContext(contextMap)
		if err != nil {
			return errors.Wrap(err, "error translating starlark context for unit test")
		}

		expectedVal := tuple.Index(1)
		if err := fb.validate(expectedVal); err != nil {
			return errors.Wrap(err, "test value validate")
		}
		switch f.FeatureType {
		case feature.FeatureTypeProto:
			message, ok := protomodule.AsProtoMessage(expectedVal)
			if !ok {
				return typeError(f.FeatureType, i, expectedVal)
			}
			f.UnitTests = append(f.UnitTests, &feature.UnitTest{
				Context:       translatedContextMap,
				ExpectedValue: message,
			})
		case feature.FeatureTypeBool:
			typedUnitTestVal, ok := expectedVal.(starlark.Bool)
			if !ok {
				return typeError(f.FeatureType, i, expectedVal)
			}
			f.UnitTests = append(f.UnitTests, &feature.UnitTest{
				Context:       translatedContextMap,
				ExpectedValue: bool(typedUnitTestVal),
			})
		case feature.FeatureTypeString:
			typedUnitTestVal, ok := expectedVal.(starlark.String)
			if !ok {
				return typeError(f.FeatureType, i, expectedVal)
			}
			f.UnitTests = append(f.UnitTests, &feature.UnitTest{
				Context:       translatedContextMap,
				ExpectedValue: typedUnitTestVal.GoString(),
			})
		case feature.FeatureTypeInt:
			typedUnitTestVal, ok := expectedVal.(starlark.Int)
			if !ok {
				return typeError(f.FeatureType, i, expectedVal)
			}
			intVal, ok := typedUnitTestVal.Int64()
			if !ok {
				return errors.Wrapf(typeError(f.FeatureType, i, expectedVal), "%T not representable as int64", intVal)
			}
			f.UnitTests = append(f.UnitTests, &feature.UnitTest{
				Context:       translatedContextMap,
				ExpectedValue: intVal,
			})
		case feature.FeatureTypeFloat:
			typedUnitTestVal, ok := expectedVal.(starlark.Float)
			if !ok {
				return typeError(f.FeatureType, i, expectedVal)
			}
			f.UnitTests = append(f.UnitTests, &feature.UnitTest{
				Context:       translatedContextMap,
				ExpectedValue: float64(typedUnitTestVal),
			})
		case feature.FeatureTypeJSON:
			encoded, err := fb.extractJSON(expectedVal)
			if err != nil {
				return errors.Wrap(err, typeError(f.FeatureType, i, expectedVal).Error())
			}
			if err := f.AddJSONUnitTest(translatedContextMap, encoded); err != nil {
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
