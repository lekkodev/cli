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
	"io"
	"os"
	"path/filepath"

	"github.com/lekkodev/cli/pkg/fs"
	"github.com/pkg/errors"
)

/*
	fs.go contains methods to interact with go's filesystem abstraction, billy.Filesystem.
*/

func (r *Repo) Save(path string, bytes []byte) error {
	f, err := r.Fs.TempFile("", path)
	if err != nil {
		return errors.Wrap(err, "temp file")
	}
	defer func() {
		_ = f.Close()
	}()
	if _, err = f.Write(bytes); err != nil {
		return fmt.Errorf("write to temp file '%s': %w", f.Name(), err)
	}
	if err := r.Fs.Rename(f.Name(), path); err != nil {
		return errors.Wrap(err, "fs rename")
	}
	return nil
}

func (r *Repo) Read(path string) ([]byte, error) {
	f, err := r.Fs.Open(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read file at path %s: %w", path, err)
	}
	defer func() {
		_ = f.Close()
	}()
	return io.ReadAll(f)
}

/* Implement fs.Provider */

func (r *Repo) GetFileContents(_ context.Context, path string) ([]byte, error) {
	return r.Read(path)
}

func (r *Repo) GetDirContents(_ context.Context, path string) ([]fs.ProviderFile, error) {
	fi, err := r.Fs.ReadDir(path)
	if err != nil {
		return nil, errors.Wrap(err, "fs read dir")
	}
	var ret []fs.ProviderFile
	for _, info := range fi {
		ret = append(ret, fs.ProviderFile{
			Name:  info.Name(),
			Path:  filepath.Join(path, info.Name()),
			IsDir: info.IsDir(),
		})
	}
	return ret, nil
}

func (r *Repo) IsNotExist(err error) bool {
	// both memfs and osfs return 'os' errors.
	return os.IsNotExist(err)
}
