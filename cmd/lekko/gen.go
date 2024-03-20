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
	"context"
	"fmt"
	"go/format"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"text/template"

	featurev1beta1 "buf.build/gen/go/lekkodev/cli/protocolbuffers/go/lekko/feature/v1beta1"
	rulesv1beta3 "buf.build/gen/go/lekkodev/cli/protocolbuffers/go/lekko/rules/v1beta3"
	"github.com/iancoleman/strcase"
	"github.com/lainio/err2/try"
	"github.com/lekkodev/cli/pkg/feature"
	"github.com/lekkodev/cli/pkg/repo"
	"github.com/lekkodev/cli/pkg/secrets"
	"github.com/lekkodev/cli/pkg/star"
	"github.com/lekkodev/cli/pkg/star/static"
	"github.com/lekkodev/go-sdk/pkg/eval"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"golang.org/x/mod/modfile"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/reflect/protoregistry"
	"google.golang.org/protobuf/types/known/anypb"
	"google.golang.org/protobuf/types/known/structpb"
	"google.golang.org/protobuf/types/known/wrapperspb"
)

func genCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "gen",
		Short: "generate library code from configs",
	}
	cmd.AddCommand(genGoCmd())
	cmd.AddCommand(genTSCmd())
	cmd.AddCommand(geneStarlarkCmd())
	return cmd
}

func geneStarlarkCmd() *cobra.Command {
	var wd string
	var ns string
	var configName string
	cmd := &cobra.Command{
		Use:   "starlark",
		Short: "generate Starlark from the json representation and compile it",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := context.Background()
			r, err := repo.NewLocal(wd, secrets.NewSecretsOrFail())
			if err != nil {
				return errors.Wrap(err, "failed to open config repo")
			}
			rootMD, _, err := r.ParseMetadata(ctx)
			if err != nil {
				return errors.Wrap(err, "failed to parse config repo metadata")
			}
			// re-build proto
			registry, err := r.ReBuildDynamicTypeRegistry(ctx, rootMD.ProtoDirectory, rootMD.UseExternalTypes)
			if err != nil {
				return errors.Wrap(err, "rebuild type registry")
			}

			// check that namespace exists and create it if it doesn't
			nsExists := false
			for _, nsFromMeta := range rootMD.Namespaces {
				if ns == nsFromMeta {
					nsExists = true
					break
				}
			}
			if !nsExists {
				if err := r.AddNamespace(cmd.Context(), ns); err != nil {
					return errors.Wrap(err, "add namespace")
				}
			}

			// read compiled proto from json
			configFile := feature.NewFeatureFile(ns, configName)
			contents, err := r.GetFileContents(ctx, filepath.Join(ns, configFile.CompiledJSONFileName))
			if err != nil {
				return err
			}
			var configProto featurev1beta1.Feature
			err = protojson.UnmarshalOptions{Resolver: registry}.Unmarshal(contents, &configProto)
			if err != nil {
				return err
			}

			// create a new starlark file from a template (based on the config type)
			var starBytes []byte
			starImports := make([]*featurev1beta1.ImportStatement, 0)

			if configProto.Type == featurev1beta1.FeatureType_FEATURE_TYPE_PROTO {
				typeURL := configProto.GetTree().GetDefault().GetTypeUrl()
				messageType, found := strings.CutPrefix(typeURL, "type.googleapis.com/")
				if !found {
					return fmt.Errorf("can't parse type url: %s", typeURL)
				}
				starInputs, err := r.BuildProtoStarInputs(ctx, messageType, feature.LatestNamespaceVersion())
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
				starBytes, err = star.GetTemplate(eval.ConfigTypeFromProto(configProto.Type), feature.LatestNamespaceVersion(), nil)
				if err != nil {
					return err
				}
			}

			// mutate starlark with the actual config
			walker := static.NewWalker("", starBytes, registry, feature.NamespaceVersionV1Beta7)
			newBytes, err := walker.Mutate(&featurev1beta1.StaticFeature{
				Key:  configProto.Key,
				Type: configProto.GetType(),
				Feature: &featurev1beta1.FeatureStruct{
					Description: configProto.GetDescription(),
				},
				FeatureOld: &configProto,
				Imports:    starImports,
			})
			if err != nil {
				return errors.Wrap(err, "walker mutate")
			}
			// write starlark to disk
			if err := r.WriteFile(path.Join(ns, configFile.StarlarkFileName), newBytes, 0600); err != nil {
				return errors.Wrap(err, "write after mutation")
			}

			// compile newly generated starlark file
			_, err = r.Compile(ctx, &repo.CompileRequest{
				Registry:        registry,
				NamespaceFilter: ns,
				FeatureFilter:   configName,
			})
			if err != nil {
				return errors.Wrap(err, "compile after mutation")
			}

			return nil
		},
	}
	cmd.Flags().StringVarP(&ns, "namespace", "n", "", "namespace to add config in")
	cmd.Flags().StringVarP(&configName, "config", "c", "", "name of config to add")
	// TODO: this doesn't fully work, as it's not propagated everywhere
	// for example `buf lint` when openning the repo
	cmd.Flags().StringVarP(&wd, "repo-path", "r", ".", "path to configuration repository")
	return cmd
}

var typeRegistry *protoregistry.Types

const StaticBytes = false
const UnSafeClient = false

// Natural language codegen is in super alpha, only handles a subset
// of what is available, namely only supports protos that are one level
// deep with non-repeated primitives, a subset of ruleslang (== and in ops)
// Also doesn't support external types.
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
				codeString := try.To1(genGoForFeature(cmd.Context(), r, f, ns, staticCtxType))
				codeStrings = append(codeStrings, codeString)
				if f.Type == featurev1beta1.FeatureType_FEATURE_TYPE_PROTO {
					protoImport := unpackProtoType(moduleRoot, f.Tree.Default.TypeUrl)
					protoImportSet[protoImport.ImportPath] = protoImport
				}
				if StaticBytes {
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
				}
			}
			// TODOs for the template:
			// proper handling of gofmt for imports, importing slices
			// depending on the go version.
			const templateBody = `package lekko{{$.Namespace}}

import (
	"context"

{{range  $.ProtoImports}}
	{{ . }}{{end}}
	client "github.com/lekkodev/go-sdk/client"
	"golang.org/x/exp/slices"
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

{{if $.StaticConfig}}
var StaticConfig = map[string]map[string][]byte{
	"{{$.Namespace}}": {
{{range  $.ProtoAsByteStrings}}{{ . }}{{end}}	},
}{{end}}{{range  $.CodeStrings}}
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
				StaticConfig       bool
			}{
				protoImports,
				ns,
				protoAsByteStrings,
				codeStrings,
				StaticBytes,
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

func genGoForFeature(ctx context.Context, r repo.ConfigurationRepository, f *featurev1beta1.Feature, ns string, staticCtxType *protoImport) (string, error) {
	const defaultTemplateBody = `{{if $.UnSafeClient}}
// {{$.Description}}
func (c *LekkoClient) {{$.FuncName}}(ctx context.Context) ({{$.RetType}}, error) {
	return c.{{$.GetFunction}}(ctx, "{{$.Namespace}}", "{{$.Key}}")
}
{{end}}
// {{$.Description}}
func (c *SafeLekkoClient) {{$.FuncName}}(ctx *{{$.StaticType}}) {{$.RetType}} {
{{range  $.NaturalLanguage}}{{ . }}
{{end}}}`

	const protoTemplateBody = `{{if $.UnSafeClient}}
// {{$.Description}}
func (c *LekkoClient) {{$.FuncName}}(ctx context.Context) (*{{$.RetType}}, error) {
	result := &{{$.RetType}}{}
	err := c.{{$.GetFunction}}(ctx, "{{$.Namespace}}", "{{$.Key}}", result)
	return result, err
}
{{end}}
// {{$.Description}}
func (c *SafeLekkoClient) {{$.FuncName}}(ctx *{{$.StaticType}}) *{{$.RetType}} {
{{range  $.NaturalLanguage}}{{ . }}
{{end}}}`

	const jsonTemplateBody = `// {{$.Description}}
func (c *LekkoClient) {{$.FuncName}}(ctx context.Context, result interface{}) error {
	return c.{{$.GetFunction}}(ctx, "{{$.Namespace}}", "{{$.Key}}", result)
}

// {{$.Description}}
func (c *SafeLekkoClient) {{$.FuncName}}(ctx context.Context, result interface{}) {
	c.{{$.GetFunction}}(ctx, "{{$.Namespace}}", "{{$.Key}}", result)
}
`

	// Generate an enum type and const declarations
	const stringEnumTemplateBody = `type {{$.EnumTypeName}} string
const (
	{{range $index, $field := $.EnumFields}}{{$field.Name}} {{$.EnumTypeName}} = "{{$field.Value}}"
	{{end}}
)

{{if $.UnSafeClient}}
// {{$.Description}}
func (c *LekkoClient) {{$.FuncName}}(ctx context.Context) ({{$.RetType}}, error) {
	return c.{{$.GetFunction}}(ctx, "{{$.Namespace}}", "{{$.Key}}")
}
{{end}}
// {{$.Description}}
func (c *SafeLekkoClient) {{$.FuncName}}(ctx *{{$.StaticType}}) {{$.RetType}} {
{{range  $.NaturalLanguage}}{{ . }}
{{end}}}`

	var funcNameBuilder strings.Builder
	funcNameBuilder.WriteString("Get")
	for _, word := range regexp.MustCompile("[_-]+").Split(f.Key, -1) {
		funcNameBuilder.WriteString(strings.ToUpper(word[:1]) + word[1:])
	}
	funcName := funcNameBuilder.String()
	var retType string
	var getFunction string
	var enumTypeName string
	type EnumField struct {
		Name  string
		Value string
	}
	var enumFields []EnumField
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
		// HACK: The metadata field is only for presentation at the moment
		// so is not part of the compiled object - need to statically parse
		// This also means that this only works for statically parseable
		// configs
		sf, err := r.Parse(ctx, ns, f.Key, typeRegistry)
		if err != nil {
			return "", errors.Wrap(err, "static parsing")
		}
		fm := sf.Feature.Metadata.AsMap()
		// TODO: This enum codegen does not handle possible conflicts at all
		if genEnum, ok := fm["gen-enum"]; ok {
			if genEnumBool, ok := genEnum.(bool); ok && genEnumBool {
				enumTypeName = strcase.ToCamel(f.Key)
				retType = enumTypeName
				templateBody = stringEnumTemplateBody
				for _, ret := range getStringRetValues(f) {
					// Result of translating ret values is wrapped in quotes
					ret = ret[1 : len(ret)-1]
					name := enumTypeName
					if ret == "" {
						name += "Unspecified"
					} else {
						name += strcase.ToCamel(ret)
					}
					enumFields = append(enumFields, EnumField{
						Name:  name,
						Value: ret,
					})
				}
			}
		}
	case featurev1beta1.FeatureType_FEATURE_TYPE_JSON:
		getFunction = "GetJSON"
		templateBody = jsonTemplateBody
	case featurev1beta1.FeatureType_FEATURE_TYPE_PROTO:
		templateBody = protoTemplateBody
		getFunction = "GetProto"
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
		UnSafeClient    bool
		EnumTypeName    string
		EnumFields      []EnumField
	}{
		f.Description,
		funcName,
		getFunction,
		retType,
		ns,
		f.Key,
		[]string{},
		"",
		UnSafeClient,
		enumTypeName,
		enumFields,
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
	if err != nil {
		return "", err
	}
	// Final canonical Go format
	formatted, err := format.Source(ret.Bytes())
	if err != nil {
		return "", errors.Wrap(err, "format")
	}
	return string(formatted), nil
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
			return fmt.Sprintf("ctx.%s == %s", strcase.ToCamel(v.Atom.ContextKey), string(try.To1(protojson.Marshal(v.Atom.ComparisonValue))))
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
			return fmt.Sprintf("slices.Contains([]%s{%s}, ctx.%s)", sliceType, strings.Join(elements, ", "), strcase.ToCamel(v.Atom.ContextKey))
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
		// TODO we may need more special casing here for primitive types.
		// This feels like horrific syntax, but I needed this because
		// Int64 was somehow serializing to "1" instead of 1, and typechecking
		// doesn't seem to work since `UnmarshalNew` returns a `dynamicpb.Message` which doesn't work with go's type casing.
		if val.MessageIs((*wrapperspb.Int64Value)(nil)) {
			var i64 wrapperspb.Int64Value
			try.To(val.UnmarshalTo(&i64))
			return strconv.FormatInt(i64.Value, 10)
		}
		return string(try.To1(protojson.Marshal(msg)))
	}
	// todo multiline formatting
	var lines []string
	msg.ProtoReflect().Range(func(f protoreflect.FieldDescriptor, val protoreflect.Value) bool {
		valueStr := val.String()
		if val, ok := val.Interface().(string); ok {
			valueStr = fmt.Sprintf(`"%s"`, val)
		}

		lines = append(lines, fmt.Sprintf("%s: %s", strcase.ToCamel(f.TextName()), valueStr))
		return true
	})
	// Replace this with interface pointing stuff
	return fmt.Sprintf("&%s.%s{%s}", protoType.PackageAlias, protoType.Type, strings.Join(lines, ", "))
}

// TODO: Generify
// Get all unique possible return values of a config
func getStringRetValues(f *featurev1beta1.Feature) []string {
	if f.Type != featurev1beta1.FeatureType_FEATURE_TYPE_STRING {
		return []string{}
	}
	valSet := make(map[string]bool)
	valSet[translateRetValue(f.Tree.Default, nil)] = true
	for _, constraint := range f.Tree.Constraints {
		ret := translateRetValue(constraint.Value, nil)
		valSet[ret] = true
	}
	var rets []string
	for val := range valSet {
		rets = append(rets, val)
	}
	sort.Strings(rets)
	return rets
}

func getTSInterface(d protoreflect.MessageDescriptor) (string, error) {
	const templateBody = `export interface {{$.Name}} {
{{range  $.Fields}}    {{ . }}
{{end}}}`

	var fields []string
	for i := 0; i < d.Fields().Len(); i++ {
		f := d.Fields().Get(i)
		var t string
		switch f.Kind() {
		case protoreflect.StringKind:
			t = "string"
		case protoreflect.BoolKind:
			t = "boolean"
		case protoreflect.DoubleKind:
			t = "number"
		case protoreflect.Int64Kind:
			t = "number"
		case protoreflect.EnumKind:
			t = "string"
		case protoreflect.MessageKind:
			t = string(f.Message().Name())
			// TODO add more
		default:
			t = f.Kind().String()
		}
		if f.Cardinality() == protoreflect.Repeated {
			t = "[]" + t
		}
		fields = append(fields, fmt.Sprintf("%s: %s;", f.TextName(), t))
	}

	data := struct {
		Name   string
		Fields []string
	}{
		string(d.Name()),
		fields,
	}
	templ, err := template.New("go func").Parse(templateBody)
	if err != nil {
		return "", err
	}
	var ret bytes.Buffer
	err = templ.Execute(&ret, data)
	if err != nil {
		return "", err
	}
	return ret.String(), nil
}

func getTSParameters(d protoreflect.MessageDescriptor) string {
	var fields []string
	for i := 0; i < d.Fields().Len(); i++ {
		f := d.Fields().Get(i)
		fields = append(fields, f.TextName())
	}

	return fmt.Sprintf("{%s}: %s", strings.Join(fields, ", "), d.Name())
}

func genTSCmd() *cobra.Command {
	var ns string
	var wd string
	var of string
	cmd := &cobra.Command{
		Use:   "ts",
		Short: "generate typescript library code from configs",
		RunE: func(cmd *cobra.Command, args []string) error {
			rs := secrets.NewSecretsOrFail()
			r, err := repo.NewLocal(wd, rs)
			if err != nil {
				return errors.Wrap(err, "new repo")
			}
			rootMD, nsMDs := try.To2(r.ParseMetadata(cmd.Context()))
			typeRegistry = try.To1(r.BuildDynamicTypeRegistry(cmd.Context(), rootMD.ProtoDirectory))

			var parameters string
			if len(nsMDs[ns].ContextProto) > 0 {
				ptype, err := typeRegistry.FindMessageByName(protoreflect.FullName(nsMDs[ns].ContextProto))
				if err != nil {
					return err
				}
				parameters = getTSParameters(ptype.Descriptor())
			}
			//TODO Handle no context proto (make signatures) ... maybe... or we just make people make a static context

			var codeStrings []string

			typeRegistry.RangeMessages(func(mt protoreflect.MessageType) bool {
				splitName := strings.Split(string(mt.Descriptor().FullName()), ".")
				if splitName[0] == "google" {
					return true
				}
				face, err := getTSInterface(mt.Descriptor())
				if err != nil {
					panic(err)
				}
				codeStrings = append(codeStrings, face)
				return true
			})

			ffs, err := r.GetFeatureFiles(cmd.Context(), ns)
			if err != nil {
				return err
			}
			sort.SliceStable(ffs, func(i, j int) bool {
				return ffs[i].CompiledProtoBinFileName < ffs[j].CompiledProtoBinFileName
			})
			for _, ff := range ffs {
				fff, err := os.ReadFile(wd + "/" + ns + "/" + ff.CompiledProtoBinFileName)
				if err != nil {
					return err
				}
				f := &featurev1beta1.Feature{}
				if err := proto.Unmarshal(fff, f); err != nil {
					return err
				}
				codeString, err := genTSForFeature(f, ns, parameters)
				if err != nil {
					return err
				}
				codeStrings = append(codeStrings, codeString)
			}
			const templateBody = `{{range  $.CodeStrings}}
{{ . }}{{end}}`

			data := struct {
				Namespace   string
				CodeStrings []string
			}{
				ns,
				codeStrings,
			}
			if len(of) == 0 {
				of = ns
			}
			f, err := os.Create(of + ".ts")
			if err != nil {
				return err
			}
			templ := template.Must(template.New("").Parse(templateBody))
			return templ.Execute(f, data)
		},
	}
	cmd.Flags().StringVarP(&ns, "namespace", "n", "default", "namespace to generate code from")
	cmd.Flags().StringVarP(&wd, "config-path", "c", ".", "path to configuration repository")
	cmd.Flags().StringVarP(&of, "output", "o", "", "output file")
	return cmd
}

func genTSForFeature(f *featurev1beta1.Feature, ns string, parameters string) (string, error) {
	const templateBody = `// {{$.Description}}
export async function {{$.FuncName}}({{$.Parameters}}): Promise<{{$.RetType}}> {
{{range  $.NaturalLanguage}}{{ . }}
{{end}}}`

	var funcNameBuilder strings.Builder
	funcNameBuilder.WriteString("get")
	for _, word := range regexp.MustCompile("[_-]+").Split(f.Key, -1) {
		funcNameBuilder.WriteString(strings.ToUpper(word[:1]) + word[1:])
	}
	funcName := funcNameBuilder.String()
	var retType string

	switch f.Type {
	case featurev1beta1.FeatureType_FEATURE_TYPE_BOOL:
		retType = "boolean"
	case featurev1beta1.FeatureType_FEATURE_TYPE_INT:
		retType = "number"
	case featurev1beta1.FeatureType_FEATURE_TYPE_FLOAT:
		retType = "number"
	case featurev1beta1.FeatureType_FEATURE_TYPE_STRING:
		retType = "string"
	case featurev1beta1.FeatureType_FEATURE_TYPE_JSON:
		retType = "any" // TODO
	case featurev1beta1.FeatureType_FEATURE_TYPE_PROTO:
		protoType := unpackProtoType("", f.Tree.Default.TypeUrl)
		retType = protoType.Type
	}

	usedVariables := make(map[string]string)
	code := translateFeatureTS(f, nil, usedVariables)
	if len(parameters) == 0 && len(usedVariables) > 0 {
		var keys []string
		var keyAndTypes []string
		for k, t := range usedVariables {
			keys = append(keys, k)
			keyAndTypes = append(keyAndTypes, fmt.Sprintf("%s: %s", k, t))
		}
		parameters = fmt.Sprintf("{%s}: {%s}", strings.Join(keys, ","), strings.Join(keyAndTypes, ","))
	}
	data := struct {
		Description     string
		FuncName        string
		RetType         string
		Namespace       string
		Key             string
		NaturalLanguage []string
		Parameters      string
	}{
		f.Description,
		funcName,
		retType,
		ns,
		f.Key,
		code,
		parameters,
	}
	templ, err := template.New("go func").Parse(templateBody)
	if err != nil {
		return "", err
	}
	var ret bytes.Buffer
	err = templ.Execute(&ret, data)
	if err != nil {
		return "", err
	}
	return ret.String(), nil
}

func translateFeatureTS(f *featurev1beta1.Feature, protoType *protoImport, usedVariables map[string]string) []string {
	var buffer []string
	for i, constraint := range f.Tree.Constraints {
		ifToken := "} else if"
		if i == 0 {
			ifToken = "if"
		}
		rule := translateRuleTS(constraint.GetRuleAstNew(), usedVariables)
		buffer = append(buffer, fmt.Sprintf("\t%s %s {", ifToken, rule))

		// TODO this doesn't work for proto, but let's try
		buffer = append(buffer, fmt.Sprintf("\t\treturn %s;", translateRetValueTS(constraint.Value, protoType)))
	}
	if len(f.Tree.Constraints) > 0 {
		buffer = append(buffer, "\t}")
	}
	buffer = append(buffer, fmt.Sprintf("\treturn %s;", translateRetValueTS(f.GetTree().GetDefault(), protoType)))
	return buffer
}

func translateRuleTS(rule *rulesv1beta3.Rule, usedVariables map[string]string) string {
	marshalOptions := protojson.MarshalOptions{
		UseProtoNames: true,
	}
	if rule == nil {
		return ""
	}
	switch v := rule.GetRule().(type) {
	case *rulesv1beta3.Rule_Atom:
		usedVariables[v.Atom.ContextKey] = "string" // TODO
		switch v.Atom.GetComparisonOperator() {
		case rulesv1beta3.ComparisonOperator_COMPARISON_OPERATOR_EQUALS:
			return fmt.Sprintf("( %s === %s )", v.Atom.ContextKey, try.To1(marshalOptions.Marshal(v.Atom.ComparisonValue)))
		case rulesv1beta3.ComparisonOperator_COMPARISON_OPERATOR_CONTAINED_WITHIN:
			var elements []string
			for _, comparisonVal := range v.Atom.ComparisonValue.GetListValue().GetValues() {
				elements = append(elements, string(try.To1(marshalOptions.Marshal(comparisonVal))))
			}
			return fmt.Sprintf("([%s].includes(%s))", strings.Join(elements, ", "), v.Atom.ContextKey)
		case rulesv1beta3.ComparisonOperator_COMPARISON_OPERATOR_CONTAINS:
			return fmt.Sprintf("(%s.includes(%s))", v.Atom.ContextKey, try.To1(marshalOptions.Marshal(v.Atom.ComparisonValue)))
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
			result = append(result, translateRuleTS(rule, usedVariables))
		}
		return "(" + strings.Join(result, operator) + ")"
	}

	return ""
}

func translateRetValueTS(val *anypb.Any, protoType *protoImport) string {
	marshalOptions := protojson.MarshalOptions{
		UseProtoNames: true,
	}

	// protos
	msg, err := anypb.UnmarshalNew(val, proto.UnmarshalOptions{Resolver: typeRegistry})
	if err != nil {
		panic(err)
	}

	if protoType == nil {
		// TODO we may need more special casing here for primitive types.
		// This feels like horrific syntax, but I needed this because
		// Int64 was somehow serializing to "1" instead of 1, and typechecking
		// doesn't seem to work since `UnmarshalNew` returns a `dynamicpb.Message` which doesn't work with go's type casing.
		if val.MessageIs((*wrapperspb.Int64Value)(nil)) {
			var i64 wrapperspb.Int64Value
			try.To(val.UnmarshalTo(&i64))
			return strconv.FormatInt(i64.Value, 10)
		}
		return string(try.To1(marshalOptions.Marshal(msg)))
	}
	// todo multiline formatting
	// TODO... why this instead of the basic shit?
	var lines []string
	msg.ProtoReflect().Range(func(f protoreflect.FieldDescriptor, val protoreflect.Value) bool {
		valueStr := val.String()
		if val, ok := val.Interface().(string); ok {
			valueStr = fmt.Sprintf(`"%s"`, val)
		}

		lines = append(lines, fmt.Sprintf("%s: %s", f.TextName(), valueStr))
		return true
	})
	return fmt.Sprintf("&%s.%s{%s}", protoType.PackageAlias, protoType.Type, strings.Join(lines, ", "))
}
