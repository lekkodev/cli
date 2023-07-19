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
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"text/tabwriter"

	"github.com/AlecAivazis/survey/v2"
	"github.com/lekkodev/cli/pkg/feature"
	"github.com/lekkodev/cli/pkg/logging"
	"github.com/lekkodev/cli/pkg/metadata"
	"github.com/lekkodev/cli/pkg/repo"
	"github.com/lekkodev/cli/pkg/secrets"
	"github.com/lekkodev/go-sdk/pkg/eval"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"google.golang.org/protobuf/types/known/structpb"
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
		featureEval(),
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
	var ns, featureName, fType, fProtoMessage string
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
					Options: eval.FeatureTypes(),
				}, &fType); err != nil {
					return errors.Wrap(err, "prompt")
				}
			}

			if fType == string(eval.FeatureTypeProto) && len(fProtoMessage) == 0 {
				protos, err := r.GetProtoMessages(cmd.Context())
				if err != nil {
					return errors.Wrap(err, "unable to get proto messages")
				}
				if err := survey.AskOne(&survey.Select{
					Message: "Messages:",
					Options: protos,
				}, &fProtoMessage); err != nil {
					return errors.Wrap(err, "prompt")
				}
			}

			ctx := cmd.Context()
			if path, err := r.AddFeature(ctx, ns, featureName, eval.FeatureType(fType), fProtoMessage); err != nil {
				return errors.Wrap(err, "add feature")
			} else {
				fmt.Printf("Successfully added feature %s/%s at path %s\n", ns, featureName, path)
				fmt.Printf("Make any changes you wish, and then run `lekko compile`.")
			}
			_, err = r.Compile(ctx, &repo.CompileRequest{})
			return err
		},
	}
	cmd.Flags().StringVarP(&ns, "namespace", "n", "", "namespace to add feature in")
	cmd.Flags().StringVarP(&featureName, "feature", "f", "", "name of feature to add")
	cmd.Flags().StringVarP(&fType, "type", "t", "", "type of feature to create")
	cmd.Flags().StringVarP(&fProtoMessage, "proto-message", "m", "", "protobuf message of feature to create")
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
			nsf, err := featureSelect(cmd.Context(), r, ns, featureName)
			if err != nil {
				return err
			}
			ns, featureName = nsf.namespace(), nsf.feature()
			// Confirm
			featurePair := fmt.Sprintf("%s/%s", ns, featureName)
			fmt.Printf("Deleting feature %s...\n", featurePair)
			if err := confirmInput(featurePair); err != nil {
				return err
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

func featureEval() *cobra.Command {
	var ns, featureName, jsonContext string
	var verbose bool
	cmd := &cobra.Command{
		Use:   "eval",
		Short: "evaluate feature",
		RunE: func(cmd *cobra.Command, args []string) error {
			wd, err := os.Getwd()
			if err != nil {
				return err
			}
			r, err := repo.NewLocal(wd, secrets.NewSecretsOrFail())
			if err != nil {
				return err
			}
			ctx := cmd.Context()
			nsf, err := featureSelect(ctx, r, ns, featureName)
			if err != nil {
				return err
			}
			ns, featureName = nsf.namespace(), nsf.feature()
			if len(jsonContext) == 0 {
				if err := survey.AskOne(&survey.Input{
					Message: "Context:",
					Help:    "Context for rules evaluation in JSON format, e.g. {\"a\": 1}.",
				}, &jsonContext); err != nil {
					return errors.Wrap(err, "prompt")
				}
			}
			if len(jsonContext) == 0 {
				jsonContext = "{}"
			}
			featureCtx := make(map[string]interface{})
			if err := json.Unmarshal([]byte(jsonContext), &featureCtx); err != nil {
				return err
			}
			fmt.Printf("Evaluating %s with context %s\n", logging.Bold(fmt.Sprintf("%s/%s", ns, featureName)), logging.Bold(jsonContext))
			fmt.Printf("-------------------\n")
			anyVal, fType, path, err := r.Eval(ctx, ns, featureName, featureCtx)
			if err != nil {
				return err
			}
			if len(fType) == 0 {
				// backwards compatibility. we don't have type information (old behavior),
				// so we resort to printing the any type instead of erroring out below.
				fmt.Printf("%s\n", logging.Bold(anyVal))
				return nil
			}
			rootMD, _, err := r.ParseMetadata(ctx)
			if err != nil {
				return errors.Wrap(err, "failed to parse config repo metadata")
			}
			registry, err := r.BuildDynamicTypeRegistry(ctx, rootMD.ProtoDirectory)
			if err != nil {
				return errors.Wrap(err, "failed to build dynamic type registry")
			}
			res, err := feature.AnyToVal(anyVal, fType, registry)
			if err != nil {
				return errors.Wrap(err, "any to val")
			}
			if fType == eval.FeatureTypeJSON {
				valueRes, ok := res.(*structpb.Value)
				if !ok {
					return errors.Errorf("invalid type for %v", res)
				}
				jsonRes, err := valueRes.MarshalJSON()
				if err != nil {
					return err
				}
				res = string(jsonRes)
			}

			fmt.Printf("[%s] %s\n", fType, logging.Bold(res))
			if verbose {
				fmt.Printf("[path] %v\n", path)
			}

			return nil
		},
	}
	cmd.Flags().StringVarP(&ns, "namespace", "n", "", "namespace to remove feature from")
	cmd.Flags().StringVarP(&featureName, "feature", "f", "", "name of feature to remove")
	cmd.Flags().StringVarP(&jsonContext, "context", "c", "", "context to evaluate with in json format")
	cmd.Flags().BoolVarP(&verbose, "verbose", "v", false, "print verbose evaluation information")
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
			if err := confirmInput(name); err != nil {
				return err
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

// Helpful method to ask the user to enter a piece of text before
// doing something irreversible, like deleting something.
func confirmInput(text string) error {
	var inputText string
	if err := survey.AskOne(&survey.Input{
		Message: fmt.Sprintf("Enter '%s' to continue:", text),
	}, &inputText); err != nil {
		return errors.Wrap(err, "prompt")
	}
	if text != inputText {
		return errors.New("incorrect input")
	}
	return nil
}

type namespaceFeature struct {
	ns, featureName string
}

func newNSF(ns, featureName string) *namespaceFeature {
	return &namespaceFeature{
		ns:          ns,
		featureName: featureName,
	}
}

func newNSFFromPair(nsfPair string) (*namespaceFeature, error) {
	parts := strings.Split(nsfPair, "/")
	if len(parts) != 2 {
		return nil, errors.Errorf("invalid namespace/feature: %s", nsfPair)
	}
	return newNSF(parts[0], parts[1]), nil
}

func (nsf *namespaceFeature) namespace() string {
	return nsf.ns
}

func (nsf *namespaceFeature) feature() string {
	return nsf.featureName
}

func (nsf *namespaceFeature) String() string {
	return fmt.Sprintf("%s/%s", nsf.namespace(), nsf.feature())
}

type namespaceFeatures []*namespaceFeature

func (nsfs namespaceFeatures) toOptions() []string {
	var options []string
	for _, nsf := range nsfs {
		options = append(options, nsf.String())
	}
	return options
}

func getNamespaceFeatures(ctx context.Context, r repo.ConfigurationRepository, ns, feature string) (namespaceFeatures, error) {
	if len(ns) > 0 && len(feature) > 0 {
		// namespace and feature already populated
		return []*namespaceFeature{newNSF(ns, feature)}, nil
	}
	nsMDs, err := r.ListNamespaces(ctx)
	if err != nil {
		return nil, err
	}
	var nsfs namespaceFeatures
	for _, nsMD := range nsMDs {
		if len(ns) == 0 || ns == nsMD.Name {
			ffs, err := r.GetFeatureFiles(ctx, nsMD.Name)
			if err != nil {
				return nil, errors.Wrap(err, "get feature files")
			}
			for _, ff := range ffs {
				nsfs = append(nsfs, newNSF(ff.NamespaceName, ff.Name))
			}
		}
	}
	return nsfs, nil
}

func featureSelect(ctx context.Context, r repo.ConfigurationRepository, ns, feature string) (*namespaceFeature, error) {
	nsfs, err := getNamespaceFeatures(ctx, r, ns, feature)
	if err != nil {
		return nil, err
	}
	options := nsfs.toOptions()
	if len(options) == 1 {
		// only 1 option, return it
		return newNSFFromPair(options[0])
	}
	var fPath string
	if err := survey.AskOne(&survey.Select{
		Message: "Feature:",
		Options: options,
	}, &fPath); err != nil {
		return nil, errors.Wrap(err, "prompt")
	}
	return newNSFFromPair(fPath)
}
