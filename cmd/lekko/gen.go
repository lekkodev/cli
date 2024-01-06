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
	"golang.org/x/exp/maps"
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
			fmt.Println("hi")
			b, err := os.ReadFile("go.mod")
			if err != nil {
				return err
			}
			fmt.Println("hi2")
			mf, err := modfile.ParseLax("go.mod", b, nil)
			if err != nil {
				return err
			}
			moduleRoot := mf.Module.Mod.Path

			rs := secrets.NewSecretsOrFail()
			fmt.Println("hi3")
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
			fmt.Println("hi4")
			var protoAsByteStrings []string
			var codeStrings []string
			protoImportSet := make(map[string]struct{})
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
					protoImportSet[genImport(moduleRoot, f.Tree.Default.TypeUrl)] = struct{}{}
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
			const templateBody = `package lekko

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
			// old one:
			// --template={"managed": {"enabled": true, "go_package_prefix": {"default": "."}}, "version":"v1","plugins":[{"plugin":"go","out":"internal/lekko/proto", "opt": "paths=source_relative"}]}
			pCmd := exec.Command(
				"buf",
				"generate",
				fmt.Sprintf(`--template={"managed": {"enabled": true, "go_package_prefix": {"default": "%s/internal/lekko/proto"}}, "version":"v1","plugins":[{"plugin":"go","out":"internal/lekko/proto", "opt": "paths=source_relative"}]}`, moduleRoot),
				"--include-imports",
				"https://github.com/lekkodev/internal.git") // #nosec G204
			pCmd.Dir = "."
			fmt.Println(pCmd.String())
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
			data := struct {
				ProtoImports       []string
				Namespace          string
				ProtoAsByteStrings []string
				CodeStrings        []string
			}{
				maps.Keys(protoImportSet),
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
		typeParts := strings.Split(strings.Split(f.Tree.Default.TypeUrl, "/")[1], ".")
		// creates configv1beta1.DBConfig
		retType = fmt.Sprintf("%s%s.%s", typeParts[len(typeParts)-3], typeParts[len(typeParts)-2], typeParts[len(typeParts)-1])
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

func genImport(moduleRoot string, typeURL string) string {
	anyURLSplit := strings.Split(typeURL, "/")
	if anyURLSplit[0] != "type.googleapis.com" {
		panic("invalid any type url: " + typeURL)
	}
	// turn default.config.v1beta1.DBConfig into:
	// moduleRoot/internal/lekko/proto/default/config/v1beta1
	typeParts := strings.Split(anyURLSplit[1], ".")

	importPath := strings.Join(append([]string{moduleRoot + "/internal/lekko/proto"}, typeParts[:len(typeParts)-1]...), "/")

	// TODO do google.protobuf.X

	// returns configv1beta1
	return fmt.Sprintf(`%s%s "%s"`, typeParts[len(typeParts)-3], typeParts[len(typeParts)-2], importPath)
}
