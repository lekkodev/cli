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
	"io/ioutil"
	"os"

	"github.com/lekkodev/cli/pkg/feature"
	"github.com/pkg/errors"
	"github.com/stripe/skycfg/go/protomodule"
	"go.starlark.net/starlark"
	"google.golang.org/protobuf/reflect/protoregistry"
)

const (
	starVariableDefault     = "default"
	starVariableDescription = "description"
	starVariableRules       = "rules"
)

type Compiler interface {
	Compile() (*feature.Feature, error)
}

type compiler struct {
	registry                            *protoregistry.Types
	protoDir, starfilePath, featureName string
}

// Compile takes the following parameters:
// 		protoDir: path to the proto directory that stores all user-defined proto files
// 		starfilePath: path to the .star file that defines this feature flag
// 		featureName: human-readable name of this feature flag. Also matches the starfile name.
func NewCompiler(registry *protoregistry.Types, protoDir, starfilePath, featureName string) Compiler {
	return &compiler{
		registry:     registry,
		protoDir:     protoDir,
		starfilePath: starfilePath,
		featureName:  featureName,
	}
}

type variables struct {
	defaultVal, description, rules starlark.Value
}

func (c *compiler) Compile() (*feature.Feature, error) {
	// Execute the starlark file to retrieve its contents (globals)
	vars, err := loadVariables(c.registry, c.starfilePath)
	if err != nil {
		return nil, errors.Wrap(err, "load globals")
	}
	// Now, get the config values and turn them into our internal representation (feature.Feature).
	f, err := getFeature(vars.defaultVal)
	if err != nil {
		return nil, errors.Wrap(err, "get feature")
	}
	f.Key = c.featureName
	f.Description, err = getDescription(vars.description)
	if err != nil {
		return nil, errors.Wrap(err, "get description")
	}
	if err = addRules(f, vars.rules); err != nil {
		return nil, errors.Wrap(err, "add rules")
	}
	return f, nil
}

func loadVariables(registry *protoregistry.Types, starfilePath string) (*variables, error) {
	thread := &starlark.Thread{
		Name: "load",
	}
	reader, err := os.Open(starfilePath)
	if err != nil {
		return nil, errors.Wrap(err, "open starfile")
	}
	defer reader.Close()
	moduleSource, err := ioutil.ReadAll(reader)
	if err != nil {
		return nil, errors.Wrap(err, "read starfile")
	}
	protoModule := protomodule.NewModule(registry)
	globals, err := starlark.ExecFile(thread, starfilePath, moduleSource, starlark.StringDict{
		"proto": protoModule,
	})
	if err != nil {
		return nil, errors.Wrap(err, "starlark execfile")
	}
	vars := &variables{}
	var ok bool
	vars.defaultVal, ok = globals[starVariableDefault]
	if !ok {
		return nil, fmt.Errorf("required variable %s not found", starVariableDefault)
	}
	vars.description, ok = globals[starVariableDescription]
	if !ok {
		return nil, fmt.Errorf("required variable %s not found", starVariableDescription)
	}
	vars.rules, ok = globals[starVariableRules]
	if !ok {
		vars.rules = starlark.None
	}
	return vars, nil
}

func getFeature(defaultVal starlark.Value) (*feature.Feature, error) {
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

func getDescription(descriptionVal starlark.Value) (string, error) {
	dsc, ok := descriptionVal.(starlark.String)
	if !ok {
		return "", fmt.Errorf("description must be a string (got a %s)", descriptionVal.Type())
	}
	return dsc.GoString(), nil
}

func addRules(f *feature.Feature, rulesVal starlark.Value) error {
	if _, ok := rulesVal.(starlark.NoneType); ok {
		// no rules provided, continue
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
