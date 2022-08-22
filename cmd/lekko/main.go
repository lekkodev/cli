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
	"encoding/json"
	"fmt"
	"log"
	"os"

	"github.com/lekkodev/cli/pkg/eval"
	"github.com/lekkodev/cli/pkg/feature"
	"github.com/lekkodev/cli/pkg/generate"
	"github.com/lekkodev/cli/pkg/verify"
	"github.com/pkg/errors"

	"github.com/spf13/cobra"
	"google.golang.org/protobuf/types/known/wrapperspb"
)

func main() {
	rootCmd.AddCommand(verifyCmd)
	rootCmd.AddCommand(compileCmd)
	rootCmd.AddCommand(evalCmd)
	rootCmd.AddCommand(addCmd())
	if err := rootCmd.Execute(); err != nil {
		log.Println(err)
		os.Exit(1)
	}
}

var rootCmd = &cobra.Command{
	Use:           "lekko",
	Short:         "lekko - dynamic configuration helper",
	SilenceUsage:  true,
	SilenceErrors: true,
}

var verifyCmd = &cobra.Command{
	Use:   "verify",
	Short: "verify a config repository with a lekko.root.yaml",
	RunE: func(cmd *cobra.Command, args []string) error {
		// TODO lint the repo with the right proto files.
		wd, err := os.Getwd()
		if err != nil {
			return err
		}
		return verify.Verify(wd)
	},
}

var compileCmd = &cobra.Command{
	Use:   "compile",
	Short: "compiles features based on individual definitions",
	RunE: func(cmd *cobra.Command, args []string) error {
		wd, err := os.Getwd()
		if err != nil {
			return err
		}
		return generate.Compile(wd)
	},
}

var evalCmd = &cobra.Command{
	Use:   "eval namespace/feature '{\"context_key\": 123}'",
	Short: "Evaluates a specified feature based on the provided context",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		wd, err := os.Getwd()
		if err != nil {
			return err
		}

		ctxMap := make(map[string]interface{})
		if err := json.Unmarshal([]byte(args[1]), &ctxMap); err != nil {
			return err
		}
		// TODO do protojson encoding by building the type registry.
		res, err := eval.Eval(wd, args[0], ctxMap)
		if err != nil {
			return err
		}

		fmt.Printf("Correctly evaluated to an any of type: %v\n", res.TypeUrl)
		boolVal := new(wrapperspb.BoolValue)
		if res.MessageIs(boolVal) {
			if err := res.UnmarshalTo(boolVal); err != nil {
				return err
			}
			fmt.Printf("Resulting value: %t\n", boolVal.Value)
		} else {
			fmt.Println("Resulting value is user defined, custom unmarshalling coming soon!")
		}
		return nil
	},
}

func addCmd() *cobra.Command {
	var complexFeature bool
	ret := &cobra.Command{
		Use:   "add namespace/feature",
		Short: "Adds a new feature flag",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			wd, err := os.Getwd()
			if err != nil {
				return err
			}
			if err := cmd.ParseFlags(args); err != nil {
				return errors.Wrap(err, "parse flags")
			}
			namespace, featureName, err := feature.ParseFeaturePath(args[0])
			if err != nil {
				return errors.Wrap(err, "parse feature path")
			}
			return generate.Add(wd, namespace, featureName, complexFeature)
		},
	}
	ret.Flags().BoolVarP(&complexFeature, "complex", "c", false, "create a complex configuration with proto, rules and validation")
	return ret
}
