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
	"io"
	"os"
	"os/exec"
	"strings"

	"github.com/pkg/errors"
	"github.com/spf13/cobra"
)

func upgradeCmd() *cobra.Command {
	var apikey string
	type execReq struct {
		stdout, stderr io.Writer
		env            []string
		verbose        bool
	}
	execCmd := func(ctx context.Context, req *execReq, name string, args ...string) ([]byte, error) {
		cmd := exec.CommandContext(ctx, name, args...)
		stdout, stderr := bytes.NewBuffer(nil), bytes.NewBuffer(nil)
		if req.stdout != nil {
			cmd.Stdout = req.stdout
		} else {
			cmd.Stdout = stdout
		}
		if req.stderr != nil {
			cmd.Stderr = req.stderr
		} else {
			cmd.Stderr = stderr
		}
		cmd.Env = append(cmd.Env, req.env...)
		if req.verbose {
			fmt.Printf("Running '%s %s'...\n", name, strings.Join(args, " "))
		}
		err := cmd.Run()
		if err != nil {
			return nil, errors.Wrapf(err, "%s %s", name, strings.Join(args, " "))
		}
		return stdout.Bytes(), nil
	}
	checkToolExists := func(ctx context.Context, name string) error {
		if _, err := execCmd(ctx, &execReq{}, name, "--version"); err != nil {
			return errors.Wrapf(err, "command not found: '%s'", name)
		}
		return nil
	}
	cmd := &cobra.Command{

		Short:                 "Upgrade lekko to the latest version using homebrew",
		Use:                   formCmdUse("upgrade", "apikey"),
		DisableFlagsInUseLine: true,
		Args:                  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(apikey) == 0 {
				return errors.New("no api key provided")
			}
			ctx := cmd.Context()
			for _, tool := range []string{"brew", "curl", "jq"} {
				if err := checkToolExists(ctx, tool); err != nil {
					return err
				}
			}
			if _, err := execCmd(ctx, &execReq{
				stdout:  os.Stdout,
				stderr:  os.Stderr,
				verbose: true,
			}, "brew", "update"); err != nil {
				return err
			}
			if _, err := execCmd(ctx, &execReq{
				stdout:  os.Stdout,
				stderr:  os.Stderr,
				verbose: true,
			}, "brew", "tap", "lekkodev/lekko"); err != nil {
				return err
			}
			brewRepoOutput, err := execCmd(ctx, &execReq{}, "brew", "--repo")
			if err != nil {
				return err
			}
			brewRepo := strings.TrimSpace(string(brewRepoOutput))
			tokenScript := fmt.Sprintf("%s/Library/Taps/lekkodev/homebrew-lekko/gen_token.sh", brewRepo)
			tokenOutput, err := execCmd(ctx, &execReq{}, tokenScript)
			if err != nil {
				return err
			}
			token := strings.TrimSpace(string(tokenOutput))
			if len(token) == 0 {
				return errors.New("failed to generate token")
			}
			envToSet := "HOMEBREW_GITHUB_API_TOKEN"
			_, err = execCmd(ctx, &execReq{
				stdout:  os.Stdout,
				stderr:  os.Stderr,
				verbose: true,
				env:     []string{fmt.Sprintf("%s=%s", envToSet, token)},
			}, "brew", "upgrade", "lekko")
			return err
		},
	}
	cmd.Flags().StringVarP(&apikey, "apikey", "a", os.Getenv("LEKKO_APIKEY"), "apikey used to upgrade")
	return cmd
}
