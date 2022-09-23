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

func (r *Repo) CompileFeature(ctx context.Context, namespace, featureName string) error {
	fc, err := r.GetFeatureContents(ctx, namespace, featureName)
	if err != nil {
		return errors.Wrap(err, "get feature contents")
	}
	registry, err := r.BuildDynamicTypeRegistry(ctx)
	if err != nil {
		return errors.Wrap(err, "build dynamic type registry")
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
