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
	"fmt"
	"log"
	"path"
	"path/filepath"
	"strings"

	featurev1beta1 "buf.build/gen/go/lekkodev/cli/protocolbuffers/go/lekko/feature/v1beta1"

	"github.com/lainio/err2"
	"github.com/lainio/err2/try"
	"github.com/lekkodev/cli/pkg/dotlekko"
	"github.com/lekkodev/cli/pkg/feature"
	"github.com/lekkodev/cli/pkg/gen"
	"github.com/lekkodev/cli/pkg/native"
	"github.com/lekkodev/cli/pkg/repo"
	"github.com/lekkodev/cli/pkg/star"
	"github.com/lekkodev/cli/pkg/star/static"
	"github.com/lekkodev/go-sdk/pkg/eval"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"google.golang.org/protobuf/encoding/protojson"
)

func genCmd() *cobra.Command {
	var lekkoPath, repoPath, ns string
	var initMode bool
	cmd := &cobra.Command{
		Use:   "gen",
		Short: "generate Lekko config functions from a local config repository",
		RunE: func(cmd *cobra.Command, args []string) error {
			meta, nativeLang := try.To2(native.DetectNativeLang(""))
			return genNative(context.Background(), meta, nativeLang, lekkoPath, repoPath, ns, initMode)
		},
	}
	cmd.Flags().StringVarP(&lekkoPath, "lekko-path", "p", "", "Path to Lekko native config files, will use autodetect if not set")
	cmd.Flags().StringVarP(&repoPath, "repo-path", "r", "", "path to config repository, will use autodetect if not set")
	cmd.Flags().StringVarP(&ns, "namespace", "n", "", "namespace to generate code from")
	cmd.Flags().BoolVar(&initMode, "init", false, "pass 'init' to generate boilerplate code for a Lekko namespace")
	cmd.AddCommand(genGoCmd())
	cmd.AddCommand(genTSCmd())
	cmd.AddCommand(genStarlarkCmd())
	return cmd
}

func genNative(ctx context.Context, nativeMetadata native.Metadata, nativeLang native.NativeLang, lekkoPath, repoPath, ns string, initMode bool) (err error) {
	defer err2.Handle(&err)
	if len(lekkoPath) == 0 {
		dot := try.To1(dotlekko.ReadDotLekko(""))
		lekkoPath = dot.LekkoPath
	}
	if len(repoPath) == 0 {
		path, err := repo.PrepareGithubRepo()
		if err != nil {
			log.Fatalf("error preparing github repo %v", err)
		}
		repoPath = path
	}
	opts := gen.GenOptions{
		InitMode:       initMode,
		NativeMetadata: nativeMetadata,
	}
	if len(ns) > 0 {
		opts.Namespaces = []string{ns}
	}
	return gen.GenNative(ctx, nativeLang, lekkoPath, repoPath, opts)
}

func genGoCmd() *cobra.Command {
	var lekkoPath, repoPath, ns string
	var initMode bool
	cmd := &cobra.Command{
		Use:   "go",
		Short: "generate Go library code from configs",
		RunE: func(cmd *cobra.Command, args []string) error {
			return genNative(cmd.Context(), nil, native.GO, lekkoPath, repoPath, ns, initMode)
		},
	}
	cmd.Flags().StringVarP(&lekkoPath, "lekko-path", "p", "", "Path to Lekko native config files, will use autodetect if not set")
	cmd.Flags().StringVarP(&repoPath, "repo-path", "r", "", "path to config repository, will use autodetect if not set")
	cmd.Flags().StringVarP(&ns, "namespace", "n", "", "namespace to generate code from")
	cmd.Flags().BoolVar(&initMode, "init", false, "pass 'init' to generate boilerplate code for a Lekko namespace")
	return cmd
}

func genTSCmd() *cobra.Command {
	var ns string
	var repoPath string
	var lekkoPath string
	cmd := &cobra.Command{
		Use:   "ts",
		Short: "generate typescript library code from configs",
		RunE: func(cmd *cobra.Command, args []string) error {
			return genNative(cmd.Context(), nil, native.TS, lekkoPath, repoPath, ns, false)
		},
	}
	cmd.Flags().StringVarP(&lekkoPath, "lekko-path", "p", "", "path to Lekko native config files, will use autodetect if not set")
	cmd.Flags().StringVarP(&repoPath, "repo-path", "r", "", "path to config repository, will use autodetect if not set")
	cmd.Flags().StringVarP(&ns, "namespace", "n", "default", "namespace to generate code from")
	return cmd
}

func genStarlarkCmd() *cobra.Command {
	var wd string
	var ns string
	var configName string
	cmd := &cobra.Command{
		Use:   "starlark",
		Short: "generate Starlark from the json representation and compile it",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := context.Background()
			r, err := repo.NewLocal(wd, nil)
			if err != nil {
				return errors.Wrap(err, "failed to open config repo")
			}
			rootMD, _, err := r.ParseMetadata(ctx)
			if err != nil {
				return errors.Wrap(err, "failed to parse config repo metadata")
			}
			// re-build proto
			registry, err := r.ReBuildDynamicTypeRegistry(ctx, rootMD.ProtoDirectory, rootMD.UseExternalTypes)
			if err != nil {
				return errors.Wrap(err, "rebuild type registry")
			}

			// check that namespace exists and create it if it doesn't
			nsExists := false
			for _, nsFromMeta := range rootMD.Namespaces {
				if ns == nsFromMeta {
					nsExists = true
					break
				}
			}
			if !nsExists {
				if err := r.AddNamespace(cmd.Context(), ns); err != nil {
					return errors.Wrap(err, "add namespace")
				}
			}

			// read compiled proto from json
			configFile := feature.NewFeatureFile(ns, configName)
			contents, err := r.GetFileContents(ctx, filepath.Join(ns, configFile.CompiledJSONFileName))
			if err != nil {
				return err
			}
			var configProto featurev1beta1.Feature
			err = protojson.UnmarshalOptions{Resolver: registry}.Unmarshal(contents, &configProto)
			if err != nil {
				return err
			}

			// create a new starlark file from a template (based on the config type)
			var starBytes []byte
			starImports := make([]*featurev1beta1.ImportStatement, 0)

			if configProto.Type == featurev1beta1.FeatureType_FEATURE_TYPE_PROTO {
				typeURL := configProto.GetTree().GetDefault().GetTypeUrl()
				messageType, found := strings.CutPrefix(typeURL, "type.googleapis.com/")
				if !found {
					return fmt.Errorf("can't parse type url: %s", typeURL)
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
						Operator: "=",
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

			// mutate starlark with the actual config
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
			// write starlark to disk
			if err := r.WriteFile(path.Join(ns, configFile.StarlarkFileName), newBytes, 0600); err != nil {
				return errors.Wrap(err, "write after mutation")
			}

			// compile newly generated starlark file
			_, err = r.Compile(ctx, &repo.CompileRequest{
				Registry:        registry,
				NamespaceFilter: ns,
				FeatureFilter:   configName,
			})
			if err != nil {
				return errors.Wrap(err, "compile after mutation")
			}

			return nil
		},
	}
	cmd.Flags().StringVarP(&ns, "namespace", "n", "", "namespace to add config in")
	cmd.Flags().StringVarP(&configName, "config", "c", "", "name of config to add")
	// TODO: this doesn't fully work, as it's not propagated everywhere
	// for example `buf lint` when openning the repo
	cmd.Flags().StringVarP(&wd, "repo-path", "r", ".", "path to configuration repository")
	return cmd
}
