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
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strings"

	"github.com/iancoleman/strcase"
	"github.com/lainio/err2"
	"github.com/lainio/err2/try"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protodesc"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/reflect/protoregistry"
	"google.golang.org/protobuf/types/descriptorpb"
	"google.golang.org/protobuf/types/dynamicpb"
	"google.golang.org/protobuf/types/known/anypb"

	bffv1beta1 "buf.build/gen/go/lekkodev/cli/protocolbuffers/go/lekko/bff/v1beta1"
	featurev1beta1 "buf.build/gen/go/lekkodev/cli/protocolbuffers/go/lekko/feature/v1beta1"
	"github.com/lekkodev/cli/pkg/dotlekko"
	"github.com/lekkodev/cli/pkg/gen"
	"github.com/lekkodev/cli/pkg/native"
	"github.com/lekkodev/cli/pkg/repo"
	"github.com/lekkodev/cli/pkg/star/prototypes"
	"github.com/lekkodev/cli/pkg/sync"
)

func syncCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "sync",
		Short: "sync code to config",
	}
	cmd.AddCommand(syncGoCmd())
	cmd.AddCommand(syncTSCmd())
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
			var err error
			if len(repoPath) == 0 {
				repoPath, err = repo.PrepareGithubRepo()
				if err != nil {
					return err
				}
			}
			f = args[0]
			syncer := sync.NewGoSyncer()
			repoContents, err := syncer.Sync(f)
			if err != nil {
				return err
			}
			err = sync.WriteContentsToLocalRepo(ctx, repoContents, repoPath)
			if err != nil {
				return err
			}
			return nil
		},
	}
	cmd.Flags().StringVarP(&repoPath, "repo-path", "r", "", "path to config repository, will use autodetect if not set")
	return cmd
}

func syncTSCmd() *cobra.Command {
	var repoPath string
	cmd := &cobra.Command{
		Use:   "ts filepath1 filepath2 ...",
		Short: "parse TypeScript files with lekkos to a local config repository",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			var err error
			if len(repoPath) == 0 {
				repoPath, err = repo.PrepareGithubRepo()
				if err != nil {
					return err
				}
			}
			var repoContents *featurev1beta1.RepositoryContents
			if len(args) > 0 {
				repoContents, err = sync.SyncTSFiles(args...)
			} else {
				dot, doterr := dotlekko.ReadDotLekko("")
				if doterr != nil {
					return doterr
				}
				repoContents, err = sync.SyncTS(dot.LekkoPath)
			}
			if err != nil {
				return err
			}
			err = sync.WriteContentsToLocalRepo(ctx, repoContents, repoPath)
			if err != nil {
				return err
			}
			return nil
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

/* Leaving in case we get more API focussed
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
*/

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

func isSame(ctx context.Context, existing map[string]map[string]*featurev1beta1.Feature, registry *protoregistry.Types, goRoot string) (result bool, err error) {
	defer err2.Handle(&err)
	startingDirectory, err := os.Getwd()
	defer func() {
		err := os.Chdir(startingDirectory)
		if err != nil {
			panic(err)
		}
	}()
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
	dot := try.To1(dotlekko.ReadDotLekko(""))
	nlProject := try.To1(native.DetectNativeLang(""))
	files := try.To1(native.ListNativeConfigFiles(dot.LekkoPath, nlProject.Language))
	var notEqual bool
	var relPaths []string
	for _, f := range files {
		relPath, err := filepath.Rel(wd, f)
		if err != nil {
			return false, err
		}
		relPaths = append(relPaths, relPath)
	}
	g := sync.NewGoSyncer()
	repoContents, err := g.Sync(relPaths...)
	if err != nil {
		return false, err
	}
	for _, namespace := range repoContents.Namespaces {
		existingNs, ok := existing[namespace.Name]
		if !ok {
			// New namespace not in existing
			return false, nil
		}
		if len(namespace.Features) != len(existingNs) {
			// Mismatched number of configs - perhaps due to addition or removal
			return false, nil
		}
		for _, f := range namespace.Features {
			if f.GetTree().GetDefault() != nil {
				f.Tree.DefaultNew = anyToLekkoAny(f.Tree.Default)
			}
			for _, c := range f.GetTree().GetConstraints() {
				if c.GetValue() != nil {
					c.ValueNew = anyToLekkoAny(c.Value)
				}
			}
			existingConfig, ok := existingNs[f.Key]
			if !ok {
				// fmt.Print("New Config!\n")
				notEqual = true
			} else if proto.Equal(f.Tree, existingConfig.Tree) {
				// fmt.Print("Equal! - from proto.Equal\n")
			} else {
				// These might still be equal, because the typescript path combines logical things in ways that the go path does not
				// Using ts since it has fewer args..
				o, err := gen.GenTSForFeature(f, namespace.Name, "", registry)
				if err != nil {
					return false, err
				}
				e, err := gen.GenTSForFeature(existingConfig, namespace.Name, "", registry)
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

func isSameTS(ctx context.Context, existing map[string]map[string]*featurev1beta1.Feature, registry *protoregistry.Types, root string) (result bool, err error) {
	defer err2.Handle(&err)
	startingDirectory, err := os.Getwd()
	defer func() {
		err := os.Chdir(startingDirectory)
		if err != nil {
			panic(err)
		}
	}()
	if err != nil {
		return false, err
	}
	dot := try.To1(dotlekko.ReadDotLekko(""))
	cmd := exec.Command("npx", "ts-to-proto", "--lekko-dir", dot.LekkoPath) // #nosec G204
	cmd.Dir = root
	nsString, err := cmd.CombinedOutput() // The output format of ts-to-proto changed, but no one is using this right now..
	if err != nil {
		fmt.Println("Error running ts-to-proto")
		return false, err
	}
	var namespaces bffv1beta1.NamespaceContents
	err = protojson.UnmarshalOptions{Resolver: registry}.Unmarshal(nsString, &namespaces)
	if err != nil {
		return false, err
	}
	var notEqual bool
	for _, namespace := range namespaces.Namespaces {
		existingNs, ok := existing[namespace.Name]
		if !ok {
			// New namespace not in existing
			fmt.Println("New namespace not in existing")
			return false, nil
		}
		if len(namespace.Configs) != len(existingNs) {
			// Mismatched number of configs - perhaps due to addition or removal
			fmt.Printf("Mismatched number of configs - perhaps due to addition or removal: %d to %d\n", len(namespace.Configs), len(existingNs))
			return false, nil
		}

		for _, c := range namespace.Configs {
			f := c.StaticFeature
			if f.GetTree().GetDefault() != nil {
				f.Tree.DefaultNew = anyToLekkoAny(f.Tree.Default)
			}
			for _, c := range f.GetTree().GetConstraints() {
				if c.GetValue() != nil {
					c.ValueNew = anyToLekkoAny(c.Value)
				}
			}
			existingConfig, ok := existingNs[f.Key]
			if !ok {
				fmt.Print("New Config!\n")
				notEqual = true
			} else if proto.Equal(f.Tree, existingConfig.Tree) {
				// fmt.Print("Equal! - from proto.Equal\n")
			} else {
				// These might still be equal, because the typescript path combines logical things in ways that the go path does not
				// Using ts since it has fewer args..
				//fmt.Printf("%+v\n\n", f)
				o, err := gen.GenTSForFeature(f, namespace.Name, "", registry)
				if err != nil {
					return false, err
				}
				e, err := gen.GenTSForFeature(existingConfig, namespace.Name, "", registry)
				if err != nil {
					return false, err
				}
				if o == e {
					// fmt.Print("Equal! - from codeGen\n")
				} else {
					fmt.Printf("Not Equal:\n\n%s\n%s\n\n", o, e)
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
	var ts bool
	cmd := &cobra.Command{
		Use:    "diff",
		Short:  "diff",
		Hidden: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()

			sameFunc := isSame
			if ts {
				sameFunc = isSameTS
			}
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
			// TODO: There might be some way to only need to have one clone of the repository
			isHeadSame, err := sameFunc(ctx, existing, registry, headPath)
			if err != nil {
				return err
			}
			isBaseSame, err := sameFunc(ctx, existing, registry, basePath)
			if err != nil {
				return err
			}
			if !isHeadSame && !isBaseSame {
				fmt.Println("Update the base branch to match Lekko")
				os.Exit(1)
			} else if !isHeadSame && isBaseSame {
				fmt.Println("Sync changes from the current branch to Lekko")
				os.Exit(2)
			} else if isHeadSame && !isBaseSame {
				fmt.Println("Merging the current branch will update the base branch to match Lekko")
				return nil
			} else if isHeadSame && isBaseSame {
				fmt.Println("The current branch does not contain any changes to Lekko")
				return nil
			}
			return nil
		},
	}
	cmd.Flags().StringVarP(&repoPath, "repo-path", "r", "", "path to config repository, will use autodetect if not set")
	cmd.Flags().StringVarP(&basePath, "base-path", "b", "", "path to head repository")
	cmd.Flags().StringVarP(&headPath, "head-path", "H", "", "path to base repository")
	cmd.Flags().BoolVar(&ts, "ts", false, "typescript mode")
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

/*
 *  Convert config from one language to another.  This can also be useful for converting to the same language to normalize the syntax.
 *
 *  This handles proto change through go fly a kite
 */
func convertLangCmd() *cobra.Command {
	var inputLang, outputLang, inputFile string
	cmd := &cobra.Command{
		Use:    "convert",
		Short:  "convert",
		Hidden: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			// TODO validate input (is this not built in??)
			f, err := os.ReadFile(inputFile)
			if err != nil {
				panic(err)
			}

			if inputLang == "proto-json" && outputLang == "ts" {
				lines := strings.Split(string(f), "\n")
				out, err := ProtoJSONToTS([]byte(lines[0]), []byte(lines[1]))
				if err != nil {
					panic(err)
				}
				fmt.Println(out)
			} else {
				privateFile := goToGo(ctx, inputFile)
				fmt.Println(privateFile)
			}
			return nil
		},
	}
	cmd.Flags().StringVarP(&inputLang, "input-language", "i", "ts", "go, ts, starlark, proto, proto-json")
	cmd.Flags().StringVarP(&inputFile, "input-file", "I", "/dev/stdin", "input file")
	cmd.Flags().StringVarP(&outputLang, "output-language", "o", "ts", "go, ts, starlark, proto, proto-json")
	return cmd
}

func goToGo(ctx context.Context, filePath string) string {
	syncer := sync.NewGoSyncer()
	repoContents, err := syncer.Sync(filePath)
	if err != nil {
		panic(errors.Wrap(err, "sync"))
	}
	if len(repoContents.Namespaces) != 1 {
		panic("expected 1 namespace")
	}
	namespace := repoContents.Namespaces[0]
	//fmt.Printf("%+v\n", namespace)
	//fmt.Printf("%+v\n", registry.Types)
	//fmt.Print("ON TO GENERATION\n")
	// code gen based off that namespace object
	g, err := gen.NewGoGenerator("", "", "", "", repoContents)
	if err != nil {
		panic(err)
	}
	gn, err := g.GenNamespaceFiles(ctx, namespace.Name, nil)
	if err != nil {
		panic(err)
	}
	return gn.Private
}

func ProtoJSONToTS(nsString []byte, fdString []byte) (string, error) {
	registry, err := prototypes.RegisterDynamicTypes(nil)
	if err != nil {
		panic(err)
	}
	var fileDescriptorProto descriptorpb.FileDescriptorProto
	err = protojson.UnmarshalOptions{Resolver: registry.Types}.Unmarshal(fdString, &fileDescriptorProto)
	if err != nil {
		return "", err
	}
	// This is partly duplicated from pkg/sync/golang:RegisterDescriptor
	fileDescriptor, err := protodesc.NewFile(&fileDescriptorProto, nil)
	if err != nil {
		return "", err
	}
	var interfaceStrings []string

	for i := 0; i < fileDescriptor.Messages().Len(); i++ {
		messageDescriptor := fileDescriptor.Messages().Get(i)
		dynamicMessage := dynamicpb.NewMessage(messageDescriptor)

		err := registry.Types.RegisterMessage(dynamicMessage.Type())
		if err != nil {
			return "", err
		}
		if !strings.HasSuffix(string(messageDescriptor.Name()), "Args") {
			face, err := gen.GetTSInterface(messageDescriptor)
			if err != nil {
				panic(err)
			}
			interfaceStrings = append(interfaceStrings, face+"\n")
		}
	}
	var namespaces bffv1beta1.NamespaceContents
	err = protojson.UnmarshalOptions{Resolver: registry.Types}.Unmarshal(nsString, &namespaces)
	if err != nil {
		return "", err
	}
	var featureStrings []string
	for _, namespace := range namespaces.Namespaces {
		for _, c := range namespace.Configs {
			f := c.StaticFeature
			if f.GetTree().GetDefault() != nil {
				f.Tree.DefaultNew = anyToLekkoAny(f.Tree.Default)
			}
			for _, c := range f.GetTree().GetConstraints() {
				if c.GetValue() != nil {
					c.ValueNew = anyToLekkoAny(c.Value)
				}
			}

			var ourParameters string
			sigType, err := registry.Types.FindMessageByName(protoreflect.FullName(namespace.Name + ".config.v1beta1." + strcase.ToCamel(f.Key) + "Args"))
			if err == nil {
				d := sigType.Descriptor()
				var varNames []string
				var fields []string
				for i := 0; i < d.Fields().Len(); i++ {
					f := d.Fields().Get(i)
					t := gen.FieldDescriptorToTS(f)
					fields = append(fields, fmt.Sprintf("%s?: %s;", strcase.ToLowerCamel(f.TextName()), t))
					varNames = append(varNames, strcase.ToLowerCamel(f.TextName()))
				}

				ourParameters = fmt.Sprintf("{%s}: {%s}", strings.Join(varNames, ", "), strings.Join(fields, " "))
			}

			fs, err := gen.GenTSForFeature(f, namespace.Name, ourParameters, registry.Types)
			featureStrings = append(featureStrings, fs)
			if err != nil {
				return "", err
			}
		}
	}
	return strings.Join(append(interfaceStrings, featureStrings...), "\n"), nil
}
