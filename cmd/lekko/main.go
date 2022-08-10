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
	"log"
	"os"

	"github.com/lekkodev/cli/pkg/verify"

	"github.com/spf13/cobra"
)

func main() {
	rootCmd.AddCommand(verifyCmd)
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
