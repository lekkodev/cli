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

	rulesv1beta1 "github.com/lekkodev/cli/pkg/gen/proto/go/lekko/rules/v1beta1"

	"google.golang.org/protobuf/encoding/protojson"
)

// Takes a version number and parses file contents into the corresponding
// type.
// TODO have this based on some sort of common internal representation.
func ParseFeature(contents []byte, version string) (*rulesv1beta1.Feature, error) {
	switch version {
	case "v1beta1":
		var feature rulesv1beta1.Feature
		err := protojson.Unmarshal(contents, &feature)
		if err != nil {
			return nil, err
		}
		return &feature, nil
	default:
		return nil, fmt.Errorf("unknown version when parsing feature: %s", version)
	}
}
