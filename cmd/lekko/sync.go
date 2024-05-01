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
	"io"
	"io/fs"
	"os"
	"path"
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
		Use:   "go path/to/lekko/file.go",
		Short: "sync a Go file with Lekko config functions to a local config repository",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
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
			f = args[0]
			syncer, err := sync.NewGoSyncer(ctx, mf.Module.Mod.Path, f, repoPath)
			if err != nil {
				return errors.Wrap(err, "initialize code syncer")
			}
			r, err := repo.NewLocal(repoPath, nil)
			if err != nil {
				return err
			}
			return syncer.Sync(ctx, r)
		},
	}
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

func getRegistryAndNamespacesFromBff(ctx context.Context) (map[string]map[string]*featurev1beta1.Feature, *protoregistry.Types, error) {
	rs := secrets.NewSecretsFromEnv()
	bff := lekko.NewBFFClient(rs)
	resp, err := bff.GetRepositoryContents(ctx, connect_go.NewRequest(&bffv1beta1.GetRepositoryContentsRequest{
		Key: &bffv1beta1.BranchKey{
			OwnerName: "lekkodev",
			RepoName:  "internal",
			Name:      "main",
		},
	}))
	if err != nil {
		return nil, nil, err
	}
	//fmt.Printf("%s\n", resp.Msg.Branch.Sha)
	//fmt.Printf("%#v\n", resp.Msg.Branch)
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
	registry, err := GetRegistryFromFileDescriptorSet(resp.Msg.FileDescriptorSet)
	return existing, registry, err
}

func getRegistryAndNamespacesFromLocal(ctx context.Context, repoPath string) (map[string]map[string]*featurev1beta1.Feature, *protoregistry.Types, error) {
	existing := make(map[string]map[string]*featurev1beta1.Feature)
	r, err := repo.NewLocal(repoPath, nil)
	if err != nil {
		return nil, nil, err
	}
	r.ConfigureLogger(&repo.LoggingConfiguration{
		Writer: io.Discard,
	})
	rootMD, _, err := r.ParseMetadata(ctx)
	if err != nil {
		return nil, nil, err
	}
	registry, err := r.BuildDynamicTypeRegistry(ctx, rootMD.ProtoDirectory)
	if err != nil {
		return nil, nil, err
	}
	namespaces, err := r.ListNamespaces(ctx)
	if err != nil {
		return nil, nil, err
	}
	for _, namespace := range namespaces {
		existing[namespace.Name] = make(map[string]*featurev1beta1.Feature)
		ffs, err := r.GetFeatureFiles(ctx, namespace.Name)
		if err != nil {
			return nil, nil, err
		}
		for _, ff := range ffs {
			fff, err := os.ReadFile(path.Join(repoPath, namespace.Name, ff.CompiledProtoBinFileName))
			if err != nil {
				return nil, nil, err
			}
			config := &featurev1beta1.Feature{}
			if err := proto.Unmarshal(fff, config); err != nil {
				return nil, nil, err
			}
			if config.Tree.DefaultNew != nil {
				config.Tree.Default = lekkoAnyToAny(config.Tree.DefaultNew)
			}
			for _, c := range config.Tree.Constraints {
				c.Rule = ""
				if c.GetValueNew() != nil {
					c.Value = lekkoAnyToAny(c.GetValueNew())
				}
			}
			existing[namespace.Name][config.Key] = config
		}
	}
	return existing, registry, nil
}

func isSame(ctx context.Context, existing map[string]map[string]*featurev1beta1.Feature, registry *protoregistry.Types, goRoot string) (bool, error) {
	startingDirectory, err := os.Getwd()
	defer os.Chdir(startingDirectory)
	if err != nil {
		return false, err
	}
	err = os.Chdir(goRoot)
	if err != nil {
		return false, err
	}
	wd, err := os.Getwd()
	if err != nil {
		return false, err
	}
	if err != nil {
		return false, err
	}
	b, err := os.ReadFile("go.mod")
	if err != nil {
		return false, err
	}
	mf, err := modfile.ParseLax("go.mod", b, nil)
	if err != nil {
		return false, err
	}
	files, err := findLekkoFiles(wd + "/internal/lekko")
	if err != nil {
		return false, err
	}
	var notEqual bool
	for _, f := range files {
		relativePath, err := filepath.Rel(wd, f)
		if err != nil {
			return false, err
		}
		//fmt.Printf("%s\n\n", mf.Module.Mod.Path)
		g := sync.NewGoSyncerLite(mf.Module.Mod.Path, relativePath, registry)
		namespace, err := g.FileLocationToNamespace(ctx)
		if err != nil {
			return false, err
		}
		//fmt.Printf("%#v\n", namespace)
		for _, f := range namespace.Features {
			if f.GetTree().GetDefault() != nil {
				f.Tree.DefaultNew = anyToLekkoAny(f.Tree.Default)
			}
			for _, c := range f.GetTree().GetConstraints() {
				if c.GetValue() != nil {
					c.ValueNew = anyToLekkoAny(c.Value)
				}
			}
			existingConfig, ok := existing[namespace.Name][f.Key]
			if !ok {
				// fmt.Print("New Config!\n")
				notEqual = true
			} else if proto.Equal(f.Tree, existingConfig.Tree) {
				// fmt.Print("Equal! - from proto.Equal\n")
			} else {
				// These might still be equal, because the typescript path combines logical things in ways that the go path does not
				// Using ts since it has fewer args..
				gen.TypeRegistry = registry
				o, err := gen.GenTSForFeature(f, namespace.Name, "")
				if err != nil {
					return false, err
				}
				e, err := gen.GenTSForFeature(existingConfig, namespace.Name, "")
				if err != nil {
					return false, err
				}
				if o == e {
					// fmt.Print("Equal! - from codeGen\n")
				} else {
					// fmt.Printf("Not Equal:\n\n%s\n%s\n\n", o, e)
					notEqual = true
				}
			}
		}
	}
	if notEqual {
		return false, nil
	}
	return true, nil
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
	var repoPath, basePath, headPath string
	cmd := &cobra.Command{
		Use:    "diff",
		Short:  "diff",
		Hidden: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()

			/*
				existing, registry, err := getRegistryAndNamespacesFromBff(ctx)
				if err != nil {
					return err
				}
			*/
			existing, registry, err := getRegistryAndNamespacesFromLocal(ctx, repoPath)
			if err != nil {
				return err
			}
			isHeadSame, err := isSame(ctx, existing, registry, headPath)
			if err != nil {
				return err
			}
			isBaseSame, err := isSame(ctx, existing, registry, basePath)
			if err != nil {
				return err
			}
			if !isHeadSame && !isBaseSame {
				return errors.New("Create a PR to fix Base first\n")
			} else if !isHeadSame && isBaseSame {
				return errors.New("Push Head changes to Lekko before Merge\n")
			} else if isHeadSame && !isBaseSame {
				fmt.Print("Merging will make Base = Lekko\n")
				return nil
			} else if isHeadSame && isBaseSame {
				return nil
			}
			return nil
		},
	}
	cmd.Flags().StringVarP(&repoPath, "repo-path", "r", "", "path to config repository, will use autodetect if not set")
	cmd.Flags().StringVarP(&basePath, "base-path", "b", "", "path to head repository")
	cmd.Flags().StringVarP(&headPath, "head-path", "H", "", "path to base repository")
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
