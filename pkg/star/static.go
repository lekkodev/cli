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
	"context"
	"fmt"
	"io/ioutil"
	"path/filepath"
	"strings"

	"github.com/bazelbuild/buildtools/build"
	butils "github.com/bazelbuild/buildtools/buildifier/utils"
	"github.com/lekkodev/cli/pkg/feature"
	"github.com/lekkodev/cli/pkg/fs"
	"github.com/lekkodev/cli/pkg/metadata"
	"github.com/pkg/errors"
)

// Parse reads the star file at the given path performs static parsing,
// converting the feature to our go-native model.
func Parse(root, filename string) error {
	parser := butils.GetParser(inputTypeAuto)
	data, err := ioutil.ReadFile(filename)
	if err != nil {
		return errors.Wrap(err, "read file")
	}
	file, err := parser(filename, data)
	if err != nil {
		return errors.Wrap(err, "parse")
	}
	sb := newStaticBuilder(file)
	f, err := sb.build()
	if err != nil {
		return err
	}
	f.Key = strings.Split(filepath.Base(filename), ".")[0]
	rootMD, _, err := metadata.ParseFullConfigRepoMetadataStrict(context.Background(), root, fs.LocalProvider())
	if err != nil {
		return errors.Wrap(err, "parse root metadata")
	}
	registry, err := BuildDynamicTypeRegistry(rootMD.ProtoDirectory)
	if err != nil {
		return errors.Wrap(err, "build dynamic type registry")
	}
	f.PrintJSON(registry)
	return nil
}

type staticBuilder struct {
	file *build.File
	f    *feature.Feature
}

func newStaticBuilder(file *build.File) *staticBuilder {
	return &staticBuilder{
		file: file,
		f:    &feature.Feature{},
	}
}

func (b *staticBuilder) build() (*feature.Feature, error) {
	kwargs, err := b.getFeatureKWArgs()
	if err != nil {
		return nil, err
	}
	for k, v := range kwargs {
		switch k {
		case descriptionAttrName:
			descriptionStr, ok := v.(*build.StringExpr)
			if !ok {
				return nil, fmt.Errorf("description kwarg: expected string, got %T", v)
			}
			b.f.Description = descriptionStr.Value
		case defaultValueAttrName:
			val, err := b.extractFeatureValue(v)
			if err != nil {
				return nil, errors.Wrap(err, "extract default feature value")
			}
			b.f.Value = val
		}
	}
	return b.feature(), nil
}

func (b *staticBuilder) feature() *feature.Feature {
	return b.f
}

func (b *staticBuilder) extractFeatureValue(v build.Expr) (interface{}, error) {
	switch t := v.(type) {
	case *build.Ident:
		switch t.Name {
		case "True":
			return true, nil
		case "False":
			return false, nil
		default:
			return nil, fmt.Errorf("unsupported identifier name %s", t.Name)
		}
	default:
		return nil, fmt.Errorf("unsupported type %T", v)
	}
}

func (b *staticBuilder) getFeatureKWArgs() (map[string]build.Expr, error) {
	ret := make(map[string]build.Expr)
	for _, expr := range b.file.Stmt {
		switch t := expr.(type) {
		case *build.AssignExpr:
			lhs, ok := t.LHS.(*build.Ident)
			if ok && lhs.Name == featureVariableName {
				rhs, ok := t.RHS.(*build.CallExpr)
				if ok && len(rhs.List) > 0 {
					structName, ok := rhs.X.(*build.Ident)
					if ok && structName.Name == featureConstructor.GoString() {
						for _, expr := range rhs.List {
							assignExpr, ok := expr.(*build.AssignExpr)
							if ok {
								kwargName, ok := assignExpr.LHS.(*build.Ident)
								if ok {
									ret[kwargName.Name] = assignExpr.RHS
								}
							}
						}
					}
				}
			}
		}
	}
	if len(ret) == 0 {
		return nil, fmt.Errorf("no feature kwargs found")
	}
	return ret, nil
}
