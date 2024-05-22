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
	"strings"

	"github.com/AlecAivazis/survey/v2"
	"github.com/go-git/go-git/v5"
	"github.com/lainio/err2"
	"github.com/lainio/err2/try"
	"github.com/lekkodev/cli/pkg/dotlekko"
	"github.com/lekkodev/cli/pkg/gen"
	"github.com/lekkodev/cli/pkg/native"
	"github.com/lekkodev/cli/pkg/repo"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
)

func initCmd() *cobra.Command {
	var lekkoPath, repoName string
	cmd := &cobra.Command{
		Use:   "init",
		Short: "initialize Lekko in your project",
		RunE: func(cmd *cobra.Command, args []string) (err error) {
			defer err2.Handle(&err)
			// TODO:
			// + create .lekko file
			// + generate from `default` namespace
			// - install lekko deps (depending on project type)
			// - setup github actions
			// - install linter
			_, err = dotlekko.ReadDotLekko("")
			if err == nil {
				fmt.Println("Lekko is already initialized in this project.")
				return nil
			}
			// TODO: print some info

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

			if lekkoPath == "" {
				lekkoPath = "lekko"
				if fi, err := os.Stat("src"); err == nil && fi.IsDir() && isNode {
					lekkoPath = "src/lekko"
				}
				if fi, err := os.Stat("internal"); err == nil && fi.IsDir() && isGo {
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
					Message: "Config repository name, for example `my-org/lekko-configs`:",
					Default: repoName,
				}, &repoName))
			}

			dot := dotlekko.NewDotLekko(lekkoPath, repoName)
			try.To(dot.WriteBack())
			// TODO: make sure that `default` namespace exists
			try.To(runGen(cmd.Context(), lekkoPath, "default"))

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
