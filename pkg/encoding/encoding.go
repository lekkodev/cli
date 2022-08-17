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

package encoding

import (
	"fmt"

	"github.com/lekkodev/cli/pkg/feature"
	featurev1beta1 "github.com/lekkodev/cli/pkg/gen/proto/go/lekko/feature/v1beta1"
	rulesv1beta1 "github.com/lekkodev/cli/pkg/gen/proto/go/lekko/rules/v1beta1"

	"google.golang.org/protobuf/encoding/protojson"
)

// Takes a version number and parses file contents into the corresponding
// type.
func ParseFeature(contents []byte, version string) (feature.EvaluableFeature, error) {
	switch version {
	case "v1beta1":
		var f rulesv1beta1.Feature
		err := protojson.Unmarshal(contents, &f)
		if err != nil {
			return nil, err
		}
		return feature.NewV1Beta1(&f), nil
	case "v1beta2":
		var f featurev1beta1.Feature
		err := protojson.Unmarshal(contents, &f)
		if err != nil {
			return nil, err
		}
		return feature.NewV1Beta2(&f), nil
	default:
		return nil, fmt.Errorf("unknown version when parsing feature: %s", version)
	}
}
