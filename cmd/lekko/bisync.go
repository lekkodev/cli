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
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/lekkodev/cli/cmd/lekko/gen"
	"github.com/lekkodev/cli/pkg/dotlekko"
	"github.com/lekkodev/cli/pkg/logging"
	"github.com/lekkodev/cli/pkg/repo"
	"github.com/lekkodev/cli/pkg/secrets"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"golang.org/x/mod/modfile"
)

func bisyncCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "bisync",
		Short: "bi-directionally sync Lekko config code from a project to a local config repository",
		Long: `Bi-directionally sync Lekko config code from a project to a local config repository.

Files at the provided path that contain valid Lekko config functions will first be translated and synced to the config repository on the local filesystem, then translated back to Lekko-canonical form, performing any code generation as necessary.
This may affect ordering of functions/parameters and formatting.`,
	}
	cmd.AddCommand(bisyncGoCmd())
	return cmd
}

func bisyncGoCmd() *cobra.Command {
	var path, repoPath string
	cmd := &cobra.Command{
		Use:   "go",
		Short: "Lekko bisync for Go. Should be run from project root.",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			b, err := os.ReadFile("go.mod")
			if err != nil {
				return errors.Wrap(err, "find go.mod in working directory")
			}
			mf, err := modfile.ParseLax("go.mod", b, nil)
			if err != nil {
				return err
			}
			if len(path) == 0 {
				dot, err := dotlekko.ReadDotLekko()
				if err != nil {
					return err
				}
				path = dot.LekkoPath
			}
			if len(repoPath) == 0 {
				rs := secrets.NewSecretsOrFail(secrets.RequireGithub(), secrets.RequireLekko())
				repoPath, err = repo.PrepareGithubRepo(rs)
				if err != nil {
					return err
				}
			}
			// Traverse target path, finding namespaces
			// TODO: consider making this more efficient for batch gen/sync
			if err := filepath.WalkDir(path, func(p string, d fs.DirEntry, err error) error {
				// Skip generated proto dir
				if d.IsDir() && d.Name() == "proto" {
					return filepath.SkipDir
				}
				// Sync and gen
				if d.Name() == "lekko.go" {
					if err := SyncGo(ctx, p, repoPath); err != nil {
						return errors.Wrapf(err, "sync %s", p)
					}
					namespace := filepath.Base(filepath.Dir(p))
					generator := gen.NewGoGenerator(mf.Module.Mod.Path, path, repoPath, namespace)
					if err := generator.Gen(ctx); err != nil {
						return errors.Wrapf(err, "generate code for %s", namespace)
					}
					fmt.Printf("Successfully bisynced %s\n", logging.Bold(p))
				}
				// Ignore others
				return nil
			}); err != nil {
				return err
			}
			return nil
		},
	}
	cmd.Flags().StringVarP(&path, "path", "p", "internal/lekko", "path in current project containing Lekko files")
	cmd.Flags().StringVarP(&repoPath, "repo-path", "r", "", "path to local config repository, will use from 'lekko repo path' if not set")
	return cmd
}
