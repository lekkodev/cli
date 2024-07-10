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

	"golang.org/x/mod/modfile"

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
	for _, namespace := range opts.Namespaces {
		generator := try.To1(NewGoGenerator(moduleRoot, outputPath, lekkoPath, repoPath, namespace))
		if opts.InitMode {
			try.To(generator.Init(ctx))
		} else {
			try.To(generator.Gen(ctx))
		}
	}
	return nil
}
