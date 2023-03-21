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

	rulesv1beta3 "github.com/lekkodev/cli/pkg/gen/proto/go/lekko/rules/v1beta3"
	"google.golang.org/protobuf/types/known/structpb"
)

// age ==
func AgeEqualsV3(age float64) *rulesv1beta3.Atom {
	return AgeV3("==", age)
}

// city ==
func CityEqualsV3(city string) *rulesv1beta3.Atom {
	return CityV3("==", city)
}

func AgeV3(op string, age float64) *rulesv1beta3.Atom {
	return atomV3(ContextKeyAge, op, structpb.NewNumberValue(age))
}

func CityV3(op, city string) *rulesv1beta3.Atom {
	return atomV3(ContextKeyCity, op, structpb.NewStringValue(city))
}

func CityInV3(cities ...interface{}) *rulesv1beta3.Atom {
	list, err := structpb.NewList(cities)
	if err != nil {
		log.Fatal(err)
	}
	return atomV3(ContextKeyCity, "IN", structpb.NewListValue(list))
}

func atomV3(key string, op string, value *structpb.Value) *rulesv1beta3.Atom {
	protoOp := rulesv1beta3.ComparisonOperator_COMPARISON_OPERATOR_UNSPECIFIED
	switch op {
	case "==":
		protoOp = rulesv1beta3.ComparisonOperator_COMPARISON_OPERATOR_EQUALS
	case "<":
		protoOp = rulesv1beta3.ComparisonOperator_COMPARISON_OPERATOR_LESS_THAN
	case ">":
		protoOp = rulesv1beta3.ComparisonOperator_COMPARISON_OPERATOR_GREATER_THAN
	case "<=":
		protoOp = rulesv1beta3.ComparisonOperator_COMPARISON_OPERATOR_LESS_THAN_OR_EQUALS
	case ">=":
		protoOp = rulesv1beta3.ComparisonOperator_COMPARISON_OPERATOR_GREATER_THAN_OR_EQUALS
	case "IN":
		protoOp = rulesv1beta3.ComparisonOperator_COMPARISON_OPERATOR_CONTAINED_WITHIN
	case "STARTS":
		protoOp = rulesv1beta3.ComparisonOperator_COMPARISON_OPERATOR_STARTS_WITH
	case "ENDS":
		protoOp = rulesv1beta3.ComparisonOperator_COMPARISON_OPERATOR_ENDS_WITH
	case "CONTAINS":
		protoOp = rulesv1beta3.ComparisonOperator_COMPARISON_OPERATOR_CONTAINS
	case "PRESENT":
		protoOp = rulesv1beta3.ComparisonOperator_COMPARISON_OPERATOR_PRESENT
	}
	return &rulesv1beta3.Atom{
		ContextKey:         key,
		ComparisonValue:    value,
		ComparisonOperator: protoOp,
	}
}
