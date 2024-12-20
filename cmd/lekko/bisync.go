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
	"context"

	"github.com/lainio/err2/try"
	"github.com/lekkodev/cli/pkg/dotlekko"
	"github.com/lekkodev/cli/pkg/native"
	"github.com/lekkodev/cli/pkg/repo"
	"github.com/lekkodev/cli/pkg/sync"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
)

func bisyncCmd() *cobra.Command {
	var lekkoPath, repoOwner, repoName, repoPath string
	cmd := &cobra.Command{
		Use:   "bisync",
		Short: "bi-directionally sync Lekko config code from a project to a local config repository",
		Long: `Bi-directionally sync Lekko config code from a project to a local config repository.

Files at the provided path that contain valid Lekko config functions will first be translated and synced to the config repository on the local filesystem, then translated back to Lekko-canonical form, performing any code generation as necessary.
This may affect ordering of functions/parameters and formatting.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			nlProject := try.To1(native.DetectNativeLang(""))
			return bisync(context.Background(), nlProject, lekkoPath, repoOwner, repoName, repoPath)
		},
	}
	cmd.Flags().StringVarP(&lekkoPath, "lekko-path", "p", "", "Path to Lekko native config files, will use autodetect if not set")
	cmd.Flags().StringVar(&repoOwner, "repo-owner", "", "GitHub owner of config repository, will use autodetect if not set")
	cmd.Flags().StringVar(&repoName, "repo-name", "", "GitHub name of config repository, will use autodetect if not set")
	cmd.Flags().StringVarP(&repoPath, "repo-path", "r", "", "path to config repository, will use autodetect if not set")
	cmd.AddCommand(bisyncGoCmd())
	cmd.AddCommand(bisyncTSCmd())
	return cmd
}

func bisync(ctx context.Context, project *native.Project, lekkoPath, repoOwner, repoName, repoPath string) error {
	if len(lekkoPath) == 0 {
		dot := try.To1(dotlekko.ReadDotLekko(""))
		lekkoPath = dot.LekkoPath
	}
	if len(repoOwner) == 0 || len(repoName) == 0 {
		dot := try.To1(dotlekko.ReadDotLekko(""))
		repoOwner, repoName = dot.GetRepoInfo()
	}
	if len(repoPath) == 0 {
		repoPath = try.To1(repo.PrepareGithubRepo())
	}
	switch project.Language {
	case native.LangGo:
		_ = try.To1(sync.BisyncGo(ctx, lekkoPath, lekkoPath, repoOwner, repoName, repoPath))
	case native.LangTypeScript:
		_ = try.To1(sync.BisyncTS(ctx, lekkoPath, repoPath))
	default:
		return errors.New("unsupported language")
	}
	return nil
}

func bisyncGoCmd() *cobra.Command {
	var lekkoPath, repoOwner, repoName, repoPath string
	cmd := &cobra.Command{
		Use:   "go",
		Short: "Lekko bisync for Go. Should be run from project root.",
		RunE: func(cmd *cobra.Command, args []string) error {
			nlProject := try.To1(native.DetectNativeLang(""))
			if nlProject.Language != native.LangGo {
				return errors.Errorf("not a Go project, detected %v instead", nlProject.Language)
			}
			return bisync(context.Background(), nlProject, lekkoPath, repoOwner, repoName, repoPath)
		},
	}
	cmd.Flags().StringVarP(&lekkoPath, "lekko-path", "p", "", "Path to Lekko native config files, will use autodetect if not set")
	cmd.Flags().StringVar(&repoOwner, "repo-owner", "", "GitHub owner of config repository, will use autodetect if not set")
	cmd.Flags().StringVar(&repoName, "repo-name", "", "GitHub name of config repository, will use autodetect if not set")
	cmd.Flags().StringVarP(&repoPath, "repo-path", "r", "", "path to config repository, will use autodetect if not set")
	return cmd
}

func bisyncTSCmd() *cobra.Command {
	var lekkoPath, repoOwner, repoName, repoPath string
	cmd := &cobra.Command{
		Use:   "ts",
		Short: "Lekko bisync for Typescript. Should be run from project root.",
		RunE: func(cmd *cobra.Command, args []string) error {
			nlProject := try.To1(native.DetectNativeLang(""))
			if nlProject.Language != native.LangTypeScript {
				return errors.Errorf("not a TypeScript project, detected %v instead", nlProject.Language)
			}
			return bisync(context.Background(), nlProject, lekkoPath, repoOwner, repoName, repoPath)
		},
	}
	cmd.Flags().StringVarP(&lekkoPath, "lekko-path", "p", "", "Path to Lekko native config files, will use autodetect if not set")
	cmd.Flags().StringVar(&repoOwner, "repo-owner", "", "GitHub owner of config repository, will use autodetect if not set")
	cmd.Flags().StringVar(&repoName, "repo-name", "", "GitHub name of config repository, will use autodetect if not set")
	cmd.Flags().StringVarP(&repoPath, "repo-path", "r", "", "path to config repository, will use autodetect if not set")
	return cmd
}
