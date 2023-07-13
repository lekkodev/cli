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
	"path"

	"github.com/AlecAivazis/survey/v2"
	"github.com/lekkodev/cli/pkg/gh"
	"github.com/lekkodev/cli/pkg/repo"
	"github.com/lekkodev/cli/pkg/secrets"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
)

func reviewCmd() *cobra.Command {
	var title, url string
	var isDryRun, isQuiet bool
	cmd := &cobra.Command{
		Short:                 "Creates a pr with your changes",
		Use:                   formCmdUse("review"),
		DisableFlagsInUseLine: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			wd, err := os.Getwd()
			if err != nil {
				return err
			}
			rs := secrets.NewSecretsOrFail(secrets.RequireGithub())
			r, err := repo.NewLocal(wd, rs)

			if isQuiet {
				r.ConfigureLogger(nil)
			}
			if err != nil {
				return errors.Wrap(err, "review")
			}

			if _, err := r.Verify(ctx, &repo.VerifyRequest{}); err != nil {
				return errors.Wrap(err, "verify")
			}
			ghCli := gh.NewGithubClientFromToken(ctx, rs.GetGithubToken())
			if _, err := ghCli.GetUser(ctx); err != nil {
				return errors.Wrap(err, "github auth fail")
			}

			if len(title) == 0 && IsInteractive {
				fmt.Printf("-------------------\n")
				if err := survey.AskOne(&survey.Input{
					Message: "Title:",
				}, &title); err != nil {
					return errors.Wrap(err, "prompt")
				}
			}

			if !isDryRun {
				url, err = r.Review(ctx, title, ghCli, rs)
				if isQuiet && err == nil {
					printLinef(cmd, "%s %s", path.Base(url), url)
				}
			}
			return err
		},
	}
	cmd.Flags().BoolVarP(&isQuiet, QuietModeFlag, QuietModeFlagShort, QuietModeFlagDVal, QuietModeFlagDescription)
	cmd.Flags().BoolVarP(&isDryRun, DryRunFlag, DryRunFlagShort, DryRunFlagDVal, DryRunFlagDescription)
	cmd.Flags().StringVarP(&title, "title", "t", "", "Title of pull request")
	return cmd
}
