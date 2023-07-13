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

func restoreCmd() *cobra.Command {
	var isQuiet, force bool
	cmd := &cobra.Command{
		Short:                 "Restores repo to a given hash",
		Use:                   formCmdUse("restore", "hash"),
		DisableFlagsInUseLine: true,
		Args:                  cobra.ExactArgs(1),
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
			if err := r.RestoreWorkingDirectory(args[0]); err != nil {
				return errors.Wrap(err, "restore wd")
			}
			ctx := cmd.Context()
			rootMD, _, err := r.ParseMetadata(ctx)
			if err != nil {
				return errors.Wrap(err, "parse metadata")
			}
			registry, err := r.ReBuildDynamicTypeRegistry(ctx, rootMD.ProtoDirectory, rootMD.UseExternalTypes)
			if err != nil {
				return errors.Wrap(err, "rebuild type registry")
			}
			if !isQuiet {
				printLinef(cmd, "Successfully rebuilt dynamic type registry.\n")
			}
			if _, err := r.Compile(ctx, &repo.CompileRequest{
				Registry:                     registry,
				DryRun:                       false,
				IgnoreBackwardsCompatibility: force,
			}); err != nil {
				return errors.Wrap(err, "compile")
			}
			if !isQuiet {
				printLinef(cmd, "Restored hash %s to your working directory. \nRun `lekko review` to create a PR with these changes.\n", args[0])
			} else {
				printLinef(cmd, "%s", args[0])
			}
			return nil
		},
	}
	cmd.Flags().BoolVarP(&isQuiet, QuietModeFlag, QuietModeFlagShort, QuietModeFlagDVal, QuietModeFlagDescription)
	cmd.Flags().BoolVarP(&force, "force", "f", false, "force compilation, ignoring validation check failures.")
	return cmd
}
