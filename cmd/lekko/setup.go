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
	"fmt"
	"net/mail"
	"os"

	"github.com/AlecAivazis/survey/v2"
	"github.com/cli/browser"
	"github.com/lekkodev/cli/pkg/apikey"
	"github.com/lekkodev/cli/pkg/gh"
	"github.com/lekkodev/cli/pkg/lekko"
	"github.com/lekkodev/cli/pkg/logging"
	"github.com/lekkodev/cli/pkg/oauth"
	"github.com/lekkodev/cli/pkg/repo"
	"github.com/lekkodev/cli/pkg/secrets"
	"github.com/lekkodev/cli/pkg/team"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
)

func setupCmd() *cobra.Command {
	var repoPath, email, githubOrgName, githubRepo string
	var resume bool
	cmd := &cobra.Command{
		Use:   "setup",
		Short: "Setup Lekko for a new user",
		RunE: func(cmd *cobra.Command, args []string) error {
			rs := secrets.NewSecretsOrFail()
			bff := lekko.NewBFFClient(rs)
			auth := oauth.NewOAuth(bff)

			if len(rs.GetLekkoUsername()) > 0 && !resume {
				return fmt.Errorf("logged in as %s, please log out first (`lekko auth logout -p lekko`)", rs.GetLekkoUsername())
			}

			var err error

			if !resume {
				if len(email) == 0 {
					if err := survey.AskOne(&survey.Input{
						Message: "Email:",
					}, &email); err != nil {
						return errors.Wrap(err, "prompt email")
					}
				}
				if _, err := mail.ParseAddress(email); err != nil {
					return errors.New("invalid email address")
				}

				err = secrets.WithWriteSecrets(func(ws secrets.WriteSecrets) error {
					// Trigger pre-registration, wait for signup & device auth
					err := auth.PreRegister(cmd.Context(), email, ws)
					if err != nil {
						return err
					}

					auth := oauth.NewOAuth(lekko.NewBFFClient(ws))
					return auth.Login(cmd.Context(), ws)
				})
				if err != nil {
					return err
				}
			}

			rs = secrets.NewSecretsOrFail(secrets.RequireLekkoToken(), secrets.RequireGithub())
			bff = lekko.NewBFFClient(rs)

			ghCli := gh.NewGithubClientFromToken(cmd.Context(), rs.GetGithubToken())

			for {
				if len(githubOrgName) > 0 {
					break
				}
				orgs, err := ghCli.GetUserOrganizations(cmd.Context())
				if err != nil {
					return err
				}
				orgNames := make([]string, len(orgs)+1)
				authorizeNewOrg := "[Authorize a new organization]"
				orgNames[0] = authorizeNewOrg
				for i, org := range orgs {
					orgNames[i+1] = org.GetLogin()
				}
				if err := survey.AskOne(&survey.Select{
					Message: "Lekko uses a GitHub repository to store configs. Please select a GitHub organization to house a new config repo:",
					Options: orgNames,
				}, &githubOrgName); err != nil {
					return errors.Wrap(err, "prompt")
				}
				if githubOrgName == authorizeNewOrg {
					githubOrgName = ""
					url := "https://github.com/apps/lekko-app/installations/new"
					if err := browser.OpenURL(url); err != nil {
						return err
					}
					fmt.Printf("Press [Enter] to refresh the list of organizations")
					_ = waitForEnter(os.Stdin)
				}
			}
			if len(githubOrgName) == 0 {
				return errors.New("no github organization selected")
			}

			// to streamline the setup we always create a team with the same name as the github org
			t := team.NewTeam(bff)
			teams, err := t.List(cmd.Context())
			if err != nil {
				return err
			}
			teamExists := false
			for _, existingTeam := range teams {
				if existingTeam.TeamName == githubOrgName {
					teamExists = true
				}
			}
			if err := secrets.WithWriteSecrets(func(ws secrets.WriteSecrets) error {
				if teamExists {
					// TODO: send a request to join the team
					ws.SetLekkoTeam(githubOrgName)
					return nil
				}
				return t.Create(cmd.Context(), githubOrgName, ws)
			}, secrets.RequireLekkoToken()); err != nil {
				return err
			}

			if len(githubRepo) == 0 {
				if err := survey.AskOne(&survey.Input{
					Message: "Name of a new GitHub repository to create:",
				}, &githubRepo); err != nil {
					return errors.Wrap(err, "prompt")
				}
			}

			rs = secrets.NewSecretsOrFail(secrets.RequireLekko(), secrets.RequireGithub())
			bff = lekko.NewBFFClient(rs)

			repo := repo.NewRepoCmd(lekko.NewBFFClient(rs), rs)
			err = repo.Import(cmd.Context(), repoPath, githubOrgName, githubRepo, "")
			if err != nil {
				return errors.Wrap(err, "import repo")
			}
			err = secrets.WithWriteSecrets(func(ws secrets.WriteSecrets) error {
				ws.SetGithubOwner(githubOrgName)
				ws.SetGithubRepo(githubRepo)
				return nil
			})
			if err != nil {
				return err
			}

			if !rs.HasLekkoAPIKey() {
				if err := secrets.WithWriteSecrets(func(ws secrets.WriteSecrets) error {
					resp, err := apikey.NewAPIKey(bff).Create(cmd.Context(), rs.GetLekkoTeam(), "")
					if err != nil {
						return err
					}
					// TODO: consolidate with create api key command
					fmt.Printf("Generated api key named '%s':\n\t%s\n", resp.GetNickname(), logging.Bold(resp.GetApiKey()))
					ws.SetLekkoAPIKey(resp.GetApiKey())
					return nil
				}, secrets.RequireLekko()); err != nil {
					return err
				}
			}

			return nil
		},
	}
	cmd.Flags().StringVarP(&email, "email", "e", "", "email to create Lekko account with")
	cmd.Flags().StringVarP(&githubOrgName, "org", "o", "", "GitHub organization to house repository in")
	cmd.Flags().StringVarP(&githubRepo, "repo", "r", "", "GitHub repository name")
	cmd.Flags().StringVarP(&repoPath, "path", "p", "", "path to the repo location")
	cmd.Flags().BoolVar(&resume, "resume", false, "resume setup using currently authenticated user")
	return cmd
}
