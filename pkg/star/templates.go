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

const starFmt = `result=feature(
	description="my feature description",
	default=%s
)
`

const protoStar = `pb = proto.package("google.protobuf")

result=feature(
	description="my feature description",
	default=pb.BoolValue(value=False)
)
`

func GetTemplate(fType feature.FeatureType) ([]byte, error) {
	switch fType {
	case feature.FeatureTypeBool:
		return []byte(fmt.Sprintf(starFmt, "False")), nil
	case feature.FeatureTypeInt:
		return []byte(fmt.Sprintf(starFmt, "1")), nil
	case feature.FeatureTypeFloat:
		return []byte(fmt.Sprintf(starFmt, "1.0")), nil
	case feature.FeatureTypeString:
		return []byte(fmt.Sprintf(starFmt, "''")), nil
	case feature.FeatureTypeJSON:
		return []byte(fmt.Sprintf(starFmt, "{}")), nil
	case feature.FeatureTypeProto:
		return []byte(protoStar), nil
	default:
		return nil, fmt.Errorf("templating is not supported for feature type %s", fType)
	}
}
