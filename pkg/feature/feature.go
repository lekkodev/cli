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

	featurev1beta1 "github.com/lekkodev/cli/pkg/gen/proto/go/lekko/feature/v1beta1"
	rulesv1beta2 "github.com/lekkodev/cli/pkg/gen/proto/go/lekko/rules/v1beta2"
	"github.com/pkg/errors"
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

func FeatureTypeFromProto(ft featurev1beta1.FeatureType) (FeatureType, error) {
	switch ft {
	case featurev1beta1.FeatureType_FEATURE_TYPE_BOOL:
		return FeatureTypeBool, nil
	case featurev1beta1.FeatureType_FEATURE_TYPE_INT:
		return FeatureTypeInt, nil
	case featurev1beta1.FeatureType_FEATURE_TYPE_FLOAT:
		return FeatureTypeFloat, nil
	case featurev1beta1.FeatureType_FEATURE_TYPE_STRING:
		return FeatureTypeString, nil
	case featurev1beta1.FeatureType_FEATURE_TYPE_JSON:
		return FeatureTypeJSON, nil
	case featurev1beta1.FeatureType_FEATURE_TYPE_PROTO:
		return FeatureTypeProto, nil
	default:
		return "", errors.Errorf("unsupported feature type %s", ft)
	}
}

var ErrTypeMismatch = fmt.Errorf("type mismatch")

type Rule struct {
	Condition    string             // source of truth
	ConditionAST *rulesv1beta2.Rule // by-product of Condition
	Value        interface{}
}

type UnitTest struct {
	Context       map[string]interface{}
	ExpectedValue interface{}
	// The starlark textual representation of the context and expected value.
	// These fields are helpful for print statements.
	ContextStar, ExpectedValueStar string
}

func NewUnitTest(context map[string]interface{}, val interface{}, starCtx, starVal string) *UnitTest {
	return &UnitTest{
		Context:           context,
		ExpectedValue:     val,
		ContextStar:       starCtx,
		ExpectedValueStar: starVal,
	}
}

func (ut UnitTest) Run(idx int, eval EvaluableFeature) *TestResult {
	tr := NewTestResult(ut.ContextStar, idx)
	a, _, err := eval.Evaluate(ut.Context)
	if err != nil {
		return tr.WithError(errors.Wrap(err, "evaluate feature"))
	}
	val, err := ValToAny(ut.ExpectedValue)
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
	UnitTests []*UnitTest
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

func ValToAny(value interface{}) (*anypb.Any, error) {
	switch typedVal := value.(type) {
	case bool:
		return newAny(wrapperspb.Bool(typedVal))
	case string:
		return newAny(wrapperspb.String(typedVal))
	case int64:
		return newAny(wrapperspb.Int64(typedVal))
	case float64:
		return newAny(wrapperspb.Double(typedVal))
	case *structpb.Value:
		return newAny(typedVal)
	case protoreflect.ProtoMessage:
		return newAny(typedVal)
	default:
		return nil, fmt.Errorf("unsupported feature type %T", typedVal)
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

func (f *Feature) AddBoolRule(rule string, ast *rulesv1beta2.Rule, val bool) error {
	if f.FeatureType != FeatureTypeBool {
		return newTypeMismatchErr(FeatureTypeBool, f.FeatureType)
	}
	f.Rules = append(f.Rules, &Rule{
		Condition:    rule,
		ConditionAST: ast,
		Value:        val,
	})
	return nil
}

func (f *Feature) AddStringRule(rule string, ast *rulesv1beta2.Rule, val string) error {
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

func (f *Feature) AddIntRule(rule string, ast *rulesv1beta2.Rule, val int64) error {
	if f.FeatureType != FeatureTypeInt {
		return newTypeMismatchErr(FeatureTypeInt, f.FeatureType)
	}
	f.Rules = append(f.Rules, &Rule{
		Condition:    rule,
		ConditionAST: ast,
		Value:        val,
	})
	return nil
}

func (f *Feature) AddFloatRule(rule string, ast *rulesv1beta2.Rule, val float64) error {
	if f.FeatureType != FeatureTypeFloat {
		return newTypeMismatchErr(FeatureTypeFloat, f.FeatureType)
	}
	f.Rules = append(f.Rules, &Rule{
		Condition:    rule,
		ConditionAST: ast,
		Value:        val,
	})
	return nil
}

func (f *Feature) AddJSONRule(rule string, ast *rulesv1beta2.Rule, val *structpb.Value) error {
	if f.FeatureType != FeatureTypeJSON {
		return newTypeMismatchErr(FeatureTypeJSON, f.FeatureType)
	}
	f.Rules = append(f.Rules, &Rule{
		Condition:    rule,
		ConditionAST: ast,
		Value:        val,
	})
	return nil
}

func (f *Feature) AddJSONUnitTest(context map[string]interface{}, val *structpb.Value, starCtx, starVal string) error {
	if f.FeatureType != FeatureTypeJSON {
		return newTypeMismatchErr(FeatureTypeJSON, f.FeatureType)
	}
	f.UnitTests = append(f.UnitTests, NewUnitTest(context, val, starCtx, starVal))
	return nil
}

func (f *Feature) ToProto() (*featurev1beta1.Feature, error) {
	ret := &featurev1beta1.Feature{
		Key:         f.Key,
		Description: f.Description,
		Type:        f.FeatureType.ToProto(),
	}
	defaultAny, err := ValToAny(f.Value)
	if err != nil {
		return nil, fmt.Errorf("default value '%T' to any: %w", f.Value, err)
	}
	tree := &featurev1beta1.Tree{
		Default: defaultAny,
	}
	// for now, our tree only has 1 level, (it's effectievly a list)
	for _, rule := range f.Rules {
		ruleAny, err := ValToAny(rule.Value)
		if err != nil {
			return nil, errors.Wrap(err, "rule value to any")
		}
		tree.Constraints = append(tree.Constraints, &featurev1beta1.Constraint{
			Rule:    rule.Condition,
			RuleAst: rule.ConditionAST,
			Value:   ruleAny,
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
	var err error
	ret.FeatureType, err = FeatureTypeFromProto(fProto.GetType())
	if err != nil {
		return nil, errors.Wrap(err, "type from proto")
	}
	// todo: pass in type registry here
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
			Condition:    constraint.Rule,
			ConditionAST: constraint.RuleAst,
			Value:        ruleVal,
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
