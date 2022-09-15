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

package rules_test

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/types/known/wrapperspb"

	"github.com/lekkodev/cli/pkg/fixtures"
	featurev1beta1 "github.com/lekkodev/cli/pkg/gen/proto/go/lekko/feature/v1beta1"
	"github.com/lekkodev/cli/pkg/rules"
)

func TestEvaluateFeatureBoolBeta2(t *testing.T) {
	t.Parallel()

	tcs := []struct {
		feature *featurev1beta1.Feature
		context map[string]interface{}
		testErr error
		testVal bool
	}{
		{
			fixtures.NewBasicFeatureOnBeta2(),
			nil,
			nil,
			true,
		},
		{
			fixtures.NewBasicFeatureOffBeta2(),
			nil,
			nil,
			false,
		},
		{
			fixtures.NewFeatureOnForUserIDBeta2(),
			map[string]interface{}{"user_id": interface{}(1)},
			nil,
			true,
		},
		{
			fixtures.NewFeatureOnForUserIDBeta2(),
			map[string]interface{}{"user_id": interface{}(2)},
			nil,
			false,
		},
		{
			fixtures.NewFeatureOnForUserIDsBeta2(),
			map[string]interface{}{"user_id": interface{}(2)},
			nil,
			true,
		},
		{
			fixtures.NewFeatureOnForUserIDBeta2(),
			map[string]interface{}{"user_id": interface{}(3)},
			nil,
			false,
		},
		{
			fixtures.NewFeatureInvalidBeta2(),
			map[string]interface{}{"user_id": interface{}(3)},
			fmt.Errorf(""),
			false,
		},
		{
			fixtures.NewFeatureInvalidBeta2(),
			nil,
			fmt.Errorf(""),
			false,
		},
	}

	for _, tc := range tcs {
		val, err := rules.EvaluateFeatureV1Beta3(tc.feature.Tree, tc.context)
		if tc.testErr != nil {
			require.Error(t, err)
		} else {
			require.NoError(t, err)
			var res wrapperspb.BoolValue
			err := val.UnmarshalTo(&res)
			require.NoError(t, err)
			require.Equal(t, tc.testVal, res.Value, "failed on test for %s", tc.feature.Key)
		}
	}
}
