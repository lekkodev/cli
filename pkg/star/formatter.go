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
	"github.com/lekkodev/cli/pkg/fs"
	"github.com/pkg/errors"
)

const (
	InputTypeAuto string = "auto"
)

type Formatter interface {
	Format() (bool, error)
}

type formatter struct {
	filePath, featureName string

	cw fs.ConfigWriter
}

func NewStarFormatter(filePath, featureName string, cw fs.ConfigWriter) Formatter {
	return &formatter{
		filePath:    filePath,
		featureName: featureName,
		cw:          cw,
	}
}

func (f *formatter) Format() (bool, error) {
	data, err := f.cw.GetFileContents(context.Background(), f.filePath)
	if err != nil {
		return false, errors.Wrapf(err, "failed to read file %s", f.filePath)
	}
	parser := butils.GetParser(InputTypeAuto)
	bfile, err := parser(f.filePath, data)
	if err != nil {
		return false, errors.Wrap(err, "bparse")
	}
	ndata := build.Format(bfile)

	if bytes.Equal(data, ndata) {
		return false, nil
	}
	if err := f.cw.WriteFile(f.filePath, ndata, 0600); err != nil {
		return false, errors.Wrap(err, "failed to write file")
	}
	return true, nil
}
