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
	"os"

	"github.com/pkg/errors"
)

// FileSystem abstraction so we can use the same writing code
// for both local and ephemeral configuration repos.
type ConfigWriter interface {
	WriteFile(name string, data []byte, perm os.FileMode) error
	MkdirAll(path string, perm os.FileMode) error
	// Returns whether or not anything was removed.
	RemoveIfExists(path string) (bool, error)

	Provider
}

type localConfigWriter struct {
	Provider
}

func (*localConfigWriter) WriteFile(name string, data []byte, perm os.FileMode) error {
	return os.WriteFile(name, data, perm)
}

func (*localConfigWriter) MkdirAll(path string, perm os.FileMode) error {
	return os.MkdirAll(path, perm)
}

func (*localConfigWriter) RemoveIfExists(path string) (bool, error) {
	fi, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, errors.Wrap(err, "os.Stat")
	}
	rmFn := os.Remove
	if fi.IsDir() {
		rmFn = os.RemoveAll
	}
	if err := rmFn(path); err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, errors.Wrap(err, "remove")
	}
	return true, nil
}

func LocalConfigWriter() ConfigWriter {
	return &localConfigWriter{
		Provider: LocalProvider(),
	}
}
