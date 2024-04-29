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
	"io/fs"
	"os"
	"os/exec"
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
	"github.com/AlecAivazis/survey/v2"
	"github.com/iancoleman/strcase"
	"github.com/lainio/err2/assert"
	"github.com/lainio/err2/try"
	"github.com/lekkodev/cli/pkg/dotlekko"
	"github.com/lekkodev/cli/pkg/repo"
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

// TODO: this can hold more state to clean up functions a bit, like storing usedVariables, etc.
type goGenerator struct {
	moduleRoot   string
	outputPath   string // Location for destination file, can be absolute or relative. Its suffix should be the same as lekkoPath. In most cases can be same as lekkoPath.
	lekkoPath    string // Location relative to project root where Lekko files are stored, e.g. internal/lekko.
	repoPath     string
	namespace    string
	typeRegistry *protoregistry.Types
}

func NewGoGenerator(moduleRoot, outputPath, lekkoPath, repoPath, namespace string) *goGenerator {
	return &goGenerator{
		moduleRoot: moduleRoot,
		outputPath: outputPath,
		lekkoPath:  filepath.Clean(lekkoPath),
		repoPath:   repoPath,
		namespace:  namespace,
	}
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

func GenGoCmd() *cobra.Command {
	var ns string
	var outputPath string
	var repoPath string
	var initMode bool
	cmd := &cobra.Command{
		Use:   "go",
		Short: "generate Go library code from configs",
		RunE: func(cmd *cobra.Command, args []string) error {
			b, err := os.ReadFile("go.mod")
			if err != nil {
				return errors.Wrap(err, "find go.mod in working directory")
			}
			mf, err := modfile.ParseLax("go.mod", b, nil)
			if err != nil {
				return err
			}
			if len(outputPath) == 0 {
				dot, err := dotlekko.ReadDotLekko()
				if err != nil {
					return err
				}
				outputPath = dot.LekkoPath
			}
			if len(repoPath) == 0 {
				repoPath, err = repo.PrepareGithubRepo()
				if err != nil {
					return err
				}
			}
			if len(ns) == 0 {
				if err := survey.AskOne(&survey.Input{
					Message: "Namespace:",
					Help:    "Lekko namespace to generate code for, determines Go package name",
				}, &ns); err != nil {
					return errors.Wrap(err, "namespace prompt")
				}
			}
			// TODO: Change this to a survey validator so it can keep re-asking
			if !regexp.MustCompile("[a-z]+").MatchString(ns) {
				return errors.New("namespace must be a lowercase alphanumeric string")
			}
			if ns == "proto" {
				return errors.New("'proto' is a reserved name")
			}
			generator := NewGoGenerator(mf.Module.Mod.Path, outputPath, outputPath, repoPath, ns)
			if initMode {
				return generator.Init(cmd.Context())
			}
			return generator.Gen(cmd.Context())
		},
	}
	cmd.Flags().StringVarP(&ns, "namespace", "n", "", "namespace to generate code from")
	cmd.Flags().StringVarP(&outputPath, "output-path", "o", "", "path to write generated directories and Go files under, autodetects if not set")
	cmd.Flags().StringVarP(&repoPath, "repo-path", "r", "", "path to config repository, autodetects if not set")
	cmd.Flags().BoolVar(&initMode, "init", false, "pass 'init' to generate boilerplate code for a Lekko namespace")
	return cmd
}

// Initialize a blank Lekko config function file
func (g *goGenerator) Init(ctx context.Context) error {
	const templateBody = `package lekko{{$.Namespace}}

// This is an example description for an example config
func getExample() bool {
	return true
}`
	fullOutputPath := path.Join(g.outputPath, g.namespace, "lekko.go")
	if _, err := os.Stat(fullOutputPath); err == nil {
		return fmt.Errorf("file %s already exists", fullOutputPath)
	}

	data := struct {
		Namespace string
	}{
		Namespace: g.namespace,
	}
	var contents bytes.Buffer
	templ := template.Must(template.New("lekko.go").Parse(templateBody))
	if err := templ.Execute(&contents, data); err != nil {
		return errors.Wrap(err, "lekko.go template")
	}
	formatted, err := format.Source(contents.Bytes())
	if err != nil {
		return errors.Wrap(err, "format lekko.go")
	}
	if err := os.MkdirAll(path.Join(g.outputPath, g.namespace), 0770); err != nil {
		return err
	}
	f, err := os.Create(fullOutputPath)
	if err != nil {
		return errors.Wrap(err, "create lekko.go")
	}
	if _, err := f.Write(formatted); err != nil {
		return errors.Wrap(err, "write lekko.go")
	}
	return nil
}

func (g *goGenerator) Gen(ctx context.Context) error {
	r, err := repo.NewLocal(g.repoPath, nil)
	if err != nil {
		return errors.Wrap(err, "read config repository")
	}
	rootMD, nsMDs := try.To2(r.ParseMetadata(ctx))
	// TODO this feels weird and there is a global set we should be able to add to but I'll worrry about it later?
	g.typeRegistry = try.To1(r.BuildDynamicTypeRegistry(ctx, rootMD.ProtoDirectory))
	nsMD, ok := nsMDs[g.namespace]
	if !ok {
		return fmt.Errorf("%s is not a namespace in the config repository", g.namespace)
	}
	staticCtxType := UnpackProtoType(g.moduleRoot, g.lekkoPath, nsMD.ContextProto)
	ffs, err := r.GetFeatureFiles(ctx, g.namespace)
	if err != nil {
		return err
	}
	// Sort configs in alphabetical order
	sort.SliceStable(ffs, func(i, j int) bool {
		return ffs[i].CompiledProtoBinFileName < ffs[j].CompiledProtoBinFileName
	})

	var publicFuncStrings []string
	var privateFuncStrings []string
	protoImportSet := make(map[string]*ProtoImport)
	addStringsImport := false
	addSlicesImport := false
	if staticCtxType != nil {
		protoImportSet[staticCtxType.ImportPath] = staticCtxType
	}
	for _, ff := range ffs {
		fff, err := os.ReadFile(path.Join(g.repoPath, g.namespace, ff.CompiledProtoBinFileName))
		if err != nil {
			return err
		}
		f := &featurev1beta1.Feature{}
		if err := proto.Unmarshal(fff, f); err != nil {
			return err
		}
		generated, err := g.genGoForFeature(ctx, r, f, g.namespace, staticCtxType)
		if err != nil {
			return errors.Wrapf(err, "generate code for %s/%s", g.namespace, f.Key)
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
			protoImport := UnpackProtoType(g.moduleRoot, g.lekkoPath, f.Tree.Default.TypeUrl)
			protoImportSet[protoImport.ImportPath] = protoImport
		}
	}
	// For each namespace, we want to generate under lekko/:
	// lekko/
	//   <namespace>/
	//     lekko.go
	//     lekko_gen.go

	// lekko.go (maybe should use namespace name?) is meant to be edited by the user, contains private native lang funcs
	// lekko_gen.go is marked as machine-generated, contains public funcs to be used in application code

	const publicFileTemplateBody = `// Generated by Lekko. DO NOT EDIT.
package lekko{{$.Namespace}}

import (
	"context"
{{range $.ProtoImports}}
	{{ . }}{{end}}
	client "github.com/lekkodev/go-sdk/client"
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

{{range $.PrivateFuncStrings}}
{{ . }}{{end}}`

	// buf generate --template '{"version":"v1","plugins":[{"plugin":"go","out":"gen/go"}]}'
	//
	// This generates the code for the config repo, assuming it has a buf.gen.yml in that repo.
	// In OUR repos, and maybe some of our customers, they may already have a buf.gen.yml, so if
	// that is the case, we should identify that, not run code gen (maybe?) and instead need to
	// take the prefix by parsing the buf.gen.yml to understand where the go code goes.
	//
	// With proto packages with >= 3 segments, the go package only takes the last 2.
	// e.g. default.config.v1beta1 -> configv1beta1
	// https://github.com/bufbuild/buf/issues/1263
	pCmd := exec.Command(
		"buf",
		"generate",
		// TODO: Fix the hardcoded stuff
		fmt.Sprintf(`--template={"managed": {"enabled": true, "go_package_prefix": {"default": "%s/%s/proto"}}, "version":"v1","plugins":[{"plugin":"buf.build/protocolbuffers/go:v1.33.0","out":"%s/proto", "opt": "paths=source_relative"}]}`, g.moduleRoot, g.lekkoPath, g.outputPath),
		"--include-imports",
		g.repoPath) // #nosec G204
	pCmd.Dir = "."
	if out, err := pCmd.CombinedOutput(); err != nil {
		fmt.Printf("Error when generating code with buf: %s\n %e\n", out, err)
		return err
	}
	if err := os.MkdirAll(path.Join(g.outputPath, g.namespace), 0770); err != nil {
		return err
	}
	var protoImports []string
	for _, imp := range protoImportSet {
		protoImports = append(protoImports, fmt.Sprintf(`%s "%s"`, imp.PackageAlias, imp.ImportPath))
	}
	data := struct {
		ProtoImports       []string
		Namespace          string
		PublicFuncStrings  []string
		PrivateFuncStrings []string
		AddStringsImport   bool
		AddSlicesImport    bool
	}{
		protoImports,
		g.namespace,
		publicFuncStrings,
		privateFuncStrings,
		addStringsImport,
		addSlicesImport,
	}
	outputs := []struct {
		templateBody string
		fileName     string
	}{
		{
			templateBody: publicFileTemplateBody,
			fileName:     "lekko_gen.go",
		},
		{
			templateBody: privateFileTemplateBody,
			fileName:     "lekko.go",
		},
	}
	for _, output := range outputs {
		var contents bytes.Buffer
		templ := template.Must(template.New(output.fileName).Parse(output.templateBody))
		if err := templ.Execute(&contents, data); err != nil {
			return errors.Wrap(err, fmt.Sprintf("%s template", output.fileName))
		}
		// Final canonical Go format
		formatted, err := format.Source(contents.Bytes())
		if err != nil {
			return errors.Wrap(err, fmt.Sprintf("format %s", path.Join(g.lekkoPath, output.fileName)))
		}
		if f, err := os.Create(path.Join(g.outputPath, g.namespace, output.fileName)); err != nil {
			return err
		} else {
			if _, err := f.Write(formatted); err != nil {
				return errors.Wrap(err, fmt.Sprintf("write formatted contents to %s", f.Name()))
			}
		}
	}

	// Under lekko/, we also generate
	// lekko/
	//   client_gen.go <--
	//   <namespace>/
	//     ...

	// This is the "entrypoint" shared client initialization code that can be imported by the user
	// and allows access to all namespaces
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
func (c *LekkoClient) {{$.FuncName}}({{$.ArgumentString}}) {{$.RetType}} {
  	{{ $.CtxStuff }}
  	result, err := c.{{$.GetFunction}}(ctx, "{{$.Namespace}}", "{{$.Key}}")
	if err == nil {
	  	return result
  	}
  	return {{$.PrivateFunc}}({{$.CallString}})
}`,
		private: `// {{$.Description}}
func {{$.PrivateFunc}}({{$.ArgumentString}}) {{$.RetType}} {
{{range  $.NaturalLanguage}}{{ . }}
{{end}}}`,
	}
}

// Template body for proto config code
func (g *goGenerator) getProtoTemplateBody() *configCodeTemplate {
	return &configCodeTemplate{
		public: `// {{$.Description}}
func (c *LekkoClient) {{$.FuncName}}({{$.ArgumentString}}) *{{$.RetType}} {
		{{ $.CtxStuff }}
	result := &{{$.RetType}}{}
	err := c.{{$.GetFunction}}(ctx, "{{$.Namespace}}", "{{$.Key}}", result)
	if err == nil {
			return result
		}
		return {{$.PrivateFunc}}({{$.CallString}})
}`,
		private: `// {{$.Description}}
func {{$.PrivateFunc}}({{$.ArgumentString}}) *{{$.RetType}} {
{{range  $.NaturalLanguage}}{{ . }}
{{end}}}`,
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
func (c *LekkoClient) {{$.FuncName}}({{$.ArgumentString}}) {{$.RetType}} {
		{{ $.CtxStuff }}
		result, err := c.{{$.GetFunction}}(ctx, "{{$.Namespace}}", "{{$.Key}}")
	if err == nil {
			return result
		}
		return {{$.PrivateFunc}}({{$.CallString}})
}`,
		private: `type {{$.EnumTypeName}} string
const (
	{{range $index, $field := $.EnumFields}}{{$field.Name}} {{$.EnumTypeName}} = "{{$field.Value}}"
	{{end}}
)

// {{$.Description}}
func {{$.PrivateFunc}}({{$.ArgumentString}}) {{$.RetType}} {
{{range  $.NaturalLanguage}}{{ . }}
{{end}}}`,
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
		// HACK: The metadata field is only for presentation at the moment
		// so is not part of the compiled object - need to statically parse
		// This also means that this only works for statically parseable
		// configs
		sf, err := r.Parse(ctx, ns, f.Key, g.typeRegistry)
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
	case featurev1beta1.FeatureType_FEATURE_TYPE_JSON:
		// TODO: Actually figure out how we want to do json configs
		return nil, fmt.Errorf("unsupported json config %s/%s", ns, f.Key)
	case featurev1beta1.FeatureType_FEATURE_TYPE_PROTO:
		getFunction = "GetProto"
		templateBody = g.getProtoTemplateBody()
		protoType = UnpackProtoType(g.moduleRoot, g.lekkoPath, f.Tree.Default.TypeUrl)
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
	generated := &generatedConfigCode{}
	usedVariables := make(map[string]string)
	if staticCtxType != nil {
		data.NaturalLanguage = g.translateFeature(f, protoType, true, usedVariables, &generated.usedStrings, &generated.usedSlices)
		data.ArgumentString = fmt.Sprintf("ctx *%s.%s", staticCtxType.PackageAlias, staticCtxType.Type)
		data.CallString = "ctx"
	} else {
		data.CtxStuff = "ctx := context.Background()\n"
		data.NaturalLanguage = g.translateFeature(f, protoType, false, usedVariables, &generated.usedStrings, &generated.usedSlices)
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
		data.CtxStuff += strings.Join(ctxAddLines, "\n")
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
		buffer = append(buffer, fmt.Sprintf("\t\treturn %s", g.translateAnyValue(constraint.Value, protoType)))
	}
	if len(f.Tree.Constraints) > 0 {
		buffer = append(buffer, "\t}")
	}
	buffer = append(buffer, fmt.Sprintf("\treturn %s", g.translateAnyValue(f.GetTree().GetDefault(), protoType)))
	return buffer
}

// If one key is used in the context of more than one type, we should fail
func (g *goGenerator) tryStoreUsedVariable(usedVariables map[string]string, k string, t string) {
	existT, ok := usedVariables[k]
	if !ok {
		usedVariables[k] = t
		return
	}
	// TODO: test with err2 handlers to handle more gracefully
	assert.Equal(t, existT)
}

func (g *goGenerator) translateRule(rule *rulesv1beta3.Rule, staticContext bool, usedVariables map[string]string, usedStrings, usedSlices *bool) string {
	if rule == nil {
		return ""
	}
	// TODO: Do we actually want to case context keys in terms of cross language?
	switch v := rule.GetRule().(type) {
	case *rulesv1beta3.Rule_Atom:
		switch v.Atom.GetComparisonOperator() {
		case rulesv1beta3.ComparisonOperator_COMPARISON_OPERATOR_EQUALS:
			g.tryStoreUsedVariable(usedVariables, v.Atom.ContextKey, structpbValueToKindStringGo(v.Atom.ComparisonValue))
			if staticContext {
				return fmt.Sprintf("ctx.%s == %s", strcase.ToCamel(v.Atom.ContextKey), string(try.To1(protojson.Marshal(v.Atom.ComparisonValue))))
			} else {
				return fmt.Sprintf("%s == %s", strcase.ToLowerCamel(v.Atom.ContextKey), string(try.To1(protojson.Marshal(v.Atom.ComparisonValue))))
			}
		case rulesv1beta3.ComparisonOperator_COMPARISON_OPERATOR_NOT_EQUALS:
			g.tryStoreUsedVariable(usedVariables, v.Atom.ContextKey, structpbValueToKindStringGo(v.Atom.ComparisonValue))
			if staticContext {
				return fmt.Sprintf("ctx.%s != %s", strcase.ToCamel(v.Atom.ContextKey), string(try.To1(protojson.Marshal(v.Atom.ComparisonValue))))
			} else {
				return fmt.Sprintf("%s != %s", strcase.ToLowerCamel(v.Atom.ContextKey), string(try.To1(protojson.Marshal(v.Atom.ComparisonValue))))
			}
		case rulesv1beta3.ComparisonOperator_COMPARISON_OPERATOR_LESS_THAN:
			g.tryStoreUsedVariable(usedVariables, v.Atom.ContextKey, structpbValueToKindStringGo(v.Atom.ComparisonValue))
			if staticContext {
				return fmt.Sprintf("ctx.%s < %s", strcase.ToCamel(v.Atom.ContextKey), string(try.To1(protojson.Marshal(v.Atom.ComparisonValue))))
			} else {
				return fmt.Sprintf("%s < %s", strcase.ToLowerCamel(v.Atom.ContextKey), string(try.To1(protojson.Marshal(v.Atom.ComparisonValue))))
			}
		case rulesv1beta3.ComparisonOperator_COMPARISON_OPERATOR_LESS_THAN_OR_EQUALS:
			g.tryStoreUsedVariable(usedVariables, v.Atom.ContextKey, structpbValueToKindStringGo(v.Atom.ComparisonValue))
			if staticContext {
				return fmt.Sprintf("ctx.%s <= %s", strcase.ToCamel(v.Atom.ContextKey), string(try.To1(protojson.Marshal(v.Atom.ComparisonValue))))
			} else {
				return fmt.Sprintf("%s <= %s", strcase.ToLowerCamel(v.Atom.ContextKey), string(try.To1(protojson.Marshal(v.Atom.ComparisonValue))))
			}
		case rulesv1beta3.ComparisonOperator_COMPARISON_OPERATOR_GREATER_THAN:
			g.tryStoreUsedVariable(usedVariables, v.Atom.ContextKey, structpbValueToKindStringGo(v.Atom.ComparisonValue))
			if staticContext {
				return fmt.Sprintf("ctx.%s > %s", strcase.ToCamel(v.Atom.ContextKey), string(try.To1(protojson.Marshal(v.Atom.ComparisonValue))))
			} else {
				return fmt.Sprintf("%s > %s", strcase.ToLowerCamel(v.Atom.ContextKey), string(try.To1(protojson.Marshal(v.Atom.ComparisonValue))))
			}
		case rulesv1beta3.ComparisonOperator_COMPARISON_OPERATOR_GREATER_THAN_OR_EQUALS:
			g.tryStoreUsedVariable(usedVariables, v.Atom.ContextKey, structpbValueToKindStringGo(v.Atom.ComparisonValue))
			if staticContext {
				return fmt.Sprintf("ctx.%s >= %s", strcase.ToCamel(v.Atom.ContextKey), string(try.To1(protojson.Marshal(v.Atom.ComparisonValue))))
			} else {
				return fmt.Sprintf("%s >= %s", strcase.ToLowerCamel(v.Atom.ContextKey), string(try.To1(protojson.Marshal(v.Atom.ComparisonValue))))
			}
		case rulesv1beta3.ComparisonOperator_COMPARISON_OPERATOR_CONTAINS:
			g.tryStoreUsedVariable(usedVariables, v.Atom.ContextKey, structpbValueToKindStringGo(v.Atom.ComparisonValue))
			*usedStrings = true
			if staticContext {
				return fmt.Sprintf("strings.Contains(ctx.%s, %s)", strcase.ToCamel(v.Atom.ContextKey), string(try.To1(protojson.Marshal(v.Atom.ComparisonValue))))
			} else {
				return fmt.Sprintf("strings.Contains(%s,  %s)", strcase.ToLowerCamel(v.Atom.ContextKey), string(try.To1(protojson.Marshal(v.Atom.ComparisonValue))))
			}
		case rulesv1beta3.ComparisonOperator_COMPARISON_OPERATOR_STARTS_WITH:
			g.tryStoreUsedVariable(usedVariables, v.Atom.ContextKey, structpbValueToKindStringGo(v.Atom.ComparisonValue))
			*usedStrings = true
			if staticContext {
				return fmt.Sprintf("strings.HasPrefix(ctx.%s, %s)", strcase.ToCamel(v.Atom.ContextKey), string(try.To1(protojson.Marshal(v.Atom.ComparisonValue))))
			} else {
				return fmt.Sprintf("strings.HasPrefix(%s,  %s)", strcase.ToLowerCamel(v.Atom.ContextKey), string(try.To1(protojson.Marshal(v.Atom.ComparisonValue))))
			}
		case rulesv1beta3.ComparisonOperator_COMPARISON_OPERATOR_ENDS_WITH:
			g.tryStoreUsedVariable(usedVariables, v.Atom.ContextKey, structpbValueToKindStringGo(v.Atom.ComparisonValue))
			*usedStrings = true
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
			g.tryStoreUsedVariable(usedVariables, v.Atom.ContextKey, sliceType)
			*usedSlices = true
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
			result = append(result, g.translateRule(rule, staticContext, usedVariables, usedStrings, usedSlices))
		}
		return strings.Join(result, operator)
	}

	fmt.Printf("Need to learn how to: %+v\n", rule.GetRule())
	return ""
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
		// If the string value is multiline, transform to raw literal instead
		valString := val.String()
		quote := "\""
		if strings.Count(valString, "\n") > 0 {
			quote = "`"
		}
		return fmt.Sprintf("%s%s%s", quote, val.String(), quote)
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
		return g.translateProtoValue(parent, val.Message(), omitLiteralType)
	default:
		panic(fmt.Errorf("unsupported proto value type: %v", kind))
	}
}

func (g *goGenerator) translateAnyValue(val *anypb.Any, protoType *ProtoImport) string {
	msg := try.To1(anypb.UnmarshalNew(val, proto.UnmarshalOptions{Resolver: g.typeRegistry}))
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
		if val.MessageIs((*wrapperspb.StringValue)(nil)) {
			var s wrapperspb.StringValue
			try.To(val.UnmarshalTo(&s))
			// If multiline string value, gen as raw string literal instead
			quote := "\""
			if strings.Count(s.Value, "\n") > 0 {
				quote = "`"
			}
			return fmt.Sprintf("%s%s%s", quote, s.Value, quote)
		}
		return string(try.To1(protojson.Marshal(msg)))
	}
	return g.translateProtoValue(nil, msg.ProtoReflect(), false)
}

func (g *goGenerator) translateProtoValue(parent protoreflect.Message, val protoreflect.Message, omitLiteralType bool) string {
	protoType := g.getProtoImportFromValue(parent, val)
	var lines []string
	val.Range(func(fd protoreflect.FieldDescriptor, fv protoreflect.Value) bool {
		lines = append(lines, fmt.Sprintf("%s: %s", strcase.ToCamel(fd.TextName()), g.translateProtoFieldValue(val, fd, fv)))
		return true
	})
	literalType := ""
	if !omitLiteralType {
		literalType = fmt.Sprintf("&%s.%s", protoType.PackageAlias, protoType.Type)
	}
	if len(lines) > 1 || (len(lines) == 1 && strings.Contains(lines[0], "\n")) {
		slices.Sort(lines)
		// Replace this with interface pointing stuff
		return fmt.Sprintf("%s{\n%s,\n}", literalType, strings.Join(lines, ",\n"))
	} else {
		return fmt.Sprintf("%s{%s}", literalType, strings.Join(lines, ""))
	}
}

// Handles getting import & type literal information for both top-level and nested messages.
// TODO: Consider moving logic into UnpackProtoType directly which is shared with TS codegen as well
// TODO: This can definitely be cached, and doesn't need values, just descriptors
func (g *goGenerator) getProtoImportFromValue(parent protoreflect.Message, val protoreflect.Message) *ProtoImport {
	// Try finding in type registry
	_, err := g.typeRegistry.FindMessageByName((val.Descriptor().FullName()))
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
	valSet[g.translateAnyValue(f.Tree.Default, nil)] = true
	for _, constraint := range f.Tree.Constraints {
		ret := g.translateAnyValue(constraint.Value, nil)
		valSet[ret] = true
	}
	var rets []string
	for val := range valSet {
		rets = append(rets, val)
	}
	sort.Strings(rets)
	return rets
}

func (g *goGenerator) genClientFile(moduleRoot string) error {
	// Template for generated client initialization code.
	// TODO: consider an ergonomic way of letting caller know that static fallback will be used due to failed remote init
	const clientTemplateBody = `// Generated by Lekko. DO NOT EDIT.
package lekko

import (
	"context"
	"errors"
	"os"

	client "github.com/lekkodev/go-sdk/client"
	{{- range $.Namespaces}}
	{{nsToImport .}}{{end}}
	"google.golang.org/protobuf/proto"
)

type LekkoClient struct {
	{{- range $.Namespaces}}
	{{nsToClientFieldType .}}{{end}}
	Close client.CloseFunc
}

// Initializes the Lekko SDK client.
// For remote configs to be fetched correctly, the LEKKO_API_KEY, LEKKO_REPOSITORY_OWNER, and LEKKO_REPOSITORY_NAME env variables are required.
// If these values are missing or if there are any connection errors, the static fallbacks will be used.
func NewLekkoClient(ctx context.Context, opts ...client.ProviderOption) *LekkoClient {
	apiKey := os.Getenv("LEKKO_API_KEY")
	repoOwner := os.Getenv("LEKKO_REPOSITORY_OWNER")
	repoName := os.Getenv("LEKKO_REPOSITORY_NAME")
	opts = append(opts, client.WithAPIKey(apiKey))
	provider, err := client.CachedAPIProvider(ctx, &client.RepositoryKey{
		OwnerName: repoOwner,
		RepoName: repoName,
	}, opts...)
	if err != nil {
		provider = &noOpProvider{}
	}
	cli, close := client.NewClient(provider)
	return &LekkoClient{
		{{- range $.Namespaces}}
		{{nsToClientField .}},{{end}}
		Close: close,
	}
}

type noOpProvider struct {}

func (p *noOpProvider) GetBool(ctx context.Context, key string, namespace string) (bool, error) {
	return false, errors.New("not implemented")
}
func (p *noOpProvider) GetInt(ctx context.Context, key string, namespace string) (int64, error) {
	return 0, errors.New("not implemented")
}
func (p *noOpProvider) GetFloat(ctx context.Context, key string, namespace string) (float64, error) {
	return 0, errors.New("not implemented")
}
func (p *noOpProvider) GetString(ctx context.Context, key string, namespace string) (string, error) {
	return "", errors.New("not implemented")
}
func (p *noOpProvider) GetProto(ctx context.Context, key string, namespace string, result proto.Message) error {
	return errors.New("not implemented")
}
func (p *noOpProvider) GetJSON(ctx context.Context, key string, namespace string, result interface{}) error {
	return errors.New("not implemented")
}
func (p *noOpProvider) Close(ctx context.Context) error {
	return nil
}
`

	clientTemplateData := struct {
		Namespaces []string
	}{
		[]string{},
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
	if err := filepath.WalkDir(g.outputPath, func(path string, d fs.DirEntry, err error) error {
		// Ignore files and root and proto directory - this will need to be updated if we ever change proto gen
		if path == g.outputPath || !d.IsDir() {
			return nil
		}
		if d.Name() == "proto" {
			return filepath.SkipDir
		}
		clientTemplateData.Namespaces = append(clientTemplateData.Namespaces, d.Name())
		return filepath.SkipDir
	}); err != nil {
		return errors.Wrap(err, "generate client initialization code: walk namespaces")
	}
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
