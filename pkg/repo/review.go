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

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/config"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/transport/http"
	"github.com/google/go-github/v47/github"
	"github.com/pkg/errors"
)

// Review will open a pull request. It takes different actions depending on
// whether or not we are currently on main, and whether or not the working
// directory is clean.
func (r *Repo) Review(ctx context.Context, title string) error {
	if err := r.CheckGithubAuth(ctx); err != nil {
		return errors.Wrap(err, "check github auth")
	}
	var main, clean bool
	var err error
	main, err = r.isMain()
	if err != nil {
		return errors.Wrap(err, "is main")
	}
	clean, err = r.wdClean()
	if err != nil {
		return errors.Wrap(err, "wd clean")
	}
	var branchName string
	if main {
		if clean {
			return errors.Errorf("nothing to review on main with clean wd")
		}
		// dirty working directory on main
		branchName, err = r.checkoutLocalBranch()
		if err != nil {
			return errors.Wrap(err, "checkout new branch")
		}
		if _, err := r.Commit(ctx, ""); err != nil {
			return errors.Wrap(err, "add commit push")
		}
	} else {
		branchName, err = r.BranchName()
		if err != nil {
			return err
		}
		// TODO: check if pr already exists on current branch, and if so, exit early
		if !clean {
			if _, err := r.Commit(ctx, ""); err != nil {
				return errors.Wrap(err, "add commit push")
			}
		}
	}
	if err := r.createPR(ctx, branchName, title); err != nil {
		return errors.Wrap(err, "create pr")
	}
	return nil
}

func (r *Repo) Merge(ctx context.Context, prNum int) error {
	if err := r.CheckGithubAuth(ctx); err != nil {
		return errors.Wrap(err, "check github auth")
	}
	owner, repo, err := r.getOwnerRepo()
	if err != nil {
		return errors.Wrap(err, "get owner repo")
	}
	result, resp, err := r.GhCli.PullRequests.Merge(ctx, owner, repo, prNum, "", &github.PullRequestOptions{
		MergeMethod: "squash",
	})
	if err != nil {
		return fmt.Errorf("ghCli merge pr %v: %w", resp.Status, err)
	}
	if result.GetMerged() {
		r.Logf("PR #%d: %s\n", prNum, result.GetMessage())
	} else {
		return errors.New("Failed to merge pull request.")
	}
	if err := r.Cleanup(ctx); err != nil {
		return errors.Wrap(err, "cleanup")
	}
	return nil
}

func (r *Repo) wdClean() (bool, error) {
	st, err := r.Wt.Status()
	if err != nil {
		return false, errors.Wrap(err, "status")
	}
	return st.IsClean(), nil
}

func (r *Repo) genBranchName() (string, error) {
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

func (r *Repo) checkoutLocalBranch() (string, error) {
	branchName, err := r.genBranchName()
	if err != nil {
		return "", errors.Wrap(err, "gen branch name")
	}
	if err := r.Wt.Checkout(&git.CheckoutOptions{
		Branch: plumbing.NewBranchReferenceName(branchName),
		Create: true,
		Keep:   true,
	}); err != nil {
		return "", errors.Wrap(err, "checkout")
	}
	r.Logf("Checked out local branch %s\n", branchName)
	return branchName, nil
}

func (r *Repo) pushToRemote(ctx context.Context, branchName string) error {
	if err := r.Repo.PushContext(ctx, &git.PushOptions{
		RemoteName: remoteName,
		Auth: &http.BasicAuth{
			Username: r.User,
			Password: r.Token,
		},
	}); err != nil {
		return errors.Wrap(err, "push")
	}
	r.Logf("Pushed local branch %q to remote %q\n", branchName, remoteName)
	return nil
}

func (r *Repo) setTrackingConfig(branchName string) error {
	if err := r.Repo.CreateBranch(&config.Branch{
		Name:   branchName,
		Remote: remoteName,
		Merge:  plumbing.NewBranchReferenceName(branchName),
	}); err != nil {
		return errors.Wrap(err, "create branch tracking configuration")
	}
	r.Logf("Local branch %s has been set up to track changes from %s\n",
		branchName, plumbing.NewRemoteReferenceName(remoteName, branchName))
	return nil
}

func (r *Repo) createPR(ctx context.Context, branchName, title string) error {
	owner, repo, err := r.getOwnerRepo()
	if err != nil {
		return errors.Wrap(err, "get owner repo")
	}
	pr, resp, err := r.GhCli.PullRequests.Create(ctx, owner, repo, &github.NewPullRequest{
		Title: &title,
		Head:  &branchName,
		Base:  strPtr(mainBranchName),
	})
	if err != nil {
		return fmt.Errorf("ghCli create pr status %v: %w", resp.Status, err)
	}
	r.Logf("Created PR:\n\t%s\n", pr.GetHTMLURL())
	return nil
}

func strPtr(s string) *string {
	return &s
}
