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

package feature

import (
	"bytes"
	"encoding/json"
	"fmt"
	"reflect"

	featurev1beta1 "buf.build/gen/go/lekkodev/cli/protocolbuffers/go/lekko/feature/v1beta1"
	rulesv1beta2 "buf.build/gen/go/lekkodev/cli/protocolbuffers/go/lekko/rules/v1beta2"
	rulesv1beta3 "buf.build/gen/go/lekkodev/cli/protocolbuffers/go/lekko/rules/v1beta3"
	"github.com/pkg/errors"
	"github.com/stripe/skycfg"
	"go.starlark.net/starlark"
	"go.starlark.net/starlarktest"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/reflect/protoregistry"
	"google.golang.org/protobuf/types/known/anypb"
	"google.golang.org/protobuf/types/known/structpb"
	"google.golang.org/protobuf/types/known/wrapperspb"
)

// Indicates the lekko-specific types that are allowed in feature flags.
type FeatureType string

const (
	FeatureTypeBool   FeatureType = "bool"
	FeatureTypeInt    FeatureType = "int"
	FeatureTypeFloat  FeatureType = "float"
	FeatureTypeString FeatureType = "string"
	FeatureTypeProto  FeatureType = "proto"
	FeatureTypeJSON   FeatureType = "json"
)

func FeatureTypes() []string {
	var ret []string
	for _, ftype := range []FeatureType{
		FeatureTypeBool,
		FeatureTypeString,
		FeatureTypeInt,
		FeatureTypeFloat,
		FeatureTypeJSON,
		FeatureTypeProto,
	} {
		ret = append(ret, string(ftype))
	}
	return ret
}

func (ft FeatureType) ToProto() featurev1beta1.FeatureType {
	switch ft {
	case FeatureTypeBool:
		return featurev1beta1.FeatureType_FEATURE_TYPE_BOOL
	case FeatureTypeInt:
		return featurev1beta1.FeatureType_FEATURE_TYPE_INT
	case FeatureTypeFloat:
		return featurev1beta1.FeatureType_FEATURE_TYPE_FLOAT
	case FeatureTypeString:
		return featurev1beta1.FeatureType_FEATURE_TYPE_STRING
	case FeatureTypeJSON:
		return featurev1beta1.FeatureType_FEATURE_TYPE_JSON
	case FeatureTypeProto:
		return featurev1beta1.FeatureType_FEATURE_TYPE_PROTO
	default:
		return featurev1beta1.FeatureType_FEATURE_TYPE_UNSPECIFIED
	}
}

func (ft FeatureType) IsPrimitive() bool {
	primitiveTypes := map[FeatureType]struct{}{
		FeatureTypeBool:   {},
		FeatureTypeString: {},
		FeatureTypeInt:    {},
		FeatureTypeFloat:  {},
	}
	_, ok := primitiveTypes[ft]
	return ok
}

func FeatureTypeFromProto(ft featurev1beta1.FeatureType) FeatureType {
	switch ft {
	case featurev1beta1.FeatureType_FEATURE_TYPE_BOOL:
		return FeatureTypeBool
	case featurev1beta1.FeatureType_FEATURE_TYPE_INT:
		return FeatureTypeInt
	case featurev1beta1.FeatureType_FEATURE_TYPE_FLOAT:
		return FeatureTypeFloat
	case featurev1beta1.FeatureType_FEATURE_TYPE_STRING:
		return FeatureTypeString
	case featurev1beta1.FeatureType_FEATURE_TYPE_JSON:
		return FeatureTypeJSON
	case featurev1beta1.FeatureType_FEATURE_TYPE_PROTO:
		return FeatureTypeProto
	default:
		return ""
	}
}

var ErrTypeMismatch = fmt.Errorf("type mismatch")

type Rule struct {
	Condition      string             // source of truth
	ConditionAST   *rulesv1beta2.Rule // by-product of Condition
	ConditionASTV3 *rulesv1beta3.Rule
	Value          interface{}
}

type UnitTest interface {
	// Run runs the unit test
	Run(idx int, eval EvaluableFeature) *TestResult
}

// ValueUnitTest is a unit test that does an equality check
type ValueUnitTest struct {
	Context       map[string]interface{}
	ExpectedValue interface{}
	// The starlark textual representation of the context and expected value.
	// These fields are helpful for print statements.
	ContextStar, ExpectedValueStar string
}

func NewValueUnitTest(context map[string]interface{}, val interface{}, starCtx, starVal string) *ValueUnitTest {
	return &ValueUnitTest{
		Context:           context,
		ExpectedValue:     val,
		ContextStar:       starCtx,
		ExpectedValueStar: starVal,
	}
}

// CallableUnitTest is a unit test that calls a function on the value of the evaluated feature
type CallableUnitTest struct {
	Context     map[string]interface{}
	Registry    *protoregistry.Types
	TestFunc    starlark.Callable
	ContextStar string
	FeatureType FeatureType
}

func NewCallableUnitTest(ctx map[string]interface{}, r *protoregistry.Types, testFn starlark.Callable, ctxStar string, ft FeatureType) *CallableUnitTest {
	return &CallableUnitTest{
		Context:     ctx,
		Registry:    r,
		TestFunc:    testFn,
		ContextStar: ctxStar,
		FeatureType: ft,
	}
}

func valToStarValue(c any) (starlark.Value, error) {
	var sv starlark.Value
	var err error
	switch c := c.(type) {
	case *structpb.Value:
		b, err := c.MarshalJSON()
		if err != nil {
			return nil, errors.Wrap(err, "unmarshal structpb.Value")
		}
		var obj interface{}
		err = json.Unmarshal(b, &obj)
		if err != nil {
			return nil, errors.Wrap(err, "unmarshal json")
		}
		sv, err = toStarlarkValue(obj)
		if err != nil {
			return nil, errors.Wrap(err, "json to starlark")
		}
	case protoreflect.ProtoMessage:
		sv, err = skycfg.NewProtoMessage(c)
		if err != nil {
			return nil, errors.Wrap(err, "unmarshal proto message")
		}
	default:
		sv, err = toStarlarkValue(c)
		if err != nil {
			return nil, errors.Wrap(err, "convert value to starlark")
		}
	}
	return sv, nil
}

func (c *CallableUnitTest) Run(idx int, eval EvaluableFeature) *TestResult {
	tr := NewTestResult(c.ContextStar, idx)
	a, _, err := eval.Evaluate(c.Context)
	if err != nil {
		return tr.WithError(err)
	}

	// unwrap from anypb to go-native object and then to starlark
	vv, err := AnyToVal(a, c.FeatureType, c.Registry)
	if err != nil {
		return tr.WithError(err)
	}
	v, err := valToStarValue(vv)
	if err != nil {
		return tr.WithError(err)
	}
	args := starlark.Tuple([]starlark.Value{v})
	thread := &starlark.Thread{Name: "test"}
	reporter := &testReporter{}
	starlarktest.SetReporter(thread, reporter)
	_, err = starlark.Call(thread, c.TestFunc, args, nil)
	if err != nil {
		return tr.WithError(err)
	}
	return tr.WithError(reporter.toErr())
}

// todo: refactor to use the validation report as this is an exact copy
type testReporter struct {
	args   []interface{}
	failed bool
}

func (vr *testReporter) Error(args ...interface{}) {
	vr.args = append(vr.args, args...)
	vr.failed = true
}

func (vr *testReporter) hasError() bool {
	return vr.failed
}

func (vr *testReporter) toErr() error {
	if !vr.hasError() {
		return nil
	}
	return fmt.Errorf("%v", vr.args...)
}

func (ut ValueUnitTest) Run(idx int, eval EvaluableFeature) *TestResult {
	tr := NewTestResult(ut.ContextStar, idx)
	a, _, err := eval.Evaluate(ut.Context)
	if err != nil {
		return tr.WithError(errors.Wrap(err, "evaluate feature"))
	}

	val, err := ValToAny(ut.ExpectedValue, eval.Type())
	if err != nil {
		return tr.WithError(errors.Wrap(err, "invalid test value"))
	}
	if !proto.Equal(a, val) {
		if a.GetTypeUrl() != val.GetTypeUrl() {
			err = fmt.Errorf("mismatched types, expecting %s, got %s", val.GetTypeUrl(), a.GetTypeUrl())
		} else {
			err = fmt.Errorf("incorrect test result, expecting %s, got %v", ut.ExpectedValueStar, a.String())
		}
		return tr.WithError(err)
	}
	// test passed
	return tr
}

type Feature struct {
	Key, Description string
	Value            interface{}
	FeatureType      FeatureType

	Rules     []*Rule
	UnitTests []UnitTest
}

func NewBoolFeature(value bool) *Feature {
	return &Feature{
		Value:       value,
		FeatureType: FeatureTypeBool,
	}
}

func NewStringFeature(value string) *Feature {
	return &Feature{
		Value:       value,
		FeatureType: FeatureTypeString,
	}
}

func NewIntFeature(value int64) *Feature {
	return &Feature{
		Value:       value,
		FeatureType: FeatureTypeInt,
	}
}

func NewFloatFeature(value float64) *Feature {
	return &Feature{
		Value:       value,
		FeatureType: FeatureTypeFloat,
	}
}

func NewProtoFeature(value protoreflect.ProtoMessage) *Feature {
	return &Feature{
		Value:       value,
		FeatureType: FeatureTypeProto,
	}
}

func NewEncodedJSONFeature(encodedJSON []byte) (*Feature, error) {
	s, err := valFromJSON(encodedJSON)
	if err != nil {
		return nil, errors.Wrap(err, "val from json")
	}
	return NewJSONFeature(s), nil
}

func NewJSONFeature(value *structpb.Value) *Feature {
	return &Feature{
		Value:       value,
		FeatureType: FeatureTypeJSON,
	}
}

// Takes a go value and an associated type, and converts the
// value to a language-agnostic protobuf any type.
func ValToAny(value interface{}, ft FeatureType) (*anypb.Any, error) {
	switch ft {
	case FeatureTypeBool:
		v, ok := value.(bool)
		if !ok {
			return nil, errors.Errorf("expecting bool, got %T", value)
		}
		return newAny(wrapperspb.Bool(v))
	case FeatureTypeInt:
		v, ok := value.(int64)
		if !ok {
			return nil, errors.Errorf("expecting int64, got %T", value)
		}
		return newAny(wrapperspb.Int64(v))
	case FeatureTypeFloat:
		v, ok := value.(float64)
		if !ok {
			return nil, errors.Errorf("expecting float64, got %T", value)
		}
		return newAny(wrapperspb.Double(v))
	case FeatureTypeString:
		v, ok := value.(string)
		if !ok {
			return nil, errors.Errorf("expecting string, got %T", value)
		}
		return newAny(wrapperspb.String(v))
	case FeatureTypeJSON:
		v, ok := value.(*structpb.Value)
		if !ok {
			return nil, errors.Errorf("expecting *structpb.Value, got %T", value)
		}
		return newAny(v)
	case FeatureTypeProto:
		v, ok := value.(protoreflect.ProtoMessage)
		if !ok {
			return nil, errors.Errorf("expecting protoreflect.ProtoMessage, got %T", value)
		}
		return newAny(v)
	default:
		return nil, fmt.Errorf("unsupported feature type %T", value)
	}
}

func newAny(pm protoreflect.ProtoMessage) (*anypb.Any, error) {
	ret := new(anypb.Any)
	if err := anypb.MarshalFrom(ret, pm, proto.MarshalOptions{Deterministic: true}); err != nil {
		return nil, err
	}
	return ret, nil
}

// Translates the pb any object to a go-native object based on the
// given type. Also takes an optional protbuf type registry, in case the
// value depends on a user-defined protobuf type.
func AnyToVal(a *anypb.Any, fType FeatureType, registry *protoregistry.Types) (interface{}, error) {
	switch fType {
	case FeatureTypeBool:
		b := wrapperspb.BoolValue{}
		if err := a.UnmarshalTo(&b); err != nil {
			return nil, errors.Wrap(err, "unmarshal to bool")
		}
		return b.Value, nil
	case FeatureTypeInt:
		i := wrapperspb.Int64Value{}
		if err := a.UnmarshalTo(&i); err != nil {
			return nil, errors.Wrap(err, "unmarshal to int")
		}
		return i.Value, nil
	case FeatureTypeFloat:
		f := wrapperspb.DoubleValue{}
		if err := a.UnmarshalTo(&f); err != nil {
			return nil, errors.Wrap(err, "unmarshal to float")
		}
		return f.Value, nil
	case FeatureTypeString:
		s := wrapperspb.StringValue{}
		if err := a.UnmarshalTo(&s); err != nil {
			return nil, errors.Wrap(err, "unmarshal to string")
		}
		return s.Value, nil
	case FeatureTypeJSON:
		v := structpb.Value{}
		if err := a.UnmarshalTo(&v); err != nil {
			return nil, errors.Wrap(err, "unmarshal to json")
		}
		return &v, nil
	case FeatureTypeProto:
		p, err := anypb.UnmarshalNew(a, proto.UnmarshalOptions{
			Resolver: registry,
		})
		if err != nil {
			return nil, errors.Wrap(err, "unmarshal to proto")
		}
		return p.ProtoReflect(), nil
	default:
		return nil, fmt.Errorf("unsupported feature type %s", a.TypeUrl)
	}
}

func valFromJSON(encoded []byte) (*structpb.Value, error) {
	val := &structpb.Value{}
	if err := val.UnmarshalJSON(encoded); err != nil {
		return nil, errors.Wrap(err, "failed to unmarshal json into struct")
	}
	return val, nil
}

func (f *Feature) AddBoolRule(rule string, ast *rulesv1beta2.Rule, astNew *rulesv1beta3.Rule, val bool) error {
	if f.FeatureType != FeatureTypeBool {
		return newTypeMismatchErr(FeatureTypeBool, f.FeatureType)
	}
	f.Rules = append(f.Rules, &Rule{
		Condition:      rule,
		ConditionAST:   ast,
		ConditionASTV3: astNew,
		Value:          val,
	})
	return nil
}

func (f *Feature) AddStringRule(rule string, ast *rulesv1beta2.Rule, astNew *rulesv1beta3.Rule, val string) error {
	if f.FeatureType != FeatureTypeString {
		return newTypeMismatchErr(FeatureTypeString, f.FeatureType)
	}
	f.Rules = append(f.Rules, &Rule{
		Condition:    rule,
		ConditionAST: ast,
		Value:        val,
	})
	return nil
}

func (f *Feature) AddIntRule(rule string, ast *rulesv1beta2.Rule, astNew *rulesv1beta3.Rule, val int64) error {
	if f.FeatureType != FeatureTypeInt {
		return newTypeMismatchErr(FeatureTypeInt, f.FeatureType)
	}
	f.Rules = append(f.Rules, &Rule{
		Condition:      rule,
		ConditionAST:   ast,
		ConditionASTV3: astNew,
		Value:          val,
	})
	return nil
}

func (f *Feature) AddFloatRule(rule string, ast *rulesv1beta2.Rule, astNew *rulesv1beta3.Rule, val float64) error {
	if f.FeatureType != FeatureTypeFloat {
		return newTypeMismatchErr(FeatureTypeFloat, f.FeatureType)
	}
	f.Rules = append(f.Rules, &Rule{
		Condition:      rule,
		ConditionAST:   ast,
		ConditionASTV3: astNew,
		Value:          val,
	})
	return nil
}

func (f *Feature) AddJSONRule(rule string, ast *rulesv1beta2.Rule, astNew *rulesv1beta3.Rule, val *structpb.Value) error {
	if f.FeatureType != FeatureTypeJSON {
		return newTypeMismatchErr(FeatureTypeJSON, f.FeatureType)
	}
	f.Rules = append(f.Rules, &Rule{
		Condition:      rule,
		ConditionAST:   ast,
		ConditionASTV3: astNew,
		Value:          val,
	})
	return nil
}

func (f *Feature) AddJSONUnitTest(context map[string]interface{}, val *structpb.Value, starCtx, starVal string) error {
	if f.FeatureType != FeatureTypeJSON {
		return newTypeMismatchErr(FeatureTypeJSON, f.FeatureType)
	}
	f.UnitTests = append(f.UnitTests, NewValueUnitTest(context, val, starCtx, starVal))
	return nil
}

func (f *Feature) ToProto() (*featurev1beta1.Feature, error) {
	ret := &featurev1beta1.Feature{
		Key:         f.Key,
		Description: f.Description,
		Type:        f.FeatureType.ToProto(),
	}
	defaultAny, err := ValToAny(f.Value, f.FeatureType)
	if err != nil {
		return nil, fmt.Errorf("default value '%T' to any: %w", f.Value, err)
	}
	tree := &featurev1beta1.Tree{
		Default: defaultAny,
	}
	// for now, our tree only has 1 level, (it's effectievly a list)
	for _, rule := range f.Rules {
		ruleAny, err := ValToAny(rule.Value, f.FeatureType)
		if err != nil {
			return nil, errors.Wrap(err, "rule value to any")
		}
		tree.Constraints = append(tree.Constraints, &featurev1beta1.Constraint{
			Rule:       rule.Condition,
			RuleAst:    rule.ConditionAST,
			RuleAstNew: rule.ConditionASTV3,
			Value:      ruleAny,
		})
	}
	ret.Tree = tree
	return ret, nil
}

// Converts a feature from its protobuf representation into a go-native
// representation. Takes an optional proto registry in case we require
// user-defined types in order to parse the feature.
func FromProto(fProto *featurev1beta1.Feature, registry *protoregistry.Types) (*Feature, error) {
	ret := &Feature{
		Key:         fProto.Key,
		Description: fProto.Description,
	}
	ret.FeatureType = FeatureTypeFromProto(fProto.GetType())
	var err error
	ret.Value, err = AnyToVal(fProto.GetTree().GetDefault(), ret.FeatureType, registry)
	if err != nil {
		return nil, errors.Wrap(err, "any to val")
	}
	for _, constraint := range fProto.GetTree().GetConstraints() {
		ruleVal, err := AnyToVal(constraint.GetValue(), ret.FeatureType, registry)
		if err != nil {
			return nil, errors.Wrap(err, "rule any to val")
		}
		ret.Rules = append(ret.Rules, &Rule{
			Condition:      constraint.Rule,
			ConditionAST:   constraint.RuleAst,
			ConditionASTV3: constraint.RuleAstNew,
			Value:          ruleVal,
		})
	}
	return ret, nil
}

func ProtoToJSON(fProto *featurev1beta1.Feature, registry *protoregistry.Types) ([]byte, error) {
	jBytes, err := protojson.MarshalOptions{
		Resolver: registry,
	}.Marshal(fProto)
	if err != nil {
		return nil, errors.Wrap(err, "failed to marshal proto to json")
	}
	indentedJBytes := bytes.NewBuffer(nil)
	// encoding/json provides a deterministic serialization output, ensuring
	// that indentation always uses the same number of characters.
	if err := json.Indent(indentedJBytes, jBytes, "", "  "); err != nil {
		return nil, errors.Wrap(err, "failed to indent json")
	}
	return indentedJBytes.Bytes(), nil
}

func (f *Feature) ToJSON(registry *protoregistry.Types) ([]byte, error) {
	fProto, err := f.ToProto()
	if err != nil {
		return nil, errors.Wrap(err, "feature to proto")
	}
	return ProtoToJSON(fProto, registry)
}

func (f *Feature) PrintJSON(registry *protoregistry.Types) {
	jBytes, err := f.ToJSON(registry)
	if err != nil {
		fmt.Printf("failed to convert feature to json: %v\n", err)
	}
	fmt.Println(string(jBytes))
}

func (f *Feature) ToEvaluableFeature() (EvaluableFeature, error) {
	res, err := f.ToProto()
	if err != nil {
		return nil, err
	}
	return &v1beta3{res}, nil
}

// Contains the compiled feature model, along with any
// validator results and unit test results that were
// collected as part of compilation.
type CompiledFeature struct {
	Feature          *Feature
	TestResults      []*TestResult
	ValidatorResults []*ValidatorResult
}

// This struct holds information about a test run.
type TestResult struct {
	Test      string // a string representation of the test, as written in starlark
	TestIndex int    // The index of the test in the list of tests.
	Error     error  // human-readable error
}

func NewTestResult(testStar string, testIndex int) *TestResult {
	return &TestResult{
		Test:      testStar,
		TestIndex: testIndex,
	}
}

func (tr *TestResult) WithError(err error) *TestResult {
	tr.Error = err
	return tr
}

func (tr *TestResult) Passed() bool {
	return tr.Error == nil
}

// Creates a short string - a human-readable version of this unit test
// that is helpful for debugging and printing errors.
// e.g. 'test 1 ["{"org"...]'
// idx refers to the index of the unit test in the list of unit tests
// written in starlark.
func (tr *TestResult) Identifier() string {
	end := 25
	if end > len(tr.Test) {
		end = len(tr.Test)
	}
	return fmt.Sprintf("test %d [%s...]", tr.TestIndex, tr.Test[0:end])
}

func (tr *TestResult) DebugString() string {
	return fmt.Sprintf("%s: %v", tr.Identifier(), tr.Error)
}

func (f *Feature) RunUnitTests() ([]*TestResult, error) {
	eval, err := f.ToEvaluableFeature()
	if err != nil {
		return nil, errors.Wrap(err, "invalid feature")
	}
	var results []*TestResult
	for idx, test := range f.UnitTests {
		results = append(results, test.Run(idx, eval))
	}
	return results, nil
}

func newTypeMismatchErr(expected, got FeatureType) error {
	return errors.Wrapf(ErrTypeMismatch, "expected %s, got %s", expected, got)
}

type ValidatorResultType int

const (
	ValidatorResultTypeDefault ValidatorResultType = iota
	ValidatorResultTypeRule
	ValidatorResultTypeTest
)

// Holds the results of validation checks performed on the compiled feature.
// Since a validation check is done on a single final feature value,
// There will be 1 validator result for the default value and 1 for each subsequent
// rule.
type ValidatorResult struct {
	// Indicates whether this validator result applies on a rule value, the default value, or a test value.
	Type ValidatorResultType
	// If this is a rule value validator result, specifies the index of the rule that this result applies to.
	// If this is a test value validator result, specifies the index of the unit test that this result applies to.
	Index int
	// A string representation of the starlark value that was invalid
	Value string
	Error error // human-readable error describing what the validation error was
}

func NewValidatorResult(t ValidatorResultType, index int, starVal string) *ValidatorResult {
	return &ValidatorResult{
		Type:  t,
		Index: index,
		Value: starVal,
	}
}

func (vr *ValidatorResult) WithError(err error) *ValidatorResult {
	vr.Error = err
	return vr
}

func (vr *ValidatorResult) Identifier() string {
	prefix := "validate default"
	if vr.Type == ValidatorResultTypeRule {
		prefix = fmt.Sprintf("validate rule %d", vr.Index)
	}
	end := 25
	if len(vr.Value) < end {
		end = len(vr.Value)
	}
	return fmt.Sprintf("%s [%s]", prefix, vr.Value[0:end])
}

func (vr *ValidatorResult) DebugString() string {
	return fmt.Sprintf("%s: %v", vr.Identifier(), vr.Error)
}

func (vr *ValidatorResult) Passed() bool {
	return vr.Error == nil
}

// toStarlarkScalarValue converts a scalar [obj] value to its starlark Value
func toStarlarkScalarValue(obj interface{}) (starlark.Value, bool) {
	if obj == nil {
		return starlark.None, true
	}
	rt := reflect.TypeOf(obj)
	v := reflect.ValueOf(obj)
	switch rt.Kind() {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return starlark.MakeInt64(v.Int()), true
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return starlark.MakeUint64(v.Uint()), true
	case reflect.Bool:
		return starlark.Bool(v.Bool()), true
	case reflect.Float32, reflect.Float64:
		return starlark.Float(v.Float()), true
	case reflect.String:
		return starlark.String(v.String()), true
	default:
		return nil, false
	}
}

// toStarlarkValue is a DFS walk to translate the DAG from go to starlark
// this is straight from skycfg
// https://github.com/stripe/skycfg/blob/a77cda5e9354b9079ee6e7feb4b7cef6895b02ae/go/yamlmodule/yamlmodule.go#L108
func toStarlarkValue(obj interface{}) (starlark.Value, error) {
	if objval, ok := toStarlarkScalarValue(obj); ok {
		return objval, nil
	}
	rt := reflect.TypeOf(obj)
	switch rt.Kind() {
	case reflect.Map:
		ret := &starlark.Dict{}
		if obj, ok := obj.(map[string]interface{}); ok {
			for k, v := range obj {
				keyval, ok := toStarlarkScalarValue(k)
				if !ok {
					return nil, fmt.Errorf("%s (%v) is not a supported key type", rt.Kind(), obj)
				}
				starval, err := toStarlarkValue(v)
				if err != nil {
					return nil, err
				}
				if err = ret.SetKey(keyval, starval); err != nil {
					return nil, err
				}
			}
			return ret, nil
		}

	case reflect.Slice:
		if slice, ok := obj.([]interface{}); ok {
			starvals := make([]starlark.Value, len(slice))
			for i, element := range slice {
				v, err := toStarlarkValue(element)
				if err != nil {
					return nil, err
				}
				starvals[i] = v
			}
			return starlark.NewList(starvals), nil
		}
	}
	return nil, fmt.Errorf("%s (%v) is not a supported type", rt.Kind(), obj)
}
