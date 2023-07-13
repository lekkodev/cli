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

	"github.com/lekkodev/cli/pkg/metadata"

	"github.com/AlecAivazis/survey/v2"
	"github.com/lekkodev/cli/pkg/feature"
	"github.com/lekkodev/cli/pkg/logging"
	"github.com/lekkodev/cli/pkg/repo"
	"github.com/lekkodev/cli/pkg/secrets"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"google.golang.org/protobuf/types/known/structpb"
)

func featureCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "feature",
		Short: "Feature management",
	}
	cmd.AddCommand(
		featureListCmd(),
		featureAddCmd(),
		featureRemoveCmd(),
		featureEval(),
	)
	return cmd
}

func featureListCmd() *cobra.Command {
	var isQuiet bool
	var ns string
	cmd := &cobra.Command{
		Short:                 "List all features",
		Use:                   formCmdUse("list", "[namespace]"),
		DisableFlagsInUseLine: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := errIfMoreArgs([]string{"namespace"}, args); err != nil {
				return err
			}
			wd, err := os.Getwd()
			if err != nil {
				return err
			}
			r, err := repo.NewLocal(wd, secrets.NewSecretsOrFail())
			if err != nil {
				return err
			}
			if len(args) == 1 {
				ns = args[0]
			}

			nss, err := r.ListNamespaces(cmd.Context())
			if err != nil {
				return errors.Wrap(err, "list namespaces")
			}
			if len(ns) > 0 {
				found := false
				for _, namespaceMD := range nss {
					if namespaceMD.Name == ns {
						nss = []*metadata.NamespaceConfigRepoMetadata{namespaceMD}
						found = true
						break
					}
				}
				if !found {
					return fmt.Errorf("namespace %s not found", ns)
				}
			}

			s := ""
			for _, namespaceMD := range nss {
				ffs, err := r.GetFeatureFiles(cmd.Context(), namespaceMD.Name)
				if err != nil {
					return errors.Wrapf(err, "get feature files for ns %s", namespaceMD.Name)
				}
				for _, ff := range ffs {
					if !isQuiet {
						s += fmt.Sprintf("%s/%s\n", ff.NamespaceName, ff.Name)
					} else {
						s += fmt.Sprintf("%s/%s ", ff.NamespaceName, ff.Name)
					}
				}
			}
			if isQuiet {
				s = strings.TrimSpace(s)
			}
			printLinef(cmd, s)
			return nil
		},
	}
	cmd.Flags().BoolVarP(&isQuiet, QuietModeFlag, QuietModeFlagShort, QuietModeFlagDVal, QuietModeFlagDescription)
	return cmd
}

func featureAddCmd() *cobra.Command {
	var isDryRun, isQuiet, isCompile bool
	var ns, featureName, fType, path, fProtoMessage string
	cmd := &cobra.Command{
		Short:                 "Add feature, requires namespace/feature_name and the new feature type",
		Use:                   formCmdUse("add", "namespace/feature", "type"),
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

			rArgs, n := getNArgs(2, args)

			if !IsInteractive && n != 2 {
				return errors.New("New feature namespace/name and type are required arguments")
			}

			sArgs, err := splitStrIntoFixedSlice(rArgs[0], "/", 2)
			if err != nil && !IsInteractive {
				return errors.Wrap(err, "wrong format for the namespace/feature arguments")
			}
			if len(sArgs) > 0 {
				ns = sArgs[0]
			} else if !IsInteractive {
				return errors.Wrap(err, "namespace cannot be empty")
			}

			if len(sArgs) > 1 {
				featureName = sArgs[1]
			} else if !IsInteractive {
				return errors.Wrap(err, "feature name cannot be empty")
			}

			if n > 1 {
				fType = rArgs[1]
			} else if !IsInteractive {
				return errors.Wrap(err, "feature type cannot be empty")
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
					Message: "*Namespace:",
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

			if fType == string(feature.FeatureTypeProto) && len(fProtoMessage) == 0 && IsInteractive {
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

			if !isDryRun {
				if path, err = r.AddFeature(cmd.Context(), ns, featureName, feature.FeatureType(fType), fProtoMessage); err != nil {
					return errors.Wrap(err, "add feature")
				}
			} else {
				path = ns + "/" + featureName + ".star"
			}

			if !isQuiet {
				printLinef(cmd, "Added feature %s/%s at path %s\n", ns, featureName, path)
			} else {
				printLinef(cmd, "%s/%s", ns, featureName)
			}
			if !isQuiet && !isDryRun && !isCompile {
				fmt.Printf("Modify the feature, and then run `lekko compile`\n")
			}

			if isCompile {
				if isQuiet {
					r.ConfigureLogger(nil)
				}
				if _, err := r.Compile(cmd.Context(), &repo.CompileRequest{
					NamespaceFilter: ns,
					FeatureFilter:   featureName,
					DryRun:          isDryRun,
					// don't verify file structure, since we may have not yet generated
					// the DSLs for newly added Flags().
				}); err != nil {
					return errors.Wrap(err, "compile")
				}
			}
			return err
		},
	}
	cmd.Flags().BoolVarP(&isQuiet, QuietModeFlag, QuietModeFlagShort, QuietModeFlagDVal, QuietModeFlagDescription)
	cmd.Flags().BoolVarP(&isDryRun, DryRunFlag, DryRunFlagShort, DryRunFlagDVal, DryRunFlagDescription)
	cmd.Flags().StringVarP(&fProtoMessage, "proto-message", "m", "", "protobuf message of feature to create")
	cmd.Flags().BoolVarP(&isCompile, "compile", "c", false, "compile feature after creation")
	return cmd
}

func featureRemoveCmd() *cobra.Command {
	var isDryRun, isQuiet, isForce bool
	var ns, featureName, nsFeature string
	cmd := &cobra.Command{
		Short:                 "Remove feature",
		Use:                   formCmdUse("remove", "namespace/feature"),
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
			if n == 1 {
				_, err := splitStrIntoFixedSlice(rArgs[0], "/", 2)
				if err != nil {
					return errors.Wrap(err, "wrong format for the namespace/feature arguments")
				}

				nsFeature = rArgs[0]
				nsfs, err := getNamespaceFeatures(cmd.Context(), r, "", "")
				if err != nil {
					return err
				}
				found := false
				for _, nsf := range nsfs {
					if nsFeature == nsf.String() {
						ns = nsf.namespace()
						featureName = nsf.featureName
						found = true
						break
					}
				}
				if !found {
					return fmt.Errorf("the %s does not exist", nsFeature)
				}
			} else {
				nsf, err := featureSelect(cmd.Context(), r, ns, featureName)
				if err != nil {
					return err
				}
				ns, featureName = nsf.namespace(), nsf.feature()
			}

			if !isForce {
				featurePair := fmt.Sprintf("%s/%s", ns, featureName)
				fmt.Printf("Deleting feature %s...\n", featurePair)
				if err := confirmInput(featurePair); err != nil {
					return err
				}
			}

			if !isDryRun {
				if err := r.RemoveFeature(cmd.Context(), ns, featureName); err != nil {
					return errors.Wrap(err, "remove feature")
				}
			}

			if !isQuiet {
				printLinef(cmd, "Removed feature %s/%s\n", ns, featureName)
			} else {
				printLinef(cmd, "%s/%s", ns, featureName)
			}
			return nil
		},
	}
	cmd.Flags().BoolVarP(&isQuiet, QuietModeFlag, QuietModeFlagShort, QuietModeFlagDVal, QuietModeFlagDescription)
	cmd.Flags().BoolVarP(&isDryRun, DryRunFlag, DryRunFlagShort, DryRunFlagDVal, DryRunFlagDescription)
	cmd.Flags().BoolVarP(&isForce, ForceFlag, ForceFlagShort, ForceFlagDVal, ForceFlagDescription)
	return cmd
}

func featureEval() *cobra.Command {
	var ns, featureName, nsFeature, jsonContext string
	var isQuiet, isVerbose bool
	cmd := &cobra.Command{
		Short:                 "Evaluate feature",
		Use:                   formCmdUse("eval", "namespace/feature", "context"),
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
			ctx := cmd.Context()
			rArgs, _ := getNArgs(2, args)

			jsonContext = rArgs[1]
			sArgs, err := splitStrIntoFixedSlice(rArgs[0], "/", 2)
			if err != nil {
				if !IsInteractive {
					return errors.Wrap(err, "wrong format for the namespace/feature arguments")
				} else {
					ns = ""
					featureName = ""
				}
			} else {
				ns = sArgs[0]
				featureName = sArgs[1]
			}

			nsFeature = rArgs[0]

			found := false
			nsfs, err := getNamespaceFeatures(cmd.Context(), r, "", "")
			if err != nil {
				return err
			}
			for _, nsf := range nsfs {
				if nsFeature == nsf.String() {
					ns = nsf.namespace()
					featureName = nsf.featureName
					found = true
					break
				}
			}
			if !found {
				if !IsInteractive {
					return fmt.Errorf("the %s does not exist", nsFeature)
				} else {
					//-- reset input
					fmt.Printf("\nns:%s %s\n", ns, featureName)
					ns = ""
					featureName = ""
				}
			}

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
			if !isQuiet {
				fmt.Printf("Evaluating %s with context %s\n", logging.Bold(fmt.Sprintf("%s/%s", ns, featureName)), logging.Bold(jsonContext))
				fmt.Printf("-------------------\n")
			}
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
			if fType == feature.FeatureTypeJSON {
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

			if !isQuiet {
				printLinef(cmd, "[%s] %s\n", fType, logging.Bold(res))
				if isVerbose {
					fmt.Printf("[path] %v\n", path)
				}
			} else {
				printLinef(cmd, "%v", res)
			}
			return nil
		},
	}
	cmd.Flags().BoolVarP(&isQuiet, QuietModeFlag, QuietModeFlagShort, QuietModeFlagDVal, QuietModeFlagDescription)
	cmd.Flags().BoolVarP(&isVerbose, VerboseFlag, VerboseFlagShort, VerboseFlagDVal, VerboseFlagDescription)
	return cmd
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
