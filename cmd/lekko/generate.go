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
	"path/filepath"
	"strings"
	"text/template"
	"time"

	"github.com/lekkodev/cli/pkg/feature"
	"github.com/lekkodev/cli/pkg/gh"
	"github.com/lekkodev/cli/pkg/repo"
	"github.com/lekkodev/cli/pkg/secrets"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"golang.org/x/exp/maps"
)

const (
	HelloWorldExample = "helloworld"
	GoAppSDKLang      = "go"
	GolangAppSDKLang  = "golang"
)

/*
const (
	VerboseFlag            = "verbose"
	VerboseFlagShort       = "v"
	VerboseFlagDescription = "verbose output"
)
*/

const (
	LanguageFlag            = "language"
	LanguageFlagShort       = "l"
	LanguageFlagDescription = "language in which the code is generated, default is go"
	LanguageFlagDefault     = ""
	OutputFileFlag          = "output-file"

	OutputFileFlagShort       = "o"
	OutputFileFlagDescription = "writes generated code to the output file, default is stdout"
	OutputFileDefault         = ""

	VerboseFlagDefault = true
)

type exampleConfig struct {
	//	name         string
	ns           string
	feature      string
	featureType  string
	featurePath  string
	lang         string
	appName      string
	codeFilepath string
}

type tmplConfig struct {
	EnvLekkoAPIKey string
	RepoName       string
	RepoOwner      string
}

// var examples []string = []string{HelloWorldExample}
var languages []string = []string{GoAppSDKLang, GolangAppSDKLang}

type featureConfig struct {
	Name     string
	Template string
}

var featuresMap map[string]featureConfig = map[string]featureConfig{
	"helloworld": {
		Name:     "helloworld",
		Template: HelloWorldFeatureTemplate,
	},
	"languages" +
		"": {
		Name:     "languages",
		Template: HelloWorldLanguagesFeatureTemplate,
	},
}
var ecHelloWord exampleConfig = exampleConfig{
	featureType: "string",
}
var examplesMap map[string]exampleConfig = map[string]exampleConfig{HelloWorldExample: ecHelloWord}

func generateCmd() *cobra.Command {
	var isDryRun, isQuiet, isForce, isVerbose bool
	var filename, language, envAPIKName, dir string
	cmd := &cobra.Command{
		Short:                 "Generates, compiles and publishes the Hello World features and generates the Hello World application code in the requested language",
		Use:                   formCmdUse("generate", "helloworld"),
		DisableFlagsInUseLine: true,
		//Use:   "generate helloworld" + FlagOptions,
		RunE: func(cmd *cobra.Command, args []string) error {
			if isVerbose && isQuiet {
				isVerbose = false
			}

			ctx := cmd.Context()
			wd, err := os.Getwd()

			if err != nil {
				return err
			}
			r, err := repo.NewLocal(wd, secrets.NewSecretsOrFail())

			if !isVerbose {
				r.ConfigureLogger(nil)
			}
			if err != nil {
				return err
			}

			if len(args) != 1 {
				return errors.New("example name is required.\n")
			}
			ex, ok := examplesMap[args[0]]
			if !ok {
				return fmt.Errorf("unknown requested %s example", args[0])
			}
			ex.appName = args[0]
			ex.ns = ex.appName

			ex.lang, _ = cmd.Flags().GetString("language")
			if len(ex.lang) > 0 {
				if !containsInSlice(languages, ex.lang) {
					return fmt.Errorf("%s currently does not support '%s' as the SDK language", AppName, ex.lang)
				}
			} else {
				ex.lang = GoAppSDKLang
			}

			ex.codeFilepath, _ = cmd.Flags().GetString("output-file")
			if len(ex.codeFilepath) > 0 {
				_, err = os.Open(ex.codeFilepath)
				if !errors.Is(err, os.ErrNotExist) && !isForce {
					return fmt.Errorf("file %s already exists, use --force to overwrite", ex.codeFilepath)
				}
			}

			// return error if the output file path is not valid and dir does not exist
			if len(filename) > 0 {
				dir = filepath.Dir(filename)
				if err, errOS := os.Stat(dir); err != nil {

				} else if os.IsNotExist(errOS) {
					return errors.New(dir + " does not exist")
				}
			}

			if !isDryRun {
				if err := r.AddNamespace(cmd.Context(), ex.ns); err != nil {
					if strings.Contains(err.Error(), "ns already exists") {
						err = errors.New(err.Error() + "( use 'lekko ns remove " + ex.ns + " -f' to remove it )")
					}
					return errors.Wrap(err, "add namespace for:"+ex.ns)
				}
			}
			if !isQuiet {
				printLinef(cmd, "\nAdded namespace '%s'\n", ex.ns)
				if isVerbose {
					printDelimiterLine()
				}
			}

			for _, f := range maps.Keys(featuresMap) {
				fConfig := featuresMap[f]
				ex.feature = fConfig.Name
				if !isQuiet {
					printLinef(cmd, "Adding feature '%s' into namespace '%s'\n", ex.feature, ex.ns)
				}
				// check if the feature already exists, if exists - fail unless -f flag is there
				if ex.featurePath, err = r.AddFeature(cmd.Context(), ex.ns, ex.feature, feature.FeatureType(ex.featureType), ""); err != nil {
					if !isForce {
						return errors.Wrap(err, "add feature")
					}
				}
				if !isDryRun {
					if err := r.WriteFile(ex.featurePath, convertFeatureTemplate(fConfig, time.Now().Unix()), 0600); err != nil {
						return fmt.Errorf("failed to add feature: %v", err)
					}
				}
				if !isQuiet {
					printLinef(cmd, "Generated content for feature '%s/%s' at path '%s'\n", ex.ns, ex.feature, ex.featurePath)
				}
				// compiling the example feature
				rootMD, _, err := r.ParseMetadata(ctx)

				if err != nil {
					return errors.Wrap(err, "parse metadata")
				}
				registry, err := r.ReBuildDynamicTypeRegistry(ctx, rootMD.ProtoDirectory, rootMD.UseExternalTypes)

				if err != nil {
					return errors.Wrap(err, "rebuild type registry")
				}

				if _, err := r.Compile(ctx, &repo.CompileRequest{
					Registry:                     registry,
					NamespaceFilter:              ex.ns,
					FeatureFilter:                ex.feature,
					DryRun:                       isDryRun,
					IgnoreBackwardsCompatibility: false,
					// don't verify file structure, since we may have not yet generated
					// the DSLs for newly added Flags().
					Verify:  false,
					Upgrade: false,
				}); err != nil {
					return errors.Wrap(err, "compile")
				}

				if !isQuiet {
					printLinef(cmd, "Feature '%s' in namespace '%s' created and compiled \n", ex.feature, ex.ns)
					if isVerbose {
						printDelimiterLine()
					}
				}
			}

			rs := secrets.NewSecretsOrFail(secrets.RequireGithub())
			ghCli := gh.NewGithubClientFromToken(ctx, rs.GetGithubToken())
			if _, err := ghCli.GetUser(ctx); err != nil {
				return errors.Wrap(err, "github auth fail")
			}
			title := fmt.Sprintf("%s-%d", ex.ns, time.Now().Unix())

			if _, err = r.Review(ctx, title, ghCli, rs); err != nil {
				return errors.Wrap(err, "review")
			}

			u, err := r.GetRemoteURL()
			if err != nil {
				return errors.Wrap(err, "get remote url")
			}
			owner, repoName, err := gh.ParseOwnerRepo(u)
			if err != nil {
				return errors.Wrap(err, "parse owner repo")
			}

			if !isQuiet {
				printLinef(cmd, "Review request issued for repo %s/%s with title '%s' \n", rs.GetGithubUser(), repoName, title)
				if isVerbose {
					printDelimiterLine()
				}
			}

			var prNum *int
			var prNumRet int
			if !isDryRun {
				if prNumRet, err = r.Merge(ctx, prNum, ghCli, rs); err != nil {
					return errors.Wrap(err, "merge")
				}
			}

			if !isQuiet {
				printLinef(cmd, "Pull request #%d merge completed\n", prNumRet)
				if isVerbose {
					printDelimiterLine()
				}
			}

			if len(rs.GetLekkoTeam()) == 0 {
				return errors.New("team name must not be empty")
			}

			//-- generate code from template
			tmplConfig := tmplConfig{EnvLekkoAPIKey: envAPIKName, RepoName: repoName, RepoOwner: owner}
			code, err := convertHelloWorldCodeTemplate(HelloWorldGoCodeTemplate, tmplConfig)
			if err != nil {
				return err
			}

			if len(ex.codeFilepath) > 0 {
				if err = os.WriteFile(ex.codeFilepath, []byte(code), 0600); err != nil {
					return errors.Wrap(err, "writing generated code to file:"+ex.codeFilepath)
				}
				if !isQuiet {
					printLinef(cmd, "The %s 'helloword' program generated and saved to %s\n", ex.lang, ex.codeFilepath)
				} else {
					printLinef(cmd, "%s", ex.codeFilepath)
				}

				if !isQuiet && isVerbose {
					printDelimiterLine()

					printLinef(cmd, "Once the helloworld go env is initialized, run \"LEKKO_API_KEY=***** GITHUB_OWNER=%s GITHUB_REPO=%s go run .\" in %s, or equivalent command.\n", tmplConfig.RepoOwner, tmplConfig.RepoName, dir)
				}
			} else {
				if !isQuiet {
					printLinef(cmd, "Generating %s 'helloword' program ...\n", ex.lang)
				}
				fmt.Printf("%s", code)
				if !isQuiet && isVerbose {
					printDelimiterLine()

					printLinef(cmd, "Compile and run the generated program.\nUse \"LEKKO_API_KEY=***** GITHUB_OWNER=%s GITHUB_REPO=%s helloworld\" or equivalent to run it.\n", tmplConfig.RepoOwner, tmplConfig.RepoName)
				}
			}

			return nil
		},
	}
	cmd.Flags().StringVarP(&language, LanguageFlag, LanguageFlagShort, LanguageFlagDefault, LanguageFlagDescription)
	cmd.Flags().StringVarP(&filename, OutputFileFlag, OutputFileFlagShort, OutputFileDefault, OutputFileFlagDescription)
	cmd.Flags().BoolVarP(&isQuiet, QuietModeFlag, QuietModeFlagShort, QuietModeFlagDVal, QuietModeFlagDescription)
	cmd.Flags().BoolVarP(&isDryRun, DryRunFlag, DryRunFlagShort, DryRunFlagDVal, DryRunFlagDescription)
	cmd.Flags().BoolVarP(&isForce, ForceFlag, ForceFlagShort, ForceFlagDVal, ForceFlagDescription)
	cmd.Flags().BoolVarP(&isVerbose, VerboseFlag, VerboseFlagShort, VerboseFlagDefault, VerboseFlagDescription)
	return cmd
}

// -- Features templates template conversion
func convertFeatureTemplate(fc featureConfig, args ...interface{}) []byte {
	return []byte(fmt.Sprintf(fc.Template, args...))
}

const HelloWorldFeatureTemplate = `
result = feature(
    description = "helloworld feature, gen-id:%d",
    default = "Hello World!",
    rules = [
		("language == \"English\"", "Hello World"),	
		("language == \"Polish\"", "Witaj Swiecie"),
		("language == \"Italian\"", "Ciao mondo"),
        ("language == \"French\"", "Bonjour le monde"),
        ("language == \"German\"", "Hallo Welt"),
        ("language == \"Spanish\"", "Hola Mundo"),	
    ],
)
`
const HelloWorldLanguagesFeatureTemplate = `
result = feature(
    description = "array of languages in which hello world greetings can be displayed, , gen-id:%d", 
	default = ["English", "Spanish", "Italian", "French", "German", "Polish"], 
)
`

// -- go code template and  template conversion
func convertHelloWorldCodeTemplate(tmpl string, config tmplConfig) (code string, err error) {
	codeTmpl := template.New("goCodeTmpl")
	codeTmpl, _ = codeTmpl.Parse(tmpl)
	var bfd bytes.Buffer
	err = codeTmpl.Execute(&bfd, config)
	code = bfd.String()
	return
}

const HelloWorldGoCodeTemplate = `package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/lekkodev/go-sdk/client"
)

const (
	CtxTimeoutSec              = 200
	PrintTimeoutSec            = 2
	EnvLekkoAPIKey             = "LEKKO_API_KEY"
	RepoName                   = "GITHUB_REPO"
	RepoOwner                  = "GITHUB_OWNER"
	NameSpace                  = "helloworld"
	HelloWorldLanguagesFeature = "languages"
	HelloWorldFeature          = "helloworld"
	HelloWorldFeatureCtx       = "language"
)

func main() {
	var err error
	var provider client.Provider

	key := os.Getenv(EnvLekkoAPIKey)
	owner := os.Getenv(RepoOwner)
	repo := os.Getenv(RepoName)

	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT)
	stdin := make(chan string)

	go func() {
		var s string
		fmt.Scanln(&s)
		stdin <- s
	}()

	ctx, _ := context.WithTimeout(context.Background(), CtxTimeoutSec*time.Second)
	provider, err = client.ConnectAPIProvider(ctx, key, &client.RepositoryKey{
		OwnerName: owner,
		RepoName:  repo,
	})
	if err != nil {
		log.Fatal("Failed to start provider:" + err.Error())
	}
	cl, closeF := client.NewClient(NameSpace, provider)
	var langs []string

	exitFunc := func() {
		if err = closeF(ctx); err != nil {
			log.Fatal(err)
		}
		os.Exit(0)
	}

	fmt.Println("Hello World in different languages ... hit return to exit")

	for {
		err = cl.GetJSON(ctx, HelloWorldLanguagesFeature, &langs)
		if err != nil {
			log.Fatalf("error retrieving feature %s:%s/n", HelloWorldLanguagesFeature, err.Error())
		}

		for _, lang := range langs {
			select {
			case <-sigs:
				exitFunc()
			case <-stdin:
				exitFunc()
			default:
				ctx = client.Add(ctx, HelloWorldFeatureCtx, lang)
				hwMsg, err := cl.GetString(ctx, HelloWorldFeature)
				if err != nil {
					log.Fatalf("Failed to fetch boolean feature: %s:%s", HelloWorldFeature, err.Error())
				}
				fmt.Printf("                                                          \r")
				fmt.Printf("Hello World in %s: %s\r", lang, hwMsg)
				time.Sleep(PrintTimeoutSec * time.Second)
			}
		}

	}
}
`
