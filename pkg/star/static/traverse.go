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
	"github.com/lekkodev/cli/pkg/star"
	"github.com/pkg/errors"
)

func defaultNoop(v build.Expr) error            { return nil }
func descriptionNoop(v *build.StringExpr) error { return nil }
func rulesNoop(rules []rule) error              { return nil }

type rule struct {
	conditionV *build.StringExpr
	v          build.Expr
}

type defaultFn func(v build.Expr) error
type descriptionFn func(v *build.StringExpr) error
type rulesFn func(rules []rule) error

// Traverses a lekko starlark file, running methods on various
// components of the file. Methods can be provided to read the
// feature defined in the file, or even modify it.
type traverser struct {
	f *build.File

	defaultFn     defaultFn
	descriptionFn descriptionFn
	rulesFn       rulesFn
}

func newTraverser(f *build.File) *traverser {
	return &traverser{
		f:             f,
		defaultFn:     defaultNoop,
		descriptionFn: descriptionNoop,
		rulesFn:       rulesNoop,
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

func (t *traverser) traverse() error {
	kwargs, err := t.getFeatureKWArgs()
	if err != nil {
		return err
	}
	defaultExpr, ok := kwargs[star.DefaultValueAttrName]
	if !ok {
		return fmt.Errorf("could not find %s attribute", star.DefaultValueAttrName)
	}

	if err := t.defaultFn(defaultExpr); err != nil {
		return errors.Wrap(err, "default fn")
	}

	for k, v := range kwargs {
		switch k {
		case star.DescriptionAttrName:
			descriptionStr, ok := v.(*build.StringExpr)
			if !ok {
				return fmt.Errorf("description kwarg: expected string, got %T", v)
			}
			if err := t.descriptionFn(descriptionStr); err != nil {
				return errors.Wrap(err, "description fn")
			}
		case star.RulesAttrName:
			if err := t.parseRules(v); err != nil {
				return errors.Wrap(err, "parse rules")
			}
		}
	}
	return nil
}

func (t *traverser) parseRules(v build.Expr) error {
	listV, ok := v.(*build.ListExpr)
	if !ok {
		return fmt.Errorf("expecting list, got %T", v)
	}
	rules := make([]rule, len(listV.List))
	for i, elemV := range listV.List {
		tupleV, ok := elemV.(*build.TupleExpr)
		if !ok {
			return fmt.Errorf("rule #%d: expecting tuple, got %T", i, elemV)
		}
		if len(tupleV.List) != 2 {
			return fmt.Errorf("rule #%d: expecting tuple of length 2, got length %d", i, len(tupleV.List))
		}
		conditionV := tupleV.List[0]
		conditionStringV, ok := conditionV.(*build.StringExpr)
		if !ok {
			return fmt.Errorf("rule #%d: expecting condition string, got %T", i, conditionV)
		}
		rules[i] = rule{
			conditionV: conditionStringV,
			v:          tupleV.List[1],
		}
	}
	if err := t.rulesFn(rules); err != nil {
		return errors.Wrap(err, "rules fn")
	}
	return nil
}

// extracts a map of kwargs that are used to construct the resulting
// feature value. E.g.
// 		result = feature(description="foo", default=False)
// has two keys, each with a corresponding build expression.
func (t *traverser) getFeatureKWArgs() (map[string]build.Expr, error) {
	ret := make(map[string]build.Expr)
	for _, expr := range t.f.Stmt {
		switch t := expr.(type) {
		case *build.AssignExpr:
			lhs, ok := t.LHS.(*build.Ident)
			if ok && lhs.Name == star.FeatureVariableName {
				rhs, ok := t.RHS.(*build.CallExpr)
				if ok && len(rhs.List) > 0 {
					structName, ok := rhs.X.(*build.Ident)
					if ok && structName.Name == star.FeatureConstructor.GoString() {
						// we've reached the list of kwarg assignments
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

func (t *traverser) format() []byte {
	return build.FormatWithoutRewriting(t.f)
}
