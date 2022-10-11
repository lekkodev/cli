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
	"github.com/lekkodev/cli/pkg/metadata"
	"github.com/pkg/errors"
	giturls "github.com/whilp/git-urls"
)

const (
	mainBranchName = "main"
	remoteName     = "origin"
)

var (
	ErrMissingCredentials = fmt.Errorf("missing credentials")
	ErrNotFound           = fmt.Errorf("not found")
)

// Abstraction around git and github operations associated with the lekko configuration repo.
// This class can be used either by the cli, or by any other system that intends to manage
// operations around the lekko config repo.
type Repo struct {
	Repo *git.Repository
	Wt   *git.Worktree
	Fs   billy.Filesystem

	Auth                       AuthProvider
	loggingEnabled, bufEnabled bool

	fs.Provider
	fs.ConfigWriter
}

// AuthProvider provides access to basic auth credentials. Depending on the
// context (local vs ephemeral), credentials may be provided in different ways,
// thus the interface. Note that email is only used for additional metadata
// on commits, and is not strictly necessary.
type AuthProvider interface {
	GetUsername() string
	GetEmail() string
	GetToken() string
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

	secrets, err := metadata.NewSecretsOrError()
	if err != nil {
		return nil, errors.Wrap(err, "new secrets")
	}
	cr := &Repo{
		Repo:           repo,
		Wt:             wt,
		Fs:             wt.Filesystem,
		Auth:           secrets,
		loggingEnabled: true,
		bufEnabled:     true,
	}
	return cr, nil
}

// Creates a new instance of Repo designed to work with ephemeral repos.
func NewEphemeral(url string, auth AuthProvider) (*Repo, error) {
	r, err := git.Clone(memory.NewStorage(), memfs.New(), &git.CloneOptions{
		URL: url,
		Auth: &http.BasicAuth{
			Username: auth.GetUsername(),
			Password: auth.GetToken(),
		},
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
		Repo: r,
		Wt:   wt,
		Fs:   wt.Filesystem,
		Auth: auth,
	}, nil
}

// Ensures wd is clean, checks out main branch and pulls from remote.
func (r *Repo) Reset() error {
	clean, err := r.wdClean()
	if err != nil {
		return errors.Wrap(err, "wdClean")
	}
	if !clean {
		return fmt.Errorf("expecting clean working directory")
	}
	// checks out main, and pulls from remote
	if err := r.ensureMainBranch(); err != nil {
		return errors.Wrap(err, "is main")
	}
	return nil
}

// Creates a new branch and switches to it, ensuring it doesn't already exist
// on local or remote.
func (r *Repo) Create(branchName string) error {
	if err := r.Reset(); err != nil {
		return errors.Wrap(err, "reset")
	}
	hasRemote, err := r.HasRemote(branchName)
	if err != nil {
		return errors.Wrap(err, "has remote")
	}
	if hasRemote {
		return errors.Errorf("branch '%s' already exists on remote", branchName)
	}
	localRef := plumbing.NewBranchReferenceName(branchName)
	hasLocal, err := r.HasReference(localRef)
	if err != nil {
		return errors.Wrap(err, "has local")
	}
	if hasLocal {
		return errors.Errorf("branch '%s' already exists locally", branchName)
	}
	// we're good, go ahead and create the branch
	if err := r.Wt.Checkout(&git.CheckoutOptions{
		Branch: localRef,
		Create: true, // will fail if the branch name already exists.
	}); err != nil {
		return errors.Wrap(err, "checkout create")
	}
	r.Logf("Checked out local branch %s\n", branchName)
	// set up remote branch tracking
	if err := r.setTrackingConfig(branchName); err != nil {
		return errors.Wrap(err, "push to remote")
	}
	return nil
}

// Restore will switch to the given branchName.
// BranchName must exist either on remote or local.
func (r *Repo) Restore(branchName string) error {
	if err := r.Reset(); err != nil {
		return errors.Wrap(err, "reset")
	}
	localRef, remoteRef := plumbing.NewBranchReferenceName(branchName), plumbing.NewRemoteReferenceName(remoteName, branchName)
	hasRemote, err := r.HasRemote(branchName)
	if err != nil {
		return errors.Wrap(err, "has remote")
	}
	hasLocal, err := r.HasReference(localRef)
	if err != nil {
		return errors.Wrap(err, "has local")
	}
	if !hasRemote && !hasLocal {
		return errors.Errorf("expecting branch '%s' to exist on remote or local", branchName)
	}
	if hasRemote {
		// set a symbolic git ref, so that the local branch we checkout to next
		// goes off of the remote ref
		if err := r.Repo.Storer.SetReference(plumbing.NewSymbolicReference(localRef, remoteRef)); err != nil {
			return errors.Wrap(err, "set ref")
		}
	}
	if err := r.Wt.Checkout(&git.CheckoutOptions{
		Branch: localRef,
	}); err != nil {
		return errors.Wrap(err, "checkout")
	}
	r.Logf("Checked out local branch %s\n", branchName)
	// set up remote branch tracking
	if err := r.setTrackingConfig(branchName); err != nil {
		return errors.Wrap(err, "set tracking config")
	}
	return nil
}

func (r *Repo) HasRemote(branchName string) (bool, error) {
	// Attempt to fetch the remote ref name
	if err := r.Fetch(branchName); err != nil {
		noRef := git.NoMatchingRefSpecError{}
		if noRef.Is(err) {
			return false, nil
		}
		return false, errors.Wrapf(err, "fetch branch %s", branchName)
	}
	return r.HasReference(plumbing.NewRemoteReferenceName(remoteName, branchName))
}

func (r *Repo) HasReference(refName plumbing.ReferenceName) (bool, error) {
	_, err := r.Repo.Storer.Reference(refName)
	if err != nil {
		if errors.Is(err, plumbing.ErrReferenceNotFound) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

func (r *Repo) BasicAuth() transport.AuthMethod {
	return &http.BasicAuth{
		Username: r.Auth.GetUsername(),
		Password: r.Auth.GetToken(),
	}
}

func (r *Repo) CredentialsExist() error {
	if r.Auth.GetUsername() == "" || r.Auth.GetToken() == "" {
		return ErrMissingCredentials
	}
	return nil
}

// Commit will take an optional commit message and push the changes in the
// local working directory to the remote branch.
func (r *Repo) Commit(ctx context.Context, message string) (string, error) {
	if err := r.CredentialsExist(); err != nil {
		return "", err
	}
	main, err := r.isMain()
	if err != nil {
		return "", errors.Wrap(err, "is main")
	}
	if main {
		return "", errors.New("cannot commit while on main branch")
	}
	clean, err := r.wdClean()
	if err != nil {
		return "", errors.Wrap(err, "wd clean")
	}
	if clean {
		return "", errors.New("working directory clean, nothing to commit")
	}

	if message == "" {
		message = "new config changes"
	}
	if err := r.Wt.AddGlob("."); err != nil {
		return "", errors.Wrap(err, "add glob")
	}

	hash, err := r.Wt.Commit(message, &git.CommitOptions{
		All: true,
		Author: &object.Signature{
			Name:  r.Auth.GetUsername(),
			Email: r.Auth.GetEmail(),
			When:  time.Now(),
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
	if err := r.pushToRemote(ctx, branchName); err != nil {
		return "", errors.Wrap(err, "push to remote")
	}
	return hash.String(), nil
}

// Cleans up all resources and references associated with the current working
// branch on local and remote. Will switch the current branch back to main, and
// pull from remote to ensure we are on the latest commit.
func (r *Repo) Cleanup(ctx context.Context, branchName *string) error {
	if err := r.CleanupBranch(ctx, branchName); err != nil {
		return errors.Wrap(err, "cleanup branch")
	}
	if err := r.Pull(); err != nil {
		return errors.Wrap(err, "pull main")
	}
	r.Logf("Pulled from remote. Local branch %s is up to date.\n", mainBranchName)
	return nil
}

func (r *Repo) Pull() error {
	if err := r.Wt.Pull(&git.PullOptions{
		RemoteName: remoteName,
		Auth:       r.BasicAuth(),
	}); err != nil && !errors.Is(err, git.NoErrAlreadyUpToDate) {
		return errors.Wrap(err, "failed to pull")
	}
	return nil
}

func (r *Repo) Fetch(branchName string) error {
	if err := r.Repo.Fetch(&git.FetchOptions{
		RemoteName: remoteName,
		Auth:       r.BasicAuth(),
		RefSpecs:   []config.RefSpec{config.RefSpec(fmt.Sprintf("+%s:%s", plumbing.NewBranchReferenceName(branchName), plumbing.NewRemoteReferenceName(remoteName, branchName)))},
	}); err != nil && !errors.Is(err, git.NoErrAlreadyUpToDate) {
		return err
	}
	return nil
}

func (r *Repo) CleanupBranch(ctx context.Context, branchName *string) error {
	currentBranch, err := r.BranchName()
	if err != nil {
		return errors.Wrap(err, "branch name")
	}
	branchToCleanup := currentBranch
	if branchName != nil {
		branchToCleanup = *branchName
	}
	if branchToCleanup == currentBranch {
		clean, err := r.wdClean()
		if err != nil {
			return errors.Wrap(err, "wd clean")
		}
		if !clean {
			return fmt.Errorf("cannot cleanup branch '%s' with local changes in the working directory", currentBranch)
		}
		if err := r.Wt.Checkout(&git.CheckoutOptions{
			Branch: plumbing.NewBranchReferenceName(mainBranchName),
		}); err != nil {
			return fmt.Errorf("failed to checkout main branch '%s': %w", mainBranchName, err)
		}
		r.Logf("Checked out local branch %s\n", mainBranchName)
	}
	// now, we are on main and need to delete branchToCleanup. First, delete on remote.
	localBranchRef := plumbing.NewBranchReferenceName(branchToCleanup)
	if branchToCleanup == mainBranchName {
		// no need to delete main branch
		return nil
	}
	if err := r.Repo.Push(&git.PushOptions{
		RemoteName: remoteName,
		// Note: the fact that the source ref is empty means this is a delete. This is
		// equivalent to doing `git push origin --delete <branch_name> on the cmd line.
		RefSpecs: []config.RefSpec{config.RefSpec(fmt.Sprintf(":%s", localBranchRef))},
		Auth:     r.BasicAuth(),
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
	if err := r.Repo.DeleteBranch(localBranchRef.Short()); err != nil {
		return fmt.Errorf("delete local branch name %s: %w", localBranchRef.Short(), err)
	}
	if err := r.Repo.Storer.RemoveReference(localBranchRef); err != nil {
		return fmt.Errorf("remove reference %s: %w", localBranchRef, err)
	}
	r.Logf("Successfully deleted local branch %s\n", localBranchRef.Short())
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

func (r *Repo) MainBranchHash() (string, error) {
	hash, err := r.Repo.ResolveRevision(plumbing.Revision(plumbing.NewBranchReferenceName(mainBranchName)))
	if err != nil {
		return "", errors.Wrap(err, "resolve main branch revision")
	}
	return hash.String(), nil
}

func (r *Repo) ensureMainBranch() error {
	h, err := r.Repo.Head()
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
		if err := r.Wt.Checkout(&git.CheckoutOptions{
			Branch: plumbing.NewBranchReferenceName(mainBranchName),
		}); err != nil {
			return errors.Wrap(err, "checkout main")
		}
		return nil
	}
	if err := r.Pull(); err != nil {
		return errors.Wrap(err, "pull main")
	}
	return nil
}

func (r *Repo) isMain() (bool, error) {
	h, err := r.Repo.Head()
	if err != nil {
		return false, errors.Wrap(err, "head")
	}
	if !h.Name().IsBranch() {
		return false, fmt.Errorf("expecting branch, got %s", h.Name().String())
	}
	return h.Name().Short() == mainBranchName, nil
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
	if !r.loggingEnabled {
		return
	}
	fmt.Printf(format, a...)
}
