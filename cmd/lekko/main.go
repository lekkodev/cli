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
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/mail"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/AlecAivazis/survey/v2"
	"github.com/lekkodev/cli/pkg/feature"
	"github.com/lekkodev/cli/pkg/gh"
	"github.com/lekkodev/cli/pkg/k8s"
	"github.com/lekkodev/cli/pkg/lekko"
	"github.com/lekkodev/cli/pkg/logging"
	"github.com/lekkodev/cli/pkg/oauth"
	"github.com/lekkodev/cli/pkg/repo"
	"github.com/lekkodev/cli/pkg/secrets"
	"github.com/mitchellh/go-homedir"
	"github.com/pkg/errors"

	"github.com/spf13/cobra"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/types/known/wrapperspb"
)

func main() {
	rootCmd.AddCommand(compileCmd())
	rootCmd.AddCommand(evalCmd)
	rootCmd.AddCommand(commitCmd())
	rootCmd.AddCommand(reviewCmd())
	rootCmd.AddCommand(mergeCmd)
	rootCmd.AddCommand(restoreCmd())
	rootCmd.AddCommand(teamCmd())
	rootCmd.AddCommand(repoCmd())
	rootCmd.AddCommand(featureCmd())
	rootCmd.AddCommand(namespaceCmd())
	rootCmd.AddCommand(apikeyCmd())
	// auth
	authCmd.AddCommand(loginCmd())
	authCmd.AddCommand(logoutCmd())
	authCmd.AddCommand(statusCmd)
	authCmd.AddCommand(registerCmd())
	authCmd.AddCommand(tokensCmd)
	rootCmd.AddCommand(authCmd)
	// exp
	k8sCmd.AddCommand(applyCmd())
	k8sCmd.AddCommand(listCmd())
	experimentalCmd.AddCommand(k8sCmd)
	experimentalCmd.AddCommand(parseCmd())
	experimentalCmd.AddCommand(startCmd)
	experimentalCmd.AddCommand(cleanupCmd)
	experimentalCmd.AddCommand(formatCmd())
	rootCmd.AddCommand(experimentalCmd)

	logging.InitColors()
	if err := rootCmd.ExecuteContext(context.Background()); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

var rootCmd = &cobra.Command{
	Use:           "lekko",
	Short:         "lekko - dynamic configuration helper",
	Version:       "v0.2.4", // TODO: autoupdate this when releasing a new tag
	SilenceUsage:  true,
	SilenceErrors: true,
}

func formatCmd() *cobra.Command {
	var verbose bool
	cmd := &cobra.Command{
		Use:   "format",
		Short: "format star files",
		RunE: func(cmd *cobra.Command, args []string) error {
			wd, err := os.Getwd()
			if err != nil {
				return err
			}
			rs := secrets.NewSecretsOrFail()
			r, err := repo.NewLocal(wd, rs)
			if err != nil {
				return errors.Wrap(err, "new repo")
			}
			return r.Format(cmd.Context())
		},
	}
	cmd.Flags().BoolVarP(&verbose, "verbose", "v", false, "enable verbose output")
	return cmd
}

func compileCmd() *cobra.Command {
	var force, dryRun bool
	cmd := &cobra.Command{
		Use:   "compile [namespace[/feature]]",
		Short: "compiles features based on individual definitions",
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
			ctx := cmd.Context()
			rootMD, _, err := r.ParseMetadata(ctx)
			if err != nil {
				return errors.Wrap(err, "parse metadata")
			}
			registry, err := r.ReBuildDynamicTypeRegistry(ctx, rootMD.ProtoDirectory)
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
				Persist:                      !dryRun,
				IgnoreBackwardsCompatibility: force,
				// don't verify file structure, since we may have not yet generated
				// the DSLs for newly added features.
				Verify: false,
			}); err != nil {
				return errors.Wrap(err, "compile")
			}
			return nil
		},
	}
	cmd.Flags().BoolVarP(&force, "force", "f", false, "force compilation, ignoring validation check failures.")
	cmd.Flags().BoolVarP(&dryRun, "dry-run", "d", false, "skip persisting any newly compiled changes to disk.")
	return cmd
}

func parseCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "parse namespace/feature",
		Short: "parse a starlark file using static analysis, and rewrite it",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			wd, err := os.Getwd()
			if err != nil {
				return err
			}
			rs := secrets.NewSecretsOrFail()
			r, err := repo.NewLocal(wd, rs)
			if err != nil {
				return errors.Wrap(err, "new repo")
			}
			namespace, featureName, err := feature.ParseFeaturePath(args[0])
			if err != nil {
				return errors.Wrap(err, "parse feature path")
			}
			return r.Parse(cmd.Context(), namespace, featureName)
		},
	}
}

func reviewCmd() *cobra.Command {
	var title string
	cmd := &cobra.Command{
		Use:   "review",
		Short: "creates a pr with your changes",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			wd, err := os.Getwd()
			if err != nil {
				return err
			}
			rs := secrets.NewSecretsOrFail(secrets.RequireGithub())
			r, err := repo.NewLocal(wd, rs)
			if err != nil {
				return errors.Wrap(err, "new repo")
			}
			if _, err := r.Compile(ctx, &repo.CompileRequest{
				Verify: true,
			}); err != nil {
				return errors.Wrap(err, "compile")
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

			_, err = r.Review(ctx, title, ghCli)
			return err
		},
	}
	cmd.Flags().StringVarP(&title, "title", "t", "", "Title of pull request")
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
		if _, err := r.Compile(ctx, &repo.CompileRequest{
			Verify: true,
		}); err != nil {
			return errors.Wrap(err, "compile")
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
		if err := r.Merge(ctx, prNum, ghCli); err != nil {
			return errors.Wrap(err, "merge")
		}
		fmt.Printf("PR merged.\n")
		if len(rs.GetLekkoTeam()) > 0 {
			u, err := r.GetRemoteURL()
			if err != nil {
				return errors.Wrap(err, "get remote url")
			}
			owner, repo, err := parseOwnerRepo(u)
			if err != nil {
				return errors.Wrap(err, "parse owner repo")
			}
			fmt.Printf("Visit %s to monitor your rollout.\n", rolloutsURL(rs.GetLekkoTeam(), owner, repo))
		}
		return nil
	},
}

func parseOwnerRepo(githubURL string) (string, string, error) {
	parts := strings.Split(githubURL, "/")
	n := len(parts)
	if n < 3 {
		return "", "", errors.New("invalid github repo url")
	}
	owner, repo := parts[n-2], parts[n-1]
	if idx := strings.Index(repo, ".git"); idx >= 0 {
		repo = repo[0:idx]
	}
	return owner, repo, nil
}

func rolloutsURL(team, owner, repo string) string {
	return fmt.Sprintf("https://app.lekko.com/teams/%s/repositories/%s/%s/commits", team, owner, repo)
}

var authCmd = &cobra.Command{
	Use:   "auth",
	Short: "authenticates lekko cli",
}

func loginCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "login",
		Short: "authenticate with lekko and github, if unauthenticated",
		RunE: func(cmd *cobra.Command, args []string) error {
			return secrets.WithWriteSecrets(func(ws secrets.WriteSecrets) error {
				auth := oauth.NewOAuth(lekko.NewBFFClient(ws))
				return auth.Login(cmd.Context(), ws)
			})
		},
	}
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

func logoutCmd() *cobra.Command {
	var p provider
	cmd := &cobra.Command{
		Use:   "logout",
		Short: "log out of lekko or github",
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(string(p)) == 0 {
				return errors.Errorf("provider must be specified")
			}
			return secrets.WithWriteSecrets(func(ws secrets.WriteSecrets) error {
				auth := oauth.NewOAuth(lekko.NewBFFClient(ws))
				return auth.Logout(cmd.Context(), string(p), ws)
			})
		},
	}
	cmd.Flags().VarP(&p, "provider", "p", "provider to log out. allowed: 'lekko', 'github'.")
	return cmd
}

func registerCmd() *cobra.Command {
	var email, password string
	cmd := &cobra.Command{
		Use:   "register",
		Short: "register an account with lekko",
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(email) == 0 {
				if err := survey.AskOne(&survey.Input{
					Message: "Email:",
				}, &email); err != nil {
					return errors.Wrap(err, "prompt email")
				}
			}
			if _, err := mail.ParseAddress(email); err != nil {
				return errors.New("invalid email address")
			}
			// prompt password
			if err := survey.AskOne(&survey.Password{
				Message: "Password:",
			}, &password); err != nil {
				return errors.Wrap(err, "prompt password")
			}
			auth := oauth.NewOAuth(lekko.NewBFFClient(secrets.NewSecretsOrFail()))
			if err := auth.Register(cmd.Context(), email, password); err != nil {
				return err
			}
			fmt.Println("Account registered with lekko.")
			fmt.Println("Run `lekko auth login` to complete oauth.")
			return nil
		},
	}
	cmd.Flags().StringVarP(&email, "email", "e", "", "email to create lekko account with")
	return cmd
}

var tokensCmd = &cobra.Command{
	Use:   "tokens",
	Short: "display token(s) currently in use",
	RunE: func(cmd *cobra.Command, args []string) error {
		rs := secrets.NewSecretsOrFail()
		tokens := oauth.NewOAuth(lekko.NewBFFClient(rs)).Tokens(cmd.Context(), rs)
		fmt.Println(strings.Join(tokens, "\n"))
		return nil
	},
}

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "display lekko authentication status",
	RunE: func(cmd *cobra.Command, args []string) error {
		rs := secrets.NewSecretsOrFail()
		auth := oauth.NewOAuth(lekko.NewBFFClient(rs))
		auth.Status(cmd.Context(), false, rs)
		return nil
	},
}

var evalCmd = &cobra.Command{
	Use:   "eval namespace/feature '{\"context_key\": 123}'",
	Short: "Evaluates a specified feature based on the provided context",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		wd, err := os.Getwd()
		if err != nil {
			return err
		}
		rs := secrets.NewSecretsOrFail()
		r, err := repo.NewLocal(wd, rs)
		if err != nil {
			return errors.Wrap(err, "new repo")
		}

		ctxMap := make(map[string]interface{})
		if err := json.Unmarshal([]byte(args[1]), &ctxMap); err != nil {
			return err
		}
		ctx := cmd.Context()
		rootMD, _, err := r.ParseMetadata(ctx)
		if err != nil {
			return errors.Wrap(err, "failed to parse config repo metadata")
		}

		registry, err := r.BuildDynamicTypeRegistry(ctx, rootMD.ProtoDirectory)
		if err != nil {
			return errors.Wrap(err, "failed to build dynamic type registry")
		}
		namespace, featureName, err := feature.ParseFeaturePath(args[0])
		if err != nil {
			return errors.Wrap(err, "parse feature path")
		}

		res, err := r.Eval(ctx, namespace, featureName, ctxMap)
		if err != nil {
			return err
		}

		fmt.Printf("Correctly evaluated to an any of type: %v\n", res.TypeUrl)
		boolVal := new(wrapperspb.BoolValue)
		if res.MessageIs(boolVal) {
			if err := res.UnmarshalTo(boolVal); err != nil {
				return err
			}
			fmt.Printf("Resulting value: %t\n", boolVal.Value)
		} else { // TODO: Handle other types
			jBytes, err := protojson.MarshalOptions{
				Resolver: registry,
			}.Marshal(res)
			if err != nil {
				return errors.Wrap(err, "failed to marshal proto to json")
			}
			indentedJBytes := bytes.NewBuffer(nil)
			if err := json.Indent(indentedJBytes, jBytes, "", "  "); err != nil {
				return errors.Wrap(err, "failed to indent json")
			}
			fmt.Printf("%v\n", indentedJBytes)
		}
		return nil
	},
}

var k8sCmd = &cobra.Command{
	Use:   "k8s",
	Short: "manage lekko configurations in kubernetes. Uses the current k8s context set in your kubeconfig file.",
}

func localKubeParams(cmd *cobra.Command, kubeConfig *string) {
	var defaultKubeconfig string
	// ref: https://github.com/kubernetes/client-go/blob/master/examples/out-of-cluster-client-configuration/main.go
	home, err := homedir.Dir()
	if err == nil {
		defaultKubeconfig = filepath.Join(home, ".kube", "config")
	}
	cmd.Flags().StringVarP(kubeConfig, "kubeconfig", "c", defaultKubeconfig, "absolute path to the kube config file")
}

func applyCmd() *cobra.Command {
	var kubeConfig string
	ret := &cobra.Command{
		Use:   "apply",
		Short: "apply local configurations to kubernetes configmaps",
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := cmd.ParseFlags(args); err != nil {
				return errors.Wrap(err, "failed to parse flags")
			}

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
			if _, err := r.Compile(ctx, &repo.CompileRequest{
				Verify: true,
			}); err != nil {
				return errors.Wrap(err, "compile")
			}
			kube, err := k8s.NewKubernetes(kubeConfig, r)
			if err != nil {
				return errors.Wrap(err, "failed to build k8s client")
			}
			if err := kube.Apply(ctx); err != nil {
				return errors.Wrap(err, "apply")
			}

			return nil
		},
	}
	localKubeParams(ret, &kubeConfig)
	return ret
}

func listCmd() *cobra.Command {
	var kubeConfig string
	ret := &cobra.Command{
		Use:   "list",
		Short: "list lekko configurations currently in kubernetes",
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := cmd.ParseFlags(args); err != nil {
				return errors.Wrap(err, "failed to parse flags")
			}

			kube, err := k8s.NewKubernetes(kubeConfig, nil)
			if err != nil {
				return errors.Wrap(err, "failed to build k8s client")
			}
			if err := kube.List(cmd.Context()); err != nil {
				return errors.Wrap(err, "list")
			}
			return nil
		},
	}
	localKubeParams(ret, &kubeConfig)
	return ret
}

var experimentalCmd = &cobra.Command{
	Use:   "exp",
	Short: "experimental commands",
}

var startCmd = &cobra.Command{
	Use:   "start",
	Short: "sets up the local config repo for making changes",
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
		var branch string
		if len(args) == 0 {
			branch, err = r.GenBranchName()
			if err != nil {
				return errors.Wrap(err, "gen branch name")
			}
		} else {
			branch = args[0]
		}
		if err = r.CreateOrRestore(branch); err != nil {
			return err
		}
		return nil
	},
}

func commitCmd() *cobra.Command {
	var message string
	cmd := &cobra.Command{
		Use:   "commit",
		Short: "commits local changes to the remote branch",
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
			if _, err := r.Compile(ctx, &repo.CompileRequest{
				Verify: true,
			}); err != nil {
				return errors.Wrap(err, "compile")
			}
			if _, err = r.Commit(ctx, message); err != nil {
				return err
			}
			return nil
		},
	}
	cmd.Flags().StringVarP(&message, "message", "m", "config change commit", "commit message")
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
		if err = r.Cleanup(cmd.Context(), optionalBranchName); err != nil {
			return err
		}
		return nil
	},
}

func restoreCmd() *cobra.Command {
	var force bool
	cmd := &cobra.Command{
		Use:   "restore [hash]",
		Short: "restores repo to a particular hash",
		Args:  cobra.ExactArgs(1),
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
			if err := r.RestoreWorkingDirectory(args[0]); err != nil {
				return errors.Wrap(err, "restore wd")
			}
			ctx := cmd.Context()
			rootMD, _, err := r.ParseMetadata(ctx)
			if err != nil {
				return errors.Wrap(err, "parse metadata")
			}
			registry, err := r.ReBuildDynamicTypeRegistry(ctx, rootMD.ProtoDirectory)
			if err != nil {
				return errors.Wrap(err, "rebuild type registry")
			}
			fmt.Printf("Successfully rebuilt dynamic type registry.\n")
			if _, err := r.Compile(ctx, &repo.CompileRequest{
				Registry:                     registry,
				Persist:                      true,
				IgnoreBackwardsCompatibility: force,
			}); err != nil {
				return errors.Wrap(err, "compile")
			}
			fmt.Printf("Successfully compiled.\n")
			fmt.Printf("Restored hash %s to your working directory. \nRun `lekko review` to create a PR with these changes.\n", args[0])
			return nil
		},
	}
	cmd.Flags().BoolVarP(&force, "force", "f", false, "force compilation, ignoring validation check failures.")
	return cmd
}
