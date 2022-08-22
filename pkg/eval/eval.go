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

package eval

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"golang.org/x/exp/slices"

	"github.com/lekkodev/cli/pkg/encoding"
	"github.com/lekkodev/cli/pkg/feature"
	"github.com/lekkodev/cli/pkg/fs"
	"github.com/lekkodev/cli/pkg/metadata"
	"google.golang.org/protobuf/types/known/anypb"
)

// Evaluates a provided feature given a provided context.
func Eval(rootPath string, featurePath string, iCtx map[string]interface{}) (*anypb.Any, error) {
	_, nsNameToNsMDs, err := metadata.ParseFullConfigRepoMetadataStrict(context.TODO(), rootPath, fs.LocalProvider())
	if err != nil {
		return nil, err
	}

	splits := strings.SplitN(featurePath, "/", 2)
	if len(splits) != 2 {
		return nil, fmt.Errorf("invalid featurepath: %s, should be of format namespace/feature", featurePath)
	}

	ns := splits[0]
	featureName := splits[1]

	nsMD, ok := nsNameToNsMDs[ns]
	if !ok {
		return nil, fmt.Errorf("invalid namespace: %s, should be of format namespace/feature", ns)
	}

	groupedFeatures, err := feature.GroupFeatureFiles(context.Background(), filepath.Join(rootPath, ns), nsMD, fs.LocalProvider())
	if err != nil {
		return nil, err
	}

	idx := slices.IndexFunc(groupedFeatures, func(c feature.FeatureFile) bool { return c.Name == featureName })
	if idx < 0 {
		return nil, fmt.Errorf("invalid featurePath: %s, feature %s was not found in namespace %s", featurePath, featureName, ns)
	}
	feature := groupedFeatures[idx]
	evalF, err := encoding.ParseFeature(rootPath, feature, nsMD, fs.LocalProvider())
	if err != nil {
		return nil, err
	}

	return evalF.Evaluate(iCtx)
}
