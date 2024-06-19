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
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/AlecAivazis/survey/v2"
	"github.com/briandowns/spinner"
	"github.com/go-git/go-git/v5"
	"github.com/lainio/err2"
	"github.com/lainio/err2/try"
	"github.com/lekkodev/cli/pkg/dotlekko"
	"github.com/lekkodev/cli/pkg/gen"
	"github.com/lekkodev/cli/pkg/logging"
	"github.com/lekkodev/cli/pkg/native"
	"github.com/lekkodev/cli/pkg/repo"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
)

type projectFramework int

const (
	pfUnknown projectFramework = iota
	pfGo
	pfNode
	pfReact
	pfVite
	pfNext
)

type packageManager string

const (
	pmUnknown packageManager = ""
	pmNPM     packageManager = "npm"
	pmYarn    packageManager = "yarn"
)

func initCmd() *cobra.Command {
	var lekkoPath, repoName string
	cmd := &cobra.Command{
		Use:   "init",
		Short: "initialize Lekko in your project",
		RunE: func(cmd *cobra.Command, args []string) (err error) {
			defer err2.Handle(&err)
			successCheck := logging.Green("\u2713")
			spin := spinner.New(spinner.CharSets[14], 100*time.Millisecond)
			// TODO:
			// + create .lekko file
			// + generate from `default` namespace
			// + install lekko deps (depending on project type)
			// + setup github actions
			// - install linter
			_, err = dotlekko.ReadDotLekko("")
			if err == nil {
				fmt.Println("Lekko is already initialized in this project.")
				return nil
			}
			// TODO: print some info

			// naive check for "known" project types
			// TODO: Consolidate into DetectNativeLang
			pf := pfUnknown
			pm := pmUnknown
			if _, err = os.Stat("go.mod"); err == nil {
				pf = pfGo
			} else if _, err = os.Stat("package.json"); err == nil {
				pf = pfNode
				pjBytes, err := os.ReadFile("package.json")
				if err != nil {
					return errors.Wrap(err, "failed to open package.json")
				}
				pjString := string(pjBytes)
				if strings.Contains(pjString, "react-dom") {
					pf = pfReact
				}
				// Vite config file could be js, cjs, mjs, etc.
				if matches, err := filepath.Glob("vite.config.*"); matches != nil && err == nil {
					pf = pfVite
				}
				// Next config file could be js, cjs, mjs, etc.
				if matches, err := filepath.Glob("next.config.*"); matches != nil && err == nil {
					pf = pfNext
				}

				pm = pmNPM
				if _, err := os.Stat("yarn.lock"); err == nil {
					pm = pmYarn
				}
			}
			if pf == pfUnknown {
				return errors.New("Unknown project type, Lekko currently supports Go and NPM projects.")
			}

			if lekkoPath == "" {
				lekkoPath = "lekko"
				if fi, err := os.Stat("src"); err == nil && fi.IsDir() {
					lekkoPath = "src/lekko"
				}
				if fi, err := os.Stat("internal"); err == nil && fi.IsDir() && pf == pfGo {
					lekkoPath = "internal/lekko"
				}
				try.To(survey.AskOne(&survey.Input{
					Message: "Location for Lekko files (relative to project root):",
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
				})))
			}

			if repoName == "" {
				// try to use owner of the current repo
				owner := ""
				if gitRepo, err := git.PlainOpen("."); err == nil {
					if remote, err := gitRepo.Remote("origin"); err == nil && len(remote.Config().URLs) > 0 {
						url := remote.Config().URLs[0]
						currentRepoName := ""
						if strings.HasPrefix(url, "https://github.com/") {
							currentRepoName = strings.TrimPrefix(url, "https://github.com/")
						} else if strings.HasPrefix(url, "git@github.com:") {
							currentRepoName = strings.TrimPrefix(url, "git@github.com:")
						}
						parts := strings.Split(currentRepoName, "/")
						if len(parts) == 2 {
							owner = strings.Split(currentRepoName, "/")[0]
						}
					}
				}
				if owner != "" {
					repoName = fmt.Sprintf("%s/lekko-configs", owner)
				}
				try.To(survey.AskOne(&survey.Input{
					Message: "Lekko repository name, for example `my-org/lekko-configs`:",
					Default: repoName,
					Help:    "If you set up your team on app.lekko.com, you can find your Lekko repository by logging in.",
				}, &repoName))
			}

			dot := dotlekko.NewDotLekko(lekkoPath, repoName)
			try.To(dot.WriteBack())

			// Add GitHub workflow file
			var addWorkflow bool
			if err := survey.AskOne(&survey.Confirm{
				Message: "Add GitHub workflow file at .github/workflows/lekko.yaml?",
				Default: true,
				Help:    "This workflow will use the Lekko Push Action, which enables the automatic mirrorring feature.",
			}, &addWorkflow); err != nil {
				return err
			}
			if addWorkflow {
				if err := os.MkdirAll(".github/workflows", os.ModePerm); err != nil {
					return errors.Wrap(err, "failed to mkdir .github/workflows")
				}
				workflowTemplate := getGitHubWorkflowTemplateBase()
				if suffix, err := getGitHubWorkflowTemplateSuffix(pf, pm); err != nil {
					return err
				} else {
					workflowTemplate += suffix
				}
				if err := os.WriteFile(".github/workflows/lekko.yaml", []byte(workflowTemplate), 0600); err != nil {
					return errors.Wrap(err, "failed to write Lekko workflow file")
				}
				// TODO: Consider moving instructions to end?
				fmt.Printf("%s Successfully added .github/workflows/lekko.yaml, please make sure to add LEKKO_API_KEY as a secret in your GitHub repository/org settings.\n", successCheck)
			}

			// TODO: Install deps depending on project type
			// TODO: Determine package manager (npm/yarn/pnpm/etc.) for ts projects
			spin.Suffix = " Installing dependencies..."
			spin.Start()
			switch pf {
			case pfGo:
				{
					goGetCmd := exec.Command("go", "get", "github.com/lekkodev/go-sdk@latest")
					if out, err := goGetCmd.CombinedOutput(); err != nil {
						spin.Stop()
						fmt.Println(goGetCmd.String())
						fmt.Println(string(out))
						return errors.Wrap(err, "failed to run go get")
					}
					spin.Stop()
					fmt.Printf("%s Successfully installed Lekko Go SDK.\n", successCheck)
					spin.Start()
				}
			case pfVite:
				// NOTE: Vite doesn't necessarily mean React but we assume for now
				{
					var installArgs, installDevArgs []string
					switch pm {
					case pmNPM:
						{
							installArgs = []string{"install", "@lekko/react-sdk"}
							installDevArgs = []string{"install", "-D", "@lekko/vite-plugin", "@lekko/eslint-plugin"}
						}
					case pmYarn:
						{
							installArgs = []string{"add", "@lekko/react-sdk"}
							installDevArgs = []string{"add", "-D", "@lekko/vite-plugin", "@lekko/eslint-plugin"}
						}
					default:
						{
							return errors.Errorf("unsupported package manager %s", pm)
						}
					}
					installCmd := exec.Command(string(pm), installArgs...) // #nosec G204
					if out, err := installCmd.CombinedOutput(); err != nil {
						spin.Stop()
						fmt.Println(installCmd.String())
						fmt.Println(string(out))
						return errors.Wrap(err, "failed to run install deps command")
					}
					spin.Stop()
					fmt.Printf("%s Successfully installed @lekko/react-sdk.\n", successCheck)
					spin.Start()
					installCmd = exec.Command(string(pm), installDevArgs...) // #nosec G204
					if out, err := installCmd.CombinedOutput(); err != nil {
						spin.Stop()
						fmt.Println(installCmd.String())
						fmt.Println(string(out))
						return errors.Wrap(err, "failed to run install dev deps command")
					}
					spin.Stop()
					fmt.Printf("%s Successfully installed @lekko/vite-plugin and @lekko/eslint-plugin. See the docs to configure these plugins.\n", successCheck)
					spin.Start()
				}
			case pfNext:
				{
					var installArgs, installDevArgs []string
					switch pm {
					case pmNPM:
						{
							installArgs = []string{"install", "@lekko/next-sdk"}
							installDevArgs = []string{"install", "-D", "@lekko/eslint-plugin"}
						}
					case pmYarn:
						{
							installArgs = []string{"add", "@lekko/next-sdk"}
							installDevArgs = []string{"add", "-D", "@lekko/eslint-plugin"}
						}
					default:
						{
							return errors.Errorf("unsupported package manager %s", pm)
						}
					}
					installCmd := exec.Command(string(pm), installArgs...) // #nosec G204
					if out, err := installCmd.CombinedOutput(); err != nil {
						spin.Stop()
						fmt.Println(installCmd.String())
						fmt.Println(string(out))
						return errors.Wrap(err, "failed to run install deps command")
					}
					spin.Stop()
					fmt.Printf("%s Successfully installed @lekko/next-sdk. See the docs to configure the SDK.\n", successCheck)
					spin.Start()
					installCmd = exec.Command(string(pm), installDevArgs...) // #nosec G204
					if out, err := installCmd.CombinedOutput(); err != nil {
						spin.Stop()
						fmt.Println(installCmd.String())
						fmt.Println(string(out))
						return errors.Wrap(err, "failed to run install dev deps command")
					}
					spin.Stop()
					fmt.Printf("%s Successfully installed @lekko/eslint-plugin. See the docs to configure this plugin.\n", successCheck)
					spin.Start()
				}
			}
			spin.Stop()

			// Codegen
			spin.Suffix = " Running codegen..."
			spin.Start()
			// TODO: make sure that `default` namespace exists
			try.To(runGen(cmd.Context(), lekkoPath, "default"))
			spin.Stop()

			// Post-gen steps
			spin.Suffix = " Running post-codegen steps..."
			spin.Start()
			switch pf {
			case pfGo:
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

			fmt.Printf("%s Complete! Your project is now set up to use Lekko.\n", successCheck)
			return nil
		},
	}
	cmd.Flags().StringVarP(&lekkoPath, "lekko-path", "p", "", "Location for Lekko files (relative to project root)")
	cmd.Flags().StringVarP(&repoName, "repo-name", "r", "", "Config repository name, for example `my-org/lekko-configs`")
	return cmd
}

func runGen(ctx context.Context, lekkoPath, ns string) (err error) {
	defer err2.Handle(&err)
	meta, nativeLang := try.To2(native.DetectNativeLang(""))
	repoPath := try.To1(repo.PrepareGithubRepo())
	return gen.GenNative(ctx, nativeLang, lekkoPath, repoPath, gen.GenOptions{
		NativeMetadata: meta,
		Namespaces:     []string{ns},
	})
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
  build:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
`
}

func getGitHubWorkflowTemplateSuffix(pf projectFramework, pm packageManager) (string, error) {
	// NOTE: Make sure to keep the indentation matched with base
	var ret string
	switch pf {
	case pfGo:
		{
			ret = `      - uses: actions/setup-go@v5
        with:
          go-version-file: go.mod
`
		}
	case pfNode:
		fallthrough
	case pfReact:
		fallthrough
	case pfVite:
		fallthrough
	case pfNext:
		{
			ret = `      - uses: actions/setup-node@v4
        with:
          node-version: lts/Hydrogen
`
			switch pm {
			case pmNPM:
				{
					ret += `      - run: npm install
`
				}
			case pmYarn:
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
