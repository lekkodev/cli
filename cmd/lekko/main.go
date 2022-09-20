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
	"log"
	"os"
	"path/filepath"
	"strconv"

	"github.com/lekkodev/cli/pkg/eval"
	"github.com/lekkodev/cli/pkg/feature"
	"github.com/lekkodev/cli/pkg/fs"
	"github.com/lekkodev/cli/pkg/generate"
	"github.com/lekkodev/cli/pkg/gh"
	"github.com/lekkodev/cli/pkg/k8s"
	"github.com/lekkodev/cli/pkg/metadata"
	"github.com/lekkodev/cli/pkg/star"
	"github.com/lekkodev/cli/pkg/star/static"
	"github.com/lekkodev/cli/pkg/verify"
	"github.com/mitchellh/go-homedir"
	"github.com/pkg/errors"

	"github.com/spf13/cobra"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/types/known/wrapperspb"
)

func main() {
	rootCmd.AddCommand(verifyCmd)
	rootCmd.AddCommand(compileCmd)
	rootCmd.AddCommand(formatCmd())
	rootCmd.AddCommand(parseCmd())
	rootCmd.AddCommand(evalCmd)
	rootCmd.AddCommand(addCmd())
	rootCmd.AddCommand(removeCmd())
	rootCmd.AddCommand(reviewCmd)
	rootCmd.AddCommand(mergeCmd)
	// auth
	authCmd.AddCommand(loginCmd)
	authCmd.AddCommand(logoutCmd)
	authCmd.AddCommand(statusCmd)
	rootCmd.AddCommand(authCmd)
	// k8s
	k8sCmd.AddCommand(applyCmd())
	k8sCmd.AddCommand(listCmd())
	rootCmd.AddCommand(k8sCmd)

	if err := rootCmd.Execute(); err != nil {
		log.Println(err)
		os.Exit(1)
	}
}

var rootCmd = &cobra.Command{
	Use:           "lekko",
	Short:         "lekko - dynamic configuration helper",
	SilenceUsage:  true,
	SilenceErrors: true,
}

var verifyCmd = &cobra.Command{
	Use:   "verify",
	Short: "verify a config repository with a lekko.root.yaml",
	RunE: func(cmd *cobra.Command, args []string) error {
		// TODO lint the repo with the right proto files.
		wd, err := os.Getwd()
		if err != nil {
			return err
		}
		return verify.Verify(wd)
	},
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
			return star.Format(wd, verbose)
		},
	}
	cmd.Flags().BoolVarP(&verbose, "verbose", "v", false, "enable verbose output")
	return cmd
}

var compileCmd = &cobra.Command{
	Use:   "compile [namespace[/feature]]",
	Short: "compiles features based on individual definitions",
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		wd, err := os.Getwd()
		if err != nil {
			return err
		}
		var ns, f string
		if len(args) > 0 {
			ns, f, err = feature.ParseFeaturePath(args[0])
			if err != nil {
				return err
			}
		}
		return generate.Compile(wd, ns, f)
	},
}

func parseCmd() *cobra.Command {
	var file string
	cmd := &cobra.Command{
		Use:   "parse",
		Short: "parse a starlark file using static analysis, and rewrite it",
		RunE: func(cmd *cobra.Command, args []string) error {
			wd, err := os.Getwd()
			if err != nil {
				return err
			}
			if file == "" {
				return errors.New("no file given")
			}
			return static.Parse(wd, filepath.Join(wd, file))
		},
	}
	cmd.Flags().StringVarP(&file, "file", "f", "", "starlark file to walk")
	return cmd
}

var reviewCmd = &cobra.Command{
	Use:   "review",
	Short: "creates a pr with your changes",
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := context.Background()
		wd, err := os.Getwd()
		if err != nil {
			return err
		}
		if err := verify.Verify(wd); err != nil {
			return errors.Wrap(err, "verification failed")
		}
		cr, err := gh.New(ctx, wd)
		if err != nil {
			return errors.Wrap(err, "new repo")
		}

		return cr.Review(ctx)
	},
}

var mergeCmd = &cobra.Command{
	Use:   "merge pr-number",
	Short: "merges a pr for the current branch",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		wd, err := os.Getwd()
		if err != nil {
			return err
		}
		if err := verify.Verify(wd); err != nil {
			return errors.Wrap(err, "verification failed")
		}
		ctx := context.Background()
		cr, err := gh.New(ctx, wd)
		if err != nil {
			return errors.Wrap(err, "new repo")
		}
		defer cr.Close()
		prNum, err := strconv.Atoi(args[0])
		if err != nil {
			return errors.Wrap(err, "pr-number arg")
		}
		return cr.Merge(prNum)
	},
}

var authCmd = &cobra.Command{
	Use:   "auth",
	Short: "authenticates lekko with github",
}

var loginCmd = &cobra.Command{
	Use:   "login",
	Short: "authenticate with github",
	RunE: func(cmd *cobra.Command, args []string) error {
		wd, err := os.Getwd()
		if err != nil {
			return err
		}
		ctx := context.Background()
		cr, err := gh.New(ctx, wd)
		if err != nil {
			return errors.Wrap(err, "gh new")
		}
		defer cr.Close()

		return cr.Login(ctx)
	},
}

var logoutCmd = &cobra.Command{
	Use:   "logout",
	Short: "log out of github",
	RunE: func(cmd *cobra.Command, args []string) error {
		wd, err := os.Getwd()
		if err != nil {
			return err
		}
		ctx := context.Background()
		cr, err := gh.New(ctx, wd)
		if err != nil {
			return errors.Wrap(err, "gh new")
		}
		defer cr.Close()

		return cr.Logout(ctx)
	},
}

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "display lekko authentication status",
	RunE: func(cmd *cobra.Command, args []string) error {
		wd, err := os.Getwd()
		if err != nil {
			return err
		}
		ctx := context.Background()
		cr, err := gh.New(ctx, wd)
		if err != nil {
			return errors.Wrap(err, "gh new")
		}
		defer cr.Close()

		cr.Status(ctx)
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

		ctxMap := make(map[string]interface{})
		if err := json.Unmarshal([]byte(args[1]), &ctxMap); err != nil {
			return err
		}
		rootMD, _, err := metadata.ParseFullConfigRepoMetadataStrict(context.Background(), wd, fs.LocalProvider())
		if err != nil {
			return errors.Wrap(err, "failed to parse config repo metadata")
		}
		registry, err := star.BuildDynamicTypeRegistryFromFile(rootMD.ProtoDirectory)
		if err != nil {
			return errors.Wrap(err, "failed to build dynamic type registry")
		}

		res, err := eval.Eval(wd, args[0], ctxMap)
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

func addCmd() *cobra.Command {
	var complexFeature bool
	ret := &cobra.Command{
		Use:   "add namespace[/feature]",
		Short: "Adds a new feature flag or namespace",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			wd, err := os.Getwd()
			if err != nil {
				return err
			}
			if err := cmd.ParseFlags(args); err != nil {
				return errors.Wrap(err, "parse flags")
			}
			namespace, featureName, err := feature.ParseFeaturePath(args[0])
			if err != nil {
				return errors.Wrap(err, "parse feature path")
			}
			return generate.Add(wd, namespace, featureName, complexFeature)
		},
	}
	ret.Flags().BoolVarP(&complexFeature, "complex", "c", false, "create a complex configuration with proto, rules and validation")
	return ret
}

func removeCmd() *cobra.Command {
	ret := &cobra.Command{
		Use:   "remove namespace[/feature]",
		Short: "Removes an existing feature flag or namespace",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			wd, err := os.Getwd()
			if err != nil {
				return err
			}
			namespace, featureName, err := feature.ParseFeaturePath(args[0])
			if err != nil {
				return errors.Wrap(err, "parse feature path")
			}
			if featureName == "" {
				return generate.RemoveNamespace(wd, namespace)
			}
			return generate.RemoveFeature(wd, namespace, featureName)
		},
	}
	return ret
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
			if err := verify.Verify(wd); err != nil {
				return errors.Wrap(err, "verification failed")
			}

			ctx := context.Background()
			cr, err := gh.New(ctx, wd)
			if err != nil {
				return err
			}

			kube, err := k8s.NewKubernetes(kubeConfig, cr)
			if err != nil {
				return errors.Wrap(err, "failed to build k8s client")
			}
			if err := kube.Apply(ctx, wd); err != nil {
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

			ctx := context.Background()
			kube, err := k8s.NewKubernetes(kubeConfig, nil)
			if err != nil {
				return errors.Wrap(err, "failed to build k8s client")
			}
			if err := kube.List(ctx); err != nil {
				return errors.Wrap(err, "list")
			}
			return nil
		},
	}
	localKubeParams(ret, &kubeConfig)
	return ret
}
