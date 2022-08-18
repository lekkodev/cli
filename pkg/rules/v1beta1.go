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

	backendv1beta1 "github.com/lekkodev/cli/pkg/gen/proto/go/lekko/backend/v1beta1"
	rulesv1beta1 "github.com/lekkodev/cli/pkg/gen/proto/go/lekko/rules/v1beta1"
	"google.golang.org/protobuf/types/known/structpb"
)

func ContextHelper(genericMap map[string]interface{}) (map[string]*backendv1beta1.Value, error) {
	ret := make(map[string]*backendv1beta1.Value)
	for key, value := range genericMap {
		var backendVal *backendv1beta1.Value
		switch v := value.(type) {
		case bool:
			backendVal = &backendv1beta1.Value{Kind: &backendv1beta1.Value_BoolValue{BoolValue: v}}
		case int:
			backendVal = &backendv1beta1.Value{Kind: &backendv1beta1.Value_IntValue{IntValue: int64(v)}}
		case int32:
			backendVal = &backendv1beta1.Value{Kind: &backendv1beta1.Value_IntValue{IntValue: int64(v)}}
		case int64:
			backendVal = &backendv1beta1.Value{Kind: &backendv1beta1.Value_IntValue{IntValue: v}}
		case float64:
			backendVal = &backendv1beta1.Value{Kind: &backendv1beta1.Value_DoubleValue{DoubleValue: v}}
		case string:
			backendVal = &backendv1beta1.Value{Kind: &backendv1beta1.Value_StringValue{StringValue: v}}
		default:
			return nil, fmt.Errorf("found unknown context value: %v", v)
		}
		ret[key] = backendVal
	}
	return ret, nil
}

func EvaluateFeatureV1Beta1(feature *rulesv1beta1.Feature, context map[string]*backendv1beta1.Value) (*structpb.Value, error) {
	for _, constraint := range feature.GetConstraints() {
		isConditionTrue, err := conditionsEvaluatesToTrue(constraint.GetConditions(), constraint.GetConditionLinker(), context)
		if err != nil {
			return nil, err
		} else if isConditionTrue {
			return constraint.GetResultingValue(), nil
		}
	}
	return feature.GetDefaultValue(), nil
}

func conditionEvaluatesToTrue(condition *rulesv1beta1.Condition, contextV *backendv1beta1.Value) bool {
	switch v := condition.GetComparisonValue().GetKind().(type) {
	case *structpb.Value_NumberValue:
		switch contextVal := contextV.GetKind().(type) {
		case *backendv1beta1.Value_DoubleValue:
			switch condition.LogicalOperator {
			case rulesv1beta1.LogicalOperator_LOGICAL_OPERATOR_EQUALS:
				return v.NumberValue == contextVal.DoubleValue
			default:
			}
		case *backendv1beta1.Value_IntValue:
			switch condition.LogicalOperator {
			case rulesv1beta1.LogicalOperator_LOGICAL_OPERATOR_EQUALS:
				return int64(v.NumberValue) == contextVal.IntValue
			default:
			}
		}
	case *structpb.Value_StringValue:
		switch contextVal := contextV.GetKind().(type) {
		case *backendv1beta1.Value_StringValue:
			switch condition.LogicalOperator {
			case rulesv1beta1.LogicalOperator_LOGICAL_OPERATOR_EQUALS:
				return v.StringValue == contextVal.StringValue
			}
		}
	case *structpb.Value_BoolValue:
		switch contextVal := contextV.GetKind().(type) {
		case *backendv1beta1.Value_BoolValue:
			switch condition.LogicalOperator {
			case rulesv1beta1.LogicalOperator_LOGICAL_OPERATOR_EQUALS:
				return v.BoolValue == contextVal.BoolValue
			}
		}
	case *structpb.Value_ListValue:
		for _, value := range v.ListValue.Values {
			isEqual := conditionEvaluatesToTrue(&rulesv1beta1.Condition{ComparisonValue: value, LogicalOperator: rulesv1beta1.LogicalOperator_LOGICAL_OPERATOR_EQUALS, ContextKey: condition.ContextKey}, contextV)
			switch condition.LogicalOperator {
			case rulesv1beta1.LogicalOperator_LOGICAL_OPERATOR_CONTAINED_WITHIN:
				if isEqual {
					return true
				}

			case rulesv1beta1.LogicalOperator_LOGICAL_OPERATOR_NOT_CONTAINED_WITHIN:
				if isEqual {
					return false
				}
			}
		}
		// If we get to the end, and its not there, return true for NOT_CONTAINER, false for CONTAINED.
		return condition.LogicalOperator == rulesv1beta1.LogicalOperator_LOGICAL_OPERATOR_NOT_CONTAINED_WITHIN
	case *structpb.Value_StructValue:
	}
	return false
}

func conditionsEvaluatesToTrue(conditions []*rulesv1beta1.Condition, conditionLinker rulesv1beta1.ConditionLinker, context map[string]*backendv1beta1.Value) (bool, error) {
	for _, condition := range conditions {
		contextV, ok := context[condition.GetContextKey()]
		if !ok {
			// If the context is missing, just continue.
			continue
		}
		conditionResult := conditionEvaluatesToTrue(condition, contextV)
		// For False && OR and True && AND, continue. Otherwise return the result.
		if (!conditionResult && conditionLinker == rulesv1beta1.ConditionLinker_CONDITION_LINKER_OR) || (conditionResult && conditionLinker == rulesv1beta1.ConditionLinker_CONDITION_LINKER_AND) {
			continue
		}
		return conditionResult, nil
	}
	// If we have an AND and made it here, it's true. If its an OR, then nothing was true, so return false.
	return conditionLinker == rulesv1beta1.ConditionLinker_CONDITION_LINKER_AND, nil
}
