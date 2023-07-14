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
	"github.com/lekkodev/cli/pkg/logging"
	"github.com/lekkodev/cli/pkg/star/static"
	"google.golang.org/protobuf/encoding/protojson"

	"github.com/lekkodev/cli/pkg/repo"
	"github.com/lekkodev/cli/pkg/secrets"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
)

func expCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "exp",
		Short: "Experimental commands",
	}

	cmd.AddCommand(
		expParseCmd(),
		expCleanupCmd(),
		expFormatCmd(),
	)

	return cmd
}

func expFormatCmd() *cobra.Command {
	var isQuiet, isVerbose bool
	cmd := &cobra.Command{
		Short:                 "Format star files",
		Use:                   formCmdUse("format"),
		DisableFlagsInUseLine: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			wd, err := os.Getwd()
			if err != nil {
				return err
			}
			rs := secrets.NewSecretsOrFail()
			r, err := repo.NewLocal(wd, rs)
			if err != nil {
				return errors.Wrap(err, "format")
			}
			if isQuiet {
				r.ConfigureLogger(nil)
			}

			if ffs, err := r.Format(cmd.Context(), isVerbose); err != nil {
				return err
			} else if isQuiet {
				printLinef(cmd, "%s", ffs)
			}
			return nil
		},
	}
	cmd.Flags().BoolVarP(&isQuiet, QuietModeFlag, QuietModeFlagShort, QuietModeFlagDVal, QuietModeFlagDescription)
	return cmd
}
func expParseCmd() *cobra.Command {
	var ns, featureName string
	var isQuiet, all, printFeature bool
	cmd := &cobra.Command{
		Short:                 "Parse a feature file using static analysis, and rewrite the starlark",
		Use:                   formCmdUse("parse", "[namespace/feature]"),
		DisableFlagsInUseLine: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			wd, err := os.Getwd()
			if err != nil {
				return err
			}
			r, err := repo.NewLocal(wd, secrets.NewSecretsOrFail())
			if err != nil {
				return errors.Wrap(err, "new repo")
			}
			if isQuiet {
				r.ConfigureLogger(nil)
			}
			ctx := cmd.Context()
			rootMD, _, err := r.ParseMetadata(ctx)
			if err != nil {
				return errors.Wrap(err, "parse metadata")
			}
			registry, err := r.BuildDynamicTypeRegistry(ctx, rootMD.ProtoDirectory)
			if err != nil {
				return errors.Wrap(err, "build dynamic type registry")
			}
			var nsfs namespaceFeatures
			if all {
				nsfs, err = getNamespaceFeatures(ctx, r, ns, featureName)
				if err != nil {
					return err
				}
			} else {
				if len(args) == 0 && !IsInteractive {
					return errors.New("namespace/feature are required arguments in the non-interactive mode")
				} else if len(args) > 1 {
					return errors.New("wrong number of args - expected one in the form: [namespace[/feature]]")
				}

				if len(args) == 1 {
					ns, featureName, err = feature.ParseFeaturePath(args[0])
					if err != nil {
						return err
					}
					nsfs, err = getNamespaceFeatures(ctx, r, ns, featureName)
					if err != nil {
						return err
					}
				}

				if len(ns) == 0 || len(featureName) == 0 {
					nsf, err := featureSelect(ctx, r, ns, featureName)
					if err != nil {
						return err
					}
					nsfs = append(nsfs, nsf)
				}
			}
			if !isQuiet {
				for _, nsf := range nsfs {
					f, err := r.Parse(ctx, nsf.namespace(), nsf.feature(), registry)
					printLinef(cmd, "%s", logging.Bold(fmt.Sprintf("[%s]", nsf.String())))
					if errors.Is(err, static.ErrUnsupportedStaticParsing) {
						fmt.Printf(" Unsupported static parsing: %v\n", err.Error())
					} else if err != nil {
						printErr(cmd, err)
					} else {
						fmt.Printf("[%s] Parsed\n", f.Type)
						if printFeature {
							printLinef(cmd, "%s\n", protojson.MarshalOptions{
								Resolver:  registry,
								Multiline: true,
							}.Format(f))
						}
					}
				}
			} else {
				s := ""
				for _, nsf := range nsfs {
					_, err := r.Parse(ctx, nsf.namespace(), nsf.feature(), registry)
					s += fmt.Sprintf("%s/%s ", nsf.ns, nsf.featureName)
					if errors.Is(err, static.ErrUnsupportedStaticParsing) {
						fmt.Printf(" Unsupported static parsing: %v\n", err.Error())
					} else if err != nil {
						printErr(cmd, err)
					}
				}
				printLinef(cmd, "%s", strings.TrimSpace(s))
			}
			return nil
		},
	}
	cmd.Flags().BoolVarP(&all, "all", "a", false, "parse all features")
	cmd.Flags().BoolVarP(&printFeature, "print", "p", false, "print parsed feature(s)")
	cmd.Flags().BoolVarP(&isQuiet, QuietModeFlag, QuietModeFlagShort, QuietModeFlagDVal, QuietModeFlagDescription)
	return cmd
}
func expCleanupCmd() *cobra.Command {
	var cmd = &cobra.Command{
		Short:                 "Deletes the current local branch or the branch specified (and its remote counterpart)",
		Use:                   formCmdUse("cleanup", "[branchname]"),
		DisableFlagsInUseLine: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			wd, err := os.Getwd()
			if err != nil {
				return err
			}
			rs := secrets.NewSecretsOrFail(secrets.RequireGithub())
			r, err := repo.NewLocal(wd, rs)
			if err != nil {
				return errors.Wrap(err, "cleanup")
			}
			var optionalBranchName *string
			if len(args) > 0 {
				optionalBranchName = &args[0]
			}
			if err = r.Cleanup(cmd.Context(), optionalBranchName, rs); err != nil {
				return err
			}
			if optionalBranchName != nil {
				printLinef(cmd, "%s", *optionalBranchName)
			}
			return nil
		},
	}
	return cmd
}
