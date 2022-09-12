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
	"go.starlark.net/starlarkstruct"
	"go.starlark.net/starlarktest"
	"google.golang.org/protobuf/reflect/protoregistry"
)

type Compiler interface {
	Compile() (*feature.Feature, error)
}

type compiler struct {
	registry                            *protoregistry.Types
	protoDir, starfilePath, featureName string

	Formatter
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
		Formatter:    NewStarFormatter(starfilePath, featureName, false),
	}
}

func (c *compiler) Compile() (*feature.Feature, error) {
	if err := c.Format(); err != nil {
		return nil, errors.Wrap(err, "failed to format star file")
	}
	// Execute the starlark file to retrieve its contents (globals)
	thread := &starlark.Thread{
		Name: "compile",
	}
	reader, err := os.Open(c.starfilePath)
	if err != nil {
		return nil, errors.Wrap(err, "open starfile")
	}
	defer reader.Close()
	moduleSource, err := ioutil.ReadAll(reader)
	if err != nil {
		return nil, errors.Wrap(err, "read starfile")
	}
	protoModule := protomodule.NewModule(c.registry)
	assertModule, err := newAssertModule()
	if err != nil {
		return nil, errors.Wrap(err, "new assert module")
	}
	globals, err := starlark.ExecFile(thread, c.starfilePath, moduleSource, starlark.StringDict{
		"proto":   protoModule,
		"assert":  assertModule,
		"feature": starlark.NewBuiltin("feature", makeFeature),
	})
	if err != nil {
		return nil, errors.Wrap(err, "starlark execfile")
	}
	f, err := newFeatureBuilder(globals).build()
	if err != nil {
		return nil, errors.Wrap(err, "build")
	}
	f.Key = c.featureName
	return f, nil
}

func load(thread *starlark.Thread, module string) (starlark.StringDict, error) {
	if module == "assert.star" {
		return starlarktest.LoadAssertModule()
	}
	return nil, fmt.Errorf("load not implemented for %s", module)
}

func newAssertModule() (*starlarkstruct.Module, error) {
	sd, err := starlarktest.LoadAssertModule()
	if err != nil {
		return nil, errors.Wrap(err, "load assert module")
	}
	assertVal, ok := sd["assert"]
	if !ok {
		return nil, fmt.Errorf("could not find assert value in keys %v", sd.Keys())
	}
	assertModule, ok := assertVal.(*starlarkstruct.Module)
	if !ok {
		return nil, fmt.Errorf("assertVal incorrect type %T", assertVal)
	}
	return assertModule, nil
}
