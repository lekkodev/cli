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

	rulesv1beta2 "buf.build/gen/go/lekkodev/cli/protocolbuffers/go/lekko/rules/v1beta2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBoolConst(t *testing.T) {
	for _, b := range []bool{true, false} {
		t.Run(fmt.Sprintf("test bool %v", b), func(t *testing.T) {
			result, err := NewV1Beta2(&rulesv1beta2.Rule{
				Rule: &rulesv1beta2.Rule_BoolConst{
					BoolConst: b,
				},
			}).EvaluateRule(nil)
			require.NoError(t, err)
			assert.Equal(t, b, result)
		})
	}
}

func TestPresent(t *testing.T) {
	rule := NewV1Beta2(&rulesv1beta2.Rule{
		Rule: &rulesv1beta2.Rule_Atom{
			Atom: Age("PRESENT", 0),
		},
	})
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

type AtomTest struct {
	atom     *rulesv1beta2.Atom
	context  map[string]interface{}
	expected bool
	hasError bool
}

func testAtom(t *testing.T, idx int, tc AtomTest) {
	t.Run(fmt.Sprintf("test atom %d", idx), func(t *testing.T) {
		rule := NewV1Beta2(&rulesv1beta2.Rule{
			Rule: &rulesv1beta2.Rule_Atom{
				Atom: tc.atom,
			},
		})
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
		rule := NewV1Beta2(&rulesv1beta2.Rule{
			Rule: &rulesv1beta2.Rule_Not{
				Not: &rulesv1beta2.Rule{
					Rule: &rulesv1beta2.Rule_Atom{
						Atom: tc.atom,
					},
				},
			},
		})
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

func TestEquals(t *testing.T) {
	for i, tc := range []AtomTest{
		{
			atom:     AgeEquals(12),
			context:  CtxBuilder().Age(12).B(),
			expected: true,
		},
		{
			atom:     AgeEquals(12),
			context:  CtxBuilder().Age(35).B(),
			expected: false,
		},
		{
			atom:     AgeEquals(12),
			context:  CtxBuilder().Age(12 + 1e-10).B(),
			expected: false,
		},
		{
			atom:     AgeEquals(12.001),
			context:  CtxBuilder().Age(12.001).B(),
			expected: true,
		},
		{
			atom:     AgeEquals(12),
			context:  CtxBuilder().Age("not a number").B(),
			hasError: true,
		},
		{
			atom:     AgeEquals(12),
			context:  nil, // not present
			expected: false,
		},
		{
			atom:     CityEquals("Rome"),
			context:  CtxBuilder().City("Rome").B(),
			expected: true,
		},
		{
			atom:     CityEquals("Rome"),
			context:  CtxBuilder().City("rome").B(),
			expected: false,
		},
		{
			atom:     CityEquals("Rome"),
			context:  CtxBuilder().City("Paris").B(),
			expected: false,
		},
		{
			atom:     CityEquals("Rome"),
			context:  CtxBuilder().City(99).B(),
			hasError: true,
		},
	} {
		testAtom(t, i, tc)
	}
}

func TestNumericalOperators(t *testing.T) {
	for i, tc := range []AtomTest{
		{
			atom:     Age("<", 12),
			context:  CtxBuilder().Age(12).B(),
			expected: false,
		},
		{
			atom:     Age("<", 12),
			context:  CtxBuilder().Age(11).B(),
			expected: true,
		},
		{
			atom:     Age("<=", 12),
			context:  CtxBuilder().Age(12).B(),
			expected: true,
		},
		{
			atom:     Age(">=", 12),
			context:  CtxBuilder().Age(12).B(),
			expected: true,
		},
		{
			atom:     Age(">", 12),
			context:  CtxBuilder().Age(12).B(),
			expected: false,
		},
		{
			atom:     Age(">", 12),
			context:  CtxBuilder().Age("string").B(),
			hasError: true,
		},
	} {
		testAtom(t, i, tc)
	}
}

func TestContainedWithin(t *testing.T) {
	for i, tc := range []AtomTest{
		{
			atom:     CityIn("Rome", "Paris"),
			context:  CtxBuilder().City("London").B(),
			expected: false,
		},
		{
			atom:     CityIn("Rome", "Paris"),
			context:  CtxBuilder().City("Rome").B(),
			expected: true,
		},
		{
			atom:     CityIn("Rome", "Paris"),
			context:  CtxBuilder().City("London").B(),
			expected: false,
		},
		{
			atom:     CityIn("Rome", "Paris"),
			context:  nil,
			expected: false,
		},
		{
			atom:     CityIn("Rome", "Paris"),
			context:  CtxBuilder().City("rome").B(),
			expected: false,
		},
	} {
		testAtom(t, i, tc)
	}
}

func TestStringComparisonOperators(t *testing.T) {
	for i, tc := range []AtomTest{
		{
			atom:     City("STARTS", "Ro"),
			context:  CtxBuilder().City("Rome").B(),
			expected: true,
		},
		{
			atom:     City("STARTS", "Ro"),
			context:  CtxBuilder().City("London").B(),
			expected: false,
		},
		{
			atom:     City("STARTS", "Ro"),
			expected: false,
		},
		{
			atom:     City("STARTS", "Ro"),
			context:  CtxBuilder().City("rome").B(),
			expected: false,
		},
		{
			atom:     City("ENDS", "me"),
			context:  CtxBuilder().City("Rome").B(),
			expected: true,
		},
		{
			atom:     City("ENDS", "me"),
			context:  CtxBuilder().City("London").B(),
			expected: false,
		},
		{
			atom:     City("CONTAINS", "Ro"),
			context:  CtxBuilder().City("Rome").B(),
			expected: true,
		},
		{
			atom:     City("CONTAINS", ""),
			context:  CtxBuilder().City("Rome").B(),
			expected: true,
		},
		{
			atom:     City("CONTAINS", "foo"),
			context:  CtxBuilder().City("Rome").B(),
			expected: false,
		},
	} {
		testAtom(t, i, tc)
	}
}

type LogicalExpressionTest struct {
	first    *rulesv1beta2.Atom
	lo       rulesv1beta2.LogicalOperator
	second   *rulesv1beta2.Atom
	context  map[string]interface{}
	expected bool
	hasError bool
}

func TestLogicalExpression(t *testing.T) {
	for i, tc := range []LogicalExpressionTest{
		{
			first:    Age("<", 5),
			lo:       rulesv1beta2.LogicalOperator_LOGICAL_OPERATOR_OR,
			second:   Age(">", 10),
			context:  CtxBuilder().Age(8).B(),
			expected: false,
		},
		{
			first:    Age("<", 5),
			lo:       rulesv1beta2.LogicalOperator_LOGICAL_OPERATOR_OR,
			second:   Age(">", 10),
			context:  CtxBuilder().Age(12).B(),
			expected: true,
		},
		{
			first:    Age("<", 5),
			lo:       rulesv1beta2.LogicalOperator_LOGICAL_OPERATOR_AND,
			second:   CityEquals("Rome"),
			context:  CtxBuilder().Age(8).City("Rome").B(),
			expected: false,
		},
		{
			first:    Age("<", 5),
			lo:       rulesv1beta2.LogicalOperator_LOGICAL_OPERATOR_AND,
			second:   CityEquals("Rome"),
			context:  CtxBuilder().Age(3).City("Rome").B(),
			expected: true,
		},
		{
			first:    Age("<", 5),
			lo:       rulesv1beta2.LogicalOperator_LOGICAL_OPERATOR_AND,
			second:   CityEquals("Rome"),
			context:  CtxBuilder().Age(3).B(),
			expected: false,
		},
		{
			first:    Age("<", 5),
			lo:       rulesv1beta2.LogicalOperator_LOGICAL_OPERATOR_UNSPECIFIED,
			second:   CityEquals("Rome"),
			context:  CtxBuilder().Age(3).B(),
			hasError: true,
		},
	} {
		t.Run(fmt.Sprintf("test logical expression %d", i), func(t *testing.T) {
			rule := NewV1Beta2(&rulesv1beta2.Rule{
				Rule: &rulesv1beta2.Rule_LogicalExpression{
					LogicalExpression: &rulesv1beta2.LogicalExpression{
						FirstRule: &rulesv1beta2.Rule{
							Rule: &rulesv1beta2.Rule_Atom{
								Atom: tc.first,
							},
						},
						SecondRule: &rulesv1beta2.Rule{
							Rule: &rulesv1beta2.Rule_Atom{
								Atom: tc.second,
							},
						},
						LogicalOperator: tc.lo,
					},
				},
			})
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
