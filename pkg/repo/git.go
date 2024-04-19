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
	"strings"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/transport"
	"github.com/go-git/go-git/v5/plumbing/transport/http"
	"github.com/lekkodev/cli/pkg/secrets"
	"github.com/pkg/errors"
)

func ResetAndClean(gitRepo *git.Repository) error {
	worktree, err := gitRepo.Worktree()
	if err != nil {
		return errors.Wrap(err, "get worktree")
	}
	head, err := gitRepo.Head()
	if err != nil {
		return errors.Wrap(err, "get head")
	}
	err = worktree.Reset(&git.ResetOptions{
		Commit: head.Hash(),
		Mode:   git.HardReset,
	})
	if err != nil {
		return errors.Wrap(err, "reset")
	}
	err = worktree.Clean(&git.CleanOptions{Dir: true})
	if err != nil {
		return errors.Wrap(err, "clean")
	}

	err = worktree.Checkout(&git.CheckoutOptions{
		Branch: plumbing.NewBranchReferenceName("main"),
	})
	if err != nil {
		return errors.Wrap(err, "checkout main")
	}
	return nil
}

func GitAuthForRemote(r *git.Repository, remoteName string, ap AuthProvider) (transport.AuthMethod, error) {
	remote, err := r.Remote(remoteName)
	if err != nil {
		return nil, err
	}
	return GitAuthForURL(remote.Config().URLs[0], ap), nil
}

func GitAuthForURL(url string, ap AuthProvider) transport.AuthMethod {
	if strings.HasPrefix(url, "https://") {
		return &http.BasicAuth{
			Username: ap.GetUsername(),
			Password: ap.GetToken(),
		}
	}
	return nil
}

// TODO: consolidate with pkg/repo/repo.go
func GitPull(r *git.Repository, rs secrets.ReadSecrets) error {
	w, err := r.Worktree()
	if err != nil {
		return err
	}
	auth, err := GitAuthForRemote(r, "origin", rs)
	if err != nil {
		return err
	}
	err = w.Pull(&git.PullOptions{
		RemoteName: "origin",
		Auth:       auth,
	})
	if err != nil && !errors.Is(err, git.NoErrAlreadyUpToDate) {
		return err
	}
	return nil
}
