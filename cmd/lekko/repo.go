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

package main

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strings"

	"github.com/AlecAivazis/survey/v2"
	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/lekkodev/cli/pkg/dotlekko"
	"github.com/lekkodev/cli/pkg/gh"
	"github.com/lekkodev/cli/pkg/lekko"
	"github.com/lekkodev/cli/pkg/logging"
	"github.com/lekkodev/cli/pkg/repo"
	"github.com/lekkodev/cli/pkg/secrets"
	"github.com/lekkodev/cli/pkg/sync"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
)

const (
	lekkoAppInstallURL string = "https://github.com/apps/lekko-app/installations/new"
)

func repoCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "repo",
		Short: "repository management",
	}
	cmd.AddCommand(
		repoListCmd,
		repoCreateCmd(),
		repoCloneCmd(),
		repoDeleteCmd(),
		repoInitCmd(),
		defaultRepoInitCmd(),
		importCmd(),
		remoteCmd(),
		pathCmd(),
		pushCmd(),
		pullCmd(),
		mergeFileCmd(),
	)
	return cmd
}

var repoListCmd = &cobra.Command{
	Use:   "list",
	Short: "List the config repositories in the currently active team",
	RunE: func(cmd *cobra.Command, args []string) error {
		rs := secrets.NewSecretsOrFail(secrets.RequireLekko())
		repo := repo.NewRepoCmd(lekko.NewBFFClient(rs), rs)
		repos, err := repo.List(cmd.Context())
		if err != nil {
			return err
		}
		fmt.Printf("%d repos found in team %s.\n", len(repos), rs.GetLekkoTeam())
		for _, r := range repos {
			fmt.Printf("%s:\n\t%s\n\t%s\n", logging.Bold(fmt.Sprintf("[%s/%s]", r.Owner, r.RepoName)), r.Description, r.URL)
		}
		return nil
	},
}

func waitForEnter(r io.Reader) error {
	scanner := bufio.NewScanner(r)
	scanner.Scan()
	return scanner.Err()
}

func repoCreateCmd() *cobra.Command {
	var owner, repoName, description string
	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create a new, empty config repository in the currently active team",
		RunE: func(cmd *cobra.Command, args []string) error {
			rs := secrets.NewSecretsOrFail(secrets.RequireLekko())
			if len(owner) == 0 {
				if err := survey.AskOne(&survey.Input{
					Message: "GitHub Owner:",
					Help:    "Name of the GitHub organization the create the repository under. If left empty, defaults to personal account.",
				}, &owner); err != nil {
					return errors.Wrap(err, "prompt")
				}
			}
			if len(owner) == 0 {
				owner = rs.GetGithubUser()
			}
			if len(repoName) == 0 {
				if err := survey.AskOne(&survey.Input{
					Message: "Repo Name:",
				}, &repoName); err != nil {
					return errors.Wrap(err, "prompt")
				}
			}
			if len(description) == 0 {
				if err := survey.AskOne(&survey.Input{
					Message: "Repo Description:",
					Help:    "Description for your new repository. If left empty, a default description message will be used.",
				}, &description); err != nil {
					return errors.Wrap(err, "prompt")
				}
			}
			fmt.Printf("Attempting to create a new configuration repository %s in team %s.\n", logging.Bold(fmt.Sprintf("[%s/%s]", owner, repoName)), rs.GetLekkoTeam())
			fmt.Printf("First, ensure that the github owner '%s' has installed Lekko App by visiting:\n\t%s\n", owner, lekkoAppInstallURL)
			fmt.Printf("Once done, press [Enter] to continue...")
			_ = waitForEnter(os.Stdin)

			repo := repo.NewRepoCmd(lekko.NewBFFClient(rs), rs)
			url, err := repo.Create(cmd.Context(), owner, repoName, description)
			if err != nil {
				return err
			}
			fmt.Printf("Successfully created a new config repository at %s.\n", url)
			fmt.Printf("To make configuration changes, run `lekko repo clone`.")
			return nil
		},
	}
	cmd.Flags().StringVarP(&owner, "owner", "o", "", "GitHub owner to create the repository under. If empty, defaults to the authorized user's personal account.")
	cmd.Flags().StringVarP(&repoName, "repo", "r", "", "GitHub repository name")
	cmd.Flags().StringVarP(&description, "description", "d", "", "GitHub repository description")
	return cmd
}

func repoCloneCmd() *cobra.Command {
	var url, wd string
	cmd := &cobra.Command{
		Use:   "clone",
		Short: "Clone an existing configuration repository to local disk",
		RunE: func(cmd *cobra.Command, args []string) error {
			rs := secrets.NewSecretsOrFail(secrets.RequireLekko())
			r := repo.NewRepoCmd(lekko.NewBFFClient(rs), rs)
			ctx := cmd.Context()
			if len(url) == 0 {
				var options []string
				repos, err := r.List(ctx)
				if err != nil {
					return errors.Wrap(err, "repos list")
				}
				for _, r := range repos {
					if len(r.URL) > 0 {
						options = append(options, r.URL)
					}
				}
				if len(options) == 0 {
					return errors.New("no url provided")
				}
				if err := survey.AskOne(&survey.Select{
					Message: "Repo URL:",
					Options: options,
				}, &url); err != nil {
					return errors.Wrap(err, "prompt")
				}
			}
			repoName := path.Base(url)
			fmt.Printf("Cloning %s into '%s'...\n", url, repoName)
			_, err := repo.NewLocalClone(path.Join(wd, repoName), url, rs)
			if err != nil {
				return errors.Wrap(err, "new local clone")
			}
			return nil
		},
	}
	cmd.Flags().StringVarP(&url, "url", "u", "", "url of GitHub-hosted configuration repository to clone")
	cmd.Flags().StringVarP(&wd, "config-path", "c", ".", "path to configuration repository")
	return cmd
}

func repoDeleteCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "delete",
		Short: "Delete an existing config repository",
		RunE: func(cmd *cobra.Command, args []string) error {
			rs := secrets.NewSecretsOrFail(secrets.RequireLekko())
			repo := repo.NewRepoCmd(lekko.NewBFFClient(rs), rs)
			ctx := cmd.Context()
			repos, err := repo.List(ctx)
			if err != nil {
				return errors.Wrap(err, "repos list")
			}
			if len(repos) == 0 {
				fmt.Println("No repositories found.")
				return nil
			}
			var options []string
			for _, r := range repos {
				options = append(options, fmt.Sprintf("%s/%s", r.Owner, r.RepoName))
			}
			var selected string
			if err := survey.AskOne(&survey.Select{
				Message: "Repository to delete:",
				Options: options,
			}, &selected); err != nil {
				return errors.Wrap(err, "prompt")
			}
			paths := strings.Split(selected, "/")
			if len(paths) != 2 {
				return errors.Errorf("malformed selection: %s", selected)
			}
			owner, repoName := paths[0], paths[1]
			var deleteOnGithub bool
			if err := survey.AskOne(&survey.Confirm{
				Message: "Also delete on GitHub?",
				Help:    "y/Y: repo is deleted on Github. n/N: repo remains on Github but is unlinked from Lekko.",
			}, &deleteOnGithub); err != nil {
				return errors.Wrap(err, "prompt")
			}
			text := "Unlinking repository '%s' from Lekko...\n"
			if deleteOnGithub {
				text = "Deleting repository '%s' from Github and Lekko...\n"
			}
			fmt.Printf(text, selected)
			if err := confirmInput(selected); err != nil {
				return err
			}
			if err := repo.Delete(ctx, owner, repoName, deleteOnGithub); err != nil {
				return errors.Wrap(err, "delete repo")
			}
			fmt.Printf("Successfully deleted repository %s.\n", selected)
			return nil
		},
	}
	return cmd
}

func repoInitCmd() *cobra.Command {
	var owner, repoName, description string
	cmd := &cobra.Command{
		Use:    "init",
		Short:  "[Deprecated] Initialize a new template git repository on GitHub",
		Hidden: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			if len(owner) == 0 || len(repoName) == 0 {
				return errors.Errorf("Must provide owner and repo name")
			}
			rs := secrets.NewSecretsOrFail(secrets.RequireGithub())
			ghCli := gh.NewGithubClientFromToken(ctx, rs.GetGithubToken())
			if rs.GetGithubUser() == owner {
				owner = "" // create repo expects an empty owner for personal accounts
			}

			ghRepo, err := ghCli.CreateRepo(ctx, owner, repoName, description, true)
			if err != nil {
				return errors.Wrap(err, "Failed to create GitHub repository")
			}
			if err := repo.MirrorAtURL(ctx, rs, ghRepo.GetCloneURL()); err != nil {
				return errors.Wrapf(err, "Failed to mirror at URL: %s", ghRepo.GetCloneURL())
			}

			fmt.Printf("Mirrored repo at URL %s\n", ghRepo.GetCloneURL())
			return nil
		},
	}
	cmd.Flags().StringVarP(&owner, "owner", "o", "", "GitHub owner to house repository in")
	cmd.Flags().StringVarP(&repoName, "repo", "r", "", "GitHub repository name")
	cmd.Flags().StringVarP(&description, "description", "d", "", "GitHub repository description")
	return cmd
}

func defaultRepoInitCmd() *cobra.Command {
	var repoPath string
	cmd := &cobra.Command{
		Use:   "init-default",
		Short: "Initialize a new template git repository in the default location",
		RunE: func(cmd *cobra.Command, args []string) error {
			rs := secrets.NewSecretsOrFail()
			_, err := repo.InitIfNotExists(rs, repoPath)
			return err
		},
	}
	cmd.Flags().StringVarP(&repoPath, "path", "p", "", "path to the repo location")
	return cmd
}

func importCmd() *cobra.Command {
	var owner, repoName, description, repoPath string
	cmd := &cobra.Command{
		Use:   "import",
		Short: "Import local repo into GitHub and Lekko",
		RunE: func(cmd *cobra.Command, args []string) error {
			rs := secrets.NewSecretsOrFail(secrets.RequireGithub(), secrets.RequireLekko())
			repo := repo.NewRepoCmd(lekko.NewBFFClient(rs), rs)
			return repo.Import(cmd.Context(), repoPath, owner, repoName, description)
		},
	}
	cmd.Flags().StringVarP(&owner, "owner", "o", "", "GitHub owner to house repository in")
	cmd.Flags().StringVarP(&repoName, "repo", "r", "", "GitHub repository name")
	cmd.Flags().StringVarP(&description, "description", "d", "", "GitHub repository description")
	cmd.Flags().StringVarP(&repoPath, "path", "p", "", "path to the repo location")
	return cmd
}

func remoteCmd() *cobra.Command {
	var remoteRepo string
	cmd := &cobra.Command{
		Use:   "remote",
		Short: "Show the remote Lekko repo currently in use",
		RunE: func(cmd *cobra.Command, args []string) error {
			dot, err := dotlekko.ReadDotLekko()
			if err != nil {
				return err
			}

			if len(remoteRepo) > 0 {
				doIt := false
				if err := survey.AskOne(&survey.Confirm{
					Message: fmt.Sprintf("Set remote repo to '%s'?", remoteRepo),
					Default: false,
				}, &doIt); err != nil {
					return err
				}
				if !doIt {
					fmt.Println("Aborted!")
					return nil
				}
				parts := strings.Split(remoteRepo, "/")
				if len(parts) != 2 || len(parts[0]) == 0 || len(parts[1]) == 0 {
					return errors.New("Invalid remote repo format, shoud be '<GitHub owner>/<GitHub repo>'")
				}
				dot.Repository = remoteRepo
				return dot.WriteBack()
			}

			if len(dot.Repository) == 0 {
				return errors.New("Repository info is not set in .lekko")
			}

			fmt.Println(dot.Repository)
			return nil
		},
	}
	cmd.Flags().StringVar(&remoteRepo, "set", "", "set the remote Lekko repo, use '<GitHub owner>/<GitHub repo>' format")
	return cmd
}

func pushCmd() *cobra.Command {
	var commitMessage string
	var forceLock bool
	cmd := &cobra.Command{
		Use:   "push",
		Short: "Push local changes to remote Lekko repo.",
		RunE: func(cmd *cobra.Command, args []string) error {
			dot, err := dotlekko.ReadDotLekko()
			if err != nil {
				return errors.Wrap(err, "read Lekko configuration file")
			}
			rs := secrets.NewSecretsOrFail(secrets.RequireGithub(), secrets.RequireLekko())
			return sync.Push(cmd.Context(), commitMessage, forceLock, rs, dot)
		},
	}
	cmd.Flags().StringVarP(&commitMessage, "commit-message", "m", "", "commit message")
	cmd.Flags().BoolVarP(&forceLock, "force", "f", false, "whether to force push, ignoring base commit information from lekko.lock")
	return cmd
}

func pathCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "path",
		Short: "Show the local repo path currently in use",
		RunE: func(cmd *cobra.Command, args []string) error {
			dot, err := dotlekko.ReadDotLekko()
			if err != nil {
				return err
			}
			if len(dot.Repository) == 0 {
				return errors.New("Repository info is not set in .lekko")
			}
			base, err := repo.DefaultRepoBasePath()
			if err != nil {
				return err
			}
			repoOwner, repoName := dot.GetRepoInfo()
			fmt.Println(filepath.Join(base, repoOwner, repoName))
			return nil
		},
	}
	return cmd
}

func HasLekkoChanges(lekkoPath string) (bool, error) {
	gitRoot, err := repo.GetGitRootPath()
	if err != nil {
		return false, err
	}
	wd, err := os.Getwd()
	if err != nil {
		return false, err
	}
	codeRepo, err := git.PlainOpen(gitRoot)
	if err != nil {
		return false, err
	}
	wt, err := codeRepo.Worktree()
	if err != nil {
		return false, err
	}
	st, err := wt.Status()
	if err != nil {
		return false, err
	}
	for p, s := range st {
		relPath, err := filepath.Rel(wd, filepath.Join(gitRoot, p))
		if err == nil && strings.HasPrefix(relPath, lekkoPath) {
			if s.Staging != git.Unmodified || s.Worktree != git.Unmodified {
				return true, nil
			}
		}
	}
	return false, nil
}

func pullCmd() *cobra.Command {
	var force bool
	cmd := &cobra.Command{
		Use:   "pull",
		Short: "Pull remote changes and merge them with local changes.",
		RunE: func(cmd *cobra.Command, args []string) error {
			dot, err := dotlekko.ReadDotLekko()
			if err != nil {
				return errors.Wrap(err, "read Lekko configuration file")
			}

			nativeLang, err := sync.DetectNativeLang()
			if err != nil {
				return err
			}

			rs := secrets.NewSecretsOrFail(secrets.RequireGithub(), secrets.RequireLekko())
			repoPath, err := repo.PrepareGithubRepo(rs)
			if err != nil {
				return err
			}
			gitRepo, err := git.PlainOpen(repoPath)
			if err != nil {
				return errors.Wrap(err, "open git repo")
			}
			// this should be safe as we generate all changes from native lang
			// git reset --hard
			// git clean -fd
			err = repo.ResetAndClean(gitRepo)
			if err != nil {
				return errors.Wrap(err, "reset and clean")
			}

			worktree, err := gitRepo.Worktree()
			if err != nil {
				return errors.Wrap(err, "get worktree")
			}
			err = worktree.Checkout(&git.CheckoutOptions{
				Branch: plumbing.NewBranchReferenceName("main"),
			})
			if err != nil {
				return errors.Wrap(err, "checkout main")
			}

			// pull from remote
			remotes, err := gitRepo.Remotes()
			if err != nil {
				return errors.Wrap(err, "get remotes")
			}
			if len(remotes) == 0 {
				return errors.New("No remote found, please finish setup instructions")
			}
			fmt.Printf("Pulling from %s\n", remotes[0].Config().URLs[0])
			err = repo.GitPull(gitRepo, rs)
			if err != nil {
				return errors.Wrap(err, "git pull")
			}
			newHead, err := gitRepo.Head()
			if err != nil {
				return err
			}

			lekkoPath := dot.LekkoPath
			if len(dot.LockSHA) == 0 {
				fmt.Println("No Lekko lock information found, syncing from remote...")
				// no lekko lock, sync from remote
				if !force {
					hasLekkoChanges, err := HasLekkoChanges(lekkoPath)
					if err != nil {
						return errors.New("Lekko requires code to be in a git repository")
					}
					if hasLekkoChanges {
						return fmt.Errorf("please commit or stash changes in '%s' before pulling", lekkoPath)
					}
				}
				nativeFiles, err := repo.ListNativeConfigFiles(lekkoPath, nativeLang.Ext())
				if err != nil {
					return err
				}
				for _, f := range nativeFiles {
					ns := nativeLang.GetNamespace(f)
					err := sync.GenNative(cmd.Context(), nativeLang, dot.LekkoPath, repoPath, ns, ".")
					if err != nil {
						return err
					}
				}

				dot.LockSHA = newHead.Hash().String()
				if err := dot.WriteBack(); err != nil {
					return errors.Wrap(err, "write back .lekko")
				}

				return nil
			}

			if newHead.Hash().String() == dot.LockSHA {
				fmt.Println("Already up to date.")
				return nil
			}
			fmt.Printf("Rebasing from %s to %s\n\n", dot.LockSHA, newHead.Hash().String())

			switch nativeLang {
			case sync.TS:
				tsPullCmd := exec.Command("npx", "lekko-repo-pull", "--lekko-dir", lekkoPath)
				output, err := tsPullCmd.CombinedOutput()
				fmt.Println(string(output))
				if err != nil {
					return errors.Wrap(err, "ts pull")
				}
			case sync.GO:
				files, err := sync.Bisync(cmd.Context(), lekkoPath, lekkoPath, repoPath)
				if err != nil {
					return errors.Wrap(err, "go bisync")
				}
				for _, f := range files {
					err = mergeFile(cmd.Context(), f, dot)
					if err != nil {
						return errors.Wrapf(err, "merge file %s", f)
					}
				}
			default:
				return fmt.Errorf("unsupported language: %s", nativeLang)
			}

			dot.LockSHA = newHead.Hash().String()
			if err := dot.WriteBack(); err != nil {
				return errors.Wrap(err, "write back .lekko")
			}

			return nil
		},
	}
	cmd.Flags().BoolVarP(&force, "force", "f", false, "allow dirty git state when pulling without valid lekko.lock")
	return cmd
}

func mergeFile(ctx context.Context, filename string, dot *dotlekko.DotLekko) error {
	nativeLang, err := sync.NativeLangFromExt(filename)
	if err != nil {
		return err
	}
	ns := nativeLang.GetNamespace(filename)

	fileBytes, err := os.ReadFile(filename)
	if err != nil {
		return errors.Wrap(err, "read file")
	}
	if bytes.Contains(fileBytes, []byte("<<<<<<<")) {
		return fmt.Errorf("%s has unresolved merge conflicts", filename)
	}

	if len(dot.LockSHA) == 0 {
		return errors.New("no Lekko lock information found")
	}

	rs := secrets.NewSecretsOrFail(secrets.RequireGithub(), secrets.RequireLekko())
	repoPath, err := repo.PrepareGithubRepo(rs)
	if err != nil {
		return err
	}
	gitRepo, err := git.PlainOpen(repoPath)
	if err != nil {
		return errors.Wrap(err, "open git repo")
	}
	err = repo.ResetAndClean(gitRepo)
	if err != nil {
		return err
	}
	worktree, err := gitRepo.Worktree()
	if err != nil {
		return errors.Wrap(err, "get worktree")
	}

	// base
	err = worktree.Checkout(&git.CheckoutOptions{
		Hash: plumbing.NewHash(dot.LockSHA),
	})
	if err != nil {
		return errors.Wrap(err, fmt.Sprintf("checkout %s", dot.LockSHA))
	}

	baseDir, err := os.MkdirTemp("", "lekko-merge-base-")
	if err != nil {
		return errors.Wrap(err, "create temp dir")
	}
	defer os.RemoveAll(baseDir)
	err = sync.GenNative(ctx, nativeLang, dot.LekkoPath, repoPath, ns, baseDir)
	if err != nil {
		return errors.Wrap(err, "gen native")
	}

	getCommitInfo := func() (string, error) {
		head, err := gitRepo.Head()
		if err != nil {
			return "", err
		}
		commit, err := gitRepo.CommitObject(head.Hash())
		if err != nil {
			return "", err
		}
		shortHash := head.Hash().String()[:7]
		title := strings.Split(commit.Message, "\n")[0]
		return fmt.Sprintf("%s - %s", shortHash, title), nil
	}

	baseCommitInfo, err := getCommitInfo()
	if err != nil {
		return errors.Wrap(err, "get commit info")
	}

	// remote
	err = worktree.Checkout(&git.CheckoutOptions{
		Branch: plumbing.NewBranchReferenceName("main"),
	})
	if err != nil {
		return errors.Wrap(err, "checkout main")
	}
	remoteDir, err := os.MkdirTemp("", "lekko-merge-remote-")
	if err != nil {
		return errors.Wrap(err, "create temp dir")
	}
	defer os.RemoveAll(remoteDir)
	err = sync.GenNative(ctx, nativeLang, dot.LekkoPath, repoPath, ns, remoteDir)
	if err != nil {
		return errors.Wrap(err, "gen native")
	}

	remoteCommitInfo, err := getCommitInfo()
	if err != nil {
		return errors.Wrap(err, "get commit info")
	}

	baseFilename := filepath.Join(baseDir, filename)
	remoteFilename := filepath.Join(remoteDir, filename)

	fmt.Printf("Auto-merging %s\n", filename)
	mergeCmd := exec.Command(
		"git", "merge-file",
		filename, baseFilename, remoteFilename,
		"-L", "local",
		"-L", fmt.Sprintf("base: %s", baseCommitInfo),
		"-L", fmt.Sprintf("remote: %s", remoteCommitInfo),
		"--diff3")
	err = mergeCmd.Run()
	if err != nil {
		exitErr, ok := err.(*exec.ExitError)
		// positive error code is fine, it signals number of conflicts
		if ok && exitErr.ExitCode() > 0 {
			fmt.Printf("dbg// exit code %s %v\n", mergeCmd.String(), exitErr.ExitCode())
			fmt.Printf("CONFLICT (content): Merge conflict in %s\n", filename)
		} else {
			return errors.Wrap(err, "git merge-file")
		}
	}

	return nil
}

func mergeFileCmd() *cobra.Command {
	var tsFilename string
	cmd := &cobra.Command{
		Use: "merge-file",
		Short: ("Merge native lekko file with remote changes. " +
			"Assumes that the repo is up-to-date. Typescript only."),
		Hidden: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			dot, err := dotlekko.ReadDotLekko()
			if err != nil {
				return errors.Wrap(err, "open Lekko configuration file")
			}
			return mergeFile(cmd.Context(), tsFilename, dot)
		},
	}
	cmd.Flags().StringVarP(&tsFilename, "filename", "f", "", "path to ts file to pull changes into")
	return cmd
}
