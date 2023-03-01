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

func (r *repository) Read(path string) ([]byte, error) {
	f, err := r.fs.Open(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read file at path %s: %w", path, err)
	}
	defer func() {
		_ = f.Close()
	}()
	return io.ReadAll(f)
}

/* Implement fs.Provider */

func (r *repository) GetFileContents(_ context.Context, path string) ([]byte, error) {
	return r.Read(path)
}

func (r *repository) GetDirContents(_ context.Context, path string) ([]fs.ProviderFile, error) {
	fi, err := r.fs.ReadDir(path)
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

func (r *repository) IsNotExist(err error) bool {
	// both memfs and osfs return 'os' errors.
	return errors.Is(err, os.ErrNotExist)
}

/* Implement fs.ConfigWriter */

// WriteFile writes data to the named file, creating it if necessary.
// If the file does not exist, WriteFile creates it with permissions perm (before umask);
// otherwise WriteFile truncates it before writing, without changing permissions.
func (r *repository) WriteFile(name string, data []byte, perm os.FileMode) error {
	f, err := r.fs.OpenFile(name, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, perm)
	if err != nil {
		return err
	}
	_, err = f.Write(data)
	if err1 := f.Close(); err1 != nil && err == nil {
		err = err1
	}
	return err
}

func (r *repository) MkdirAll(path string, perm os.FileMode) error {
	return r.fs.MkdirAll(path, perm)
}

func (r *repository) RemoveIfExists(path string) (bool, error) {
	fi, err := r.fs.Stat(path)
	if err != nil {
		if r.IsNotExist(err) {
			return false, nil
		}
		return false, errors.Wrap(err, "fs.Stat")
	}
	if fi.IsDir() {
		fis, err := r.fs.ReadDir(path)
		if err != nil {
			return false, errors.Wrap(err, "read dir")
		}
		for _, subFI := range fis {
			subPath := filepath.Join(path, subFI.Name())
			if _, err := r.RemoveIfExists(subPath); err != nil {
				return false, fmt.Errorf("remove '%s' if exists: %w", subPath, err)
			}
		}
	}
	if err := r.fs.Remove(path); err != nil {
		if r.IsNotExist(err) {
			return false, nil
		}
		return false, errors.Wrap(err, "remove")
	}
	return true, nil
}
