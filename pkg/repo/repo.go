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
	"os"
	"strings"
	"time"

	"github.com/cenkalti/backoff/v4"
	"github.com/go-git/go-billy/v5"
	"github.com/go-git/go-billy/v5/memfs"
	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/config"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/go-git/go-git/v5/plumbing/transport"
	gitclient "github.com/go-git/go-git/v5/plumbing/transport/client"
	"github.com/go-git/go-git/v5/plumbing/transport/http"
	"github.com/go-git/go-git/v5/storage/memory"
	"github.com/lekkodev/cli/pkg/fs"
	"github.com/lekkodev/cli/pkg/gh"
	"github.com/pkg/errors"
	giturls "github.com/whilp/git-urls"
)

const (
	RemoteName        = "origin"
	backoffMaxElapsed = 7 * time.Second
)

var (
	ErrMissingCredentials = fmt.Errorf("missing credentials")
	ErrNotFound           = fmt.Errorf("not found")
)

// ConfigurationRepository provides read and write access to Lekko configuration
// stored in a git repository.
type ConfigurationRepository interface {
	// Provides CRUD functionality on Lekko configuration stored anywhere
	ConfigurationStore
	// Allows interacting with git
	GitRepository
	// Allows interacting with a git provider, e.g. GitHub
	GitProvider
	// Allows writing logs to configurable destinations
	Logger
	// Underlying filesystem interfaces
	fs.Provider
	fs.ConfigWriter
}

// Provides functionality for interacting with git.
type GitRepository interface {
	// Checks out the branch at origin/${branchName}. The remote branch must exist.
	CheckoutRemoteBranch(branchName string) error
	// Checks out the given sha. Sha must exist on the local
	// git repository (i.e. we shouldn't have to consult the remote
	// repo to check it out).
	CheckoutLocalHash(sha string) error
	// Returns the url of the remote that the local repository is set up to track.
	GetRemoteURL() (string, error)
	// Commit will take an optional commit message and push the changes in the
	// local working directory to the remote branch.
	Commit(ctx context.Context, ap AuthProvider, message string, signature *object.Signature) (string, error)
	// Cleans up all resources and references associated with the given branch on
	// local and remote, if they exist. If branchName is nil, uses the current
	// (non-master) branch. Will switch the current branch back to the default, and
	// pull from remote to ensure we are on the latest commit.
	Cleanup(ctx context.Context, branchName *string, ap AuthProvider) error
	// Pull the latest changes from the given branch name.
	Pull(ctx context.Context, ap AuthProvider, branchName string) error
	// Returns the hash of the current commit that HEAD is pointing to.
	Hash() (string, error)
	BranchName() (string, error)
	IsClean() (bool, error)
	// Creates new remote branch config at the given sha, and checks out the
	// new branch locally. branchName must be sufficiently unique.
	NewRemoteBranch(branchName string) error
	Read(path string) ([]byte, error)
	Status() (git.Status, error)
	HeadCommit() (*object.Commit, error)
	DefaultBranchName() string

	mirror(ctx context.Context, ap AuthProvider, url string) error
}

// Abstraction around git and github operations associated with the lekko configuration repo.
// This class can be used either by the cli, or by any other system that intends to manage
// operations around the lekko config repo.
type repository struct {
	repo *git.Repository
	wt   *git.Worktree
	fs   billy.Filesystem
	// if empty, logging will be disabled.
	log *LoggingConfiguration

	bufEnabled    bool
	path          string // path to the root of the repository
	defaultBranch string // name of the default branch, e.g. 'main', 'trunk'

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

	cr := &repository{
		repo: repo,
		wt:   wt,
		fs:   wt.Filesystem,
		path: path,
		log: &LoggingConfiguration{
			Writer: os.Stdout,
		},
		bufEnabled: true,
	}

	return cr, cr.storeDefaultBranchName(auth)
}

// Creates a local clone of a remote github config repository based on the
// given url at the provided path.
func NewLocalClone(path, url string, auth AuthProvider) (ConfigurationRepository, error) {
	r, err := git.PlainClone(path, false, &git.CloneOptions{
		URL:  url,
		Auth: basicAuth(auth),
		// Note: the default branch selection logic below relies on
		// us cloning from the default branch here.
	})
	if err != nil {
		return nil, errors.Wrapf(err, "plain clone url '%s'", url)
	}
	wt, err := r.Worktree()
	if err != nil {
		return nil, errors.Wrap(err, "failed to get work tree")
	}
	cr := &repository{
		repo: r,
		wt:   wt,
		fs:   wt.Filesystem,
		path: path,
		log: &LoggingConfiguration{
			Writer: os.Stdout,
		},
		bufEnabled: true,
	}
	return cr, cr.storeDefaultBranchName(auth)
}

// Creates a new instance of Repo designed to work with ephemeral repos.
func NewEphemeral(url string, auth AuthProvider, branchName *string) (ConfigurationRepository, error) {
	// clone from default, then check out the requested branch.
	// this allows us to populate the repo's default branch name.
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
	cr := &repository{
		repo: r,
		wt:   wt,
		fs:   wt.Filesystem,
	}
	if err := cr.storeDefaultBranchName(auth); err != nil {
		return nil, errors.Wrapf(err, "store default branch name")
	}
	if branchName != nil {
		return cr, cr.CheckoutRemoteBranch(*branchName)
	}
	return cr, nil
}

func getDefaultBranchName(r *git.Repository) (string, error) {
	envName := os.Getenv("LEKKO_DEFAULT_BRANCH")
	if len(envName) > 0 {
		return envName, nil
	}
	remoteHeadRefName := plumbing.NewRemoteHEADReferenceName(RemoteName)
	ref, err := r.Reference(remoteHeadRefName, true)
	if err != nil {
		return "", errors.Wrapf(err, "remote reference not found '%s'", remoteHeadRefName)
	}
	parts := strings.Split(ref.Name().Short(), "/")
	return parts[len(parts)-1], nil
}

// This method will check the git references for the git repository
// and deduce what the default branch name is. The reference it is looking for
// is the symbolic remote HEAD reference, which you can replicate on a
// filesystem by running `cat .git/refs/remotes/origin/HEAD`.
// If the above reference does not exist, this method will query the
// remote url and determine what that reference points to, updating
// its local filesystem to match. Thus, on subsequent runs, the default
// branch name can be deduced appropriately.
func (r *repository) storeDefaultBranchName(ap AuthProvider) error {
	defaultBranch, err := getDefaultBranchName(r.repo)
	if err == nil {
		r.defaultBranch = defaultBranch
		return nil
	}
	// The local filesystem does not have enough info to deduce the
	// default branch name. Query remote, and save the result
	remote, err := r.repo.Remote(RemoteName)
	if err != nil {
		return errors.Wrap(err, "remote")
	}
	if len(remote.Config().URLs) == 0 {
		return errors.Errorf("no urls found for '%s' remote config", RemoteName)
	}

	e, err := transport.NewEndpoint(remote.Config().URLs[0])
	if err != nil {
		return errors.Wrap(err, "new endpoint")
	}

	cli, err := gitclient.NewClient(e)
	if err != nil {
		return errors.Wrap(err, "new git client")
	}

	s, err := cli.NewUploadPackSession(e, basicAuth(ap))
	if err != nil {
		return errors.Wrap(err, "new upload pack session")
	}

	info, err := s.AdvertisedReferences()
	if err != nil {
		return errors.Wrap(err, "adv refs")
	}

	allrefs, err := info.AllReferences()
	if err != nil {
		return errors.Wrap(err, "all refs")
	}
	headReference, err := allrefs.Reference(plumbing.HEAD)
	if err != nil {
		return errors.Wrapf(err, "no head reference found in remote ref set")
	}
	defaultBranch = headReference.Target().Short()
	// Now that we have the default branch according to remote,
	// store the reference in the local filesystem so that it is
	// available for subsequent runs of the cli.
	refName := plumbing.NewRemoteHEADReferenceName(RemoteName)
	targetName := plumbing.NewRemoteReferenceName(RemoteName, defaultBranch)
	refToSet := plumbing.NewSymbolicReference(refName, targetName)
	if err := r.repo.Storer.SetReference(refToSet); err != nil {
		return errors.Wrap(err, "set reference")
	}
	r.defaultBranch = defaultBranch
	return nil
}

func (r *repository) DefaultBranchName() string { return r.defaultBranch }

// Checks out the remote branch
func (r *repository) CheckoutRemoteBranch(branchName string) error {
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

// Checks out the given sha. Sha must exist on the local
// git repository (i.e. we shouldn't have to consult the remote
// repo to check it out).
func (r *repository) CheckoutLocalHash(sha string) error {
	return r.wt.Checkout(&git.CheckoutOptions{
		Hash: plumbing.NewHash(sha),
	})
}

func (r *repository) GetRemoteURL() (string, error) {
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
// It will try to associate the authorized user's GitHub identity (name, primary email address)
// with the commit and fall back to a username-only commit author if not available.
func (r *repository) Commit(ctx context.Context, ap AuthProvider, message string, signature *object.Signature) (string, error) {
	defaultBranch, err := r.isDefaultBranch()
	if err != nil {
		return "", errors.Wrap(err, "is default branch")
	}
	if defaultBranch {
		return "", errors.New("cannot commit while on default branch")
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
		All:    true,
		Author: signature,
	})
	if err != nil {
		return "", errors.Wrap(err, "commit")
	}
	r.Logf("Committed new hash %s locally\n", hash)
	branchName, err := r.BranchName()
	if err != nil {
		return "", errors.Wrap(err, "branch name")
	}
	if err := r.Push(ctx, ap, branchName, false); err != nil {
		return "", errors.Wrap(err, "push")
	}
	r.Logf("Pushed local branch %q to remote %q\n", branchName, RemoteName)
	return hash.String(), nil
}

func GetCommitSignature(ctx context.Context, ap AuthProvider) (*object.Signature, error) {
	if err := credentialsExist(ap); err != nil {
		return nil, err
	}
	ghCli := gh.NewGithubClientFromToken(ctx, ap.GetToken())
	user, err := ghCli.GetUser(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "could not fetch author identity from GitHub")
	}
	// Try to use name/email associated with GitHub account if available
	var name string
	if user.Name != nil {
		name = *user.Name
	} else {
		name = ap.GetUsername()
	}
	var email string
	if user.Email != nil {
		email = *user.Email
	} else {
		// If user's email address is private, we need to fetch separately
		// but don't fail loudly if we fail to fetch (could be due to app permissions, etc.)
		if emails, err := ghCli.GetUserEmails(ctx); err == nil {
			for _, fetchedEmail := range emails {
				if fetchedEmail.GetPrimary() && fetchedEmail.GetVisibility() == "public" {
					email = fetchedEmail.GetEmail()
					break
				}
			}
		}
		// Use GitHub's noreply email as fallback
		// https://docs.github.com/en/account-and-profile/setting-up-and-managing-your-personal-account-on-github/managing-email-preferences/setting-your-commit-email-address
		if email == "" {
			email = fmt.Sprintf("%d+%s@users.noreply.github.com", *user.ID, *user.Login)
		}
	}

	return &object.Signature{
		Name:  name,
		Email: email,
		When:  time.Now(),
	}, nil
}

// Cleans up all resources and references associated with the given branch on
// local and remote, if they exist. If branchName is nil, uses the current
// (non-master) branch. Will switch the current branch back to the default, and
// pull from remote to ensure we are on the latest commit.
func (r *repository) Cleanup(ctx context.Context, branchName *string, ap AuthProvider) error {
	if err := r.cleanupBranch(ctx, branchName, ap); err != nil {
		return errors.Wrap(err, "cleanup branch")
	}
	if err := r.ensureDefaultBranch(ctx, ap); err != nil {
		return errors.Wrap(err, "ensure default branch")
	}
	r.Logf("Pulled from remote. Local branch %s is up to date.\n", r.DefaultBranchName())
	return nil
}

func (r *repository) Pull(ctx context.Context, ap AuthProvider, branchName string) error {
	operation := func() error {
		if err := r.wt.PullContext(ctx, &git.PullOptions{
			RemoteName:    RemoteName,
			Auth:          basicAuth(ap),
			ReferenceName: plumbing.NewBranchReferenceName(branchName),
		}); err != nil && !errors.Is(err, git.NoErrAlreadyUpToDate) {
			return errors.Wrap(err, "failed to pull")
		}
		return nil
	}
	b := backoff.NewExponentialBackOff()
	b.MaxElapsedTime = backoffMaxElapsed
	return backoff.Retry(operation, backoff.WithContext(b, ctx))
}

func (r *repository) Push(ctx context.Context, ap AuthProvider, branchName string, deleteOnRemote bool) error {
	ref := plumbing.NewBranchReferenceName(branchName)
	var refspec config.RefSpec
	if deleteOnRemote {
		// Note: the fact that the source ref is empty means this is a delete. This is
		// equivalent to doing `git push origin --delete <branch_name> on the cmd line.
		refspec = config.RefSpec(fmt.Sprintf(":%s", ref))
	} else {
		refspec = config.RefSpec(fmt.Sprintf("%s:%s", ref, ref))
	}
	operation := func() error {
		if err := r.repo.PushContext(ctx, &git.PushOptions{
			RemoteName: RemoteName,
			// We push only the branch provided. To understand how refspecs
			// are constructed, see https://git-scm.com/book/en/v2/Git-Internals-The-Refspec
			// and https://stackoverflow.com/a/48430450.
			RefSpecs: []config.RefSpec{refspec},
			Auth:     basicAuth(ap),
		}); err != nil && !errors.Is(err, git.NoErrAlreadyUpToDate) {
			return errors.Wrap(err, "failed to push")
		}
		return nil
	}
	b := backoff.NewExponentialBackOff()
	b.MaxElapsedTime = backoffMaxElapsed
	return backoff.Retry(operation, backoff.WithContext(b, ctx))
}

func (r *repository) cleanupBranch(ctx context.Context, branchName *string, ap AuthProvider) error {
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
			Branch: plumbing.NewBranchReferenceName(r.DefaultBranchName()),
		}); err != nil {
			return fmt.Errorf("failed to checkout default branch '%s': %w", r.DefaultBranchName(), err)
		}
		r.Logf("Checked out local branch %s\n", r.DefaultBranchName())
	}
	if branchToCleanup == r.DefaultBranchName() {
		// no need to delete default branch
		return nil
	}
	// now, we are on default and need to delete branchToCleanup. First, delete on remote.
	localBranchRef := plumbing.NewBranchReferenceName(branchToCleanup)
	if err := r.Push(ctx, ap, branchToCleanup, true); err != nil {
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
func (r *repository) Hash() (string, error) {
	h, err := r.headHash()
	if err != nil {
		return "", err
	}
	return h.String(), nil
}

func (r *repository) headHash() (*plumbing.Hash, error) {
	ref, err := r.repo.Head()
	if err != nil {
		return nil, err
	}
	hash := ref.Hash()
	return &hash, nil
}

func (r *repository) ensureDefaultBranch(ctx context.Context, ap AuthProvider) error {
	h, err := r.repo.Head()
	if err != nil {
		return errors.Wrap(err, "head")
	}
	if !h.Name().IsBranch() {
		return fmt.Errorf("expecting branch, got %s", h.Name().String())
	}
	isMain, err := r.isDefaultBranch()
	if err != nil {
		return err
	}
	if !isMain {
		if err := r.wt.Checkout(&git.CheckoutOptions{
			Branch: plumbing.NewBranchReferenceName(r.DefaultBranchName()),
		}); err != nil {
			return errors.Wrap(err, "checkout default")
		}
		return nil
	}
	if err := r.Pull(ctx, ap, r.DefaultBranchName()); err != nil {
		return errors.Wrap(err, "pull default")
	}
	return nil
}

func (r *repository) isDefaultBranch() (bool, error) {
	h, err := r.repo.Head()
	if err != nil {
		return false, errors.Wrap(err, "head")
	}
	if !h.Name().IsBranch() {
		return false, fmt.Errorf("expecting branch, got %s", h.Name().String())
	}
	return h.Name().Short() == r.DefaultBranchName(), nil
}

func (r *repository) BranchName() (string, error) {
	h, err := r.repo.Head()
	if err != nil {
		return "", errors.Wrap(err, "head")
	}
	return h.Name().Short(), nil
}

func (r *repository) getURL() (*url.URL, error) {
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

func (r *repository) getOwnerRepo() (string, string, error) {
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
func (r *repository) IsClean() (bool, error) {
	st, err := r.wt.Status()
	if err != nil {
		return false, errors.Wrap(err, "status")
	}
	return st.IsClean(), nil
}

func (r *repository) Status() (git.Status, error) {
	return r.wt.Status()
}

func (r *repository) HeadCommit() (*object.Commit, error) {
	hash, err := r.headHash()
	if err != nil {
		return nil, errors.Wrap(err, "resolve head")
	}
	return r.repo.CommitObject(*hash)
}

// Creates new remote branch config at the given sha, and checks out the
// new branch locally. branchName must be sufficiently unique.
func (r *repository) NewRemoteBranch(branchName string) error {
	localRef := plumbing.NewBranchReferenceName(branchName)
	if err := r.wt.Checkout(&git.CheckoutOptions{
		Branch: localRef,
		Create: true,
		Keep:   true,
	}); err != nil {
		return errors.Wrap(err, "checkout")
	}
	r.Logf("Checked out local branch %s\n", r.Bold(branchName))
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

func (r *repository) mirror(ctx context.Context, ap AuthProvider, url string) error {
	ref := plumbing.NewBranchReferenceName(r.DefaultBranchName())
	remote, err := r.repo.CreateRemote(&config.RemoteConfig{
		Name: "mirror",
		URLs: []string{url},
	})
	if err != nil {
		return errors.Wrap(err, "failed to create new mirrored remote")
	}
	if err := r.repo.PushContext(ctx, &git.PushOptions{
		RemoteName: remote.Config().Name,
		RefSpecs:   []config.RefSpec{config.RefSpec(fmt.Sprintf("%s:%s", ref, ref))},
		Auth:       basicAuth(ap),
	}); err != nil && !errors.Is(err, git.NoErrAlreadyUpToDate) {
		return errors.Wrap(err, "failed to push")
	}
	return nil
}

// Mirrors the template config repo at the provided url. Note: a repository must already
// exist at the given url.
func MirrorAtURL(ctx context.Context, ap AuthProvider, url string) error {
	templateURL := "https://github.com/lekkodev/template.git"

	templateRepo, err := NewEphemeral(templateURL, ap, nil)
	if err != nil {
		return errors.Wrap(err, "failed to create ephemeral template repo")
	}

	return templateRepo.mirror(ctx, ap, url)
}
