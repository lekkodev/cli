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

	lekkov1beta1 "github.com/lekkodev/cli/pkg/gen/proto/go/lekko/feature/v1beta1"
	"github.com/pkg/errors"
	"github.com/stripe/skycfg/go/protomodule"
	"go.starlark.net/starlark"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/types/known/anypb"
)

// Compile takes the following parameters:
// 		protoDir: path to the proto directory that stores all user-defined proto files
// 		starfilePath: path to the .star file that defines this feature flag
// 		featureName: human-readable name of this feature flag. Also matches the starfile name.
// It returns a fully formed v1beta2 lekko feature flag.
func Compile(protoDir, starfilePath, featureName string) (*lekkov1beta1.Feature, error) {
	// Build the buf image
	// NOTE: in the future, we will want to create the buf image once, and load it into
	// compilation of all feature flags. For now we just have 1 feature flag so this works.
	image, err := newBufImage(protoDir)
	if err != nil {
		return nil, errors.Wrap(err, "new buf image")
	}
	defer func() {
		// Note: we choose not to check in the buf image to the config repo and instead
		// always generate on-the-fly. This decision can be reevaluated.
		if err := image.cleanup(); err != nil {
			fmt.Printf("Error encountered when cleaning up buf image: %v", err)
		}
	}()

	// Execute the starlark file to retrieve its contents (globals)
	globals, err := loadGlobals(image, starfilePath)
	if err != nil {
		return nil, errors.Wrap(err, "load globals")
	}

	// Now, get the config values and turn them into proto.
	description, err := getDescription(globals)
	if err != nil {
		return nil, errors.Wrap(err, "getDescription")
	}
	def, err := getDefault(globals)
	if err != nil {
		return nil, errors.Wrap(err, "getDefault")
	}
	rules, err := getRules(globals)
	if err != nil {
		return nil, errors.Wrap(err, "getRules")
	}
	tree, err := constructTree(def, rules)
	if err != nil {
		return nil, errors.Wrap(err, "constructTree")
	}

	return &lekkov1beta1.Feature{
		Key:         featureName,
		Description: description,
		Tree:        tree,
	}, nil
}

func loadGlobals(image *bufImage, starfilePath string) (starlark.StringDict, error) {
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
	types, err := buildTypes(image.filename)
	if err != nil {
		return nil, errors.Wrap(err, "build types")
	}
	protoModule := protomodule.NewModule(types)
	globals, err := starlark.ExecFile(thread, starfilePath, moduleSource, starlark.StringDict{
		"proto": protoModule,
	})
	if err != nil {
		return nil, errors.Wrap(err, "starlark execfile")
	}
	return globals, nil
}

func getDefault(globals starlark.StringDict) (protoreflect.ProtoMessage, error) {
	defaultVal, ok := globals["default"]
	if !ok {
		return nil, fmt.Errorf("no `default` function found")
	}
	message, ok := protomodule.AsProtoMessage(defaultVal)
	if !ok {
		return nil, fmt.Errorf("default returned something that is not proto, got %s", defaultVal.Type())
	}
	return message, nil
}

func getDescription(globals starlark.StringDict) (string, error) {
	sd, ok := globals["description"]
	if !ok {
		return "", fmt.Errorf("no `description` global variable found")
	}
	dsc, ok := sd.(starlark.String)
	if !ok {
		return "", fmt.Errorf("`description` must be a string (got a %s)", sd.Type())
	}
	return dsc.GoString(), nil
}

type Rule struct {
	rule  string
	value protoreflect.ProtoMessage
}

func getRules(globals starlark.StringDict) ([]Rule, error) {
	rulesVal, ok := globals["rules"]
	if !ok {
		return nil, fmt.Errorf("no `rules` function found ")
	}

	seq, ok := rulesVal.(starlark.Sequence)
	if !ok {
		return nil, fmt.Errorf("rules: did not get back a starlark sequence: %v", rulesVal)
	}
	var rulesArr []Rule
	it := seq.Iterate()
	defer it.Done()
	var val starlark.Value
	for it.Next(&val) {
		if val == starlark.None {
			return nil, fmt.Errorf("type error: [%v] %v", val.Type(), val.String())
		}
		tuple, ok := val.(starlark.Tuple)
		if !ok {
			return nil, fmt.Errorf("type error: expecting tuple, got %v", val.Type())
		}
		if tuple.Len() != 2 {
			return nil, fmt.Errorf("expecting tuple of length 2, got length %d: %v", tuple.Len(), tuple)
		}
		conditionStr, ok := tuple.Index(0).(starlark.String)
		if !ok {
			return nil, fmt.Errorf("type error: expecting string, got %v: %v", tuple.Index(0).Type(), tuple.Index(0))
		}
		// TODO: parse into ruleslang
		if conditionStr.GoString() == "" {
			return nil, fmt.Errorf("expecting valid ruleslang, got %s", conditionStr.GoString())
		}
		message, ok := protomodule.AsProtoMessage(tuple.Index(1))
		if !ok {
			return nil, fmt.Errorf("condition val returned something that is not proto, got %s", tuple.Index(1).Type())
		}
		rulesArr = append(rulesArr, Rule{
			rule:  conditionStr.GoString(),
			value: message,
		})
	}

	return rulesArr, nil
}

func constructTree(defaultValue protoreflect.ProtoMessage, rules []Rule) (*lekkov1beta1.Tree, error) {
	defaultAny, err := anypb.New(defaultValue)
	if err != nil {
		return nil, errors.Wrap(err, "anypb.New")
	}
	tree := &lekkov1beta1.Tree{
		Default: defaultAny,
	}
	// for now, our tree only has 1 level, (its effectievly a list)
	for _, rule := range rules {
		ruleAny, err := anypb.New(rule.value)
		if err != nil {
			return nil, errors.Wrap(err, "rule value to any")
		}
		tree.Constraints = append(tree.Constraints, &lekkov1beta1.Constraint{
			Rule:  rule.rule,
			Value: ruleAny,
		})
	}
	return tree, nil
}
