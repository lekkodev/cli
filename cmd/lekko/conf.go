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
	"os"

	"github.com/lekkodev/cli/pkg/dotlekko"
	"github.com/spf13/cobra"
)

func confCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:    "conf",
		Short:  "parse a .lekko-like configuration file in the working directory",
		Hidden: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			// By default, read dotlekko and output as JSON
			wd, err := os.Getwd()
			if err != nil {
				return err
			}
			dot, err := dotlekko.ReadDotLekko(wd)
			if err != nil {
				return err
			}
			b, err := json.MarshalIndent(dot, "", "    ")
			if err != nil {
				return err
			}
			fmt.Println(string(b))

			return nil
		},
	}
	return cmd
}
