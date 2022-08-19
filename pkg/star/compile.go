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
	"io/ioutil"
	"os"

	"github.com/lekkodev/cli/pkg/feature"
	"github.com/pkg/errors"
	"github.com/stripe/skycfg/go/protomodule"
	"go.starlark.net/starlark"
	"google.golang.org/protobuf/reflect/protoregistry"
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

func (c *compiler) Compile() (*feature.Feature, error) {
	// Execute the starlark file to retrieve its contents (globals)
	thread := &starlark.Thread{
		Name: "load",
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
	globals, err := starlark.ExecFile(thread, c.starfilePath, moduleSource, starlark.StringDict{
		"proto":   protoModule,
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
