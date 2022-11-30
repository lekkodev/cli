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
	"testing"

	"github.com/lekkodev/cli/pkg/fixtures"
	featurev1beta4 "github.com/lekkodev/cli/pkg/gen/proto/go/lekko/feature/v1beta4"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/types/known/wrapperspb"
)

func TestEvaluateFeatureBoolV1Beta4(t *testing.T) {
	t.Parallel()

	tcs := []struct {
		feature  *featurev1beta4.Feature
		context  map[string]interface{}
		testErr  error
		testVal  bool
		testPath []int
	}{
		{
			fixtures.NewBasicFeatureOnV1Beta4(),
			nil,
			nil,
			true,
			[]int{},
		},
		{
			fixtures.NewBasicFeatureOffV1Beta4(),
			nil,
			nil,
			false,
			[]int{},
		},
		{
			fixtures.NewFeatureOnForUserIDV1Beta4(),
			map[string]interface{}{"user_id": interface{}(1)},
			nil,
			true,
			[]int{0},
		},
		{
			fixtures.NewFeatureOnForUserIDV1Beta4(),
			map[string]interface{}{"user_id": interface{}(2)},
			nil,
			false,
			[]int{},
		},
		{
			fixtures.NewFeatureOnForUserIDsV1Beta4(),
			map[string]interface{}{"user_id": interface{}(2)},
			nil,
			true,
			[]int{0},
		},
		{
			fixtures.NewFeatureOnForUserIDV1Beta4(),
			map[string]interface{}{"user_id": interface{}(3)},
			nil,
			false,
			[]int{},
		},
	}

	for i, tc := range tcs {
		val, path, err := NewV1Beta4(tc.feature).Evaluate(tc.context)
		if tc.testErr != nil {
			require.Error(t, err)
		} else {
			require.NoError(t, err)
			var res wrapperspb.BoolValue
			err := val.UnmarshalTo(&res)
			require.NoError(t, err)
			require.Equal(t, tc.testVal, res.Value, "failed on test %d for %s", i, tc.feature.Key)
			require.EqualValues(t, tc.testPath, path, "expected equal paths")
		}
	}
}

// The following tests mimic the ones described in ./pkg/feature/README.md
func TestEvaluateFeatureComplexV1Beta4(t *testing.T) {
	t.Parallel()
	complexFeature := fixtures.NewComplexTreeFeatureV1Beta4()
	tcs := []struct {
		context  map[string]interface{}
		testVal  int64
		testPath []int
	}{
		{
			nil,
			12, []int{},
		},
		{
			map[string]interface{}{"a": 1},
			38, []int{0},
		},
		{
			map[string]interface{}{"a": 11},
			12, []int{},
		},
		{
			map[string]interface{}{"a": 11, "x": "c"},
			21, []int{1, 0},
		},
		{
			map[string]interface{}{"a": 8},
			23, []int{2},
		},
	}

	for i, tc := range tcs {
		val, path, err := NewV1Beta4(complexFeature).Evaluate(tc.context)
		require.NoError(t, err)
		var res wrapperspb.Int64Value
		require.NoError(t, val.UnmarshalTo(&res))
		require.Equal(t, tc.testVal, res.Value, "failed on test %d for %s", i, complexFeature.Key)
		require.EqualValues(t, tc.testPath, path, "expected equal paths")
	}
}
