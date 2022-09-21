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

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
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
	repo  *git.Repository
	wt    *git.Worktree
	ghCli *gh.GithubClient

	User, token string
}

// Creates a new instance of Repo designed to work with filesystem-based repos.
func NewFS(path string) (*Repo, error) {
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
		repo:  repo,
		wt:    wt,
		ghCli: gh.NewGithubClientFromToken(context.Background(), secrets.GetGithubToken()),
		User:  secrets.GetGithubUser(),
		token: secrets.GetGithubToken(),
	}
	return cr, nil
}

func (cr *Repo) CheckGithubAuth(ctx context.Context) error {
	if cr.User == "" || cr.token == "" {
		return fmt.Errorf("user unauthenticated")
	}
	if _, err := cr.ghCli.GetUserLogin(ctx); err != nil {
		return errors.Wrap(err, "get user login")
	}
	return nil
}

func (cr *Repo) WorkingDirectoryHash() (string, error) {
	hash, err := cr.repo.ResolveRevision(plumbing.Revision(plumbing.HEAD))
	if err != nil {
		return "", errors.Wrap(err, "resolve revision")
	}
	var suffix string
	clean, err := cr.wdClean()
	if err != nil {
		return "", errors.Wrap(err, "wd clean")
	}
	if !clean {
		suffix = "-dirty"
	}
	return fmt.Sprintf("%s%s", hash.String(), suffix), nil
}

func (cr *Repo) isMain() (bool, error) {
	h, err := cr.repo.Head()
	if err != nil {
		return false, errors.Wrap(err, "head")
	}
	return h.Name().IsBranch() && h.Name().Short() == mainBranchName, nil
}

func (cr *Repo) BranchName() (string, error) {
	h, err := cr.repo.Head()
	if err != nil {
		return "", errors.Wrap(err, "head")
	}
	return h.Name().Short(), nil
}

func (cr *Repo) getOwnerRepo() (string, string, error) {
	rm, err := cr.repo.Remote(remoteName)
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
