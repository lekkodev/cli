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
	"io/fs"
	"os"
	"path/filepath"

	bffv1beta1 "buf.build/gen/go/lekkodev/cli/protocolbuffers/go/lekko/bff/v1beta1"
	featurev1beta1 "buf.build/gen/go/lekkodev/cli/protocolbuffers/go/lekko/feature/v1beta1"
	connect_go "github.com/bufbuild/connect-go"
	"github.com/lekkodev/cli/pkg/gen"
	"github.com/lekkodev/cli/pkg/lekko"
	"github.com/lekkodev/cli/pkg/repo"
	"github.com/lekkodev/cli/pkg/secrets"
	"github.com/lekkodev/cli/pkg/star/prototypes"

	"github.com/lekkodev/cli/pkg/sync"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoregistry"
	"google.golang.org/protobuf/types/descriptorpb"
	"google.golang.org/protobuf/types/known/anypb"

	"golang.org/x/mod/modfile"
)

func syncCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "sync",
		Short: "sync code to config",
	}
	cmd.AddCommand(syncGoCmd())
	return cmd
}

func syncGoCmd() *cobra.Command {
	var f string
	var repoPath string
	cmd := &cobra.Command{
		Use:   "go",
		Short: "sync a Go file with Lekko config functions to a local config repository",
		RunE: func(cmd *cobra.Command, args []string) error {
			b, err := os.ReadFile("go.mod")
			if err != nil {
				return errors.Wrap(err, "find go.mod in working directory")
			}
			mf, err := modfile.ParseLax("go.mod", b, nil)
			if err != nil {
				return err
			}

			if len(repoPath) == 0 {
				repoPath, err = repo.PrepareGithubRepo()
				if err != nil {
					return err
				}
			}
			r, err := repo.NewLocal(repoPath, nil)
			if err != nil {
				return err
			}
			syncer, err := sync.NewGoSyncer(cmd.Context(), mf.Module.Mod.Path, f, repoPath)
			if err != nil {
				return err
			}
			return syncer.Sync(cmd.Context(), r)
		},
	}
	cmd.Flags().StringVarP(&f, "file", "f", "lekko.go", "Go file to sync to config repository") // TODO make this less dumb
	cmd.Flags().StringVarP(&repoPath, "repo-path", "r", "", "path to config repository, will use autodetect if not set")
	return cmd
}

func anyToLekkoAny(a *anypb.Any) *featurev1beta1.Any {
	return &featurev1beta1.Any{
		TypeUrl: a.GetTypeUrl(),
		Value:   a.GetValue(),
	}
}

func lekkoAnyToAny(a *featurev1beta1.Any) *anypb.Any {
	return &anypb.Any{
		TypeUrl: a.GetTypeUrl(),
		Value:   a.GetValue(),
	}
}

/*
 * Questions we need answered:
 * Is repo main = lekko main?
 * will repo main be ahead of lekko main if we merge ourselves?
 *
 * Things we need to do:
 * Open a PR with the new code-gen for main
 * Push sync changes to lekko before we merge them
 */

func diffCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "diff",
		Short: "diff",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			rs := secrets.NewSecretsOrFail(secrets.RequireGithub(), secrets.RequireLekko())

			wd, err := os.Getwd()
			if err != nil {
				return err
			}
			b, err := os.ReadFile("go.mod")
			if err != nil {
				return err
			}
			mf, err := modfile.ParseLax("go.mod", b, nil)
			if err != nil {
				return err
			}
			files, err := findLekkoFiles(wd + "/internal/lekko")
			if err != nil {
				return err
			}
			bff := lekko.NewBFFClient(rs)
			resp, err := bff.GetRepositoryContents(ctx, connect_go.NewRequest(&bffv1beta1.GetRepositoryContentsRequest{
				Key: &bffv1beta1.BranchKey{
					OwnerName: "lekkodev",
					RepoName:  "internal",
					Name:      "main",
				},
			}))
			if err != nil {
				return err
			}
			registry, err := GetRegistryFromFileDescriptorSet(resp.Msg.FileDescriptorSet)
			if err != nil {
				return err
			}
			existing := make(map[string]map[string]*featurev1beta1.Feature)
			for _, namespace := range resp.Msg.NamespaceContents.Namespaces {
				existing[namespace.Name] = make(map[string]*featurev1beta1.Feature)
				for _, config := range namespace.Configs {
					if config.StaticFeature.Tree.DefaultNew != nil {
						config.StaticFeature.Tree.Default = lekkoAnyToAny(config.StaticFeature.Tree.DefaultNew)
					}
					for _, c := range config.StaticFeature.Tree.Constraints {
						c.Rule = ""
						if c.GetValueNew() != nil {
							c.Value = lekkoAnyToAny(c.GetValueNew())
						}
					}
					existing[namespace.Name][config.Name] = config.StaticFeature
				}
			}
			for _, f := range files {
				relativePath, err := filepath.Rel(wd, f)
				if err != nil {
					return err
				}
				fmt.Printf("%s\n\n", mf.Module.Mod.Path)
				g := sync.NewGoSyncerLite(mf.Module.Mod.Path, relativePath, registry)
				namespace, err := g.FileLocationToNamespace(ctx)
				if err != nil {
					return err
				}
				fmt.Printf("%#v\n", namespace)
				for _, f := range namespace.Features {
					if f.GetTree().GetDefault() != nil {
						f.Tree.DefaultNew = anyToLekkoAny(f.Tree.Default)
					}
					for _, c := range f.GetTree().GetConstraints() {
						if c.GetValue() != nil {
							c.ValueNew = anyToLekkoAny(c.Value)
						}
					}

					if proto.Equal(f.Tree, existing[namespace.Name][f.Key].Tree) {
						fmt.Print("Equal! - from proto.Equal\n")
					} else {
						// These might still be equal, because the typescript path combines logical things in ways that the go path does not
						// Using ts since it has fewer args..
						gen.TypeRegistry = registry
						o, err := gen.GenTSForFeature(f, namespace.Name, "")
						if err != nil {
							return err
						}
						e, err := gen.GenTSForFeature(existing[namespace.Name][f.Key], namespace.Name, "")
						if err != nil {
							return err
						}
						if o == e {
							fmt.Print("Equal! - from codeGen\n")
						} else {
							fmt.Printf("Not Equal:\n\n%s\n%s\n\n", o, e)
						}
					}
				}
			}

			return nil
		},
	}
	return cmd
}

func GetRegistryFromFileDescriptorSet(fds *descriptorpb.FileDescriptorSet) (*protoregistry.Types, error) {
	b, err := proto.Marshal(fds)
	if err != nil {
		return nil, err
	}
	st, err := prototypes.BuildDynamicTypeRegistryFromBufImage(b)
	if err != nil {
		return nil, err
	}
	return st.Types, nil
}

func findLekkoFiles(lekkoPath string) ([]string, error) {
	files := make([]string, 0)
	if err := filepath.WalkDir(lekkoPath, func(p string, d fs.DirEntry, err error) error {
		if d.IsDir() && d.Name() == "proto" {
			return filepath.SkipDir
		}
		if d.Name() == "lekko.go" {
			files = append(files, p)
		}
		return nil
	}); err != nil {
		return nil, err
	}
	return files, nil
}
