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
	"fmt"
	"os"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"text/template"

	featurev1beta1 "buf.build/gen/go/lekkodev/cli/protocolbuffers/go/lekko/feature/v1beta1"
	rulesv1beta3 "buf.build/gen/go/lekkodev/cli/protocolbuffers/go/lekko/rules/v1beta3"
	"github.com/lainio/err2/try"
	"github.com/lekkodev/cli/pkg/repo"
	"github.com/lekkodev/cli/pkg/secrets"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/types/known/anypb"
	"google.golang.org/protobuf/types/known/structpb"
	"google.golang.org/protobuf/types/known/wrapperspb"
)

func fieldDescriptorToTS(f protoreflect.FieldDescriptor) string {
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
		if f.IsMap() {
			t = fmt.Sprintf("Record<%s, %s>", fieldDescriptorToTS(f.MapKey()), fieldDescriptorToTS(f.MapValue()))
		} else {
			t = string(f.Message().Name())
		}
		// TODO add more
	default:
		t = f.Kind().String()
	}
	if f.Cardinality() == protoreflect.Repeated && !f.IsMap() {
		t = "[]" + t
	}
	return t
}

func getTSInterface(d protoreflect.MessageDescriptor) (string, error) {
	const templateBody = `export interface {{$.Name}} {
{{range  $.Fields}}    {{ . }}
{{end}}}`

	var fields []string
	for i := 0; i < d.Fields().Len(); i++ {
		f := d.Fields().Get(i)
		t := fieldDescriptorToTS(f)
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

func GenTSCmd() *cobra.Command {
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
			/* RangeMessages only ranges over top level messages - do we want to put stuff like this in a different file?
			   ptype, err := typeRegistry.FindMessageByName(protoreflect.FullName("lekko.bff.v1beta1.RepositoryKey"))
			   print(ptype)
			*/

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
		protoType := UnpackProtoType("", f.Tree.Default.TypeUrl)
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

func translateFeatureTS(f *featurev1beta1.Feature, protoType *ProtoImport, usedVariables map[string]string) []string {
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

func structpbValueToKindString(v *structpb.Value) string {
	switch v.GetKind().(type) {
	case *structpb.Value_NumberValue:
		// technically doubles may not work for ints....
		return "number"
	case *structpb.Value_BoolValue:
		return "boolean"
	case *structpb.Value_StringValue:
		return "string"
	}
	return "unknown"
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
		usedVariables[v.Atom.ContextKey] = "string" // TODO - ugly as hell
		switch v.Atom.GetComparisonOperator() {
		case rulesv1beta3.ComparisonOperator_COMPARISON_OPERATOR_EQUALS:
			usedVariables[v.Atom.ContextKey] = structpbValueToKindString(v.Atom.ComparisonValue)
			return fmt.Sprintf("( %s === %s )", v.Atom.ContextKey, try.To1(marshalOptions.Marshal(v.Atom.ComparisonValue)))
		case rulesv1beta3.ComparisonOperator_COMPARISON_OPERATOR_CONTAINED_WITHIN:
			usedVariables[v.Atom.ContextKey] = structpbValueToKindString(v.Atom.ComparisonValue.GetListValue().GetValues()[0])
			var elements []string
			for _, comparisonVal := range v.Atom.ComparisonValue.GetListValue().GetValues() {
				elements = append(elements, string(try.To1(marshalOptions.Marshal(comparisonVal))))
			}
			return fmt.Sprintf("([%s].includes(%s))", strings.Join(elements, ", "), v.Atom.ContextKey)
		case rulesv1beta3.ComparisonOperator_COMPARISON_OPERATOR_CONTAINS:
			usedVariables[v.Atom.ContextKey] = structpbValueToKindString(v.Atom.ComparisonValue)
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

func translateRetValueTS(val *anypb.Any, protoType *ProtoImport) string {
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
