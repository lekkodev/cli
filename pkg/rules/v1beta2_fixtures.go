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
	"log"

	rulesv1beta2 "buf.build/gen/go/lekkodev/cli/protocolbuffers/go/lekko/rules/v1beta2"
	"google.golang.org/protobuf/types/known/structpb"
)

const (
	ContextKeyAge  = "age"
	ContextKeyCity = "city"
)

type ctxBuilder struct {
	c map[string]interface{}
}

func CtxBuilder() *ctxBuilder {
	return &ctxBuilder{
		c: make(map[string]interface{}),
	}
}

func (cb *ctxBuilder) Age(val interface{}) *ctxBuilder {
	cb.c[ContextKeyAge] = val
	return cb
}

func (cb *ctxBuilder) City(val interface{}) *ctxBuilder {
	cb.c[ContextKeyCity] = val
	return cb
}

func (cb *ctxBuilder) B() map[string]interface{} {
	return cb.c
}

// age ==
func AgeEquals(age float64) *rulesv1beta2.Atom {
	return Age("==", age)
}

// city ==
func CityEquals(city string) *rulesv1beta2.Atom {
	return City("==", city)
}

func Age(op string, age float64) *rulesv1beta2.Atom {
	return atom(ContextKeyAge, op, structpb.NewNumberValue(age))
}

func City(op, city string) *rulesv1beta2.Atom {
	return atom(ContextKeyCity, op, structpb.NewStringValue(city))
}

func CityIn(cities ...interface{}) *rulesv1beta2.Atom {
	list, err := structpb.NewList(cities)
	if err != nil {
		log.Fatal(err)
	}
	return atom(ContextKeyCity, "IN", structpb.NewListValue(list))
}

func atom(key string, op string, value *structpb.Value) *rulesv1beta2.Atom {
	protoOp := rulesv1beta2.ComparisonOperator_COMPARISON_OPERATOR_UNSPECIFIED
	switch op {
	case "==":
		protoOp = rulesv1beta2.ComparisonOperator_COMPARISON_OPERATOR_EQUALS
	case "<":
		protoOp = rulesv1beta2.ComparisonOperator_COMPARISON_OPERATOR_LESS_THAN
	case ">":
		protoOp = rulesv1beta2.ComparisonOperator_COMPARISON_OPERATOR_GREATER_THAN
	case "<=":
		protoOp = rulesv1beta2.ComparisonOperator_COMPARISON_OPERATOR_LESS_THAN_OR_EQUALS
	case ">=":
		protoOp = rulesv1beta2.ComparisonOperator_COMPARISON_OPERATOR_GREATER_THAN_OR_EQUALS
	case "IN":
		protoOp = rulesv1beta2.ComparisonOperator_COMPARISON_OPERATOR_CONTAINED_WITHIN
	case "STARTS":
		protoOp = rulesv1beta2.ComparisonOperator_COMPARISON_OPERATOR_STARTS_WITH
	case "ENDS":
		protoOp = rulesv1beta2.ComparisonOperator_COMPARISON_OPERATOR_ENDS_WITH
	case "CONTAINS":
		protoOp = rulesv1beta2.ComparisonOperator_COMPARISON_OPERATOR_CONTAINS
	case "PRESENT":
		protoOp = rulesv1beta2.ComparisonOperator_COMPARISON_OPERATOR_PRESENT
	}
	return &rulesv1beta2.Atom{
		ContextKey:         key,
		ComparisonValue:    value,
		ComparisonOperator: protoOp,
	}
}
