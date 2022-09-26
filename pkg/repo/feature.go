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
	"os"
	"path/filepath"

	"github.com/lekkodev/cli/pkg/encoding"
	"github.com/lekkodev/cli/pkg/feature"
	"github.com/lekkodev/cli/pkg/metadata"
	"github.com/lekkodev/cli/pkg/star"
	"github.com/pkg/errors"
	"google.golang.org/protobuf/reflect/protoregistry"
	"google.golang.org/protobuf/types/known/anypb"
)

func (r *Repo) Compile(ctx context.Context, registry *protoregistry.Types) error {
	var err error
	if registry == nil {
		rootMD, _, err := r.ParseMetadata(ctx)
		if err != nil {
			return errors.Wrap(err, "parse metadata")
		}
		registry, err = r.BuildDynamicTypeRegistry(ctx, rootMD.ProtoDirectory)
		if err != nil {
			return errors.Wrap(err, "build dynamic type registry")
		}
	}
	contents, err := r.GetContents(ctx)
	if err != nil {
		return errors.Wrap(err, "get contents")
	}
	for nsMD, ffs := range contents {
		for _, ff := range ffs {
			if _, err := r.CompileFeature(ctx, registry, nsMD.Name, ff.Name, true); err != nil {
				return errors.Wrap(err, "compile feature")
			}
		}
	}
	return nil
}

func (r *Repo) CompileNamespace(ctx context.Context, registry *protoregistry.Types, namespace string) error {
	ffs, err := r.GetFeatureFiles(ctx, namespace)
	if err != nil {
		return errors.Wrap(err, "get feature files")
	}
	if registry == nil {
		rootMD, _, err := r.ParseMetadata(ctx)
		if err != nil {
			return errors.Wrap(err, "parse metadata")
		}
		registry, err = r.BuildDynamicTypeRegistry(ctx, rootMD.ProtoDirectory)
		if err != nil {
			return errors.Wrap(err, "build dynamic type registry")
		}
	}
	for _, ff := range ffs {
		if _, err := r.CompileFeature(ctx, registry, namespace, ff.Name, true); err != nil {
			return errors.Wrap(err, "compile feature")
		}
	}
	return nil
}

func (r *Repo) CompileFeature(ctx context.Context, registry *protoregistry.Types, namespace, featureName string, persist bool) (*feature.Feature, error) {
	fc, err := r.GetFeatureContents(ctx, namespace, featureName)
	if err != nil {
		return nil, errors.Wrap(err, "get feature contents")
	}
	if registry == nil {
		rootMD, _, err := r.ParseMetadata(ctx)
		if err != nil {
			return nil, errors.Wrap(err, "parse metadata")
		}
		registry, err = r.BuildDynamicTypeRegistry(ctx, rootMD.ProtoDirectory)
		if err != nil {
			return nil, errors.Wrap(err, "build dynamic type registry")
		}
	}
	compiler := star.NewCompiler(registry, fc.File, r)
	f, err := compiler.Compile()
	if err != nil {
		return nil, errors.Wrap(err, "compile")
	}
	if persist {
		if err := compiler.Persist(ctx, f); err != nil {
			return nil, errors.Wrap(err, "persist")
		}
		if err := r.Format(ctx); err != nil {
			return nil, errors.Wrap(err, "format")
		}
	}
	return f, nil
}

func (r *Repo) BuildDynamicTypeRegistry(ctx context.Context, protoDirPath string) (*protoregistry.Types, error) {
	return star.BuildDynamicTypeRegistry(protoDirPath, r)
}

// Actually regenerates the buf image, and writes it to the file system.
// Note: we don't have a way yet to run this from an ephemeral repo,
// because we need to first ensure that buf cmd line can be executed in the
// ephemeral env.
func (r *Repo) ReBuildDynamicTypeRegistry(ctx context.Context, protoDirPath string) (*protoregistry.Types, error) {
	return star.ReBuildDynamicTypeRegistry(protoDirPath, r)
}

func (r *Repo) Verify(ctx context.Context) error {
	rootMD, nsMDs, err := r.ParseMetadata(ctx)
	if err != nil {
		return errors.Wrap(err, "parse metadata")
	}
	registry, err := r.BuildDynamicTypeRegistry(ctx, rootMD.ProtoDirectory)
	if err != nil {
		return errors.Wrap(err, "build dynamic type registry")
	}
	for ns, nsMD := range nsMDs {
		ffs, err := r.GetFeatureFiles(ctx, ns)
		if err != nil {
			return errors.Wrap(err, "get feature files")
		}
		for _, ff := range ffs {
			ff := ff
			if err := r.verifyFeature(ctx, registry, ns, ff, nsMD); err != nil {
				return fmt.Errorf("verify feature %s/%s: %w", ns, ff.Name, err)
			}
		}
	}
	// lint protos
	if r.bufEnabled {
		if err := star.Lint(rootMD.ProtoDirectory); err != nil {
			return errors.Wrap(err, "lint protos")
		}
	}
	return nil
}

func (r *Repo) verifyFeature(
	ctx context.Context,
	registry *protoregistry.Types,
	ns string,
	ff feature.FeatureFile,
	nsMD *metadata.NamespaceConfigRepoMetadata,
) error {
	if err := feature.ComplianceCheck(ff, nsMD); err != nil {
		return errors.Wrap(err, "compliance check")
	}
	if err := ff.Verify(); err != nil {
		return errors.Wrap(err, "verify feature file")
	}
	if _, err := encoding.ParseFeature(ctx, "", ff, nsMD, r); err != nil {
		return errors.Wrap(err, "parse feature")
	}
	if nsMD.Version == metadata.LatestNamespaceVersion {
		f, err := r.CompileFeature(ctx, registry, ns, ff.Name, false)
		if err != nil {
			return errors.Wrap(err, "compile feature")
		}
		if len(f.UnitTests) > 0 {
			r.Logf("running %d unit tests for feature %s/%s: ", len(f.UnitTests), ns, ff.Name)
			if err := f.RunUnitTests(registry); err != nil {
				r.Logf("FAIL: %v\n", err)
				return errors.Wrap(err, "failed unit test(s)")
			} else {
				r.Logf("PASS\n")
			}
		}
	}
	return nil
}

func (r *Repo) Format(ctx context.Context) error {
	_, nsMDs, err := r.ParseMetadata(ctx)
	if err != nil {
		return errors.Wrap(err, "parse metadata")
	}
	for ns, nsMD := range nsMDs {
		if nsMD.Version != metadata.LatestNamespaceVersion {
			r.Logf("Skipping namespace %s since version %s doesn't conform to compilation\n", ns, nsMD.Version)
			continue
		}
		ffs, err := r.GetFeatureFiles(ctx, ns)
		if err != nil {
			return errors.Wrap(err, "get feature files")
		}

		for _, ff := range ffs {
			formatter := star.NewStarFormatter(ff.RootPath(ff.StarlarkFileName), ff.Name, r)
			ok, err := formatter.Format()
			if err != nil {
				return fmt.Errorf("star format %s/%s: %w", ns, ff.Name, err)
			}
			if ok {
				r.Logf("Formatted and rewrote %s/%s\n", ns, ff.Name)
			}
		}
	}
	return nil
}

func (r *Repo) Add(ctx context.Context, ns, featureName string, fType feature.FeatureType) error {
	nsMD, err := metadata.ParseNamespaceMetadataStrict(ctx, "", ns, r)
	if err != nil && errors.Is(os.ErrNotExist, err) {
		if err := metadata.CreateNamespaceMetadata(ctx, "", ns, r); err != nil {
			return err
		}
	} else if err != nil {
		return fmt.Errorf("error parsing namespace metadata: %v", err)
	}
	if featureName == "" {
		r.Logf("Your new namespace has been created: %s\n", ns)
		return nil
	}
	ffs, err := r.GetFeatureFiles(ctx, ns)
	if err != nil {
		return fmt.Errorf("failed to get feature files: %v", err)
	}
	for _, ff := range ffs {
		if err := feature.ComplianceCheck(ff, nsMD); err != nil {
			return fmt.Errorf("compliance check for feature %s: %w", ff.Name, err)
		}
		if ff.Name == featureName {
			return fmt.Errorf("feature named %s already exists", featureName)
		}
	}

	featurePath := filepath.Join(ns, fmt.Sprintf("%s.star", featureName))
	template, err := star.GetTemplate(fType)
	if err != nil {
		return errors.Wrap(err, "get template")
	}
	if err := r.WriteFile(featurePath, template, 0600); err != nil {
		return fmt.Errorf("failed to add feature: %v", err)
	}
	r.Logf("Your new feature has been written to %s\n", featurePath)
	r.Logf("Make your changes, and run 'lekko compile'.\n")
	return nil
}

func (r *Repo) RemoveFeature(ctx context.Context, ns, featureName string) error {
	_, err := metadata.ParseNamespaceMetadataStrict(ctx, "", ns, r)
	if err != nil {
		return fmt.Errorf("error parsing namespace metadata: %v", err)
	}
	var removed bool
	for _, file := range []string{
		fmt.Sprintf("%s.star", featureName),
		filepath.Join(metadata.GenFolderPathJSON, fmt.Sprintf("%s.json", featureName)),
		filepath.Join(metadata.GenFolderPathProto, fmt.Sprintf("%s.proto.bin", featureName)),
	} {
		ok, err := r.RemoveIfExists(filepath.Join(ns, file))
		if err != nil {
			return fmt.Errorf("remove if exists failed to remove %s: %v", file, err)
		}
		if ok {
			removed = true
		}
	}
	if removed {
		r.Logf("Feature %s has been removed\n", featureName)
	} else {
		r.Logf("Feature %s does not exist\n", featureName)
	}
	return nil
}

func (r *Repo) RemoveNamespace(ctx context.Context, ns string) error {
	_, err := metadata.ParseNamespaceMetadataStrict(ctx, "", ns, r)
	if err != nil {
		return fmt.Errorf("error parsing namespace metadata: %v", err)
	}
	ok, err := r.RemoveIfExists(ns)
	if err != nil {
		return fmt.Errorf("failed to remove namespace %s: %v", ns, err)
	}
	if !ok {
		r.Logf("Namespace %s does not exist\n", ns)
	}
	if err := metadata.UpdateRootConfigRepoMetadata(ctx, "", r, func(rcrm *metadata.RootConfigRepoMetadata) {
		var updatedNamespaces []string
		for _, n := range rcrm.Namespaces {
			if n != ns {
				updatedNamespaces = append(updatedNamespaces, n)
			}
		}
		rcrm.Namespaces = updatedNamespaces
	}); err != nil {
		return fmt.Errorf("failed to update root config md: %v", err)
	}
	r.Logf("Namespace %s has been removed\n", ns)
	return nil
}

func (r *Repo) Eval(ctx context.Context, ns, featureName string, iCtx map[string]interface{}) (*anypb.Any, error) {
	_, nsMDs, err := r.ParseMetadata(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "parse metadata")
	}
	nsMD, ok := nsMDs[ns]
	if !ok {
		return nil, fmt.Errorf("invalid namespace: %s", ns)
	}

	ff, err := r.GetFeatureFile(ctx, ns, featureName)
	if err != nil {
		return nil, errors.Wrap(err, "get feature file")
	}

	if err := feature.ComplianceCheck(*ff, nsMD); err != nil {
		return nil, errors.Wrap(err, "compliance check")
	}

	evalF, err := encoding.ParseFeature(ctx, "", *ff, nsMD, r)
	if err != nil {
		return nil, err
	}

	return evalF.Evaluate(iCtx)
}
