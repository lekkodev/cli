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
	"strings"

	rulesv1beta3 "github.com/lekkodev/cli/pkg/gen/proto/go/lekko/rules/v1beta3"
	"github.com/pkg/errors"
	"google.golang.org/protobuf/types/known/structpb"
)

// v1beta3 refers to the version of the rules protobuf type in lekko.rules.v1beta3.rules.proto
type v1beta3 struct {
	rule *rulesv1beta3.Rule
}

// Represents the rules defined in the proto package 'lekko.rules.v1beta3'.
func NewV1Beta3(rule *rulesv1beta3.Rule) *v1beta3 {
	return &v1beta3{rule: rule}
}

func (v1b3 *v1beta3) EvaluateRule(context map[string]interface{}) (bool, error) {
	return v1b3.evaluateRule(v1b3.rule, context)
}

func (v1b3 *v1beta3) evaluateRule(rule *rulesv1beta3.Rule, context map[string]interface{}) (bool, error) {
	if rule == nil {
		return false, ErrEmptyRule
	}
	switch r := rule.Rule.(type) {
	case *rulesv1beta3.Rule_BoolConst:
		return r.BoolConst, nil
	case *rulesv1beta3.Rule_Not:
		innerPasses, err := v1b3.evaluateRule(r.Not, context)
		if err != nil {
			return false, errors.Wrap(err, "not: ")
		}
		return !innerPasses, nil
	case *rulesv1beta3.Rule_LogicalExpression:
		var bools []bool
		if len(r.LogicalExpression.GetRules()) == 0 {
			return false, errors.New("no rules found in logical expression")
		}
		for i, rule := range r.LogicalExpression.GetRules() {
			passes, err := v1b3.evaluateRule(rule, context)
			if err != nil {
				return false, errors.Wrapf(err, "rule idx %d", i)
			}
			bools = append(bools, passes)
		}
		return reduce(bools, r.LogicalExpression.LogicalOperator)
	case *rulesv1beta3.Rule_Atom:
		contextKey := r.Atom.GetContextKey()
		runtimeCtxVal, present := context[contextKey]
		if r.Atom.ComparisonOperator == rulesv1beta3.ComparisonOperator_COMPARISON_OPERATOR_PRESENT {
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
		case rulesv1beta3.ComparisonOperator_COMPARISON_OPERATOR_EQUALS:
			return v1b3.evaluateEquals(r.Atom.GetComparisonValue(), runtimeCtxVal)
		case rulesv1beta3.ComparisonOperator_COMPARISON_OPERATOR_LESS_THAN:
			return v1b3.evaluateNumberComparator(r.Atom.ComparisonOperator, r.Atom.GetComparisonValue(), runtimeCtxVal)
		case rulesv1beta3.ComparisonOperator_COMPARISON_OPERATOR_LESS_THAN_OR_EQUALS:
			return v1b3.evaluateNumberComparator(r.Atom.ComparisonOperator, r.Atom.GetComparisonValue(), runtimeCtxVal)
		case rulesv1beta3.ComparisonOperator_COMPARISON_OPERATOR_GREATER_THAN:
			return v1b3.evaluateNumberComparator(r.Atom.ComparisonOperator, r.Atom.GetComparisonValue(), runtimeCtxVal)
		case rulesv1beta3.ComparisonOperator_COMPARISON_OPERATOR_GREATER_THAN_OR_EQUALS:
			return v1b3.evaluateNumberComparator(r.Atom.ComparisonOperator, r.Atom.GetComparisonValue(), runtimeCtxVal)
		case rulesv1beta3.ComparisonOperator_COMPARISON_OPERATOR_CONTAINED_WITHIN:
			return v1b3.evaluateContainedWithin(r.Atom.GetComparisonValue(), runtimeCtxVal)
		case rulesv1beta3.ComparisonOperator_COMPARISON_OPERATOR_STARTS_WITH:
			return v1b3.evaluateStringComparator(r.Atom.ComparisonOperator, r.Atom.GetComparisonValue(), runtimeCtxVal)
		case rulesv1beta3.ComparisonOperator_COMPARISON_OPERATOR_ENDS_WITH:
			return v1b3.evaluateStringComparator(r.Atom.ComparisonOperator, r.Atom.GetComparisonValue(), runtimeCtxVal)
		case rulesv1beta3.ComparisonOperator_COMPARISON_OPERATOR_CONTAINS:
			return v1b3.evaluateStringComparator(r.Atom.ComparisonOperator, r.Atom.GetComparisonValue(), runtimeCtxVal)
		}
	}
	return false, errors.Errorf("unknown rule type %T", rule.Rule)
}

func reduce(bools []bool, op rulesv1beta3.LogicalOperator) (bool, error) {
	ret := op == rulesv1beta3.LogicalOperator_LOGICAL_OPERATOR_AND
	for _, b := range bools {
		switch op {
		case rulesv1beta3.LogicalOperator_LOGICAL_OPERATOR_AND:
			ret = ret && b
		case rulesv1beta3.LogicalOperator_LOGICAL_OPERATOR_OR:
			ret = ret || b
		default:
			return false, errors.Wrap(ErrUnknownLogicalOperator, op.String())
		}
	}
	return ret, nil
}

// Only accepts bool, number or string ruleVal.
func (v1b3 *v1beta3) evaluateEquals(ruleVal *structpb.Value, contextVal interface{}) (bool, error) {
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
		return false, errUnsupportedType(rulesv1beta3.ComparisonOperator_COMPARISON_OPERATOR_EQUALS, ruleVal)
	}
}

func (v1b3 *v1beta3) evaluateNumberComparator(co rulesv1beta3.ComparisonOperator, ruleVal *structpb.Value, contextVal interface{}) (bool, error) {
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
	case rulesv1beta3.ComparisonOperator_COMPARISON_OPERATOR_LESS_THAN:
		return numVal < ruleNumVal, nil
	case rulesv1beta3.ComparisonOperator_COMPARISON_OPERATOR_LESS_THAN_OR_EQUALS:
		return numVal <= ruleNumVal, nil
	case rulesv1beta3.ComparisonOperator_COMPARISON_OPERATOR_GREATER_THAN:
		return numVal > ruleNumVal, nil
	case rulesv1beta3.ComparisonOperator_COMPARISON_OPERATOR_GREATER_THAN_OR_EQUALS:
		return numVal >= ruleNumVal, nil
	default:
		return false, errors.Errorf("expected numerical comparison operator, got %v", co)
	}
}

func (v1b3 *v1beta3) evaluateContainedWithin(ruleVal *structpb.Value, contextVal interface{}) (bool, error) {
	listRuleVal, ok := ruleVal.Kind.(*structpb.Value_ListValue)
	if !ok {
		return false, errUnsupportedType(rulesv1beta3.ComparisonOperator_COMPARISON_OPERATOR_CONTAINED_WITHIN, ruleVal)
	}
	for _, elem := range listRuleVal.ListValue.Values {
		result, err := v1b3.evaluateEquals(elem, contextVal)
		if err != nil || !result {
			continue // no match, check next element
		}
		return true, nil
	}
	return false, nil
}

func (v1b3 *v1beta3) evaluateStringComparator(co rulesv1beta3.ComparisonOperator, ruleVal *structpb.Value, contextVal interface{}) (bool, error) {
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
	case rulesv1beta3.ComparisonOperator_COMPARISON_OPERATOR_STARTS_WITH:
		return strings.HasPrefix(stringVal, ruleStringVal), nil
	case rulesv1beta3.ComparisonOperator_COMPARISON_OPERATOR_ENDS_WITH:
		return strings.HasSuffix(stringVal, ruleStringVal), nil
	case rulesv1beta3.ComparisonOperator_COMPARISON_OPERATOR_CONTAINS:
		return strings.Contains(stringVal, ruleStringVal), nil
	default:
		return false, errors.Errorf("expected string comparison operator, got %v", co)
	}
}
