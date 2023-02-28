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
	"os"
	"path/filepath"
	"strings"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/pkg/errors"
)

// Restores the entire repository to the git hash specified. The given
// hash must be a fully formed hash (e.g. 11586ea8bd20290ddc8fceaaab782b4d3dbe15c9).
// We do this by checking out the given hash in a detached head state,
// copying all the files (except the .git directory) to local memory,
// and writing out all the contents to our current working directory.
func (r *Repo) RestoreWorkingDirectory(hash string) error {
	currentBranchName, err := r.BranchName()
	if err != nil {
		return errors.Wrap(err, "get branch name")
	}
	if err := r.wt.Checkout(&git.CheckoutOptions{
		Hash: plumbing.NewHash(hash),
	}); err != nil {
		return errors.Wrap(err, "checkout hash to restore")
	}
	shouldWalk := func(p string) bool {
		for _, suffix := range []string{".git"} {
			if strings.HasSuffix(p, suffix) {
				return false
			}
		}
		return true
	}
	contents, err := r.walkContents("", shouldWalk, false)
	if err != nil {
		return errors.Wrap(err, "get raw contents")
	}

	if err := r.wt.Checkout(&git.CheckoutOptions{
		Branch: plumbing.NewBranchReferenceName(currentBranchName),
		Keep:   false,
		Create: false,
	}); err != nil {
		return errors.Wrapf(err, "failed to checkout %s", currentBranchName)
	}

	if _, err := r.walkContents("", shouldWalk, true); err != nil {
		return errors.Wrap(err, "failed to delete existing contents")
	}

	if err := r.restore(contents); err != nil {
		return errors.Wrap(err, "restore")
	}

	return nil
}

type rawContent struct {
	path  string
	bytes []byte
	mode  os.FileMode
}

func (r *Repo) walkContents(path string, shouldWalk func(string) bool, del bool) ([]rawContent, error) {
	fis, err := r.fs.ReadDir(path)
	if err != nil {
		return nil, errors.Wrap(err, "read dir")
	}
	var ret []rawContent
	for _, fi := range fis {
		fPath := filepath.Join(path, fi.Name())
		if !shouldWalk(fPath) {
			continue
		}

		if !fi.IsDir() {
			bytes, err := r.Read(fPath)
			if err != nil {
				return nil, errors.Wrapf(err, "read file %s", fPath)
			}
			ret = append(ret, rawContent{path: fPath, bytes: bytes, mode: fi.Mode()})
		} else {
			// recursive case
			dirContents, err := r.walkContents(fPath, shouldWalk, del)
			if err != nil {
				return nil, errors.Wrapf(err, "get raw contents %s", fPath)
			}
			ret = append(ret, dirContents...)
		}

		if del {
			if err := r.fs.Remove(fPath); err != nil {
				return nil, errors.Wrapf(err, "remove %s", fPath)
			}
		}
	}
	return ret, nil
}

func (r *Repo) restore(rcs []rawContent) error {
	for _, rc := range rcs {
		dir := filepath.Dir(rc.path)
		if err := r.MkdirAll(dir, rc.mode); err != nil {
			return errors.Wrapf(err, "mkdir all %s", dir)
		}
		if err := r.WriteFile(rc.path, rc.bytes, rc.mode); err != nil {
			return errors.Wrapf(err, "write file %s", rc.path)
		}
	}
	return nil
}
