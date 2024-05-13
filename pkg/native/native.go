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

package native

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/lainio/err2"
	"github.com/lainio/err2/try"
)

type NativeLang string

var (
	GO NativeLang = "go"
	TS NativeLang = "ts"
)

func DetectNativeLang() (NativeLang, error) {
	// naive check for "known" project types
	if _, err := os.Stat("go.mod"); err == nil {
		return GO, nil
	} else if _, err = os.Stat("package.json"); err == nil {
		return TS, nil
	}
	return "", errors.New("unknown project type, Lekko currently supports Go and NPM projects")
}

func NativeLangFromExt(filename string) (NativeLang, error) {
	ext := filepath.Ext(filename)
	switch ext {
	case ".go":
		return GO, nil
	case ".ts":
		return TS, nil
	}
	return "", fmt.Errorf("unsupported file extension: %v", ext)
}

func (l *NativeLang) Ext() string {
	switch *l {
	case GO:
		return ".go"
	case TS:
		return ".ts"
	}
	return ""
}

func (l *NativeLang) GetNamespace(filename string) (string, error) {
	var ns string
	switch *l {
	case GO:
		// TODO: check that filename == ns too
		ns = filepath.Base(filepath.Dir(filename))
	case TS:
		base := filepath.Base(filename)
		ns = strings.TrimSuffix(base, l.Ext())
	default:
		return "", fmt.Errorf("unsupported language: %v", l)
	}
	if !regexp.MustCompile("[a-z]+").MatchString(ns) {
		return "", fmt.Errorf("namespace must be a lowercase alphanumeric string: %s", ns)
	}
	return ns, nil
}

func ListNativeConfigFiles(lekkoPath string, nativeLang NativeLang) ([]string, error) {
	var files []string
	err := filepath.WalkDir(lekkoPath, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() && (d.Name() == "gen" || d.Name() == "proto") {
			return fs.SkipDir
		}
		if !d.IsDir() &&
			strings.HasSuffix(d.Name(), nativeLang.Ext()) &&
			!strings.HasSuffix(d.Name(), "_gen"+nativeLang.Ext()) {
			//
			files = append(files, path)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return files, nil
}

func ListNamespaces(lekkoPath string, nativeLang NativeLang) (namespaces []string, err error) {
	defer err2.Handle(&err)
	files, err := ListNativeConfigFiles(lekkoPath, nativeLang)
	if err != nil {
		return nil, err
	}
	for _, file := range files {
		namespaces = append(namespaces, try.To1(nativeLang.GetNamespace(file)))
	}
	return namespaces, nil
}
