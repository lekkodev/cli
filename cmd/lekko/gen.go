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
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"sort"
	"strings"
	"text/template"

	featurev1beta1 "buf.build/gen/go/lekkodev/cli/protocolbuffers/go/lekko/feature/v1beta1"
	"github.com/lekkodev/cli/pkg/repo"
	"github.com/lekkodev/cli/pkg/secrets"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"golang.org/x/mod/modfile"
	"google.golang.org/protobuf/proto"
)

func genGoCmd() *cobra.Command {
	var ns string
	var wd string
	var of string
	cmd := &cobra.Command{
		Use:   "go",
		Short: "generate Go library code from configs",
		RunE: func(cmd *cobra.Command, args []string) error {
			b, err := os.ReadFile("go.mod")
			if err != nil {
				return err
			}
			mf, err := modfile.ParseLax("go.mod", b, nil)
			if err != nil {
				return err
			}
			moduleRoot := mf.Module.Mod.Path

			rs := secrets.NewSecretsOrFail()
			r, err := repo.NewLocal(wd, rs)
			if err != nil {
				return errors.Wrap(err, "new repo")
			}
			ffs, err := r.GetFeatureFiles(cmd.Context(), ns)
			if err != nil {
				return err
			}
			sort.SliceStable(ffs, func(i, j int) bool {
				return ffs[i].CompiledProtoBinFileName < ffs[j].CompiledProtoBinFileName
			})
			var protoAsByteStrings []string
			var codeStrings []string
			protoImportSet := make(map[string]*protoImport)
			for _, ff := range ffs {
				fff, err := os.ReadFile(wd + "/" + ns + "/" + ff.CompiledProtoBinFileName)
				if err != nil {
					return err
				}
				f := &featurev1beta1.Feature{}
				if err := proto.Unmarshal(fff, f); err != nil {
					return err
				}
				codeString, err := genGoForFeature(f, ns)
				if err != nil {
					return err
				}
				if f.Type == featurev1beta1.FeatureType_FEATURE_TYPE_PROTO {
					protoImport := unpackProtoType(moduleRoot, f.Tree.Default.TypeUrl)
					protoImportSet[protoImport.ImportPath] = &protoImport
				}
				protoAsBytes := fmt.Sprintf("\t\t\"%s\": []byte{", f.Key)
				for idx, b := range fff {
					if idx%16 == 0 {
						protoAsBytes += "\n\t\t\t"
					} else {
						protoAsBytes += " "
					}
					protoAsBytes += fmt.Sprintf("0x%02x,", b)
				}
				protoAsBytes += "\n\t\t},\n"
				protoAsByteStrings = append(protoAsByteStrings, protoAsBytes)
				codeStrings = append(codeStrings, codeString)
			}
			const templateBody = `package lekko{{$.Namespace}}

import (
{{range  $.ProtoImports}}
	{{ . }}{{end}}
	"context"
	client "github.com/lekkodev/go-sdk/client"
)

type LekkoClient struct {
	client.Client
	Close client.CloseFunc
}

type SafeLekkoClient struct {
	client.Client
	Close client.CloseFunc
}

func (c *SafeLekkoClient) GetBool(ctx context.Context, namespace string, key string) bool {
	res, err := c.Client.GetBool(ctx, namespace, key)
	if err != nil {
		panic(err)
	}
	return res
}
func (c *SafeLekkoClient) GetString(ctx context.Context, namespace string, key string) string {
	res, err := c.Client.GetString(ctx, namespace, key)
	if err != nil {
		panic(err)
	}
	return res
}

func (c *SafeLekkoClient) GetFloat(ctx context.Context, key string, namespace string) float64 {
	res, err := c.Client.GetFloat(ctx, namespace, key)
	if err != nil {
		panic(err)
	}
	return res
}

func (c *SafeLekkoClient) GetInt(ctx context.Context, key string, namespace string) int64 {
	res, err := c.Client.GetInt(ctx, namespace, key)
	if err != nil {
		panic(err)
	}
	return res
}

var StaticConfig = map[string]map[string][]byte{
	"{{$.Namespace}}": {
{{range  $.ProtoAsByteStrings}}{{ . }}{{end}}	},
}
{{range  $.CodeStrings}}
{{ . }}{{end}}`

			// buf generate --template '{"version":"v1","plugins":[{"plugin":"go","out":"gen/go"}]}'
			//
			// This generates the code for the config repo, assuming it has a buf.gen.yml in that repo.
			// In OUR repos, and maybe some of our customers, they may already have a buf.gen.yml, so if
			// that is the case, we should identify that, not run code gen (maybe?) and instead need to
			// take the prefix by parsing the buf.gen.yml to understand where the go code goes.
			pCmd := exec.Command(
				"buf",
				"generate",
				fmt.Sprintf(`--template={"managed": {"enabled": true, "go_package_prefix": {"default": "%s/internal/lekko/proto"}}, "version":"v1","plugins":[{"plugin":"go","out":"internal/lekko/proto", "opt": "paths=source_relative"}]}`, moduleRoot),
				"--include-imports",
				wd) // #nosec G204
			pCmd.Dir = "."
			fmt.Println("executing in wd: " + wd + " command: " + pCmd.String())
			if out, err := pCmd.CombinedOutput(); err != nil {
				fmt.Println("this is the error probably")
				fmt.Println(string(out))
				fmt.Println(err)
				return err
			}
			if err := os.MkdirAll("./internal/lekko/"+ns, 0770); err != nil {
				return err
			}
			f, err := os.Create("./internal/lekko/" + ns + "/" + of)
			if err != nil {
				return err
			}
			var protoImports []string
			for _, imp := range protoImportSet {
				protoImports = append(protoImports, fmt.Sprintf(`%s "%s"`, imp.PackageAlias, imp.ImportPath))
			}
			data := struct {
				ProtoImports       []string
				Namespace          string
				ProtoAsByteStrings []string
				CodeStrings        []string
			}{
				protoImports,
				ns,
				protoAsByteStrings,
				codeStrings,
			}
			templ := template.Must(template.New("").Parse(templateBody))
			return templ.Execute(f, data)
		},
	}
	cmd.Flags().StringVarP(&ns, "namespace", "n", "default", "namespace to generate code from")
	cmd.Flags().StringVarP(&wd, "config-path", "c", ".", "path to configuration repository")
	cmd.Flags().StringVarP(&of, "output", "o", "lekko.go", "output file")
	return cmd
}

var genCmd = &cobra.Command{
	Use:   "gen",
	Short: "generate library code from configs",
}

func genGoForFeature(f *featurev1beta1.Feature, ns string) (string, error) {
	const defaultTemplateBody = `// {{$.Description}}
func (c *LekkoClient) {{$.FuncName}}(ctx context.Context) ({{$.RetType}}, error) {
	return c.{{$.GetFunction}}(ctx, "{{$.Namespace}}", "{{$.Key}}")
}

// {{$.Description}}
func (c *SafeLekkoClient) {{$.FuncName}}(ctx context.Context) {{$.RetType}} {
	return c.{{$.GetFunction}}(ctx, "{{$.Namespace}}", "{{$.Key}}")
}
`

	const protoTemplateBody = `// {{$.Description}}
func (c *LekkoClient) {{$.FuncName}}(ctx context.Context) (*{{$.RetType}}, error) {
	result := &{{$.RetType}}{}
	err := c.{{$.GetFunction}}(ctx, "{{$.Namespace}}", "{{$.Key}}", result)
	return result, err
}

// {{$.Description}}
func (c *SafeLekkoClient) {{$.FuncName}}(ctx context.Context) *{{$.RetType}} {
	result := &{{$.RetType}}{}
	c.{{$.GetFunction}}(ctx, "{{$.Namespace}}", "{{$.Key}}", result)
	return result
}
`
	const jsonTemplateBody = `// {{$.Description}}
func (c *LekkoClient) {{$.FuncName}}(ctx context.Context, result interface{}) error {
	return c.{{$.GetFunction}}(ctx, "{{$.Namespace}}", "{{$.Key}}", result)
}

// {{$.Description}}
func (c *SafeLekkoClient) {{$.FuncName}}(ctx context.Context, result interface{}) {
	c.{{$.GetFunction}}(ctx, "{{$.Namespace}}", "{{$.Key}}", result)
}
`
	var funcNameBuilder strings.Builder
	funcNameBuilder.WriteString("Get")
	for _, word := range regexp.MustCompile("[_-]+").Split(f.Key, -1) {
		funcNameBuilder.WriteString(strings.ToUpper(word[:1]) + word[1:])
	}
	funcName := funcNameBuilder.String()
	var retType string
	var getFunction string
	templateBody := defaultTemplateBody
	switch f.Type {
	case 1:
		retType = "bool"
		getFunction = "GetBool"
	case 2:
		retType = "int64"
		getFunction = "GetInt"
	case 3:
		retType = "float64"
		getFunction = "GetFloat"
	case 4:
		retType = "string"
		getFunction = "GetString"
	case 5:
		getFunction = "GetJSON"
		templateBody = jsonTemplateBody
	case 6:
		getFunction = "GetProto"
		templateBody = protoTemplateBody
		// we don't need the import path so sending in empty string
		protoType := unpackProtoType("", f.Tree.Default.TypeUrl)
		// creates configv1beta1.DBConfig
		retType = fmt.Sprintf("%s.%s", protoType.PackageAlias, protoType.Type)
	}

	data := struct {
		Description string
		FuncName    string
		GetFunction string
		RetType     string
		Namespace   string
		Key         string
	}{
		f.Description,
		funcName,
		getFunction,
		retType,
		ns,
		f.Key,
	}
	templ, err := template.New("go func").Parse(templateBody)
	if err != nil {
		return "", err
	}
	var ret bytes.Buffer
	err = templ.Execute(&ret, data)
	return ret.String(), err
}

type protoImport struct {
	PackageAlias string
	ImportPath   string
	Type         string
}

func unpackProtoType(moduleRoot string, typeURL string) protoImport {
	anyURLSplit := strings.Split(typeURL, "/")
	if anyURLSplit[0] != "type.googleapis.com" {
		panic("invalid any type url: " + typeURL)
	}
	// turn default.config.v1beta1.DBConfig into:
	// moduleRoot/internal/lekko/proto/default/config/v1beta1
	typeParts := strings.Split(anyURLSplit[1], ".")

	importPath := strings.Join(append([]string{moduleRoot + "/internal/lekko/proto"}, typeParts[:len(typeParts)-1]...), "/")

	prefix := fmt.Sprintf(`%s%s`, typeParts[len(typeParts)-3], typeParts[len(typeParts)-2])

	// TODO do google.protobuf.X
	switch anyURLSplit[1] {
	case "google.protobuf.Duration":
		importPath = "google.golang.org/protobuf/types/known/durationpb"
		prefix = "durationpb"
	default:
	}

	return protoImport{PackageAlias: prefix, ImportPath: importPath, Type: typeParts[len(typeParts)-1]}
}
