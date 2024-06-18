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
	"io"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"slices"
	"sort"
	"strconv"
	"strings"
	"text/template"

	featurev1beta1 "buf.build/gen/go/lekkodev/cli/protocolbuffers/go/lekko/feature/v1beta1"
	rulesv1beta3 "buf.build/gen/go/lekkodev/cli/protocolbuffers/go/lekko/rules/v1beta3"
	"github.com/iancoleman/strcase"
	"github.com/lainio/err2/try"
	"github.com/lekkodev/cli/pkg/repo"
	"github.com/pkg/errors"
	"golang.org/x/exp/maps"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/reflect/protoregistry"
	"google.golang.org/protobuf/types/known/anypb"
	"google.golang.org/protobuf/types/known/durationpb"
	"google.golang.org/protobuf/types/known/structpb"
	"google.golang.org/protobuf/types/known/wrapperspb"
)

var TypeRegistry *protoregistry.Types

func FieldDescriptorToTS(f protoreflect.FieldDescriptor) string {
	var t string
	switch f.Kind() {
	case protoreflect.StringKind:
		t = "string"
	case protoreflect.BoolKind:
		t = "boolean"
	case protoreflect.BytesKind:
		t = "Uint8Array"
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
		t = "number"
	case protoreflect.EnumKind:
		t = "string"
	case protoreflect.MessageKind:
		if f.IsMap() {
			t = fmt.Sprintf("Record<%s, %s>", FieldDescriptorToTS(f.MapKey()), FieldDescriptorToTS(f.MapValue()))
		} else if strings.HasPrefix(string(f.Message().FullName()), "google") {
			t = fmt.Sprintf("protobuf.%s", f.Message().Name())
		} else {
			d := f.Message()
			t = "{\n"
			for i := 0; i < d.Fields().Len(); i++ {
				f := d.Fields().Get(i)
				t += fmt.Sprintf("\t%s?: %s;\n", strcase.ToLowerCamel(f.TextName()), FieldDescriptorToTS(f))
			}
			t += "}"
		}
		// TODO add more
	default:
		t = f.Kind().String()
	}
	if f.Cardinality() == protoreflect.Repeated && !f.IsMap() {
		t += "[]"
	}
	return t
}

func GetTSInterface(d protoreflect.MessageDescriptor) (string, error) {
	const templateBody = `export interface {{$.Name}} {
{{range  $.Fields}}    {{ . }}
{{end}}}`

	var fields []string
	for i := 0; i < d.Fields().Len(); i++ {
		f := d.Fields().Get(i)
		t := FieldDescriptorToTS(f)
		fields = append(fields, fmt.Sprintf("%s?: %s;", strcase.ToLowerCamel(f.TextName()), t))
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

// Pipe `GenTS` to prettier
func genFormattedTS(ctx context.Context, repoPath, ns, outFilename string) error {
	prettierCmd := exec.Command("npx", "prettier", "--parser", "typescript")
	stdinPipe, err := prettierCmd.StdinPipe()
	if err != nil {
		return err
	}
	err = GenTS(ctx, repoPath, ns, func() (io.Writer, error) {
		return stdinPipe, nil
	})
	if err != nil {
		return err
	}
	stdinPipe.Close()
	out, err := prettierCmd.Output()
	if err != nil {
		return err
	}
	return os.WriteFile(outFilename, out, 0600)
}

func GenTS(ctx context.Context, repoPath, ns string, getWriter func() (io.Writer, error)) error {
	// TODO to avoid weird error message we should compile first.
	var err error
	if len(repoPath) == 0 {
		repoPath, err = repo.PrepareGithubRepo()
		if err != nil {
			return err
		}
	}
	r, err := repo.NewLocal(repoPath, nil)
	if err != nil {
		return errors.Wrap(err, "new repo")
	}
	rootMD, nsMDs := try.To2(r.ParseMetadata(ctx))
	TypeRegistry = try.To1(r.BuildDynamicTypeRegistry(ctx, rootMD.ProtoDirectory))
	var parameters string
	interfaces := make(map[string]string)
	if _, ok := nsMDs[ns]; !ok {
		log.Fatal("unknown namespace: ", ns)
	}
	if len(nsMDs[ns].ContextProto) > 0 {
		ptype, err := TypeRegistry.FindMessageByName(protoreflect.FullName(nsMDs[ns].ContextProto))
		if err != nil {
			log.Fatal("error finding the message in the registry", err)
		}
		parameters = getTSParameters(ptype.Descriptor())
		face, err := GetTSInterface(ptype.Descriptor())
		if err != nil {
			return err
		}
		interfaces[nsMDs[ns].ContextProto] = face
	}
	var codeStrings []string
	/*
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
	*/

	ffs, err := r.GetFeatureFiles(ctx, ns)
	if err != nil {
		return err
	}
	sort.SliceStable(ffs, func(i, j int) bool {
		return ffs[i].CompiledProtoBinFileName < ffs[j].CompiledProtoBinFileName
	})

	protoImports := make(map[string]struct{})
	for _, ff := range ffs {
		ourParameters := parameters
		fff, err := os.ReadFile(filepath.Join(repoPath, ns, ff.CompiledProtoBinFileName))
		if err != nil {
			return err
		}
		f := &featurev1beta1.Feature{}
		if err := proto.Unmarshal(fff, f); err != nil {
			return err
		}
		if f.Type == featurev1beta1.FeatureType_FEATURE_TYPE_PROTO {
			// TODO: refactor this shared function?
			pImport := UnpackProtoType("", "internal/lekko", f.Tree.Default.TypeUrl)
			if strings.HasPrefix(pImport.ImportPath, "google.golang.org") {
				protoImports["import * as protobuf from '@bufbuild/protobuf';"] = struct{}{}
			} else {
				name := strings.Split(f.Tree.Default.TypeUrl, "/")[1]
				if _, ok := interfaces[name]; !ok {
					ptype, err := TypeRegistry.FindMessageByName(protoreflect.FullName(name))
					if err != nil {
						return errors.Wrapf(err, "could not find message: %s", protoreflect.FullName(name))
					}
					face, err := GetTSInterface(ptype.Descriptor())
					if err != nil {
						return err
					}
					interfaces[name] = face
				}
			}
		}
		// Check if there is a per-config signature proto
		sigType, err := TypeRegistry.FindMessageByName(protoreflect.FullName(ns + ".config.v1beta1." + strcase.ToCamel(f.Key) + "Args"))
		if err == nil {
			d := sigType.Descriptor()
			var varNames []string
			var fields []string
			for i := 0; i < d.Fields().Len(); i++ {
				f := d.Fields().Get(i)
				t := FieldDescriptorToTS(f)
				fields = append(fields, fmt.Sprintf("%s?: %s;", strcase.ToLowerCamel(f.TextName()), t))
				varNames = append(varNames, strcase.ToLowerCamel(f.TextName()))
			}

			ourParameters = fmt.Sprintf("{%s}: {%s}", strings.Join(varNames, ", "), strings.Join(fields, " "))
		}

		codeString, err := GenTSForFeature(f, ns, ourParameters)
		if err != nil {
			return err
		}
		codeStrings = append(codeStrings, codeString)
	}
	const templateBody = `{{range  $.CodeStrings}}
{{ . }}
{{end}}`

	interfaceStrings := make([]string, 0, len(interfaces))
	keys := maps.Keys(interfaces)
	slices.Sort(keys)
	for _, k := range keys {
		interfaceStrings = append(interfaceStrings, interfaces[k])
	}

	data := struct {
		Namespace   string
		CodeStrings []string
	}{
		ns,
		append(maps.Keys(protoImports), append(interfaceStrings, codeStrings...)...),
	}
	wr, err := getWriter()
	if err != nil {
		return err
	}
	templ := template.Must(template.New("").Parse(templateBody))
	return templ.Execute(wr, data)
}

func GenTSForFeature(f *featurev1beta1.Feature, ns string, parameters string) (string, error) {
	// TODO: support multiline descriptions
	const templateBody = `{{if $.Description}}/** {{$.Description}} */{{end}}
export function {{$.FuncName}}({{$.Parameters}}): {{$.RetType}} {
{{range  $.NaturalLanguage}}{{ . }}
{{end}}}`

	var funcNameBuilder strings.Builder
	funcNameBuilder.WriteString("get")
	for _, word := range regexp.MustCompile("[_-]+").Split(f.Key, -1) {
		funcNameBuilder.WriteString(strings.ToUpper(word[:1]) + word[1:])
	}
	funcName := funcNameBuilder.String()
	var retType string
	var protoType *ProtoImport
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
		// TODO: refactor this shared function?
		protoType = UnpackProtoType("", "internal/lekko", f.Tree.Default.TypeUrl)
		if strings.HasPrefix(protoType.ImportPath, "google") {
			retType = fmt.Sprintf("protobuf.%s", protoType.Type)
		} else {
			retType = protoType.Type
		}
	}

	usedVariables := make(map[string]string)
	code := translateFeatureTS(f, usedVariables)
	if len(parameters) == 0 && len(usedVariables) > 0 {
		var keys []string
		var keyAndTypes []string
		var sortedKeys []string
		for k := range usedVariables {
			sortedKeys = append(sortedKeys, k)
		}
		sort.Strings(sortedKeys)
		for _, k := range sortedKeys {
			t := usedVariables[k]
			casedK := strcase.ToLowerCamel(k)
			keys = append(keys, casedK)
			keyAndTypes = append(keyAndTypes, fmt.Sprintf("%s: %s", casedK, t))
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

func translateFeatureTS(f *featurev1beta1.Feature, usedVariables map[string]string) []string {
	var buffer []string
	for i, constraint := range f.Tree.Constraints {
		ifToken := "} else if"
		if i == 0 {
			ifToken = "if"
		}
		rule := translateRuleTS(constraint.GetRuleAstNew(), usedVariables)
		buffer = append(buffer, fmt.Sprintf("\t%s %s {", ifToken, rule))

		// TODO this doesn't work for proto, but let's try
		buffer = append(buffer, fmt.Sprintf("\t\treturn %s;", translateRetValueTS(constraint.Value, f.Type)))
	}
	if len(f.Tree.Constraints) > 0 {
		buffer = append(buffer, "\t}")
	}
	buffer = append(buffer, fmt.Sprintf("\treturn %s;", translateRetValueTS(f.GetTree().GetDefault(), f.Type)))
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
		contextKeyName := strcase.ToLowerCamel(v.Atom.ContextKey)
		usedVariables[v.Atom.ContextKey] = "string" // TODO - ugly as hell
		switch v.Atom.GetComparisonOperator() {
		case rulesv1beta3.ComparisonOperator_COMPARISON_OPERATOR_EQUALS:
			if b, ok := v.Atom.ComparisonValue.GetKind().(*structpb.Value_BoolValue); ok {
				usedVariables[v.Atom.ContextKey] = "boolean"
				if b.BoolValue {
					return fmt.Sprintf("(%s)", contextKeyName)
				} else {
					return fmt.Sprintf("(!%s)", contextKeyName)
				}
			}
			usedVariables[v.Atom.ContextKey] = structpbValueToKindString(v.Atom.ComparisonValue)
			return fmt.Sprintf("( %s === %s )", contextKeyName, try.To1(marshalOptions.Marshal(v.Atom.ComparisonValue)))
		case rulesv1beta3.ComparisonOperator_COMPARISON_OPERATOR_NOT_EQUALS:
			usedVariables[v.Atom.ContextKey] = structpbValueToKindString(v.Atom.ComparisonValue)
			return fmt.Sprintf("( %s !== %s )", contextKeyName, try.To1(marshalOptions.Marshal(v.Atom.ComparisonValue)))
		case rulesv1beta3.ComparisonOperator_COMPARISON_OPERATOR_CONTAINED_WITHIN:
			usedVariables[v.Atom.ContextKey] = structpbValueToKindString(v.Atom.ComparisonValue.GetListValue().GetValues()[0])
			var elements []string
			for _, comparisonVal := range v.Atom.ComparisonValue.GetListValue().GetValues() {
				elements = append(elements, string(try.To1(marshalOptions.Marshal(comparisonVal))))
			}
			return fmt.Sprintf("([%s].includes(%s))", strings.Join(elements, ", "), contextKeyName)
		case rulesv1beta3.ComparisonOperator_COMPARISON_OPERATOR_LESS_THAN:
			usedVariables[v.Atom.ContextKey] = structpbValueToKindString(v.Atom.ComparisonValue)
			return fmt.Sprintf("(%s < %s)", contextKeyName, try.To1(marshalOptions.Marshal(v.Atom.ComparisonValue)))
		case rulesv1beta3.ComparisonOperator_COMPARISON_OPERATOR_LESS_THAN_OR_EQUALS:
			usedVariables[v.Atom.ContextKey] = structpbValueToKindString(v.Atom.ComparisonValue)
			return fmt.Sprintf("(%s <= %s)", contextKeyName, try.To1(marshalOptions.Marshal(v.Atom.ComparisonValue)))
		case rulesv1beta3.ComparisonOperator_COMPARISON_OPERATOR_GREATER_THAN:
			usedVariables[v.Atom.ContextKey] = structpbValueToKindString(v.Atom.ComparisonValue)
			return fmt.Sprintf("(%s > %s)", contextKeyName, try.To1(marshalOptions.Marshal(v.Atom.ComparisonValue)))
		case rulesv1beta3.ComparisonOperator_COMPARISON_OPERATOR_GREATER_THAN_OR_EQUALS:
			usedVariables[v.Atom.ContextKey] = structpbValueToKindString(v.Atom.ComparisonValue)
			return fmt.Sprintf("(%s >= %s)", contextKeyName, try.To1(marshalOptions.Marshal(v.Atom.ComparisonValue)))
		case rulesv1beta3.ComparisonOperator_COMPARISON_OPERATOR_CONTAINS:
			usedVariables[v.Atom.ContextKey] = structpbValueToKindString(v.Atom.ComparisonValue)
			return fmt.Sprintf("(%s.includes(%s))", contextKeyName, try.To1(marshalOptions.Marshal(v.Atom.ComparisonValue)))
		case rulesv1beta3.ComparisonOperator_COMPARISON_OPERATOR_STARTS_WITH:
			usedVariables[v.Atom.ContextKey] = structpbValueToKindString(v.Atom.ComparisonValue)
			return fmt.Sprintf("(%s.startsWith(%s))", contextKeyName, try.To1(marshalOptions.Marshal(v.Atom.ComparisonValue)))
		case rulesv1beta3.ComparisonOperator_COMPARISON_OPERATOR_ENDS_WITH:
			usedVariables[v.Atom.ContextKey] = structpbValueToKindString(v.Atom.ComparisonValue)
			return fmt.Sprintf("(%s.endsWith(%s))", contextKeyName, try.To1(marshalOptions.Marshal(v.Atom.ComparisonValue)))
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
	case *rulesv1beta3.Rule_Not:
		return "!(" + translateRuleTS(v.Not, usedVariables) + ")"
	}

	fmt.Printf("Need to learn how to: %+v\n", rule.GetRule())
	return ""
}

// returns only the formatted value
func FieldValueToTS(f protoreflect.FieldDescriptor, val protoreflect.Value) string {
	if msg, ok := val.Interface().(protoreflect.Message); ok {
		if _, err := TypeRegistry.FindMessageByName((msg.Descriptor().FullName())); err != nil {
			// THIS SUCKS but is probably a bug we should file with anypb if someone / konrad is bored.
			try.To(TypeRegistry.RegisterMessage(msg.Type()))
		}
		kind := featurev1beta1.FeatureType_FEATURE_TYPE_PROTO
		return translateRetValueTS(try.To1(anypb.New(msg.Interface())), kind)
	} else {
		switch f.Kind() {
		case protoreflect.EnumKind:
			fallthrough
		case protoreflect.StringKind:
			if f.IsList() {
				var results []string
				list := val.List()
				for i := 0; i < list.Len(); i++ {
					item := list.Get(i)
					results = append(results, fmt.Sprintf("%q", item.String()))
				}
				return "[" + strings.Join(results, ", ") + "]"
			}
			return fmt.Sprintf("%q", val.String())
		case protoreflect.BoolKind:
			if f.IsList() {
				var results []string
				list := val.List()
				for i := 0; i < list.Len(); i++ {
					item := list.Get(i)
					results = append(results, item.String())
				}
				return "[" + strings.Join(results, ", ") + "]"
			}
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
			if f.IsList() {
				var results []string
				list := val.List()
				for i := 0; i < list.Len(); i++ {
					item := list.Get(i)
					results = append(results, item.String())
				}
				return "[" + strings.Join(results, ", ") + "]"
			}
			return val.String()
		case protoreflect.MessageKind:
			if f.IsMap() {
				var lines []string
				res := "{ "
				val.Map().Range(func(mk protoreflect.MapKey, mv protoreflect.Value) bool {
					lines = append(lines, fmt.Sprintf("\"%s\": %s",
						mk.String(),
						FieldValueToTS(f.MapValue(), mv)))
					return true
				})
				sort.Strings(lines)
				res += strings.Join(lines, ", ")
				res += " }"
				return res
			} else if f.IsList() {
				var results []string
				list := val.List()
				for i := 0; i < list.Len(); i++ {
					item := list.Get(i)
					results = append(results, FieldValueToTS(f, item))
				}
				return "[" + strings.Join(results, ", ") + "]"
			}
		default:
			panic(fmt.Sprintf("Unknown: %+v", f))
		}
	}
	panic("Unreachable code was reached")
}

func translateRetValueTS(val *anypb.Any, t featurev1beta1.FeatureType) string {
	//var dypb *dynamicpb.Message
	marshalOptions := protojson.MarshalOptions{
		UseProtoNames: true,
	}
	if val.MessageIs((*wrapperspb.Int64Value)(nil)) {
		var i64 wrapperspb.Int64Value
		try.To(val.UnmarshalTo(&i64))
		return strconv.FormatInt(i64.Value, 10)
	}
	// TODO double
	if t != featurev1beta1.FeatureType_FEATURE_TYPE_PROTO {
		// we are guessing this is a primitive, (unless we have i64 so let's do that later)
		return marshalOptions.Format(try.To1(anypb.UnmarshalNew(val, proto.UnmarshalOptions{Resolver: TypeRegistry})))
	}

	switch strings.Split(val.TypeUrl, "/")[1] {
	// TODO: other WKTs
	case "google.protobuf.Duration":
		var v durationpb.Duration
		try.To(val.UnmarshalTo(&v))
		return fmt.Sprintf("protobuf.Duration.fromJsonString(%s)", marshalOptions.Format(&v))
	default:
		dynMsg, err := anypb.UnmarshalNew(val, proto.UnmarshalOptions{Resolver: TypeRegistry})
		if err != nil {
			TypeRegistry.RangeMessages(func(m protoreflect.MessageType) bool {
				return true
			})
			panic(fmt.Sprintf("idk what is going on: %e %+v", err, err))
		}
		var lines []string
		dynMsg.ProtoReflect().Range(func(f protoreflect.FieldDescriptor, val protoreflect.Value) bool {
			lines = append(lines, fmt.Sprintf("\t\"%s\": %s", strcase.ToLowerCamel(f.TextName()), FieldValueToTS(f, val)))
			return true
		})

		sort.Strings(lines)
		return fmt.Sprintf("{\n%s\n}", strings.Join(lines, ",\n"))
	}
}
