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
	"github.com/lekkodev/cli/pkg/metadata"
	"github.com/pkg/errors"
	giturls "github.com/whilp/git-urls"
	"golang.org/x/oauth2"
)

const (
	mainBranchName = "main"
	remoteName     = "origin"
)

// Abstraction around git and github operations associated with the lekko configuration repo.
type ConfigRepo struct {
	Secrets metadata.Secrets

	repo  *git.Repository
	wt    *git.Worktree
	ghCli *github.Client
}

func New(ctx context.Context, path string) (*ConfigRepo, error) {
	repo, err := git.PlainOpen(path)
	if err != nil {
		return nil, errors.Wrap(err, "failed to open git repo")
	}
	wt, err := repo.Worktree()
	if err != nil {
		return nil, errors.Wrap(err, "failed to get work tree")
	}
	secrets := metadata.NewSecretsOrFail()
	cr := &ConfigRepo{
		repo:    repo,
		wt:      wt,
		Secrets: secrets,
	}
	cr.mkGhCli(ctx)
	return cr, nil
}

func (cr *ConfigRepo) mkGhCli(ctx context.Context) {
	ts := oauth2.StaticTokenSource(&oauth2.Token{AccessToken: cr.Secrets.GetGithubToken()})
	tc := oauth2.NewClient(ctx, ts)
	cr.ghCli = github.NewClient(tc)
}

func (cr *ConfigRepo) CheckGithubAuth(ctx context.Context) error {
	if !cr.Secrets.HasGithubToken() {
		fmt.Println(metadata.AuthenticateGituhubMessage)
		return errors.New("user unauthenticated")
	}
	_, resp, err := cr.ghCli.Users.Get(ctx, "")
	if err != nil {
		return fmt.Errorf("github auth check failed [%v]: %w", resp, err)
	}
	return nil
}

func (cr *ConfigRepo) Close() {
	if err := cr.Secrets.Close(); err != nil {
		log.Printf("error closing secrets: %v\n", err)
	}
}

func (cr *ConfigRepo) Review(ctx context.Context) error {
	if err := cr.CheckGithubAuth(ctx); err != nil {
		return err
	}
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

	if err := cr.pushLocalBranch(branchName); err != nil {
		return errors.Wrap(err, "push")
	}
	if err := cr.createPR(ctx, branchName); err != nil {
		return errors.Wrap(err, "create pr")
	}
	return nil
}

func (cr *ConfigRepo) Merge(prNum int) error {
	ctx := context.Background()
	owner, repo, err := cr.getOwnerRepo()
	if err != nil {
		return errors.Wrap(err, "get owner repo")
	}
	result, resp, err := cr.ghCli.PullRequests.Merge(ctx, owner, repo, prNum, "", &github.PullRequestOptions{
		MergeMethod: "squash",
	})
	if err != nil {
		return fmt.Errorf("ghCli merge pr %v: %w", resp.Status, err)
	}
	if result.GetMerged() {
		fmt.Printf("PR #%d: %s\n", prNum, result.GetMessage())
	} else {
		return errors.New("Failed to merge pull request.")
	}

	head, err := cr.repo.Head()
	if err != nil {
		return errors.Wrap(err, "head")
	}

	if err := cr.repo.Push(&git.PushOptions{
		RemoteName: remoteName,
		RefSpecs:   []config.RefSpec{config.RefSpec(fmt.Sprintf(":%s", head.Name()))},
		Auth: &http.BasicAuth{
			Username: cr.Secrets.GetGithubUser(),
			Password: cr.Secrets.GetGithubToken(),
		},
	}); err != nil {
		return fmt.Errorf("delete remote branch name %s: %w", head.Name(), err)
	}
	fmt.Printf("Successfully deleted remote branch %s\n", head.Name())

	if err := cr.repo.DeleteBranch(head.Name().Short()); err != nil {
		return fmt.Errorf("delete local branch name %s: %w", head.Name().Short(), err)
	}
	fmt.Printf("Successfully deleted local branch %s\n", head.Name().Short())

	// cmd := exec.Command("gh", "pr", "merge", "-sd")
	// cmd.Stderr = os.Stderr
	// cmd.Stdout = os.Stdout
	// if err := cmd.Run(); err != nil {
	// 	return errors.Wrap(err, "gh pr merge")
	// }
	return nil
}

func (cr *ConfigRepo) WorkingDirectoryHash() (string, error) {
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

func (cr *ConfigRepo) isMain() (bool, error) {
	h, err := cr.repo.Head()
	if err != nil {
		return false, errors.Wrap(err, "head")
	}
	return h.Name().IsBranch() && h.Name().Short() == mainBranchName, nil
}

func (cr *ConfigRepo) BranchName() (string, error) {
	h, err := cr.repo.Head()
	if err != nil {
		return "", errors.Wrap(err, "head")
	}
	return h.Name().Short(), nil
}

func (cr *ConfigRepo) getOwnerRepo() (string, string, error) {
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

func (cr *ConfigRepo) pushLocalBranch(branchName string) error {
	if err := cr.repo.Push(&git.PushOptions{
		RemoteName: remoteName,
		Auth: &http.BasicAuth{
			Username: cr.Secrets.GetGithubUser(),
			Password: cr.Secrets.GetGithubToken(),
		},
	}); err != nil {
		return errors.Wrap(err, "push")
	}
	// set up tracking in case we need to pull from the
	// remote branch later
	if err := cr.repo.CreateBranch(&config.Branch{
		Name:   branchName,
		Remote: remoteName,
		Merge:  plumbing.NewBranchReferenceName(branchName),
	}); err != nil {
		return errors.Wrap(err, "create branch tracking configuration")
	}
	fmt.Printf("Pushed local branch %q to remote %q\n", branchName, remoteName)
	return nil
}

func (cr *ConfigRepo) createPR(ctx context.Context, branchName string) error {
	owner, repo, err := cr.getOwnerRepo()
	if err != nil {
		return errors.Wrap(err, "get owner repo")
	}
	pr, resp, err := cr.ghCli.PullRequests.Create(ctx, owner, repo, &github.NewPullRequest{
		Title: strPtr("New feature change"),
		Head:  &branchName,
		Base:  strPtr(mainBranchName),
	})
	if err != nil {
		return fmt.Errorf("ghCli create pr status %v: %w", resp.Status, err)
	}
	fmt.Printf("Created PR:\n\t%s\n", pr.GetHTMLURL())
	return nil
}
func strPtr(s string) *string {
	return &s
}
