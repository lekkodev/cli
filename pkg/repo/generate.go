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

	"github.com/lekkodev/cli/pkg/metadata"
	"github.com/lekkodev/cli/pkg/star"
	"github.com/pkg/errors"
	"google.golang.org/protobuf/reflect/protoregistry"
)

func (r *Repo) Compile(ctx context.Context, registry *protoregistry.Types) error {
	var err error
	if registry == nil {
		registry, err = r.BuildDynamicTypeRegistry(ctx)
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
			if err := r.CompileFeature(ctx, registry, nsMD.Name, ff.Name); err != nil {
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
		registry, err = r.BuildDynamicTypeRegistry(ctx)
		if err != nil {
			return errors.Wrap(err, "build dynamic type registry")
		}
	}
	for _, ff := range ffs {
		if err := r.CompileFeature(ctx, registry, namespace, ff.Name); err != nil {
			return errors.Wrap(err, "compile feature")
		}
	}
	return nil
}

func (r *Repo) CompileFeature(ctx context.Context, registry *protoregistry.Types, namespace, featureName string) error {
	fc, err := r.GetFeatureContents(ctx, namespace, featureName)
	if err != nil {
		return errors.Wrap(err, "get feature contents")
	}
	if registry == nil {
		registry, err = r.BuildDynamicTypeRegistry(ctx)
		if err != nil {
			return errors.Wrap(err, "build dynamic type registry")
		}
	}
	compiler := star.NewCompiler(registry, fc.File, r)
	f, err := compiler.Compile()
	if err != nil {
		return errors.Wrap(err, "compile")
	}
	if err := compiler.Persist(f); err != nil {
		return errors.Wrap(err, "persist")
	}
	return nil
}

func (r *Repo) BuildDynamicTypeRegistry(ctx context.Context) (*protoregistry.Types, error) {
	rootMd, _, err := metadata.ParseFullConfigRepoMetadataStrict(ctx, "", r)
	if err != nil {
		return nil, errors.Wrap(err, "parse root md")
	}
	return star.BuildDynamicTypeRegistry(rootMd.ProtoDirectory, r)
}

// Actually regenerates the buf image, and writes it to the file system.
// Note: we don't have a way yet to run this from an ephemeral repo,
// because we need to first ensure that buf cmd line can be executed in the
// ephemeral env.
func (r *Repo) ReBuildDynamicTypeRegistry(ctx context.Context) (*protoregistry.Types, error) {
	rootMd, _, err := metadata.ParseFullConfigRepoMetadataStrict(ctx, "", r)
	if err != nil {
		return nil, errors.Wrap(err, "parse root md")
	}
	return star.ReBuildDynamicTypeRegistry(rootMd.ProtoDirectory, r)
}
