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

package fs

import (
	"context"
	"os"
	"path/filepath"
)

// This file will contain both a filename and a filepath. The
// filepath will be relative to the provider, so it can be passed in
// to another call and be a valid file.
type ProviderFile struct {
	Name  string
	Path  string
	IsDir bool
}

// FileSystem abstraction so we can use the same parsing code
// for both local and remote configuration repos.
type Provider interface {
	GetFileContents(ctx context.Context, path string) ([]byte, error)
	GetDirContents(ctx context.Context, path string) ([]ProviderFile, error)
	IsNotExist(err error) bool
}

type localProvider struct{}

func (*localProvider) GetFileContents(_ context.Context, path string) ([]byte, error) {
	return os.ReadFile(path)
}

func (*localProvider) GetDirContents(_ context.Context, path string) ([]ProviderFile, error) {
	fs, err := os.ReadDir(path)
	if err != nil {
		return nil, err
	}
	files := make([]ProviderFile, len(fs))
	for i, f := range fs {
		files[i] = ProviderFile{Name: f.Name(), Path: filepath.Join(path, f.Name()), IsDir: f.IsDir()}
	}
	return files, nil
}

func (*localProvider) IsNotExist(err error) bool {
	return os.IsNotExist(err)
}

func LocalProvider() Provider {
	return &localProvider{}
}
