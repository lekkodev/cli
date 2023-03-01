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
	"net/url"
	"strings"
	"time"

	"github.com/go-git/go-billy/v5"
	"github.com/go-git/go-billy/v5/memfs"
	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/config"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/go-git/go-git/v5/plumbing/transport"
	"github.com/go-git/go-git/v5/plumbing/transport/http"
	"github.com/go-git/go-git/v5/storage/memory"
	"github.com/lekkodev/cli/pkg/fs"
	"github.com/lekkodev/cli/pkg/logging"
	"github.com/pkg/errors"
	giturls "github.com/whilp/git-urls"
)

const (
	MainBranchName = "main"
	RemoteName     = "origin"
)

var (
	ErrMissingCredentials = fmt.Errorf("missing credentials")
	ErrNotFound           = fmt.Errorf("not found")
)

type ConfigurationRepository interface {
	ConfigurationStore
	GitRepository
	fs.Provider
	fs.ConfigWriter
}

type GitRepository interface {
	// Checks out the branch at origin/${branchName}. The remote branch must exist.
	CheckoutRemoteBranch(branchName string) error
	// Returns the url of the remote that the local repository is set up to track.
	GetRemoteURL() (string, error)
	// Commit will take an optional commit message and push the changes in the
	// local working directory to the remote branch.
	Commit(ctx context.Context, ap AuthProvider, message string) (string, error)
	// Cleans up all resources and references associated with the given branch on
	// local and remote, if they exist. If branchName is nil, uses the current
	// (non-master) branch. Will switch the current branch back to main, and
	// pull from remote to ensure we are on the latest commit.
	Cleanup(ctx context.Context, branchName *string, ap AuthProvider) error
	// Pull the latest changes from the branch that HEAD is set up to track.
	Pull(ap AuthProvider) error
	// Returns the hash of the current commit that HEAD is pointing to.
	Hash() (string, error)
	BranchName() (string, error)
	IsClean() (bool, error)
	// Creates new remote branch config at the given sha, and checks out the
	// new branch locally. branchName must be sufficiently unique.
	NewRemoteBranch(branchName string) error
	Read(path string) ([]byte, error)
}

// Abstraction around git and github operations associated with the lekko configuration repo.
// This class can be used either by the cli, or by any other system that intends to manage
// operations around the lekko config repo.
type Repo struct {
	repo *git.Repository
	wt   *git.Worktree
	fs   billy.Filesystem

	loggingEnabled, bufEnabled bool
	path                       string // path to the root of the repository

	fs.Provider
	fs.ConfigWriter
}

// AuthProvider provides access to basic auth credentials. Depending on the
// context (local vs ephemeral), credentials may be provided in different ways,
// thus the interface.
type AuthProvider interface {
	GetUsername() string
	GetToken() string
}

func basicAuth(ap AuthProvider) transport.AuthMethod {
	return &http.BasicAuth{
		Username: ap.GetUsername(),
		Password: ap.GetToken(),
	}
}

func credentialsExist(ap AuthProvider) error {
	if ap.GetUsername() == "" || ap.GetToken() == "" {
		return ErrMissingCredentials
	}
	return nil
}

// Creates a new instance of Repo designed to work with filesystem-based repos.
func NewLocal(path string, auth AuthProvider) (ConfigurationRepository, error) {
	repo, err := git.PlainOpen(path)
	if err != nil {
		return nil, errors.Wrap(err, "failed to open git repo")
	}
	wt, err := repo.Worktree()
	if err != nil {
		return nil, errors.Wrap(err, "failed to get work tree")
	}

	cr := &Repo{
		repo:           repo,
		wt:             wt,
		fs:             wt.Filesystem,
		path:           path,
		loggingEnabled: true,
		bufEnabled:     true,
	}

	return cr, nil
}

// Creates a local clone of a remote github config repository based on the
// given url at the provided path.
func NewLocalClone(path, url string, auth AuthProvider) (ConfigurationRepository, error) {
	repo, err := git.PlainClone(path, false, &git.CloneOptions{
		URL: url,
		Auth: &http.BasicAuth{
			Username: auth.GetUsername(),
			Password: auth.GetToken(),
		},
	})
	if err != nil {
		return nil, errors.Wrapf(err, "plain clone url '%s'", url)
	}
	wt, err := repo.Worktree()
	if err != nil {
		return nil, errors.Wrap(err, "failed to get work tree")
	}
	cr := &Repo{
		repo:           repo,
		wt:             wt,
		fs:             wt.Filesystem,
		path:           path,
		loggingEnabled: true,
		bufEnabled:     true,
	}
	return cr, nil
}

// Creates a new instance of Repo designed to work with ephemeral repos.
func NewEphemeral(url string, auth AuthProvider, branchName string) (ConfigurationRepository, error) {
	r, err := git.Clone(memory.NewStorage(), memfs.New(), &git.CloneOptions{
		URL: url,
		Auth: &http.BasicAuth{
			Username: auth.GetUsername(),
			Password: auth.GetToken(),
		},
		ReferenceName: plumbing.NewBranchReferenceName(branchName),
	})
	if err != nil {
		return nil, errors.Wrap(err, "failed to clone in-mem repo")
	}
	wt, err := r.Worktree()
	if err != nil {
		return nil, errors.Wrap(err, "failed to get work tree")
	}
	if err != nil {
		return nil, errors.Wrap(err, "git clone")
	}
	return &Repo{
		repo: r,
		wt:   wt,
		fs:   wt.Filesystem,
	}, nil
}

// Checks out the remote branch
func (r *Repo) CheckoutRemoteBranch(branchName string) error {
	localRef, remoteRef := plumbing.NewBranchReferenceName(branchName), plumbing.NewRemoteReferenceName(RemoteName, branchName)
	// set a symbolic git ref, so that the local branch we checkout to next
	// goes off of the remote ref
	if err := r.repo.Storer.SetReference(plumbing.NewSymbolicReference(localRef, remoteRef)); err != nil {
		return errors.Wrap(err, "set ref")
	}
	if err := r.wt.Checkout(&git.CheckoutOptions{
		Branch: localRef,
	}); err != nil {
		return errors.Wrap(err, "checkout")
	}
	return nil
}

func (r *Repo) GetRemoteURL() (string, error) {
	cfg, err := r.repo.Config()
	if err != nil {
		return "", errors.Wrap(err, "config")
	}
	remote, ok := cfg.Remotes[RemoteName]
	if !ok {
		return "", errors.Errorf("could not find remote %s in config", RemoteName)
	}
	if len(remote.URLs) == 0 {
		return "", errors.Errorf("remote %s has no urls", RemoteName)
	}
	return remote.URLs[0], nil
}

// Commit will take an optional commit message and push the changes in the
// local working directory to the remote branch.
func (r *Repo) Commit(ctx context.Context, ap AuthProvider, message string) (string, error) {
	if err := credentialsExist(ap); err != nil {
		return "", err
	}
	main, err := r.isMain()
	if err != nil {
		return "", errors.Wrap(err, "is main")
	}
	if main {
		return "", errors.New("cannot commit while on main branch")
	}
	clean, err := r.IsClean()
	if err != nil {
		return "", errors.Wrap(err, "wd clean")
	}
	if clean {
		return "", errors.New("working directory clean, nothing to commit")
	}

	if message == "" {
		message = "new config changes"
	}
	if err := r.wt.AddGlob("."); err != nil {
		return "", errors.Wrap(err, "add glob")
	}

	hash, err := r.wt.Commit(message, &git.CommitOptions{
		All: true,
		Author: &object.Signature{
			Name: ap.GetUsername(),
			When: time.Now(),
		},
	})
	if err != nil {
		return "", errors.Wrap(err, "commit")
	}
	r.Logf("Committed new hash %s locally\n", hash)
	branchName, err := r.BranchName()
	if err != nil {
		return "", errors.Wrap(err, "branch name")
	}
	if err := r.repo.PushContext(ctx, &git.PushOptions{
		RemoteName: RemoteName,
		Auth:       basicAuth(ap),
	}); err != nil {
		return "", errors.Wrap(err, "push")
	}
	r.Logf("Pushed local branch %q to remote %q\n", branchName, RemoteName)
	return hash.String(), nil
}

// Cleans up all resources and references associated with the given branch on
// local and remote, if they exist. If branchName is nil, uses the current
// (non-master) branch. Will switch the current branch back to main, and
// pull from remote to ensure we are on the latest commit.
func (r *Repo) Cleanup(ctx context.Context, branchName *string, ap AuthProvider) error {
	if err := r.cleanupBranch(ctx, branchName, ap); err != nil {
		return errors.Wrap(err, "cleanup branch")
	}
	if err := r.ensureMainBranch(ap); err != nil {
		return errors.Wrap(err, "pull main")
	}
	r.Logf("Pulled from remote. Local branch %s is up to date.\n", MainBranchName)
	return nil
}

func (r *Repo) Pull(ap AuthProvider) error {
	if err := r.wt.Pull(&git.PullOptions{
		RemoteName: RemoteName,
		Auth:       basicAuth(ap),
	}); err != nil && !errors.Is(err, git.NoErrAlreadyUpToDate) {
		return errors.Wrap(err, "failed to pull")
	}
	return nil
}

func (r *Repo) cleanupBranch(ctx context.Context, branchName *string, ap AuthProvider) error {
	currentBranch, err := r.BranchName()
	if err != nil {
		return errors.Wrap(err, "branch name")
	}
	branchToCleanup := currentBranch
	if branchName != nil {
		branchToCleanup = *branchName
	}
	if branchToCleanup == currentBranch {
		clean, err := r.IsClean()
		if err != nil {
			return errors.Wrap(err, "wd clean")
		}
		if !clean {
			return fmt.Errorf("cannot cleanup branch '%s' with local changes in the working directory", currentBranch)
		}
		if err := r.wt.Checkout(&git.CheckoutOptions{
			Branch: plumbing.NewBranchReferenceName(MainBranchName),
		}); err != nil {
			return fmt.Errorf("failed to checkout main branch '%s': %w", MainBranchName, err)
		}
		r.Logf("Checked out local branch %s\n", MainBranchName)
	}
	if branchToCleanup == MainBranchName {
		// no need to delete main branch
		return nil
	}
	// now, we are on main and need to delete branchToCleanup. First, delete on remote.
	localBranchRef := plumbing.NewBranchReferenceName(branchToCleanup)
	if err := r.repo.Push(&git.PushOptions{
		RemoteName: RemoteName,
		// Note: the fact that the source ref is empty means this is a delete. This is
		// equivalent to doing `git push origin --delete <branch_name> on the cmd line.
		RefSpecs: []config.RefSpec{config.RefSpec(fmt.Sprintf(":%s", localBranchRef))},
		Auth:     basicAuth(ap),
	}); err != nil {
		if errors.Is(err, git.NoErrAlreadyUpToDate) {
			r.Logf("Remote branch %s already up to date\n", localBranchRef)
		} else {
			return fmt.Errorf("delete remote branch name %s: %w", localBranchRef, err)
		}
	} else {
		r.Logf("Successfully deleted remote branch %s\n", localBranchRef)
	}
	// Next, delete local branch
	if err := r.repo.DeleteBranch(localBranchRef.Short()); err != nil && !errors.Is(err, git.ErrBranchNotFound) {
		return fmt.Errorf("delete local branch name %s: %w", localBranchRef.Short(), err)
	}
	if err := r.repo.Storer.RemoveReference(localBranchRef); err != nil {
		return fmt.Errorf("remove reference %s: %w", localBranchRef, err)
	}
	r.Logf("Successfully deleted local branch %s\n", localBranchRef.Short())
	return nil
}

// Returns the hash of the current commit that HEAD is pointing to.
func (r *Repo) Hash() (string, error) {
	h, err := r.headHash()
	if err != nil {
		return "", err
	}
	return h.String(), nil
}

func (r *Repo) headHash() (*plumbing.Hash, error) {
	hash, err := r.repo.ResolveRevision(plumbing.Revision(plumbing.HEAD))
	if err != nil {
		return &plumbing.Hash{}, errors.Wrap(err, "resolve revision")
	}
	return hash, nil
}

func (r *Repo) ensureMainBranch(ap AuthProvider) error {
	h, err := r.repo.Head()
	if err != nil {
		return errors.Wrap(err, "head")
	}
	if !h.Name().IsBranch() {
		return fmt.Errorf("expecting branch, got %s", h.Name().String())
	}
	isMain, err := r.isMain()
	if err != nil {
		return err
	}
	if !isMain {
		if err := r.wt.Checkout(&git.CheckoutOptions{
			Branch: plumbing.NewBranchReferenceName(MainBranchName),
		}); err != nil {
			return errors.Wrap(err, "checkout main")
		}
		return nil
	}
	if err := r.Pull(ap); err != nil {
		return errors.Wrap(err, "pull main")
	}
	return nil
}

func (r *Repo) isMain() (bool, error) {
	h, err := r.repo.Head()
	if err != nil {
		return false, errors.Wrap(err, "head")
	}
	if !h.Name().IsBranch() {
		return false, fmt.Errorf("expecting branch, got %s", h.Name().String())
	}
	return h.Name().Short() == MainBranchName, nil
}

func (r *Repo) BranchName() (string, error) {
	h, err := r.repo.Head()
	if err != nil {
		return "", errors.Wrap(err, "head")
	}
	return h.Name().Short(), nil
}

func (r *Repo) getURL() (*url.URL, error) {
	rm, err := r.repo.Remote(RemoteName)
	if err != nil {
		return nil, errors.Wrap(err, "remote")
	}
	if len(rm.Config().URLs) == 0 {
		return nil, errors.Wrap(err, "remote has no URLs")
	}
	u, err := giturls.Parse(rm.Config().URLs[0])
	if err != nil {
		return nil, errors.Wrap(err, "url parse")
	}
	return u, nil
}

func (r *Repo) getOwnerRepo() (string, string, error) {
	u, err := r.getURL()
	if err != nil {
		return "", "", errors.Wrap(err, "get url")
	}
	parts := strings.SplitN(strings.Trim(u.Path, "/"), "/", 3)
	if len(parts) != 2 {
		return "", "", fmt.Errorf("invalid path: %s", u.Path)
	}
	return parts[0], strings.TrimSuffix(parts[1], ".git"), nil
}

// Returns whether or not the working directory is
// clean (i.e. has no uncommitted changes)
func (r *Repo) IsClean() (bool, error) {
	st, err := r.wt.Status()
	if err != nil {
		return false, errors.Wrap(err, "status")
	}
	return st.IsClean(), nil
}

// Creates new remote branch config at the given sha, and checks out the
// new branch locally. branchName must be sufficiently unique.
func (r *Repo) NewRemoteBranch(branchName string) error {
	localRef := plumbing.NewBranchReferenceName(branchName)
	if err := r.wt.Checkout(&git.CheckoutOptions{
		Branch: localRef,
		Create: true,
		Keep:   true,
	}); err != nil {
		return errors.Wrap(err, "checkout")
	}
	r.Logf("Checked out local branch %s\n", logging.Bold(branchName))
	// set tracking config
	if err := r.repo.CreateBranch(&config.Branch{
		Name:   branchName,
		Remote: RemoteName,
		Merge:  localRef,
	}); err != nil && !errors.Is(err, git.ErrBranchExists) {
		return errors.Wrap(err, "create branch tracking configuration")
	}
	r.Logf("Local branch %s has been set up to track changes from %s\n",
		branchName, plumbing.NewRemoteReferenceName(RemoteName, branchName))
	return nil
}
