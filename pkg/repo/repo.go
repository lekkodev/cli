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
	"strings"
	"time"

	"github.com/go-git/go-billy/v5"
	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/lekkodev/cli/pkg/gh"
	"github.com/lekkodev/cli/pkg/metadata"
	"github.com/pkg/errors"
	giturls "github.com/whilp/git-urls"
)

const (
	mainBranchName = "main"
	remoteName     = "origin"
)

// Abstraction around git and github operations associated with the lekko configuration repo.
// This class can be used either by the cli, or by any other system that intends to manage
// operations around the lekko config repo.
type Repo struct {
	Repo  *git.Repository
	Wt    *git.Worktree
	Fs    billy.Filesystem
	GhCli *gh.GithubClient

	User, Token    string
	LoggingEnabled bool
}

// Creates a new instance of Repo designed to work with filesystem-based repos.
func NewLocal(path string) (*Repo, error) {
	repo, err := git.PlainOpen(path)
	if err != nil {
		return nil, errors.Wrap(err, "failed to open git repo")
	}
	wt, err := repo.Worktree()
	if err != nil {
		return nil, errors.Wrap(err, "failed to get work tree")
	}
	secrets := metadata.NewSecretsOrFail()
	cr := &Repo{
		Repo:           repo,
		Wt:             wt,
		Fs:             wt.Filesystem,
		GhCli:          gh.NewGithubClientFromToken(context.Background(), secrets.GetGithubToken()),
		User:           secrets.GetGithubUser(),
		Token:          secrets.GetGithubToken(),
		LoggingEnabled: true,
	}
	return cr, nil
}

func (r *Repo) CheckGithubAuth(ctx context.Context) error {
	if r.User == "" || r.Token == "" {
		return fmt.Errorf("user unauthenticated")
	}
	if _, err := r.GhCli.GetUserLogin(ctx); err != nil {
		return errors.Wrap(err, "get user login")
	}
	return nil
}

func (r *Repo) WorkingDirectoryHash() (string, error) {
	hash, err := r.Repo.ResolveRevision(plumbing.Revision(plumbing.HEAD))
	if err != nil {
		return "", errors.Wrap(err, "resolve revision")
	}
	var suffix string
	clean, err := r.wdClean()
	if err != nil {
		return "", errors.Wrap(err, "wd clean")
	}
	if !clean {
		suffix = "-dirty"
	}
	return fmt.Sprintf("%s%s", hash.String(), suffix), nil
}

// Start is invoked right before one is about to start working on a config change.
// It checks that we are on a clean master, checks out a local branch and syncs
// it to the remote to ensure any future changes can be saved.
func (r *Repo) Start() (string, error) {
	clean, err := r.wdClean()
	if err != nil {
		return "", errors.Wrap(err, "wdClean")
	}
	if !clean {
		return "", fmt.Errorf("expecting clean working directory")
	}
	if err := r.ensureMainBranch(true); err != nil {
		return "", errors.Wrap(err, "is main")
	}
	branchName, err := r.checkoutLocalBranch()
	if err != nil {
		return "", errors.Wrap(err, "checkout new branch")
	}
	// init the remote branch
	if err := r.pushToRemote(branchName); err != nil {
		return "", errors.Wrap(err, "push to remote")
	}
	// set up tracking in case we need to pull from the
	// remote branch later
	if err := r.setTrackingConfig(branchName); err != nil {
		return "", errors.Wrap(err, "push to remote")
	}
	return branchName, nil
}

func (r *Repo) ensureMainBranch(switchToMain bool) error {
	h, err := r.Repo.Head()
	if err != nil {
		return errors.Wrap(err, "head")
	}
	if !h.Name().IsBranch() {
		return fmt.Errorf("expecting branch, got %s", h.Name().String())
	}
	if h.Name().Short() != mainBranchName {
		if switchToMain {
			if err := r.Wt.Checkout(&git.CheckoutOptions{
				Branch: plumbing.NewBranchReferenceName(mainBranchName),
			}); err != nil {
				return errors.Wrap(err, "checkout main")
			}
			return nil
		}
		return fmt.Errorf("expecting main branch, currently on %s", h.Name().Short())
	}
	return nil
}

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

// Commit will take an optional commit message and push the changes in the
// local working directory to the remote branch.
func (r *Repo) Commit(message string) (string, error) {
	if message == "" {
		message = "new config changes"
	}
	if err := r.Wt.AddGlob("."); err != nil {
		return "", errors.Wrap(err, "add glob")
	}

	hash, err := r.Wt.Commit(message, &git.CommitOptions{
		All: true,
		Author: &object.Signature{
			Name: r.User,
			// TODO: add author email here
			When: time.Now(),
		},
	})
	if err != nil {
		return "", errors.Wrap(err, "commit")
	}
	r.Logf("committed new hash %s locally\n", hash)
	branchName, err := r.BranchName()
	if err != nil {
		return "", errors.Wrap(err, "branch name")
	}
	if err := r.pushToRemote(branchName); err != nil {
		return "", errors.Wrap(err, "push to remote")
	}
	return hash.String(), nil
}

func (r *Repo) BranchName() (string, error) {
	h, err := r.Repo.Head()
	if err != nil {
		return "", errors.Wrap(err, "head")
	}
	return h.Name().Short(), nil
}

func (r *Repo) getOwnerRepo() (string, string, error) {
	rm, err := r.Repo.Remote(remoteName)
	if err != nil {
		return "", "", errors.Wrap(err, "remote")
	}
	if len(rm.Config().URLs) == 0 {
		return "", "", errors.Wrap(err, "remote has no URLs")
	}
	u, err := giturls.Parse(rm.Config().URLs[0])
	if err != nil {
		return "", "", errors.Wrap(err, "url parse")
	}
	parts := strings.SplitN(strings.Trim(u.Path, "/"), "/", 3)
	if len(parts) != 2 {
		return "", "", fmt.Errorf("invalid path: %s", u.Path)
	}
	return parts[0], strings.TrimSuffix(parts[1], ".git"), nil
}

func (r *Repo) Logf(format string, a ...any) {
	if !r.LoggingEnabled {
		return
	}
	fmt.Printf(format, a...)
}
