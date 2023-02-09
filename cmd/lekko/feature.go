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
	"text/tabwriter"

	"github.com/AlecAivazis/survey/v2"
	"github.com/lekkodev/cli/pkg/feature"
	"github.com/lekkodev/cli/pkg/metadata"
	"github.com/lekkodev/cli/pkg/repo"
	"github.com/lekkodev/cli/pkg/secrets"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
)

func featureCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "feature",
		Short: "feature management",
	}
	cmd.AddCommand(
		featureList(),
		featureAdd(),
		featureRemove(),
	)
	return cmd
}

func featureList() *cobra.Command {
	var ns string
	cmd := &cobra.Command{
		Use:   "list",
		Short: "list all features",
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
				return errors.Wrap(err, "list namespaces")
			}
			if len(ns) > 0 {
				for _, namespaceMD := range nss {
					if namespaceMD.Name == ns {
						nss = []*metadata.NamespaceConfigRepoMetadata{namespaceMD}
						break
					}
				}
			}
			for _, namespaceMD := range nss {
				ffs, err := r.GetFeatureFiles(cmd.Context(), namespaceMD.Name)
				if err != nil {
					return errors.Wrapf(err, "get feature files for ns %s", namespaceMD.Name)
				}
				for _, ff := range ffs {
					fmt.Printf("%s/%s\n", ff.NamespaceName, ff.Name)
				}
			}
			return nil
		},
	}
	cmd.Flags().StringVarP(&ns, "namespace", "n", "", "name of namespace to filter by")
	return cmd
}

func featureAdd() *cobra.Command {
	var ns, featureName, fType string
	cmd := &cobra.Command{
		Use:   "add",
		Short: "add feature",
		RunE: func(cmd *cobra.Command, args []string) error {
			wd, err := os.Getwd()
			if err != nil {
				return err
			}
			r, err := repo.NewLocal(wd, secrets.NewSecretsOrFail())
			if err != nil {
				return err
			}
			if len(ns) == 0 {
				nss, err := r.ListNamespaces(cmd.Context())
				if err != nil {
					return errors.Wrap(err, "list namespaces")
				}
				var options []string
				for _, ns := range nss {
					options = append(options, ns.Name)
				}
				if err := survey.AskOne(&survey.Select{
					Message: "Namespace:",
					Options: options,
				}, &ns); err != nil {
					return errors.Wrap(err, "prompt")
				}
			}
			if len(featureName) == 0 {
				if err := survey.AskOne(&survey.Input{
					Message: "Feature Name:",
				}, &featureName); err != nil {
					return errors.Wrap(err, "prompt")
				}
			}
			if len(fType) == 0 {
				if err := survey.AskOne(&survey.Select{
					Message: "Feature Type:",
					Options: feature.FeatureTypes(),
				}, &fType); err != nil {
					return errors.Wrap(err, "prompt")
				}
			}
			if path, err := r.AddFeature(cmd.Context(), ns, featureName, feature.FeatureType(fType)); err != nil {
				return errors.Wrap(err, "add feature")
			} else {
				fmt.Printf("Successfully added feature %s/%s at path %s\n", ns, featureName, path)
				fmt.Printf("Make any changes you wish, and then run `lekko compile`.")
			}
			return nil
		},
	}
	cmd.Flags().StringVarP(&ns, "namespace", "n", "", "namespace to add feature in")
	cmd.Flags().StringVarP(&featureName, "feature", "f", "", "name of feature to add")
	cmd.Flags().StringVarP(&fType, "type", "t", "", "type of feature to create")
	return cmd
}

func featureRemove() *cobra.Command {
	var ns, featureName string
	cmd := &cobra.Command{
		Use:   "remove",
		Short: "remove feature",
		RunE: func(cmd *cobra.Command, args []string) error {
			wd, err := os.Getwd()
			if err != nil {
				return err
			}
			r, err := repo.NewLocal(wd, secrets.NewSecretsOrFail())
			if err != nil {
				return err
			}
			if len(ns) == 0 {
				nss, err := r.ListNamespaces(cmd.Context())
				if err != nil {
					return errors.Wrap(err, "list namespaces")
				}
				var options []string
				for _, ns := range nss {
					options = append(options, ns.Name)
				}
				if err := survey.AskOne(&survey.Select{
					Message: "Namespace:",
					Options: options,
				}, &ns); err != nil {
					return errors.Wrap(err, "prompt")
				}
			}
			if len(featureName) == 0 {
				ffs, err := r.GetFeatureFiles(cmd.Context(), ns)
				if err != nil {
					return errors.Wrap(err, "get feature files")
				}
				var options []string
				for _, ff := range ffs {
					options = append(options, ff.Name)
				}
				if err := survey.AskOne(&survey.Select{
					Message: "Feature:",
					Options: options,
				}, &featureName); err != nil {
					return errors.Wrap(err, "prompt")
				}
			}
			if err := r.RemoveFeature(cmd.Context(), ns, featureName); err != nil {
				return errors.Wrap(err, "remove feature")
			}
			fmt.Printf("Successfully removed feature %s/%s\n", ns, featureName)
			return nil
		},
	}
	cmd.Flags().StringVarP(&ns, "namespace", "n", "", "namespace to remove feature from")
	cmd.Flags().StringVarP(&featureName, "feature", "f", "", "name of feature to remove")
	return cmd
}

func namespaceCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "ns",
		Short: "namespace management",
	}
	cmd.AddCommand(
		nsList,
		nsAdd(),
		nsRemove(),
	)
	return cmd
}

var nsList = &cobra.Command{
	Use:   "list",
	Short: "list namespaces in the current repository",
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
		w := tabwriter.NewWriter(os.Stdout, 0, 2, 2, ' ', 0)
		fmt.Fprintf(w, "Namespace\tVersion\n")
		for _, ns := range nss {
			fmt.Fprintf(w, "%s\t%s\n", ns.Name, ns.Version)
		}
		w.Flush()
		return nil
	},
}

func nsAdd() *cobra.Command {
	var name string
	cmd := &cobra.Command{
		Use:   "add",
		Short: "add namespace",
		RunE: func(cmd *cobra.Command, args []string) error {
			wd, err := os.Getwd()
			if err != nil {
				return err
			}
			r, err := repo.NewLocal(wd, secrets.NewSecretsOrFail())
			if err != nil {
				return err
			}
			if len(name) == 0 {
				if err := survey.AskOne(&survey.Input{
					Message: "Namespace name:",
				}, &name); err != nil {
					return errors.Wrap(err, "prompt")
				}
			}
			if err := r.AddNamespace(cmd.Context(), name); err != nil {
				return errors.Wrap(err, "add namespace")
			}
			fmt.Printf("Successfully added namespace %s\n", name)
			return nil
		},
	}
	cmd.Flags().StringVarP(&name, "name", "n", "", "name of namespace to delete")
	return cmd
}

func nsRemove() *cobra.Command {
	var name string
	cmd := &cobra.Command{
		Use:   "remove",
		Short: "remove namespace",
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
			// Confirm deletion
			fmt.Printf("Deleting namespace %s...\n", name)
			var inputName string
			if err := survey.AskOne(&survey.Input{
				Message: fmt.Sprintf("Enter '%s' to continue:", name),
			}, &inputName); err != nil {
				return errors.Wrap(err, "prompt")
			}
			if name != inputName {
				return errors.New("incorrect namespace input")
			}
			// actually delete
			if err := r.RemoveNamespace(cmd.Context(), name); err != nil {
				return errors.Wrap(err, "remove namespace")
			}
			fmt.Printf("Successfully deleted namespace %s\n", name)
			return nil
		},
	}
	cmd.Flags().StringVarP(&name, "name", "n", "", "name of namespace to delete")
	return cmd
}
