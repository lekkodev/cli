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

package fixtures

import (
	"google.golang.org/protobuf/types/known/anypb"
	"google.golang.org/protobuf/types/known/structpb"
	"google.golang.org/protobuf/types/known/wrapperspb"

	featurev1beta1 "github.com/lekkodev/cli/pkg/gen/proto/go/lekko/feature/v1beta1"
	rulesv1beta1 "github.com/lekkodev/cli/pkg/gen/proto/go/lekko/rules/v1beta1"
)

func NewBasicFeatureOn() *rulesv1beta1.Feature {
	return &rulesv1beta1.Feature{
		Name:         "basic_feature_on",
		Type:         rulesv1beta1.Type_TYPE_BOOL,
		DefaultValue: structpb.NewBoolValue(true),
		Constraints:  nil,
	}
}

func NewBasicFeatureOff() *rulesv1beta1.Feature {
	return &rulesv1beta1.Feature{
		Name:         "basic_feature_off",
		Type:         rulesv1beta1.Type_TYPE_BOOL,
		DefaultValue: structpb.NewBoolValue(false),
		Constraints:  nil,
	}
}

func NewFeatureOnForUserID() *rulesv1beta1.Feature {
	return &rulesv1beta1.Feature{
		Name:         "feature_on_for_user_id",
		Type:         rulesv1beta1.Type_TYPE_BOOL,
		DefaultValue: structpb.NewBoolValue(false),
		Constraints:  []*rulesv1beta1.Constraint{NewConstraintOnForUserID()},
	}
}

func NewFeatureOnForUserIDs() *rulesv1beta1.Feature {
	return &rulesv1beta1.Feature{
		Name:         "feature_on_for_user_ids",
		Type:         rulesv1beta1.Type_TYPE_BOOL,
		DefaultValue: structpb.NewBoolValue(false),
		Constraints:  []*rulesv1beta1.Constraint{NewConstraintOnForUserIDs()},
	}
}

func NewConstraintOnForUserID() *rulesv1beta1.Constraint {
	return &rulesv1beta1.Constraint{
		Conditions:     []*rulesv1beta1.Condition{NewConditionEqualUserID()},
		ResultingValue: structpb.NewBoolValue(true),
	}
}

func NewConstraintOnForUserIDs() *rulesv1beta1.Constraint {
	return &rulesv1beta1.Constraint{
		Conditions:     []*rulesv1beta1.Condition{NewConditionContainsUserID()},
		ResultingValue: structpb.NewBoolValue(true),
	}
}

func NewConditionEqualUserID() *rulesv1beta1.Condition {
	return &rulesv1beta1.Condition{
		ContextKey:      "user_id",
		ComparisonValue: structpb.NewNumberValue(float64(1)),
		LogicalOperator: rulesv1beta1.LogicalOperator_LOGICAL_OPERATOR_EQUALS,
	}
}

func NewConditionContainsUserID() *rulesv1beta1.Condition {
	list, err := structpb.NewList([]interface{}{float64(1), float64(2)})
	if err != nil {
		panic(err)
	}
	return &rulesv1beta1.Condition{
		ContextKey:      "user_id",
		ComparisonValue: structpb.NewListValue(list),
		LogicalOperator: rulesv1beta1.LogicalOperator_LOGICAL_OPERATOR_CONTAINED_WITHIN,
	}
}

func NewBasicFeatureOnBeta2() *featurev1beta1.Feature {
	return &featurev1beta1.Feature{
		Key: "basic_feature_on",
		Tree: &featurev1beta1.Tree{
			Default:     NewAnyTrue(),
			Constraints: nil,
		},
	}
}

func NewBasicFeatureOffBeta2() *featurev1beta1.Feature {
	return &featurev1beta1.Feature{
		Key: "basic_feature_off",
		Tree: &featurev1beta1.Tree{
			Default:     NewAnyFalse(),
			Constraints: nil,
		},
	}
}

func NewFeatureOnForUserIDBeta2() *featurev1beta1.Feature {
	return &featurev1beta1.Feature{
		Key: "feature_on_for_user_id",
		Tree: &featurev1beta1.Tree{
			Default:     NewAnyFalse(),
			Constraints: []*featurev1beta1.Constraint{NewConstraintOnForUserIDBeta2()},
		},
	}
}

func NewFeatureOnForUserIDsBeta2() *featurev1beta1.Feature {
	return &featurev1beta1.Feature{
		Key: "feature_on_for_user_ids",
		Tree: &featurev1beta1.Tree{
			Default:     NewAnyFalse(),
			Constraints: []*featurev1beta1.Constraint{NewConstraintOnForUserIDsBeta2()},
		},
	}
}

func NewFeatureInvalidBeta2() *featurev1beta1.Feature {
	return &featurev1beta1.Feature{
		Key: "feature_on_for_user_ids",
		Tree: &featurev1beta1.Tree{
			Default:     NewAnyFalse(),
			Constraints: []*featurev1beta1.Constraint{NewConstraintInvalidBeta2()},
		},
	}
}

func NewConstraintOnForUserIDBeta2() *featurev1beta1.Constraint {
	return &featurev1beta1.Constraint{
		Rule:  NewRuleLangEqualUserID(),
		Value: NewAnyTrue(),
	}
}

func NewConstraintOnForUserIDsBeta2() *featurev1beta1.Constraint {
	return &featurev1beta1.Constraint{
		Rule:  NewRuleLangContainsUserID(),
		Value: NewAnyTrue(),
	}
}

func NewConstraintInvalidBeta2() *featurev1beta1.Constraint {
	return &featurev1beta1.Constraint{
		Rule:  NewRuleLangInvalid(),
		Value: NewAnyTrue(),
	}
}

func NewAnyFalse() *anypb.Any {
	a, err := anypb.New(&wrapperspb.BoolValue{Value: false})
	if err != nil {
		panic(err)
	}
	return a
}

func NewAnyTrue() *anypb.Any {
	a, err := anypb.New(&wrapperspb.BoolValue{Value: true})
	if err != nil {
		panic(err)
	}
	return a
}

func NewAnyInt(i int64) *anypb.Any {
	a, err := anypb.New(&wrapperspb.Int64Value{Value: i})
	if err != nil {
		panic(err)
	}
	return a
}

func NewRuleLangEqualUserID() string {
	return "user_id == 1"
}

func NewRuleLangContainsUserID() string {
	return "user_id IN [1, 2]"
}

func NewRuleLangInvalid() string {
	return "user_id IN (1, 2)"
}

func NewComplexTreeFeature() *featurev1beta1.Feature {
	return &featurev1beta1.Feature{
		Key: "complex-tree",
		Tree: &featurev1beta1.Tree{
			Default: NewAnyInt(12),
			Constraints: []*featurev1beta1.Constraint{
				{Rule: "a == 1", Value: NewAnyInt(38), Constraints: []*featurev1beta1.Constraint{
					{Rule: "x IN [\"a\", \"b\"]", Value: NewAnyInt(108)},
				}},
				{Rule: "a > 10", Value: nil, Constraints: []*featurev1beta1.Constraint{
					{Rule: "x == \"c\"", Value: NewAnyInt(21)},
				}},
				{Rule: "a > 5", Value: NewAnyInt(23)},
			},
		},
	}
}
