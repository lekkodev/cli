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
	"github.com/lekkodev/cli/pkg/native"
	"github.com/pkg/errors"
)

func GenNative(ctx context.Context, nativeLang native.NativeLang, lekkoPath, repoPath, namespace, nativeRepoPath string, initMode bool) (err error) {
	defer err2.Handle(&err)

	absLekkoPath := filepath.Join(nativeRepoPath, lekkoPath)
	err = os.MkdirAll(absLekkoPath, 0770)
	if err != nil {
		return errors.Wrap(err, "create output dir")
	}

	var namespaces []string
	if len(namespace) == 0 {
		namespaces = try.To1(native.ListNamespaces(absLekkoPath, nativeLang))
	} else {
		namespaces = []string{namespace}
	}

	switch nativeLang {
	case native.TS:
		if initMode {
			return errors.New("init mode not supported for TS")
		}
		for _, ns := range namespaces {
			outFilename := filepath.Join(absLekkoPath, ns+nativeLang.Ext())
			try.To(GenFormattedTS(ctx, repoPath, ns, outFilename))
		}
		return nil
	case native.GO:
		return genFormattedGo(ctx, namespaces, repoPath, absLekkoPath, lekkoPath, initMode)
	default:
		return errors.New("Unsupported language")
	}
}

func genFormattedGo(ctx context.Context, namespaces []string, repoPath, outputPath, lekkoPath string, initMode bool) (err error) {
	defer err2.Handle(&err)
	b := try.To1(os.ReadFile("go.mod"))
	mf := try.To1(modfile.ParseLax("go.mod", b, nil))
	for _, namespace := range namespaces {
		generator := try.To1(NewGoGenerator(mf.Module.Mod.Path, outputPath, lekkoPath, repoPath, namespace))
		if initMode {
			try.To(generator.Init(ctx))
		} else {
			try.To(generator.Gen(ctx))
		}
	}
	return nil
}
