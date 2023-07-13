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
	"path/filepath"

	"github.com/lekkodev/cli/pkg/k8s"
	"github.com/lekkodev/cli/pkg/repo"
	"github.com/lekkodev/cli/pkg/secrets"
	"github.com/mitchellh/go-homedir"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
)

func k8sCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "k8s",
		Short: "Manage lekko configurations in kubernetes. Uses the current k8s context set in your kubeconfig file.",
	}
	cmd.AddCommand(
		applyCmd(),
		listCmd(),
	)
	return cmd
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
		Short:                 "Apply local configurations to kubernetes configmaps",
		Use:                   formCmdUse("apply"),
		DisableFlagsInUseLine: true,
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
			if _, err := r.Verify(ctx, &repo.VerifyRequest{}); err != nil {
				return errors.Wrap(err, "verify")
			}
			kube, err := k8s.NewKubernetes(kubeConfig, r)
			if err != nil {
				return errors.Wrap(err, "failed to build k8s client")
			}
			if err := kube.Apply(ctx, rs.GetUsername()); err != nil {
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
		Short:                 "List lekko configurations currently in kubernetes",
		Use:                   formCmdUse("list"),
		DisableFlagsInUseLine: true,
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
