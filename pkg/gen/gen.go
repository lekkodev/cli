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
)

type GenOptions struct {
	CodeRepoPath   string
	Namespaces     []string
	InitMode       bool
	NativeMetadata native.Metadata
}

func GenAuto(ctx context.Context, configRepoPath, codeRepoPath string) (err error) {
	defer err2.Handle(&err)
	nativeMetadata, nativeLang := try.To2(native.DetectNativeLang(codeRepoPath))
	dot := try.To1(dotlekko.ReadDotLekko(codeRepoPath))
	return GenNative(ctx, nativeLang, dot.LekkoPath, configRepoPath, GenOptions{CodeRepoPath: codeRepoPath, NativeMetadata: nativeMetadata})
}

func GenNative(ctx context.Context, nativeLang native.NativeLang, lekkoPath, repoPath string, opts GenOptions) (err error) {
	defer err2.Handle(&err)

	absLekkoPath := filepath.Join(opts.CodeRepoPath, lekkoPath)
	err = os.MkdirAll(absLekkoPath, 0770)
	if err != nil {
		return errors.Wrap(err, "create output dir")
	}

	if len(opts.Namespaces) == 0 {
		opts.Namespaces = try.To1(native.ListNamespaces(absLekkoPath, nativeLang))
	}

	switch nativeLang {
	case native.TS:
		if opts.InitMode {
			return errors.New("init mode not supported for TS")
		}
		for _, ns := range opts.Namespaces {
			outFilename := filepath.Join(absLekkoPath, ns+nativeLang.Ext())
			try.To(genFormattedTS(ctx, repoPath, ns, outFilename))
		}
		return nil
	case native.GO:
		return genFormattedGo(ctx, repoPath, lekkoPath, opts)
	default:
		return errors.New("Unsupported language")
	}
}

func genFormattedGo(ctx context.Context, repoPath, lekkoPath string, opts GenOptions) (err error) {
	defer err2.Handle(&err)
	var moduleRoot string
	switch m := opts.NativeMetadata.(type) {
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
