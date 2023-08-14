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

package star

import (
	"bytes"
	"context"

	"github.com/bazelbuild/buildtools/build"
	butils "github.com/bazelbuild/buildtools/buildifier/utils"
	"github.com/lekkodev/cli/pkg/feature"
	"github.com/lekkodev/cli/pkg/fs"
	"github.com/lekkodev/cli/pkg/star/static"
	"github.com/pkg/errors"
	"google.golang.org/protobuf/reflect/protoregistry"
)

type Formatter interface {
	// Runs the starlark formatter from bazel buildtools.
	Format(ctx context.Context) (persisted, diffExists bool, err error)
	// Runs the lekko static parser to format starlark the way the UI would.
	StaticFormat(ctx context.Context) (persisted, diffExists bool, err error)
}

type formatter struct {
	filePath, featureName string
	dryRun                bool
	registry              *protoregistry.Types

	cw fs.ConfigWriter
	nv feature.NamespaceVersion
}

func NewStarFormatter(filePath, featureName string, cw fs.ConfigWriter, dryRun bool, registry *protoregistry.Types, nv feature.NamespaceVersion) Formatter {
	return &formatter{
		filePath:    filePath,
		featureName: featureName,
		cw:          cw,
		dryRun:      dryRun,
		registry:    registry,
		nv:          nv,
	}
}

func (f *formatter) Format(ctx context.Context) (persisted, diffExists bool, err error) {
	return f.format(ctx, false)
}

func (f *formatter) StaticFormat(ctx context.Context) (persisted, diffExists bool, err error) {
	return f.format(ctx, true)
}

func (f *formatter) format(ctx context.Context, static bool) (persisted, diffExists bool, err error) {
	data, err := f.cw.GetFileContents(ctx, f.filePath)
	if err != nil {
		return false, false, errors.Wrapf(err, "failed to read file %s", f.filePath)
	}
	var ndata []byte
	if static {
		ndata, err = f.staticFormat(data)
	} else {
		ndata, err = f.bazelFormat(data)
	}
	if err != nil {
		return false, false, err
	}
	diffExists = !bytes.Equal(data, ndata)
	if !diffExists {
		return false, false, nil
	}
	if !f.dryRun {
		if err := f.cw.WriteFile(f.filePath, ndata, 0600); err != nil {
			return false, diffExists, errors.Wrap(err, "failed to write file")
		}
	}
	return !f.dryRun, diffExists, nil
}

func (f *formatter) bazelFormat(data []byte) ([]byte, error) {
	parser := butils.GetParser(static.InputTypeAuto)
	bfile, err := parser(f.filePath, data)
	if err != nil {
		return nil, errors.Wrap(err, "bparse")
	}
	return build.Format(bfile), nil
}

func (f *formatter) staticFormat(data []byte) ([]byte, error) {
	// This statically parses the feature
	// and writes it back to the file without any functional changes.
	// Doing this ensures that the file is formatted exactly the way that
	// the UI would format it, essentially eliminating formatting
	// changes from UI generated PRs.
	// If static formatting passes, we can skip buildifier formatting.
	// If static formatting fails, we can show a warning to the user
	// that the UI will not be able to parse the feature.
	return static.NewWalker(f.filePath, data, f.registry, f.nv).Format()
}
