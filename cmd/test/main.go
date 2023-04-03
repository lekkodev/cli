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

package main

import (
	"context"
	"log"

	"github.com/lekkodev/cli/pkg/repo"
	"github.com/lekkodev/cli/pkg/secrets"
	"github.com/lekkodev/cli/pkg/star/static"
	"github.com/pkg/errors"
)

func main() {
	wd := "/Users/konrad/devel/lekkodev/config-test"
	rs := secrets.NewSecretsOrFail()
	r, err := repo.NewLocal(wd, rs)
	if err != nil {
		log.Fatal(err)
	}
	ctx := context.Background()
	rootMD, _, err := r.ParseMetadata(ctx)
	if err != nil {
		log.Fatal(errors.Wrap(err, "parse metadata"))
	}
	registry, err := r.BuildDynamicTypeRegistry(ctx, rootMD.ProtoDirectory)
	if err != nil {
		log.Fatal(errors.Wrap(err, "rebuild type registry"))
	}
	ns := "types"
	f := "proto"

	fc, err := r.GetFeatureContents(ctx, ns, f)
	if err != nil {
		log.Fatal(err)
	}

	filename := fc.File.RootPath(fc.File.StarlarkFileName)

	w := static.NewWalker(filename, fc.Star)

	str, err := w.Test(registry)
	if err != nil {
		log.Fatal(err)
	}
	log.Println(str)

	/*	if _, err := r.Compile(ctx, &repo.CompileRequest{
			Registry:                     registry,
			NamespaceFilter:              ns,
			FeatureFilter:                f,
			DryRun:                       dryRun,
			IgnoreBackwardsCompatibility: force,
			// don't verify file structure, since we may have not yet generated
			// the DSLs for newly added features.
			Verify:  false,
			Upgrade: upgrade,
		}); err != nil {
			log.Fatal( errors.Wrap(err, "compile")
		}*/
}
