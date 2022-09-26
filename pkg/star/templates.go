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

	"github.com/lekkodev/cli/pkg/feature"
)

const simpleStar = `result=feature(
	description="my feature description",
	default=False
)
`

const complexStar = `load("assert.star", "assert")

pb = proto.package("google.protobuf")

description = "my feature description"
default = pb.BoolValue(value=False)
# Provide a list of rules/conditions/exceptions for this feature flag.
# Rules are evaluated in order from start to end.
rules = [("age > 12", pb.BoolValue(value=True))]
# The validator allows you to define invariants. It is a function that takes 
# your feature value and raises errors with the help of the starlarktest module:
# https://github.com/google/starlark-go/blob/master/starlarktest/assert.go
# The validator is run on the default value as well as every rule value.
def validator(val):
	assert.true(val.value or not val.value)

result=feature(
	description=description,
	default=default,
	rules=rules,
	validator=validator
)
`

func GetTemplate(fType feature.FeatureType) ([]byte, error) {
	switch fType {
	case feature.FeatureTypeBool:
		return []byte(simpleStar), nil
	case feature.FeatureTypeProto:
		return []byte(complexStar), nil
	default:
		return nil, fmt.Errorf("templating is not supported for feature type %s", fType)
	}
}
