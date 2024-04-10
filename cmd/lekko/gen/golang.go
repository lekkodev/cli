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
	"bytes"
	"context"
	"fmt"
	"go/format"
	"os"
	"os/exec"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"text/template"

	featurev1beta1 "buf.build/gen/go/lekkodev/cli/protocolbuffers/go/lekko/feature/v1beta1"
	rulesv1beta3 "buf.build/gen/go/lekkodev/cli/protocolbuffers/go/lekko/rules/v1beta3"
	"github.com/iancoleman/strcase"
	"github.com/lainio/err2/try"
	"github.com/lekkodev/cli/pkg/repo"
	"github.com/lekkodev/cli/pkg/secrets"
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

var typeRegistry *protoregistry.Types

const StaticBytes = false

// Natural language codegen is in super alpha, only handles a subset
// of what is available, namely only supports protos that are one level
// deep with non-repeated primitives, a subset of ruleslang (== and in ops)
// Also doesn't support external types.
func GenGoCmd() *cobra.Command {
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
			staticCtxType := UnpackProtoType(moduleRoot, nsMDs[ns].ContextProto)
			ffs, err := r.GetFeatureFiles(cmd.Context(), ns)
			if err != nil {
				return err
			}
			sort.SliceStable(ffs, func(i, j int) bool {
				return ffs[i].CompiledProtoBinFileName < ffs[j].CompiledProtoBinFileName
			})
			var protoAsByteStrings []string
			var codeStrings []string
			protoImportSet := make(map[string]*ProtoImport)
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
					protoImport := UnpackProtoType(moduleRoot, f.Tree.Default.TypeUrl)
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
			// only importing strings if needed
			const templateBody = `package lekko{{$.Namespace}}

import (
	"context"
  "strings"
{{range  $.ProtoImports}}
	{{ . }}{{end}}
	client "github.com/lekkodev/go-sdk/client"
	"golang.org/x/exp/slices"
)

type LekkoClient struct {
	client.Client
	Close client.CloseFunc
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

func genGoForFeature(ctx context.Context, r repo.ConfigurationRepository, f *featurev1beta1.Feature, ns string, staticCtxType *ProtoImport) (string, error) {
	const defaultTemplateBody = `
// {{$.Description}}
func (c *LekkoClient) {{$.FuncName}}({{$.ArgumentString}}) {{$.RetType}} {
  {{ $.CtxStuff }}
  err := c.{{$.GetFunction}}(ctx, "{{$.Namespace}}", "{{$.Key}}")
	if err == nil {
	  return result
  }
  return {{$.PrivateFunc}}({{$.CallString}})
}
// {{$.Description}}
func {{$.PrivateFunc}}({{$.ArgumentString}}) {{$.RetType}} {
{{range  $.NaturalLanguage}}{{ . }}
{{end}}}`

	const protoTemplateBody = `
// {{$.Description}}
func (c *LekkoClient) {{$.FuncName}}({{$.ArgumentString}}) *{{$.RetType}} {
  {{ $.CtxStuff }}
	result := &{{$.RetType}}{}
	err := c.{{$.GetFunction}}(ctx, "{{$.Namespace}}", "{{$.Key}}", result)
	if err == nil {
	  return result
  }
  return {{$.PrivateFunc}}({{$.CallString}})
}
// {{$.Description}}
func {{$.PrivateFunc}}({{$.ArgumentString}}) *{{$.RetType}} {
{{range  $.NaturalLanguage}}{{ . }}
{{end}}}`

  // TODO this is broke as hell because we don't know how we want to do it
	const jsonTemplateBody = `// {{$.Description}}
func (c *LekkoClient) {{$.FuncName}}(ctx context.Context, result interface{}) {
  {{ $.CtxStuff }}
  err := c.{{$.GetFunction}}(ctx, "{{$.Namespace}}", "{{$.Key}}", result)
	if err == nil {
	  return result
  }
  return {{$.PrivateFunc}}({{$.CallString}})
}

// {{$.Description}}
func {{$.PrivateFunc}}(ctx context.Context, result interface{}) {
 	c.{{$.GetFunction}}(ctx, "{{$.Namespace}}", "{{$.Key}}", result)
  {{range  $.NaturalLanguage}}{{ . }}{{end}}
}
`

	// Generate an enum type and const declarations
	const stringEnumTemplateBody = `type {{$.EnumTypeName}} string
const (
	{{range $index, $field := $.EnumFields}}{{$field.Name}} {{$.EnumTypeName}} = "{{$field.Value}}"
	{{end}}
)


func (c *LekkoClient) {{$.FuncName}}({{$.ArgumentString}}) {{$.RetType}} {
  {{ $.CtxStuff }}
  result, err := c.{{$.GetFunction}}(ctx, "{{$.Namespace}}", "{{$.Key}}")
	if err == nil {
	  return result
  }
  return {{$.PrivateFunc}}({{$.CallString}})
}
// {{$.Description}}
func {{$.PrivateFunc}}({{$.ArgumentString}}) {{$.RetType}} {
{{range  $.NaturalLanguage}}{{ . }}
{{end}}}`

	var funcNameBuilder strings.Builder
	funcNameBuilder.WriteString("Get")
	for _, word := range regexp.MustCompile("[_-]+").Split(f.Key, -1) {
		funcNameBuilder.WriteString(strings.ToUpper(word[:1]) + word[1:])
	}
	funcName := funcNameBuilder.String()
	privateFunc := strcase.ToLowerCamel(funcName)
	var retType string
	var getFunction string
	var enumTypeName string
	type EnumField struct {
		Name  string
		Value string
	}
	var enumFields []EnumField
	templateBody := defaultTemplateBody

	var protoType *ProtoImport
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
		protoType = UnpackProtoType("", f.Tree.Default.TypeUrl)
		// creates configv1beta1.DBConfig
		retType = fmt.Sprintf("%s.%s", protoType.PackageAlias, protoType.Type)
	}

	data := struct {
		Description     string
		FuncName        string
		PrivateFunc     string
		GetFunction     string
		RetType         string
		Namespace       string
		Key             string
		NaturalLanguage []string
		ArgumentString  string
		CallString      string
		EnumTypeName    string
		EnumFields      []EnumField
		CtxStuff        string
	}{
		f.Description,
		funcName,
		privateFunc,
		getFunction,
		retType,
		ns,
		f.Key,
		[]string{},
		"",
		"",
		enumTypeName,
		enumFields,
		"",
	}
	usedVariables := make(map[string]string)
	if staticCtxType != nil {
		data.NaturalLanguage = translateFeature(f, protoType, true, usedVariables)
		data.ArgumentString = fmt.Sprintf("ctx *%s.%s", staticCtxType.PackageAlias, staticCtxType.Type)
		data.CallString = "ctx"
	} else {
		data.CtxStuff = "ctx := context.Background()\n"
		data.NaturalLanguage = translateFeature(f, protoType, false, usedVariables)
		var arguments []string
		for f, t := range usedVariables {
			arguments = append(arguments, fmt.Sprintf("%s %s", f, t))
			data.CtxStuff += fmt.Sprintf("ctx = context.WithValue(ctx, \"%s\", %s)\n", f, f)
		}
		data.ArgumentString = strings.Join(arguments, ", ")
		var keys []string
		for f := range usedVariables {
			keys = append(keys, f)
		}
		data.CallString = strings.Join(keys, ", ")
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
    //return ret.String(), nil // Leave this here for easy debugging
		return "", errors.Wrap(err, "format")
	}
	return string(formatted), nil
}

func translateFeature(f *featurev1beta1.Feature, protoType *ProtoImport, staticContext bool, usedVariables map[string]string) []string {
	var buffer []string
	for i, constraint := range f.Tree.Constraints {
		ifToken := "} else if"
		if i == 0 {
			ifToken = "if"
		}
		rule := translateRule(constraint.GetRuleAstNew(), staticContext, usedVariables)
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

func translateRule(rule *rulesv1beta3.Rule, staticContext bool, usedVariables map[string]string) string {
	if rule == nil {
		return ""
	}
	switch v := rule.GetRule().(type) {
	case *rulesv1beta3.Rule_Atom:
		switch v.Atom.GetComparisonOperator() {
		case rulesv1beta3.ComparisonOperator_COMPARISON_OPERATOR_EQUALS:
			usedVariables[v.Atom.ContextKey] = structpbValueToKindString(v.Atom.ComparisonValue)
			if staticContext {
				return fmt.Sprintf("ctx.%s == %s", strcase.ToCamel(v.Atom.ContextKey), string(try.To1(protojson.Marshal(v.Atom.ComparisonValue))))
			} else {
				return fmt.Sprintf("%s == %s", strcase.ToLowerCamel(v.Atom.ContextKey), string(try.To1(protojson.Marshal(v.Atom.ComparisonValue))))
			}
		case rulesv1beta3.ComparisonOperator_COMPARISON_OPERATOR_NOT_EQUALS:
			usedVariables[v.Atom.ContextKey] = structpbValueToKindString(v.Atom.ComparisonValue)
			if staticContext {
				return fmt.Sprintf("ctx.%s != %s", strcase.ToCamel(v.Atom.ContextKey), string(try.To1(protojson.Marshal(v.Atom.ComparisonValue))))
			} else {
				return fmt.Sprintf("%s != %s", strcase.ToLowerCamel(v.Atom.ContextKey), string(try.To1(protojson.Marshal(v.Atom.ComparisonValue))))
			}
		case rulesv1beta3.ComparisonOperator_COMPARISON_OPERATOR_LESS_THAN:
			usedVariables[v.Atom.ContextKey] = structpbValueToKindString(v.Atom.ComparisonValue)
			if staticContext {
				return fmt.Sprintf("ctx.%s < %s", strcase.ToCamel(v.Atom.ContextKey), string(try.To1(protojson.Marshal(v.Atom.ComparisonValue))))
			} else {
				return fmt.Sprintf("%s < %s", strcase.ToLowerCamel(v.Atom.ContextKey), string(try.To1(protojson.Marshal(v.Atom.ComparisonValue))))
			}
		case rulesv1beta3.ComparisonOperator_COMPARISON_OPERATOR_LESS_THAN_OR_EQUALS:
			usedVariables[v.Atom.ContextKey] = structpbValueToKindString(v.Atom.ComparisonValue)
			if staticContext {
				return fmt.Sprintf("ctx.%s <= %s", strcase.ToCamel(v.Atom.ContextKey), string(try.To1(protojson.Marshal(v.Atom.ComparisonValue))))
			} else {
				return fmt.Sprintf("%s <= %s", strcase.ToLowerCamel(v.Atom.ContextKey), string(try.To1(protojson.Marshal(v.Atom.ComparisonValue))))
			}
		case rulesv1beta3.ComparisonOperator_COMPARISON_OPERATOR_GREATER_THAN:
			usedVariables[v.Atom.ContextKey] = structpbValueToKindString(v.Atom.ComparisonValue)
			if staticContext {
				return fmt.Sprintf("ctx.%s > %s", strcase.ToCamel(v.Atom.ContextKey), string(try.To1(protojson.Marshal(v.Atom.ComparisonValue))))
			} else {
				return fmt.Sprintf("%s > %s", strcase.ToLowerCamel(v.Atom.ContextKey), string(try.To1(protojson.Marshal(v.Atom.ComparisonValue))))
			}
		case rulesv1beta3.ComparisonOperator_COMPARISON_OPERATOR_GREATER_THAN_OR_EQUALS:
			usedVariables[v.Atom.ContextKey] = structpbValueToKindString(v.Atom.ComparisonValue)
			if staticContext {
				return fmt.Sprintf("ctx.%s >= %s", strcase.ToCamel(v.Atom.ContextKey), string(try.To1(protojson.Marshal(v.Atom.ComparisonValue))))
			} else {
				return fmt.Sprintf("%s >= %s", strcase.ToLowerCamel(v.Atom.ContextKey), string(try.To1(protojson.Marshal(v.Atom.ComparisonValue))))
			}
		case rulesv1beta3.ComparisonOperator_COMPARISON_OPERATOR_CONTAINS:
			usedVariables[v.Atom.ContextKey] = structpbValueToKindString(v.Atom.ComparisonValue)
			if staticContext {
				return fmt.Sprintf("strings.Contains(ctx.%s, %s)", strcase.ToCamel(v.Atom.ContextKey), string(try.To1(protojson.Marshal(v.Atom.ComparisonValue))))
			} else {
				return fmt.Sprintf("strings.Contains(%s,  %s)", strcase.ToLowerCamel(v.Atom.ContextKey), string(try.To1(protojson.Marshal(v.Atom.ComparisonValue))))
			}
		case rulesv1beta3.ComparisonOperator_COMPARISON_OPERATOR_STARTS_WITH:
			usedVariables[v.Atom.ContextKey] = structpbValueToKindString(v.Atom.ComparisonValue)
			if staticContext {
				return fmt.Sprintf("strings.HasPrefix(ctx.%s, %s)", strcase.ToCamel(v.Atom.ContextKey), string(try.To1(protojson.Marshal(v.Atom.ComparisonValue))))
			} else {
				return fmt.Sprintf("strings.HasPrefix(%s,  %s)", strcase.ToLowerCamel(v.Atom.ContextKey), string(try.To1(protojson.Marshal(v.Atom.ComparisonValue))))
			}
		case rulesv1beta3.ComparisonOperator_COMPARISON_OPERATOR_ENDS_WITH:
			usedVariables[v.Atom.ContextKey] = structpbValueToKindString(v.Atom.ComparisonValue)
			if staticContext {
				return fmt.Sprintf("strings.HasSuffix(ctx.%s, %s)", strcase.ToCamel(v.Atom.ContextKey), string(try.To1(protojson.Marshal(v.Atom.ComparisonValue))))
			} else {
				return fmt.Sprintf("strings.HasSuffix(%s,  %s)", strcase.ToLowerCamel(v.Atom.ContextKey), string(try.To1(protojson.Marshal(v.Atom.ComparisonValue))))
			}
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
			usedVariables[v.Atom.ContextKey] = sliceType
			if staticContext {
				return fmt.Sprintf("slices.Contains([]%s{%s}, ctx.%s)", sliceType, strings.Join(elements, ", "), strcase.ToCamel(v.Atom.ContextKey))
			} else {
				return fmt.Sprintf("slices.Contains([]%s{%s}, %s)", sliceType, strings.Join(elements, ", "), strcase.ToLowerCamel(v.Atom.ContextKey))
			}
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
			result = append(result, translateRule(rule, staticContext, usedVariables))
		}
		return strings.Join(result, operator)
	}

	fmt.Printf("Need to learn how to: %+v\n", rule.GetRule())
	return ""
}

func FieldValueToString(f protoreflect.FieldDescriptor, val protoreflect.Value, protoType *ProtoImport) string {
	if msg, ok := val.Interface().(protoreflect.Message); ok {
		if _, err := typeRegistry.FindMessageByName((msg.Descriptor().FullName())); err != nil {
			// THIS SUCKS but is probably a bug we should file with anypb if someone / konrad is bored.
			try.To(typeRegistry.RegisterMessage(msg.Type()))
		}
		return translateRetValue(try.To1(anypb.New(msg.Interface())), protoType) // not sure if this will work..
	} else {
		switch f.Kind() {
		case protoreflect.EnumKind:
			fallthrough
		case protoreflect.StringKind:
			return fmt.Sprintf("\"%s\"", val.String())
		case protoreflect.BoolKind:
			return val.String()
		case protoreflect.BytesKind:
			panic("Don't know how to take bytes, try nibbles")
		case protoreflect.FloatKind:
			fallthrough
		case protoreflect.DoubleKind:
			fallthrough
		case protoreflect.Int64Kind:
			fallthrough
		case protoreflect.Int32Kind:
			fallthrough
		case protoreflect.Uint64Kind:
			fallthrough
		case protoreflect.Uint32Kind:
			return val.String()
		case protoreflect.MessageKind:
			if f.IsMap() {
				var lines []string
				res := fmt.Sprintf("map[%s]%s{ ", f.MapKey().Kind().String(), f.MapValue().Kind().String()) // TODO - this probbaly breaks for nested
				val.Map().Range(func(mk protoreflect.MapKey, mv protoreflect.Value) bool {
					lines = append(lines, fmt.Sprintf("\"%s\": %s",
						mk.String(),
						FieldValueToString(f.MapValue(), mv, protoType)))
					return true
				})
				res += strings.Join(lines, ", ")
				res += " }"
				return res
			} else if f.IsList() {
				panic(fmt.Sprintf("Do not know how to count: %+v", f))
			}
		default:
			panic(fmt.Sprintf("Unknown: %+v", f))
		}
	}
	panic("Unreachable code was reached")
}
func translateRetValue(val *anypb.Any, protoType *ProtoImport) string {
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
		/*	valueStr := val.String()
			if val, ok := val.Interface().(string); ok {
				valueStr = fmt.Sprintf(`"%s"`, val)
			}

			lines = append(lines, fmt.Sprintf("%s: %s", strcase.ToCamel(f.TextName()), valueStr))
		*/
		lines = append(lines, fmt.Sprintf("%s: %s", strcase.ToCamel(f.TextName()), FieldValueToString(f, val, protoType)))
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
