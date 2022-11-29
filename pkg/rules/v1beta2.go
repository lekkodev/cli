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
	"strings"

	rulesv1beta2 "github.com/lekkodev/cli/pkg/gen/proto/go/lekko/rules/v1beta2"
	"github.com/pkg/errors"
	"google.golang.org/protobuf/types/known/structpb"
)

var (
	ErrEmptyRule                error = fmt.Errorf("empty rule")
	ErrUnknownLogicalOperator   error = fmt.Errorf("unknown logical operator")
	ErrEmptyRuleComparisonValue error = fmt.Errorf("empty rule comparison value")
	ErrMismatchedType           error = fmt.Errorf("mismatched context type")
	ErrUnsupportedType          error = fmt.Errorf("unsupported type")
)

type v1beta2 struct {
	rule *rulesv1beta2.Rule
}

// Represents the rules defined in the proto package 'lekko.rules.v1beta2'.
func NewV1Beta2(rule *rulesv1beta2.Rule) *v1beta2 {
	return &v1beta2{rule: rule}
}

func (v1b2 *v1beta2) EvaluateRule(context map[string]interface{}) (bool, error) {
	return v1b2.evaluateRule(v1b2.rule, context)
}

func (v1b2 *v1beta2) evaluateRule(rule *rulesv1beta2.Rule, context map[string]interface{}) (bool, error) {
	if rule == nil {
		return false, ErrEmptyRule
	}
	switch r := rule.Rule.(type) {
	case *rulesv1beta2.Rule_BoolConst:
		return r.BoolConst, nil
	case *rulesv1beta2.Rule_Not:
		innerPasses, err := v1b2.evaluateRule(r.Not, context)
		if err != nil {
			return false, errors.Wrap(err, "not: ")
		}
		return !innerPasses, nil
	case *rulesv1beta2.Rule_LogicalExpression:
		firstPasses, err := v1b2.evaluateRule(r.LogicalExpression.GetFirstRule(), context)
		if err != nil {
			return false, errors.Wrap(err, "first: ")
		}
		secondPasses, err := v1b2.evaluateRule(r.LogicalExpression.GetSecondRule(), context)
		if err != nil {
			return false, errors.Wrap(err, "second: ")
		}
		switch r.LogicalExpression.LogicalOperator {
		case rulesv1beta2.LogicalOperator_LOGICAL_OPERATOR_AND:
			return firstPasses && secondPasses, nil
		case rulesv1beta2.LogicalOperator_LOGICAL_OPERATOR_OR:
			return firstPasses || secondPasses, nil
		default:
			return false, errors.Wrapf(ErrUnknownLogicalOperator, "%s", r.LogicalExpression.GetLogicalOperator().String())
		}
	case *rulesv1beta2.Rule_Atom:
		contextKey := r.Atom.GetContextKey()
		runtimeCtxVal, present := context[contextKey]
		if r.Atom.ComparisonOperator == rulesv1beta2.ComparisonOperator_COMPARISON_OPERATOR_PRESENT {
			return present, nil
		}
		if r.Atom.ComparisonValue == nil {
			return false, errors.Wrapf(ErrEmptyRuleComparisonValue, "%s", r.Atom.String())
		}
		if !present {
			// All other comparison operators expect the context key to be present. If
			// it is not present, return false.
			return false, nil
		}
		switch r.Atom.ComparisonOperator {
		case rulesv1beta2.ComparisonOperator_COMPARISON_OPERATOR_EQUALS:
			return v1b2.evaluateEquals(r.Atom.GetComparisonValue(), runtimeCtxVal)
		case rulesv1beta2.ComparisonOperator_COMPARISON_OPERATOR_LESS_THAN:
			return v1b2.evaluateNumberComparator(r.Atom.ComparisonOperator, r.Atom.GetComparisonValue(), runtimeCtxVal)
		case rulesv1beta2.ComparisonOperator_COMPARISON_OPERATOR_LESS_THAN_OR_EQUALS:
			return v1b2.evaluateNumberComparator(r.Atom.ComparisonOperator, r.Atom.GetComparisonValue(), runtimeCtxVal)
		case rulesv1beta2.ComparisonOperator_COMPARISON_OPERATOR_GREATER_THAN:
			return v1b2.evaluateNumberComparator(r.Atom.ComparisonOperator, r.Atom.GetComparisonValue(), runtimeCtxVal)
		case rulesv1beta2.ComparisonOperator_COMPARISON_OPERATOR_GREATER_THAN_OR_EQUALS:
			return v1b2.evaluateNumberComparator(r.Atom.ComparisonOperator, r.Atom.GetComparisonValue(), runtimeCtxVal)
		case rulesv1beta2.ComparisonOperator_COMPARISON_OPERATOR_CONTAINED_WITHIN:
			return v1b2.evaluateContainedWithin(r.Atom.GetComparisonValue(), runtimeCtxVal)
		case rulesv1beta2.ComparisonOperator_COMPARISON_OPERATOR_STARTS_WITH:
			return v1b2.evaluateStringComparator(r.Atom.ComparisonOperator, r.Atom.GetComparisonValue(), runtimeCtxVal)
		case rulesv1beta2.ComparisonOperator_COMPARISON_OPERATOR_ENDS_WITH:
			return v1b2.evaluateStringComparator(r.Atom.ComparisonOperator, r.Atom.GetComparisonValue(), runtimeCtxVal)
		case rulesv1beta2.ComparisonOperator_COMPARISON_OPERATOR_CONTAINS:
			return v1b2.evaluateStringComparator(r.Atom.ComparisonOperator, r.Atom.GetComparisonValue(), runtimeCtxVal)
		}
	}
	return false, errors.Errorf("unknown rule type %T", rule.Rule)
}

// Only accepts bool, number or string ruleVal.
func (v1b2 *v1beta2) evaluateEquals(ruleVal *structpb.Value, contextVal interface{}) (bool, error) {
	switch typed := ruleVal.Kind.(type) {
	case *structpb.Value_BoolValue:
		boolVal, ok := contextVal.(bool)
		if !ok {
			return false, errMismatchedType(contextVal, "bool")
		}
		return typed.BoolValue == boolVal, nil
	case *structpb.Value_NumberValue:
		numVal, err := getNumber(contextVal)
		if err != nil {
			return false, err
		}
		return typed.NumberValue == numVal, nil
	case *structpb.Value_StringValue:
		stringVal, ok := contextVal.(string)
		if !ok {
			return false, errMismatchedType(contextVal, "string")
		}
		return typed.StringValue == stringVal, nil
	default:
		return false, errUnsupportedType(rulesv1beta2.ComparisonOperator_COMPARISON_OPERATOR_EQUALS, ruleVal)
	}
}

func (v1b2 *v1beta2) evaluateNumberComparator(co rulesv1beta2.ComparisonOperator, ruleVal *structpb.Value, contextVal interface{}) (bool, error) {
	numVal, err := getNumber(contextVal)
	if err != nil {
		return false, err
	}
	typedNumVal, ok := ruleVal.Kind.(*structpb.Value_NumberValue)
	if !ok {
		return false, errUnsupportedType(co, ruleVal)
	}
	ruleNumVal := typedNumVal.NumberValue
	switch co {
	case rulesv1beta2.ComparisonOperator_COMPARISON_OPERATOR_LESS_THAN:
		return numVal < ruleNumVal, nil
	case rulesv1beta2.ComparisonOperator_COMPARISON_OPERATOR_LESS_THAN_OR_EQUALS:
		return numVal < ruleNumVal || numVal == ruleNumVal, nil
	case rulesv1beta2.ComparisonOperator_COMPARISON_OPERATOR_GREATER_THAN:
		return numVal > ruleNumVal, nil
	case rulesv1beta2.ComparisonOperator_COMPARISON_OPERATOR_GREATER_THAN_OR_EQUALS:
		return numVal > ruleNumVal || numVal == ruleNumVal, nil
	default:
		return false, errors.Errorf("expected numerical comparison operator, got %v", co)
	}
}

func (v1b2 *v1beta2) evaluateContainedWithin(ruleVal *structpb.Value, contextVal interface{}) (bool, error) {
	listRuleVal, ok := ruleVal.Kind.(*structpb.Value_ListValue)
	if !ok {
		return false, errUnsupportedType(rulesv1beta2.ComparisonOperator_COMPARISON_OPERATOR_CONTAINED_WITHIN, ruleVal)
	}
	for _, elem := range listRuleVal.ListValue.Values {
		result, err := v1b2.evaluateEquals(elem, contextVal)
		if err != nil || !result {
			continue // no match, check next element
		}
		return true, nil
	}
	return false, nil
}

func (v1b2 *v1beta2) evaluateStringComparator(co rulesv1beta2.ComparisonOperator, ruleVal *structpb.Value, contextVal interface{}) (bool, error) {
	stringVal, ok := contextVal.(string)
	if !ok {
		return false, errMismatchedType(contextVal, "string")
	}
	typedRuleVal, ok := ruleVal.Kind.(*structpb.Value_StringValue)
	if !ok {
		return false, errUnsupportedType(co, ruleVal)
	}
	ruleStringVal := typedRuleVal.StringValue
	switch co {
	case rulesv1beta2.ComparisonOperator_COMPARISON_OPERATOR_STARTS_WITH:
		return strings.HasPrefix(stringVal, ruleStringVal), nil
	case rulesv1beta2.ComparisonOperator_COMPARISON_OPERATOR_ENDS_WITH:
		return strings.HasSuffix(stringVal, ruleStringVal), nil
	case rulesv1beta2.ComparisonOperator_COMPARISON_OPERATOR_CONTAINS:
		return strings.Contains(stringVal, ruleStringVal), nil
	default:
		return false, errors.Errorf("expected string comparison operator, got %v", co)
	}
}

func getNumber(val interface{}) (float64, error) {
	switch typed := val.(type) {
	case int:
		return float64(typed), nil
	case int8:
		return float64(typed), nil
	case int16:
		return float64(typed), nil
	case int32:
		return float64(typed), nil
	case int64:
		return float64(typed), nil
	case uint:
		return float64(typed), nil
	case uint8:
		return float64(typed), nil
	case uint16:
		return float64(typed), nil
	case uint32:
		return float64(typed), nil
	case uint64:
		return float64(typed), nil
	case float32:
		return float64(typed), nil
	case float64:
		return typed, nil
	default:
		return 0, errMismatchedType(val, "int", "float")
	}
}

func errMismatchedType(actual interface{}, expected ...string) error {
	return errors.Wrapf(ErrMismatchedType, "expected %v, got %T", strings.Join(expected, ","), actual)
}

func errUnsupportedType(co rulesv1beta2.ComparisonOperator, actual *structpb.Value) error {
	return errors.Wrapf(ErrUnsupportedType, "rule type %T for comparison operator %s", actual.GetKind(), co.String())
}
