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

	"github.com/AlecAivazis/survey/v2"
	"github.com/go-git/go-git/v5"
	"github.com/lekkodev/cli/pkg/apikey"
	"github.com/lekkodev/cli/pkg/lekko"
	"github.com/lekkodev/cli/pkg/oauth"
	"github.com/lekkodev/cli/pkg/repo"
	"github.com/lekkodev/cli/pkg/secrets"
	"github.com/lekkodev/cli/pkg/team"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
)

func setupCmd() *cobra.Command {
	var repoPath, teamName, email, githubOwner, githubRepo string
	cmd := &cobra.Command{
		Use:   "setup",
		Short: "Setup Lekko for a new user",
		RunE: func(cmd *cobra.Command, args []string) error {
			rs := secrets.NewSecretsOrFail()
			bff := lekko.NewBFFClient(rs)
			auth := oauth.NewOAuth(bff)

			if len(rs.GetLekkoUsername()) > 0 {
				return fmt.Errorf("logged in as %s, please log out first (`lekko auth logout -p lekko`)", rs.GetLekkoUsername())
			}

			var err error

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

			// Trigger pre-registration, wait for signup & oauth
			creds, err := auth.PreRegister(cmd.Context(), email)
			if err != nil {
				return err
			}
			fmt.Printf("Sign-up complete! You are now logged into Lekko as %s.\n", email)

			err = secrets.WithWriteSecrets(func(ws secrets.WriteSecrets) error {
				ws.SetLekkoToken(creds.Token)
				auth := oauth.NewOAuth(lekko.NewBFFClient(ws))
				return auth.Login(cmd.Context(), ws)
			})
			if err != nil {
				return err
			}
			rs = secrets.NewSecretsOrFail(secrets.RequireLekkoToken())
			bff = lekko.NewBFFClient(rs)

			if _, err := git.PlainOpen("."); err == nil {
				// git repo detected
				if len(githubOwner) == 0 {
					if err := survey.AskOne(&survey.Input{
						Message: "GitHub Owner:",
					}, &githubOwner); err != nil {
						return errors.Wrap(err, "prompt")
					}
				}
				if len(githubRepo) == 0 {
					if err := survey.AskOne(&survey.Input{
						Message: "GitHub Repo:",
					}, &githubRepo); err != nil {
						return errors.Wrap(err, "prompt")
					}
				}
			}

			if len(teamName) == 0 {
				if err := survey.AskOne(&survey.Input{
					Message: "Team Name:",
					Default: githubOwner,
				}, &teamName); err != nil {
					return errors.Wrap(err, "prompt")
				}
			}
			t := team.NewTeam(bff)
			teams, err := t.List(cmd.Context())
			if err != nil {
				return err
			}
			teamExists := false
			for _, existingTeam := range teams {
				if existingTeam.TeamName == teamName {
					teamExists = true
				}
			}
			if err := secrets.WithWriteSecrets(func(ws secrets.WriteSecrets) error {
				if teamExists {
					ws.SetLekkoTeam(teamName)
					return nil
				}
				return t.Create(cmd.Context(), teamName, ws)
			}, secrets.RequireLekkoToken()); err != nil {
				return err
			}

			rs = secrets.NewSecretsOrFail(secrets.RequireLekko(), secrets.RequireGithub())
			bff = lekko.NewBFFClient(rs)

			if !rs.HasLekkoAPIKey() {
				if err := secrets.WithWriteSecrets(func(ws secrets.WriteSecrets) error {
					resp, err := apikey.NewAPIKey(bff).Create(cmd.Context(), rs.GetLekkoTeam(), "")
					if err != nil {
						return err
					}
					fmt.Printf("Generated api key named '%s'\n", resp.GetNickname())
					ws.SetLekkoAPIKey(resp.GetApiKey())
					return nil
				}, secrets.RequireLekko()); err != nil {
					return err
				}
			}

			if len(githubRepo) > 0 {
				repo := repo.NewRepoCmd(lekko.NewBFFClient(rs), rs)
				err = repo.Import(cmd.Context(), repoPath, githubOwner, githubRepo, "")
				if err != nil {
					return errors.Wrap(err, "import repo")
				}
				err = secrets.WithWriteSecrets(func(ws secrets.WriteSecrets) error {
					ws.SetGithubOwner(githubOwner)
					ws.SetGithubRepo(githubRepo)
					return nil
				})
				if err != nil {
					return err
				}
			}

			return nil
		},
	}
	cmd.Flags().StringVarP(&email, "email", "e", "", "email to create Lekko account with")
	cmd.Flags().StringVarP(&teamName, "team", "t", "", "name of team to create")
	cmd.Flags().StringVarP(&githubOwner, "owner", "o", "", "GitHub owner to house repository in")
	cmd.Flags().StringVarP(&githubRepo, "repo", "r", "", "GitHub repository name")
	cmd.Flags().StringVarP(&repoPath, "path", "p", "", "path to the repo location")
	return cmd
}
