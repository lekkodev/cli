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
	"os"
	"testing"

	featurev1beta1 "buf.build/gen/go/lekkodev/cli/protocolbuffers/go/lekko/feature/v1beta1"
	rulesv1beta3 "buf.build/gen/go/lekkodev/cli/protocolbuffers/go/lekko/rules/v1beta3"
	"github.com/lekkodev/cli/pkg/star/prototypes"
	testdatav1beta1 "github.com/lekkodev/cli/pkg/star/static/testdata/gen/testproto/v1beta1"
	"github.com/lekkodev/go-sdk/pkg/eval"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/anypb"
	"google.golang.org/protobuf/types/known/structpb"
	"google.golang.org/protobuf/types/known/wrapperspb"
)

var (
	parsableFeatureTypes = []eval.FeatureType{
		eval.FeatureTypeBool,
		eval.FeatureTypeString,
		eval.FeatureTypeInt,
		eval.FeatureTypeFloat,
		eval.FeatureTypeJSON,
	}
)

type testVal struct {
	goVal    interface{}
	starRepr string
}

func typedVals(t *testing.T, ft eval.FeatureType) (defaultVal testVal, ruleVal testVal) {
	switch ft {
	case eval.FeatureTypeBool:
		return testVal{true, "True"}, testVal{false, "False"}
	case eval.FeatureTypeFloat:
		return testVal{float64(23.98), "23.98"}, testVal{float64(22.01), "22.01"}
	case eval.FeatureTypeInt:
		return testVal{int64(23), "23"}, testVal{int64(42), "42"}
	case eval.FeatureTypeString:
		return testVal{"foo", "\"foo\""}, testVal{"bar", "\"bar\""}
	case eval.FeatureTypeJSON:
		goVal, err := structpb.NewValue([]interface{}{"foo", 1, 2, 4.2})
		require.NoError(t, err)
		ruleVal, err := structpb.NewValue(map[string]interface{}{"a": 1, "b": false, "c": []interface{}{99, "bar"}})
		require.NoError(t, err)
		ruleStarVal := `{
            "a": 1,
            "b": False,
            "c": [99, "bar"],
        }`
		return testVal{goVal, "[\"foo\", 1, 2, 4.2]"}, testVal{ruleVal, ruleStarVal}
	}
	t.Fatalf("unsupported feature type %s", ft)
	return
}

func testStar(t *testing.T, ft eval.FeatureType) (testVal, []byte) {
	val, ruleVal := typedVals(t, ft)
	return val, []byte(fmt.Sprintf(`result = feature(
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
	sTypes, err := prototypes.RegisterDynamicTypes(nil)
	require.NoError(t, err)
	require.NoError(t, sTypes.AddFileDescriptor(testdatav1beta1.File_testproto_v1beta1_test_proto, true))
	return &walker{
		filename:  "test.star",
		starBytes: testStar,
		registry:  sTypes.Types,
	}
}

func TestWalkerBuild(t *testing.T) {
	_, starBytes := testStar(t, eval.FeatureTypeBool)
	b := testWalker(t, starBytes)
	f, err := b.Build()
	require.NoError(t, err)
	require.NotNil(t, f)
}

func TestWalkerBuildJSON(t *testing.T) {
	_, starBytes := testStar(t, eval.FeatureTypeJSON)
	b := testWalker(t, starBytes)
	f, err := b.Build()
	require.NoError(t, err)
	require.NotNil(t, f)
}

func TestWalkerMutateNoop(t *testing.T) {
	for _, fType := range parsableFeatureTypes {
		t.Run(string(fType), func(t *testing.T) {
			_, starBytes := testStar(t, fType)
			b := testWalker(t, starBytes)
			f, err := b.Build()
			require.NoError(t, err)
			require.NotNil(t, f)

			bytes, err := b.Mutate(f)
			require.NoError(t, err)
			assert.EqualValues(t, string(starBytes), string(bytes))
		})
	}
}

func TestWalkerMutateDefault(t *testing.T) {
	_, starBytes := testStar(t, eval.FeatureTypeBool)
	b := testWalker(t, starBytes)
	f, err := b.Build()
	require.NoError(t, err)
	require.NotNil(t, f)
	bv := &wrapperspb.BoolValue{}
	require.NoError(t, f.FeatureOld.Tree.Default.UnmarshalTo(bv))
	require.True(t, bv.Value)

	f.FeatureOld.Tree.Default, err = b.toAny(wrapperspb.Bool(false))
	require.NoError(t, err)
	bytes, err := b.Mutate(f)
	require.NoError(t, err)
	assert.NotEqualValues(t, starBytes, bytes)
}

func TestWalkerMutateModifyRuleCondition(t *testing.T) {
	_, starBytes := testStar(t, eval.FeatureTypeBool)
	b := testWalker(t, starBytes)
	f, err := b.Build()
	require.NoError(t, err)
	require.NotNil(t, f)

	f.FeatureOld.Tree.Constraints[0].RuleAstNew = nil // set to nil so that mutation favors the raw string
	f.FeatureOld.Tree.Constraints[0].Rule = "age == 12"

	bytes, err := b.Mutate(f)
	require.NoError(t, err)
	assert.Contains(t, string(bytes), "age == 12")
}

func TestWalkerMutateModifyOverrideRuleV3(t *testing.T) {
	_, starBytes := testStar(t, eval.FeatureTypeBool)
	b := testWalker(t, starBytes)
	f, err := b.Build()
	require.NoError(t, err)
	require.NotNil(t, f)

	// the AST takes precedence over the rule string
	f.FeatureOld.Tree.Constraints[0].RuleAstNew.GetAtom().ComparisonValue = structpb.NewNumberValue(12)

	bytes, err := b.Mutate(f)
	require.NoError(t, err)
	assert.Contains(t, string(bytes), "age == 12")
}

func TestWalkerMutateAddOverride(t *testing.T) {
	_, starBytes := testStar(t, eval.FeatureTypeBool)
	b := testWalker(t, starBytes)
	f, err := b.Build()
	require.NoError(t, err)
	require.NotNil(t, f)

	falseAny, err := b.toAny(wrapperspb.Bool(false))
	require.NoError(t, err)

	f.FeatureOld.Tree.Constraints = append(f.FeatureOld.Tree.Constraints, &featurev1beta1.Constraint{
		Rule: "age == 12",
		RuleAstNew: &rulesv1beta3.Rule{
			Rule: &rulesv1beta3.Rule_Atom{
				Atom: &rulesv1beta3.Atom{
					ContextKey:         "age",
					ComparisonValue:    structpb.NewNumberValue(12),
					ComparisonOperator: rulesv1beta3.ComparisonOperator_COMPARISON_OPERATOR_EQUALS,
				},
			},
		},
		Value: falseAny,
	})

	bytes, err := b.Mutate(f)
	require.NoError(t, err)
	assert.Contains(t, string(bytes), "(\"age == 12\", False)")
}

func TestWalkerMutateAddFirstOverride(t *testing.T) {
	val, _ := typedVals(t, eval.FeatureTypeBool)
	starBytes := []byte(fmt.Sprintf(`result = feature(
    description = "this is a simple feature",
    default = %s,
	)
	`, val.starRepr))
	b := testWalker(t, starBytes)
	f, err := b.Build()
	require.NoError(t, err)
	require.NotNil(t, f)

	falseAny, err := b.toAny(wrapperspb.Bool(false))
	require.NoError(t, err)

	f.FeatureOld.Tree.Constraints = append(f.FeatureOld.Tree.Constraints, &featurev1beta1.Constraint{
		Rule: "age == 12",
		RuleAstNew: &rulesv1beta3.Rule{
			Rule: &rulesv1beta3.Rule_Atom{
				Atom: &rulesv1beta3.Atom{
					ContextKey:         "age",
					ComparisonValue:    structpb.NewNumberValue(12),
					ComparisonOperator: rulesv1beta3.ComparisonOperator_COMPARISON_OPERATOR_EQUALS,
				},
			},
		},
		Value: falseAny,
	})

	bytes, err := b.Mutate(f)
	require.NoError(t, err)
	assert.Contains(t, string(bytes), "(\"age == 12\", False)")
}

func TestWalkerMutateRemoveOverride(t *testing.T) {
	_, starBytes := testStar(t, eval.FeatureTypeBool)
	b := testWalker(t, starBytes)
	f, err := b.Build()
	require.NoError(t, err)
	require.NotNil(t, f)

	f.FeatureOld.Tree.Constraints = f.FeatureOld.Tree.Constraints[1:]

	bytes, err := b.Mutate(f)
	require.NoError(t, err)
	assert.NotContains(t, string(bytes), "(\"age == 10\", False)")
}

func TestWalkerMutateRemoveOnlyOverride(t *testing.T) {
	val, ruleVal := typedVals(t, eval.FeatureTypeBool)
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

	f.FeatureOld.Tree.Constraints = nil

	bytes, err := b.Mutate(f)
	require.NoError(t, err)
	assert.NotContains(t, string(bytes), "age == 10")
}

func TestWalkerMutateDescription(t *testing.T) {
	for _, fType := range parsableFeatureTypes {
		t.Run(string(fType), func(t *testing.T) {
			_, starBytes := testStar(t, fType)
			b := testWalker(t, starBytes)
			f, err := b.Build()
			require.NoError(t, err)
			require.NotNil(t, f)

			f.FeatureOld.Description = "a NEW way to describe this feature."

			bytes, err := b.Mutate(f)
			require.NoError(t, err)
			assert.Contains(t, string(bytes), "a NEW way to describe this feature.")
		})
	}
}

func TestWalkerMutateTypeMismatch(t *testing.T) {
	_, starBytes := testStar(t, eval.FeatureTypeFloat)
	b := testWalker(t, starBytes)
	f, err := b.Build()
	require.NoError(t, err)
	require.NotNil(t, f)

	// change from float to int
	f.FeatureOld.Tree.Default, err = b.toAny(wrapperspb.Int64(29))
	require.NoError(t, err)
	_, err = b.Mutate(f)
	require.Error(t, err)
}

func TestWalkerMutateDefaultFloat(t *testing.T) {
	val, starBytes := testStar(t, eval.FeatureTypeFloat)
	b := testWalker(t, starBytes)
	f, err := b.Build()
	require.NoError(t, err)
	require.NotNil(t, f)
	dv := &wrapperspb.DoubleValue{}
	require.NoError(t, f.FeatureOld.Tree.Default.UnmarshalTo(dv))
	require.EqualValues(t, dv.Value, val.goVal)

	newFloatAny, err := b.toAny(wrapperspb.Double(99.99))
	require.NoError(t, err)
	f.FeatureOld.Tree.Default = newFloatAny
	bytes, err := b.Mutate(f)
	require.NoError(t, err)
	assert.NotEqualValues(t, starBytes, bytes)
}

func TestWalkerMutateDefaultInt(t *testing.T) {
	val, starBytes := testStar(t, eval.FeatureTypeInt)
	b := testWalker(t, starBytes)
	f, err := b.Build()
	require.NoError(t, err)
	require.NotNil(t, f)
	iv := &wrapperspb.Int64Value{}
	require.NoError(t, f.FeatureOld.Tree.Default.UnmarshalTo(iv))
	require.EqualValues(t, iv.Value, val.goVal)

	newIntAny, err := b.toAny(wrapperspb.Int64(99))
	require.NoError(t, err)
	f.FeatureOld.Tree.Default = newIntAny
	bytes, err := b.Mutate(f)
	require.NoError(t, err)
	assert.NotEqualValues(t, starBytes, bytes)
}

func TestWalkerMutateDefaultString(t *testing.T) {
	val, starBytes := testStar(t, eval.FeatureTypeString)
	b := testWalker(t, starBytes)
	f, err := b.Build()
	require.NoError(t, err)
	require.NotNil(t, f)
	sv := &wrapperspb.StringValue{}
	require.NoError(t, f.FeatureOld.Tree.Default.UnmarshalTo(sv))
	require.EqualValues(t, sv.Value, val.goVal)

	newStringAny, err := b.toAny(wrapperspb.String("hello"))
	require.NoError(t, err)
	f.FeatureOld.Tree.Default = newStringAny
	bytes, err := b.Mutate(f)
	require.NoError(t, err)
	assert.NotEqualValues(t, starBytes, bytes)
	assert.Contains(t, string(bytes), "hello")
}

func TestWalkerMutateDefaultJSON(t *testing.T) {
	_, starBytes := testStar(t, eval.FeatureTypeJSON)
	b := testWalker(t, starBytes)
	f, err := b.Build()
	require.NoError(t, err)
	require.NotNil(t, f)
	sv := &structpb.Value{}
	require.NoError(t, f.FeatureOld.Tree.Default.UnmarshalTo(sv))
	require.NotEmpty(t, sv.GetListValue().Values)

	sv.GetListValue().Values = append(sv.GetListValue().Values, structpb.NewStringValue("foobar"))
	f.FeatureOld.Tree.Default, err = b.toAny(sv)
	require.NoError(t, err)
	bytes, err := b.Mutate(f)
	require.NoError(t, err)
	assert.NotEqualValues(t, starBytes, bytes)
	assert.Contains(t, string(bytes), "foobar")
}

func TestWalkerProto(t *testing.T) {
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
			tpb = proto.package("testproto.v1beta1")
			result = feature(
				description = "proto feature",
				default = %s,
				rules = [("age > 1", %s)]
			)
			`, tc.starVal, tc.starVal))
			b := testWalker(t, star)
			f, err := b.Build()
			require.NoError(t, err)
			require.NotNil(t, f)
			require.Len(t, f.Imports, 2)
			checkEqualProtos(t, b, f.FeatureOld.Tree.Default, tc.expected)
			checkEqualProtos(t, b, f.FeatureOld.Tree.Constraints[0].Value, tc.expected)

			// Now that static parsing is done, try a no-op static mutation.
			result, err := b.Mutate(f)
			require.NoError(t, err)
			if !tc.suppress { // suppress some test cases because defaults are not supported yet.
				require.Contains(t, string(result), fmt.Sprintf("default = %s", tc.starVal))
				require.Contains(t, string(result), fmt.Sprintf("(\"age > 1\", %s)", tc.starVal))
			}
		})
	}
}

func checkEqualProtos(t *testing.T, b *walker, a *anypb.Any, expected proto.Message) {
	protoMessage, err := b.fromAnyDynamic(a)
	require.NoError(t, err)
	require.NotNil(t, protoMessage)
	require.True(t, proto.Equal(protoMessage, expected), "expected %v %T, got %v %T", expected, expected, protoMessage, protoMessage)
}

func TestWalkerBuildMultiline(t *testing.T) {
	star, err := os.ReadFile("testdata/device.star")
	require.NoError(t, err)

	w := testWalker(t, star)
	f, err := w.Build()
	require.NoError(t, err)
	require.NotNil(t, f)
	assert.True(t, f.GetFeature().GetDefault().GetMeta().GetMultiline())

	// Now that static parsing is done, try a no-op static mutation.
	result, err := w.Mutate(f)
	require.NoError(t, err)
	assert.EqualValues(t, string(star), string(result))
}

func TestWalkerNestedProto(t *testing.T) {
	star, err := os.ReadFile("testdata/nested.star")
	require.NoError(t, err)

	w := testWalker(t, star)
	f, err := w.Build()
	require.NoError(t, err)
	require.NotNil(t, f)

	newStar, err := w.Mutate(f)
	require.NoError(t, err)
	assert.EqualValues(t, string(star), string(newStar))
}

func TestWalkerRepeatedProto(t *testing.T) {
	star, err := os.ReadFile("testdata/repeated.star")
	require.NoError(t, err)

	w := testWalker(t, star)
	f, err := w.Build()
	require.NoError(t, err)
	require.NotNil(t, f)

	newStar, err := w.Mutate(f)
	require.NoError(t, err)
	assert.EqualValues(t, string(star), string(newStar))
}

func TestWalkerMapProto(t *testing.T) {
	star, err := os.ReadFile("testdata/map.star")
	require.NoError(t, err)

	w := testWalker(t, star)
	f, err := w.Build()
	require.NoError(t, err)
	require.NotNil(t, f)

	newStar, err := w.Mutate(f)
	require.NoError(t, err)
	assert.EqualValues(t, string(star), string(newStar))
}

func TestWalkerFormatCtxKey(t *testing.T) {
	star, err := os.ReadFile("testdata/ctxkey.star")
	require.NoError(t, err)

	w := testWalker(t, star)
	_, err = w.Build()
	require.Error(t, err) // periods in context keys not allowed
}
