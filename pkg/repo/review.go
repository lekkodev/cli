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
	"github.com/lekkodev/cli/pkg/logging"
	"github.com/pkg/errors"
)

// Review will open a pull request. It takes different actions depending on
// whether or not we are currently on main, and whether or not the working
// directory is clean.
func (r *Repo) Review(ctx context.Context, title string, ghCli *gh.GithubClient, ap AuthProvider) (string, error) {
	if err := credentialsExist(ap); err != nil {
		return "", err
	}
	var main, clean bool
	var err error
	main, err = r.isMain()
	if err != nil {
		return "", errors.Wrap(err, "is main")
	}
	clean, err = r.IsClean()
	if err != nil {
		return "", errors.Wrap(err, "wd clean")
	}
	var branchName string
	if main {
		if clean {
			return "", errors.Errorf("nothing to review on main with clean wd")
		}
		// dirty working directory on main
		branchName, err = r.checkoutLocalBranch(ap)
		if err != nil {
			return "", errors.Wrap(err, "checkout new branch")
		}
		// set up remote branch tracking
		if err := r.setTrackingConfig(branchName); err != nil {
			return "", errors.Wrap(err, "push to remote")
		}
		if _, err := r.Commit(ctx, ap, title); err != nil {
			return "", errors.Wrap(err, "main add commit push")
		}
	} else {
		branchName, err = r.BranchName()
		if err != nil {
			return "", err
		}
		if !clean {
			if _, err := r.Commit(ctx, ap, title); err != nil {
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

func (r *Repo) Merge(ctx context.Context, prNum *int, ghCli *gh.GithubClient, ap AuthProvider) error {
	if err := credentialsExist(ap); err != nil {
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
	if err := r.Cleanup(ctx, &branchName, ap); err != nil {
		return errors.Wrap(err, "cleanup")
	}
	return nil
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

func (r *Repo) GenBranchName(ghUsername string) (string, error) {
	if ghUsername == "" {
		cfg, err := config.LoadConfig(config.GlobalScope)
		if err != nil {
			return "", errors.Wrap(err, "load global config")
		}
		ghUsername = strings.ToLower(strings.Split(cfg.User.Name, " ")[0])
	}
	if ghUsername == "" {
		ghUsername = "anon"
	}
	return fmt.Sprintf("%s-review-%d", ghUsername, time.Now().Nanosecond()), nil
}

// Creates a new remote branch at the given sha, and checks out the
// new branch locally, setting up the correct tracking config.
// branchName must be sufficiently unique.
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

func (r *Repo) checkoutLocalBranch(ap AuthProvider) (string, error) {
	branchName, err := r.GenBranchName(ap.GetUsername())
	if err != nil {
		return "", errors.Wrap(err, "gen branch name")
	}
	if err := r.wt.Checkout(&git.CheckoutOptions{
		Branch: plumbing.NewBranchReferenceName(branchName),
		Create: true,
		Keep:   true,
	}); err != nil {
		return "", errors.Wrap(err, "checkout")
	}
	r.Logf("Checked out local branch %s\n", branchName)
	return branchName, nil
}

func (r *Repo) pushToRemote(ctx context.Context, ap AuthProvider) error {
	if err := r.repo.PushContext(ctx, &git.PushOptions{
		RemoteName: RemoteName,
		Auth:       basicAuth(ap),
	}); err != nil {
		return errors.Wrap(err, "push")
	}
	return nil
}

func (r *Repo) setTrackingConfig(branchName string) error {
	if err := r.repo.CreateBranch(&config.Branch{
		Name:   branchName,
		Remote: RemoteName,
		Merge:  plumbing.NewBranchReferenceName(branchName),
	}); err != nil && !errors.Is(err, git.ErrBranchExists) {
		return errors.Wrap(err, "create branch tracking configuration")
	}
	r.Logf("Local branch %s has been set up to track changes from %s\n",
		branchName, plumbing.NewRemoteReferenceName(RemoteName, branchName))
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
		Base:  strPtr(MainBranchName),
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
		return nil, errors.Wrapf(ErrNotFound, "no open prs found for branch %s", branchName)
	}
	if len(prs) > 1 {
		return nil, fmt.Errorf("more that one open pr found for branch %s", branchName)
	}
	return prs[0], nil
}

func (r *Repo) GetPRInfo(ctx context.Context, branchName string, ghCli *gh.GithubClient) (*github.PullRequest, []*github.PullRequestReview, []*github.CheckRun, error) {
	owner, repo, err := r.getOwnerRepo()
	if err != nil {
		return nil, nil, nil, errors.Wrap(err, "get owner repo")
	}
	pr, err := r.getPRForBranch(ctx, owner, repo, branchName, ghCli)
	if err != nil {
		return nil, nil, nil, errors.Wrap(err, "get pr for branch")
	}
	reviews, resp, err := ghCli.PullRequests.ListReviews(ctx, owner, repo, *pr.Number, nil)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("failed to list pull requests #%d reviews, resp %v: %w", *pr.Number, resp.Status, err)
	}
	checks, resp, err := ghCli.Checks.ListCheckRunsForRef(ctx, owner, repo, *pr.Head.Ref, nil)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("failed to list pull requests #%d checks, resp %v: %w", *pr.Number, resp.Status, err)
	}
	var checkRuns []*github.CheckRun
	if checks != nil {
		checkRuns = checks.CheckRuns
	}
	return pr, reviews, checkRuns, nil
}
