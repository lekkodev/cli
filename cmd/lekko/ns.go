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
	"text/tabwriter"

	"github.com/AlecAivazis/survey/v2"
	"github.com/lekkodev/cli/pkg/metadata"
	"github.com/lekkodev/cli/pkg/repo"
	"github.com/lekkodev/cli/pkg/secrets"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
)

const (
	NsAddCmd = "add"
	NsRmCmd  = "remove"
	NsLsCmd  = "list"
)

func namespaceCmd() *cobra.Command {
	cmd := &cobra.Command{
		Short: "Namespace management",
		Use:   "ns command" + FlagOptions + "[args]",
	}
	cmd.AddCommand(
		nsListCmd(),
		nsAddCmd(),
		nsRemoveCmd(),
	)
	return cmd
}

func nsListCmd() *cobra.Command {
	cmd := &cobra.Command{
		Short:                 "List namespaces in the current repository",
		Use:                   formCmdUse(NsLsCmd, FlagOptions),
		DisableFlagsInUseLine: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			wd, err := os.Getwd()
			if err != nil {
				return err
			}
			r, err := repo.NewLocal(wd, secrets.NewSecretsOrFail())
			if err != nil {
				return err
			}
			nss, err := r.ListNamespaces(cmd.Context())
			if err != nil {
				return err
			}
			nsPrint(cmd, nss)
			return nil
		},
	}
	cmd.Flags().BoolP(QuietModeFlag, QuietModeFlagShort, QuietModeFlagDVal, QuietModeFlagDescription)
	return cmd
}

func nsAddCmd() *cobra.Command {
	var isDryRun, isQuiet bool
	var name string
	cmd := &cobra.Command{
		Short:                 "Add namespace",
		Use:                   formCmdUse(NsAddCmd, "namespace"),
		DisableFlagsInUseLine: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			wd, err := os.Getwd()
			if err != nil {
				return err
			}
			r, err := repo.NewLocal(wd, secrets.NewSecretsOrFail())
			if err != nil {
				return err
			}
			rArgs, n := getNArgs(1, args)
			if n != 1 && !IsInteractive {
				return errors.New("Namespace name is required.\n")
			}
			name = rArgs[0]

			if len(name) == 0 {
				if err := survey.AskOne(&survey.Input{
					Message: "Namespace name:",
				}, &name); err != nil {
					return errors.Wrap(err, "prompt")
				}
			}

			if !isDryRun {
				if err := r.AddNamespace(cmd.Context(), name); err != nil {
					return errors.Wrap(err, "add namespace")
				}
			}

			if !isQuiet {
				printLinef(cmd, "Added namespace %s\n", name)
			} else {
				printLinef(cmd, "%s", name)
			}

			return nil
		},
	}
	cmd.Flags().BoolVarP(&isDryRun, DryRunFlag, DryRunFlagShort, DryRunFlagDVal, DryRunFlagDescription)
	cmd.Flags().BoolVarP(&isQuiet, QuietModeFlag, QuietModeFlagShort, QuietModeFlagDVal, QuietModeFlagDescription)
	return cmd
}

func nsRemoveCmd() *cobra.Command {
	var isDryRun, isQuiet, isForce bool
	var name string
	cmd := &cobra.Command{
		Short:                 "Remove namespace",
		Use:                   formCmdUse(NsRmCmd, "namespace"),
		DisableFlagsInUseLine: true,
		//		Use:   NsRmCmd + FlagOptions + "namespace",
		RunE: func(cmd *cobra.Command, args []string) error {
			wd, err := os.Getwd()
			if err != nil {
				return err
			}
			r, err := repo.NewLocal(wd, secrets.NewSecretsOrFail())
			if err != nil {
				return err
			}
			rArgs, n := getNArgs(1, args)
			if n != 1 && !IsInteractive {
				return errors.New("Namespace name is required.\n")
			}
			name = rArgs[0]

			nss, err := r.ListNamespaces(cmd.Context())
			if err != nil {
				return err
			}
			if len(name) == 0 {
				var options []string
				for _, ns := range nss {
					options = append(options, ns.Name)
				}
				if err := survey.AskOne(&survey.Select{
					Message: "Select namespace to remove:",
					Options: options,
				}, &name); err != nil {
					return errors.Wrap(err, "prompt")
				}
			} else {
				// let's verify that the input namespace actually exists
				var exists bool
				for _, ns := range nss {
					if name == ns.Name {
						exists = true
						break
					}
				}
				if !exists {
					return errors.Errorf("Namespace %s does not exist", name)
				}
			}

			if !isForce {
				fmt.Printf("Deleting namespace %s...\n", name)
				if err := confirmInput(name); err != nil {
					return err
				}
			}

			if !isDryRun {
				if err := r.RemoveNamespace(cmd.Context(), name); err != nil {
					return errors.Wrap(err, "remove namespace")
				}
			}
			if !isQuiet {
				printLinef(cmd, "Deleted namespace %s\n", name)
			} else {
				printLinef(cmd, "%s", name)
			}
			return nil
		},
	}
	cmd.Flags().BoolVarP(&isQuiet, QuietModeFlag, QuietModeFlagShort, QuietModeFlagDVal, QuietModeFlagDescription)
	cmd.Flags().BoolVarP(&isDryRun, DryRunFlag, DryRunFlagShort, DryRunFlagDVal, DryRunFlagDescription)
	cmd.Flags().BoolVarP(&isForce, ForceFlag, ForceFlagShort, ForceFlagDVal, ForceFlagDescription)
	return cmd
}

func nsPrint(cmd *cobra.Command, nss []*metadata.NamespaceConfigRepoMetadata) {
	if isQuiet, _ := cmd.Flags().GetBool(QuietModeFlag); isQuiet {
		nsStr := ""
		for _, ns := range nss {
			nsStr += fmt.Sprintf("%s ", ns.Name)
		}
		printLinef(cmd, "%s", strings.TrimSpace(nsStr))
	} else {
		w := tabwriter.NewWriter(os.Stdout, 0, 2, 2, ' ', 0)
		_, _ = fmt.Fprintf(w, "Namespace\tVersion\n")
		for _, ns := range nss {
			_, _ = fmt.Fprintf(w, "%s\t%s\n", ns.Name, ns.Version)
		}
		_ = w.Flush()
	}
}
