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

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/types/known/structpb"
)

func TestFeatureProtoRoundTripBool(t *testing.T) {
	f := NewBoolFeature(false)
	require.NoError(t, f.AddBoolOverride("foo", nil, true))
	proto, err := f.ToProto()
	require.NoError(t, err)
	require.NotNil(t, proto)
	newF, err := FromProto(proto, nil)
	require.NoError(t, err)
	require.NotNil(t, newF)
	assert.EqualValues(t, f, newF)
}

func TestFeatureProtoRoundTripString(t *testing.T) {
	f := NewStringFeature("a")
	require.NoError(t, f.AddStringOverride("foo", nil, "b"))
	proto, err := f.ToProto()
	require.NoError(t, err)
	require.NotNil(t, proto)
	newF, err := FromProto(proto, nil)
	require.NoError(t, err)
	require.NotNil(t, newF)
	assert.EqualValues(t, f, newF)
}

func TestFeatureProtoRoundTripInt(t *testing.T) {
	f := NewIntFeature(1)
	require.NoError(t, f.AddIntOverride("foo", nil, 2))
	proto, err := f.ToProto()
	require.NoError(t, err)
	require.NotNil(t, proto)
	newF, err := FromProto(proto, nil)
	require.NoError(t, err)
	require.NotNil(t, newF)
	assert.EqualValues(t, f, newF)
}

func TestFeatureProtoRoundTripFloat(t *testing.T) {
	f := NewFloatFeature(1.2)
	require.NoError(t, f.AddFloatOverride("foo", nil, 3.0))
	proto, err := f.ToProto()
	require.NoError(t, err)
	require.NotNil(t, proto)
	newF, err := FromProto(proto, nil)
	require.NoError(t, err)
	require.NotNil(t, newF)
	assert.EqualValues(t, f, newF)
}

func TestFeatureProtoRoundTripJSON(t *testing.T) {
	defaultVal, err := structpb.NewValue(map[string]interface{}{
		"a": []interface{}{1, 2, 3},
		"b": false,
	})
	require.NoError(t, err)
	f := NewJSONFeature(defaultVal)
	overrideVal, err := structpb.NewValue(map[string]interface{}{
		"a": 1,
	})
	require.NoError(t, err)
	require.NoError(t, f.AddJSONOverride("foo", nil, overrideVal))
	proto, err := f.ToProto()
	require.NoError(t, err)
	require.NotNil(t, proto)
	newF, err := FromProto(proto, nil)
	require.NoError(t, err)
	require.NotNil(t, newF)
	compareJSONFeatures(t, f, newF)
}

func compareStructVal(t *testing.T, expected, actual *structpb.Value) {
	expBytes, err := expected.MarshalJSON()
	require.NoError(t, err)
	actBytes, err := actual.MarshalJSON()
	require.NoError(t, err)
	require.Equal(t, expBytes, actBytes)
}

func compareJSONFeatures(t *testing.T, expected, actual *Feature) {
	if expected == nil || actual == nil {
		assert.Nil(t, expected)
		assert.Nil(t, actual)
		return
	}
	expDef, ok := expected.Value.(*structpb.Value)
	require.True(t, ok)
	require.NotNil(t, expDef)
	actDef, ok := actual.Value.(*structpb.Value)
	require.True(t, ok)
	require.NotNil(t, actDef)
	compareStructVal(t, expDef, actDef)

	require.Equal(t, len(expected.Overrides), len(actual.Overrides))
	for i, expOverride := range expected.Overrides {
		actOverride := actual.Overrides[i]
		assert.EqualValues(t, expOverride.RuleASTV3, actOverride.RuleASTV3)
		expVal, ok := expOverride.Value.(*structpb.Value)
		require.True(t, ok)
		require.NotNil(t, expVal)
		actVal, ok := actOverride.Value.(*structpb.Value)
		require.True(t, ok)
		require.NotNil(t, actVal)
		compareStructVal(t, expVal, actVal)
	}
}
