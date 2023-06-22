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

package rules

import (
	"fmt"
	"testing"

	rulesv1beta3 "buf.build/gen/go/lekkodev/cli/protocolbuffers/go/lekko/rules/v1beta3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var testEvalCtx = EvalContext{
	Namespace:   "namespace",
	FeatureName: "feature",
}

func TestBoolConstV3(t *testing.T) {
	for _, b := range []bool{true, false} {
		t.Run(fmt.Sprintf("test bool %v", b), func(t *testing.T) {
			result, err := NewV1Beta3(
				&rulesv1beta3.Rule{
					Rule: &rulesv1beta3.Rule_BoolConst{
						BoolConst: b,
					},
				},
				testEvalCtx,
			).EvaluateRule(nil)
			require.NoError(t, err)
			assert.Equal(t, b, result)
		})
	}
}

func TestPresentV3(t *testing.T) {
	rule := NewV1Beta3(
		&rulesv1beta3.Rule{
			Rule: &rulesv1beta3.Rule_Atom{
				Atom: AgeV3("PRESENT", 0),
			},
		},
		testEvalCtx,
	)
	result, err := rule.EvaluateRule(nil)
	require.NoError(t, err)
	assert.False(t, result)

	result, err = rule.EvaluateRule(CtxBuilder().Age(12).B())
	require.NoError(t, err)
	assert.True(t, result)

	result, err = rule.EvaluateRule(CtxBuilder().Age("not a number").B())
	require.NoError(t, err)
	assert.True(t, result)
}

type AtomTestV3 struct {
	atom     *rulesv1beta3.Atom
	context  map[string]interface{}
	expected bool
	hasError bool
}

func testAtomV3(t *testing.T, idx int, tc AtomTestV3) {
	t.Run(fmt.Sprintf("test atom %d", idx), func(t *testing.T) {
		rule := NewV1Beta3(
			&rulesv1beta3.Rule{
				Rule: &rulesv1beta3.Rule_Atom{
					Atom: tc.atom,
				},
			},
			testEvalCtx,
		)
		result, err := rule.EvaluateRule(tc.context)
		if tc.hasError {
			require.Error(t, err)
			return
		} else {
			require.NoError(t, err)
		}
		assert.Equal(t, tc.expected, result)
	})
	t.Run(fmt.Sprintf("test not %d", idx), func(t *testing.T) {
		rule := NewV1Beta3(
			&rulesv1beta3.Rule{
				Rule: &rulesv1beta3.Rule_Not{
					Not: &rulesv1beta3.Rule{
						Rule: &rulesv1beta3.Rule_Atom{
							Atom: tc.atom,
						},
					},
				},
			},
			testEvalCtx,
		)
		result, err := rule.EvaluateRule(tc.context)
		if tc.hasError {
			require.Error(t, err)
			return
		} else {
			require.NoError(t, err)
		}
		assert.Equal(t, !tc.expected /* not */, result)
	})
}

func TestEqualsV3(t *testing.T) {
	for i, tc := range []AtomTestV3{
		{
			atom:     AgeEqualsV3(12),
			context:  CtxBuilder().Age(12).B(),
			expected: true,
		},
		{
			atom:     AgeEqualsV3(12),
			context:  CtxBuilder().Age(35).B(),
			expected: false,
		},
		{
			atom:     AgeEqualsV3(12),
			context:  CtxBuilder().Age(12 + 1e-10).B(),
			expected: false,
		},
		{
			atom:     AgeEqualsV3(12.001),
			context:  CtxBuilder().Age(12.001).B(),
			expected: true,
		},
		{
			atom:     AgeEqualsV3(12),
			context:  CtxBuilder().Age("not a number").B(),
			hasError: true,
		},
		{
			atom:     AgeNotEqualsV3(12),
			context:  CtxBuilder().Age(25).B(),
			expected: true,
		},
		{
			atom:     AgeNotEqualsV3(12),
			context:  CtxBuilder().Age(12).B(),
			expected: false,
		},
		{
			atom:     AgeEqualsV3(12),
			context:  nil, // not present
			expected: false,
		},
		{
			atom:     CityEqualsV3("Rome"),
			context:  CtxBuilder().City("Rome").B(),
			expected: true,
		},
		{
			atom:     CityEqualsV3("Rome"),
			context:  CtxBuilder().City("rome").B(),
			expected: false,
		},
		{
			atom:     CityEqualsV3("Rome"),
			context:  CtxBuilder().City("Paris").B(),
			expected: false,
		},
		{
			atom:     CityEqualsV3("Rome"),
			context:  CtxBuilder().City(99).B(),
			hasError: true,
		},
	} {
		testAtomV3(t, i, tc)
	}
}

func TestNumericalOperatorsV3(t *testing.T) {
	for i, tc := range []AtomTestV3{
		{
			atom:     AgeV3("<", 12),
			context:  CtxBuilder().Age(12).B(),
			expected: false,
		},
		{
			atom:     AgeV3("<", 12),
			context:  CtxBuilder().Age(11).B(),
			expected: true,
		},
		{
			atom:     AgeV3("<=", 12),
			context:  CtxBuilder().Age(12).B(),
			expected: true,
		},
		{
			atom:     AgeV3(">=", 12),
			context:  CtxBuilder().Age(12).B(),
			expected: true,
		},
		{
			atom:     AgeV3(">", 12),
			context:  CtxBuilder().Age(12).B(),
			expected: false,
		},
		{
			atom:     AgeV3(">", 12),
			context:  CtxBuilder().Age("string").B(),
			hasError: true,
		},
	} {
		testAtomV3(t, i, tc)
	}
}

func TestContainedWithinV3(t *testing.T) {
	for i, tc := range []AtomTestV3{
		{
			atom:     CityInV3("Rome", "Paris"),
			context:  CtxBuilder().City("London").B(),
			expected: false,
		},
		{
			atom:     CityInV3("Rome", "Paris"),
			context:  CtxBuilder().City("Rome").B(),
			expected: true,
		},
		{
			atom:     CityInV3("Rome", "Paris"),
			context:  CtxBuilder().City("London").B(),
			expected: false,
		},
		{
			atom:     CityInV3("Rome", "Paris"),
			context:  nil,
			expected: false,
		},
		{
			atom:     CityInV3("Rome", "Paris"),
			context:  CtxBuilder().City("rome").B(),
			expected: false,
		},
	} {
		testAtomV3(t, i, tc)
	}
}

func TestStringComparisonOperatorsV3(t *testing.T) {
	for i, tc := range []AtomTestV3{
		{
			atom:     CityV3("STARTS", "Ro"),
			context:  CtxBuilder().City("Rome").B(),
			expected: true,
		},
		{
			atom:     CityV3("STARTS", "Ro"),
			context:  CtxBuilder().City("London").B(),
			expected: false,
		},
		{
			atom:     CityV3("STARTS", "Ro"),
			expected: false,
		},
		{
			atom:     CityV3("STARTS", "Ro"),
			context:  CtxBuilder().City("rome").B(),
			expected: false,
		},
		{
			atom:     CityV3("ENDS", "me"),
			context:  CtxBuilder().City("Rome").B(),
			expected: true,
		},
		{
			atom:     CityV3("ENDS", "me"),
			context:  CtxBuilder().City("London").B(),
			expected: false,
		},
		{
			atom:     CityV3("CONTAINS", "Ro"),
			context:  CtxBuilder().City("Rome").B(),
			expected: true,
		},
		{
			atom:     CityV3("CONTAINS", ""),
			context:  CtxBuilder().City("Rome").B(),
			expected: true,
		},
		{
			atom:     CityV3("CONTAINS", "foo"),
			context:  CtxBuilder().City("Rome").B(),
			expected: false,
		},
	} {
		testAtomV3(t, i, tc)
	}
}

type LogicalExpressionTestV3 struct {
	rules    []*rulesv1beta3.Rule
	lo       rulesv1beta3.LogicalOperator
	context  map[string]interface{}
	expected bool
	hasError bool
}

func rules(atoms ...*rulesv1beta3.Atom) []*rulesv1beta3.Rule {
	var ret []*rulesv1beta3.Rule
	for _, a := range atoms {
		ret = append(ret, &rulesv1beta3.Rule{
			Rule: &rulesv1beta3.Rule_Atom{
				Atom: a,
			},
		})
	}
	return ret
}

func TestLogicalExpressionV3(t *testing.T) {
	for i, tc := range []LogicalExpressionTestV3{
		{
			rules:    rules(AgeV3("<", 5), AgeV3(">", 10)),
			lo:       rulesv1beta3.LogicalOperator_LOGICAL_OPERATOR_OR,
			context:  CtxBuilder().Age(8).B(),
			expected: false,
		},
		{
			rules:    rules(AgeV3("<", 5), AgeV3(">", 10)),
			lo:       rulesv1beta3.LogicalOperator_LOGICAL_OPERATOR_OR,
			context:  CtxBuilder().Age(12).B(),
			expected: true,
		},
		{
			rules:    rules(AgeV3("<", 5), CityEqualsV3("Rome")),
			lo:       rulesv1beta3.LogicalOperator_LOGICAL_OPERATOR_AND,
			context:  CtxBuilder().Age(8).City("Rome").B(),
			expected: false,
		},
		{
			rules:    rules(AgeV3("<", 5), CityEqualsV3("Rome")),
			lo:       rulesv1beta3.LogicalOperator_LOGICAL_OPERATOR_AND,
			context:  CtxBuilder().Age(3).City("Rome").B(),
			expected: true,
		},
		{
			rules:    rules(AgeV3("<", 5), CityEqualsV3("Rome")),
			lo:       rulesv1beta3.LogicalOperator_LOGICAL_OPERATOR_AND,
			context:  CtxBuilder().Age(3).B(),
			expected: false,
		},
		{
			rules:    rules(AgeV3("<", 5), CityEqualsV3("Rome")),
			lo:       rulesv1beta3.LogicalOperator_LOGICAL_OPERATOR_UNSPECIFIED,
			context:  CtxBuilder().Age(3).B(),
			hasError: true,
		},
		{ // Single atom in the n-ary tree, and
			rules:    rules(AgeV3("<", 5)),
			lo:       rulesv1beta3.LogicalOperator_LOGICAL_OPERATOR_AND,
			context:  CtxBuilder().Age(3).B(),
			expected: true,
		},
		{ // Single atom in the n-ary tree, or
			rules:    rules(AgeV3("<", 5)),
			lo:       rulesv1beta3.LogicalOperator_LOGICAL_OPERATOR_OR,
			context:  CtxBuilder().Age(3).B(),
			expected: true,
		},
		{ // No atoms in the n-ary tree
			rules:    rules(),
			lo:       rulesv1beta3.LogicalOperator_LOGICAL_OPERATOR_AND,
			context:  CtxBuilder().B(),
			hasError: true,
		},
		{
			rules:    rules(AgeV3("<", 5), CityEqualsV3("Rome"), AgeEqualsV3(8)),
			lo:       rulesv1beta3.LogicalOperator_LOGICAL_OPERATOR_AND,
			context:  CtxBuilder().Age(8).B(),
			expected: false,
		},
		{
			rules:    rules(AgeV3("<", 5), CityEqualsV3("Rome"), AgeEqualsV3(8)),
			lo:       rulesv1beta3.LogicalOperator_LOGICAL_OPERATOR_OR,
			context:  CtxBuilder().Age(8).B(),
			expected: true,
		},
	} {
		t.Run(fmt.Sprintf("test logical expression %d", i), func(t *testing.T) {
			rule := NewV1Beta3(
				&rulesv1beta3.Rule{
					Rule: &rulesv1beta3.Rule_LogicalExpression{
						LogicalExpression: &rulesv1beta3.LogicalExpression{
							Rules:           tc.rules,
							LogicalOperator: tc.lo,
						},
					},
				},
				testEvalCtx,
			)
			result, err := rule.EvaluateRule(tc.context)
			if tc.hasError {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
			assert.Equal(t, tc.expected, result)
		})
	}
}
