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
	rulesv1beta3 "buf.build/gen/go/lekkodev/cli/protocolbuffers/go/lekko/rules/v1beta3"
	"github.com/lainio/err2/try"
	"github.com/lekkodev/cli/pkg/repo"
	"github.com/lekkodev/cli/pkg/secrets"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	strcase "github.com/stoewer/go-strcase"
	"golang.org/x/mod/modfile"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoregistry"
	"google.golang.org/protobuf/types/known/anypb"
	"google.golang.org/protobuf/types/known/structpb"
)

var typeRegistry *protoregistry.Types

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
			rootMD, nsMDs := try.To2(r.ParseMetadata(cmd.Context()))
			// TODO this feels weird and there is a global set we should be able to add to but I'll worrry about it later?
			typeRegistry = try.To1(r.BuildDynamicTypeRegistry(cmd.Context(), rootMD.ProtoDirectory))
			staticCtxType := unpackProtoType(moduleRoot, nsMDs[ns].ContextProto)
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
			if staticCtxType != nil {
				protoImportSet[staticCtxType.ImportPath] = staticCtxType
			}
			for _, ff := range ffs {
				fff, err := os.ReadFile(wd + "/" + ns + "/" + ff.CompiledProtoBinFileName)
				if err != nil {
					return err
				}
				f := &featurev1beta1.Feature{}
				if err := proto.Unmarshal(fff, f); err != nil {
					return err
				}
				codeString, err := genGoForFeature(f, ns, staticCtxType)
				if err != nil {
					return err
				}
				if f.Type == featurev1beta1.FeatureType_FEATURE_TYPE_PROTO {
					protoImport := unpackProtoType(moduleRoot, f.Tree.Default.TypeUrl)
					protoImportSet[protoImport.ImportPath] = protoImport
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
	"golang.org/x/exp/slices"
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

func (c *SafeLekkoClient) GetFloat(ctx context.Context, namespace string, key string) float64 {
	res, err := c.Client.GetFloat(ctx, namespace, key)
	if err != nil {
		panic(err)
	}
	return res
}

func (c *SafeLekkoClient) GetInt(ctx context.Context, namespace string, key string) int64 {
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
{{ . }}
{{end}}`

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
				fmt.Printf("Error when generating code with buf: %s\n %e\n", out, err)
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

func genGoForFeature(f *featurev1beta1.Feature, ns string, staticCtxType *protoImport) (string, error) {
	const defaultTemplateBody = `// {{$.Description}}
func (c *LekkoClient) {{$.FuncName}}(ctx context.Context) ({{$.RetType}}, error) {
	return c.{{$.GetFunction}}(ctx, "{{$.Namespace}}", "{{$.Key}}")
}

// {{$.Description}}
func (c *SafeLekkoClient) {{$.FuncName}}(ctx *{{$.StaticType}}) {{$.RetType}} {
{{range  $.NaturalLanguage}}{{ . }}
{{end}}}`

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

	type StaticContextInfo struct {
		// natural language lines
		Natty             []string
		StaticContextType string
	}
	var staticContextInfo *StaticContextInfo
	var protoType *protoImport
	switch f.Type {
	case featurev1beta1.FeatureType_FEATURE_TYPE_BOOL:
		retType = "bool"
		getFunction = "GetBool"
	case featurev1beta1.FeatureType_FEATURE_TYPE_INT:
		retType = "int64"
		getFunction = "GetInt"
	case featurev1beta1.FeatureType_FEATURE_TYPE_FLOAT:
		retType = "float64"
		getFunction = "GetFloat"
	case featurev1beta1.FeatureType_FEATURE_TYPE_STRING:
		retType = "string"
		getFunction = "GetString"
	case featurev1beta1.FeatureType_FEATURE_TYPE_JSON:
		getFunction = "GetJSON"
		templateBody = jsonTemplateBody
	case featurev1beta1.FeatureType_FEATURE_TYPE_PROTO:
		getFunction = "GetProto"
		//templateBody = protoTemplateBody
		// we don't need the import path so sending in empty string
		protoType = unpackProtoType("", f.Tree.Default.TypeUrl)
		// creates configv1beta1.DBConfig
		retType = fmt.Sprintf("%s.%s", protoType.PackageAlias, protoType.Type)
	}

	if staticCtxType != nil {
		staticContextInfo = &StaticContextInfo{
			Natty:             translateFeature(f, protoType),
			StaticContextType: fmt.Sprintf("%s.%s", staticCtxType.PackageAlias, staticCtxType.Type),
		}
	}

	data := struct {
		Description     string
		FuncName        string
		GetFunction     string
		RetType         string
		Namespace       string
		Key             string
		NaturalLanguage []string
		StaticType      string
	}{
		f.Description,
		funcName,
		getFunction,
		retType,
		ns,
		f.Key,
		[]string{},
		"",
	}
	if staticContextInfo != nil {
		data.NaturalLanguage = staticContextInfo.Natty
		data.StaticType = staticContextInfo.StaticContextType
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

// This function handles both the google.protobuf.Any.TypeURL variable
// which has the format of `types.googleapis.com/fully.qualified.v1beta1.Proto`
// and purely `fully.qualified.v1beta1.Proto`
//
// return nil if typeURL is empty. Panics on any problems like the rest of the file.
func unpackProtoType(moduleRoot string, typeURL string) *protoImport {
	if typeURL == "" {
		return nil
	}
	anyURLSplit := strings.Split(typeURL, "/")
	fqType := anyURLSplit[0]
	if len(anyURLSplit) > 1 {
		if anyURLSplit[0] != "type.googleapis.com" {
			panic("invalid any type url: " + typeURL)
		}
		fqType = anyURLSplit[1]
	}

	// turn default.config.v1beta1.DBConfig into:
	// moduleRoot/internal/lekko/proto/default/config/v1beta1
	typeParts := strings.Split(fqType, ".")

	importPath := strings.Join(append([]string{moduleRoot + "/internal/lekko/proto"}, typeParts[:len(typeParts)-1]...), "/")

	prefix := fmt.Sprintf(`%s%s`, typeParts[len(typeParts)-3], typeParts[len(typeParts)-2])

	// TODO do google.protobuf.X
	switch fqType {
	case "google.protobuf.Duration":
		importPath = "google.golang.org/protobuf/types/known/durationpb"
		prefix = "durationpb"
	default:
	}
	return &protoImport{PackageAlias: prefix, ImportPath: importPath, Type: typeParts[len(typeParts)-1]}
}

func translateFeature(f *featurev1beta1.Feature, protoType *protoImport) []string {
	var buffer []string
	for i, constraint := range f.Tree.Constraints {
		ifToken := "} else if"
		if i == 0 {
			ifToken = "if"
		}
		rule := translateRule(constraint.GetRuleAstNew())
		buffer = append(buffer, fmt.Sprintf("\t%s %s {", ifToken, rule))

		// TODO this doesn't work for proto, but let's try
		buffer = append(buffer, fmt.Sprintf("\t\treturn %s", translateRetValue(constraint.Value, protoType)))
	}
	if len(f.Tree.Constraints) > 0 {
		buffer = append(buffer, "\t}")
	}
	buffer = append(buffer, fmt.Sprintf("\treturn %s", translateRetValue(f.GetTree().GetDefault(), protoType)))
	return buffer
}

func translateRule(rule *rulesv1beta3.Rule) string {
	if rule == nil {
		return ""
	}
	switch v := rule.GetRule().(type) {
	case *rulesv1beta3.Rule_Atom:
		switch v.Atom.GetComparisonOperator() {
		case rulesv1beta3.ComparisonOperator_COMPARISON_OPERATOR_EQUALS:
			return fmt.Sprintf("ctx.%s == %s", strcase.UpperCamelCase(v.Atom.ContextKey), string(try.To1(protojson.Marshal(v.Atom.ComparisonValue))))
		case rulesv1beta3.ComparisonOperator_COMPARISON_OPERATOR_CONTAINED_WITHIN:
			sliceType := "string"
			switch v.Atom.ComparisonValue.GetListValue().GetValues()[0].GetKind().(type) {
			case *structpb.Value_NumberValue:
				// technically doubles may not work for ints....
				sliceType = "float64"
			case *structpb.Value_BoolValue:
				sliceType = "bool"
			case *structpb.Value_StringValue:
				// technically doubles may not work for ints....
				sliceType = "string"
			}
			var elements []string
			for _, comparisonVal := range v.Atom.ComparisonValue.GetListValue().GetValues() {
				elements = append(elements, string(try.To1(protojson.Marshal(comparisonVal))))
			}
			return fmt.Sprintf("slices.Contains([]%s{%s}, ctx.%s)", sliceType, strings.Join(elements, ", "), strcase.UpperCamelCase(v.Atom.ContextKey))
			// TODO, probably logical to have this here but we need slice syntax, use slices as of golang 1.21
		}
	case *rulesv1beta3.Rule_LogicalExpression:
		operator := " && "
		switch v.LogicalExpression.GetLogicalOperator() {
		case rulesv1beta3.LogicalOperator_LOGICAL_OPERATOR_OR:
			operator = " || "
		}
		var result []string
		for _, rule := range v.LogicalExpression.Rules {
			// worry about inner parens later
			result = append(result, translateRule(rule))
		}
		return strings.Join(result, operator)
	}

	return ""
}

func translateRetValue(val *anypb.Any, protoType *protoImport) string {
	// protos
	msg, err := anypb.UnmarshalNew(val, proto.UnmarshalOptions{Resolver: typeRegistry})
	if err != nil {
		panic(err)
	}

	if protoType == nil {
		return string(try.To1(protojson.MarshalOptions{Resolver: typeRegistry}.Marshal(msg)))
	}
	return fmt.Sprintf("&%s.%s{}", protoType.PackageAlias, protoType.Type)
}
