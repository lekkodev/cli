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
	"github.com/google/go-github/v47/github"
	"github.com/lekkodev/cli/pkg/gh"
	"github.com/pkg/errors"
)

// Review will open a pull request. It takes different actions depending on
// whether or not we are currently on main, and whether or not the working
// directory is clean.
func (r *Repo) Review(ctx context.Context, title string, ghCli *gh.GithubClient) (string, error) {
	if err := r.CredentialsExist(); err != nil {
		return "", err
	}
	var main, clean bool
	var err error
	main, err = r.isMain()
	if err != nil {
		return "", errors.Wrap(err, "is main")
	}
	clean, err = r.wdClean()
	if err != nil {
		return "", errors.Wrap(err, "wd clean")
	}
	var branchName string
	if main {
		if clean {
			return "", errors.Errorf("nothing to review on main with clean wd")
		}
		// dirty working directory on main
		branchName, err = r.checkoutLocalBranch()
		if err != nil {
			return "", errors.Wrap(err, "checkout new branch")
		}
		// set up remote branch tracking
		if err := r.setTrackingConfig(branchName); err != nil {
			return "", errors.Wrap(err, "push to remote")
		}
		if _, err := r.Commit(ctx, ""); err != nil {
			return "", errors.Wrap(err, "main add commit push")
		}
	} else {
		branchName, err = r.BranchName()
		if err != nil {
			return "", err
		}
		if !clean {
			if _, err := r.Commit(ctx, ""); err != nil {
				return "", errors.Wrap(err, "branch add commit push")
			}
		}
	}
	url, err := r.createPR(ctx, branchName, title, ghCli)
	if err != nil {
		return "", errors.Wrap(err, "create pr")
	}
	return url, nil
}

func (r *Repo) Merge(ctx context.Context, prNum *int, ghCli *gh.GithubClient) error {
	if err := r.CredentialsExist(); err != nil {
		return err
	}
	owner, repo, err := r.getOwnerRepo()
	if err != nil {
		return errors.Wrap(err, "get owner repo")
	}
	branchName, err := r.BranchName()
	if err != nil {
		return errors.Wrap(err, "branch name")
	}
	if prNum == nil {
		pr, err := r.getPRForBranch(ctx, owner, repo, branchName, ghCli)
		if err != nil {
			return errors.Wrap(err, "get pr for branch")
		}
		prNum = pr.Number
	}
	r.Logf("Merging PR #%d...\n", *prNum)
	result, resp, err := ghCli.PullRequests.Merge(ctx, owner, repo, *prNum, "", &github.PullRequestOptions{
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
	if err := r.Cleanup(ctx, &branchName); err != nil {
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
	user := r.Auth.GetUsername()
	if user == "" {
		cfg, err := config.LoadConfig(config.GlobalScope)
		if err != nil {
			return "", errors.Wrap(err, "load global config")
		}
		user = strings.ToLower(strings.Split(cfg.User.Name, " ")[0])
	}
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
		Auth:       r.BasicAuth(),
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

func (r *Repo) createPR(ctx context.Context, branchName, title string, ghCli *gh.GithubClient) (string, error) {
	owner, repo, err := r.getOwnerRepo()
	if err != nil {
		return "", errors.Wrap(err, "get owner repo")
	}
	pr, err := r.getPRForBranch(ctx, owner, repo, branchName, ghCli)
	if err == nil {
		// pr already exists
		return pr.GetHTMLURL(), nil
	}
	if title == "" {
		title = "new PR"
	}
	pr, resp, err := ghCli.PullRequests.Create(ctx, owner, repo, &github.NewPullRequest{
		Title: &title,
		Head:  &branchName,
		Base:  strPtr(mainBranchName),
	})
	if err != nil {
		return "", fmt.Errorf("ghCli create pr status %v: %w", resp.Status, err)
	}
	r.Logf("Created PR:\n\t%s\n", pr.GetHTMLURL())
	return pr.GetHTMLURL(), nil
}

func strPtr(s string) *string {
	return &s
}

func (r *Repo) getPRForBranch(ctx context.Context, owner, repo, branchName string, ghCli *gh.GithubClient) (*github.PullRequest, error) {
	prs, resp, err := ghCli.PullRequests.List(ctx, owner, repo, &github.PullRequestListOptions{
		Head: fmt.Sprintf("%s:%s", owner, branchName),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to list pull requests for branch '%s', resp %v: %w", branchName, resp.Status, err)
	}
	if len(prs) == 0 {
		return nil, fmt.Errorf("no open prs found for branch %s", branchName)
	}
	if len(prs) > 1 {
		return nil, fmt.Errorf("more that one open pr found for branch %s", branchName)
	}
	return prs[0], nil
}

func (r *Repo) GetPR(ctx context.Context, branchName string, ghCli *gh.GithubClient) (*github.PullRequest, error) {
	owner, repo, err := r.getOwnerRepo()
	if err != nil {
		return nil, errors.Wrap(err, "get owner repo")
	}
	return r.getPRForBranch(ctx, owner, repo, branchName, ghCli)
}
