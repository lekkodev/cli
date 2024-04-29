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
	"os"

	"github.com/lekkodev/cli/pkg/repo"
	"github.com/lekkodev/cli/pkg/sync"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"golang.org/x/mod/modfile"
)

func syncCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "sync",
		Short: "sync code to config",
	}
	cmd.AddCommand(syncGoCmd())
	return cmd
}

func syncGoCmd() *cobra.Command {
	var f string
	var repoPath string
	cmd := &cobra.Command{
		Use:   "go",
		Short: "sync a Go file with Lekko config functions to a local config repository",
		RunE: func(cmd *cobra.Command, args []string) error {
			b, err := os.ReadFile("go.mod")
			if err != nil {
				return errors.Wrap(err, "find go.mod in working directory")
			}
			mf, err := modfile.ParseLax("go.mod", b, nil)
			if err != nil {
				return err
			}

			if len(repoPath) == 0 {
				repoPath, err = repo.PrepareGithubRepo()
				if err != nil {
					return err
				}
			}
			syncer := sync.NewGoSyncer(mf.Module.Mod.Path, f, repoPath)
			return syncer.Sync(cmd.Context())
		},
	}
	cmd.Flags().StringVarP(&f, "file", "f", "lekko.go", "Go file to sync to config repository") // TODO make this less dumb
	cmd.Flags().StringVarP(&repoPath, "repo-path", "r", "", "path to config repository, will use autodetect if not set")
	return cmd
}
