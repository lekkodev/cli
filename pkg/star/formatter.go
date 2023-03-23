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
)

type Formatter interface {
	Format(ctx context.Context) (persisted, diffExists bool, err error)
}

type formatter struct {
	filePath, featureName string
	fType                 feature.FeatureType
	dryRun                bool

	cw fs.ConfigWriter
}

func NewStarFormatter(filePath, featureName string, fType feature.FeatureType, cw fs.ConfigWriter, dryRun bool) Formatter {
	return &formatter{
		filePath:    filePath,
		featureName: featureName,
		fType:       fType,
		cw:          cw,
		dryRun:      dryRun,
	}
}

func (f *formatter) Format(ctx context.Context) (persisted, diffExists bool, err error) {
	data, err := f.cw.GetFileContents(ctx, f.filePath)
	if err != nil {
		return false, false, errors.Wrapf(err, "failed to read file %s", f.filePath)
	}
	ndata, err := f.format(data)
	if err != nil {
		return false, false, errors.Wrap(err, "static parser format")
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

func (f *formatter) format(data []byte) ([]byte, error) {
	// Static formatter is only supported for primitive types.
	// Protobuf features cannot be statically parsed.
	// Json features can be statically parsed, but static mutation
	// is not roundtrip stable because keys in a json object can be
	// reordered.
	if f.fType.IsPrimitive() {
		// first try static walking
		ndata, err := static.NewWalker(f.filePath, data).Format()
		if err != nil && !errors.Is(err, static.ErrUnsupportedStaticParsing) {
			return nil, errors.Wrap(err, "static parser format")
		} else if err == nil {
			return ndata, nil
		}
	}
	// raw formatting without static parsing
	parser := butils.GetParser(static.InputTypeAuto)
	bfile, err := parser(f.filePath, data)
	if err != nil {
		return nil, errors.Wrap(err, "bparse")
	}
	return build.Format(bfile), nil
}
