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
	"path/filepath"
	"strings"

	"github.com/AlecAivazis/survey/v2"
	"github.com/cli/browser"
	"github.com/go-git/go-git/v5"
	"github.com/lekkodev/cli/pkg/apikey"
	"github.com/lekkodev/cli/pkg/dotlekko"
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
	var email, githubOrgName, githubRepo string
	var resume bool
	cmd := &cobra.Command{
		Use:   "setup",
		Short: "Setup Lekko for a new user",
		RunE: func(cmd *cobra.Command, args []string) error {
			rs := secrets.NewSecretsOrFail()
			bff := lekko.NewBFFClient(rs)
			auth := oauth.NewOAuth(bff)

			fmt.Printf("Welcome to Lekko!\n\n")
			if len(rs.GetLekkoUsername()) > 0 {
				fmt.Printf("Logged in as %s. Answer 'No' to log out and start over with a new account.\n", logging.Bold(rs.GetLekkoUsername()))
				if err := survey.AskOne(&survey.Confirm{
					Message: "Resume?",
					Default: true,
				}, &resume); err != nil {
					return errors.Wrap(err, "prompt resume")
				}
				if !resume {
					err := secrets.WithWriteSecrets(func(ws secrets.WriteSecrets) error {
						auth := oauth.NewOAuth(lekko.NewBFFClient(ws))
						for _, provider := range []string{"lekko", "github"} {
							if err := auth.Logout(cmd.Context(), provider, ws); err != nil {
								return err
							}
						}
						return nil
					})
					if err != nil {
						return errors.Wrap(err, "logout")
					}
				}
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
					return auth.PreRegister(cmd.Context(), email, ws)
				})
				if err != nil {
					return errors.Wrap(err, "pre-register")
				}
			}
			// login is no-op if already logged in
			err = secrets.WithWriteSecrets(func(ws secrets.WriteSecrets) error {
				auth := oauth.NewOAuth(lekko.NewBFFClient(ws))
				return auth.Login(cmd.Context(), ws)
			})
			if err != nil {
				return errors.Wrap(err, "login")
			}
			fmt.Println()

			dot, err := dotlekko.ReadDotLekko("")
			if err != nil {
				fmt.Println("Lekko is not detected in this project.")
				doInit := false
				if err := survey.AskOne(&survey.Confirm{
					Message: "Initialize Lekko?",
					Default: true,
				}, &doInit); err != nil {
					return errors.Wrap(err, "prompt init lekko")
				}
				if !doInit {
					return errors.New("Aborted!")
				}

				// naive check for "known" project types
				isGo := false
				isNode := false
				if _, err = os.Stat("go.mod"); err == nil {
					isGo = true
				} else if _, err = os.Stat("package.json"); err == nil {
					isNode = true
				}
				if !isGo && !isNode {
					return errors.New("Unknown project type, Lekko currently supports Go and NPM projects.")
				}

				lekkoPath := "lekko"
				if fi, err := os.Stat("src"); err == nil && fi.IsDir() && isNode {
					lekkoPath = "src/lekko"
				}
				if fi, err := os.Stat("internal"); err == nil && fi.IsDir() && isGo {
					lekkoPath = "internal/lekko"
				}

				if err := survey.AskOne(&survey.Input{
					Message: "Location for Lekko config functions (relative to project root):",
					Default: lekkoPath,
				}, &lekkoPath, survey.WithValidator(func(val interface{}) error {
					s, ok := val.(string)
					if !ok {
						return errors.New("invalid path")
					}
					if !strings.HasSuffix(s, "lekko") {
						return errors.New("path must end with 'lekko'")
					}
					return nil
				})); err != nil {
					return errors.Wrap(err, "prompt lekko path")
				}

				dot = dotlekko.NewDotLekko(lekkoPath)
			}

			rs = secrets.NewSecretsOrFail(secrets.RequireLekkoToken(), secrets.RequireGithub())
			bff = lekko.NewBFFClient(rs)
			ghCli := gh.NewGithubClientFromToken(cmd.Context(), rs.GetGithubToken())

			// if the user is already a member of a team and `githubRepo` repo exists under that team,
			// suggest to clone it
			repoExists := false
			if len(rs.GetLekkoTeam()) > 0 {
				repoCli := repo.NewRepoCmd(lekko.NewBFFClient(rs), rs)
				repos, err := repoCli.List(cmd.Context())
				if err != nil {
					return err
				}
				for _, repo := range repos {
					if repo.RepoName == githubRepo {
						repoExists = true
						break
					}
				}
			}

			githubRepoURL := fmt.Sprintf("https://github.com/%s/%s.git", rs.GetLekkoTeam(), githubRepo)

			useExistingRepo := repoExists
			if repoExists {
				fmt.Printf("Found existing repo: %s\n", githubRepoURL)
				if err := survey.AskOne(&survey.Confirm{
					Message: "Use it?",
					Default: true,
				}, &useExistingRepo); err != nil {
					return errors.Wrap(err, "prompt resume")
				}
			}

			if useExistingRepo {
				base, err := repo.DefaultRepoBasePath()
				if err != nil {
					return err
				}
				repoPath := filepath.Join(base, rs.GetLekkoTeam(), githubRepo)
				if _, err := os.Stat(repoPath); err == nil {
					gitRepo, err := git.PlainOpen(repoPath)
					if err != nil {
						return errors.Wrapf(err, "invalid git repo at %s", repoPath)
					}
					remote, err := gitRepo.Remote("origin")
					if err != nil {
						return errors.Wrapf(err, "invalid git repo at %s", repoPath)
					}
					// TODO: support comparing ssh and https urls
					if len(remote.Config().URLs) == 0 || remote.Config().URLs[0] != githubRepoURL {
						return errors.Errorf("repo already exists at %s but with different origin: %s", repoPath, remote.Config().URLs[0])
					}
					fmt.Printf("Repo already exists at %s\n\n", repoPath)
				} else {
					_, err := repo.NewLocalClone(repoPath, githubRepoURL, rs)
					if err != nil {
						return err
					}
					fmt.Printf("Cloned into %s\n\n", repoPath)
				}
				githubOrgName = rs.GetLekkoTeam()
			} else {
				for {
					if len(githubOrgName) > 0 {
						break
					}
					appInstalls, err := ghCli.GetAllUserInstallations(cmd.Context(), true)
					if err != nil {
						return err
					}
					var orgNames []string
					authorizeNewOrg := "[Authorize a new organization]"
					orgNames = append(orgNames, authorizeNewOrg, rs.GetGithubUser())
					installedOnPersonal := false
					for _, install := range appInstalls {
						if install.GetAccount().GetLogin() == rs.GetGithubUser() {
							installedOnPersonal = true
							continue
						}
						orgNames = append(orgNames, install.GetAccount().GetLogin())
					}
					if err := survey.AskOne(&survey.Select{
						Message: "Please select a GitHub organization to house a new config repo:",
						Options: orgNames,
						Description: func(value string, index int) string {
							if value == rs.GetGithubUser() {
								return "[personal account]"
							}
							return ""
						},
						Default: rs.GetGithubUser(),
					}, &githubOrgName); err != nil {
						return errors.Wrap(err, "prompt")
					}
					if githubOrgName == rs.GetGithubUser() && !installedOnPersonal {
						ghUser, err := ghCli.GetUser(cmd.Context())
						if err != nil {
							return errors.Wrap(err, "get user")
						}
						url := fmt.Sprintf("https://github.com/apps/lekko-app/installations/new/permissions?target_id=%d", ghUser.GetID())
						if err := browser.OpenURL(url); err != nil {
							return err
						}
						fmt.Printf("Press %s to continue", logging.Bold("[Enter]"))
						_ = waitForEnter(os.Stdin)
						continue
					}
					if githubOrgName == authorizeNewOrg {
						githubOrgName = ""
						url := "https://github.com/apps/lekko-app/installations/new"
						if err := browser.OpenURL(url); err != nil {
							return err
						}
						fmt.Printf("Press %s to refresh the list of organizations", logging.Bold("[Enter]"))
						_ = waitForEnter(os.Stdin)
						continue
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

				rs = secrets.NewSecretsOrFail(secrets.RequireLekko(), secrets.RequireGithub())
				bff = lekko.NewBFFClient(rs)

				base, err := repo.DefaultRepoBasePath()
				if err != nil {
					return err
				}
				repoPath := filepath.Join(base, githubOrgName, githubRepo)

				repoCmd := repo.NewRepoCmd(lekko.NewBFFClient(rs), rs)
				err = repoCmd.Import(cmd.Context(), repoPath, githubOrgName, githubRepo, "")
				if err != nil {
					return errors.Wrap(err, "import repo")
				}
			}

			dot.Repository = fmt.Sprintf("%s/%s", githubOrgName, githubRepo)
			if err := dot.WriteBack(); err != nil {
				return err
			}

			if !rs.HasLekkoAPIKey() {
				if err := secrets.WithWriteSecrets(func(ws secrets.WriteSecrets) error {
					resp, err := apikey.NewAPIKey(bff).Create(cmd.Context(), rs.GetLekkoTeam(), "")
					if err != nil {
						return err
					}
					fmt.Printf("Lekko API key:\n\t%s\n", logging.Bold(resp.GetApiKey()))
					fmt.Printf("Use %s command to copy the API key to your clipboard\n", logging.Bold("lekko apikey copy"))
					ws.SetLekkoAPIKey(resp.GetApiKey())
					return nil
				}, secrets.RequireLekko()); err != nil {
					return err
				}
			}

			fmt.Println("Lekko setup complete!")

			return nil
		},
	}
	cmd.Flags().StringVarP(&email, "email", "e", "", "email to create Lekko account with")
	cmd.Flags().StringVarP(&githubOrgName, "org", "o", "", "GitHub organization to house repository in")
	cmd.Flags().StringVarP(&githubRepo, "repo", "r", "lekko-configs", "GitHub repository name")
	cmd.Flags().BoolVar(&resume, "resume", false, "resume setup using currently authenticated user")
	return cmd
}
