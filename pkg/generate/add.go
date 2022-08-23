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

package generate

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/lekkodev/cli/pkg/feature"
	"github.com/lekkodev/cli/pkg/fs"
	"github.com/lekkodev/cli/pkg/metadata"
	"github.com/lekkodev/cli/pkg/star"
)

func Add(rootPath, namespaceName, featureName string, complexFeature bool) error {
	ctx := context.TODO()
	provider := fs.LocalProvider()
	cw := fs.LocalConfigWriter()
	nsMD, err := metadata.ParseNamespaceMetadataStrict(ctx, rootPath, namespaceName, provider)
	if err != nil && errors.Is(err, os.ErrNotExist) {
		if err := metadata.CreateNamespaceMetadata(ctx, rootPath, namespaceName, provider, cw); err != nil {
			return err
		}
	} else if err != nil {
		return fmt.Errorf("error parsing namespace metadata: %v", err)
	}
	ffs, err := feature.GroupFeatureFiles(ctx, filepath.Join(rootPath, namespaceName), nsMD, provider, true)
	if err != nil {
		return fmt.Errorf("failed to group feature files: %v", err)
	}
	for _, ff := range ffs {
		if ff.Name == featureName {
			return fmt.Errorf("feature named %s already exists", featureName)
		}
	}

	featurePath := filepath.Join(rootPath, namespaceName, fmt.Sprintf("%s.star", featureName))
	if err := cw.WriteFile(featurePath, star.GetTemplate(complexFeature), 0600); err != nil {
		return fmt.Errorf("failed to add feature: %v", err)
	}
	fmt.Printf("Your new feature has been written to %s\n", featurePath)
	fmt.Printf("Make your changes, and run 'lekko compile'.\n")
	return nil
}

func Remove(rootPath, namespaceName, featureName string) error {
	ctx := context.TODO()
	provider := fs.LocalProvider()
	cw := fs.LocalConfigWriter()
	_, err := metadata.ParseNamespaceMetadataStrict(ctx, rootPath, namespaceName, provider)
	if err != nil {
		return fmt.Errorf("error parsing namespace metadata: %v", err)
	}
	var removed bool
	for _, filename := range []string{
		fmt.Sprintf("%s.star", featureName),
		fmt.Sprintf("%s.json", featureName),
		fmt.Sprintf("%s.proto.bin", featureName),
	} {
		ok, err := cw.RemoveIfExists(filepath.Join(rootPath, namespaceName, filename))
		if err != nil {
			return fmt.Errorf("remove if exists failed to remove %s: %v", filename, err)
		}
		if ok {
			removed = true
		}
	}
	if removed {
		fmt.Printf("Feature %s has been removed\n", featureName)
	} else {
		fmt.Printf("Feature %s does not exist\n", featureName)
	}
	return nil
}
