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
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/AlecAivazis/survey/v2"
	"github.com/lekkodev/cli/pkg/gh"
	"github.com/lekkodev/cli/pkg/lekko"
	"github.com/lekkodev/cli/pkg/logging"
	"github.com/lekkodev/cli/pkg/repo"
	"github.com/lekkodev/cli/pkg/secrets"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
)

const (
	lekkoAppInstallURL string = "https://github.com/apps/lekko-app/installations/new"
)

const (
	DeleteOnGitHubFlag            = "delete-on-github"
	DeleteOnGitHubFlagShort       = "x"
	DeleteOnGitHubFlagDVal        = false
	DeleteOnGitHubFlagDescription = "deletes the repository on GitHub"
)

var ForceFlagDescriptionLocal string = "forces all user confirmations to true, requires non-interactive mode"

func repoCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "repo",
		Short: "Repository management",
	}
	cmd.AddCommand(
		repoListCmd(),
		repoCreateCmd(),
		repoCloneCmd(),
		repoDeleteCmd(),
		repoInitCmd(),
	)
	return cmd
}

func repoListCmd() *cobra.Command {
	var isQuiet bool
	cmd := &cobra.Command{
		Short:                 "List the config repositories in the currently active team",
		Use:                   formCmdUse("list"),
		DisableFlagsInUseLine: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := errIfMoreArgs([]string{}, args); err != nil {
				return err
			}
			rs := secrets.NewSecretsOrFail(secrets.RequireLekko())
			repo := repo.NewRepoCmd(lekko.NewBFFClient(rs))
			repos, err := repo.List(cmd.Context())
			if err != nil {
				return err
			}
			if !isQuiet {
				fmt.Printf("%d repos found in team %s\n", len(repos), rs.GetLekkoTeam())
			}
			printRepos(cmd, repos)
			return nil
		},
	}
	cmd.Flags().BoolVarP(&isQuiet, QuietModeFlag, QuietModeFlagShort, QuietModeFlagDVal, QuietModeFlagDescription)
	return cmd
}

func waitForEnter(r io.Reader) error {
	scanner := bufio.NewScanner(r)
	scanner.Scan()
	return scanner.Err()
}

func repoCreateCmd() *cobra.Command {
	var owner, repoName, description string
	var isDryRun, isQuiet bool
	cmd := &cobra.Command{
		Short:                 "Create a new, empty config repository in the currently active team",
		Use:                   formCmdUse("create", "[owner/]repoName"),
		DisableFlagsInUseLine: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := errIfMoreArgs([]string{"[owner/]repoName"}, args); err != nil {
				return err
			}
			rs := secrets.NewSecretsOrFail(secrets.RequireLekko())
			rArgs, _ := getNArgs(1, args)

			if strings.Contains(rArgs[0], "/") {
				ownerRepoName := strings.Split(rArgs[0], "/")
				owner = ownerRepoName[0]
				repoName = ownerRepoName[1]
			} else {
				repoName = rArgs[0]
			}

			if len(owner) == 0 && IsInteractive {
				if err := survey.AskOne(&survey.Input{
					Message: "GitHub Owner:",
					Help:    "Name of the GitHub organization the create the repository under. If left empty, defaults to personal account.",
				}, &owner); err != nil {
					return errors.Wrap(err, "prompt")
				}
			}
			if len(owner) == 0 {
				owner = rs.GetGithubUser()
				if !IsInteractive && !isQuiet {
					printLinef(cmd, "Owner defaulted to %s\n", owner)
				}
			}

			if len(repoName) == 0 {
				if IsInteractive {
					if err := survey.AskOne(&survey.Input{
						Message: "Repo Name:",
					}, &repoName); err != nil {
						return errors.Wrap(err, "prompt")
					}
				} else {
					return errors.New("New repo argument is required in the form [owner/]repoName\n")
				}
			}

			if len(description) == 0 {
				if IsInteractive {
					if err := survey.AskOne(&survey.Input{
						Message: "Repo Description:",
						Help:    "Description for your new repository. If left empty, a default description message will be used.",
					}, &description); err != nil {
						return errors.Wrap(err, "prompt")
					}
				} else if !isQuiet {
					printLinef(cmd, "Default description message will be used for your new repository\n")
				}
			}

			if IsInteractive && !isQuiet {
				fmt.Printf("Attempting to create a new configuration repository %s in team %s.\n", logging.Bold(fmt.Sprintf("[%s/%s]", owner, repoName)), rs.GetLekkoTeam())
				fmt.Printf("First, ensure that the github owner '%s' has installed Lekko App by visiting:\n\t%s\n", owner, lekkoAppInstallURL)
				fmt.Printf("Once done, press [Enter] to continue...")
				_ = waitForEnter(os.Stdin)
			}

			if !isDryRun {
				rpo := repo.NewRepoCmd(lekko.NewBFFClient(rs))
				url, err := rpo.Create(cmd.Context(), owner, repoName, description)
				url = strings.TrimRight(url, ".git")
				if err != nil {
					return err
				}
				if !isQuiet {
					printLinef(cmd, "Created a new config repository %s/%s at %s\n", owner, repoName, url)
					if IsInteractive {
						printLinef(cmd, "To make configuration changes, run `lekko repo clone`\n")
					}
				} else {
					printLinef(cmd, "%s", url)
				}
			} else {
				if !isQuiet {
					printLinef(cmd, "Created a new config repository %s/%s\n", owner, repoName)
					fmt.Printf("To make configuration changes, run `lekko repo clone`.")
				} else {
					fmt.Printf("%s/%s", owner, repoName)
				}
			}
			return nil
		},
	}
	cmd.Flags().BoolVarP(&isQuiet, QuietModeFlag, QuietModeFlagShort, QuietModeFlagDVal, QuietModeFlagDescription)
	cmd.Flags().BoolVarP(&isDryRun, DryRunFlag, DryRunFlagShort, DryRunFlagDVal, DryRunFlagDescription)
	cmd.Flags().StringVarP(&description, "description", "m", "", "GitHub repository description")

	return cmd
}

func repoCloneCmd() *cobra.Command {
	var isDryRun, isQuiet bool
	var url, repoPath string
	cmd := &cobra.Command{
		Short:                 "Clone an existing configuration repository to local disk",
		Use:                   formCmdUse("clone", "repoUrl"),
		DisableFlagsInUseLine: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := errIfMoreArgs([]string{"repoUrl"}, args); err != nil {
				return err
			}
			wd, err := os.Getwd()
			if err != nil {
				return err
			}
			rs := secrets.NewSecretsOrFail(secrets.RequireLekko())
			r := repo.NewRepoCmd(lekko.NewBFFClient(rs))
			ctx := cmd.Context()
			rArgs, _ := getNArgs(1, args)
			url = rArgs[0]

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
			if repoPath, err = filepath.Abs(repoName); err != nil {
				return errors.Wrap(err, "clone")
			}
			if !isQuiet {
				printLinef(cmd, "cloning '%s' of url %s into '%s'... ", repoName, url, repoPath)
			}
			if !isDryRun {
				_, err = repo.NewLocalClone(path.Join(wd, repoName), url, rs)
			}
			if err != nil {
				return errors.Wrap(err, "new local clone")
			} else if !isQuiet {
				printLinef(cmd, "completed\n")
			} else {
				printLinef(cmd, "%s", repoPath)
			}
			return nil
		},
	}
	cmd.Flags().BoolVarP(&isQuiet, QuietModeFlag, QuietModeFlagShort, QuietModeFlagDVal, QuietModeFlagDescription)
	cmd.Flags().BoolVarP(&isDryRun, DryRunFlag, DryRunFlagShort, DryRunFlagDVal, DryRunFlagDescription)
	return cmd
}

func repoDeleteCmd() *cobra.Command {
	var isQuiet, isDryRun, isForce, isDeleteOnGithub bool
	cmd := &cobra.Command{
		Short:                 "Delete an existing config repository",
		Use:                   formCmdUse("delete", "owner/repoName"),
		DisableFlagsInUseLine: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := errIfMoreArgs([]string{"owner/repoName"}, args); err != nil {
				return err
			}
			rs := secrets.NewSecretsOrFail(secrets.RequireLekko())
			repo := repo.NewRepoCmd(lekko.NewBFFClient(rs))
			ctx := cmd.Context()
			repos, err := repo.List(ctx)
			if err != nil {
				return errors.Wrap(err, "repos list")
			}
			var selected string
			rArgs, n := getNArgs(1, args)

			if n == 1 {
				selected = rArgs[0]
			} else {
				var options []string
				for _, r := range repos {
					options = append(options, fmt.Sprintf("%s/%s", r.Owner, r.RepoName))
				}

				if err := survey.AskOne(&survey.Select{
					Message: "Repository to delete:",
					Options: options,
				}, &selected); err != nil {
					return errors.Wrap(err, "prompt")
				}
			}
			paths := strings.Split(selected, "/")
			if len(paths) != 2 {
				return errors.Errorf("malformed selection: %s", selected)
			}
			owner, repoName := paths[0], paths[1]

			if !isForce || IsInteractive {
				if !isDeleteOnGithub {
					if err := survey.AskOne(&survey.Confirm{
						Message: "Also delete on GitHub?",
						Help:    "y/Y: repo is deleted on Github. n/N: repo remains on Github but is unlinked from Lekko.",
					}, &isDeleteOnGithub); err != nil {
						return errors.Wrap(err, "prompt")
					}
				}
				text := "Unlinking repository '%s' from Lekko...\n"
				if isDeleteOnGithub {
					text = "Deleting repository '%s' from Github and Lekko...\n"
				}
				fmt.Printf(text, selected)
				if err := confirmInput(selected); err != nil {
					return err
				}
			} else {
				isDeleteOnGithub = true
				if !isQuiet {
					text := "Deleting repository '%s' from Github and Lekko...\n"
					fmt.Printf(text, selected)
				}
			}
			if !isDryRun {
				if err := repo.Delete(ctx, owner, repoName, isDeleteOnGithub); err != nil {
					return errors.Wrap(err, "delete repo")
				}
			}
			if !isQuiet {
				printLinef(cmd, "Successfully deleted repository %s.\n", selected)
			} else {
				printLinef(cmd, "%s", selected)
			}
			return nil
		},
	}
	cmd.Flags().BoolVarP(&isDeleteOnGithub, DeleteOnGitHubFlag, DeleteOnGitHubFlagShort, DeleteOnGitHubFlagDVal, DeleteOnGitHubFlagDescription)
	cmd.Flags().BoolVarP(&isQuiet, QuietModeFlag, QuietModeFlagShort, QuietModeFlagDVal, QuietModeFlagDescription)
	cmd.Flags().BoolVarP(&isDryRun, DryRunFlag, DryRunFlagShort, DryRunFlagDVal, DryRunFlagDescription)
	cmd.Flags().BoolVarP(&isForce, ForceFlag, ForceFlagShort, ForceFlagDVal, ForceFlagDescriptionLocal)
	return cmd
}

func repoInitCmd() *cobra.Command {
	var owner, repoName, description string
	cmd := &cobra.Command{
		Use:   "init",
		Short: "Initialize a new template git repository",
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

func printRepos(cmd *cobra.Command, repos []*repo.Repository) {
	if isQuiet, _ := cmd.Flags().GetBool(QuietModeFlag); isQuiet {
		reposStr := ""
		for _, r := range repos {
			reposStr += fmt.Sprintf("%s ", r.URL)
		}
		reposStr = strings.TrimSpace(reposStr)
		printLinef(cmd, "%s", strings.TrimSpace(reposStr))
	} else {
		for _, r := range repos {
			fmt.Printf("%s:\n\t%s\n\t%s\n", logging.Bold(fmt.Sprintf("[%s/%s]", r.Owner, r.RepoName)), r.Description, r.URL)
		}
	}
}
