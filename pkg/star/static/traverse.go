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

package static

import (
	"fmt"

	"github.com/bazelbuild/buildtools/build"
	"github.com/lekkodev/cli/pkg/feature"
	"github.com/pkg/errors"
	"go.starlark.net/starlark"
)

const (
	FeatureConstructor   starlark.String = "feature"
	ExportConstructor    starlark.String = "export"
	ConfigConstructor    starlark.String = "Config"
	ResultVariableName   string          = "result"
	DefaultValueAttrName string          = "default"
	DescriptionAttrName  string          = "description"
	MetadataAttrName     string          = "metadata"
	// TODO: Fully migrate to overrides over rules
	RulesAttrName     string = "rules"
	OverridesAttrName string = "overrides"
	InputTypeAuto     string = "auto"
)

func defaultNoop(v *build.Expr) error           { return nil }
func descriptionNoop(v *build.StringExpr) error { return nil }
func rulesNoop(rules *overridesWrapper) error   { return nil }
func importsNoop(imports *importsWrapper) error { return nil }
func metadataNoop(ast *starFeatureAST) error { return nil}

type defaultFn func(v *build.Expr) error
type descriptionFn func(v *build.StringExpr) error
type overridesFn func(rules *overridesWrapper) error
type importsFn func(imports *importsWrapper) error
type metadataFn func(ast *starFeatureAST) error

// Traverses a lekko starlark file, running methods on various
// components of the file. Methods can be provided to read the
// feature defined in the file, or even modify it.
type traverser struct {
	f  *build.File
	nv feature.NamespaceVersion

	defaultFn      defaultFn
	descriptionFn  descriptionFn
	overridesFn    overridesFn
	protoImportsFn importsFn
	metadataFn     metadataFn
}

func newTraverser(f *build.File, nv feature.NamespaceVersion) *traverser {
	return &traverser{
		f:              f,
		nv:             nv,
		defaultFn:      defaultNoop,
		descriptionFn:  descriptionNoop,
		overridesFn:    rulesNoop,
		protoImportsFn: importsNoop,
		metadataFn: metadataNoop,
	}
}

func (t *traverser) withDefaultFn(fn defaultFn) *traverser {
	t.defaultFn = fn
	return t
}

func (t *traverser) withDescriptionFn(fn descriptionFn) *traverser {
	t.descriptionFn = fn
	return t
}

func (t *traverser) withOverridesFn(fn overridesFn) *traverser {
	t.overridesFn = fn
	return t
}

func (t *traverser) withProtoImportsFn(fn importsFn) *traverser {
	t.protoImportsFn = fn
	return t
}

func (t *traverser) withMetadataFn(fn metadataFn) *traverser {
	t.metadataFn = fn
	return t
} 

func (t *traverser) traverse() error {
	imports := t.getProtoImports()
	if err := t.protoImportsFn(imports); err != nil {
		return errors.Wrap(err, "imports fn")
	}
	ast, err := t.getFeatureAST()
	if err != nil {
		return err
	}
	defaultExpr, found := ast.get(DefaultValueAttrName)
	if !found {
		return errors.New("missing config default value")
	}
	if err := t.defaultFn(defaultExpr); err != nil {
		return errors.Wrap(err, "default fn")
	}
	descriptionExprPtr, found := ast.get(DescriptionAttrName)
	if !found {
		return errors.New("missing config description")
	}
	descriptionExpr := *descriptionExprPtr
	descriptionStr, ok := descriptionExpr.(*build.StringExpr)
	if !ok {
		return errors.Wrapf(ErrUnsupportedStaticParsing, "description kwarg: expected string, got %T", descriptionExpr)
	}
	if err := t.descriptionFn(descriptionStr); err != nil {
		return errors.Wrap(err, "description fn")
	}
	if err := t.metadataFn(ast); err != nil {
		return errors.Wrap(err, "metadata fn")
	}
	// rules
	if err := ast.parseOverrides(t.overridesFn, t.nv); err != nil {
		return err
	}
	return nil
}

// A wrapper type around the config AST, e.g.
// `Config(description="foo", default=False)`
type starFeatureAST struct {
	*build.CallExpr
}

// Returns true as the second value if key is in AST, false otherwise
func (ast *starFeatureAST) get(key string) (*build.Expr, bool) {
	for _, expr := range ast.List {
		assignExpr, ok := expr.(*build.AssignExpr)
		if ok {
			kwargName, ok := assignExpr.LHS.(*build.Ident)
			if ok {
				if kwargName.Name == key {
					return &assignExpr.RHS, true
				}
			}
		}
	}
	return nil, false
}

func (ast *starFeatureAST) set(key string, value build.Expr) {
	if existing, found := ast.get(key); found {
		*existing = value
		return
	}
	assignExpr := &build.AssignExpr{
		LHS:       &build.Ident{Name: key},
		Op:        "=",
		LineBreak: false,
		RHS:       value,
	}
	ast.List = append(ast.List, assignExpr)
}

func (ast *starFeatureAST) unset(key string) {
	newAssignmentList := make([]build.Expr, 0)
	for _, expr := range ast.List {
		assignExpr, ok := expr.(*build.AssignExpr)
		if ok {
			kwargName, ok := assignExpr.LHS.(*build.Ident)
			if ok {
				if kwargName.Name == key {
					continue
				}
				newAssignmentList = append(newAssignmentList, expr)
			}
		}
	}
	ast.List = newAssignmentList
}

func (ast *starFeatureAST) parseOverrides(fn overridesFn, nv feature.NamespaceVersion) error {
	overridesW := &overridesWrapper{}
	// By default, assume used "overrides" and not "rules"
	usedOverrides := true
	usedRules := false
	overridesExprPtr, overridesFound := ast.get(OverridesAttrName)
	if !overridesFound {
		// Fall back to rules
		// TODO: fully migrate to overrides instead of rules
		usedOverrides = false
		overridesExprPtr, overridesFound = ast.get(RulesAttrName)
		if overridesFound {
			usedRules = true
		}
	} else {
		// Overrides and rules should not be set at the same time
		if _, rulesFound := ast.get(RulesAttrName); rulesFound {
			return errors.New("overrides and rules should not both be present")
		}
	}
	if overridesFound { // extract existing rules
		v := *overridesExprPtr
		listV, ok := v.(*build.ListExpr)
		if !ok {
			return errors.Wrapf(ErrUnsupportedStaticParsing, "expecting list, got %T", v)
		}
		for i, elemV := range listV.List {
			o, err := newOverride(elemV)
			if err != nil {
				return errors.Wrapf(err, "override %d", i)
			}
			overridesW.overrides = append(overridesW.overrides, *o)
		}
	}
	if err := fn(overridesW); err != nil {
		return err
	}
	if len(overridesW.overrides) == 0 {
		ast.unset(OverridesAttrName)
		ast.unset(RulesAttrName)
		return nil
	}
	var newList []build.Expr
	for _, override := range overridesW.overrides {
		newList = append(newList, override.toExpr())
	}
	// Updated attribute name determined by which was used and config version
	var setAttrName string
	if usedOverrides {
		setAttrName = OverridesAttrName
	} else if usedRules {
		setAttrName = RulesAttrName
	} else if nv >= feature.NamespaceVersionV1Beta6 {
		setAttrName = OverridesAttrName
	} else {
		setAttrName = RulesAttrName
	}
	ast.set(setAttrName, &build.ListExpr{
		List:           newList,
		ForceMultiLine: true,
	})
	return nil
}

func tryExtractConfigAST(expr build.Expr) (*starFeatureAST, bool) {
	config, ok := expr.(*build.CallExpr)
	if ok && len(config.List) > 0 {
		structName, ok := config.X.(*build.Ident)
		if ok && (structName.Name == ConfigConstructor.GoString() || structName.Name == FeatureConstructor.GoString()) {
			return &starFeatureAST{config}, true
		}
	}
	return nil, false
}

// extracts a pointer to the feature AST in starlark.
func (t *traverser) getFeatureAST() (*starFeatureAST, error) {
	for _, expr := range t.f.Stmt {
		switch t := expr.(type) {
		case *build.CallExpr:
			if len(t.List) == 1 {
				funcName, ok := t.X.(*build.Ident)
				if ok && funcName.Name == ExportConstructor.GoString() {
					config, ok := tryExtractConfigAST(t.List[0])
					if ok {
						return config, nil
					}
				}
			}
		case *build.AssignExpr:
			lhs, ok := t.LHS.(*build.Ident)
			if ok && lhs.Name == ResultVariableName {
				rhs, ok := t.RHS.(*build.CallExpr)
				if ok {
					config, ok := tryExtractConfigAST(rhs)
					if ok {
						return config, nil
					}
				}
			}
		}
	}
	return nil, fmt.Errorf("no config found in star file")
}

func (t *traverser) getProtoImports() *importsWrapper {
	ret := &importsWrapper{}
	for _, expr := range t.f.Stmt {
		switch t := expr.(type) {
		case *build.AssignExpr:
			rhs, ok := t.RHS.(*build.CallExpr)
			if ok {
				dotExpr, ok := rhs.X.(*build.DotExpr)
				if ok {
					x, ok := dotExpr.X.(*build.Ident)
					if ok && x.Name == "proto" && dotExpr.Name == "package" {
						ret.imports = append(ret.imports, importVal{
							assignExpr: t,
						})
					}
				}
			}
		}
	}
	return ret
}

func (t *traverser) format() []byte {
	return build.FormatWithoutRewriting(t.f)
}

type overridesWrapper struct {
	overrides []override
}

type override struct {
	ruleV *build.StringExpr
	v     build.Expr
}

type importsWrapper struct {
	imports []importVal
}

type importVal struct {
	assignExpr *build.AssignExpr
}

type metadataWrapper struct {
	metadataExpr *build.DictExpr
}

func newOverride(li build.Expr) (*override, error) {
	tupleV, ok := li.(*build.TupleExpr)
	if !ok {
		return nil, errors.Wrapf(ErrUnsupportedStaticParsing, "expecting tuple, got %T", li)
	}
	if len(tupleV.List) != 2 {
		return nil, errors.Wrapf(ErrUnsupportedStaticParsing, "expecting tuple of length 2, got length %d", len(tupleV.List))
	}
	ruleV := tupleV.List[0]
	ruleStringV, ok := ruleV.(*build.StringExpr)
	if !ok {
		return nil, errors.Wrapf(ErrUnsupportedStaticParsing, "expecting rule string, got %T", ruleV)
	}
	return &override{
		ruleV: ruleStringV,
		v:     tupleV.List[1],
	}, nil
}

func (r *override) toExpr() build.Expr {
	return &build.TupleExpr{
		List: []build.Expr{r.ruleV, r.v},
		// TODO: expose the following fields in the mutator to make them round-trip safe
		Comments: build.Comments{
			Before: nil,
			Suffix: nil,
			After:  nil,
		},
		NoBrackets:     false,
		ForceCompact:   true,
		ForceMultiLine: false,
	}
}

func (r *override) String() string {
	return fmt.Sprintf("r: '%s', v: '%v'", r.ruleV.Value, r.v)
}
