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

package repo

import (
	"context"
	"fmt"

	"github.com/lekkodev/cli/pkg/feature"
	"github.com/lekkodev/cli/pkg/metadata"
	"github.com/pkg/errors"
)

func (r *Repo) GetFeatureFiles(ctx context.Context, namespace string) ([]feature.FeatureFile, error) {
	_, nsMDs, err := metadata.ParseFullConfigRepoMetadataStrict(ctx, "", r)
	if err != nil {
		return nil, errors.Wrap(err, "parse root md")
	}
	nsMD, ok := nsMDs[namespace]
	if !ok {
		return nil, fmt.Errorf("namespace %s not found in root metadata", namespace)
	}
	ffs, err := feature.GroupFeatureFiles(ctx, namespace, nsMD, r, true)
	if err != nil {
		return nil, errors.Wrap(err, "group feature files")
	}
	return ffs, nil
}

func (r *Repo) GetFeatureFile(ctx context.Context, namespace, featureName string) (*feature.FeatureFile, error) {
	ffs, err := r.GetFeatureFiles(ctx, namespace)
	if err != nil {
		return nil, errors.Wrap(err, "get feature files")
	}
	var ff *feature.FeatureFile
	for _, file := range ffs {
		file := file
		if file.Name == featureName {
			ff = &file
			break
		}
	}
	if ff == nil {
		return nil, fmt.Errorf("feature '%s' not found in namespace '%s'", featureName, namespace)
	}
	return ff, nil
}
