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
	)
	return cmd
}

var repoListCmd = &cobra.Command{
	Use:   "list",
	Short: "List the config repositories in the currently active team",
	RunE: func(cmd *cobra.Command, args []string) error {
		rs := secrets.NewSecretsOrFail(secrets.RequireLekko())
		repo := repo.NewRepoCmd(lekko.NewBFFClient(rs))
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
	var owner, repoName string
	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create a new, empty config repository in the currently active team",
		RunE: func(cmd *cobra.Command, args []string) error {
			rs := secrets.NewSecretsOrFail(secrets.RequireLekko())
			if len(owner) == 0 {
				if err := survey.AskOne(&survey.Input{
					Message: "Github Owner:",
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
			fmt.Printf("Attempting to create a new configuration repository %s in team %s.\n", logging.Bold(fmt.Sprintf("[%s/%s]", owner, repoName)), rs.GetLekkoTeam())
			fmt.Printf("First, ensure that the github owner '%s' has installed Lekko App by visiting:\n\t%s\n", owner, lekkoAppInstallURL)
			fmt.Printf("Once done, press [Enter] to continue...")
			_ = waitForEnter(os.Stdin)

			repo := repo.NewRepoCmd(lekko.NewBFFClient(rs))
			url, err := repo.Create(cmd.Context(), owner, repoName)
			if err != nil {
				return err
			}
			fmt.Printf("Successfully created a new config repository at %s.\n", url)
			fmt.Printf("To make configuration changes, run `lekko repo clone`.")
			return nil
		},
	}
	cmd.Flags().StringVarP(&owner, "owner", "o", "", "Github owner to create the repository under. If empty, defaults to the authorized user's personal account.")
	cmd.Flags().StringVarP(&repoName, "repo", "r", "", "Github repository name")
	return cmd
}

func repoCloneCmd() *cobra.Command {
	var url string
	cmd := &cobra.Command{
		Use:   "clone",
		Short: "Clone an existing configuration repository to local disk",
		RunE: func(cmd *cobra.Command, args []string) error {
			wd, err := os.Getwd()
			if err != nil {
				return err
			}
			rs := secrets.NewSecretsOrFail(secrets.RequireLekko())
			r := repo.NewRepoCmd(lekko.NewBFFClient(rs))
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
			_, err = repo.NewLocalClone(path.Join(wd, repoName), url, rs)
			if err != nil {
				return errors.Wrap(err, "new local clone")
			}
			return nil
		},
	}
	cmd.Flags().StringVarP(&url, "url", "u", "", "url of GitHub-hosted configuration repository to clone")
	return cmd
}

func repoDeleteCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "delete",
		Short: "Delete an existing config repository",
		RunE: func(cmd *cobra.Command, args []string) error {
			rs := secrets.NewSecretsOrFail(secrets.RequireLekko())
			repo := repo.NewRepoCmd(lekko.NewBFFClient(rs))
			ctx := cmd.Context()
			repos, err := repo.List(ctx)
			if err != nil {
				return errors.Wrap(err, "repos list")
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
		Use:   "init",
		Short: "Initialize a new template git repository",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			if len(owner) == 0 || len(repoName) == 0 {
				return errors.Errorf("must provide owner and repo name")
			}
			rs := secrets.NewSecretsOrFail(secrets.RequireGithub())
			ghCli := gh.NewGithubClientFromToken(ctx, rs.GetGithubToken())
			if rs.GetGithubUser() == owner {
				owner = "" // create repo expects an empty owner for personal accounts
			}

			ghRepo, err := ghCli.CreateRepo(ctx, owner, repoName, description, true)
			if err != nil {
				return errors.Wrap(err, "create github repo")
			}
			if err := repo.MirrorAtURL(ctx, rs, ghRepo.GetCloneURL()); err != nil {
				return errors.Wrapf(err, "mirror at url: %s", ghRepo.GetCloneURL())
			}

			fmt.Printf("Mirrored repo at url %s\n", ghRepo.GetCloneURL())
			return nil
		},
	}
	cmd.Flags().StringVarP(&owner, "owner", "o", "", "GitHub owner to house repository in")
	cmd.Flags().StringVarP(&repoName, "repo", "r", "", "GitHub repository name")
	cmd.Flags().StringVarP(&description, "description", "d", "", "GitHub repository description")
	return cmd
}
