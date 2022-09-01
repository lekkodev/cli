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

package gh

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/config"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/pkg/errors"
)

const (
	mainBranchName = "main"
)

// Abstraction around git and github operations associated with the lekko configuration repo.
type ConfigRepo struct {
	repo *git.Repository
	wt   *git.Worktree
}

func New(path string) (*ConfigRepo, error) {
	fmt.Printf("%s\n", path)
	repo, err := git.PlainOpen(path)
	if err != nil {
		return nil, errors.Wrap(err, "failed to open git repo")
	}
	wt, err := repo.Worktree()
	if err != nil {
		return nil, errors.Wrap(err, "failed to get work tree")
	}
	return &ConfigRepo{
		repo: repo,
		wt:   wt,
	}, nil
}

func (cr *ConfigRepo) Review() error {
	isMain, err := cr.isMain()
	if err != nil {
		return errors.Wrap(err, "is main")
	}
	if !isMain {
		return errors.New("can only start reviews off the main branch")
	}

	clean, err := cr.wdClean()
	if err != nil {
		return errors.Wrap(err, "wd clean")
	}
	if clean {
		return errors.New("current working directory is clean, nothing to review")
	}

	branchName, err := cr.checkoutBranch()
	if err != nil {
		return errors.Wrap(err, "checkout new branch")
	}

	if err := cr.wt.AddGlob("."); err != nil {
		return errors.Wrap(err, "add glob")
	}
	log.Printf("checked out new branch %s\n", branchName)

	hash, err := cr.wt.Commit("new config changes", &git.CommitOptions{
		All: true,
	})
	if err != nil {
		return errors.Wrap(err, "commit")
	}
	log.Printf("committed new hash %s\n", hash)
	if err := cr.createPR(branchName); err != nil {
		return errors.Wrap(err, "create pr")
	}
	return nil
}

func (cr *ConfigRepo) Merge() error {
	cmd := exec.Command("gh", "pr", "merge", "-sd")
	cmd.Stderr = os.Stderr
	cmd.Stdout = os.Stdout
	if err := cmd.Run(); err != nil {
		return errors.Wrap(err, "gh pr merge")
	}
	return nil
}

func (cr *ConfigRepo) isMain() (bool, error) {
	h, err := cr.repo.Head()
	if err != nil {
		return false, errors.Wrap(err, "head")
	}
	return h.Name().IsBranch() && h.Name().Short() == mainBranchName, nil
}

func (cr *ConfigRepo) wdClean() (bool, error) {
	st, err := cr.wt.Status()
	if err != nil {
		return false, errors.Wrap(err, "status")
	}
	return st.IsClean(), nil
}

func (cr *ConfigRepo) genBranchName() (string, error) {
	cfg, err := config.LoadConfig(config.GlobalScope)
	if err != nil {
		return "", errors.Wrap(err, "load global config")
	}
	user := strings.ToLower(strings.Split(cfg.User.Name, " ")[0])
	if user == "" {
		user = "anon"
	}
	return fmt.Sprintf("%s-review-%d", user, time.Now().Nanosecond()), nil
}

func (cr *ConfigRepo) checkoutBranch() (string, error) {
	branchName, err := cr.genBranchName()
	if err != nil {
		return "", errors.Wrap(err, "gen branch name")
	}
	if err := cr.wt.Checkout(&git.CheckoutOptions{
		Branch: plumbing.NewBranchReferenceName(branchName),
		Create: true,
		Keep:   true,
	}); err != nil {
		return "", errors.Wrap(err, "checkout")
	}
	return branchName, nil
}

func (cr *ConfigRepo) createPR(branchName string) error {
	cmd := exec.Command("git", "push", "--set-upstream", "origin", branchName)
	cmd.Stderr = os.Stderr
	cmd.Stdout = os.Stdout
	if err := cmd.Run(); err != nil {
		return errors.Wrap(err, "git push set upstream")
	}
	cmd = exec.Command("gh", "pr", "create", "-B", mainBranchName, "-H", branchName, "-f")
	cmd.Stderr = os.Stderr
	cmd.Stdout = os.Stdout
	if err := cmd.Run(); err != nil {
		return errors.Wrap(err, "gh pr create")
	}
	return nil
}
