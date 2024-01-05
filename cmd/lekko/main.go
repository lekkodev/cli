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
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"text/template"

	bffv1beta1 "buf.build/gen/go/lekkodev/cli/protocolbuffers/go/lekko/bff/v1beta1"
	featurev1beta1 "buf.build/gen/go/lekkodev/cli/protocolbuffers/go/lekko/feature/v1beta1"
	"github.com/AlecAivazis/survey/v2"
	"github.com/bufbuild/connect-go"
	"github.com/lekkodev/cli/pkg/feature"
	"github.com/lekkodev/cli/pkg/gh"
	"github.com/lekkodev/cli/pkg/k8s"
	"github.com/lekkodev/cli/pkg/lekko"
	"github.com/lekkodev/cli/pkg/logging"
	"github.com/lekkodev/cli/pkg/repo"
	"github.com/lekkodev/cli/pkg/secrets"
	"github.com/lekkodev/cli/pkg/star/static"
	"github.com/mitchellh/go-homedir"
	"github.com/pkg/errors"
	"golang.org/x/mod/modfile"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"

	"github.com/spf13/cobra"
)

// Updated at build time using ldflags
var version = "development"

func main() {
	rootCmd := rootCmd()
	rootCmd.AddCommand(compileCmd())
	rootCmd.AddCommand(verifyCmd())
	rootCmd.AddCommand(commitCmd())
	rootCmd.AddCommand(reviewCmd())
	rootCmd.AddCommand(mergeCmd)
	rootCmd.AddCommand(restoreCmd())
	rootCmd.AddCommand(teamCmd())
	rootCmd.AddCommand(repoCmd())
	rootCmd.AddCommand(featureCmd())
	rootCmd.AddCommand(namespaceCmd())
	rootCmd.AddCommand(apikeyCmd())
	rootCmd.AddCommand(upgradeCmd())
	// auth
	rootCmd.AddCommand(authCmd())
	// exp
	k8sCmd.AddCommand(applyCmd())
	k8sCmd.AddCommand(listCmd())
	experimentalCmd.AddCommand(k8sCmd)
	experimentalCmd.AddCommand(parseCmd())
	experimentalCmd.AddCommand(cleanupCmd)
	experimentalCmd.AddCommand(formatCmd())
	// code generation
	genCmd.AddCommand(genGoCmd())
	experimentalCmd.AddCommand(genCmd)
	rootCmd.AddCommand(experimentalCmd)

	logging.InitColors()
	if err := rootCmd.ExecuteContext(context.Background()); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

func rootCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:           "lekko",
		Short:         "lekko - dynamic configuration helper",
		Version:       version,
		SilenceUsage:  true,
		SilenceErrors: true,
	}
	cmd.PersistentFlags().StringVar(&lekko.URL, "backend-url", "https://prod.api.lekko.dev", "Lekko backend url")
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
		typeParts := strings.Split(f.Tree.Default.TypeUrl, ".")
		retType = strings.Join(typeParts[len(typeParts)-2:], ".")
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
			for _, ff := range ffs {
				fff, err := os.ReadFile(wd + "/" + ns + "/" + ff.CompiledProtoBinFileName)
				if err != nil {
					return err
				}
				f := &featurev1beta1.Feature{}
				err = proto.Unmarshal(fff, f)
				if err != nil {
					return err
				}
				codeString, err := genGoForFeature(f, ns)
				if err != nil {
					return err
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
	v1beta1 "{{$.ModuleRoot}}/internal/lekko/{{$.Namespace}}/proto"
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
			err = os.MkdirAll("./internal/lekko/"+ns+"/proto/", 0770)
			if err != nil {
				return err
			}
			pCmd := exec.Command(
				"protoc",
				"--go_opt=M"+ns+"/config/v1beta1/example.proto=.", // #nosec G204
				"--proto_path="+wd+"/proto",                       // #nosec G204
				"--go_out=proto",
				ns+"/config/v1beta1/example.proto") // #nosec G204
			pCmd.Dir = "./internal/lekko/" + ns
			err = pCmd.Run()
			if err != nil {
				return err
			}
			f, err := os.Create("./internal/lekko/" + ns + "/" + of)
			if err != nil {
				return err
			}
			data := struct {
				ModuleRoot         string
				Namespace          string
				ProtoAsByteStrings []string
				CodeStrings        []string
			}{
				moduleRoot,
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

func formatCmd() *cobra.Command {
	var verbose bool
	cmd := &cobra.Command{
		Use:   "format",
		Short: "format star files",
		RunE: func(cmd *cobra.Command, args []string) error {
			wd, err := os.Getwd()
			if err != nil {
				return err
			}
			rs := secrets.NewSecretsOrFail()
			r, err := repo.NewLocal(wd, rs)
			if err != nil {
				return errors.Wrap(err, "new repo")
			}
			return r.Format(cmd.Context(), verbose)
		},
	}
	cmd.Flags().BoolVarP(&verbose, "verbose", "v", false, "enable verbose output")
	return cmd
}

func compileCmd() *cobra.Command {
	var force, dryRun, upgrade, verbose bool
	cmd := &cobra.Command{
		Use:   "compile [namespace[/config]]",
		Short: "compiles configs based on individual definitions",
		RunE: func(cmd *cobra.Command, args []string) error {
			wd, err := os.Getwd()
			if err != nil {
				return err
			}
			rs := secrets.NewSecretsOrFail()
			r, err := repo.NewLocal(wd, rs)
			if err != nil {
				return err
			}
			ctx := cmd.Context()
			rootMD, _, err := r.ParseMetadata(ctx)
			if err != nil {
				return errors.Wrap(err, "parse metadata")
			}
			registry, err := r.ReBuildDynamicTypeRegistry(ctx, rootMD.ProtoDirectory, rootMD.UseExternalTypes)
			if err != nil {
				return errors.Wrap(err, "rebuild type registry")
			}
			var ns, f string
			if len(args) > 0 {
				ns, f, err = feature.ParseFeaturePath(args[0])
				if err != nil {
					return err
				}
			}
			if _, err := r.Compile(ctx, &repo.CompileRequest{
				Registry:                     registry,
				NamespaceFilter:              ns,
				FeatureFilter:                f,
				DryRun:                       dryRun,
				IgnoreBackwardsCompatibility: force,
				// don't verify file structure, since we may have not yet generated
				// the DSLs for newly added features.
				Verify:  false,
				Upgrade: upgrade,
				Verbose: verbose,
			}); err != nil {
				return errors.Wrap(err, "compile")
			}
			return nil
		},
	}
	cmd.Flags().BoolVarP(&force, "force", "f", false, "force compilation, ignoring validation check failures.")
	cmd.Flags().BoolVarP(&dryRun, "dry-run", "d", false, "skip persisting any newly compiled changes to disk.")
	cmd.Flags().BoolVarP(&upgrade, "upgrade", "u", false, "upgrade any of the requested namespaces that are behind the latest version.")
	cmd.Flags().BoolVarP(&verbose, "verbose", "v", false, "enable verbose error logging.")
	return cmd
}

func verifyCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "verify [namespace[/config]]",
		Short: "verifies configs based on individual definitions",
		RunE: func(cmd *cobra.Command, args []string) error {
			wd, err := os.Getwd()
			if err != nil {
				return err
			}
			rs := secrets.NewSecretsOrFail()
			r, err := repo.NewLocal(wd, rs)
			if err != nil {
				return err
			}
			ctx := cmd.Context()
			rootMD, _, err := r.ParseMetadata(ctx)
			if err != nil {
				return errors.Wrap(err, "parse metadata")
			}
			registry, err := r.ReBuildDynamicTypeRegistry(ctx, rootMD.ProtoDirectory, rootMD.UseExternalTypes)
			if err != nil {
				return errors.Wrap(err, "rebuild type registry")
			}
			var ns, f string
			if len(args) > 0 {
				ns, f, err = feature.ParseFeaturePath(args[0])
				if err != nil {
					return err
				}
			}
			return r.Verify(ctx, &repo.VerifyRequest{
				Registry:        registry,
				NamespaceFilter: ns,
				FeatureFilter:   f,
			})
		},
	}
	return cmd
}

func parseCmd() *cobra.Command {
	var ns, featureName string
	var all, printFeature bool
	cmd := &cobra.Command{
		Use:   "parse",
		Short: "parse a feature file using static analysis, and rewrite the starlark",
		RunE: func(cmd *cobra.Command, args []string) error {
			wd, err := os.Getwd()
			if err != nil {
				return err
			}
			r, err := repo.NewLocal(wd, secrets.NewSecretsOrFail())
			if err != nil {
				return errors.Wrap(err, "new repo")
			}
			ctx := cmd.Context()
			rootMD, _, err := r.ParseMetadata(ctx)
			if err != nil {
				return errors.Wrap(err, "parse metadata")
			}
			registry, err := r.BuildDynamicTypeRegistry(ctx, rootMD.ProtoDirectory)
			if err != nil {
				return errors.Wrap(err, "build dynamic type registry")
			}
			var nsfs namespaceFeatures
			if all {
				nsfs, err = getNamespaceFeatures(ctx, r, ns, featureName)
				if err != nil {
					return err
				}
			} else {
				nsf, err := featureSelect(ctx, r, ns, featureName)
				if err != nil {
					return err
				}
				nsfs = append(nsfs, nsf)
			}
			for _, nsf := range nsfs {
				f, err := r.Parse(ctx, nsf.namespace(), nsf.feature(), registry)
				fmt.Print(logging.Bold(fmt.Sprintf("[%s]", nsf.String())))
				if errors.Is(err, static.ErrUnsupportedStaticParsing) {
					fmt.Printf(" Unsupported static parsing: %v\n", err.Error())
				} else if err != nil {
					fmt.Printf(" %v\n", err)
				} else {
					fmt.Printf("[%s] Parsed\n", f.Type)
					if printFeature {
						fmt.Println(protojson.MarshalOptions{
							Resolver:  registry,
							Multiline: true,
						}.Format(f))
					}
				}
			}
			return nil
		},
	}
	cmd.Flags().StringVarP(&ns, "namespace", "n", "", "namespace to remove config from")
	cmd.Flags().StringVarP(&featureName, "feature", "f", "", "name of config to remove")
	_ = cmd.Flags().MarkHidden("feature")
	cmd.Flags().StringVarP(&featureName, "config", "c", "", "name of config to remove")
	cmd.Flags().BoolVarP(&all, "all", "a", false, "parse all configs")
	cmd.Flags().BoolVarP(&printFeature, "print", "p", false, "print parsed config(s)")
	return cmd
}

func reviewCmd() *cobra.Command {
	var title string
	cmd := &cobra.Command{
		Use:   "review",
		Short: "creates a pr with your changes",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			wd, err := os.Getwd()
			if err != nil {
				return err
			}
			rs := secrets.NewSecretsOrFail(secrets.RequireGithub())
			r, err := repo.NewLocal(wd, rs)
			if err != nil {
				return errors.Wrap(err, "new repo")
			}
			if err := r.Verify(ctx, &repo.VerifyRequest{}); err != nil {
				return errors.Wrap(err, "verify")
			}

			ghCli := gh.NewGithubClientFromToken(ctx, rs.GetGithubToken())
			if _, err := ghCli.GetUser(ctx); err != nil {
				return errors.Wrap(err, "github auth fail")
			}

			if len(title) == 0 {
				fmt.Printf("-------------------\n")
				if err := survey.AskOne(&survey.Input{
					Message: "Title:",
				}, &title); err != nil {
					return errors.Wrap(err, "prompt")
				}
			}

			_, err = r.Review(ctx, title, ghCli, rs)
			return err
		},
	}
	cmd.Flags().StringVarP(&title, "title", "t", "", "Title of pull request")
	return cmd
}

var mergeCmd = &cobra.Command{
	Use:   "merge [pr-number]",
	Short: "merges a pr for the current branch",
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		wd, err := os.Getwd()
		if err != nil {
			return err
		}
		rs := secrets.NewSecretsOrFail(secrets.RequireGithub())
		r, err := repo.NewLocal(wd, rs)
		if err != nil {
			return errors.Wrap(err, "new repo")
		}
		ctx := cmd.Context()
		if err := r.Verify(ctx, &repo.VerifyRequest{}); err != nil {
			return errors.Wrap(err, "verify")
		}
		var prNum *int
		if len(args) > 0 {
			num, err := strconv.Atoi(args[0])
			if err != nil {
				return errors.Wrap(err, "pr-number arg")
			}
			prNum = &num
		}

		ghCli := gh.NewGithubClientFromToken(ctx, rs.GetGithubToken())
		if _, err := ghCli.GetUser(ctx); err != nil {
			return errors.Wrap(err, "github auth fail")
		}
		if err := r.Merge(ctx, prNum, ghCli, rs); err != nil {
			return errors.Wrap(err, "merge")
		}
		fmt.Printf("PR merged.\n")
		if len(rs.GetLekkoTeam()) > 0 {
			u, err := r.GetRemoteURL()
			if err != nil {
				return errors.Wrap(err, "get remote url")
			}
			owner, repo, err := gh.ParseOwnerRepo(u)
			if err != nil {
				return errors.Wrap(err, "parse owner repo")
			}
			repos, err := lekko.NewBFFClient(rs).ListRepositories(ctx, connect.NewRequest(&bffv1beta1.ListRepositoriesRequest{}))
			if err != nil {
				return errors.Wrap(err, "repository fetch failed")
			}
			defaultBranch := ""
			for _, r := range repos.Msg.GetRepositories() {
				if r.OwnerName == owner && r.RepoName == repo {
					defaultBranch = r.BranchName
				}
			}
			if len(defaultBranch) == 0 {
				return errors.New("repository not found when rolling out")
			}
			fmt.Printf("Visit %s to monitor your rollout.\n", rolloutsURL(rs.GetLekkoTeam(), owner, repo, defaultBranch))
		}
		return nil
	},
}

func rolloutsURL(team, owner, repo, branch string) string {
	return fmt.Sprintf("https://app.lekko.com/teams/%s/repositories/%s/%s/branches/%s/commits", team, owner, repo, branch)
}

type provider string

const (
	providerLekko  provider = "lekko"
	providerGithub provider = "github"
)

func (p *provider) String() string {
	return string(*p)
}

func (p *provider) Set(v string) error {
	switch v {
	case string(providerLekko), string(providerGithub):
		*p = provider(v)
	default:
		return errors.New(`must be one of "lekko" or "github"`)
	}
	return nil
}

func (p *provider) Type() string {
	return "provider"
}

var k8sCmd = &cobra.Command{
	Use:   "k8s",
	Short: "manage lekko configurations in kubernetes. Uses the current k8s context set in your kubeconfig file.",
}

func localKubeParams(cmd *cobra.Command, kubeConfig *string) {
	var defaultKubeconfig string
	// ref: https://github.com/kubernetes/client-go/blob/master/examples/out-of-cluster-client-configuration/main.go
	home, err := homedir.Dir()
	if err == nil {
		defaultKubeconfig = filepath.Join(home, ".kube", "config")
	}
	cmd.Flags().StringVarP(kubeConfig, "kubeconfig", "c", defaultKubeconfig, "absolute path to the kube config file")
}

func applyCmd() *cobra.Command {
	var kubeConfig string
	ret := &cobra.Command{
		Use:   "apply",
		Short: "apply local configurations to kubernetes configmaps",
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := cmd.ParseFlags(args); err != nil {
				return errors.Wrap(err, "failed to parse flags")
			}

			wd, err := os.Getwd()
			if err != nil {
				return err
			}
			rs := secrets.NewSecretsOrFail(secrets.RequireGithub())
			r, err := repo.NewLocal(wd, rs)
			if err != nil {
				return errors.Wrap(err, "new repo")
			}
			ctx := cmd.Context()
			if err := r.Verify(ctx, &repo.VerifyRequest{}); err != nil {
				return errors.Wrap(err, "verify")
			}
			kube, err := k8s.NewKubernetes(kubeConfig, r)
			if err != nil {
				return errors.Wrap(err, "failed to build k8s client")
			}
			if err := kube.Apply(ctx, rs.GetUsername()); err != nil {
				return errors.Wrap(err, "apply")
			}

			return nil
		},
	}
	localKubeParams(ret, &kubeConfig)
	return ret
}

func listCmd() *cobra.Command {
	var kubeConfig string
	ret := &cobra.Command{
		Use:   "list",
		Short: "list lekko configurations currently in kubernetes",
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := cmd.ParseFlags(args); err != nil {
				return errors.Wrap(err, "failed to parse flags")
			}

			kube, err := k8s.NewKubernetes(kubeConfig, nil)
			if err != nil {
				return errors.Wrap(err, "failed to build k8s client")
			}
			if err := kube.List(cmd.Context()); err != nil {
				return errors.Wrap(err, "list")
			}
			return nil
		},
	}
	localKubeParams(ret, &kubeConfig)
	return ret
}

var experimentalCmd = &cobra.Command{
	Use:   "exp",
	Short: "experimental commands",
}

func commitCmd() *cobra.Command {
	var message string
	cmd := &cobra.Command{
		Use:   "commit",
		Short: "commits local changes to the remote branch",
		RunE: func(cmd *cobra.Command, args []string) error {
			wd, err := os.Getwd()
			if err != nil {
				return err
			}
			rs := secrets.NewSecretsOrFail(secrets.RequireGithub())
			r, err := repo.NewLocal(wd, rs)
			if err != nil {
				return errors.Wrap(err, "new repo")
			}
			ctx := cmd.Context()
			if err := r.Verify(ctx, &repo.VerifyRequest{}); err != nil {
				return errors.Wrap(err, "verify")
			}
			signature, err := repo.GetCommitSignature(ctx, rs, rs.GetLekkoUsername())
			if err != nil {
				return err
			}
			if _, err = r.Commit(ctx, rs, message, signature); err != nil {
				return err
			}
			return nil
		},
	}
	cmd.Flags().StringVarP(&message, "message", "m", "config change commit", "commit message")
	return cmd
}

var cleanupCmd = &cobra.Command{
	Use:   "cleanup [branchname]",
	Short: "deletes the current local branch or the branch specified (and its remote counterpart)",
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		wd, err := os.Getwd()
		if err != nil {
			return err
		}
		rs := secrets.NewSecretsOrFail(secrets.RequireGithub())
		r, err := repo.NewLocal(wd, rs)
		if err != nil {
			return errors.Wrap(err, "new repo")
		}
		var optionalBranchName *string
		if len(args) > 0 {
			optionalBranchName = &args[0]
		}
		if err = r.Cleanup(cmd.Context(), optionalBranchName, rs); err != nil {
			return err
		}
		return nil
	},
}

func restoreCmd() *cobra.Command {
	var force bool
	cmd := &cobra.Command{
		Use:   "restore [hash]",
		Short: "restores repo to a particular hash",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			wd, err := os.Getwd()
			if err != nil {
				return err
			}
			rs := secrets.NewSecretsOrFail(secrets.RequireGithub())
			r, err := repo.NewLocal(wd, rs)
			if err != nil {
				return errors.Wrap(err, "new repo")
			}
			if err := r.RestoreWorkingDirectory(args[0]); err != nil {
				return errors.Wrap(err, "restore wd")
			}
			ctx := cmd.Context()
			rootMD, _, err := r.ParseMetadata(ctx)
			if err != nil {
				return errors.Wrap(err, "parse metadata")
			}
			registry, err := r.ReBuildDynamicTypeRegistry(ctx, rootMD.ProtoDirectory, rootMD.UseExternalTypes)
			if err != nil {
				return errors.Wrap(err, "rebuild type registry")
			}
			fmt.Printf("Successfully rebuilt dynamic type registry.\n")
			if _, err := r.Compile(ctx, &repo.CompileRequest{
				Registry:                     registry,
				DryRun:                       false,
				IgnoreBackwardsCompatibility: force,
			}); err != nil {
				return errors.Wrap(err, "compile")
			}
			fmt.Printf("Restored hash %s to your working directory. \nRun `lekko review` to create a PR with these changes.\n", args[0])
			return nil
		},
	}
	cmd.Flags().BoolVarP(&force, "force", "f", false, "force compilation, ignoring validation check failures.")
	return cmd
}

func upgradeCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "upgrade",
		Short: "upgrade lekko to the latest version using homebrew",
		RunE: func(cmd *cobra.Command, args []string) error {
			fmt.Printf(
				`Our CLI is currently managed by Homebrew.
In order to upgrade, run the following commands:

	brew update
	brew upgrade lekko

For more information, check out our docs:
https://app.lekko.com/docs/cli/
`)
			return nil
		},
	}
	return cmd
}
