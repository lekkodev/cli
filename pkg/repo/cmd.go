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
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	bffv1beta1connect "buf.build/gen/go/lekkodev/cli/bufbuild/connect-go/lekko/bff/v1beta1/bffv1beta1connect"
	bffv1beta1 "buf.build/gen/go/lekkodev/cli/protocolbuffers/go/lekko/bff/v1beta1"
	"github.com/AlecAivazis/survey/v2"
	connect_go "github.com/bufbuild/connect-go"
	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/config"
	"github.com/lekkodev/cli/pkg/dotlekko"
	"github.com/lekkodev/cli/pkg/gh"
	"github.com/lekkodev/cli/pkg/gitcli"
	"github.com/lekkodev/cli/pkg/logging"
	"github.com/lekkodev/cli/pkg/secrets"
	"github.com/pkg/errors"
)

// Responsible for all repository management actions on the command line.
// e.g. create/delete/list repos.
type RepoCmd struct {
	lekkoBFFClient bffv1beta1connect.BFFServiceClient
	rs             secrets.ReadSecrets
}

func NewRepoCmd(bff bffv1beta1connect.BFFServiceClient, rs secrets.ReadSecrets) *RepoCmd {
	return &RepoCmd{
		lekkoBFFClient: bff,
		rs:             rs,
	}
}

type Repository struct {
	Owner       string
	RepoName    string
	Description string
	URL         string
}

var ErrRemoteHasChanges = errors.New("Remote repository has new changes, " +
	fmt.Sprintf("please run %s to merge them locally and try again.", logging.Bold("lekko pull")))

func (r *RepoCmd) List(ctx context.Context) ([]*Repository, error) {
	resp, err := r.lekkoBFFClient.ListRepositories(ctx, connect_go.NewRequest(&bffv1beta1.ListRepositoriesRequest{}))
	if err != nil {
		return nil, errors.Wrap(err, "list repos")
	}
	var ret []*Repository
	for _, r := range resp.Msg.GetRepositories() {
		ret = append(ret, repoFromProto(r))
	}
	return ret, nil
}

func repoFromProto(repo *bffv1beta1.Repository) *Repository {
	if repo == nil {
		return nil
	}
	return &Repository{
		Owner:       repo.OwnerName,
		RepoName:    repo.RepoName,
		Description: repo.Description,
		URL:         repo.Url,
	}
}

func (r *RepoCmd) Create(ctx context.Context, owner, repo, description string) (string, error) {
	resp, err := r.lekkoBFFClient.CreateRepository(ctx, connect_go.NewRequest(&bffv1beta1.CreateRepositoryRequest{
		RepoKey: &bffv1beta1.RepositoryKey{
			OwnerName: owner,
			RepoName:  repo,
		},
		Description: description,
	}))
	if err != nil {
		return "", errors.Wrap(err, "create repository")
	}
	return resp.Msg.GetUrl(), nil
}

func (r *RepoCmd) Delete(ctx context.Context, owner, repo string, deleteOnRemote bool) error {
	_, err := r.lekkoBFFClient.DeleteRepository(ctx, connect_go.NewRequest(&bffv1beta1.DeleteRepositoryRequest{
		RepoKey: &bffv1beta1.RepositoryKey{
			OwnerName: owner,
			RepoName:  repo,
		},
		DeleteOnRemote: deleteOnRemote,
	}))
	if err != nil {
		return errors.Wrap(err, "delete repository")
	}
	return nil
}

func DefaultRepoBasePath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, "Library/Application Support/Lekko/Config Repositories"), nil
}

func PrepareGithubRepo() (string, error) {
	base, err := DefaultRepoBasePath()
	if err != nil {
		return "", err
	}
	dot, err := dotlekko.ReadDotLekko()
	if err != nil || len(dot.Repository) == 0 {
		return InitIfNotExists("")
	}
	repoOwner, repoName := dot.GetRepoInfo()
	repoPath := filepath.Join(base, repoOwner, repoName)

	shouldClone := false
	fi, err := os.Stat(repoPath)
	if errors.Is(err, fs.ErrNotExist) {
		shouldClone = true
	}
	if err == nil && fi.IsDir() {
		entries, err := os.ReadDir(repoPath)
		if err != nil {
			return "", err
		}
		if len(entries) == 0 {
			shouldClone = true
		}
	}

	githubRepoURL := fmt.Sprintf("https://github.com/%s/%s.git", repoOwner, repoName)

	if shouldClone {
		err := gitcli.Clone(githubRepoURL, repoPath)
		if err != nil {
			return "", err
		}
	} else {
		gitRepo, err := git.PlainOpen(repoPath)
		if err == nil {
			remote, err := gitRepo.Remote("origin")
			if err == nil {
				// TODO: support comparing ssh and https urls
				if len(remote.Config().URLs) == 0 || remote.Config().URLs[0] != githubRepoURL {
					return "", errors.Errorf("repo already exists at %s but with different origin: %s", repoPath, remote.Config().URLs[0])
				}
				// TODO: checkout main and pull?
			}
		}
		if err != nil {
			return "", errors.Wrapf(err, "invalid git repo at %s", repoPath)
		}
	}
	return repoPath, nil
}

func InitIfNotExists(repoPath string) (string, error) {
	if len(repoPath) == 0 {
		base, err := DefaultRepoBasePath()
		if err != nil {
			return "", err
		}
		repoPath = filepath.Join(base, "default")
	}
	err := os.MkdirAll(repoPath, 0777)
	if err != nil {
		return "", err
	}

	entries, err := os.ReadDir(repoPath)
	if err != nil {
		return "", err
	}
	if len(entries) > 0 {
		return repoPath, nil // assume that everything is fine
	}

	gitRepo, err := git.PlainClone(repoPath, false, &git.CloneOptions{
		URL: "https://github.com/lekkodev/template.git",
	})
	if err != nil {
		return "", err
	}
	err = gitRepo.DeleteRemote("origin")
	if err != nil {
		return "", err
	}
	return repoPath, nil
}

func (r *RepoCmd) Import(ctx context.Context, repoPath, owner, repoName, description string) error {
	repoPath, err := InitIfNotExists(repoPath)
	if err != nil {
		return errors.Wrap(err, "init repo")
	}

	gitRepo, err := git.PlainOpen(repoPath)
	if err != nil {
		return errors.Wrap(err, "open git repo")
	}

	var createGitHubRepo bool
	if err := survey.AskOne(&survey.Confirm{Message: fmt.Sprintf("Create a new repository %s/%s on GitHub?", owner, repoName)}, &createGitHubRepo); err != nil {
		return errors.Wrap(err, "prompt")
	}
	if !createGitHubRepo {
		return errors.New("Cancelled")
	}

	list, err := gitRepo.Remotes()
	if err != nil {
		return errors.Wrap(err, "get remotes")
	}
	if len(list) > 0 {
		return errors.New("Remote already exists, import manually")
	}
	worktree, err := gitRepo.Worktree()
	if err != nil {
		return errors.Wrap(err, "get worktree")
	}
	status, err := worktree.Status()
	if err != nil {
		return errors.Wrap(err, "get status")
	}
	if !status.IsClean() {
		_, err = worktree.Add(".")
		if err != nil {
			return errors.Wrap(err, "add files")
		}
		_, err = worktree.Commit("Configs commit", &git.CommitOptions{
			All: true,
		})
		if err != nil {
			return errors.Wrap(err, "commit files")
		}
	}

	ghCli := gh.NewGithubClientFromToken(ctx, r.rs.GetGithubToken())
	githubOwner := owner
	if r.rs.GetGithubUser() == owner {
		githubOwner = "" // GitHub expects an empty owner for personal accounts
	}
	// try using dir name as a repo name
	if len(repoName) == 0 {
		repoName = filepath.Base(repoPath)
	}
	// create empty repo on GitHub
	ghRepo, err := ghCli.CreateRepo(ctx, githubOwner, repoName, description, true)
	if err != nil && !errors.Is(err, git.ErrRepositoryAlreadyExists) {
		return errors.Wrap(err, "create repo on GitHub")
	}

	// create remote pointing to GitHub (if it not exists)
	_, err = gitRepo.CreateRemote(&config.RemoteConfig{
		Name: "origin",
		URLs: []string{ghRepo.GetCloneURL()},
	})
	if err != nil && !errors.Is(err, git.ErrRemoteExists) {
		return errors.Wrap(err, "create remote")
	}

	// push to GitHub
	auth, err := GitAuthForRemote(gitRepo, "origin", r.rs)
	if err != nil {
		return err
	}
	err = gitRepo.Push(&git.PushOptions{
		Auth: auth,
	})
	if err != nil && !errors.Is(err, git.NoErrAlreadyUpToDate) {
		return errors.Wrap(err, "push to GitHub")
	}

	// Pull to get remote branches
	err = GitPull(gitRepo, r.rs)
	if err != nil {
		return errors.Wrap(err, "pull from GitHub")
	}

	// Create branch config tracking remote
	err = gitRepo.CreateBranch(&config.Branch{
		Name:   "main",
		Remote: "origin",
		Merge:  "refs/heads/main",
	})
	if err != nil && !errors.Is(err, git.ErrBranchExists) {
		return errors.Wrap(err, "create branch")
	}

	// Import new repo into Lekko
	_, err = r.lekkoBFFClient.ImportRepository(ctx, connect_go.NewRequest(&bffv1beta1.ImportRepositoryRequest{
		RepoKey: &bffv1beta1.RepositoryKey{
			OwnerName: owner,
			RepoName:  repoName,
		},
	}))
	if err != nil {
		return errors.Wrap(err, "import repository into Lekko")
	}
	return nil
}

func ListNativeConfigFiles(lekkoPath string, ext string) ([]string, error) {
	var files []string
	err := filepath.WalkDir(lekkoPath, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() && (d.Name() == "gen" || d.Name() == "proto") {
			return fs.SkipDir
		}
		if !d.IsDir() && strings.HasSuffix(d.Name(), ext) && !strings.HasSuffix(d.Name(), "_gen"+ext) {
			files = append(files, path)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return files, nil
}
