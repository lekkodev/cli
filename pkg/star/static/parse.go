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

package static

import (
	"context"
	"path/filepath"
	"strings"

	"github.com/lekkodev/cli/pkg/fs"
	"github.com/lekkodev/cli/pkg/metadata"
	"github.com/lekkodev/cli/pkg/star"
	"github.com/pkg/errors"
)

// Parse reads the star file at the given path and performs
// static parsing, converting the feature to our go-native model.
func Parse(root, filename string, cw fs.ConfigWriter) error {
	ctx := context.Background()
	data, err := cw.GetFileContents(ctx, filename)
	if err != nil {
		return errors.Wrap(err, "read file")
	}
	w := &walker{filename: filename, starBytes: data}
	f, err := w.Build()
	if err != nil {
		return err
	}
	f.Key = strings.Split(filepath.Base(filename), ".")[0]
	rootMD, _, err := metadata.ParseFullConfigRepoMetadataStrict(ctx, root, fs.LocalProvider())
	if err != nil {
		return errors.Wrap(err, "parse root metadata")
	}
	registry, err := star.BuildDynamicTypeRegistry(rootMD.ProtoDirectory, cw)
	if err != nil {
		return errors.Wrap(err, "build dynamic type registry")
	}
	// Print json to stdout
	f.PrintJSON(registry)
	// Rewrite the bytes to the starfile path, based on the parse AST.
	// This is just an illustration, but in the future we could modify
	// the feature and use the following code to write it out.
	bytes, err := w.Mutate(f)
	if err != nil {
		return errors.Wrap(err, "mutate")
	}
	if err := cw.WriteFile(filename, bytes, 0600); err != nil {
		return errors.Wrap(err, "failed to write file")
	}
	return nil
}
