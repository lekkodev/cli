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
	"testing"

	rulesv1beta3 "buf.build/gen/go/lekkodev/cli/protocolbuffers/go/lekko/rules/v1beta3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var testEvalCtxs []EvalContext = []EvalContext{
	{
		Namespace:   "ns_1",
		FeatureName: "feature_1",
	},
	{
		Namespace:   "ns_2",
		FeatureName: "feature_2",
	},
}

// NOTE: to test consistency of the hashing/bucketing algorithms cross-platform
// test cases (data and expected evaluation results) should be identical
func testBucket[K comparable](t *testing.T, testCases []map[K]bool) {
	for eIdx, evalCtx := range testEvalCtxs {
		for ctxValue, expected := range testCases[eIdx] {
			rule := NewV1Beta3(
				&rulesv1beta3.Rule{
					Rule: &rulesv1beta3.Rule_CallExpression{
						CallExpression: &rulesv1beta3.CallExpression{
							Function: &rulesv1beta3.CallExpression_Bucket_{
								// 50% bucketing
								Bucket: &rulesv1beta3.CallExpression_Bucket{
									ContextKey: "key",
									Threshold:  50000,
								},
							},
						},
					},
				},
				evalCtx,
			)

			featureCtx := map[string]interface{}{
				"key": ctxValue,
			}

			result, err := rule.EvaluateRule(featureCtx)

			require.NoError(t, err)
			assert.Equal(
				t,
				expected,
				result,
				"Unexpected bucket result for %v/%v/%v:%v: expected %v, got %v",
				evalCtx.Namespace,
				evalCtx.FeatureName,
				"key",
				ctxValue,
				expected,
				result,
			)
		}
	}
}

func TestBucketUnsupported(t *testing.T) {
	rule := NewV1Beta3(
		&rulesv1beta3.Rule{
			Rule: &rulesv1beta3.Rule_CallExpression{
				CallExpression: &rulesv1beta3.CallExpression{
					Function: &rulesv1beta3.CallExpression_Bucket_{
						Bucket: &rulesv1beta3.CallExpression_Bucket{
							ContextKey: "key",
							Threshold:  50000,
						},
					},
				},
			},
		},
		testEvalCtxs[0],
	)

	// Context with unsupported value type for bucket bool
	featureCtx := map[string]interface{}{
		"key": false,
	}

	_, err := rule.EvaluateRule(featureCtx)
	require.Error(t, err, "Expected error for unsupported type for bucketing")
}

func TestBucketInts(t *testing.T) {
	testCases := []map[int]bool{
		{
			1:   false,
			2:   false,
			3:   true,
			4:   false,
			5:   true,
			101: true,
			102: true,
			103: false,
			104: false,
			105: true,
		},
		{
			1:   false,
			2:   true,
			3:   false,
			4:   false,
			5:   true,
			101: true,
			102: true,
			103: false,
			104: true,
			105: true,
		},
	}
	testBucket[int](t, testCases)
}

func TestBucketDoubles(t *testing.T) {
	testCases := []map[float64]bool{
		{
			3.1415: false,
			2.7182: false,
			1.6180: true,
			6.6261: true,
			6.0221: false,
			2.9979: true,
			6.6730: false,
			1.3807: true,
			1.4142: true,
			2.0000: false,
		},
		{
			3.1415: true,
			2.7182: false,
			1.6180: true,
			6.6261: false,
			6.0221: false,
			2.9979: false,
			6.6730: false,
			1.3807: false,
			1.4142: true,
			2.0000: false,
		},
	}
	testBucket[float64](t, testCases)
}

func TestBucketStrings(t *testing.T) {
	testCases := []map[string]bool{
		{
			"hello":  false,
			"world":  false,
			"i":      true,
			"am":     true,
			"a":      true,
			"unit":   false,
			"test":   true,
			"case":   true,
			"for":    false,
			"bucket": false,
		},
		{
			"hello":  true,
			"world":  false,
			"i":      true,
			"am":     true,
			"a":      true,
			"unit":   false,
			"test":   true,
			"case":   false,
			"for":    false,
			"bucket": false,
		},
	}
	testBucket[string](t, testCases)
}
