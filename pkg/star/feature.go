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

package star

import (
	"fmt"

	"github.com/lekkodev/cli/pkg/feature"
	"github.com/pkg/errors"
	"github.com/stripe/skycfg/go/protomodule"
	"go.starlark.net/starlark"
	"go.starlark.net/starlarkstruct"
)

const (
	featureConstructor   starlark.String = "feature"
	featureVariablename  string          = "result"
	defaultValueAttrName string          = "default"
	descriptionAttrName  string          = "description"
	rulesAttrName        string          = "rules"
)

func makeFeature(_ *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	if len(args) > 0 {
		return nil, fmt.Errorf("feature: unexpected positional arguments")
	}
	return starlarkstruct.FromKeywords(featureConstructor, kwargs), nil
}

type featureBuilder struct {
	globals starlark.StringDict
}

func newFeatureBuilder(globals starlark.StringDict) *featureBuilder {
	return &featureBuilder{
		globals: globals,
	}
}

func (fb *featureBuilder) build() (*feature.Feature, error) {
	resultVal, ok := fb.globals[featureVariablename]
	if !ok {
		return nil, fmt.Errorf("required variable %s is not found", featureVariablename)
	}
	featureVal, ok := resultVal.(*starlarkstruct.Struct)
	if !ok {
		return nil, fmt.Errorf("expecting variable of type %s, instead got %T", featureConstructor.GoString(), featureVal)
	}
	f, err := fb.init(featureVal)
	if err != nil {
		return nil, errors.Wrap(err, "initialize feature")
	}
	f.Description, err = fb.getDescription(featureVal)
	if err != nil {
		return nil, errors.Wrap(err, "description")
	}
	if err = fb.addRules(f, featureVal); err != nil {
		return nil, errors.Wrap(err, "add rules")
	}
	return f, nil
}

func (fb *featureBuilder) init(featureVal *starlarkstruct.Struct) (*feature.Feature, error) {
	defaultVal, err := featureVal.Attr(defaultValueAttrName)
	if err != nil {
		return nil, errors.Wrap(err, "default attribute")
	}
	// check if this is a complex type
	message, ok := protomodule.AsProtoMessage(defaultVal)
	if ok {
		return feature.NewComplexFeature(message), nil
	}
	// check if this is a supported primitive type
	switch typedVal := defaultVal.(type) {
	case starlark.Bool:
		return feature.NewBoolFeature(bool(typedVal)), nil
	default:
		return nil, fmt.Errorf("received default value with unsupported type %T", typedVal)
	}
}

func (fb *featureBuilder) getDescription(featureVal *starlarkstruct.Struct) (string, error) {
	descriptionVal, err := featureVal.Attr(descriptionAttrName)
	if err != nil {
		return "", errors.Wrap(err, "default attribute")
	}
	dsc, ok := descriptionVal.(starlark.String)
	if !ok {
		return "", fmt.Errorf("description must be a string (got a %s)", descriptionVal.Type())
	}
	return dsc.GoString(), nil
}

func (fb *featureBuilder) addRules(f *feature.Feature, featureVal *starlarkstruct.Struct) error {
	rulesVal, err := featureVal.Attr(rulesAttrName)
	if err != nil {
		// no rules provided
		return nil
	}
	seq, ok := rulesVal.(starlark.Sequence)
	if !ok {
		return fmt.Errorf("rules: did not get back a starlark sequence: %v", rulesVal)
	}
	it := seq.Iterate()
	defer it.Done()
	var val starlark.Value
	var i int
	for it.Next(&val) {
		if val == starlark.None {
			return fmt.Errorf("type error: [%v] %v", val.Type(), val.String())
		}
		tuple, ok := val.(starlark.Tuple)
		if !ok {
			return fmt.Errorf("type error: expecting tuple, got %v", val.Type())
		}
		if tuple.Len() != 2 {
			return fmt.Errorf("expecting tuple of length 2, got length %d: %v", tuple.Len(), tuple)
		}
		conditionStr, ok := tuple.Index(0).(starlark.String)
		if !ok {
			return fmt.Errorf("type error: expecting string, got %v: %v", tuple.Index(0).Type(), tuple.Index(0))
		}
		// TODO: parse into ruleslang
		if conditionStr.GoString() == "" {
			return fmt.Errorf("expecting valid ruleslang, got %s", conditionStr.GoString())
		}
		ruleVal := tuple.Index(1)
		switch f.FeatureType {
		case feature.FeatureTypeComplex:
			message, ok := protomodule.AsProtoMessage(ruleVal)
			if !ok {
				return typeError(f.FeatureType, i, ruleVal)
			}
			f.Rules = append(f.Rules, &feature.Rule{
				Condition: conditionStr.GoString(),
				Value:     message,
			})
		case feature.FeatureTypeBool:
			typedRuleVal, ok := ruleVal.(starlark.Bool)
			if !ok {
				return typeError(f.FeatureType, i, ruleVal)
			}
			f.Rules = append(f.Rules, &feature.Rule{
				Condition: conditionStr.GoString(),
				Value:     bool(typedRuleVal),
			})
		default:
			return fmt.Errorf("unsupported type %s for rule #%d", f.FeatureType, i)
		}
		i++
	}

	return nil
}

func typeError(expectedType feature.FeatureType, ruleIdx int, value starlark.Value) error {
	return fmt.Errorf("expecting %s for rule idx #%d, instead got %T", starType(expectedType), ruleIdx, value)
}

func starType(ft feature.FeatureType) string {
	switch ft {
	case feature.FeatureTypeComplex:
		return "protoMessage"
	case feature.FeatureTypeBool:
		return fmt.Sprintf("%T", starlark.False)
	default:
		return "unknown"
	}
}
