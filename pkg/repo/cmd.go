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
	"os"
	"path/filepath"

	bffv1beta1connect "buf.build/gen/go/lekkodev/cli/bufbuild/connect-go/lekko/bff/v1beta1/bffv1beta1connect"
	bffv1beta1 "buf.build/gen/go/lekkodev/cli/protocolbuffers/go/lekko/bff/v1beta1"
	connect_go "github.com/bufbuild/connect-go"
	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/config"
	"github.com/go-git/go-git/v5/plumbing/transport/http"
	"github.com/lekkodev/cli/pkg/gh"
	"github.com/lekkodev/cli/pkg/secrets"
	"github.com/pkg/errors"
)

// Responsible for all repository management actions on the command line.
// e.g. create/delete/list repos.
type RepoCmd struct {
	lekkoBFFClient bffv1beta1connect.BFFServiceClient
	rs secrets.ReadSecrets
}

func NewRepoCmd(bff bffv1beta1connect.BFFServiceClient, rs secrets.ReadSecrets) *RepoCmd {
	return &RepoCmd{
		lekkoBFFClient: bff,
		rs: rs,
	}
}

type Repository struct {
	Owner       string
	RepoName    string
	Description string
	URL         string
}

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

func (r *RepoCmd) Import(ctx context.Context, owner, repoName, description string) error {
	if len(owner) == 0 {
		return errors.Errorf("Must provide owner")
	}
	if len(repoName) == 0 {
		// try using current dir as a repo name
		wd, err := os.Getwd()
		if err == nil {
			repoName = filepath.Base(wd)
		}
	}
	ghCli := gh.NewGithubClientFromToken(ctx, r.rs.GetGithubToken())
	if r.rs.GetGithubUser() == owner {
		owner = "" // create repo expects an empty owner for personal accounts
	}
	// create empty repo on GitHub
	_, err := ghCli.CreateRepo(ctx, owner, repoName, description, true)
	if err != nil && !errors.Is(err, git.ErrRepositoryAlreadyExists) {
		return err
	}
	gitRepo, err := git.PlainOpen(".")
	if err != nil {
		return err
	}
	// create remote pointing to GitHub (if it not exists)
	_, err = gitRepo.CreateRemote(&config.RemoteConfig{
		Name: "origin",
		URLs: []string{fmt.Sprintf("https://github.com/%s/%s.git", owner, repoName)},
	})
	if err != nil && !errors.Is(err, git.ErrRemoteExists) {
		return err
	}
	// push to GitHub
	err = gitRepo.Push(&git.PushOptions{
		Auth: &http.BasicAuth{
			Username: r.rs.GetGithubUser(),
			Password: r.rs.GetGithubToken(),
		},
	})
	if err != nil && !errors.Is(err, git.NoErrAlreadyUpToDate) {
		return err
	}
	// Pull to get remote branches
	w, err := gitRepo.Worktree()
	if err != nil {
		return err
	}
	err = w.Pull(&git.PullOptions{
		RemoteName: "origin",
		Auth: &http.BasicAuth{
			Username: r.rs.GetGithubUser(),
			Password: r.rs.GetGithubToken(),
		},
	})
	if err != nil && !errors.Is(err, git.NoErrAlreadyUpToDate) {
		return err
	}
	// Create branch config tracking remote
	err = gitRepo.CreateBranch(&config.Branch{
		Name:   "main",
		Remote: "origin",
		Merge:  "refs/heads/main",
	})
	if err != nil && !errors.Is(err, git.ErrBranchExists) {
		return err
	}

	// Import new repo into Lekko
	_, err = r.lekkoBFFClient.ImportRepository(ctx, connect_go.NewRequest(&bffv1beta1.ImportRepositoryRequest{
		RepoKey: &bffv1beta1.RepositoryKey{
			OwnerName: owner,
			RepoName:  repoName,
		},
	}))
	if err != nil {
		return errors.Wrap(err, "import repository")
	}
	return nil
}