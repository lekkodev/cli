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
	"io"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"text/tabwriter"
	"time"

	featurev1beta1 "buf.build/gen/go/lekkodev/cli/protocolbuffers/go/lekko/feature/v1beta1"
	"github.com/AlecAivazis/survey/v2"
	"github.com/briandowns/spinner"
	"github.com/iancoleman/strcase"
	"github.com/lekkodev/cli/pkg/feature"
	"github.com/lekkodev/cli/pkg/logging"
	"github.com/lekkodev/cli/pkg/metadata"
	"github.com/lekkodev/cli/pkg/repo"
	"github.com/lekkodev/cli/pkg/secrets"
	"github.com/lekkodev/cli/pkg/star"
	"github.com/lekkodev/cli/pkg/star/static"
	"github.com/lekkodev/go-sdk/pkg/eval"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/types/known/structpb"
)

func featureCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "config",
		Aliases: []string{"feature"},
		Short:   "config management",
	}
	cmd.AddCommand(
		featureList(),
		featureAdd(),
		featureRemove(),
		featureEval(),
		configGroup(),
		generateCmd(),
	)
	return cmd
}

func generateCmd() *cobra.Command {
	var wd string
	var ns string
	var configName string
	cmd := &cobra.Command{
		Use: "gen",
		Short: "generate starlark from proto",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := context.Background()
			r, err := repo.NewLocal(wd, secrets.NewSecretsOrFail())
			if err != nil {
				return err
			}
			rootMD, _, err := r.ParseMetadata(ctx)
			if err != nil {
				return errors.Wrap(err, "failed to parse config repo metadata")
			}
			registry, err := r.ReBuildDynamicTypeRegistry(ctx, rootMD.ProtoDirectory, rootMD.UseExternalTypes)
			if err != nil {
				return errors.Wrap(err, "rebuild type registry")
			}

			// read compiled proto
			configFile := feature.NewFeatureFile(ns, configName)
			var configProto featurev1beta1.Feature

			contents, err := r.GetFileContents(ctx, filepath.Join("", ns, configFile.CompiledJSONFileName))
			if err != nil {
				return err
			}
			err = protojson.UnmarshalOptions{Resolver: registry}.Unmarshal(contents, &configProto)
			if err != nil {
				return err
			}

			// create new config of the same type
			var starBytes []byte
			starImports := make([]*featurev1beta1.ImportStatement, 0)

			if configProto.Type == featurev1beta1.FeatureType_FEATURE_TYPE_PROTO {
				typeUrl := configProto.GetTree().GetDefault().GetTypeUrl()
				messageType, found := strings.CutPrefix(typeUrl, "type.googleapis.com/")
				if !found {
					return fmt.Errorf("can't parse type url: %s", typeUrl)
				}
				starInputs, err := r.BuildProtoStarInputs(ctx, messageType, feature.LatestNamespaceVersion())
				if err != nil {
					return err
				}
				starBytes, err = star.RenderExistingProtoTemplate(*starInputs, feature.LatestNamespaceVersion())
				if err != nil {
					return err
				}
				for importPackage, importAlias := range starInputs.Packages {
					starImports = append(starImports, &featurev1beta1.ImportStatement{
						Lhs: &featurev1beta1.IdentExpr{
							Token: importAlias,
						},
						Operator:  "=",
						Rhs: &featurev1beta1.ImportExpr{
							Dot: &featurev1beta1.DotExpr{
								X:    "proto",
								Name: "package",
							},
							Args: []string{importPackage},
						},
					})
				}
			} else {
				starBytes, err = star.GetTemplate(eval.ConfigTypeFromProto(configProto.Type), feature.LatestNamespaceVersion(), nil)
				if err != nil {
					return err 
				}
			}
					
			// mutate star template with actual config
			walker := static.NewWalker("", starBytes, registry, feature.NamespaceVersionV1Beta7)
			newBytes, err := walker.Mutate(&featurev1beta1.StaticFeature{
				Key:  configProto.Key,
				Type: configProto.GetType(),
				Feature: &featurev1beta1.FeatureStruct{
					Description: configProto.GetDescription(),
				},
				FeatureOld: &configProto,
				Imports:    starImports,
			})
			if err != nil {
				return errors.Wrap(err, "walker mutate")
			}

			fmt.Printf("path: %s",path.Join(ns, configFile.StarlarkFileName))
			if err := r.WriteFile(path.Join(ns, configFile.StarlarkFileName), newBytes, 0600); err != nil {
				return errors.Wrap(err, "write after mutation")
			}
			// fmt.Println(string(newBytes))

			// Final compile
			_, err = r.Compile(ctx, &repo.CompileRequest{
				Registry:        registry,
				NamespaceFilter: ns,
				FeatureFilter: configName,
			})
			if err != nil {
				return errors.Wrap(err, "compile after mutation")
			}
	
			return nil
		},
	}
	cmd.Flags().StringVarP(&ns, "namespace", "n", "", "namespace to add config in")
	cmd.Flags().StringVarP(&configName, "config", "c", "", "name of config to add")
	cmd.Flags().StringVarP(&wd, "config-path", "p", ".", "path to configuration repository")
	return cmd
}

func featureList() *cobra.Command {
	var ns string
	var wd string
	cmd := &cobra.Command{
		Use:   "list",
		Short: "list all configs",
		RunE: func(cmd *cobra.Command, args []string) error {
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
					return errors.Wrapf(err, "get config files for ns %s", namespaceMD.Name)
				}
				for _, ff := range ffs {
					fmt.Printf("%s/%s\n", ff.NamespaceName, ff.Name)
				}
			}
			return nil
		},
	}
	cmd.Flags().StringVarP(&ns, "namespace", "n", "", "name of namespace to filter by")
	cmd.Flags().StringVarP(&wd, "config-path", "c", ".", "path to configuration repository")
	return cmd
}

func featureAdd() *cobra.Command {
	var ns, featureName, fType, fProtoMessage, valueStr, wd string
	cmd := &cobra.Command{
		Use:   "add",
		Short: "add config",
		RunE: func(cmd *cobra.Command, args []string) error {
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
					Message: "Config Name:",
				}, &featureName); err != nil {
					return errors.Wrap(err, "prompt")
				}
			}
			if len(fType) == 0 {
				if err := survey.AskOne(&survey.Select{
					Message: "Config Type:",
					Options: eval.ConfigTypes(),
				}, &fType); err != nil {
					return errors.Wrap(err, "prompt")
				}
			}

			var defaultValue interface{} = nil
			if len(valueStr) != 0 {
				switch fType {
				case string(eval.ConfigTypeBool):
					defaultValue, err = strconv.ParseBool(valueStr)
					if err != nil {
						return errors.Errorf("invalid bool value %s", valueStr)
					}
				case string(eval.ConfigTypeString):
					defaultValue = valueStr
				case string(eval.ConfigTypeInt):
					defaultValue, err = strconv.ParseInt(valueStr, 10, 64)
					if err != nil {
						return errors.Errorf("invalid int value %s", valueStr)
					}
				case string(eval.ConfigTypeFloat):
					defaultValue, err = strconv.ParseFloat(valueStr, 64)
					if err != nil {
						return errors.Errorf("invalid float value %s", valueStr)
					}
				default:
					return errors.Errorf("--value is not supported for type %s", fType)
				}
			}

			if fType == string(eval.ConfigTypeProto) && len(fProtoMessage) == 0 {
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
			if path, err := r.AddFeature(ctx, ns, featureName, eval.ConfigType(fType), fProtoMessage, defaultValue); err != nil {
				return errors.Wrap(err, "add config")
			} else {
				fmt.Printf("Successfully added config %s/%s at path %s\n", ns, featureName, path)
			}
			_, err = r.Compile(ctx, &repo.CompileRequest{})
			return err
		},
	}
	cmd.Flags().StringVarP(&ns, "namespace", "n", "", "namespace to add config in")
	cmd.Flags().StringVarP(&featureName, "feature", "f", "", "name of config to add")
	_ = cmd.Flags().MarkHidden("feature")
	cmd.Flags().StringVarP(&featureName, "config", "c", "", "name of config to add")
	cmd.Flags().StringVarP(&fType, "type", "t", "", "type of config to create")
	cmd.Flags().StringVarP(&fProtoMessage, "proto-message", "m", "", "protobuf message of config to create, if type is proto")
	cmd.Flags().StringVarP(&valueStr, "value", "v", "", "default value of config (not supported for json and proto types)")
	cmd.Flags().StringVar(&wd, "config-path", ".", "path to configuration repository")
	return cmd
}

func featureRemove() *cobra.Command {
	var wd, ns, featureName string
	cmd := &cobra.Command{
		Use:   "remove",
		Short: "remove config",
		RunE: func(cmd *cobra.Command, args []string) error {
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
			fmt.Printf("Deleting config %s...\n", featurePair)
			if err := confirmInput(featurePair); err != nil {
				return err
			}
			if err := r.RemoveFeature(cmd.Context(), ns, featureName); err != nil {
				return errors.Wrap(err, "remove config")
			}
			fmt.Printf("Successfully removed config %s/%s\n", ns, featureName)
			return nil
		},
	}
	cmd.Flags().StringVarP(&ns, "namespace", "n", "", "namespace to remove config from")
	cmd.Flags().StringVarP(&featureName, "feature", "f", "", "name of config to remove")
	_ = cmd.Flags().MarkHidden("feature")
	cmd.Flags().StringVarP(&featureName, "config", "c", "", "name of config to remove")
	cmd.Flags().StringVar(&wd, "config-path", ".", "path to configuration repository") // TODO this is gross maybe make them all full?
	return cmd
}

func featureEval() *cobra.Command {
	var ns, featureName, jsonContext, wd string
	var verbose bool
	cmd := &cobra.Command{
		Use:   "eval",
		Short: "evaluate config",
		RunE: func(cmd *cobra.Command, args []string) error {
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
			fmt.Fprintf(os.Stderr, "Evaluating %s with context %s\n", logging.Bold(fmt.Sprintf("%s/%s", ns, featureName)), logging.Bold(jsonContext))
			fmt.Fprintf(os.Stderr, "-------------------\n")
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
			if fType == eval.ConfigTypeJSON {
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

			fmt.Fprintf(os.Stderr, "[%s] ", fType)
			fmt.Printf("%v", res)
			fmt.Println()
			if verbose {
				fmt.Fprintf(os.Stderr, "[path] %v\n", path)
			}

			return nil
		},
	}
	cmd.Flags().StringVarP(&ns, "namespace", "n", "", "namespace of config to evaluate")
	cmd.Flags().StringVarP(&featureName, "feature", "f", "", "name of config to evaluate")
	_ = cmd.Flags().MarkHidden("feature")
	cmd.Flags().StringVarP(&featureName, "config", "c", "", "name of config to evaluate")
	cmd.Flags().StringVarP(&jsonContext, "context", "t", "", "context to evaluate with in json format")
	cmd.Flags().BoolVarP(&verbose, "verbose", "v", false, "print verbose evaluation information")
	cmd.Flags().StringVar(&wd, "config-path", ".", "path to configuration repository") // TODO this is gross maybe make them all full?
	return cmd
}

func configGroup() *cobra.Command {
	var wd, ns, protoPkg, outName, description string
	var configNames []string
	var disableGenEnum bool
	cmd := &cobra.Command{
		Use:   "group",
		Short: "group multiple configs into 1 config",
		RunE: func(cmd *cobra.Command, args []string) error {
			r, err := repo.NewLocal(wd, secrets.NewSecretsOrFail())
			if err != nil {
				return err
			}
			// Don't output for compilations
			// Downside: for unhappy path, compile errors will be less obvious
			r.ConfigureLogger(&repo.LoggingConfiguration{
				Writer: io.Discard,
			})
			ctx := cmd.Context()
			// Take namespace input if necessary
			if ns == "" {
				nss, err := r.ListNamespaces(ctx)
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
					return errors.Wrap(err, "prompt for namespace")
				}
			}
			allNsfs, err := getNamespaceFeatures(ctx, r, ns, "")
			if err != nil {
				return err
			}
			// Take configs input if necessary
			if len(configNames) == 0 {
				var options []string
				for _, nsf := range allNsfs {
					options = append(options, nsf.featureName)
				}
				sort.Strings(options)
				// NOTE: Currently this doesn't respect selection order
				// so if someone wants a specific order they have to pass the flag explicitly
				if err := survey.AskOne(&survey.MultiSelect{
					Message: "Configs to group:",
					Options: options,
				}, &configNames, survey.WithValidator(survey.MinItems(2))); err != nil {
					return errors.Wrap(err, "prompt for configs")
				}
			}
			if len(configNames) < 2 {
				return errors.New("at least 2 configs must be specified for grouping")
			}
			cMap := make(map[string]struct{})
			for _, nsf := range allNsfs {
				cMap[nsf.featureName] = struct{}{}
			}
			// Check all specified configs exist
			for _, c := range configNames {
				if _, ok := cMap[c]; !ok {
					return errors.Errorf("config %s/%s not found", ns, c)
				}
			}
			// Compile to check for healthy state and get compiled objects (for type info)
			rootMD, nsMDs, err := r.ParseMetadata(ctx)
			if err != nil {
				return errors.Wrap(err, "parse metadata")
			}
			// Can't support grouping if using external proto types
			if rootMD.UseExternalTypes {
				return errors.New("grouping is not supported with external protobuf types")
			}
			registry, err := r.ReBuildDynamicTypeRegistry(ctx, rootMD.ProtoDirectory, rootMD.UseExternalTypes)
			if err != nil {
				return errors.Wrap(err, "rebuild type registry")
			}
			compileResults, err := r.Compile(ctx, &repo.CompileRequest{
				Registry:        registry,
				NamespaceFilter: ns,
			})
			if err != nil {
				return errors.Wrap(err, "pre-group compile")
			}
			var compiledList []*feature.Feature
			compiledMap := make(map[string]*feature.Feature)
			for _, res := range compileResults {
				compiledMap[res.FeatureName] = res.CompiledFeature.Feature
			}
			for _, c := range configNames {
				compiledList = append(compiledList, compiledMap[c])
			}
			// For now, it's required to statically parse configs to get metadata info
			// which we use to determine if a config is an enum config
			// Keep map of (enum) config names to translated enum name and values
			// We'll build up the values lookup map in a later stage
			// TODO: maybe cleanup into a proper type
			enumLookupMap := make(map[string]struct {
				name   string
				values map[string]string
			})
			for _, cn := range configNames {
				sf, err := r.Parse(ctx, ns, cn, registry)
				if err != nil {
					return errors.Wrap(err, "pre-group static parsing")
				}
				c := compiledMap[cn]
				if genEnum, ok := sf.Feature.Metadata.AsMap()["gen-enum"]; ok && c.FeatureType == eval.ConfigTypeString && !disableGenEnum {
					if genEnumBool, ok := genEnum.(bool); ok && genEnumBool {
						enumLookupMap[cn] = struct {
							name   string
							values map[string]string
						}{
							name:   strcase.ToCamel(cn),
							values: make(map[string]string),
						}
					}
				}
			}

			// Prompt for grouped name if necessary, using suggestion engine
			if outName == "" {
				// tab for suggestions?
				if err := survey.AskOne(&survey.Input{
					Message: "Grouped config name:",
					Suggest: func(_ string) []string {
						s := spinner.New(spinner.CharSets[14], 100*time.Millisecond)
						s.Suffix = " Generating AI suggestions..."
						s.Start()
						suggestions := feature.SuggestGroupedNames(compiledList...)
						s.Stop()
						return suggestions
					},
				}, &outName); err != nil {
					return errors.Wrap(err, "prompt for grouped name")
				}
			}
			// Generate new proto message def string based on config types
			// TODO: handle name collisions
			protoMsgName := strcase.ToCamel(outName)
			pdf := feature.NewProtoDefBuilder(protoMsgName)
			protoFieldNames := make([]string, len(configNames))
			for i, cn := range configNames {
				c := compiledMap[cn]
				// Handle "enum" string configs
				if enumLookup, ok := enumLookupMap[cn]; ok {
					pedf := feature.NewProtoEnumDefBuilder(enumLookup.name)
					if defaultVal, ok := c.Value.(string); ok {
						enumLookup.values[defaultVal] = pedf.AddValue(defaultVal)
					}
					for _, o := range c.Overrides {
						if val, ok := o.Value.(string); ok {
							// Check to prevent duplicate values
							if _, ok := enumLookup.values[val]; !ok {
								enumLookup.values[val] = pedf.AddValue(val)
							}
						}
					}
					enumDefStr := pedf.Build()
					pdf.AddEnum(enumDefStr)
					protoFieldNames[i] = pdf.AddField(cn, enumLookup.name, c.Description)
					continue
				}

				pt, err := pdf.ToProtoTypeName(c.FeatureType)
				if err != nil {
					return err
				}
				// Use original description as comment in generated proto
				protoFieldNames[i] = pdf.AddField(cn, pt, c.Description)
			}
			// Update proto file
			protoPath := path.Join(rootMD.ProtoDirectory, strings.ReplaceAll(protoPkg, ".", "/"), "gen.proto")
			if _, err := os.Stat(protoPath); errors.Is(err, os.ErrNotExist) {
				if err := os.MkdirAll(path.Dir(protoPath), 0775); err != nil {
					return errors.Wrap(err, "create destination proto file")
				}
				pf, err := os.Create(protoPath)
				if err != nil {
					return errors.Wrap(err, "create destination proto file")
				}
				defer pf.Close()
				// Write proto file preamble
				if _, err := pf.WriteString(fmt.Sprintf("syntax = \"proto3\";\n\npackage %s;\n\n", protoPkg)); err != nil {
					return errors.Wrap(err, "write preamble to destination proto file")
				}
			}
			pf, err := os.OpenFile(protoPath, os.O_APPEND|os.O_WRONLY, 0644)
			if err != nil {
				return errors.Wrap(err, "open destination proto file for write")
			}
			defer pf.Close()
			if _, err := pf.WriteString(pdf.Build()); err != nil {
				return errors.Wrap(err, "write to destination proto file")
			}
			// Format & rebuild registry for new type
			formatCmd := exec.Command("buf", "format", protoPath, "-w")
			if err := formatCmd.Run(); err != nil {
				return errors.Wrap(err, "buf format")
			}
			registry, err = r.ReBuildDynamicTypeRegistry(ctx, rootMD.ProtoDirectory, rootMD.UseExternalTypes)
			if err != nil {
				return errors.Wrap(err, "rebuild type registry")
			}
			// Create new config using generated proto type
			protoFullName := strings.Join([]string{protoPkg, protoMsgName}, ".")
			_, err = r.AddFeature(ctx, ns, outName, eval.ConfigTypeProto, protoFullName, nil)
			if err != nil {
				return errors.Wrap(err, "add new config")
			}
			compileResults, err = r.Compile(ctx, &repo.CompileRequest{
				Registry:        registry,
				NamespaceFilter: ns,
				FeatureFilter:   outName,
			})
			if err != nil {
				return errors.Wrap(err, "add and compile new config")
			}
			newF := compileResults[0].CompiledFeature.Feature
			// Update default value based on original configs' default values
			// TODO: Handle overrides, handling precedence based on arg order
			mt, err := registry.FindMessageByName(protoreflect.FullName(protoFullName))
			if err != nil {
				return errors.Wrap(err, "find message")
			}
			defaultValue := mt.New()
			for i, cn := range configNames {
				orig := compiledMap[cn]
				// If we converted to an enum, need to set the enum number value accordingly
				if enumLookup, ok := enumLookupMap[cn]; ok {
					if origVal, ok := orig.Value.(string); ok {
						enumDescriptor := mt.Descriptor().Enums().ByName(protoreflect.Name(enumLookup.name))
						if enumDescriptor == nil {
							return errors.Errorf("missing enum descriptor for %s", enumLookup.name)
						}
						valueDescriptor := enumDescriptor.Values().ByName(protoreflect.Name(enumLookup.values[origVal]))
						if valueDescriptor == nil {
							return errors.Errorf("missing enum value for %s", enumLookup.values[origVal])
						}
						defaultValue.Set(mt.Descriptor().Fields().ByName(protoreflect.Name(protoFieldNames[i])), protoreflect.ValueOf(valueDescriptor.Number()))
						continue
					} else {
						return errors.New("unexpected non-string original value")
					}
				}
				defaultValue.Set(mt.Descriptor().Fields().ByName(protoreflect.Name(protoFieldNames[i])), protoreflect.ValueOf(orig.Value))
			}
			newF.Value = defaultValue
			// TODO: Should probably extract this/enum handling logic out for better code org...
			// Handle overrides using the following logic:
			// 1. Flatten out all overrides
			// 2. Scan rules to check if any are identical to a previous one, note
			// 3. Merge overrides with the same rules "upwards"
			// 4. Write into overall overrides
			overrides := make([]*feature.Override, 0)
			ruleFirsts := make(map[string]*feature.Override) // Map of rulestrings to pointers of the first override that has them
			for i, cn := range configNames {
				orig := compiledMap[cn]
				enumLookup, isEnum := enumLookupMap[cn]
				for _, origOverride := range orig.Overrides {
					var origValProto protoreflect.Value
					if isEnum {
						if origVal, ok := origOverride.Value.(string); ok {
							enumDescriptor := mt.Descriptor().Enums().ByName(protoreflect.Name(enumLookup.name))
							if enumDescriptor == nil {
								return errors.Errorf("missing enum descriptor for %s", enumLookup.name)
							}
							valueDescriptor := enumDescriptor.Values().ByName(protoreflect.Name(enumLookup.values[origVal]))
							if valueDescriptor == nil {
								return errors.Errorf("missing enum value for %s", enumLookup.values[origVal])
							}
							origValProto = protoreflect.ValueOf(valueDescriptor.Number())
						} else {
							return errors.New("unexpected non-string original value")
						}
					} else {
						origValProto = protoreflect.ValueOf(origOverride.Value)
					}
					protoField := mt.Descriptor().Fields().ByName(protoreflect.Name(protoFieldNames[i]))
					// Check if an earlier override had this rule, if so merge into it
					ruleFirst, ruleSeen := ruleFirsts[origOverride.Rule]
					if ruleSeen {
						if value, ok := ruleFirst.Value.(protoreflect.Message); ok {
							value.Set(protoField, origValProto)
						} else {
							return errors.Errorf("unexpected type while merging overrides %T", ruleFirst.Value)
						}
					} else {
						value := mt.New()
						value.Set(protoField, origValProto)
						override := &feature.Override{
							Rule:      origOverride.Rule,
							RuleASTV3: origOverride.RuleASTV3,
							Value:     value,
						}
						overrides = append(overrides, override)
						if _, ok := ruleFirsts[origOverride.Rule]; !ok {
							ruleFirsts[origOverride.Rule] = override
						}
						ruleFirsts[override.Rule] = override
					}
				}
			}
			newF.Overrides = overrides
			sf, err := r.Parse(ctx, ns, outName, registry)
			if err != nil {
				return errors.Wrap(err, "parse generated config")
			}
			newFProto, err := newF.ToProto()
			if err != nil {
				return errors.Wrap(err, "convert before mutation")
			}
			newFF, err := r.GetFeatureFile(ctx, ns, outName)
			if err != nil {
				return errors.Wrap(err, "get new config file")
			}
			newFBytes, err := os.ReadFile(path.Join(ns, newFF.StarlarkFileName))
			if err != nil {
				return errors.Wrap(err, "read new config starlark")
			}
			// TODO: description suggestion
			if description == "" {
				description = fmt.Sprintf("Grouped from %s", strings.Join(configNames, ", "))
			}
			newFProto.Description = description
			newFBytes, err = static.NewWalker(newFF.StarlarkFileName, newFBytes, registry, feature.NamespaceVersion(nsMDs[ns].Version)).Mutate(&featurev1beta1.StaticFeature{
				Key:  newFProto.Key,
				Type: newFProto.Type,
				Feature: &featurev1beta1.FeatureStruct{
					Description: newFProto.Description,
				},
				FeatureOld: newFProto,
				Imports:    sf.Imports,
			})
			if err != nil {
				return errors.Wrap(err, "mutate new config")
			}
			if err := r.WriteFile(path.Join(ns, newFF.StarlarkFileName), newFBytes, 0600); err != nil {
				return errors.Wrap(err, "write after mutation")
			}
			// Remove old configs
			for _, c := range configNames {
				if err := r.RemoveFeature(ctx, ns, c); err != nil {
					return errors.Wrapf(err, "remove config %s/%s", ns, c)
				}
			}
			// Final compile
			_, err = r.Compile(ctx, &repo.CompileRequest{
				Registry:        registry,
				NamespaceFilter: ns,
			})
			if err != nil {
				return errors.Wrap(err, "compile after mutation")
			}
			return nil
		},
	}
	// This might not be the cleanest CLI design, but not sure how to do it cleaner
	cmd.Flags().StringVarP(&outName, "out", "o", "", "name of grouped config")
	cmd.Flags().StringVarP(&ns, "namespace", "n", "", "namespace of configs to group together")
	cmd.Flags().StringSliceVarP(&configNames, "configs", "c", []string{}, "comma-separated names of configs to group together")
	cmd.Flags().StringVarP(&protoPkg, "proto-pkg", "p", "default.config.v1beta1", "package for generated protobuf type(s)")
	cmd.Flags().StringVarP(&description, "description", "d", "", "description for the grouped config")
	cmd.Flags().BoolVar(&disableGenEnum, "disable-gen-enum", false, "whether to disable conversion of protobuf enums from string enum configs")
	cmd.Flags().StringVar(&wd, "config-path", ".", "path to configuration repository") // TODO this is gross maybe make them all full?
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
		wd, err := os.Getwd() // TODO
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
	var name, wd string
	cmd := &cobra.Command{
		Use:   "add",
		Short: "add namespace",
		RunE: func(cmd *cobra.Command, args []string) error {
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
	cmd.Flags().StringVar(&wd, "config-path", ".", "path to configuration repository") // TODO this is gross maybe make them all full?
	return cmd
}

func nsRemove() *cobra.Command {
	var name, wd string
	cmd := &cobra.Command{
		Use:   "remove",
		Short: "remove namespace",
		RunE: func(cmd *cobra.Command, args []string) error {
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
	cmd.Flags().StringVar(&wd, "config-path", ".", "path to configuration repository") // TODO this is gross maybe make them all full?
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
		return nil, errors.Errorf("invalid namespace/config: %s", nsfPair)
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
				return nil, errors.Wrap(err, "get config files")
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
		Message: "Config:",
		Options: options,
	}, &fPath); err != nil {
		return nil, errors.Wrap(err, "prompt")
	}
	return newNSFFromPair(fPath)
}
