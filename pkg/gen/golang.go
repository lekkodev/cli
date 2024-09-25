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
	"path"
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
	"github.com/lainio/err2"
	"github.com/lainio/err2/assert"
	"github.com/lainio/err2/try"
	"github.com/lekkodev/cli/pkg/dotlekko"
	protoutils "github.com/lekkodev/cli/pkg/proto"
	"github.com/lekkodev/cli/pkg/repo"
	"github.com/pkg/errors"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/reflect/protoregistry"
	"google.golang.org/protobuf/types/known/anypb"
	"google.golang.org/protobuf/types/known/structpb"

	"github.com/lekkodev/cli/pkg/native"
)

type goGenerator struct {
	moduleRoot   string // e.g. github.com/lekkodev/cli
	outputPath   string // Location for destination file, can be absolute or relative. Its suffix should be the same as lekkoPath. In most cases can be same as lekkoPath.
	lekkoPath    string // Location relative to project root where Lekko files are stored, e.g. internal/lekko.
	repoContents *featurev1beta1.RepositoryContents
	TypeRegistry *protoregistry.Types
}

// Initializes a new generator from config repository contents.
func NewGoGenerator(moduleRoot, outputPath, lekkoPath string, repoContents *featurev1beta1.RepositoryContents) (*goGenerator, error) {
	typeRegistry, err := protoutils.FileDescriptorSetToTypeRegistry(repoContents.FileDescriptorSet)
	if err != nil {
		return nil, errors.Wrap(err, "convert fds to type registry")
	}
	return &goGenerator{
		moduleRoot:   moduleRoot,
		outputPath:   outputPath,
		lekkoPath:    filepath.Clean(lekkoPath),
		repoContents: repoContents,
		TypeRegistry: typeRegistry,
	}, nil
}

// Initializes a new generator, parsing config repository contents from a local repository.
func NewGoGeneratorFromLocal(ctx context.Context, moduleRoot, outputPath, lekkoPath string, repoPath string) (*goGenerator, error) {
	repoContents, err := ReadRepoContents(ctx, repoPath)
	if err != nil {
		return nil, errors.Wrapf(err, "read contents from %s", repoContents)
	}
	typeRegistry, err := protoutils.FileDescriptorSetToTypeRegistry(repoContents.FileDescriptorSet)
	if err != nil {
		return nil, errors.Wrap(err, "convert fds to type registry")
	}
	return &goGenerator{
		moduleRoot:   moduleRoot,
		outputPath:   outputPath,
		lekkoPath:    lekkoPath,
		repoContents: repoContents,
		TypeRegistry: typeRegistry,
	}, nil
}

// TODO make this work for GO
func structpbValueToKindStringGo(v *structpb.Value) string {
	switch v.GetKind().(type) {
	case *structpb.Value_NumberValue:
		// TODO: figure out how to handle this ambiguity better
		return "float64"
	case *structpb.Value_BoolValue:
		return "bool"
	case *structpb.Value_StringValue:
		return "string"
	}
	return "unknown" // do we just want to panic?
}

// Initialize a blank Lekko config function file
func (g *goGenerator) Init(ctx context.Context, namespaceName string) error {
	const templateBody = `package lekko{{$.Namespace}}

// This is an example description for an example config
func getExample() bool {
	return true
}`
	fullOutputPath := path.Join(g.outputPath, namespaceName, fmt.Sprintf("%s.go", namespaceName))
	if _, err := os.Stat(fullOutputPath); err == nil {
		return fmt.Errorf("file %s already exists", fullOutputPath)
	}

	data := struct {
		Namespace string
	}{
		Namespace: namespaceName,
	}
	var contents bytes.Buffer
	templ := template.Must(template.New(fullOutputPath).Parse(templateBody))
	if err := templ.Execute(&contents, data); err != nil {
		return errors.Wrapf(err, "%s template", fullOutputPath)
	}
	formatted, err := format.Source(contents.Bytes())
	if err != nil {
		return errors.Wrapf(err, "format %s", fullOutputPath)
	}
	try.To(os.MkdirAll(path.Join(g.outputPath, namespaceName), 0770))
	f, err := os.Create(fullOutputPath)
	if err != nil {
		return errors.Wrapf(err, "create %s", fullOutputPath)
	}
	if _, err := f.Write(formatted); err != nil {
		return errors.Wrapf(err, "write %s", fullOutputPath)
	}
	return nil
}

// This is an attempt to pull out a simpler component that is more re-usable - the other one should probably be removed/changed, but that depends on
// how far this change goes
func (g *goGenerator) GenNamespaceFiles(ctx context.Context, namespaceName string, features []*featurev1beta1.Feature, staticCtxType *ProtoImport) (string, string, error) {
	// For each namespace, we want to generate under lekko/:
	// lekko/
	//   <namespace>/
	//     <namespace>.go
	//     <namespace>_gen.go

	// <namespace>.go is meant to be edited by the user, contains private native lang funcs
	// <namespace>_gen.go is marked as machine-generated, contains public funcs to be used in application code

	const publicFileTemplateBody = `// Generated by Lekko. DO NOT EDIT.
package lekko{{$.Namespace}}

import (
	"context"
	"errors"
{{if $.ImportProtoReflect}}
	"google.golang.org/protobuf/reflect/protoreflect"
{{else}}{{end}}
{{range $.ProtoImports}}
	{{ . }}{{end}}
	"github.com/lekkodev/go-sdk/client"
	"github.com/lekkodev/go-sdk/pkg/debug"
)

type LekkoClient struct {
client.Client
}

{{range $.PublicFuncStrings}}
{{ . }}{{end}}`

	// TODOs for the template:
	// - make sure to test if slices is valid depending on go versions
	// - add go generate directive to invoke this command
	//   - but if doing 2-way and directive already exists, should respect original
	const privateFileTemplateBody = `package lekko{{$.Namespace}}

{{if or $.AddStringsImport $.AddSlicesImport (gt (len $.ProtoImports) 0)}}
import (
	{{if $.AddStringsImport}}"strings"{{end}}
{{range $.ProtoImports}}
	{{ . }}{{end}}
	{{if $.AddSlicesImport}}"golang.org/x/exp/slices"{{end}}
)
{{end}}

{{range $.StructDefs}}
{{ . }}{{end}}
{{range $.PrivateFuncStrings}}
{{ . }}{{end}}`

	var publicFuncStrings []string
	var privateFuncStrings []string
	protoImportSet := make(map[string]*ProtoImport)
	addStringsImport := false
	addSlicesImport := false
	StructDefMap := make(map[string]string)
	importProtoReflect := false
	for _, f := range features {
		if f.GetTree().GetDefaultNew() == nil {
			f.GetTree().DefaultNew = anyToLekkoAny(f.GetTree().Default)
		}
		var ctxType *ProtoImport
		if f.SignatureTypeUrl != "" {
			mt, err := g.TypeRegistry.FindMessageByURL(f.SignatureTypeUrl)
			if err == nil {
				privateFuncStrings = append(privateFuncStrings, DescriptorToStructDeclaration(mt.Descriptor()))
				ctxType = &ProtoImport{Type: string(mt.Descriptor().Name()), TypeUrl: f.SignatureTypeUrl}
			}
		}
		if ctxType == nil {
			messagePath := fmt.Sprintf("%s.config.v1beta1.%sArgs", namespaceName, strcase.ToCamel(f.Key))
			mt, err := g.TypeRegistry.FindMessageByName(protoreflect.FullName(messagePath))
			if err == nil {
				privateFuncStrings = append(privateFuncStrings, DescriptorToStructDeclaration(mt.Descriptor()))
				ctxType = &ProtoImport{Type: string(mt.Descriptor().Name()), TypeUrl: "type.googleapis.com/" + messagePath}
			} else {
				ctxType = staticCtxType
			}
		}

		if f.Type == featurev1beta1.FeatureType_FEATURE_TYPE_PROTO {
			mt, err := g.TypeRegistry.FindMessageByURL(f.Tree.DefaultNew.TypeUrl)
			if err != nil {
				panic(err)
			}
			msg := mt.New().Interface()
			err = proto.UnmarshalOptions{Resolver: g.TypeRegistry}.Unmarshal(f.Tree.DefaultNew.Value, msg)
			if err != nil {
				panic(errors.Wrapf(err, "%s", f.Tree.DefaultNew.TypeUrl))
			}
			if msg.ProtoReflect().Descriptor().FullName() != "google.protobuf.Duration" {
				// This feels bad...
				StructDefMap[f.Tree.DefaultNew.TypeUrl] = DescriptorToStructDeclaration(msg.ProtoReflect().Descriptor())
			}
			importProtoReflect = true
		}

		generated, err := g.genGoForFeature(ctx, nil, f, namespaceName, ctxType)
		if err != nil {
			return "", "", errors.Wrapf(err, "generate code for %s/%s", namespaceName, f.Key)
		}
		publicFuncStrings = append(publicFuncStrings, generated.public)
		privateFuncStrings = append(privateFuncStrings, generated.private)
		if generated.usedStrings {
			addStringsImport = true
		}
		if generated.usedSlices {
			addSlicesImport = true
		}
		if f.Type == featurev1beta1.FeatureType_FEATURE_TYPE_PROTO {
			// TODO: Return imports from gen methods and collect, this doesn't handle imports for nested
			protoImport := UnpackProtoType(g.moduleRoot, g.lekkoPath, f.Tree.DefaultNew.TypeUrl)
			if protoImport.PackageAlias != "" {
				protoImportSet[protoImport.ImportPath] = protoImport
			}
		}
	}

	var protoImports []string
	for _, imp := range protoImportSet {
		protoImports = append(protoImports, fmt.Sprintf(`%s "%s"`, imp.PackageAlias, imp.ImportPath))
	}

	var structDefs []string
	for _, sd := range StructDefMap {
		structDefs = append(structDefs, sd)
	}
	sort.Strings(structDefs)

	data := struct {
		ProtoImports       []string
		Namespace          string
		PublicFuncStrings  []string
		PrivateFuncStrings []string
		AddStringsImport   bool
		AddSlicesImport    bool
		StructDefs         []string
		ImportProtoReflect bool
	}{
		protoImports,
		namespaceName,
		publicFuncStrings,
		privateFuncStrings,
		addStringsImport,
		addSlicesImport,
		structDefs,
		importProtoReflect,
	}

	public, err := renderGoTemplate(publicFileTemplateBody, fmt.Sprintf("%s_gen.go", namespaceName), data)
	if err != nil {
		return "", "", err
	}
	private, err := renderGoTemplate(privateFileTemplateBody, fmt.Sprintf("%s.go", namespaceName), data)
	if err != nil {
		return "", "", err
	}

	return public, private, nil
}

func renderGoTemplate(templateBody string, fileName string, data any) (string, error) {
	var contents bytes.Buffer
	templ := template.Must(template.New(fileName).Parse(templateBody))
	if err := templ.Execute(&contents, data); err != nil {
		return "", errors.Wrap(err, fmt.Sprintf("%s template", fileName))
	}

	// Final canonical Go format
	formatted, err := format.Source(contents.Bytes())
	if err != nil {
		return "", errors.Wrap(err, fmt.Sprintf("format %s\n %s\n\n", fileName, contents.String()))
	}
	return string(formatted), nil
}

// Generates public and private function files for the namespace as well as the overall client file.
// Writes outputs to the output paths specified in the construction args.
// TODO: since generator takes in whole repo contents now, could generate for all/filtered namespaces
func (g *goGenerator) Gen(ctx context.Context, namespaceName string) (err error) {
	defer err2.Handle(&err)
	// Validate namespace
	if !regexp.MustCompile("[a-z]+").MatchString(namespaceName) {
		return errors.Errorf("namespace must be a lowercase alphabetic string: %s", namespaceName)
	}
	if namespaceName == "proto" {
		return errors.Errorf("'%s' is a reserved name", namespaceName)
	}
	var namespace *featurev1beta1.Namespace
	for _, ns := range g.repoContents.Namespaces {
		if ns.Name == namespaceName {
			namespace = ns
		}
	}
	if namespace == nil {
		return errors.Errorf("namespace '%s' not found", namespaceName)
	}
	// Create intermediate directories for output
	if err := os.MkdirAll(path.Join(g.outputPath, namespaceName), 0770); err != nil {
		return err
	}
	public, private, err := g.GenNamespaceFiles(ctx, namespaceName, namespace.Features, nil)
	if err != nil {
		return err
	}
	if f, err := os.Create(path.Join(g.outputPath, namespaceName, fmt.Sprintf("%s_gen.go", namespaceName))); err != nil {
		return err
	} else {
		if _, err := f.WriteString(public); err != nil {
			return errors.Wrap(err, fmt.Sprintf("write formatted contents to %s", f.Name()))
		}
	}
	if f, err := os.Create(path.Join(g.outputPath, namespaceName, fmt.Sprintf("%s.go", namespaceName))); err != nil {
		return err
	} else {
		if _, err := f.WriteString(private); err != nil {
			return errors.Wrap(err, fmt.Sprintf("write formatted contents to %s", f.Name()))
		}
	}
	return g.genClientFile(g.moduleRoot)
}

type generatedConfigCode struct {
	// "Generated" code, public interface for consumption
	public string
	// For user creation/editing
	private string
	// Whether the std package "strings" was used
	usedStrings bool
	// Whether the std package "slices" was used
	usedSlices bool
}

type configCodeTemplate struct {
	public  string
	private string
}

// Template body for primitive config code
func (g *goGenerator) getDefaultTemplateBody() *configCodeTemplate {
	return &configCodeTemplate{
		public: `// {{$.Description}}
func (c *LekkoClient) {{$.FuncName}}{{ if $.PassCtx }}Ctx{{end}}({{ if $.PassCtx }}ctx context.Context, {{end}}{{$.ArgumentString}}) {{$.RetType}} {
	{{- if not $.PassCtx}}
	ctx := context.Background()
	{{ else -}}{{- end -}}
  	{{$.CtxStuff }}
  	result, err := c.{{$.GetFunction}}(ctx, "{{$.Namespace}}", "{{$.Key}}")
	if err == nil {
	  	return result
  	}
   	result = {{$.PrivateFunc}}({{$.CallString}})
    if !errors.Is(err, client.ErrNoOpProvider) {
        debug.LogError("Lekko evaluation error", "name", "{{$.Namespace}}/{{$.Key}}", "err", err)
    }
    debug.LogDebug("Lekko fallback", "name", "{{$.Namespace}}/{{$.Key}}", "result", result)
  	return result
}
`,
		private: `// {{$.Description}}
func {{$.PrivateFunc}}({{$.ArgumentString}}) {{$.RetType}} {
{{range  $.NativeLanguage}}{{ . }}
{{end}}}
`,
	}
}

// Template body for proto config code
func (g *goGenerator) getProtoTemplateBody() *configCodeTemplate {
	return &configCodeTemplate{
		public: `// {{$.Description}}
func (c *LekkoClient) {{$.FuncName}}{{ if $.PassCtx }}Ctx{{end}}({{ if $.PassCtx }}ctx context.Context, {{end}}{{$.ArgumentString}}) *{{$.RetType}} {
	{{- if not $.PassCtx}}
	ctx := context.Background()
	{{ else -}}{{- end -}}
	{{ $.CtxStuff }}
    ret := &{{$.RetType}}{}
	result, err := c.GetAny(ctx, "{{$.Namespace}}", "{{$.Key}}")
	if err == nil {
	{{$.ProtoStructFilling}}
		return ret
	}
	ret = {{$.PrivateFunc}}({{$.CallString}})
    if !errors.Is(err, client.ErrNoOpProvider) {
        debug.LogError("Lekko evaluation error", "name", "{{$.Namespace}}/{{$.Key}}", "err", err)
    }
    debug.LogDebug("Lekko fallback", "name", "{{$.Namespace}}/{{$.Key}}", "result", ret)
    return ret
}
`,
		private: `// {{$.Description}}
func {{$.PrivateFunc}}({{$.ArgumentString}}) *{{$.RetType}} {
{{range  $.NativeLanguage}}{{ . }}
{{end}}}
`,
	}
}

// Template body for configs that are top-level enums
// TODO: This isn't actually supported, figure this out as well
// Includes const declarations for the enums
func (g *goGenerator) getStringEnumTemplateBody() *configCodeTemplate {
	return &configCodeTemplate{
		public: `type {{$.EnumTypeName}} string
const (
	{{range $index, $field := $.EnumFields}}{{$field.Name}} {{$.EnumTypeName}} = "{{$field.Value}}"
	{{end}}
)

// {{$.Description}}
func (c *LekkoClient) {{$.FuncName}}{{ if $.PassCtx }}Ctx{{end}}({{ if $.PassCtx }}ctx context.Context, {{end}}{{$.ArgumentString}}) {{$.RetType}} {
		{{- if not $.PassCtx}}
		ctx := context.Background()
		{{ else -}}{{- end -}}
		{{ $.CtxStuff }}
		result, err := c.{{$.GetFunction}}(ctx, "{{$.Namespace}}", "{{$.Key}}")
	if err == nil {
			return result
		}
		return {{$.PrivateFunc}}({{$.CallString}})
}
`,
		private: `type {{$.EnumTypeName}} string
const (
	{{range $index, $field := $.EnumFields}}{{$field.Name}} {{$.EnumTypeName}} = "{{$field.Value}}"
	{{end}}
)

// {{$.Description}}
func {{$.PrivateFunc}}({{$.ArgumentString}}) {{$.RetType}} {
{{range  $.NativeLanguage}}{{ . }}
{{end}}}
`,
	}
}

func (g *goGenerator) genGoForFeature(ctx context.Context, r repo.ConfigurationRepository, f *featurev1beta1.Feature, ns string, staticCtxType *ProtoImport) (*generatedConfigCode, error) {
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
	var protoStructFilling string
	type EnumField struct {
		Name  string
		Value string
	}
	var enumFields []EnumField
	templateBody := g.getDefaultTemplateBody()

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
		if r != nil {
			// HACK: The metadata field is only for presentation at the moment
			// so is not part of the compiled object - need to statically parse
			// This also means that this only works for statically parseable
			// configs
			sf, err := r.Parse(ctx, ns, f.Key, g.TypeRegistry) // TODO - wtf is this about? - just enums right?
			if err != nil {
				return nil, errors.Wrap(err, "static parsing")
			}
			fm := sf.Feature.Metadata.AsMap()
			// TODO: This enum codegen does not handle possible conflicts at all
			if genEnum, ok := fm["gen-enum"]; ok {
				if genEnumBool, ok := genEnum.(bool); ok && genEnumBool {
					enumTypeName = strcase.ToCamel(f.Key)
					retType = enumTypeName
					templateBody = g.getStringEnumTemplateBody()
					for _, ret := range g.getStringRetValues(f) {
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
		}
	case featurev1beta1.FeatureType_FEATURE_TYPE_JSON:
		// TODO: Actually figure out how we want to do json configs
		return nil, fmt.Errorf("unsupported json config %s/%s", ns, f.Key)
	case featurev1beta1.FeatureType_FEATURE_TYPE_PROTO:
		getFunction = "GetProto"
		templateBody = g.getProtoTemplateBody()
		matched, err := regexp.MatchString(fmt.Sprintf("type.googleapis.com/%s.config.v1beta1.[a-zA-Z0-9]", ns), f.Tree.DefaultNew.TypeUrl)
		if err != nil {
			panic(err)
		}
		if matched {
			parts := strings.Split(f.Tree.DefaultNew.TypeUrl, ".")
			retType = parts[len(parts)-1]
			protoType = UnpackProtoType(g.moduleRoot, g.lekkoPath, f.Tree.DefaultNew.TypeUrl)
			protoType.PackageAlias = ""
			// TODO - dups
		} else { // For things like returning WKT like Duration
			protoType = UnpackProtoType(g.moduleRoot, g.lekkoPath, f.Tree.DefaultNew.TypeUrl)
			retType = fmt.Sprintf("%s.%s", protoType.PackageAlias, protoType.Type)
		}
		mt, err := g.TypeRegistry.FindMessageByURL(f.Tree.DefaultNew.TypeUrl)
		if err != nil {
			panic(err)
		}
		// TODO: In case of type changes that result in only some of the fields being unpacked correctly, is it better to
		// succeed partially or defer to static fallback?
		protoStructFilling = `result.ProtoReflect().Range(func(fd protoreflect.FieldDescriptor, v protoreflect.Value) bool {
						switch fd.Name() {`
		for i := 0; i < mt.Descriptor().Fields().Len(); i++ {
			fd := mt.Descriptor().Fields().Get(i)
			fieldName := fd.Name()
			fieldType := FieldDescriptorToGoTypeString(fd)
			if fd.Kind() == protoreflect.MessageKind {
				// This includes durationpb.Duration
				return nil, fmt.Errorf("generate code for field %s: nested complex types are currently not supported", fd.FullName())
			}
			if fd.IsList() {
				protoStructFilling = protoStructFilling + fmt.Sprintf(`
		case "%[1]s":
							if !fd.IsList() {
								return true
							}
							l := v.List()
							ret.%[2]s = make(%[3]s, l.Len())
							for i := range l.Len() {
								if iv, ok := l.Get(i).Interface().(%[4]s); ok {
									ret.%[2]s[i] = iv
								} else {
									return true
								}
							}`, string(fieldName), strcase.ToCamel(string(fieldName)), fieldType, fd.Kind().String())
			} else if fd.IsMap() {
				// TODO: Maps don't work because the value is a dynamicpb.dynamicMap that can't be cast to map[key]value
				return nil, fmt.Errorf("generate code for field %s: maps are currently not supported", fd.FullName())
			} else {
				protoStructFilling = protoStructFilling + fmt.Sprintf(`
		case "%s":
							fv, ok := v.Interface().(%s)
							if (ok) {
								ret.%s = fv
							}`, string(fieldName), fieldType, strcase.ToCamel(string(fieldName)))
			}
		}
		protoStructFilling = protoStructFilling + `
		}
		 return true
					})`
	}

	data := struct {
		Description        string
		FuncName           string
		PrivateFunc        string
		GetFunction        string
		RetType            string
		Namespace          string
		Key                string
		NativeLanguage     []string
		ArgumentString     string
		CallString         string
		EnumTypeName       string
		EnumFields         []EnumField
		PassCtx            bool // whether the public function will accept the context or create it
		CtxStuff           string
		ProtoStructFilling string
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
		false,
		"",
		protoStructFilling,
	}
	generated := &generatedConfigCode{}
	usedVariables := make(map[string]string)
	if staticCtxType != nil {
		data.NativeLanguage = g.translateFeature(f, protoType, true, usedVariables, &generated.usedStrings, &generated.usedSlices)
		if staticCtxType.PackageAlias != "" {
			data.ArgumentString = fmt.Sprintf("args *%s.%s", staticCtxType.PackageAlias, staticCtxType.Type)
		} else {
			data.ArgumentString = fmt.Sprintf("args *%s", staticCtxType.Type)
		}
		data.CallString = "args"
		mt, err := g.TypeRegistry.FindMessageByURL(staticCtxType.TypeUrl)
		if err != nil {
			panic(err)
		}
		for i := 0; i < mt.Descriptor().Fields().Len(); i++ {
			fd := mt.Descriptor().Fields().Get(i)
			fieldName := string(fd.Name())
			data.CtxStuff = data.CtxStuff + fmt.Sprintf("ctx = client.Add(ctx, \"%s\", args.%s)\n", fieldName, strcase.ToCamel(fieldName))
		}
	} else {
		data.NativeLanguage = g.translateFeature(f, protoType, false, usedVariables, &generated.usedStrings, &generated.usedSlices)
		var arguments []string
		var ctxAddLines []string
		for f, t := range usedVariables {
			arguments = append(arguments, fmt.Sprintf("%s %s", strcase.ToLowerCamel(f), t))
			ctxAddLines = append(ctxAddLines, fmt.Sprintf("ctx = client.Add(ctx, \"%s\", %s)", f, strcase.ToLowerCamel(f)))
		}
		// TODO: Sorting by name might not be the best solution for long-term UX... but it's simple and it works for now
		slices.Sort(arguments)
		slices.Sort(ctxAddLines)
		data.ArgumentString = strings.Join(arguments, ", ")
		if len(ctxAddLines) > 0 {
			data.CtxStuff += strings.Join(ctxAddLines, "\n")
		}
		var keys []string
		for f := range usedVariables {
			keys = append(keys, strcase.ToLowerCamel(f))
		}
		slices.Sort(keys)
		data.CallString = strings.Join(keys, ", ")
	}
	if templ, err := template.New("public func").Parse(templateBody.public); err != nil {
		return nil, err
	} else {
		var ret bytes.Buffer
		if err := templ.Execute(&ret, data); err != nil {
			return nil, err
		}
		// generate context-aware variant, e.g. GetFooCtx(ctx context.Context, ...)
		data.PassCtx = true
		if err := templ.Execute(&ret, data); err != nil {
			return nil, err
		}
		generated.public = ret.String()
	}
	if templ, err := template.New("private func").Parse(templateBody.private); err != nil {
		return nil, err
	} else {
		var ret bytes.Buffer
		if err := templ.Execute(&ret, data); err != nil {
			return nil, err
		}
		generated.private = ret.String()
	}
	return generated, nil
}

func anyToLekkoAny(a *anypb.Any) *featurev1beta1.Any {
	return &featurev1beta1.Any{
		TypeUrl: a.GetTypeUrl(),
		Value:   a.GetValue(),
	}
}

func (g *goGenerator) translateFeature(f *featurev1beta1.Feature, protoType *ProtoImport, staticContext bool, usedVariables map[string]string, usedStrings, usedSlices *bool) []string {
	var buffer []string
	for i, constraint := range f.Tree.Constraints {
		ifToken := "} else if"
		if i == 0 {
			ifToken = "if"
		}
		rule := g.translateRule(constraint.GetRuleAstNew(), staticContext, usedVariables, usedStrings, usedSlices)
		buffer = append(buffer, fmt.Sprintf("\t%s %s {", ifToken, rule))

		// TODO this doesn't work for proto, but let's try
		if constraint.ValueNew == nil {
			constraint.ValueNew = anyToLekkoAny(constraint.Value)
		}
		buffer = append(buffer, fmt.Sprintf("\t\treturn %s", g.translateAnyValue(constraint.ValueNew, protoType)))
	}
	if len(f.Tree.Constraints) > 0 {
		buffer = append(buffer, "\t}")
	}
	buffer = append(buffer, fmt.Sprintf("\treturn %s", g.translateAnyValue(f.GetTree().GetDefaultNew(), protoType)))
	return buffer
}

// If one key is used in the context of more than one type, we should fail
func (g *goGenerator) tryStoreUsedVariable(usedVariables map[string]string, k string, t string) {
	existT, ok := usedVariables[k]
	if !ok {
		usedVariables[k] = t
		return
	}
	assert.Equal(t, existT)
}

// Recursively translate a rule, which is an n-ary tree. See lekko.rules.v1beta3.Rule.
func (g *goGenerator) translateRule(rule *rulesv1beta3.Rule, staticContext bool, usedVariables map[string]string, usedStrings, usedSlices *bool) string {
	if rule == nil {
		return ""
	}
	switch v := rule.GetRule().(type) {
	case *rulesv1beta3.Rule_Atom:
		var contextKeyName string
		if staticContext {
			contextKeyName = fmt.Sprintf("args.%s", strcase.ToCamel(v.Atom.ContextKey))
		} else {
			contextKeyName = strcase.ToLowerCamel(v.Atom.ContextKey)
		}

		switch v.Atom.GetComparisonOperator() {
		case rulesv1beta3.ComparisonOperator_COMPARISON_OPERATOR_EQUALS:
			if b, ok := v.Atom.ComparisonValue.GetKind().(*structpb.Value_BoolValue); ok {
				g.tryStoreUsedVariable(usedVariables, v.Atom.ContextKey, "bool")
				if b.BoolValue {
					return contextKeyName
				} else {
					return fmt.Sprintf("!%s", contextKeyName)
				}
			}
			g.tryStoreUsedVariable(usedVariables, v.Atom.ContextKey, structpbValueToKindStringGo(v.Atom.ComparisonValue))
			return fmt.Sprintf("%s == %s", contextKeyName, string(try.To1(protojson.Marshal(v.Atom.ComparisonValue))))
		case rulesv1beta3.ComparisonOperator_COMPARISON_OPERATOR_NOT_EQUALS:
			g.tryStoreUsedVariable(usedVariables, v.Atom.ContextKey, structpbValueToKindStringGo(v.Atom.ComparisonValue))
			return fmt.Sprintf("%s != %s", contextKeyName, string(try.To1(protojson.Marshal(v.Atom.ComparisonValue))))
		case rulesv1beta3.ComparisonOperator_COMPARISON_OPERATOR_LESS_THAN:
			g.tryStoreUsedVariable(usedVariables, v.Atom.ContextKey, structpbValueToKindStringGo(v.Atom.ComparisonValue))
			return fmt.Sprintf("%s < %s", contextKeyName, string(try.To1(protojson.Marshal(v.Atom.ComparisonValue))))
		case rulesv1beta3.ComparisonOperator_COMPARISON_OPERATOR_LESS_THAN_OR_EQUALS:
			g.tryStoreUsedVariable(usedVariables, v.Atom.ContextKey, structpbValueToKindStringGo(v.Atom.ComparisonValue))
			return fmt.Sprintf("%s <= %s", contextKeyName, string(try.To1(protojson.Marshal(v.Atom.ComparisonValue))))
		case rulesv1beta3.ComparisonOperator_COMPARISON_OPERATOR_GREATER_THAN:
			g.tryStoreUsedVariable(usedVariables, v.Atom.ContextKey, structpbValueToKindStringGo(v.Atom.ComparisonValue))
			return fmt.Sprintf("%s > %s", contextKeyName, string(try.To1(protojson.Marshal(v.Atom.ComparisonValue))))
		case rulesv1beta3.ComparisonOperator_COMPARISON_OPERATOR_GREATER_THAN_OR_EQUALS:
			g.tryStoreUsedVariable(usedVariables, v.Atom.ContextKey, structpbValueToKindStringGo(v.Atom.ComparisonValue))
			return fmt.Sprintf("%s >= %s", contextKeyName, string(try.To1(protojson.Marshal(v.Atom.ComparisonValue))))
		case rulesv1beta3.ComparisonOperator_COMPARISON_OPERATOR_CONTAINS:
			g.tryStoreUsedVariable(usedVariables, v.Atom.ContextKey, structpbValueToKindStringGo(v.Atom.ComparisonValue))
			*usedStrings = true
			return fmt.Sprintf("strings.Contains(%s,  %s)", contextKeyName, string(try.To1(protojson.Marshal(v.Atom.ComparisonValue))))
		case rulesv1beta3.ComparisonOperator_COMPARISON_OPERATOR_STARTS_WITH:
			g.tryStoreUsedVariable(usedVariables, v.Atom.ContextKey, structpbValueToKindStringGo(v.Atom.ComparisonValue))
			*usedStrings = true
			return fmt.Sprintf("strings.HasPrefix(%s,  %s)", contextKeyName, string(try.To1(protojson.Marshal(v.Atom.ComparisonValue))))
		case rulesv1beta3.ComparisonOperator_COMPARISON_OPERATOR_ENDS_WITH:
			g.tryStoreUsedVariable(usedVariables, v.Atom.ContextKey, structpbValueToKindStringGo(v.Atom.ComparisonValue))
			*usedStrings = true
			return fmt.Sprintf("strings.HasSuffix(%s,  %s)", contextKeyName, string(try.To1(protojson.Marshal(v.Atom.ComparisonValue))))
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
			g.tryStoreUsedVariable(usedVariables, v.Atom.ContextKey, sliceType)
			*usedSlices = true
			return fmt.Sprintf("slices.Contains([]%s{%s}, %s)", sliceType, strings.Join(elements, ", "), contextKeyName)
			// TODO, probably logical to have this here but we need slice syntax, use slices as of golang 1.21
		default:
			panic(fmt.Errorf("unsupported operator %+v", v.Atom.ComparisonOperator))
		}
	case *rulesv1beta3.Rule_Not:
		ruleStrFmt := "!%s"
		// For some cases, we want to wrap the generated Go expression string in parens
		switch rule := v.Not.Rule.(type) {
		case *rulesv1beta3.Rule_LogicalExpression:
			ruleStrFmt = "!(%s)"
		case *rulesv1beta3.Rule_Atom:
			if rule.Atom.ComparisonOperator != rulesv1beta3.ComparisonOperator_COMPARISON_OPERATOR_CONTAINED_WITHIN &&
				rule.Atom.ComparisonOperator != rulesv1beta3.ComparisonOperator_COMPARISON_OPERATOR_CONTAINS &&
				rule.Atom.ComparisonOperator != rulesv1beta3.ComparisonOperator_COMPARISON_OPERATOR_STARTS_WITH &&
				rule.Atom.ComparisonOperator != rulesv1beta3.ComparisonOperator_COMPARISON_OPERATOR_ENDS_WITH {
				ruleStrFmt = "!(%s)"
			}
		}
		return fmt.Sprintf(ruleStrFmt, g.translateRule(v.Not, staticContext, usedVariables, usedStrings, usedSlices))
	case *rulesv1beta3.Rule_LogicalExpression:
		operator := " && "
		switch v.LogicalExpression.GetLogicalOperator() {
		case rulesv1beta3.LogicalOperator_LOGICAL_OPERATOR_OR:
			operator = " || "
		}
		var result []string
		for _, rule := range v.LogicalExpression.Rules {
			ruleStrFmt := "%s"
			// If child is a nested logical expression, wrap in parens
			if l, nested := rule.Rule.(*rulesv1beta3.Rule_LogicalExpression); nested {
				// Exception: if current level is || and child is &&, we don't need parens
				// This technically depends on dev preference, we should pick one version and stick with it for canonicity
				if !(v.LogicalExpression.LogicalOperator == rulesv1beta3.LogicalOperator_LOGICAL_OPERATOR_OR && l.LogicalExpression.LogicalOperator == rulesv1beta3.LogicalOperator_LOGICAL_OPERATOR_AND) {
					ruleStrFmt = "(%s)"
				}
			}
			result = append(result, fmt.Sprintf(ruleStrFmt, g.translateRule(rule, staticContext, usedVariables, usedStrings, usedSlices)))
		}
		return strings.Join(result, operator)
	default:
		panic(fmt.Errorf("unsupported type of rule %+v", v))
	}
}

func (g *goGenerator) translateProtoFieldValue(parent protoreflect.Message, f protoreflect.FieldDescriptor, val protoreflect.Value) string {
	if f.IsMap() {
		// For map fields, f.Kind() is MessageKind but we need to handle key and value descriptors separately
		// TODO: Add support for protobuf type values
		assert.NotEqual(f.MapValue().Kind(), protoreflect.MessageKind, "unsupported protobuf type for map values")
		var lines []string
		res := fmt.Sprintf("map[%s]%s{", f.MapKey().Kind().String(), f.MapValue().Kind().String())
		val.Map().Range(func(mk protoreflect.MapKey, mv protoreflect.Value) bool {
			lines = append(lines, fmt.Sprintf("\"%s\": %s",
				mk.String(),
				g.translateProtoFieldValue(parent, f.MapValue(), mv)))
			return true
		})
		if len(lines) > 1 {
			slices.Sort(lines)
			res += "\n"
			res += strings.Join(lines, ",\n")
			res += ",\n}"
		} else {
			res += lines[0]
			res += "}"
		}
		return res
	} else if f.IsList() {
		// For list fields, f.Kind() is the type of each item (not necessarily MessageKind)
		lVal := val.List()
		var lines []string
		res := fmt.Sprintf("[]%s{", f.Kind().String())
		// For repeated messages, literal type can't just be stringified
		if f.Kind() == protoreflect.MessageKind {
			protoType := g.getProtoImportFromValue(parent, lVal.NewElement().Message()) // Not sure if this is the best way to get message type for list
			res = fmt.Sprintf("[]*%s.%s{", protoType.PackageAlias, protoType.Type)
		}
		for i := range lVal.Len() {
			lines = append(lines, g.translateProtoNonRepeatedValue(parent, f.Kind(), lVal.Get(i), true))
		}
		// Multiline formatting
		if len(lines) > 1 || (len(lines) == 1 && strings.Contains(lines[0], "\n")) {
			res += "\n"
			res += strings.Join(lines, ",\n")
			res += ",\n}"
		} else {
			res += strings.Join(lines, "")
			res += "}"
		}
		return res
	} else {
		return g.translateProtoNonRepeatedValue(parent, f.Kind(), val, false)
	}
}

func (g *goGenerator) translateProtoNonRepeatedValue(parent protoreflect.Message, kind protoreflect.Kind, val protoreflect.Value, omitLiteralType bool) string {
	switch kind {
	case protoreflect.EnumKind:
		// TODO: Actually handle enums, right now they're just numbers
		return val.String()
	case protoreflect.StringKind:
		return g.toQuoted(val.String())
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
		// Don't need to do anything special for numerics
		return val.String()
	case protoreflect.MessageKind:
		// TODO - Maps are a special thing - do they work here?
		return g.translateProtoValue(parent, val.Message(), omitLiteralType, make([]*featurev1beta1.ValueOveride, 0)) // TODO - nesting sucks.. need to redo all this shit
	default:
		panic(fmt.Errorf("unsupported proto value type: %v", kind))
	}
}

func (g *goGenerator) translateAnyValue(val *featurev1beta1.Any, protoType *ProtoImport) string {
	if val.TypeUrl == "type.googleapis.com/lekko.feature.v1beta1.ConfigCall" {
		call := &featurev1beta1.ConfigCall{}
		err := proto.Unmarshal(val.Value, call)
		if err != nil {
			panic(err)
		}
		var funcNameBuilder strings.Builder
		funcNameBuilder.WriteString("Get")
		for _, word := range regexp.MustCompile("[_-]+").Split(call.Key, -1) {
			funcNameBuilder.WriteString(strings.ToUpper(word[:1]) + word[1:])
		}
		funcName := funcNameBuilder.String()
		privateFunc := strcase.ToLowerCamel(funcName)
		return privateFunc + "()"
	}
	mt, err := g.TypeRegistry.FindMessageByURL(val.TypeUrl)
	if err != nil {
		panic(err)
	}
	msg := mt.New().Interface()
	err = proto.UnmarshalOptions{Resolver: g.TypeRegistry}.Unmarshal(val.Value, msg)
	if err != nil {
		panic(errors.Wrapf(err, "%s", val.TypeUrl))
	}

	if protoType == nil { // This is a jank way of handling google WKT
		var ret string
		switch val.TypeUrl {
		case "type.googleapis.com/google.protobuf.BoolValue":
			ret = "false"
		case "type.googleapis.com/google.protobuf.StringValue":
			ret = "\"\""
		case "type.googleapis.com/google.protobuf.DoubleValue":
			ret = "0.0"
		case "type.googleapis.com/google.protobuf.Int32Value":
			ret = "0"
		case "type.googleapis.com/google.protobuf.Int64Value":
			ret = "0"
		}

		msg.ProtoReflect().Range(func(fd protoreflect.FieldDescriptor, v protoreflect.Value) bool {
			switch fd.Kind() {
			case protoreflect.StringKind:
				ret = g.toQuoted(v.String())
			case protoreflect.Int64Kind:
				ret = strconv.FormatInt(v.Int(), 10)
			case protoreflect.BoolKind:
				if v.Bool() {
					ret = "true"
				} else {
					ret = "false"
				}
			default:
				//lint:ignore S1025 Reason for ignoring this warning
				ret = fmt.Sprintf("%s", v)
			}
			// limit this to just wrappers.. wtf proto libraries
			return true
		})
		return ret
	}
	return g.translateProtoValue(nil, msg.ProtoReflect(), false, val.Overrides)
}

func (g *goGenerator) translateProtoValue(parent protoreflect.Message, val protoreflect.Message, omitLiteralType bool, overrides []*featurev1beta1.ValueOveride) string {
	protoType := g.getProtoImportFromValue(parent, val)
	fields := make(map[int]string)
	for _, override := range overrides {
		fieldNumber := override.FieldPath[0]
		fd := val.Type().Descriptor().Fields().ByNumber(protoreflect.FieldNumber(fieldNumber))
		if fd == nil {
			panic(fmt.Errorf("field number %d not found in message", fieldNumber))
		}
		var funcNameBuilder strings.Builder
		funcNameBuilder.WriteString("Get")
		for _, word := range regexp.MustCompile("[_-]+").Split(override.Call.Key, -1) {
			funcNameBuilder.WriteString(strings.ToUpper(word[:1]) + word[1:])
		}
		funcName := funcNameBuilder.String()
		privateFunc := strcase.ToLowerCamel(funcName)
		fields[int(fieldNumber)] = fmt.Sprintf("%s: %s()", strcase.ToCamel(fd.TextName()), privateFunc)
	}

	var lines []string
	val.Range(func(fd protoreflect.FieldDescriptor, fv protoreflect.Value) bool {
		fields[int(fd.Number())] = fmt.Sprintf("%s: %s", strcase.ToCamel(fd.TextName()), g.translateProtoFieldValue(val, fd, fv))
		return true
	})
	literalType := ""
	if !omitLiteralType {
		if protoType.PackageAlias == "" {
			literalType = fmt.Sprintf("&%s", protoType.Type)
		} else {
			literalType = fmt.Sprintf("&%s.%s", protoType.PackageAlias, protoType.Type)
		}
	}

	keys := []int{}
	for key := range fields {
		keys = append(keys, key)
	}
	sort.Ints(keys)

	for _, key := range keys {
		lines = append(lines, fields[key])
	}

	if len(lines) > 1 || (len(lines) == 1 && strings.Contains(lines[0], "\n")) {
		slices.Sort(lines)
		// Replace this with interface pointing stuff
		return fmt.Sprintf("%s{\n%s,\n}", literalType, strings.Join(lines, ",\n"))
	} else {
		return fmt.Sprintf("%s{%s}", literalType, strings.Join(lines, ""))
	}
}

// Takes a string and returns a double-quoted literal with applicable internal characters escaped, etc.
// For strings with newlines, returns a raw string literal instead.
// TODO - this type of thing might be a lot simpler with the AST library and the Printer
func (g *goGenerator) toQuoted(s string) string {
	if strings.Count(s, "\n") > 0 {
		return fmt.Sprintf("`%s`", s)
	}
	// Quote automatically handles escaping, etc.
	return strconv.Quote(s)
}

// Handles getting import & type literal information for both top-level and nested messages.
// TODO: Consider moving logic into UnpackProtoType directly which is shared with TS codegen as well
// TODO: This can definitely be cached, and doesn't need values, just descriptors
func (g *goGenerator) getProtoImportFromValue(parent protoreflect.Message, val protoreflect.Message) *ProtoImport {
	// Try finding in type registry
	_, err := g.TypeRegistry.FindMessageByName((val.Descriptor().FullName()))
	if errors.Is(err, protoregistry.NotFound) {
		// If there's no parent, this can't be a nested message, which is a problem
		assert.NotEqual(parent, nil, "missing in type registry with no parent")
		// Try finding in parent's nested message definitions
		nestedMsgDesc := parent.Descriptor().Messages().ByName(val.Descriptor().Name())
		if nestedMsgDesc == nil {
			panic(fmt.Sprintf("unable to find message type %s", val.Descriptor().FullName()))
		}
		parentProtoType := g.getProtoImportFromValue(nil, parent)
		return &ProtoImport{
			ImportPath:   parentProtoType.ImportPath,
			PackageAlias: parentProtoType.PackageAlias,
			Type:         fmt.Sprintf("%s_%s", parentProtoType.Type, string(nestedMsgDesc.Name())),
		}
	} else if err != nil {
		panic(errors.Wrap(err, "unknown error while checking type registry"))
	} else {
		// Found in type registry (i.e. top-level message)
		return UnpackProtoType(g.moduleRoot, g.lekkoPath, string(val.Descriptor().FullName()))
	}
}

// TODO: Generify
// Get all unique possible return values of a config
func (g *goGenerator) getStringRetValues(f *featurev1beta1.Feature) []string {
	if f.Type != featurev1beta1.FeatureType_FEATURE_TYPE_STRING {
		return []string{}
	}
	valSet := make(map[string]bool)
	valSet[g.translateAnyValue(f.Tree.DefaultNew, nil)] = true
	for _, constraint := range f.Tree.Constraints {
		ret := g.translateAnyValue(constraint.ValueNew, nil)
		valSet[ret] = true
	}
	var rets []string
	for val := range valSet {
		rets = append(rets, val)
	}
	sort.Strings(rets)
	return rets
}

func (g *goGenerator) genClientFile(moduleRoot string) (err error) {
	defer err2.Handle(&err)
	// Template for generated client initialization code.
	const clientTemplateBody = `// Generated by Lekko. DO NOT EDIT.
package lekko

import (
	"context"

	client "github.com/lekkodev/go-sdk/client"
	{{- range $.Namespaces}}
	{{nsToImport .}}{{end}}
)

type LekkoClient struct {
	{{- range $.Namespaces}}
	{{nsToClientFieldType .}}{{end}}
	Close client.CloseFunc
}

// Initializes the Lekko SDK client.
// For remote configs to be fetched correctly, the LEKKO_API_KEY env variable is required.
// If the env variable is missing or if there are any connection errors, the static fallbacks will be used.
func NewLekkoClient(ctx context.Context, opts ...client.ProviderOption) *LekkoClient {
	repoOwner := "{{$.RepositoryOwner}}"
	repoName := "{{$.RepositoryName}}"
	cli, close := client.NewClientFromEnv(ctx, repoOwner, repoName, opts...)
	return &LekkoClient{
		{{- range $.Namespaces}}
		{{nsToClientField .}},{{end}}
		Close: close,
	}
}
`

	// TODO: Refactor to just pass dotlekko when constructing generator
	dot := try.To1(dotlekko.ReadDotLekko(""))
	repoOwner, repoName := dot.GetRepoInfo()

	clientTemplateData := struct {
		Namespaces      []string
		RepositoryOwner string
		RepositoryName  string
	}{
		[]string{},
		repoOwner,
		repoName,
	}
	clientTemplateFuncs := map[string]any{
		"nsToImport": func(ns string) string {
			return fmt.Sprintf("lekko%s \"%s/%s/%s\"", ns, moduleRoot, g.lekkoPath, ns)
		},
		"nsToClientFieldType": func(ns string) string {
			return fmt.Sprintf("%s *lekko%s.LekkoClient", strcase.ToCamel(ns), ns)
		},
		"nsToClientField": func(ns string) string {
			return fmt.Sprintf("%s: &lekko%s.LekkoClient{Client: cli}", strcase.ToCamel(ns), ns)
		},
	}
	// Walk through lekko/ directory to find namespaces
	// We walk through dir instead of just using the namespace from above because shared client init code should include every namespace
	clientTemplateData.Namespaces = try.To1(native.ListNamespaces(g.outputPath, native.LangGo))
	var contents bytes.Buffer
	templ := template.Must(template.New("client").Funcs(clientTemplateFuncs).Parse(clientTemplateBody))
	if err := templ.Execute(&contents, clientTemplateData); err != nil {
		return errors.Wrap(err, "generate client initialization code: template exec")
	}
	formatted, err := format.Source(contents.Bytes())
	if err != nil {
		return errors.Wrap(err, "generation client initialization code: format")
	}
	f, err := os.Create(path.Join(g.outputPath, "client_gen.go"))
	if err != nil {
		return errors.Wrap(err, "generate client initialization code: create file")
	}
	if _, err := f.Write(formatted); err != nil {
		return errors.Wrap(err, fmt.Sprintf("write formatted contents to %s", f.Name()))
	}
	return nil
}

func DescriptorToStructDeclaration(d protoreflect.MessageDescriptor) string {
	var result string
	result += fmt.Sprintf("type %s struct {\n", d.Name()) // TODO
	fields := d.Fields()
	for i := 0; i < fields.Len(); i++ {
		field := fields.Get(i)
		fieldName := strcase.ToCamel(string(field.Name()))
		goType := FieldDescriptorToGoTypeString(field)
		result += fmt.Sprintf("\t%s %s;\n", fieldName, goType)
	}
	result += "}\n"
	return result
}

func FieldDescriptorToGoTypeString(field protoreflect.FieldDescriptor) string {
	goType := ""
	if field.Cardinality() == protoreflect.Repeated {
		goType = "[]"
	}
	switch field.Kind() {
	case protoreflect.BoolKind:
		goType += "bool"
	case protoreflect.Int32Kind, protoreflect.Sint32Kind, protoreflect.Sfixed32Kind:
		goType += "int32"
	case protoreflect.Int64Kind, protoreflect.Sint64Kind, protoreflect.Sfixed64Kind:
		goType += "int64"
	case protoreflect.Uint32Kind, protoreflect.Fixed32Kind:
		goType += "uint32"
	case protoreflect.Uint64Kind, protoreflect.Fixed64Kind:
		goType += "uint64"
	case protoreflect.FloatKind:
		goType += "float32"
	case protoreflect.DoubleKind:
		goType += "float64"
	case protoreflect.StringKind:
		goType += "string"
	case protoreflect.BytesKind:
		goType += "[]byte"
	case protoreflect.MessageKind, protoreflect.GroupKind:
		if field.IsMap() {
			keyField := field.MapKey()
			valueField := field.MapValue()
			goType = "map[" + FieldDescriptorToGoTypeString(keyField) + "]" + FieldDescriptorToGoTypeString(valueField)
		} else if field.Message().FullName() == "google.protobuf.Duration" {
			goType = "*durationpb.Duration"
		} else {
			goType = "*" + string(field.Message().Name())
		}
	case protoreflect.EnumKind:
		goType = string(field.Enum().Name())
	default:
		goType = "interface{}"
	}
	return goType
}
