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
	"os"

	"github.com/lekkodev/cli/pkg/repo"
	"github.com/lekkodev/cli/pkg/secrets"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
)

func commitCmd() *cobra.Command {
	var message, hash string
	var isQuiet, isVerify bool
	cmd := &cobra.Command{
		Short:                 "Commits local changes to the remote branch",
		Use:                   formCmdUse("commit"),
		DisableFlagsInUseLine: true,
		//Use:   "commit" + FlagOptions,
		RunE: func(cmd *cobra.Command, args []string) error {
			wd, err := os.Getwd()
			if err != nil {
				return err
			}
			rs := secrets.NewSecretsOrFail(secrets.RequireGithub())
			r, err := repo.NewLocal(wd, rs)
			if err != nil {
				return errors.Wrap(err, "new repo")
			}
			ctx := cmd.Context()

			if isQuiet {
				r.ConfigureLogger(nil)
			}

			if isVerify {
				if _, err := r.Verify(ctx, &repo.VerifyRequest{}); err != nil {
					return errors.Wrap(err, "verify")
				}
			}

			if hash, err = r.Commit(ctx, rs, message); err != nil {
				return err
			}
			if isQuiet {
				printLinef(cmd, "%s", hash)
			}
			return nil
		},
	}
	cmd.Flags().BoolVarP(&isQuiet, QuietModeFlag, QuietModeFlagShort, QuietModeFlagDVal, QuietModeFlagDescription)
	cmd.Flags().BoolVarP(&isVerify, "verify", "v", false, "verify changes before committing")
	cmd.Flags().StringVarP(&message, "message", "m", "config change commit", "commit message")
	return cmd
}
