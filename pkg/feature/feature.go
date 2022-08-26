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
	"fmt"

	lekkov1beta1 "github.com/lekkodev/cli/pkg/gen/proto/go/lekko/feature/v1beta1"
	"github.com/pkg/errors"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/types/known/anypb"
	"google.golang.org/protobuf/types/known/wrapperspb"
)

// Indicates the lekko-specific types that are allowed in feature flags.
type FeatureType string

const (
	FeatureTypeBool    FeatureType = "bool"
	FeatureTypeInt     FeatureType = "int"
	FeatureTypeString  FeatureType = "string"
	FeatureTypeComplex FeatureType = "complex"
)

type Rule struct {
	Condition string
	Value     interface{}
}

type UnitTest struct {
	Context       map[string]interface{}
	ExpectedValue interface{}
}

type Feature struct {
	Key, Description string
	Value            interface{}
	FeatureType      FeatureType

	Rules     []*Rule
	UnitTests []*UnitTest
}

func NewBoolFeature(value bool) *Feature {
	return &Feature{
		Value:       value,
		FeatureType: FeatureTypeBool,
	}
}

func NewComplexFeature(value protoreflect.ProtoMessage) *Feature {
	return &Feature{
		Value:       value,
		FeatureType: FeatureTypeComplex,
	}
}

func valToAny(value interface{}) (*anypb.Any, error) {
	switch typedVal := value.(type) {
	case bool:
		return newAny(wrapperspb.Bool(typedVal))
	case string:
		return newAny(wrapperspb.String(typedVal))
	case protoreflect.ProtoMessage:
		return newAny(typedVal)
	default:
		return nil, fmt.Errorf("unsupported feature type %T", typedVal)
	}
}

func newAny(pm protoreflect.ProtoMessage) (*anypb.Any, error) {
	ret := new(anypb.Any)
	if err := anypb.MarshalFrom(ret, pm, proto.MarshalOptions{Deterministic: true}); err != nil {
		return nil, err
	}
	return ret, nil
}

func (f *Feature) ToProto() (*lekkov1beta1.Feature, error) {
	ret := &lekkov1beta1.Feature{
		Key:         f.Key,
		Description: f.Description,
	}
	defaultAny, err := valToAny(f.Value)
	if err != nil {
		return nil, errors.Wrap(err, "default value to any")
	}
	tree := &lekkov1beta1.Tree{
		Default: defaultAny,
	}
	// for now, our tree only has 1 level, (it's effectievly a list)
	for _, rule := range f.Rules {
		ruleAny, err := valToAny(rule.Value)
		if err != nil {
			return nil, errors.Wrap(err, "rule value to any")
		}
		tree.Constraints = append(tree.Constraints, &lekkov1beta1.Constraint{
			Rule:  rule.Condition,
			Value: ruleAny,
		})
	}
	ret.Tree = tree
	return ret, nil
}
