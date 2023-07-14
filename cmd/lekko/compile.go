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
	"strings"

	"github.com/lekkodev/cli/pkg/feature"
	"github.com/lekkodev/cli/pkg/repo"
	"github.com/lekkodev/cli/pkg/secrets"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
)

const (
	UpgradeFlag      = "upgrade"
	UpgradeFlagShort = "u"
	UpgradeFlagDVal  = false
)

func compileCmd() *cobra.Command {
	var isDryRun, isQuiet, isForce, isUpgrade, isVerbose bool
	cmd := &cobra.Command{
		Short:                 "Compiles features based on individual definitions",
		Use:                   formCmdUse("compile", "[namespace[/feature]]"),
		DisableFlagsInUseLine: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			wd, err := os.Getwd()
			if err != nil {
				return err
			}
			rs := secrets.NewSecretsOrFail()

			r, err := repo.NewLocal(wd, rs)
			if err != nil {
				return err
			}
			if isQuiet {
				r.ConfigureLogger(nil)
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
			var ns, f string
			if len(args) > 0 {
				ns, f, err = feature.ParseFeaturePath(args[0])
				if err != nil {
					return err
				}
			}
			if result, err := r.Compile(ctx, &repo.CompileRequest{
				Registry:                     registry,
				NamespaceFilter:              ns,
				FeatureFilter:                f,
				DryRun:                       isDryRun,
				IgnoreBackwardsCompatibility: isForce,
				// don't verify file structure, since we may have not yet generated
				// the DSLs for newly added Flags().
				Verify:  false,
				Upgrade: isUpgrade,
				Verbose: isVerbose,
			}); err != nil {
				return errors.Wrap(err, "compile")
			} else {
				if !isQuiet {
					printLinef(cmd, "Compilation completed\n")
				} else {
					s := ""
					for _, r := range result {
						s += fmt.Sprintf("%s/%s ", r.NamespaceName, r.FeatureName)
					}
					printLinef(cmd, "%s", strings.TrimSpace(s))
				}
			}
			return nil
		},
	}
	cmd.Flags().BoolVarP(&isForce, ForceFlag, ForceFlagShort, ForceFlagDVal, "force compilation, ignoring validation check failures")
	cmd.Flags().BoolVarP(&isQuiet, QuietModeFlag, QuietModeFlagShort, QuietModeFlagDVal, QuietModeFlagDescription)
	cmd.Flags().BoolVarP(&isDryRun, DryRunFlag, DryRunFlagShort, DryRunFlagDVal, "skip persisting any newly compiled changes to disk")
	cmd.Flags().BoolVarP(&isUpgrade, UpgradeFlag, UpgradeFlagShort, UpgradeFlagDVal, "upgrade any of the requested namespaces that are behind the latest version")
	cmd.Flags().BoolVarP(&isVerbose, VerboseFlag, VerboseFlagShort, VerboseFlagDVal, "enable verbose error logging")
	return cmd
}
