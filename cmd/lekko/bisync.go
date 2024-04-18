package main

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/lekkodev/cli/cmd/lekko/gen"
	"github.com/lekkodev/cli/pkg/logging"
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
			if repoPath == "" {
				rs := secrets.NewSecretsOrFail()
				if len(rs.GetLekkoRepoPath()) == 0 {
					return errors.New("no local config repository available, pass '--config-path' or use 'lekko repo path --set'")
				}
				repoPath = rs.GetLekkoRepoPath()
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
					fmt.Printf("Successfully bisynced namespace %s\n", logging.Bold(p))
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
