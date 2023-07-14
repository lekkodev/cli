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

func verifyCmd() *cobra.Command {
	var isQuiet bool
	cmd := &cobra.Command{
		Short:                 "Verifies features based on individual definitions",
		Use:                   formCmdUse("verify", "[namespace[/feature]]"),
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

			if featureCompRes, err := r.Verify(ctx, &repo.VerifyRequest{
				Registry:        registry,
				NamespaceFilter: ns,
				FeatureFilter:   f,
			}); err != nil {
				return err
			} else if isQuiet {
				s := ""
				for _, fcr := range featureCompRes {
					s += fmt.Sprintf("%s/%s ", fcr.NamespaceName, fcr.FeatureName)
				}
				printLinef(cmd, "%s", strings.TrimSpace(s))
			}
			return nil
		},
	}
	cmd.Flags().BoolVarP(&isQuiet, QuietModeFlag, QuietModeFlagShort, QuietModeFlagDVal, QuietModeFlagDescription)
	return cmd
}
