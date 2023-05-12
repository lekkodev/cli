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
	"testing"

	rulesv1beta2 "buf.build/gen/go/lekkodev/cli/protocolbuffers/go/lekko/rules/v1beta2"
	"github.com/lekkodev/cli/pkg/feature"
	"github.com/lekkodev/cli/pkg/star/prototypes"
	testdatav1beta1 "github.com/lekkodev/cli/pkg/star/static/testdata/gen/testproto/v1beta1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoregistry"
	"google.golang.org/protobuf/types/known/structpb"
	"google.golang.org/protobuf/types/known/wrapperspb"
)

var (
	parsableFeatureTypes = []feature.FeatureType{
		feature.FeatureTypeBool,
		feature.FeatureTypeString,
		feature.FeatureTypeInt,
		feature.FeatureTypeFloat,
		feature.FeatureTypeJSON,
	}
)

type testVal struct {
	goVal    interface{}
	starRepr string
}

func typedVals(t *testing.T, ft feature.FeatureType) (defaultVal testVal, ruleVal testVal) {
	switch ft {
	case feature.FeatureTypeBool:
		return testVal{true, "True"}, testVal{false, "False"}
	case feature.FeatureTypeFloat:
		return testVal{float64(23.98), "23.98"}, testVal{float64(22.01), "22.01"}
	case feature.FeatureTypeInt:
		return testVal{int64(23), "23"}, testVal{int64(42), "42"}
	case feature.FeatureTypeString:
		return testVal{"foo", "\"foo\""}, testVal{"bar", "\"bar\""}
	case feature.FeatureTypeJSON:
		goVal, err := structpb.NewValue([]interface{}{1, 2, 4.2, "foo"})
		require.NoError(t, err)
		ruleVal, err := structpb.NewValue(map[string]interface{}{"a": 1, "b": false, "c": []interface{}{99, "bar"}})
		require.NoError(t, err)
		return testVal{goVal, "[1, 2, 4.2, \"foo\"]"}, testVal{ruleVal, "{\"a\": 1, \"b\": False, \"c\": [99, \"bar\"]}"}
	}
	t.Fatalf("unsupported feature type %s", ft)
	return
}

func testStar(t *testing.T, ft feature.FeatureType) (testVal, testVal, []byte) {
	val, ruleVal := typedVals(t, ft)
	return val, ruleVal, []byte(fmt.Sprintf(`result = feature(
    description = "this is a simple feature",
    default = %s,
    rules = [
        ("age == 10", %s),
        ("city in [\"Rome\",\"Milan\"]", %s),
    ],
)
`, val.starRepr, ruleVal.starRepr, ruleVal.starRepr))
}

func testWalker(t *testing.T, testStar []byte) *walker {
	registry, err := prototypes.RegisterDynamicTypes(nil)
	require.NoError(t, err)
	require.NoError(t, prototypes.RegisterTypes(registry, testdatav1beta1.File_testproto_v1beta1_test_proto, true))
	return &walker{
		filename:  "test.star",
		starBytes: testStar,
		registry:  registry,
	}
}

func TestWalkerBuild(t *testing.T) {
	_, _, starBytes := testStar(t, feature.FeatureTypeBool)
	b := testWalker(t, starBytes)
	f, err := b.Build()
	require.NoError(t, err)
	require.NotNil(t, f)
	_, err = f.ToJSON(protoregistry.GlobalTypes)
	require.NoError(t, err)
}

func TestWalkerBuildJSON(t *testing.T) {
	_, _, starBytes := testStar(t, feature.FeatureTypeJSON)
	b := testWalker(t, starBytes)
	f, err := b.Build()
	require.NoError(t, err)
	require.NotNil(t, f)
	_, err = f.ToJSON(protoregistry.GlobalTypes)
	require.NoError(t, err)
}

func TestWalkerMutateNoop(t *testing.T) {
	for _, fType := range parsableFeatureTypes {
		t.Run(string(fType), func(t *testing.T) {
			_, _, starBytes := testStar(t, fType)
			b := testWalker(t, starBytes)
			f, err := b.Build()
			require.NoError(t, err)
			require.NotNil(t, f)

			bytes, err := b.Mutate(f)
			require.NoError(t, err)
			if fType != feature.FeatureTypeJSON {
				// JSON struct feature types are represented as go maps
				// after being parsed. Go maps have nondeterministic ordering
				// of keys. As a result, when they are transformed back to
				// starlark bytes, the order of the json object may be different.
				assert.EqualValues(t, string(starBytes), string(bytes))
			}
		})
	}
}

func TestWalkerMutateDefault(t *testing.T) {
	_, _, starBytes := testStar(t, feature.FeatureTypeBool)
	b := testWalker(t, starBytes)
	f, err := b.Build()
	require.NoError(t, err)
	require.NotNil(t, f)
	defaultVal, ok := f.Value.(bool)
	require.True(t, ok)
	require.True(t, defaultVal)

	f.Value = false
	bytes, err := b.Mutate(f)
	require.NoError(t, err)
	assert.NotEqualValues(t, starBytes, bytes)
}

func TestWalkerMutateModifyRuleCondition(t *testing.T) {
	_, _, starBytes := testStar(t, feature.FeatureTypeBool)
	b := testWalker(t, starBytes)
	f, err := b.Build()
	require.NoError(t, err)
	require.NotNil(t, f)

	f.Rules[0].ConditionASTV3 = nil // set to nil so that mutation favors the raw string
	f.Rules[0].Condition = "age == 12"

	bytes, err := b.Mutate(f)
	require.NoError(t, err)
	assert.Contains(t, string(bytes), "age == 12")
}

func TestWalkerMutateModifyRuleConditionV3(t *testing.T) {
	_, _, starBytes := testStar(t, feature.FeatureTypeBool)
	b := testWalker(t, starBytes)
	f, err := b.Build()
	require.NoError(t, err)
	require.NotNil(t, f)

	// the AST takes precedence over the condition string
	f.Rules[0].ConditionASTV3.GetAtom().ComparisonValue = structpb.NewNumberValue(12)

	bytes, err := b.Mutate(f)
	require.NoError(t, err)
	assert.Contains(t, string(bytes), "age == 12")
}

func TestWalkerMutateAddRule(t *testing.T) {
	_, _, starBytes := testStar(t, feature.FeatureTypeBool)
	b := testWalker(t, starBytes)
	f, err := b.Build()
	require.NoError(t, err)
	require.NotNil(t, f)

	f.Rules = append(f.Rules, &feature.Rule{
		Condition: "age == 12",
		ConditionAST: &rulesv1beta2.Rule{
			Rule: &rulesv1beta2.Rule_Atom{
				Atom: &rulesv1beta2.Atom{
					ContextKey:         "age",
					ComparisonValue:    structpb.NewNumberValue(12),
					ComparisonOperator: rulesv1beta2.ComparisonOperator_COMPARISON_OPERATOR_EQUALS,
				},
			},
		},
		Value: false,
	})

	bytes, err := b.Mutate(f)
	require.NoError(t, err)
	assert.Contains(t, string(bytes), "(\"age == 12\", False)")
}

func TestWalkerMutateAddFirstRule(t *testing.T) {
	val, _ := typedVals(t, feature.FeatureTypeBool)
	starBytes := []byte(fmt.Sprintf(`result = feature(
    description = "this is a simple feature",
    default = %s,
	)
	`, val.starRepr))
	b := testWalker(t, starBytes)
	f, err := b.Build()
	require.NoError(t, err)
	require.NotNil(t, f)

	f.Rules = append(f.Rules, &feature.Rule{
		Condition: "age == 12",
		ConditionAST: &rulesv1beta2.Rule{
			Rule: &rulesv1beta2.Rule_Atom{
				Atom: &rulesv1beta2.Atom{
					ContextKey:         "age",
					ComparisonValue:    structpb.NewNumberValue(12),
					ComparisonOperator: rulesv1beta2.ComparisonOperator_COMPARISON_OPERATOR_EQUALS,
				},
			},
		},
		Value: false,
	})

	bytes, err := b.Mutate(f)
	require.NoError(t, err)
	assert.Contains(t, string(bytes), "(\"age == 12\", False)")
}

func TestWalkerMutateRemoveRule(t *testing.T) {
	_, _, starBytes := testStar(t, feature.FeatureTypeBool)
	b := testWalker(t, starBytes)
	f, err := b.Build()
	require.NoError(t, err)
	require.NotNil(t, f)

	f.Rules = f.Rules[1:]

	bytes, err := b.Mutate(f)
	require.NoError(t, err)
	assert.NotContains(t, string(bytes), "(\"age == 10\", False)")
}

func TestWalkerMutateRemoveOnlyRule(t *testing.T) {
	val, ruleVal := typedVals(t, feature.FeatureTypeBool)
	starBytes := []byte(fmt.Sprintf(`result = feature(
    description = "this is a simple feature",
    default = %s,
	rules = [
		("age == 10", %s),
	],
)
	`, val.starRepr, ruleVal.starRepr))
	b := testWalker(t, starBytes)
	f, err := b.Build()
	require.NoError(t, err)
	require.NotNil(t, f)

	f.Rules = nil

	bytes, err := b.Mutate(f)
	require.NoError(t, err)
	assert.NotContains(t, string(bytes), "age == 10")
}

func TestWalkerMutateDescription(t *testing.T) {
	for _, fType := range parsableFeatureTypes {
		t.Run(string(fType), func(t *testing.T) {
			_, _, starBytes := testStar(t, fType)
			b := testWalker(t, starBytes)
			f, err := b.Build()
			require.NoError(t, err)
			require.NotNil(t, f)

			f.Description = "a NEW way to describe this feature."

			bytes, err := b.Mutate(f)
			require.NoError(t, err)
			assert.Contains(t, string(bytes), "a NEW way to describe this feature.")
		})
	}
}

func TestWalkerMutateTypeMismatch(t *testing.T) {
	_, _, starBytes := testStar(t, feature.FeatureTypeFloat)
	b := testWalker(t, starBytes)
	f, err := b.Build()
	require.NoError(t, err)
	require.NotNil(t, f)

	f.Value = int64(29) // change from float to int
	_, err = b.Mutate(f)
	require.Error(t, err)
}

func TestWalkerMutateDefaultFloat(t *testing.T) {
	val, _, starBytes := testStar(t, feature.FeatureTypeFloat)
	b := testWalker(t, starBytes)
	f, err := b.Build()
	require.NoError(t, err)
	require.NotNil(t, f)
	defaultVal, ok := f.Value.(float64)
	require.True(t, ok)
	require.EqualValues(t, defaultVal, val.goVal)

	f.Value = float64(99.99)
	bytes, err := b.Mutate(f)
	require.NoError(t, err)
	assert.NotEqualValues(t, starBytes, bytes)
}

func TestWalkerMutateDefaultInt(t *testing.T) {
	val, _, starBytes := testStar(t, feature.FeatureTypeInt)
	b := testWalker(t, starBytes)
	f, err := b.Build()
	require.NoError(t, err)
	require.NotNil(t, f)
	defaultVal, ok := f.Value.(int64)
	require.True(t, ok)
	require.EqualValues(t, defaultVal, val.goVal)

	f.Value = int64(99)
	bytes, err := b.Mutate(f)
	require.NoError(t, err)
	assert.NotEqualValues(t, starBytes, bytes)
}

func TestWalkerMutateDefaultString(t *testing.T) {
	val, _, starBytes := testStar(t, feature.FeatureTypeString)
	b := testWalker(t, starBytes)
	f, err := b.Build()
	require.NoError(t, err)
	require.NotNil(t, f)
	defaultVal, ok := f.Value.(string)
	require.True(t, ok)
	require.EqualValues(t, defaultVal, val.goVal)

	f.Value = "hello"
	bytes, err := b.Mutate(f)
	require.NoError(t, err)
	assert.NotEqualValues(t, starBytes, bytes)
	assert.Contains(t, string(bytes), "hello")
}

func TestWalkerMutateDefaultJSON(t *testing.T) {
	_, _, starBytes := testStar(t, feature.FeatureTypeJSON)
	b := testWalker(t, starBytes)
	f, err := b.Build()
	require.NoError(t, err)
	require.NotNil(t, f)
	defaultVal, ok := f.Value.(*structpb.Value)
	require.True(t, ok)
	require.NotEmpty(t, defaultVal.GetListValue().Values)

	defaultVal.GetListValue().Values = append(defaultVal.GetListValue().Values, structpb.NewStringValue("foobar"))
	bytes, err := b.Mutate(f)
	require.NoError(t, err)
	assert.NotEqualValues(t, starBytes, bytes)
	assert.Contains(t, string(bytes), "foobar")
}

func TestWalkerBuildProtoDefault(t *testing.T) {
	for i, tc := range []struct {
		starVal  string
		expected proto.Message
		suppress bool // if true, this test is not enforced.
	}{
		{
			starVal:  "pb.BoolValue(value = False)",
			expected: &wrapperspb.BoolValue{Value: false},
			suppress: true,
		},
		{
			starVal:  "pb.BoolValue(value = True)",
			expected: &wrapperspb.BoolValue{Value: true},
		},
		{
			starVal:  "pb.StringValue(value = \"foo\")",
			expected: &wrapperspb.StringValue{Value: "foo"},
		},
		{
			starVal:  "pb.StringValue(value = \"\")",
			expected: &wrapperspb.StringValue{Value: ""},
			suppress: true,
		},
		{
			starVal:  "pb.Int32Value(value = 42)",
			expected: &wrapperspb.Int32Value{Value: 42},
		},
		{
			starVal:  "pb.UInt32Value(value = 42)",
			expected: &wrapperspb.UInt32Value{Value: 42},
		},
		{
			starVal:  "pb.Int64Value(value = 42)",
			expected: &wrapperspb.Int64Value{Value: 42},
		},
		{
			starVal:  "pb.UInt64Value(value = 42)",
			expected: &wrapperspb.UInt64Value{Value: 42},
		},
		{
			starVal:  "pb.FloatValue(value = 42.42)",
			expected: &wrapperspb.FloatValue{Value: 42.42},
		},
		{
			starVal:  "pb.DoubleValue(value = 42.42)",
			expected: &wrapperspb.DoubleValue{Value: 42.42},
		},
		{
			starVal:  "pb.BytesValue(value = \"foo\")",
			expected: &wrapperspb.BytesValue{Value: []byte(`foo`)},
		},
		{
			starVal:  "tpb.TestMessage(val = \"foo\")",
			expected: &testdatav1beta1.TestMessage{Val: "foo"},
		},
	} {
		t.Run(fmt.Sprintf("%d|%s", i, tc.starVal), func(t *testing.T) {
			star := []byte(fmt.Sprintf(`
			pb = proto.package("google.protobuf")
			result = feature(
				description = "proto feature",
				default = %s,
			)
			`, tc.starVal))
			b := testWalker(t, star)
			f, err := b.Build()
			require.NoError(t, err)
			require.NotNil(t, f)
			protoMessage, ok := f.Value.(proto.Message)
			require.True(t, ok)
			require.NotNil(t, protoMessage)
			require.True(t, proto.Equal(protoMessage, tc.expected), "expected %v %T, got %v %T", tc.expected, tc.expected, protoMessage, protoMessage)
			// Now that static parsing is done, try a no-op static mutation.
			result, err := b.Mutate(f)
			require.NoError(t, err)
			if !tc.suppress { // suppress some test cases because defaults are not supported yet.
				assert.Contains(t, string(result), tc.starVal)
			}
		})
	}
}
