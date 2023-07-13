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
	"os"

	"github.com/lekkodev/cli/pkg/lekko"
	"github.com/lekkodev/cli/pkg/logging"
	"github.com/spf13/cobra"
)

const (
	AppName = "lekko"
)

// Updated at build time using ldflags
var version = "development"

func main() {
	rootCmd := rootCmd()
	rootCmd.AddCommand(compileCmd())
	rootCmd.AddCommand(verifyCmd())
	rootCmd.AddCommand(commitCmd())
	rootCmd.AddCommand(reviewCmd())
	rootCmd.AddCommand(mergeCmd())
	rootCmd.AddCommand(restoreCmd())
	rootCmd.AddCommand(teamCmd())
	rootCmd.AddCommand(repoCmd())
	rootCmd.AddCommand(featureCmd())
	rootCmd.AddCommand(namespaceCmd())
	rootCmd.AddCommand(apikeyCmd())
	rootCmd.AddCommand(upgradeCmd())
	rootCmd.AddCommand(authCmd())
	rootCmd.AddCommand(expCmd())
	rootCmd.AddCommand(generateCmd())

	if err := rootCmd.ExecuteContext(context.Background()); err != nil {
		printErr(rootCmd, err)
		os.Exit(1)
	} else {
		os.Exit(0)
	}
}

func rootCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:           "lekko",
		Short:         "lekko - dynamic configuration helper",
		Version:       version,
		SilenceUsage:  true,
		SilenceErrors: true,
		PersistentPreRun: func(cmd *cobra.Command, args []string) {
			logging.InitColors(IsInteractive)
			//-- checking passed pos args
			s := getUseCmdParams(cmd.UseLine(), cmd.Name())
			if err := errIfMoreArgs(append(s.PosArgs, s.PosOptionalArgs...), args); err != nil {
				printErr(cmd, err)
				os.Exit(1)
			}
			if !IsInteractive {
				if err := errIfLessArgs(s.PosArgs, args); err != nil {
					printErr(cmd, err)
					os.Exit(1)
				}
			}
		},
	}
	cmd.PersistentFlags().BoolVarP(&IsInteractive, InteractiveModeFlag, InteractiveModeFlagShort, InteractiveModeFlagDVal, InteractiveModeFlagDescription)
	cmd.CompletionOptions.DisableDefaultCmd = true
	cmd.PersistentFlags().StringVar(&lekko.URL, LekkoBackendFlag, LekkoBackendFlagURL, LekkoBackendFlagDescription)

	if IsLekkoBackendFlagHidden {
		if err := cmd.PersistentFlags().MarkHidden(LekkoBackendFlag); err != nil {
			printErrExit(cmd, err)
		}
	}
	return cmd
}
