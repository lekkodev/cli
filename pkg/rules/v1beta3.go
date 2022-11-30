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

package rules

import (
	featurev1beta1 "github.com/lekkodev/cli/pkg/gen/proto/go/lekko/feature/v1beta1"

	"github.com/lekkodev/rules/pkg/parser"
	"github.com/pkg/errors"
	"google.golang.org/protobuf/types/known/anypb"
)

func EvaluateFeatureV1Beta3(feature *featurev1beta1.Tree, context map[string]interface{}) (*anypb.Any, []int, error) {
	for i, constraint := range feature.Constraints {
		childVal, childPasses, childPath, err := traverseConstraint(constraint, context)
		if err != nil {
			return nil, []int{}, err
		}
		if childPasses {
			if childVal != nil {
				return childVal, append([]int{i}, childPath...), nil
			}
			break
		}
	}
	return feature.Default, []int{}, nil
}

func traverseConstraint(constraint *featurev1beta1.Constraint, context map[string]interface{}) (*anypb.Any, bool, []int, error) {
	passes, err := evaluate(constraint.GetRule(), context)
	if err != nil {
		return nil, false, []int{}, errors.Wrap(err, "processing")
	}
	if !passes {
		// If the rule fails, we avoid further traversal
		return nil, passes, []int{}, nil
	}
	// rule passed
	retVal := constraint.Value // may be null
	for i, child := range constraint.GetConstraints() {
		childVal, childPasses, childPath, err := traverseConstraint(child, context)
		if err != nil {
			return nil, false, []int{}, errors.Wrapf(err, "traverse %d", i)
		}
		if childPasses {
			// We may stop iterating. But first, remember the traversed
			// value if it exists
			if childVal != nil {
				return childVal, passes, append([]int{i}, childPath...), nil
			}
			break
		}
		// Child evaluation did not pass, continue iterating
	}
	return retVal, passes, []int{}, nil
}

func evaluate(rule string, context map[string]interface{}) (bool, error) {
	if len(rule) == 0 {
		// empty rule evaluates to 'true'
		return true, nil
	}
	evaluator, err := parser.NewEvaluator(rule)
	if err != nil {
		return false, errors.Wrap(err, "creating evaluator")
	}
	passes, err := evaluator.Process(context)
	if err != nil {
		return false, errors.Wrap(err, "processing")
	}
	return passes, nil
}
