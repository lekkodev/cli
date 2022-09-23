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

/* Implement fs.ConfigWriter */

// WriteFile writes data to the named file, creating it if necessary.
// If the file does not exist, WriteFile creates it with permissions perm (before umask);
// otherwise WriteFile truncates it before writing, without changing permissions.
func (r *Repo) WriteFile(name string, data []byte, perm os.FileMode) error {
	f, err := r.Fs.OpenFile(name, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, perm)
	if err != nil {
		return err
	}
	_, err = f.Write(data)
	if err1 := f.Close(); err1 != nil && err == nil {
		err = err1
	}
	return err
}

func (r *Repo) MkdirAll(path string, perm os.FileMode) error {
	return r.Fs.MkdirAll(path, perm)
}

func (r *Repo) RemoveIfExists(path string) (bool, error) {
	fi, err := r.Fs.Stat(path)
	if err != nil {
		if r.IsNotExist(err) {
			return false, nil
		}
		return false, errors.Wrap(err, "os.Stat")
	}
	if fi.IsDir() {
		fis, err := r.Fs.ReadDir(fi.Name())
		if err != nil {
			return false, errors.Wrap(err, "read dir")
		}
		for _, fi := range fis {
			if _, err := r.RemoveIfExists(fi.Name()); err != nil {
				return false, fmt.Errorf("remove '%s' if exists: %w", fi.Name(), err)
			}
		}
	}
	if err := r.Fs.Remove(path); err != nil {
		if r.IsNotExist(err) {
			return false, nil
		}
		return false, errors.Wrap(err, "remove")
	}
	return true, nil
}
