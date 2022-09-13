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
	"bytes"
	"encoding/json"
	"fmt"

	lekkov1beta1 "github.com/lekkodev/cli/pkg/gen/proto/go/lekko/feature/v1beta1"
	"github.com/pkg/errors"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/reflect/protoregistry"
	"google.golang.org/protobuf/types/known/anypb"
	"google.golang.org/protobuf/types/known/structpb"
	"google.golang.org/protobuf/types/known/wrapperspb"
)

// Indicates the lekko-specific types that are allowed in feature flags.
type FeatureType string

const (
	FeatureTypeBool   FeatureType = "bool"
	FeatureTypeInt    FeatureType = "int"
	FeatureTypeString FeatureType = "string"
	FeatureTypeProto  FeatureType = "proto"
	FeatureTypeJSON   FeatureType = "json"
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
		FeatureType: FeatureTypeProto,
	}
}

func NewJSONFeature(encodedJSON []byte) (*Feature, error) {
	s, err := valFromJSON(encodedJSON)
	if err != nil {
		return nil, errors.Wrap(err, "val from json")
	}
	return &Feature{
		Value:       s,
		FeatureType: FeatureTypeJSON,
	}, nil
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

func valFromJSON(encoded []byte) (interface{}, error) {
	val := &structpb.Value{}
	if err := val.UnmarshalJSON(encoded); err != nil {
		return nil, errors.Wrap(err, "failed to unmarshal json into struct")
	}
	return val, nil
}

func (f *Feature) AddJSONRule(condition string, encoded []byte) error {
	val, err := valFromJSON(encoded)
	if err != nil {
		return errors.Wrap(err, "val from json")
	}
	f.Rules = append(f.Rules, &Rule{
		Condition: condition,
		Value:     val,
	})
	return nil
}

func (f *Feature) AddJSONUnitTest(context map[string]interface{}, encoded []byte) error {
	val, err := valFromJSON(encoded)
	if err != nil {
		return errors.Wrap(err, "val from json")
	}
	f.UnitTests = append(f.UnitTests, &UnitTest{
		Context:       context,
		ExpectedValue: val,
	})
	return nil
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

func ProtoToJSON(fProto *lekkov1beta1.Feature, registry *protoregistry.Types) ([]byte, error) {
	jBytes, err := protojson.MarshalOptions{
		Resolver: registry,
	}.Marshal(fProto)
	if err != nil {
		return nil, errors.Wrap(err, "failed to marshal proto to json")
	}
	indentedJBytes := bytes.NewBuffer(nil)
	// encoding/json provides a deterministic serialization output, ensuring
	// that indentation always uses the same number of characters.
	if err := json.Indent(indentedJBytes, jBytes, "", "  "); err != nil {
		return nil, errors.Wrap(err, "failed to indent json")
	}
	return indentedJBytes.Bytes(), nil
}

func (f *Feature) ToJSON(registry *protoregistry.Types) ([]byte, error) {
	fProto, err := f.ToProto()
	if err != nil {
		return nil, errors.Wrap(err, "feature to proto")
	}
	return ProtoToJSON(fProto, registry)
}

func (f *Feature) PrintJSON(registry *protoregistry.Types) {
	jBytes, err := f.ToJSON(registry)
	if err != nil {
		fmt.Printf("failed to convert feature to json: %v\n", err)
	}
	fmt.Println(string(jBytes))
}

func (f *Feature) ToEvaluableFeature() (EvaluableFeature, error) {
	res, err := f.ToProto()
	if err != nil {
		return nil, err
	}
	return &v1beta2{res}, nil
}

func (f *Feature) RunUnitTests(_ *protoregistry.Types) error {
	eval, err := f.ToEvaluableFeature()
	if err != nil {
		return err
	}
	for idx, test := range f.UnitTests {
		a, err := eval.Evaluate(test.Context)
		if err != nil {
			return err
		}
		val, err := valToAny(test.ExpectedValue)
		if err != nil {
			return err
		}
		if !proto.Equal(a, val) {
			return fmt.Errorf("test failed: %v index: %d %+v %+v", f.Key, idx, a, val)
		}
	}
	return nil
}

func (f *Feature) String() string {
	return fmt.Sprintf("key: %s\ndescription: %s\nvalue [%T]: %v\nrules: %v", f.Key, f.Description, f.Value, f.Value, f.Rules)
}
