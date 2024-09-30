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

package sync

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"

	featurev1beta1 "buf.build/gen/go/lekkodev/cli/protocolbuffers/go/lekko/feature/v1beta1"
	"github.com/lekkodev/cli/pkg/feature"
	"github.com/lekkodev/cli/pkg/proto"
	"github.com/lekkodev/cli/pkg/repo"
	"github.com/lekkodev/cli/pkg/star"
	"github.com/lekkodev/cli/pkg/star/prototypes"
	"github.com/lekkodev/cli/pkg/star/static"
	"github.com/lekkodev/go-sdk/pkg/eval"
	"github.com/pkg/errors"
	"google.golang.org/protobuf/reflect/protodesc"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/types/descriptorpb"
	"google.golang.org/protobuf/types/known/anypb"
)

// Writes contents to a local repository at the specified path.
// Conflicting namespaces are essentially completely overwritten.
// Namespaces in the local repository but not in the passed contents are untouched.
// New namespaces are created.
func WriteContentsToLocalRepo(ctx context.Context, contents *featurev1beta1.RepositoryContents, repoPath string) error {
	// NOTE: For now, this function still needs a proper Lekko repository as a prereq,
	// because it's uncertain if we'll ever need functionality to create a local repository
	// from scratch from native lang code for whatever reason
	r, err := repo.NewLocal(repoPath, nil)
	if err != nil {
		return errors.Wrap(err, "prepare repo")
	}
	return WriteContentsToRepo(ctx, contents, r)
}

func WriteContentsToRepo(ctx context.Context, contents *featurev1beta1.RepositoryContents, r repo.ConfigurationRepository) error {
	// Discard logs, mainly for silencing compilation later
	// TODO: Allow passing in writer
	clearFn := r.ConfigureLogger(&repo.LoggingConfiguration{
		Writer:         io.Discard,
		ColorsDisabled: true,
	})
	defer clearFn()
	rootMD, _, err := r.ParseMetadata(ctx)
	if err != nil {
		return err
	}
	typeRegistry, err := proto.FileDescriptorSetToTypeRegistry(contents.FileDescriptorSet)
	if err != nil {
		return errors.Wrap(err, "type registry from contents fds")
	}
	for _, ns := range contents.Namespaces {
		nsExists := false
		// Any configs that were already present but not in incoming contents should be removed
		toRemove := make(map[string]struct{}) // Set of config names in existing namespace
		for _, nsFromMeta := range rootMD.Namespaces {
			if ns.Name == nsFromMeta {
				nsExists = true
				ffs, err := r.GetFeatureFiles(ctx, ns.Name)
				if err != nil {
					return errors.Wrapf(err, "read existing lekkos in namespace %s", ns.Name)
				}
				for _, ff := range ffs {
					toRemove[ff.Name] = struct{}{}
				}
				break
			}
		}
		if !nsExists {
			if err := r.AddNamespace(ctx, ns.Name); err != nil {
				return errors.Wrapf(err, "add namespace %s", ns.Name)
			}
		}

		for _, f := range ns.Features {
			// create a new starlark file from a template (based on the config type)
			var starBytes []byte
			starImports := make([]*featurev1beta1.ImportStatement, 0)
			if f.Type == featurev1beta1.FeatureType_FEATURE_TYPE_PROTO {
				typeURL := f.GetTree().GetDefaultNew().GetTypeUrl()
				messageType, found := strings.CutPrefix(typeURL, "type.googleapis.com/")
				if !found {
					return errors.Errorf("can't parse type url: %s", typeURL)
				}
				starInputs, err := r.BuildProtoStarInputsWithTypes(ctx, messageType, feature.LatestNamespaceVersion(), typeRegistry)
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
				starBytes, err = star.GetTemplate(eval.ConfigTypeFromProto(f.Type), feature.LatestNamespaceVersion(), nil)
				if err != nil {
					return err
				}
			}
			if f.Tree.Default == nil {
				f.Tree.Default = &anypb.Any{
					TypeUrl: f.Tree.DefaultNew.GetTypeUrl(),
					Value:   f.Tree.DefaultNew.GetValue(),
				}
			}
			// mutate starlark with the actual config
			walker := static.NewWalker("", starBytes, typeRegistry, feature.NamespaceVersionV1Beta7)
			newBytes, err := walker.Mutate(&featurev1beta1.StaticFeature{
				Key:  f.Key,
				Type: f.GetType(),
				Feature: &featurev1beta1.FeatureStruct{
					Description: f.GetDescription(),
				},
				FeatureOld: f,
				Imports:    starImports,
			})
			if err != nil {
				return errors.Wrap(err, "walker mutate")
			}
			configFile := feature.NewFeatureFile(ns.Name, f.Key)
			// write starlark to disk
			if err := r.WriteFile(filepath.Join(ns.Name, configFile.StarlarkFileName), newBytes, 0600); err != nil {
				return errors.Wrap(err, "write after mutation")
			}
			delete(toRemove, f.Key)
		}
		// Remove leftovers
		for configName := range toRemove {
			if err := r.RemoveFeature(ctx, ns.Name, configName); err != nil {
				return errors.Wrapf(err, "remove %s/%s", ns.Name, configName)
			}
		}
	}
	// Write types to files & rebuild in-repo type registry
	if err := WriteTypesToRepo(ctx, contents.FileDescriptorSet, r); err != nil {
		return errors.Wrap(err, "write type files")
	}
	// NOTE: This is a workaround to correctly rebuild the type registry in ephemeral repositories.
	// Because ephemeral repos use an in-mem fs, we can't run the buf cmd line on its files.
	// So we copy the proto dir to a temp dir in the real fs, build there, then copy the image back.
	// Still does require the buf CLI to be available.
	tmpProtoDir, err := os.MkdirTemp("", "proto")
	if err != nil {
		return errors.Wrap(err, "create tmp proto dir")
	}
	defer os.RemoveAll(tmpProtoDir)
	if err := copyDirFiles(ctx, r, rootMD.ProtoDirectory, tmpProtoDir); err != nil {
		return errors.Wrap(err, "copy proto files")
	}
	bufIn := filepath.Join(tmpProtoDir, rootMD.ProtoDirectory)
	if err := prototypes.BufLint(bufIn); err != nil {
		return errors.Wrap(err, "lint protos")
	}
	bufImage, err := prototypes.NewBufImage(bufIn)
	if err != nil {
		return errors.Wrap(err, "build protos")
	}
	bufContents, err := os.ReadFile(bufImage.Filename)
	if err != nil {
		return errors.Wrap(err, "read buf image")
	}
	if err := r.WriteFile(filepath.Join(rootMD.ProtoDirectory, filepath.Base(bufImage.Filename)), bufContents, 0644); err != nil {
		return errors.Wrap(err, "copy back buf image")
	}
	if _, err := r.BuildDynamicTypeRegistry(ctx, rootMD.ProtoDirectory); err != nil {
		return errors.Wrap(err, "final build type registry")
	}

	// Final compile to verify overall health
	if _, err := r.Compile(ctx, &repo.CompileRequest{
		IgnoreBackwardsCompatibility: true,
	}); err != nil {
		return errors.Wrap(err, "final compile")
	}

	return nil
}

func WriteTypesToRepo(ctx context.Context, fds *descriptorpb.FileDescriptorSet, r repo.ConfigurationRepository) error {
	rootMD, _, err := r.ParseMetadata(ctx)
	if err != nil {
		return errors.Wrap(err, "parse repository metadata")
	}
	fr, err := protodesc.NewFiles(fds)
	if err != nil {
		return errors.Wrap(err, "convert to file registry")
	}
	var writeErr error
	fr.RangeFiles(func(fd protoreflect.FileDescriptor) bool {
		// Ignore well-known types since they shouldn't be written as files
		if strings.HasPrefix(string(fd.FullName()), "google.protobuf") {
			return true
		}
		// Ignore our types since they shouldn't be written as files
		if strings.HasPrefix(string(fd.FullName()), "lekko.") {
			return true
		}
		contents, err := proto.PrintFileDescriptor(fd)
		if err != nil {
			writeErr = errors.Wrapf(err, "stringify file descriptor %s", fd.FullName())
			return false
		}
		path := filepath.Join(rootMD.ProtoDirectory, fd.Path())
		if err := r.WriteFile(path, []byte(contents), 0600); err != nil {
			writeErr = errors.Wrapf(err, "write to %s", path)
			return false
		}
		return true
	})
	return writeErr
}

// Copies a directory in a configuration repository to a directory on the OS filesystem.
// e.g. <dir> -> <dst>/<dir>
func copyDirFiles(ctx context.Context, r repo.ConfigurationRepository, dir string, dst string) error {
	pfs, err := r.GetDirContents(ctx, dir)
	if err != nil {
		return errors.Wrap(err, "get dir contents")
	}
	dstDir := filepath.Join(dst, dir)
	if err := os.MkdirAll(dstDir, 0777); err != nil {
		return errors.Wrapf(err, "mkdir %s", dstDir)
	}
	for _, pf := range pfs {
		dstPath := filepath.Join(dst, pf.Path)
		if pf.IsDir {
			if err := os.MkdirAll(dstPath, 0777); err != nil {
				return errors.Wrapf(err, "mkdir %s", pf.Path)
			}
			if err := copyDirFiles(ctx, r, pf.Path, dst); err != nil {
				return err
			}
			continue
		}
		b, err := r.Read(pf.Path)
		if err != nil {
			return errors.Wrapf(err, "read %s", pf.Path)
		}
		f, err := os.Create(dstPath)
		if err != nil {
			return errors.Wrapf(err, "create %s", dstPath)
		}
		if _, err := f.Write(b); err != nil {
			return errors.Wrapf(err, "copy %s", dstPath)
		}
	}
	return nil
}
