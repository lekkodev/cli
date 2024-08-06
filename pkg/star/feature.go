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

	featurev1beta1 "buf.build/gen/go/lekkodev/cli/protocolbuffers/go/lekko/feature/v1beta1"
	rulesv1beta3 "buf.build/gen/go/lekkodev/cli/protocolbuffers/go/lekko/rules/v1beta3"
	"github.com/lekkodev/cli/pkg/feature"
	"github.com/lekkodev/go-sdk/pkg/eval"
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
	ExportConstructor    starlark.String = "export"
	ConfigConstructor    starlark.String = "Config"
	ResultVariableName   string          = "result"
	DefaultValueAttrName string          = "default"
	DescriptionAttrName  string          = "description"
	MetadataAttrName     string          = "metadata"
	// TODO: Fully migrate to overrides over rules
	RulesAttrName     string = "rules"
	OverridesAttrName string = "overrides"
	validatorAttrName string = "validator"
	unitTestsAttrName string = "tests"
)

var (
	allowedAttrNamesMap map[string]struct{} = map[string]struct{}{
		DefaultValueAttrName: {},
		DescriptionAttrName:  {},
		RulesAttrName:        {},
		OverridesAttrName:    {},
		validatorAttrName:    {},
		unitTestsAttrName:    {},
		MetadataAttrName:     {},
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
		return nil, fmt.Errorf("config: unexpected positional arguments")
	}
	return starlarkstruct.FromKeywords(FeatureConstructor, kwargs), nil
}

func makeConfig(_ *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	if len(args) > 0 {
		return nil, fmt.Errorf("config: unexpected positional arguments")
	}
	return starlarkstruct.FromKeywords(ConfigConstructor, kwargs), nil
}

func makeCallBoolean(_ *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	if len(args) > 0 {
		return nil, fmt.Errorf("config: unexpected positional arguments")
	}
	return starlarkstruct.FromKeywords(starlark.String("CallBoolean"), kwargs), nil
}

func makeExport(lekkoGlobals starlark.StringDict) func(thread *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	f := func(thread *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
		if len(args) != 1 {
			return nil, fmt.Errorf("export: exactly one possitional argument is required")
		}
		config, ok := args[0].(*starlarkstruct.Struct)
		if !ok || config.Constructor() != ConfigConstructor {
			return nil, fmt.Errorf("export: argument is not Config")
		}
		lekkoGlobals[ResultVariableName] = config
		return starlark.None, nil
	}
	return f
}

type Builder interface {
	Build() (*feature.CompiledFeature, error)
}

type featureBuilder struct {
	featureName string
	namespace   string
	globals     starlark.StringDict
	validator   starlark.Callable
	nv          feature.NamespaceVersion
	registry    *protoregistry.Types
}

func newFeatureBuilder(featureName string, namespace string, globals starlark.StringDict, nv feature.NamespaceVersion, registry *protoregistry.Types) Builder {
	return &featureBuilder{
		featureName: featureName,
		namespace:   namespace,
		globals:     globals,
		nv:          nv,
		registry:    registry,
	}
}

func (fb *featureBuilder) Build() (*feature.CompiledFeature, error) {
	resultVal, ok := fb.globals[ResultVariableName]
	if !ok {
		return nil, fmt.Errorf("required variable %s is not found", ResultVariableName)
	}
	featureVal, ok := resultVal.(*starlarkstruct.Struct)
	if !ok {
		return nil, fmt.Errorf("expecting variable of type %s, instead got %T", ConfigConstructor.GoString(), featureVal)
	}
	if err := fb.validateFeature(featureVal); err != nil {
		return nil, errors.Wrap(err, "validate config")
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
	f, err := fb.init(defaultVal) // probably where we reverse the config call
	if err != nil {
		return nil, errors.Wrap(err, "initialize config")
	}
	f.Key = fb.featureName
	f.Description, err = fb.getDescription(featureVal)
	if err != nil {
		return nil, errors.Wrap(err, "description")
	}
	f.Namespace = fb.namespace
	if fb.nv >= feature.NamespaceVersionV1Beta7 {
		f.Metadata, err = fb.getMetadata(featureVal)
		if err != nil {
			return nil, errors.Wrap(err, "metadata")
		}
	}

	overrideVals, err := fb.addOverrides(f, featureVal)
	if err != nil {
		return nil, errors.Wrap(err, "add overrides")
	}

	if err := fb.addUnitTests(f, featureVal); err != nil {
		return nil, errors.Wrap(err, "add unit tests")
	}

	// Run user validation
	var validatorResults []*feature.ValidatorResult
	if fb.validator != nil {
		validatorResults = append(validatorResults, fb.validate(feature.ValidatorResultTypeDefault, 0, defaultVal))
		for idx, ov := range overrideVals {
			validatorResults = append(validatorResults, fb.validate(feature.ValidatorResultTypeOverride, idx, ov))
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
	case *starlarkstruct.Struct:
		key, err := typedVal.Attr("key")
		if err != nil {
			return nil, err
		}
		namespace, err := typedVal.Attr("namespace")
		if err != nil {
			return nil, err
		}
		value := &featurev1beta1.ConfigCall{
			Key:       key.(starlark.String).GoString(),
			Namespace: namespace.(starlark.String).GoString(),
		}
		return &feature.Feature{
			Value:       value,
			FeatureType: eval.ConfigTypeBool, // TODO set proper return type
		}, nil
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

func (fb *featureBuilder) getMetadata(featureVal *starlarkstruct.Struct) (map[string]any, error) {
	metadataVal, err := featureVal.Attr(MetadataAttrName)
	if err != nil {
		//lint:ignore nilerr `Struct.Attr` returns error if attribute doesn't exist
		return nil, nil
	}
	metadataDict, ok := metadataVal.(*starlark.Dict)
	if !ok {
		return nil, fmt.Errorf("metadata must be a dict (got a %s)", metadataVal.Type())
	}
	metadataMap, err := translateContext(metadataDict)
	if err != nil {
		return nil, errors.Wrap(err, "translate metadata attribute")
	}
	return metadataMap, nil
}

func (fb *featureBuilder) addOverrides(f *feature.Feature, featureVal *starlarkstruct.Struct) ([]starlark.Value, error) {
	overridesVal, err := featureVal.Attr(OverridesAttrName)
	if err != nil {
		// If "overrides" is not set, check if "rules" is
		// TODO: Migrate fully to overrides
		overridesVal, err = featureVal.Attr(RulesAttrName)
		if err != nil {
			//lint:ignore nilerr Attr returns nil, err when not present, which is terrible.
			return nil, nil
		}
	} else {
		// Overrides and rules should not be set at the same time
		if _, err := featureVal.Attr(RulesAttrName); err == nil {
			return nil, errors.New("overrides and rules should not both be present")
		}
	}
	seq, ok := overridesVal.(starlark.Sequence)
	if !ok {
		return nil, fmt.Errorf("overrides: did not get back a starlark sequence: %v", overridesVal)
	}
	it := seq.Iterate()
	defer it.Done()
	var val starlark.Value
	var i int
	var overrideVals []starlark.Value
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
		ruleStr, ok := tuple.Index(0).(starlark.String)
		if !ok {
			return nil, fmt.Errorf("type error: expecting string, got %v: %v", tuple.Index(0).Type(), tuple.Index(0))
		}
		if ruleStr.GoString() == "" {
			return nil, fmt.Errorf("expecting valid ruleslang, got %s", ruleStr.GoString())
		}
		ruleASTv3, err := parseRulesLangV3(ruleStr.GoString())
		if err != nil {
			return nil, err
		}
		overrideVal := tuple.Index(1)
		overrideVals = append(overrideVals, overrideVal)
		switch f.FeatureType {
		case eval.ConfigTypeProto:
			message, ok := protomodule.AsProtoMessage(overrideVal)
			if !ok {
				return nil, typeError(f.FeatureType, i, overrideVal)
			}
			f.Overrides = append(f.Overrides, &feature.Override{
				Rule:      ruleStr.GoString(),
				RuleASTV3: ruleASTv3,
				Value:     message,
			})
		case eval.ConfigTypeBool:
			typedOverrideVal, ok := overrideVal.(starlark.Bool)
			if !ok {
				return nil, typeError(f.FeatureType, i, overrideVal)
			}
			if err := f.AddBoolOverride(ruleStr.GoString(), ruleASTv3, bool(typedOverrideVal)); err != nil {
				return nil, err
			}
		case eval.ConfigTypeString:
			typedOverrideVal, ok := overrideVal.(starlark.String)
			if !ok {
				return nil, typeError(f.FeatureType, i, overrideVal)
			}
			if err := f.AddStringOverride(ruleStr.GoString(), ruleASTv3, typedOverrideVal.GoString()); err != nil {
				return nil, err
			}
		case eval.ConfigTypeInt:
			typedOverrideVal, ok := overrideVal.(starlark.Int)
			if !ok {
				return nil, typeError(f.FeatureType, i, overrideVal)
			}
			intVal, ok := typedOverrideVal.Int64()
			if !ok {
				return nil, errors.Wrapf(typeError(f.FeatureType, i, overrideVal), "%T not representable as int64", typedOverrideVal)
			}
			if err := f.AddIntOverride(ruleStr.GoString(), ruleASTv3, intVal); err != nil {
				return nil, err
			}
		case eval.ConfigTypeFloat:
			typedOverrideVal, ok := overrideVal.(starlark.Float)
			if !ok {
				return nil, typeError(f.FeatureType, i, overrideVal)
			}
			if err := f.AddFloatOverride(ruleStr.GoString(), ruleASTv3, float64(typedOverrideVal)); err != nil {
				return nil, err
			}
		case eval.ConfigTypeJSON:
			encoded, err := fb.extractJSON(overrideVal)
			if err != nil {
				return nil, errors.Wrap(err, typeError(f.FeatureType, i, overrideVal).Error())
			}
			structVal := &structpb.Value{}
			if err := structVal.UnmarshalJSON(encoded); err != nil {
				return nil, errors.Wrapf(err, "failed to unmarshal encoded json '%s'", string(encoded))
			}
			if err := f.AddJSONOverride(ruleStr.GoString(), ruleASTv3, structVal); err != nil {
				return nil, errors.Wrap(err, "failed to add json rule")
			}
		default:
			return nil, fmt.Errorf("unsupported type %s for rule #%d", f.FeatureType, i)
		}
		i++
	}

	return overrideVals, nil
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
		case eval.ConfigTypeProto:
			message, ok := protomodule.AsProtoMessage(expectedVal)
			if !ok {
				return typeError(f.FeatureType, i, expectedVal)
			}
			f.UnitTests = append(f.UnitTests, feature.NewValueUnitTest(goCtx, message, starCtx.String(), expectedVal.String()))
		case eval.ConfigTypeBool:
			typedUnitTestVal, ok := expectedVal.(starlark.Bool)
			if !ok {
				return typeError(f.FeatureType, i, expectedVal)
			}
			f.UnitTests = append(f.UnitTests, feature.NewValueUnitTest(goCtx, bool(typedUnitTestVal), starCtx.String(), expectedVal.String()))
		case eval.ConfigTypeString:
			typedUnitTestVal, ok := expectedVal.(starlark.String)
			if !ok {
				return typeError(f.FeatureType, i, expectedVal)
			}
			f.UnitTests = append(f.UnitTests, feature.NewValueUnitTest(goCtx, typedUnitTestVal.GoString(), starCtx.String(), expectedVal.String()))
		case eval.ConfigTypeInt:
			typedUnitTestVal, ok := expectedVal.(starlark.Int)
			if !ok {
				return typeError(f.FeatureType, i, expectedVal)
			}
			intVal, ok := typedUnitTestVal.Int64()
			if !ok {
				return errors.Wrapf(typeError(f.FeatureType, i, expectedVal), "%T not representable as int64", intVal)
			}
			f.UnitTests = append(f.UnitTests, feature.NewValueUnitTest(goCtx, intVal, starCtx.String(), expectedVal.String()))
		case eval.ConfigTypeFloat:
			typedUnitTestVal, ok := expectedVal.(starlark.Float)
			if !ok {
				return typeError(f.FeatureType, i, expectedVal)
			}
			f.UnitTests = append(f.UnitTests, feature.NewValueUnitTest(goCtx, float64(typedUnitTestVal), starCtx.String(), expectedVal.String()))
		case eval.ConfigTypeJSON:
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

func typeError(expectedType eval.ConfigType, ruleIdx int, value starlark.Value) error {
	return fmt.Errorf("expecting %s for value of override idx #%d, instead got %T", starType(expectedType), ruleIdx, value)
}

func starType(ft eval.ConfigType) string {
	switch ft {
	case eval.ConfigTypeProto:
		return "protoMessage"
	case eval.ConfigTypeBool:
		return fmt.Sprintf("%T", starlark.False)
	case eval.ConfigTypeString:
		return fmt.Sprintf("%T", starlark.String(""))
	case eval.ConfigTypeInt:
		return fmt.Sprintf("%T", starlark.MakeInt64(0))
	case eval.ConfigTypeFloat:
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
