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
	"strconv"

	bffv1beta1 "buf.build/gen/go/lekkodev/cli/protocolbuffers/go/lekko/bff/v1beta1"
	"github.com/bufbuild/connect-go"
	"github.com/lekkodev/cli/pkg/gh"
	"github.com/lekkodev/cli/pkg/lekko"
	"github.com/lekkodev/cli/pkg/repo"
	"github.com/lekkodev/cli/pkg/secrets"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
)

func mergeCmd() *cobra.Command {
	var isQuiet bool
	cmd := &cobra.Command{
		Short:                 "Merges a pr for the current branch",
		Use:                   formCmdUse("merge", "[pr-number]"),
		DisableFlagsInUseLine: true,
		Args:                  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
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
				return errors.Wrap(err, "new repo")
			}
			ctx := cmd.Context()
			if _, err := r.Verify(ctx, &repo.VerifyRequest{}); err != nil {
				return errors.Wrap(err, "verify")
			}
			var prNum *int
			if len(args) > 0 {
				num, err := strconv.Atoi(args[0])
				if err != nil {
					return errors.Wrap(err, "pr-number arg")
				}
				prNum = &num
			}
			ghCli := gh.NewGithubClientFromToken(ctx, rs.GetGithubToken())
			if _, err := ghCli.GetUser(ctx); err != nil {
				return errors.Wrap(err, "github auth fail")
			}
			var prNumRet int
			if prNumRet, err = r.Merge(ctx, prNum, ghCli, rs); err != nil {
				return errors.Wrap(err, "merge")
			}
			if !isQuiet {
				printLinef(cmd, "%d PR merged.\n", *prNum)
				if len(rs.GetLekkoTeam()) > 0 {
					u, err := r.GetRemoteURL()
					if err != nil {
						return errors.Wrap(err, "get remote url")
					}
					owner, repo, err := gh.ParseOwnerRepo(u)
					if err != nil {
						return errors.Wrap(err, "parse owner repo")
					}
					repos, err := lekko.NewBFFClient(rs).ListRepositories(ctx, connect.NewRequest(&bffv1beta1.ListRepositoriesRequest{}))
					if err != nil {
						return errors.Wrap(err, "repository fetch failed")
					}
					defaultBranch := ""
					for _, r := range repos.Msg.GetRepositories() {
						if r.OwnerName == owner && r.RepoName == repo {
							defaultBranch = r.BranchName
						}
					}
					if len(defaultBranch) == 0 {
						return errors.New("repository not found when rolling out")
					}
					printLinef(cmd, "Visit %s to monitor your rollout.\n", rolloutsURL(rs.GetLekkoTeam(), owner, repo, defaultBranch))
				}
			} else {
				printLinef(cmd, "%d", prNumRet)
			}
			return nil
		},
	}
	cmd.Flags().BoolVarP(&isQuiet, QuietModeFlag, QuietModeFlagShort, QuietModeFlagDVal, QuietModeFlagDescription)
	return cmd
}

func rolloutsURL(team, owner, repo, branch string) string {
	return fmt.Sprintf("https://app.lekko.com/teams/%s/repositories/%s/%s/branches/%s/commits", team, owner, repo, branch)
}
