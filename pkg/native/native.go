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
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"

	"golang.org/x/mod/modfile"

	"github.com/lainio/err2"
	"github.com/lainio/err2/try"
	"github.com/pkg/errors"
)

type Metadata interface {
	isMetadata()
}

type GoMetadata struct {
	ModulePath string
}

func (GoMetadata) isMetadata() {}

// Representation of detected information about a native lang code project
type Project struct {
	Language       Language
	PackageManager PackageManager
	// A project can have multiple "frameworks" - e.g. React and Vite
	Frameworks []Framework

	Metadata Metadata
}

type Language string

const (
	LangUnknown    Language = ""
	LangGo         Language = "Go"
	LangTypeScript Language = "TypeScript"
)

type PackageManager string

const (
	PmUnknown PackageManager = ""
	PmNPM     PackageManager = "npm"
	PmYarn    PackageManager = "yarn"
)

type Framework string

const (
	FwUnknown Framework = ""
	FwNode    Framework = "Node"
	FwReact   Framework = "React"
	FwVite    Framework = "Vite"
	FwNext    Framework = "Next.js"
)

func (p *Project) HasFramework(fw Framework) bool {
	return slices.Contains(p.Frameworks, fw)
}

// Check files in a code project to detect native lang information
func DetectNativeLang(codeRepoPath string) (*Project, error) {
	if _, err := os.Stat(filepath.Join(codeRepoPath, "go.mod")); err == nil {
		// For Go, also parse go.mod
		b := try.To1(os.ReadFile(filepath.Join(codeRepoPath, "go.mod")))
		mf := try.To1(modfile.ParseLax("go.mod", b, nil))
		return &Project{
			Language: LangGo,
			Metadata: GoMetadata{ModulePath: mf.Module.Mod.Path},
		}, nil
	} else if _, err = os.Stat(filepath.Join(codeRepoPath, "package.json")); err == nil {
		project := &Project{
			Language:       LangTypeScript,
			PackageManager: PmNPM,
		}

		// Read package.json to see if this is a React project
		pjBytes, err := os.ReadFile(filepath.Join(codeRepoPath, "package.json"))
		if err != nil {
			return nil, errors.Wrap(err, "failed to open package.json")
		}
		pjString := string(pjBytes)
		if strings.Contains(pjString, "react-dom") {
			project.Frameworks = append(project.Frameworks, FwReact)
		} else {
			// if it's not React, assume that it's Node.js backend
			project.Frameworks = append(project.Frameworks, FwNode)
		}
		// Vite config file could be js, cjs, mjs, etc.
		if matches, err := filepath.Glob("vite.config.*"); matches != nil && err == nil {
			project.Frameworks = append(project.Frameworks, FwVite)
		}
		// Next config file could be js, cjs, mjs, etc.
		if matches, err := filepath.Glob("next.config.*"); matches != nil && err == nil {
			project.Frameworks = append(project.Frameworks, FwNext)
		}

		if _, err := os.Stat("yarn.lock"); err == nil {
			project.PackageManager = PmYarn
		}

		return project, nil
	}
	return nil, errors.New("unknown project type, Lekko currently supports Go and NPM projects")
}

func NativeLangFromExt(filename string) (Language, error) {
	ext := filepath.Ext(filename)
	switch ext {
	case ".go":
		return LangGo, nil
	case ".ts":
		return LangTypeScript, nil
	}
	return "", fmt.Errorf("unsupported file extension: %v", ext)
}

func (l *Language) Ext() string {
	switch *l {
	case LangGo:
		return ".go"
	case LangTypeScript:
		return ".ts"
	}
	return ""
}

func (l *Language) GetNamespace(filename string) (string, error) {
	var ns string
	switch *l {
	case LangGo:
		// TODO: check that filename == ns too
		ns = filepath.Base(filepath.Dir(filename))
	case LangTypeScript:
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

func ListNativeConfigFiles(lekkoPath string, lang Language) ([]string, error) {
	var files []string
	err := filepath.WalkDir(lekkoPath, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() && (d.Name() == "gen" || d.Name() == "proto") {
			return fs.SkipDir
		}
		if !d.IsDir() &&
			strings.HasSuffix(d.Name(), lang.Ext()) &&
			!strings.HasSuffix(d.Name(), "_gen"+lang.Ext()) {
			files = append(files, path)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return files, nil
}

func ListNamespaces(lekkoPath string, lang Language) (namespaces []string, err error) {
	defer err2.Handle(&err)
	files, err := ListNativeConfigFiles(lekkoPath, lang)
	if err != nil {
		return nil, err
	}
	for _, file := range files {
		namespaces = append(namespaces, try.To1(lang.GetNamespace(file)))
	}
	return namespaces, nil
}
