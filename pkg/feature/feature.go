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
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"reflect"
	"regexp"
	"sort"
	"strings"
	"text/template"
	"time"

	featurev1beta1 "buf.build/gen/go/lekkodev/cli/protocolbuffers/go/lekko/feature/v1beta1"
	rulesv1beta3 "buf.build/gen/go/lekkodev/cli/protocolbuffers/go/lekko/rules/v1beta3"
	"github.com/iancoleman/strcase"
	"github.com/lekkodev/cli/pkg/fs"
	"github.com/lekkodev/cli/pkg/metadata"
	"github.com/lekkodev/go-sdk/pkg/eval"
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
		return fmt.Errorf("config file has no name")
	}
	if ff.StarlarkFileName == "" {
		return fmt.Errorf("config file %s has no .star file", ff.Name)
	}
	if ff.CompiledJSONFileName == "" {
		return fmt.Errorf("config file %s has no .json file", ff.Name)
	}
	if ff.CompiledProtoBinFileName == "" {
		return fmt.Errorf("config file %s has no .proto.bin file", ff.Name)
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
	case "v1beta7":
		fallthrough
	case "v1beta6":
		fallthrough
	case "v1beta5":
		fallthrough
	case "v1beta4":
		fallthrough
	case "v1beta3":
		if len(f.CompiledJSONFileName) == 0 {
			return fmt.Errorf("empty compiled JSON for config: %s", f.Name)
		}
		if len(f.CompiledProtoBinFileName) == 0 {
			return fmt.Errorf("empty compiled proto for config: %s", f.Name)
		}
		if len(f.StarlarkFileName) == 0 {
			return fmt.Errorf("empty starlark file for config: %s", f.Name)
		}
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
	return "", "", fmt.Errorf("invalid config path: %s, should be of format namespace[/config]", featurePath)
}

var ErrTypeMismatch = fmt.Errorf("type mismatch")

type Override struct {
	Rule      string // source of truth
	RuleASTV3 *rulesv1beta3.Rule
	Value     interface{}
}

type UnitTest interface {
	// Run runs the unit test
	Run(idx int, eval eval.EvaluableConfig) *TestResult
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
	FeatureType eval.ConfigType
}

func NewCallableUnitTest(ctx map[string]interface{}, r *protoregistry.Types, testFn starlark.Callable, ctxStar string, ft eval.ConfigType) *CallableUnitTest {
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

func (c *CallableUnitTest) Run(idx int, eval eval.EvaluableConfig) *TestResult {
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

func (ut ValueUnitTest) Run(idx int, eval eval.EvaluableConfig) *TestResult {
	tr := NewTestResult(ut.ContextStar, idx)
	a, _, err := eval.Evaluate(ut.Context)
	if err != nil {
		return tr.WithError(errors.Wrap(err, "evaluate config"))
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
	FeatureType      eval.ConfigType
	Namespace        string
	Metadata         map[string]any

	Overrides []*Override
	UnitTests []UnitTest
}

func NewBoolFeature(value bool) *Feature {
	return &Feature{
		Value:       value,
		FeatureType: eval.ConfigTypeBool,
	}
}

func NewStringFeature(value string) *Feature {
	return &Feature{
		Value:       value,
		FeatureType: eval.ConfigTypeString,
	}
}

func NewIntFeature(value int64) *Feature {
	return &Feature{
		Value:       value,
		FeatureType: eval.ConfigTypeInt,
	}
}

func NewFloatFeature(value float64) *Feature {
	return &Feature{
		Value:       value,
		FeatureType: eval.ConfigTypeFloat,
	}
}

func NewProtoFeature(value protoreflect.ProtoMessage) *Feature {
	return &Feature{
		Value:       value,
		FeatureType: eval.ConfigTypeProto,
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
		FeatureType: eval.ConfigTypeJSON,
	}
}

// Takes a go value and an associated type, and converts the
// value to a language-agnostic protobuf any type.
func ValToAny(value interface{}, ft eval.ConfigType) (*anypb.Any, error) { // TODO kill Any
	v, ok := value.(*featurev1beta1.ConfigCall)
	if ok {
		return newAny(v)
	}
	switch ft {
	case eval.ConfigTypeBool:
		v, ok := value.(bool)
		if !ok {
			return nil, errors.Errorf("expecting bool, got %T", value)
		}
		return newAny(wrapperspb.Bool(v))
	case eval.ConfigTypeInt:
		v, ok := value.(int64)
		if !ok {
			return nil, errors.Errorf("expecting int64, got %T", value)
		}
		return newAny(wrapperspb.Int64(v))
	case eval.ConfigTypeFloat:
		v, ok := value.(float64)
		if !ok {
			return nil, errors.Errorf("expecting float64, got %T", value)
		}
		return newAny(wrapperspb.Double(v))
	case eval.ConfigTypeString:
		v, ok := value.(string)
		if !ok {
			return nil, errors.Errorf("expecting string, got %T", value)
		}
		return newAny(wrapperspb.String(v))
	case eval.ConfigTypeJSON:
		v, ok := value.(*structpb.Value)
		if !ok {
			return nil, errors.Errorf("expecting *structpb.Value, got %T", value)
		}
		return newAny(v)
	case eval.ConfigTypeProto:
		v, ok := value.(protoreflect.ProtoMessage)
		if !ok {
			return nil, errors.Errorf("expecting protoreflect.ProtoMessage, got %T", value)
		}
		return newAny(v)
	default:
		return nil, fmt.Errorf("unsupported config type %T", value)
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
func AnyToVal(a *anypb.Any, fType eval.ConfigType, registry *protoregistry.Types) (interface{}, error) {
	switch fType {
	case eval.ConfigTypeBool:
		b := wrapperspb.BoolValue{}
		if err := a.UnmarshalTo(&b); err != nil {
			return nil, errors.Wrap(err, "unmarshal to bool")
		}
		return b.Value, nil
	case eval.ConfigTypeInt:
		i := wrapperspb.Int64Value{}
		if err := a.UnmarshalTo(&i); err != nil {
			return nil, errors.Wrap(err, "unmarshal to int")
		}
		return i.Value, nil
	case eval.ConfigTypeFloat:
		f := wrapperspb.DoubleValue{}
		if err := a.UnmarshalTo(&f); err != nil {
			return nil, errors.Wrap(err, "unmarshal to float")
		}
		return f.Value, nil
	case eval.ConfigTypeString:
		s := wrapperspb.StringValue{}
		if err := a.UnmarshalTo(&s); err != nil {
			return nil, errors.Wrap(err, "unmarshal to string")
		}
		return s.Value, nil
	case eval.ConfigTypeJSON:
		v := structpb.Value{}
		if err := a.UnmarshalTo(&v); err != nil {
			return nil, errors.Wrap(err, "unmarshal to json")
		}
		return &v, nil
	case eval.ConfigTypeProto:
		p, err := anypb.UnmarshalNew(a, proto.UnmarshalOptions{
			Resolver: registry,
		})
		if err != nil {
			return nil, errors.Wrap(err, "unmarshal to proto")
		}
		return p.ProtoReflect(), nil
	default:
		return nil, fmt.Errorf("unsupported config type %s", a.TypeUrl)
	}
}

func valFromJSON(encoded []byte) (*structpb.Value, error) {
	val := &structpb.Value{}
	if err := val.UnmarshalJSON(encoded); err != nil {
		return nil, errors.Wrap(err, "failed to unmarshal json into struct")
	}
	return val, nil
}

func (f *Feature) AddBoolOverride(rule string, astNew *rulesv1beta3.Rule, val bool) error {
	if f.FeatureType != eval.ConfigTypeBool {
		return newTypeMismatchErr(eval.ConfigTypeBool, f.FeatureType)
	}
	f.Overrides = append(f.Overrides, &Override{
		Rule:      rule,
		RuleASTV3: astNew,
		Value:     val,
	})
	return nil
}

func (f *Feature) AddStringOverride(rule string, astNew *rulesv1beta3.Rule, val string) error {
	if f.FeatureType != eval.ConfigTypeString {
		return newTypeMismatchErr(eval.ConfigTypeString, f.FeatureType)
	}
	f.Overrides = append(f.Overrides, &Override{
		Rule:      rule,
		RuleASTV3: astNew,
		Value:     val,
	})
	return nil
}

func (f *Feature) AddIntOverride(rule string, astNew *rulesv1beta3.Rule, val int64) error {
	if f.FeatureType != eval.ConfigTypeInt {
		return newTypeMismatchErr(eval.ConfigTypeInt, f.FeatureType)
	}
	f.Overrides = append(f.Overrides, &Override{
		Rule:      rule,
		RuleASTV3: astNew,
		Value:     val,
	})
	return nil
}

func (f *Feature) AddFloatOverride(rule string, astNew *rulesv1beta3.Rule, val float64) error {
	if f.FeatureType != eval.ConfigTypeFloat {
		return newTypeMismatchErr(eval.ConfigTypeFloat, f.FeatureType)
	}
	f.Overrides = append(f.Overrides, &Override{
		Rule:      rule,
		RuleASTV3: astNew,
		Value:     val,
	})
	return nil
}

func (f *Feature) AddJSONOverride(rule string, astNew *rulesv1beta3.Rule, val *structpb.Value) error {
	if f.FeatureType != eval.ConfigTypeJSON {
		return newTypeMismatchErr(eval.ConfigTypeJSON, f.FeatureType)
	}
	f.Overrides = append(f.Overrides, &Override{
		Rule:      rule,
		RuleASTV3: astNew,
		Value:     val,
	})
	return nil
}

func (f *Feature) AddJSONUnitTest(context map[string]interface{}, val *structpb.Value, starCtx, starVal string) error {
	if f.FeatureType != eval.ConfigTypeJSON {
		return newTypeMismatchErr(eval.ConfigTypeJSON, f.FeatureType)
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
	for _, override := range f.Overrides {
		ruleAny, err := ValToAny(override.Value, f.FeatureType)
		if err != nil {
			return nil, errors.Wrap(err, "rule value to any")
		}
		tree.Constraints = append(tree.Constraints, &featurev1beta1.Constraint{
			Rule:       override.Rule,
			RuleAstNew: override.RuleASTV3,
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
	ret.FeatureType = eval.ConfigTypeFromProto(fProto.GetType())
	var err error
	ret.Value, err = AnyToVal(fProto.GetTree().GetDefault(), ret.FeatureType, registry)
	if err != nil {
		return nil, errors.Wrap(err, "any to val")
	}
	for _, constraint := range fProto.GetTree().GetConstraints() {
		overrideVal, err := AnyToVal(constraint.GetValue(), ret.FeatureType, registry)
		if err != nil {
			return nil, errors.Wrap(err, "rule any to val")
		}
		ret.Overrides = append(ret.Overrides, &Override{
			Rule:      constraint.Rule,
			RuleASTV3: constraint.RuleAstNew,
			Value:     overrideVal,
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
		return nil, errors.Wrap(err, "config to proto")
	}
	return ProtoToJSON(fProto, registry)
}

func (f *Feature) PrintJSON(registry *protoregistry.Types) {
	jBytes, err := f.ToJSON(registry)
	if err != nil {
		fmt.Printf("failed to convert config to json: %v\n", err)
	}
	fmt.Println(string(jBytes))
}

func (f *Feature) ToEvaluableConfig() (eval.EvaluableConfig, error) {
	res, err := f.ToProto()
	if err != nil {
		return nil, err
	}
	// TODO: pass referenced configs to support `evaluate_to`
	return eval.NewV1Beta3(res, f.Namespace, nil), nil
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
	eval, err := f.ToEvaluableConfig()
	if err != nil {
		return nil, errors.Wrap(err, "invalid config")
	}
	var results []*TestResult
	for idx, test := range f.UnitTests {
		results = append(results, test.Run(idx, eval))
	}
	return results, nil
}

func newTypeMismatchErr(expected, got eval.ConfigType) error {
	return errors.Wrapf(ErrTypeMismatch, "expected %s, got %s", expected, got)
}

type ValidatorResultType int

const (
	ValidatorResultTypeDefault ValidatorResultType = iota
	ValidatorResultTypeOverride
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
	if vr.Type == ValidatorResultTypeOverride {
		prefix = fmt.Sprintf("validate override %d", vr.Index)
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

// Automagically suggest names for a grouped config based on the inputs
// Should return options from highest to lowest confidence
// Lowest confidence item will be a simple concatenation of names
// TODO: take options like max len, call an API endpoint, etc.
func SuggestGroupedNames(configs ...*Feature) []string {
	var names []string
	for _, c := range configs {
		names = append(names, strcase.ToKebab(c.Key))
	}
	var suggestions []string
	// Really complicated AI magic goes here (a.k.a. hardcoded fake demo entries)
	time.Sleep(2 * time.Second)
	suggestions = append(suggestions, "memcache-config")
	suggestions = append(suggestions, "connection-options")
	suggestions = append(suggestions, "network-config")
	suggestions = append(suggestions, "grouped-conn-config")

	suggestions = append(suggestions, strings.Join(names, "-"))

	return suggestions
}

// Builder for a protobuf message definition string
type ProtoDefBuilder struct {
	template string
	data     struct {
		Name   string
		Enums  []string
		Fields []struct {
			Name    string
			Type    string
			Comment string
		}
	}
	done bool
}

func NewProtoDefBuilder(name string) *ProtoDefBuilder {
	// This template is not perfect - relies on post-formatting
	template := `message {{$.Name}} {
	{{range $.Enums}}{{.}}
	{{end}}

	{{range $index, $field := $.Fields}}{{- if eq $field.Comment ""}}{{- else }}
	{{$field.Comment}}{{end}}{{$field.Type}} {{$field.Name}} = {{plusOne $index}};
	{{end}}
}`
	sb := &strings.Builder{}
	sb.WriteString(fmt.Sprintf("message %s {\n", name))
	return &ProtoDefBuilder{
		template: template,
		data: struct {
			Name   string
			Enums  []string
			Fields []struct {
				Name    string
				Type    string
				Comment string
			}
		}{
			Name: name,
		},
	}
}

// Convert config type to applicable proto type name
// Returns error if type is not supported
func (b *ProtoDefBuilder) ToProtoTypeName(ct eval.ConfigType) (string, error) {
	switch ct {
	case eval.ConfigTypeBool:
		return "bool", nil
	case eval.ConfigTypeInt:
		return "int64", nil
	case eval.ConfigTypeFloat:
		return "double", nil
	case eval.ConfigTypeString:
		return "string", nil
	default:
		return "", errors.Errorf("unsupported config type %v for proto def builder", ct)
	}
}

func (b *ProtoDefBuilder) AddField(name string, typeName string, comment string) string {
	if b.done {
		return ""
	}
	field := struct {
		Name    string
		Type    string
		Comment string
	}{
		Name: b.formatFieldName(name),
		Type: typeName,
	}
	cls := strings.Split(comment, "\n")
	for _, cl := range cls {
		field.Comment += fmt.Sprintf("// %s\n", cl)
	}
	b.data.Fields = append(b.data.Fields, field)

	return field.Name
}

// Use ProtoEnumDefBuilder to get the enum definition string
func (b *ProtoDefBuilder) AddEnum(enumDefStr string) {
	if b.done {
		return
	}
	b.data.Enums = append(b.data.Enums, enumDefStr)
}

// TODO: Need to handle error cases such as language-reserved keywords
func (b *ProtoDefBuilder) formatFieldName(name string) string {
	return strcase.ToSnake(name)
}

func (b *ProtoDefBuilder) Build() string {
	tFuncs := template.FuncMap{
		"plusOne": func(i int) int {
			return i + 1
		},
	}
	t, err := template.New("proto message").Funcs(tFuncs).Parse(b.template)
	if err != nil {
		panic(err)
	}
	var ret bytes.Buffer
	err = t.Execute(&ret, b.data)
	if err != nil {
		panic(err)
	}
	return ret.String()
}

// Builder for a protobuf enum definition string
type ProtoEnumDefBuilder struct {
	template string
	data     struct {
		Name   string
		Values []string
	}
	done bool
}

func NewProtoEnumDefBuilder(name string) *ProtoEnumDefBuilder {
	// This template is not perfect - relies on post-formatting
	template := `enum {{$.Name}} {
	{{range $index, $value := $.Values}}{{$value}} = {{$index}};
	{{end}}
}`
	return &ProtoEnumDefBuilder{
		template: template,
		data: struct {
			Name   string
			Values []string
		}{Name: name, Values: make([]string, 0)},
		done: false,
	}
}

// Returns the canonical translated enum field name
func (b *ProtoEnumDefBuilder) AddValue(value string) string {
	if b.done {
		return ""
	}
	// If value is empty string, we don't have to do anything
	// because we can count it as the UNSPECIFIED value which
	// we automatically add when building
	if value == "" {
		return b.formatValue("unspecified")
	}
	valueName := b.formatValue(value)
	b.data.Values = append(b.data.Values, valueName)
	return valueName
}

func (b *ProtoEnumDefBuilder) Build() string {
	// Sort & prepend UNSPECIFIED value before building
	sort.Strings(b.data.Values)
	b.data.Values = append([]string{b.formatValue("unspecified")}, b.data.Values...)
	t, err := template.New("proto enum").Parse(b.template)
	if err != nil {
		panic(err)
	}
	var ret bytes.Buffer
	err = t.Execute(&ret, b.data)
	if err != nil {
		panic(err)
	}
	return ret.String()
}

// e.g. "hello goodbye" -> "<ENUM_NAME>_HELLO_GOODBYE"
// Removes special characters (TODO: consider erroring instead)
// TODO: handle conflicts gracefully
func (b *ProtoEnumDefBuilder) formatValue(value string) string {
	replaced := regexp.MustCompile(`[^a-zA-Z0-9 ]+`).ReplaceAllString(value, " ")
	replaced = strings.Join(strings.Fields(replaced), " ") // Handle continuous whitespace
	return strcase.ToScreamingSnake(fmt.Sprintf("%s_%s", b.data.Name, replaced))
}
