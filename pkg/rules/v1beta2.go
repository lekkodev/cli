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

	"github.com/lekkodev/rules/parser"
	"github.com/pkg/errors"
	"google.golang.org/protobuf/types/known/anypb"
)

func EvaluateFeatureV1Beta2(feature *featurev1beta1.Tree, context map[string]interface{}) (*anypb.Any, error) {
	for _, constraint := range feature.Constraints {
		retVal, err := traverseConstraint(constraint, context)
		if err != nil {
			return nil, err
		}
		if retVal != nil {
			return retVal, nil
		}
	}
	return feature.Default, nil
}

func traverseConstraint(constraint *featurev1beta1.Constraint, context map[string]interface{}) (*anypb.Any, error) {
	if len(constraint.Rule) == 0 {
		// if the rule is empty, then this is true
		return constraint.Value, nil
	}
	evaluator, err := parser.NewEvaluator(constraint.Rule)
	if err != nil {
		return nil, errors.Wrap(err, "creating evaluator")
	}

	passes, err := evaluator.Process(context)
	if err != nil {
		return nil, errors.Wrap(err, "processing")
	}
	if passes {
		if constraint.Value != nil {
			return constraint.Value, nil
		} else {
			// Traverse constraints
			for _, constraint := range constraint.Constraints {
				retVal, err := traverseConstraint(constraint, context)
				if err != nil {
					return nil, err
				}
				if retVal != nil {
					return retVal, nil
				}
			}
		}
	}
	return nil, nil
}
