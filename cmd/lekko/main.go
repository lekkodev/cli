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
	goflag "flag"
	"fmt"
	"os"
	"strconv"

	"github.com/AlecAivazis/survey/v2"
	"github.com/bufbuild/connect-go"
	"github.com/lainio/err2"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"google.golang.org/protobuf/encoding/protojson"

	bffv1beta1 "buf.build/gen/go/lekkodev/cli/protocolbuffers/go/lekko/bff/v1beta1"
	"github.com/lekkodev/cli/pkg/feature"
	"github.com/lekkodev/cli/pkg/gh"
	"github.com/lekkodev/cli/pkg/lekko"
	"github.com/lekkodev/cli/pkg/logging"
	"github.com/lekkodev/cli/pkg/repo"
	"github.com/lekkodev/cli/pkg/secrets"
	"github.com/lekkodev/cli/pkg/star/static"
)

// Updated at build time using ldflags
var version = "development"

var coreCommands = &cobra.Group{
	ID:    "core",
	Title: "CORE COMMANDS",
}
var extraCommands = &cobra.Group{
	ID:    "extra",
	Title: "EXTRA COMMANDS",
}
var managementCommands = &cobra.Group{
	ID:    "management",
	Title: "MANAGEMENT COMMANDS",
}
var internalCommands = &cobra.Group{
	ID:    "internal",
	Title: "INTERNAL COMMANDS",
}

func addGroup(parent *cobra.Command, group *cobra.Group, cmds ...*cobra.Command) {
	parent.AddGroup(group)
	for _, cmd := range cmds {
		cmd.GroupID = group.ID
		parent.AddCommand(cmd)
	}
}

func main() {
	err2.SetErrorTracer(os.Stderr)
	// to support standard go flags defined by 3rd party libraries
	// for example err2
	pflag.CommandLine.AddGoFlagSet(goflag.CommandLine)

	rootCmd := rootCmd()

	addGroup(
		rootCmd,
		coreCommands,
		initCmd(),
		genCmd(),
		syncCmd(),
		bisyncCmd(),
		pushCmd(),
		pullCmd(),
	)

	addGroup(
		rootCmd,
		managementCommands,
		authCmd(),
		setupCmd(),
		teamCmd(),
		repoCmd(),
		featureCmd(),
		namespaceCmd(),
		apikeyCmd(),
		upgradeCmd(),
	)

	addGroup(
		rootCmd,
		extraCommands,
		// config repo commands
		compileCmd(),
		verifyCmd(),
		commitCmd(),
		reviewCmd(),
		mergeCmd,
		restoreCmd(),
		parseCmd(),
		cleanupCmd,
		formatCmd(),
	)

	addGroup(
		rootCmd,
		internalCommands,
		mergeFileCmd(),
		diffCmd(),
		confCmd(),
		convertLangCmd(),
	)

	logging.InitColors()

	handleErr := func(err error) error {
		fmt.Println(err)
		os.Exit(1)
		return nil
	}
	defer err2.Catch(handleErr)
	if err := rootCmd.ExecuteContext(context.Background()); err != nil {
		_ = handleErr(err)
	}
}

func rootCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:           "lekko",
		Short:         "Lekko is a dynamic configuration management tool",
		Version:       version,
		SilenceUsage:  true,
		SilenceErrors: true,
	}
	cmd.PersistentFlags().StringVar(&lekko.URL, "backend-url", "https://prod.api.lekko.dev", "Lekko backend url")
	return cmd
}

func formatCmd() *cobra.Command {
	var verbose bool
	var wd string
	cmd := &cobra.Command{
		Use:   "format",
		Short: "format star files",
		RunE: func(cmd *cobra.Command, args []string) error {
			r, err := repo.NewLocal(wd, nil)
			if err != nil {
				return errors.Wrap(err, "new repo")
			}
			return r.Format(cmd.Context(), verbose)
		},
	}
	cmd.Flags().BoolVarP(&verbose, "verbose", "v", false, "enable verbose output")
	cmd.Flags().StringVarP(&wd, "config-path", "c", ".", "path to configuration repository")
	return cmd
}

func compileCmd() *cobra.Command {
	var force, dryRun, upgrade, verbose bool
	var wd string
	cmd := &cobra.Command{
		Use:   "compile [namespace[/config]]",
		Short: "compiles configs based on individual definitions",
		RunE: func(cmd *cobra.Command, args []string) error {
			r, err := repo.NewLocal(wd, nil)
			if err != nil {
				return err
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
			if _, err := r.Compile(ctx, &repo.CompileRequest{
				Registry:                     registry,
				NamespaceFilter:              ns,
				FeatureFilter:                f,
				DryRun:                       dryRun,
				IgnoreBackwardsCompatibility: force,
				// don't verify file structure, since we may have not yet generated
				// the DSLs for newly added features.
				Verify:  false,
				Upgrade: upgrade,
				Verbose: verbose,
			}); err != nil {
				return errors.Wrap(err, "compile")
			}
			return nil
		},
	}
	cmd.Flags().BoolVarP(&force, "force", "f", false, "force compilation, ignoring validation check failures.")
	cmd.Flags().BoolVarP(&dryRun, "dry-run", "d", false, "skip persisting any newly compiled changes to disk.")
	cmd.Flags().BoolVarP(&upgrade, "upgrade", "u", false, "upgrade any of the requested namespaces that are behind the latest version.")
	cmd.Flags().BoolVarP(&verbose, "verbose", "v", false, "enable verbose error logging.")
	cmd.Flags().StringVarP(&wd, "config-path", "c", ".", "path to configuration repository")
	return cmd
}

func verifyCmd() *cobra.Command {
	var wd string
	cmd := &cobra.Command{
		Use:   "verify [namespace[/config]]",
		Short: "verifies configs based on individual definitions",
		RunE: func(cmd *cobra.Command, args []string) error {
			r, err := repo.NewLocal(wd, nil)
			if err != nil {
				return err
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
			return r.Verify(ctx, &repo.VerifyRequest{
				Registry:        registry,
				NamespaceFilter: ns,
				FeatureFilter:   f,
			})
		},
	}
	cmd.Flags().StringVarP(&wd, "config-path", "c", ".", "path to configuration repository")
	return cmd
}

func parseCmd() *cobra.Command {
	var wd, ns, featureName string
	var all, printFeature bool
	cmd := &cobra.Command{
		Use:   "parse",
		Short: "parse a feature file using static analysis, and rewrite the starlark",
		RunE: func(cmd *cobra.Command, args []string) error {
			r, err := repo.NewLocal(wd, nil)
			if err != nil {
				return errors.Wrap(err, "new repo")
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
				nsf, err := featureSelect(ctx, r, ns, featureName)
				if err != nil {
					return err
				}
				nsfs = append(nsfs, nsf)
			}
			for _, nsf := range nsfs {
				f, err := r.Parse(ctx, nsf.namespace(), nsf.feature(), registry)
				fmt.Print(logging.Bold(fmt.Sprintf("[%s]", nsf.String())))
				if errors.Is(err, static.ErrUnsupportedStaticParsing) {
					fmt.Printf(" Unsupported static parsing: %v\n", err.Error())
				} else if err != nil {
					fmt.Printf(" %v\n", err)
				} else {
					fmt.Printf("[%s] Parsed\n", f.Type)
					if printFeature {
						fmt.Println(protojson.MarshalOptions{
							Resolver:  registry,
							Multiline: true,
						}.Format(f))
					}
				}
			}
			return nil
		},
	}
	cmd.Flags().StringVarP(&ns, "namespace", "n", "", "namespace to remove config from")
	cmd.Flags().StringVarP(&featureName, "feature", "f", "", "name of config to remove")
	_ = cmd.Flags().MarkHidden("feature")
	cmd.Flags().StringVarP(&featureName, "config", "c", "", "name of config to remove")
	cmd.Flags().BoolVarP(&all, "all", "a", false, "parse all configs")
	cmd.Flags().BoolVarP(&printFeature, "print", "p", false, "print parsed config(s)")
	cmd.Flags().StringVar(&wd, "config-path", ".", "path to configuration repository")
	return cmd
}

func reviewCmd() *cobra.Command {
	var title, wd string
	cmd := &cobra.Command{
		Use:   "review",
		Short: "creates a pr with your changes",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			rs := secrets.NewSecretsOrFail(secrets.RequireGithub())
			r, err := repo.NewLocal(wd, rs)
			if err != nil {
				return errors.Wrap(err, "new repo")
			}
			if err := r.Verify(ctx, &repo.VerifyRequest{}); err != nil {
				return errors.Wrap(err, "verify")
			}

			ghCli := gh.NewGithubClientFromToken(ctx, rs.GetGithubToken())
			if _, err := ghCli.GetUser(ctx); err != nil {
				return errors.Wrap(err, "github auth fail")
			}

			if len(title) == 0 {
				fmt.Printf("-------------------\n")
				if err := survey.AskOne(&survey.Input{
					Message: "Title:",
				}, &title); err != nil {
					return errors.Wrap(err, "prompt")
				}
			}

			_, err = r.Review(ctx, title, ghCli, rs)
			return err
		},
	}
	cmd.Flags().StringVarP(&title, "title", "t", "", "Title of pull request")
	cmd.Flags().StringVarP(&wd, "config-path", "c", ".", "path to configuration repository")
	return cmd
}

var mergeCmd = &cobra.Command{
	Use:   "merge [pr-number]",
	Short: "merges a pr for the current branch",
	Args:  cobra.MaximumNArgs(1),
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
		if err := r.Verify(ctx, &repo.VerifyRequest{}); err != nil {
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
		if err := r.Merge(ctx, prNum, ghCli, rs); err != nil {
			return errors.Wrap(err, "merge")
		}
		fmt.Printf("PR merged.\n")
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
			fmt.Printf("Visit %s to monitor your rollout.\n", rolloutsURL(rs.GetLekkoTeam(), owner, repo, defaultBranch))
		}
		return nil
	},
}

func rolloutsURL(team, owner, repo, branch string) string {
	return fmt.Sprintf("https://app.lekko.com/teams/%s/repositories/%s/%s/branches/%s/commits", team, owner, repo, branch)
}

type provider string

const (
	providerLekko  provider = "lekko"
	providerGithub provider = "github"
)

func (p *provider) String() string {
	return string(*p)
}

func (p *provider) Set(v string) error {
	switch v {
	case string(providerLekko), string(providerGithub):
		*p = provider(v)
	default:
		return errors.New(`must be one of "lekko" or "github"`)
	}
	return nil
}

func (p *provider) Type() string {
	return "provider"
}

func commitCmd() *cobra.Command {
	var message, wd string
	cmd := &cobra.Command{
		Use:   "commit",
		Short: "commits local changes to the remote branch",
		RunE: func(cmd *cobra.Command, args []string) error {
			rs := secrets.NewSecretsOrFail(secrets.RequireGithub())
			r, err := repo.NewLocal(wd, rs)
			if err != nil {
				return errors.Wrap(err, "new repo")
			}
			ctx := cmd.Context()
			if err := r.Verify(ctx, &repo.VerifyRequest{}); err != nil {
				return errors.Wrap(err, "verify")
			}
			signature, err := repo.GetCommitSignature(ctx, rs, rs.GetLekkoUsername())
			if err != nil {
				return err
			}
			if _, err = r.Commit(ctx, rs, message, signature); err != nil {
				return err
			}
			return nil
		},
	}
	cmd.Flags().StringVarP(&message, "message", "m", "config change commit", "commit message")
	cmd.Flags().StringVarP(&wd, "config-path", "c", ".", "path to configuration repository")
	return cmd
}

var cleanupCmd = &cobra.Command{
	Use:   "cleanup [branchname]",
	Short: "deletes the current local branch or the branch specified (and its remote counterpart)",
	Args:  cobra.MaximumNArgs(1),
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
		var optionalBranchName *string
		if len(args) > 0 {
			optionalBranchName = &args[0]
		}
		if err = r.Cleanup(cmd.Context(), optionalBranchName, rs); err != nil {
			return err
		}
		return nil
	},
}

func restoreCmd() *cobra.Command {
	var force bool
	var wd string
	cmd := &cobra.Command{
		Use:   "restore [hash]",
		Short: "restores repo to a particular hash",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			rs := secrets.NewSecretsOrFail(secrets.RequireGithub())
			r, err := repo.NewLocal(wd, rs)
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
			fmt.Printf("Successfully rebuilt dynamic type registry.\n")
			if _, err := r.Compile(ctx, &repo.CompileRequest{
				Registry:                     registry,
				DryRun:                       false,
				IgnoreBackwardsCompatibility: force,
			}); err != nil {
				return errors.Wrap(err, "compile")
			}
			fmt.Printf("Restored hash %s to your working directory. \nRun `lekko review` to create a PR with these changes.\n", args[0])
			return nil
		},
	}
	cmd.Flags().BoolVarP(&force, "force", "f", false, "force compilation, ignoring validation check failures.")
	cmd.Flags().StringVarP(&wd, "config-path", "c", ".", "path to configuration repository")
	return cmd
}

func upgradeCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "upgrade",
		Short: "upgrade lekko to the latest version using homebrew",
		RunE: func(cmd *cobra.Command, args []string) error {
			fmt.Printf(
				`Our CLI is currently managed by Homebrew.
In order to upgrade, run the following commands:

	brew update
	brew upgrade lekko

For more information, check out our docs:
https://app.lekko.com/docs/cli/
`)
			return nil
		},
	}
	return cmd
}
