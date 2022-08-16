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
	"testing"

	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/types/known/structpb"

	"github.com/lekkodev/cli/internal/fixtures"
	backendv1beta1 "github.com/lekkodev/cli/pkg/gen/proto/go/lekko/backend/v1beta1"
	rulesv1beta1 "github.com/lekkodev/cli/pkg/gen/proto/go/lekko/rules/v1beta1"
	"github.com/lekkodev/cli/pkg/rules"
)

func TestEvaluateFeatureBool(t *testing.T) {
	t.Parallel()

	tcs := []struct {
		feature *rulesv1beta1.Feature
		context map[string]*backendv1beta1.Value
		testErr error
		testVal bool
	}{
		{
			fixtures.NewBasicFeatureOn(),
			nil,
			nil,
			true,
		},
		{
			fixtures.NewBasicFeatureOff(),
			nil,
			nil,
			false,
		},
		{
			fixtures.NewFeatureOnForUserID(),
			map[string]*backendv1beta1.Value{"user_id": {Kind: &backendv1beta1.Value_IntValue{IntValue: 1}}},
			nil,
			true,
		},
		{
			fixtures.NewFeatureOnForUserID(),
			map[string]*backendv1beta1.Value{"user_id": {Kind: &backendv1beta1.Value_IntValue{IntValue: 2}}},
			nil,
			false,
		},
		{
			fixtures.NewFeatureOnForUserIDs(),
			map[string]*backendv1beta1.Value{"user_id": {Kind: &backendv1beta1.Value_IntValue{IntValue: 2}}},
			nil,
			true,
		},
		{
			fixtures.NewFeatureOnForUserID(),
			map[string]*backendv1beta1.Value{"user_id": {Kind: &backendv1beta1.Value_IntValue{IntValue: 3}}},
			nil,
			false,
		},
	}

	for _, tc := range tcs {
		val, err := rules.EvaluateFeatureV1Beta1(tc.feature, tc.context)
		if tc.testErr != nil {
			require.Error(t, err)
		} else {
			require.NoError(t, err)
			res, ok := val.GetKind().(*structpb.Value_BoolValue)
			require.True(t, ok)
			require.Equal(t, tc.testVal, res.BoolValue)
		}
	}
}
