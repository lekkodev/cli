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
	"github.com/pkg/errors"
	"go.starlark.net/starlark"
)

const (
	FeatureConstructor   starlark.String = "feature"
	FeatureVariableName  string          = "result"
	DefaultValueAttrName string          = "default"
	DescriptionAttrName  string          = "description"
	RulesAttrName        string          = "rules"
	InputTypeAuto        string          = "auto"
)

func defaultNoop(v *build.Expr) error           { return nil }
func descriptionNoop(v *build.StringExpr) error { return nil }
func rulesNoop(rules *rulesWrapper) error       { return nil }
func importsNoop(imports *importsWrapper) error { return nil }

type defaultFn func(v *build.Expr) error
type descriptionFn func(v *build.StringExpr) error
type rulesFn func(rules *rulesWrapper) error
type importsFn func(imports *importsWrapper) error

// Traverses a lekko starlark file, running methods on various
// components of the file. Methods can be provided to read the
// feature defined in the file, or even modify it.
type traverser struct {
	f *build.File

	defaultFn      defaultFn
	descriptionFn  descriptionFn
	rulesFn        rulesFn
	protoImportsFn importsFn
}

func newTraverser(f *build.File) *traverser {
	return &traverser{
		f:              f,
		defaultFn:      defaultNoop,
		descriptionFn:  descriptionNoop,
		rulesFn:        rulesNoop,
		protoImportsFn: importsNoop,
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

func (t *traverser) withRulesFn(fn rulesFn) *traverser {
	t.rulesFn = fn
	return t
}

func (t *traverser) withProtoImportsFn(fn importsFn) *traverser {
	t.protoImportsFn = fn
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
	defaultExpr, err := ast.get(DefaultValueAttrName)
	if err != nil {
		return err
	}
	if err := t.defaultFn(defaultExpr); err != nil {
		return errors.Wrap(err, "default fn")
	}
	descriptionExprPtr, err := ast.get(DescriptionAttrName)
	if err != nil {
		return err
	}
	descriptionExpr := *descriptionExprPtr
	descriptionStr, ok := descriptionExpr.(*build.StringExpr)
	if !ok {
		return errors.Wrapf(ErrUnsupportedStaticParsing, "description kwarg: expected string, got %T", descriptionExpr)
	}
	if err := t.descriptionFn(descriptionStr); err != nil {
		return errors.Wrap(err, "description fn")
	}
	// rules
	if err := ast.parseRules(t.rulesFn); err != nil {
		return err
	}
	return nil
}

// A wrapper type around the feature AST, e.g.
// `feature(description="foo", default=False)`
type starFeatureAST struct {
	*build.CallExpr
}

func (ast *starFeatureAST) get(key string) (*build.Expr, error) {
	for _, expr := range ast.List {
		assignExpr, ok := expr.(*build.AssignExpr)
		if ok {
			kwargName, ok := assignExpr.LHS.(*build.Ident)
			if ok {
				if kwargName.Name == key {
					return &assignExpr.RHS, nil
				}
			}
		}
	}
	return nil, errors.Errorf("could not find '%s' attribute in feature", key)
}

func (ast *starFeatureAST) set(key string, value build.Expr) {
	if existing, err := ast.get(key); err == nil {
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

func (ast *starFeatureAST) parseRules(fn rulesFn) error {
	rulesW := &rulesWrapper{}
	rulesExprPtr, err := ast.get(RulesAttrName)
	if err == nil { // extract existing rules
		v := *rulesExprPtr
		listV, ok := v.(*build.ListExpr)
		if !ok {
			return errors.Wrapf(ErrUnsupportedStaticParsing, "expecting list, got %T", v)
		}
		for i, elemV := range listV.List {
			r, err := newRule(elemV)
			if err != nil {
				return errors.Wrapf(err, "rule %d", i)
			}
			rulesW.rules = append(rulesW.rules, *r)
		}
	}
	if err := fn(rulesW); err != nil {
		return err
	}
	if len(rulesW.rules) == 0 {
		ast.unset(RulesAttrName)
		return nil
	}
	var newList []build.Expr
	for _, rule := range rulesW.rules {
		newList = append(newList, rule.toExpr())
	}
	ast.set(RulesAttrName, &build.ListExpr{
		List:           newList,
		ForceMultiLine: true,
	})
	return nil
}

// extracts a pointer to the feature AST in starlark.
func (t *traverser) getFeatureAST() (*starFeatureAST, error) {
	for _, expr := range t.f.Stmt {
		switch t := expr.(type) {
		case *build.AssignExpr:
			lhs, ok := t.LHS.(*build.Ident)
			if ok && lhs.Name == FeatureVariableName {
				rhs, ok := t.RHS.(*build.CallExpr)
				if ok && len(rhs.List) > 0 {
					structName, ok := rhs.X.(*build.Ident)
					if ok && structName.Name == FeatureConstructor.GoString() {
						return &starFeatureAST{rhs}, nil
					}
				}
			}
		}
	}
	return nil, fmt.Errorf("no feature found in star file")
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

type rulesWrapper struct {
	rules []rule
}

type rule struct {
	conditionV *build.StringExpr
	v          build.Expr
}

type importsWrapper struct {
	imports []importVal
}

type importVal struct {
	assignExpr *build.AssignExpr
}

func newRule(li build.Expr) (*rule, error) {
	tupleV, ok := li.(*build.TupleExpr)
	if !ok {
		return nil, errors.Wrapf(ErrUnsupportedStaticParsing, "expecting tuple, got %T", li)
	}
	if len(tupleV.List) != 2 {
		return nil, errors.Wrapf(ErrUnsupportedStaticParsing, "expecting tuple of length 2, got length %d", len(tupleV.List))
	}
	conditionV := tupleV.List[0]
	conditionStringV, ok := conditionV.(*build.StringExpr)
	if !ok {
		return nil, errors.Wrapf(ErrUnsupportedStaticParsing, "expecting condition string, got %T", conditionV)
	}
	return &rule{
		conditionV: conditionStringV,
		v:          tupleV.List[1],
	}, nil
}

func (r *rule) toExpr() build.Expr {
	return &build.TupleExpr{
		List: []build.Expr{r.conditionV, r.v},
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

func (r *rule) String() string {
	return fmt.Sprintf("c: '%s', v: '%v'", r.conditionV.Value, r.v)
}
