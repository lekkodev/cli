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
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/AlecAivazis/survey/v2"
	"github.com/briandowns/spinner"
	"github.com/cli/browser"
	"github.com/lainio/err2"
	"github.com/lainio/err2/try"
	"github.com/lekkodev/cli/pkg/dotlekko"
	"github.com/lekkodev/cli/pkg/gen"
	"github.com/lekkodev/cli/pkg/lekko"
	"github.com/lekkodev/cli/pkg/logging"
	"github.com/lekkodev/cli/pkg/native"
	"github.com/lekkodev/cli/pkg/oauth"
	"github.com/lekkodev/cli/pkg/repo"
	"github.com/lekkodev/cli/pkg/secrets"
	"github.com/lekkodev/cli/pkg/team"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
)

func initCmd() *cobra.Command {
	var lekkoPath, fullRepoName string
	cmd := &cobra.Command{
		Use:   "init",
		Short: "initialize Lekko in your project",
		RunE: func(cmd *cobra.Command, args []string) (err error) {
			defer err2.Handle(&err)
			ctx := cmd.Context()
			successCheck := logging.Green("\u2713")
			if err != nil {
				return errors.Wrap(err, "get working directory")
			}
			spin := spinner.New(spinner.CharSets[14], 100*time.Millisecond)

			nlProject, err := native.DetectNativeLang("")
			if err != nil {
				return errors.Wrap(err, "detect project information")
			}
			// Output detected information
			fmt.Println("Detected the following project information:")
			fmt.Printf("- Language: %s\n", logging.Bold(nlProject.Language))
			if nlProject.PackageManager != native.PmUnknown {
				fmt.Printf("- Package manager: %s\n", logging.Bold(nlProject.PackageManager))
			}
			if len(nlProject.Frameworks) > 0 {
				fmt.Printf("- Frameworks: ")
				for i, fw := range nlProject.Frameworks {
					fmt.Printf("%s", logging.Bold(fw))
					if i < len(nlProject.Frameworks)-1 {
						fmt.Printf(", ")
					}
				}
				fmt.Printf("\n")
			}
			fmt.Println("")

			// If dotlekko already exists, ask if want to re-run; if yes, try use prevDot info for nicer defaults
			prevDot, err := dotlekko.ReadDotLekko("")
			if err == nil {
				rerun := false
				if err := survey.AskOne(&survey.Confirm{
					Message: "A Lekko configuration file (.lekko) already exists. Re-run initialization?",
					Default: rerun,
				}, &rerun); err != nil {
					return errors.Wrap(err, "confirm rerun")
				}
				if !rerun {
					return nil
				}
			}

			// Prompt user to log in, extract team and repo information
			var teamName string
			if err := secrets.WithWriteSecrets(func(ws secrets.WriteSecrets) error {
				auth := oauth.NewOAuth(lekko.NewBFFClient(ws))
				fmt.Println("Logging in to Lekko...")
				if err := auth.LoginLekko(ctx, ws); err != nil {
					return errors.Wrap(err, "login")
				}
				fmt.Printf("%s Successfully logged in as %s!\n\n", successCheck, logging.Bold(ws.GetLekkoUsername()))

				bff := lekko.NewBFFClient(ws)

				teamCmd := team.NewTeam(bff)
				memberships, err := teamCmd.List(ctx)
				if err != nil {
					return errors.Wrap(err, "list teams")
				}
				if len(memberships) == 0 {
					fmt.Printf("You are not a member of any Lekko team. Please finish onboarding at https://app.lekko.com to create or join a team.\n")
					os.Exit(1)
				}
				teamName = memberships[0].TeamName
				// If more than one team (e.g. tried onboarding on personal, invited to team) ask to choose
				if len(memberships) > 1 {
					teams := make([]string, len(memberships))
					for i, mem := range memberships {
						teams[i] = mem.TeamName
					}
					if err := survey.AskOne(&survey.Select{
						Message: "Choose Lekko team:",
						Help:    "You can always change the active team with `lekko team switch`.",
						Options: teams,
					}, &teamName); err != nil {
						return errors.Wrap(err, "choose Lekko team")
					}
					if err := teamCmd.Use(ctx, teamName, ws); err != nil {
						return errors.Wrapf(err, "use Lekko team %s", teamName)
					}
				}
				if fullRepoName == "" {
					repoCmd := repo.NewRepoCmd(bff, ws)
					repos, err := repoCmd.List(ctx)
					if err != nil {
						return errors.Wrap(err, "list repositories")
					}
					if len(repos) == 0 {
						fmt.Printf("No Lekko repositories found for team %s. Please finish onboarding at https://app.lekko.com to create a Lekko repository.\n", logging.Bold(teamName))
					}
					fullRepoName = fmt.Sprintf("%s/%s", repos[0].Owner, repos[0].RepoName)
					if len(repos) > 1 {
						repoNames := make([]string, len(repos))
						for i, repo := range repos {
							repoNames[i] = fmt.Sprintf("%s/%s", repo.Owner, repo.RepoName)
						}
						if err := survey.AskOne(&survey.Select{
							Message: "Choose Lekko repository:",
							Help:    fmt.Sprintf("These are machine-managed repositories owned by your Lekko team %s.", teamName),
							Options: repoNames,
						}, &fullRepoName); err != nil {
							return errors.Wrap(err, "choose Lekko repository")
						}
					}
				}
				return nil
			}); err != nil {
				return err
			}

			if lekkoPath == "" {
				lekkoPath = "lekko"
				if fi, err := os.Stat("src"); err == nil && fi.IsDir() {
					lekkoPath = "src/lekko"
				}
				if fi, err := os.Stat("internal"); err == nil && fi.IsDir() && nlProject.Language == native.LangGo {
					lekkoPath = "internal/lekko"
				}
				if prevDot != nil && prevDot.LekkoPath != "" {
					lekkoPath = prevDot.LekkoPath
				}
				try.To(survey.AskOne(&survey.Input{
					Message: "Location for Lekko files (relative to project root):",
					Default: lekkoPath,
					Help:    "You will write/manage dynamic functions written in files under this path.",
				}, &lekkoPath, survey.WithValidator(func(val interface{}) error {
					s, ok := val.(string)
					if !ok {
						return errors.New("invalid path")
					}
					// TODO: Remove this check if we remove the requirement from our codegen/transformer tools
					if !strings.HasSuffix(s, "lekko") {
						return errors.New("path must end with 'lekko'")
					}
					return nil
				})))
			}

			dot := dotlekko.NewDotLekko(lekkoPath, fullRepoName)
			try.To(dot.WriteBack())
			fmt.Printf("%s Successfully added %s.\n", successCheck, dot.GetPath())

			repoPath := try.To1(repo.PrepareGithubRepo())

			// Instructions for next steps that user should take, categorized by top level lib/feature/concept
			nextSteps := make(map[string][]string)
			nextSteps["API key"] = make([]string, 0)

			// Add GitHub workflow file
			var addWorkflow bool
			if err := survey.AskOne(&survey.Confirm{
				Message: "Add GitHub workflow file at .github/workflows/lekko.yaml?",
				Default: true,
				Help:    "This workflow will use the Lekko Push Action, which enables the automatic mirroring feature.",
			}, &addWorkflow); err != nil {
				return err
			}
			if addWorkflow {
				if err := os.MkdirAll(".github/workflows", os.ModePerm); err != nil {
					return errors.Wrap(err, "failed to mkdir .github/workflows")
				}
				workflowTemplate := getGitHubWorkflowTemplateBase()
				if suffix, err := getGitHubWorkflowTemplateSuffix(nlProject); err != nil {
					return err
				} else {
					workflowTemplate += suffix
				}
				if err := os.WriteFile(".github/workflows/lekko.yaml", []byte(workflowTemplate), 0600); err != nil {
					return errors.Wrap(err, "failed to write Lekko workflow file")
				}
				fmt.Printf("%s Successfully added .github/workflows/lekko.yaml.\n", successCheck)
				nextSteps["API key"] = append(nextSteps["API key"], fmt.Sprintf("Add %s as a secret in your GitHub repository/organization settings", logging.Bold("LEKKO_API_KEY")))
			}

			// Install Lekko-related project dependencies
			spin.Suffix = " Installing dependencies..."
			spin.Start()
			switch nlProject.Language {
			case native.LangGo:
				{
					// TODO: Try to install (and maybe setup) Go linter here
					goGetCmd := exec.Command("go", "get", "github.com/lekkodev/go-sdk@latest")
					if out, err := goGetCmd.CombinedOutput(); err != nil {
						spin.Stop()
						fmt.Println(goGetCmd.String())
						fmt.Println(string(out))
						return errors.Wrap(err, "failed to run go get")
					}
					spin.Stop()
					fmt.Printf("%s Successfully installed the Lekko Go SDK.\n", successCheck)
					nextSteps["Go SDK"] = append(nextSteps["Go SDK"], "See https://docs.lekko.com/sdks/go-sdk to get started")
					spin.Start()
				}
			case native.LangTypeScript:
				{
					if nlProject.HasFramework(native.FwVite) {
						// NOTE: Vite doesn't necessarily mean React but we assume for now
						var installArgs, installDevArgs []string
						switch nlProject.PackageManager {
						case native.PmNPM:
							{
								installArgs = []string{"install", "@lekko/react-sdk"}
								installDevArgs = []string{"install", "-D", "@lekko/vite-plugin", "@lekko/eslint-plugin"}
							}
						case native.PmYarn:
							{
								installArgs = []string{"add", "@lekko/react-sdk"}
								installDevArgs = []string{"add", "-D", "@lekko/vite-plugin", "@lekko/eslint-plugin"}
							}
						default:
							{
								return errors.Errorf("unsupported package manager %s", nlProject.PackageManager)
							}
						}
						installCmd := exec.Command(string(nlProject.PackageManager), installArgs...) // #nosec G204
						if out, err := installCmd.CombinedOutput(); err != nil {
							spin.Stop()
							fmt.Println(installCmd.String())
							fmt.Println(string(out))
							return errors.Wrap(err, "failed to run install deps command")
						}
						spin.Stop()
						fmt.Printf("%s Successfully installed @lekko/react-sdk.\n", successCheck)
						nextSteps["React SDK"] = append(nextSteps["React SDK"], "See https://docs.lekko.com/sdks/react-sdk to get started")
						spin.Start()
						installCmd = exec.Command(string(nlProject.PackageManager), installDevArgs...) // #nosec G204
						if out, err := installCmd.CombinedOutput(); err != nil {
							spin.Stop()
							fmt.Println(installCmd.String())
							fmt.Println(string(out))
							return errors.Wrap(err, "failed to run install dev deps command")
						}
						spin.Stop()
						fmt.Printf("%s Successfully installed @lekko/vite-plugin.\n", successCheck)
						nextSteps["Vite"] = append(nextSteps["Vite"], "See https://www.npmjs.com/package/@lekko/vite-plugin to configure the Lekko Vite plugin")
						fmt.Printf("%s Successfully installed @lekko/eslint-plugin.\n", successCheck)
						nextSteps["ESLint"] = append(nextSteps["ESLint"], "See https://www.npmjs.com/package/@lekko/eslint-plugin to configure the Lekko ESLint plugin")
						spin.Start()
					} else if nlProject.HasFramework(native.FwNext) {
						var installArgs, installDevArgs []string
						switch nlProject.PackageManager {
						case native.PmNPM:
							{
								installArgs = []string{"install", "@lekko/next-sdk"}
								installDevArgs = []string{"install", "-D", "@lekko/eslint-plugin"}
							}
						case native.PmYarn:
							{
								installArgs = []string{"add", "@lekko/next-sdk"}
								installDevArgs = []string{"add", "-D", "@lekko/eslint-plugin"}
							}
						default:
							{
								return errors.Errorf("unsupported package manager %s", nlProject.PackageManager)
							}
						}
						installCmd := exec.Command(string(nlProject.PackageManager), installArgs...) // #nosec G204
						if out, err := installCmd.CombinedOutput(); err != nil {
							spin.Stop()
							fmt.Println(installCmd.String())
							fmt.Println(string(out))
							return errors.Wrap(err, "failed to run install deps command")
						}
						spin.Stop()
						fmt.Printf("%s Successfully installed @lekko/next-sdk.\n", successCheck)
						nextSteps["Next.js SDK"] = append(nextSteps["Next.js SDK"], "See https://docs.lekko.com/sdks/next-sdk to get started")
						spin.Start()
						installCmd = exec.Command(string(nlProject.PackageManager), installDevArgs...) // #nosec G204
						if out, err := installCmd.CombinedOutput(); err != nil {
							spin.Stop()
							fmt.Println(installCmd.String())
							fmt.Println(string(out))
							return errors.Wrap(err, "failed to run install dev deps command")
						}
						spin.Stop()
						fmt.Printf("%s Successfully installed @lekko/eslint-plugin.\n", successCheck)
						nextSteps["ESLint"] = append(nextSteps["ESLint"], "See https://www.npmjs.com/package/@lekko/eslint-plugin to configure the Lekko ESLint plugin")
						spin.Start()
					} else if nlProject.HasFramework(native.FwNode) {
						var installArgs, installDevArgs []string
						switch nlProject.PackageManager {
						case native.PmNPM:
							{
								installArgs = []string{"install", "@lekko/js-sdk"}
								installDevArgs = []string{"install", "-D", "@lekko/ts-transformer", "@lekko/eslint-plugin"}
							}
						case native.PmYarn:
							{
								installArgs = []string{"add", "@lekko/js-sdk"}
								installDevArgs = []string{"add", "-D", "@lekko/ts-transformer", "@lekko/eslint-plugin"}
							}
						default:
							{
								return errors.Errorf("unsupported package manager %s", nlProject.PackageManager)
							}
						}
						installCmd := exec.Command(string(nlProject.PackageManager), installArgs...) // #nosec G204
						if out, err := installCmd.CombinedOutput(); err != nil {
							spin.Stop()
							fmt.Println(installCmd.String())
							fmt.Println(string(out))
							return errors.Wrap(err, "failed to run install deps command")
						}
						spin.Stop()
						fmt.Printf("%s Successfully installed @lekko/js-sdk.\n", successCheck)
						fmt.Printf("%s Successfully installed @lekko/ts-transformer.\n", successCheck)
						nextSteps["Node.js SDK"] = append(nextSteps["Node.js SDK"], "See https://docs.lekko.com/sdks/node-sdk to get started")
						spin.Start()
						installCmd = exec.Command(string(nlProject.PackageManager), installDevArgs...) // #nosec G204
						if out, err := installCmd.CombinedOutput(); err != nil {
							spin.Stop()
							fmt.Println(installCmd.String())
							fmt.Println(string(out))
							return errors.Wrap(err, "failed to run install dev deps command")
						}
						spin.Stop()
						fmt.Printf("%s Successfully installed @lekko/eslint-plugin.\n", successCheck)
						nextSteps["ESLint"] = append(nextSteps["ESLint"], "See https://www.npmjs.com/package/@lekko/eslint-plugin to configure the Lekko ESLint plugin")
						spin.Start()
					}
				}
			}
			spin.Stop()

			// Codegen
			spin.Suffix = " Running codegen..."
			spin.Start()
			repoOwner, repoName := dot.GetRepoInfo()
			if err := gen.GenNative(ctx, nlProject, lekkoPath, repoOwner, repoName, repoPath, gen.GenOptions{}); err != nil {
				return errors.Wrap(err, "codegen for default namespace")
			}
			spin.Stop()
			fmt.Printf("%s Successfully generated example code in %s.\n", successCheck, dot.LekkoPath)

			// Post-gen steps
			spin.Suffix = " Running post-codegen steps..."
			spin.Start()
			switch nlProject.Language {
			case native.LangGo:
				{
					// For Go we want to run `go mod tidy` - this handles transitive deps
					goTidyCmd := exec.Command("go", "mod", "tidy")
					if out, err := goTidyCmd.CombinedOutput(); err != nil {
						spin.Stop()
						fmt.Println(goTidyCmd.String())
						fmt.Println(string(out))
						return errors.Wrap(err, "failed to run go mod tidy")
					}
				}
			}
			spin.Stop()

			fmt.Printf("\n%s Complete! Your project is now set up to use Lekko.\n", successCheck)

			nextSteps["API key"] = append([]string{fmt.Sprintf("Go to https://app.lekko.com/teams/%s/admin?tab=APIKeys to generate an API key", teamName)}, nextSteps["API key"]...)

			// Output next steps/references
			var sb strings.Builder
			if len(nextSteps) > 0 {
				sb.WriteString("References:\n")
				sb.WriteString("-----------\n")
				for category, steps := range nextSteps {
					if len(steps) > 0 {
						sb.WriteString("* ")
						sb.WriteString(logging.Bold(category))
						sb.WriteString(":\n")
						for _, step := range steps {
							sb.WriteString("  - ")
							sb.WriteString(step)
							sb.WriteString("\n")
						}
					}
				}
				fmt.Printf("\n%s", sb.String())
			}

			// TODO: If possible, lekko-fy message and URL
			docURL := "https://docs.lekko.com/#add-lekko-build-decorators"
			fmt.Printf("\nPress %s to open the getting started documentation...", logging.Bold("[Enter]"))
			_ = waitForEnter(os.Stdin)
			if err := browser.OpenURL(docURL); err != nil {
				return errors.Wrapf(err, "failed to open browser at url %s", docURL)
			}

			return nil
		},
	}
	cmd.Flags().StringVarP(&lekkoPath, "lekko-path", "p", "", "Location for Lekko files (relative to project root)")
	cmd.Flags().StringVarP(&fullRepoName, "repo-name", "r", "", "Config repository name, for example `my-org/lekko-configs`")
	return cmd
}

func getGitHubWorkflowTemplateBase() string {
	// TODO: determine default branch name (might not be main)
	return `name: lekko
on:
  pull_request:
    branches:
      - main
  push:
    branches:
      - main
permissions:
  contents: read
jobs:
  push:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
`
}

func getGitHubWorkflowTemplateSuffix(nlProject *native.Project) (string, error) {
	// NOTE: Make sure to keep the indentation matched with base
	var ret string
	switch nlProject.Language {
	case native.LangGo:
		{
			ret = `      - uses: actions/setup-go@v5
        with:
          go-version-file: go.mod
`
		}
	case native.LangTypeScript:
		{
			ret = `      - uses: actions/setup-node@v4
        with:
          node-version: lts/Hydrogen
`
			switch nlProject.PackageManager {
			case native.PmNPM:
				{
					ret += `      - run: npm install
`
				}
			case native.PmYarn:
				{
					ret += `          cache: yarn
      - run: yarn install
`
				}
			default:
				return "", errors.New("unsupported package manager for GitHub workflow setup")
			}
		}
	// TODO: For TS projects need to detect package manager
	default:
		{
			return "", errors.New("unsupported framework for GitHub workflow setup")
		}
	}
	ret += `      - uses: lekkodev/push-action@v1
        with:
          api_key: ${{ secrets.LEKKO_API_KEY }}
`
	return ret, nil
}
