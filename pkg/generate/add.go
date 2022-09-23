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
	if featureName == "" {
		fmt.Printf("Your new namespace has been created: %v\n", filepath.Join(rootPath, namespaceName))
		return nil
	}
	ffs, err := feature.GroupFeatureFiles(ctx, filepath.Join(rootPath, namespaceName), provider)
	if err != nil {
		return fmt.Errorf("failed to group feature files: %v", err)
	}
	for _, ff := range ffs {
		if err := feature.ComplianceCheck(ff, nsMD); err != nil {
			return fmt.Errorf("compliance check for feature %s: %w", ff.Name, err)
		}
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

func RemoveFeature(rootPath, namespaceName, featureName string) error {
	ctx := context.TODO()
	provider := fs.LocalProvider()
	cw := fs.LocalConfigWriter()
	_, err := metadata.ParseNamespaceMetadataStrict(ctx, rootPath, namespaceName, provider)
	if err != nil {
		return fmt.Errorf("error parsing namespace metadata: %v", err)
	}
	var removed bool
	for _, file := range []string{
		fmt.Sprintf("%s.star", featureName),
		filepath.Join(metadata.GenFolderPathJSON, fmt.Sprintf("%s.json", featureName)),
		filepath.Join(metadata.GenFolderPathProto, fmt.Sprintf("%s.proto.bin", featureName)),
	} {
		ok, err := cw.RemoveIfExists(filepath.Join(rootPath, namespaceName, file))
		if err != nil {
			return fmt.Errorf("remove if exists failed to remove %s: %v", file, err)
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

func RemoveNamespace(rootPath, namespaceName string) error {
	ctx := context.TODO()
	provider := fs.LocalProvider()
	cw := fs.LocalConfigWriter()
	_, err := metadata.ParseNamespaceMetadataStrict(ctx, rootPath, namespaceName, provider)
	if err != nil {
		return fmt.Errorf("error parsing namespace metadata: %v", err)
	}
	ok, err := cw.RemoveIfExists(filepath.Join(rootPath, namespaceName))
	if err != nil {
		return fmt.Errorf("failed to remove namespace %s: %v", namespaceName, err)
	}
	if !ok {
		fmt.Printf("Namespace %s does not exist\n", namespaceName)
	}
	if err := metadata.UpdateRootConfigRepoMetadata(ctx, rootPath, provider, cw, func(rcrm *metadata.RootConfigRepoMetadata) {
		var updatedNamespaces []string
		for _, ns := range rcrm.Namespaces {
			if ns != namespaceName {
				updatedNamespaces = append(updatedNamespaces, ns)
			}
		}
		rcrm.Namespaces = updatedNamespaces
	}); err != nil {
		return fmt.Errorf("failed to update root config md: %v", err)
	}
	fmt.Printf("Namespace %s has been removed\n", namespaceName)
	return nil
}
