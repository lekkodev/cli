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
	"log"
	"strings"
	"time"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/config"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/transport/http"
	"github.com/google/go-github/v47/github"
	"github.com/pkg/errors"
)

func (r *Repo) Review(ctx context.Context) error {
	if err := r.CheckGithubAuth(ctx); err != nil {
		return errors.Wrap(err, "check github auth")
	}
	if err := r.ensureMainBranch(false); err != nil {
		return errors.Wrap(err, "is main")
	}

	clean, err := r.wdClean()
	if err != nil {
		return errors.Wrap(err, "wd clean")
	}
	if clean {
		return errors.New("current working directory is clean, nothing to review")
	}

	branchName, err := r.checkoutLocalBranch()
	if err != nil {
		return errors.Wrap(err, "checkout new branch")
	}

	if err := r.Wt.AddGlob("."); err != nil {
		return errors.Wrap(err, "add glob")
	}
	log.Printf("checked out new branch %s\n", branchName)

	hash, err := r.Wt.Commit("new config changes", &git.CommitOptions{
		All: true,
	})
	if err != nil {
		return errors.Wrap(err, "commit")
	}
	log.Printf("committed new hash %s\n", hash)

	if err := r.pushToRemote(branchName); err != nil {
		return errors.Wrap(err, "push")
	}
	if err := r.createPR(ctx, branchName); err != nil {
		return errors.Wrap(err, "create pr")
	}
	return nil
}

func (r *Repo) Merge(prNum int) error {
	ctx := context.Background()
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

	head, err := r.Repo.Head()
	if err != nil {
		return errors.Wrap(err, "head")
	}
	localBranchRef := head.Name()

	if err := r.Repo.Push(&git.PushOptions{
		RemoteName: remoteName,
		RefSpecs:   []config.RefSpec{config.RefSpec(fmt.Sprintf(":%s", localBranchRef))},
		Auth: &http.BasicAuth{
			Username: r.User,
			Password: r.Token,
		},
	}); err != nil {
		return fmt.Errorf("delete remote branch name %s: %w", localBranchRef, err)
	}
	r.Logf("Successfully deleted remote branch %s\n", localBranchRef)

	if err := r.Wt.Checkout(&git.CheckoutOptions{
		Branch: plumbing.NewBranchReferenceName(mainBranchName),
	}); err != nil {
		return fmt.Errorf("failed to checkout main branch '%s': %w", mainBranchName, err)
	}
	r.Logf("Checked out local branch %s\n", mainBranchName)
	if err := r.Repo.DeleteBranch(localBranchRef.Short()); err != nil {
		return fmt.Errorf("delete local branch name %s: %w", localBranchRef.Short(), err)
	}
	if err := r.Repo.Storer.RemoveReference(localBranchRef); err != nil {
		return fmt.Errorf("remove reference %s: %w", localBranchRef, err)
	}
	r.Logf("Successfully deleted local branch %s\n", localBranchRef.Short())
	if err := r.Wt.Pull(&git.PullOptions{
		RemoteName: remoteName,
		Auth: &http.BasicAuth{
			Username: r.User,
			Password: r.Token,
		},
	}); err != nil {
		return errors.Wrap(err, "failed to pull main")
	}
	r.Logf("Pulled from remote. Local branch %s is up to date.\n", mainBranchName)
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

func (r *Repo) pushToRemote(branchName string) error {
	if err := r.Repo.Push(&git.PushOptions{
		RemoteName: remoteName,
		RefSpecs: []config.RefSpec{
			config.RefSpec(fmt.Sprintf(
				"%s:%s",
				plumbing.NewBranchReferenceName(branchName),
				plumbing.NewRemoteReferenceName(remoteName, branchName),
			)),
		},
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

func (r *Repo) createPR(ctx context.Context, branchName string) error {
	owner, repo, err := r.getOwnerRepo()
	if err != nil {
		return errors.Wrap(err, "get owner repo")
	}
	pr, resp, err := r.GhCli.PullRequests.Create(ctx, owner, repo, &github.NewPullRequest{
		Title: strPtr("New feature change"),
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
