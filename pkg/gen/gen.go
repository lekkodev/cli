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

package gen

import (
	"context"
	"os"
	"path/filepath"
	"sort"

	featurev1beta1 "buf.build/gen/go/lekkodev/cli/protocolbuffers/go/lekko/feature/v1beta1"
	"golang.org/x/mod/modfile"
	"google.golang.org/protobuf/proto"

	"github.com/lainio/err2"
	"github.com/lainio/err2/try"
	"github.com/pkg/errors"

	"github.com/lekkodev/cli/pkg/dotlekko"
	"github.com/lekkodev/cli/pkg/native"
	"github.com/lekkodev/cli/pkg/repo"
)

type GenOptions struct {
	CodeRepoPath string
	Namespaces   []string
	InitMode     bool
}

func GenAuto(ctx context.Context, configRepoPath, codeRepoPath string) (err error) {
	defer err2.Handle(&err)
	project := try.To1(native.DetectNativeLang(codeRepoPath))
	dot := try.To1(dotlekko.ReadDotLekko(codeRepoPath))
	return GenNative(ctx, project, dot.LekkoPath, configRepoPath, GenOptions{CodeRepoPath: codeRepoPath})
}

func GenNative(ctx context.Context, project *native.Project, lekkoPath, repoPath string, opts GenOptions) (err error) {
	defer err2.Handle(&err)

	absLekkoPath := filepath.Join(opts.CodeRepoPath, lekkoPath)
	err = os.MkdirAll(absLekkoPath, 0770)
	if err != nil {
		return errors.Wrap(err, "create output dir")
	}
	r, err := repo.NewLocal(repoPath, nil)
	if err != nil {
		return errors.Wrap(err, "read local repository")
	}

	if len(opts.Namespaces) == 0 {
		// Try to generate from only existing namespaces in code
		opts.Namespaces = try.To1(native.ListNamespaces(absLekkoPath, project.Language))
		// If no existing namespaces in code, generate from all namespaces in config repo
		if len(opts.Namespaces) == 0 {
			nsMDs, err := r.ListNamespaces(ctx)
			if err != nil {
				return errors.Wrap(err, "list namespaces")
			}
			for _, nsMD := range nsMDs {
				opts.Namespaces = append(opts.Namespaces, nsMD.Name)
			}
		}
	}

	switch project.Language {
	case native.LangTypeScript:
		if opts.InitMode {
			return errors.New("init mode not supported for TS")
		}
		for _, ns := range opts.Namespaces {
			outFilename := filepath.Join(absLekkoPath, ns+project.Language.Ext())
			try.To(genFormattedTS(ctx, repoPath, ns, outFilename))
		}
		return nil
	case native.LangGo:
		return genFormattedGo(ctx, project, repoPath, lekkoPath, opts)
	default:
		return errors.New("Unsupported language")
	}
}

// TODO: move to golang and split repo/repoless
func genFormattedGo(ctx context.Context, project *native.Project, repoPath, lekkoPath string, opts GenOptions) (err error) {
	defer err2.Handle(&err)
	var moduleRoot string
	switch m := project.Metadata.(type) {
	case native.GoMetadata:
		moduleRoot = m.ModulePath
	default:
		b := try.To1(os.ReadFile(filepath.Join(opts.CodeRepoPath, "go.mod")))
		mf := try.To1(modfile.ParseLax("go.mod", b, nil))
		moduleRoot = mf.Module.Mod.Path
	}
	outputPath := filepath.Join(opts.CodeRepoPath, lekkoPath)
	generator := try.To1(NewGoGeneratorFromLocal(ctx, moduleRoot, outputPath, lekkoPath, repoPath))
	for _, namespace := range opts.Namespaces {
		if opts.InitMode {
			try.To(generator.Init(ctx, namespace))
		} else {
			try.To(generator.Gen(ctx, namespace))
		}
	}
	return nil
}

// Reads repository contents from a local config repository.
func ReadRepoContents(ctx context.Context, repoPath string) (repoContents *featurev1beta1.RepositoryContents, err error) {
	defer err2.Handle(&err)
	r, err := repo.NewLocal(repoPath, nil)
	if err != nil {
		return nil, errors.Wrap(err, "read config repository")
	}
	rootMD, nsMDs := try.To2(r.ParseMetadata(ctx))
	repoContents = &featurev1beta1.RepositoryContents{}
	repoContents.FileDescriptorSet = try.To1(r.GetFileDescriptorSet(ctx, rootMD.ProtoDirectory))
	for nsName := range nsMDs {
		ns := &featurev1beta1.Namespace{Name: nsName}
		ffs, err := r.GetFeatureFiles(ctx, nsName)
		if err != nil {
			return nil, errors.Wrapf(err, "read files for ns %s", nsName)
		}
		// Sort configs in alphabetical order
		sort.SliceStable(ffs, func(i, j int) bool {
			return ffs[i].CompiledProtoBinFileName < ffs[j].CompiledProtoBinFileName
		})
		for _, ff := range ffs {
			fc, err := r.GetFeatureContents(ctx, nsName, ff.Name)
			if err != nil {
				return nil, errors.Wrapf(err, "read contents for %s/%s", nsName, ff.Name)
			}
			f := &featurev1beta1.Feature{}
			if err := proto.Unmarshal(fc.Proto, f); err != nil {
				return nil, errors.Wrapf(err, "unmarshal %s/%s", nsName, ff.Name)
			}
			ns.Features = append(ns.Features, f)
		}
		repoContents.Namespaces = append(repoContents.Namespaces, ns)
	}
	return repoContents, nil
}
