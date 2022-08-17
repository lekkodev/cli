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
	"context"
	"fmt"
	"path/filepath"

	"github.com/lekkodev/cli/pkg/feature"
	"github.com/lekkodev/cli/pkg/fs"
	featurev1beta1 "github.com/lekkodev/cli/pkg/gen/proto/go/lekko/feature/v1beta1"
	rulesv1beta1 "github.com/lekkodev/cli/pkg/gen/proto/go/lekko/rules/v1beta1"
	"github.com/lekkodev/cli/pkg/metadata"

	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
)

// Takes a version number and parses file contents into the corresponding
// type.
func ParseFeature(rootPath string, featureFile feature.FeatureFile, nsMD *metadata.NamespaceConfigRepoMetadata, provider fs.Provider) (feature.EvaluableFeature, error) {
	switch nsMD.Version {
	case "v1beta1":
		var f rulesv1beta1.Feature
		contents, err := provider.GetFileContents(context.TODO(), filepath.Join(rootPath, nsMD.Name, featureFile.CompiledJSONFileName))
		if err != nil {
			return nil, err
		}
		if err := protojson.Unmarshal(contents, &f); err != nil {
			return nil, err
		}
		return feature.NewV1Beta1(&f), nil
	case "v1beta2":
		var f featurev1beta1.Feature
		contents, err := provider.GetFileContents(context.TODO(), filepath.Join(rootPath, nsMD.Name, featureFile.CompiledProtoBinFileName))
		if err != nil {
			return nil, err
		}
		if err := proto.Unmarshal(contents, &f); err != nil {
			return nil, err
		}
		return feature.NewV1Beta2(&f), nil
	default:
		return nil, fmt.Errorf("unknown version when parsing feature: %s", nsMD.Version)
	}
}
