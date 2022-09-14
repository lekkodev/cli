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
	"fmt"
	"io/ioutil"
	"path/filepath"

	"github.com/bazelbuild/buildtools/build"
	butils "github.com/bazelbuild/buildtools/buildifier/utils"
	"github.com/lekkodev/cli/pkg/feature"
	"github.com/lekkodev/cli/pkg/fs"
	"github.com/lekkodev/cli/pkg/metadata"
	"github.com/pkg/errors"
)

const (
	InputTypeAuto string = "auto"
)

type Formatter interface {
	Format() error
}

type formatter struct {
	filePath, featureName string
	verbose               bool
}

func Format(root string, verbose bool) error {
	ctx := context.Background()
	provider := fs.LocalProvider()
	_, nsNameToNsMDs, err := metadata.ParseFullConfigRepoMetadataStrict(ctx, root, provider)
	if err != nil {
		return err
	}
	for ns, nsMD := range nsNameToNsMDs {
		if _, ok := map[string]struct{}{"v1beta2": {}, "v1beta3": {}}[nsMD.Version]; !ok {
			fmt.Printf("Skipping namespace %s since version %s doesn't conform to compilation\n", ns, nsMD.Version)
			continue
		}

		pathToNamespace := filepath.Join(root, ns)
		featureFiles, err := feature.GroupFeatureFiles(
			context.Background(),
			pathToNamespace,
			nsMD,
			fs.LocalProvider(),
			false,
		)
		if err != nil {
			return errors.Wrap(err, "group feature files")
		}
		for _, ff := range featureFiles {
			formatter := NewStarFormatter(
				filepath.Join(root, ns, ff.StarlarkFileName),
				ff.Name, verbose,
			)
			if err := formatter.Format(); err != nil {
				return errors.Wrap(err, "star format")
			}
		}
	}
	return nil
}

func NewStarFormatter(filePath, featureName string, verbose bool) Formatter {
	return &formatter{
		filePath:    filePath,
		featureName: featureName,
		verbose:     verbose,
	}
}

func (f *formatter) Format() error {
	data, err := ioutil.ReadFile(f.filePath)
	if err != nil {
		return errors.Wrapf(err, "failed to read file %s", f.filePath)
	}
	parser := butils.GetParser(InputTypeAuto)
	bfile, err := parser(f.filePath, data)
	if err != nil {
		return errors.Wrap(err, "bparse")
	}
	ndata := build.Format(bfile)

	if bytes.Equal(data, ndata) {
		return nil
	}
	if err := ioutil.WriteFile(f.filePath, ndata, 0600); err != nil {
		return errors.Wrap(err, "failed to write file")
	}
	if f.verbose {
		fmt.Printf("Fixed %s\n", bfile.DisplayPath())
	}
	return nil
}
