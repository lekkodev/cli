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

package sync

import (
	"context"
	stderrors "errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/AlecAivazis/survey/v2"
	"github.com/go-git/go-git/v5"
	"github.com/lainio/err2"
	"github.com/lainio/err2/try"
	"github.com/lekkodev/cli/pkg/dotlekko"
	"github.com/lekkodev/cli/pkg/gen"
	"github.com/lekkodev/cli/pkg/gitcli"
	"github.com/lekkodev/cli/pkg/native"
	"github.com/lekkodev/cli/pkg/repo"
	"github.com/pkg/errors"
)

func Push(ctx context.Context, commitMessage string, force bool, dot *dotlekko.DotLekko) (err error) {
	defer err2.Handle(&err)
	nlProject, err := native.DetectNativeLang("")
	if err != nil {
		return err
	}

	repoPath, err := repo.PrepareGithubRepo()
	if err != nil {
		return err
	}
	gitRepo, err := git.PlainOpen(repoPath)
	if err != nil {
		return errors.Wrap(err, "open git repo")
	}
	remotes, err := gitRepo.Remotes()
	if err != nil {
		return errors.Wrap(err, "get remotes")
	}
	if len(remotes) == 0 {
		return errors.New("No remote found, please finish setup instructions")
	}

	lekkoPath := dot.LekkoPath
	err = repo.ResetAndClean(gitRepo)
	if err != nil {
		return errors.Wrap(err, "reset and clean")
	}
	if _, err = gitcli.Pull(repoPath); err != nil {
		return errors.Wrap(err, "pull from GitHub")
	}
	head, err := gitRepo.Head()
	if err != nil {
		return errors.Wrap(err, "get head")
	}

	configRepo, err := repo.NewLocal(repoPath, nil)
	if err != nil {
		return errors.Wrap(err, "failed to open config repo")
	}

	updatesExistingNamespace := false
	rootMD, _, err := configRepo.ParseMetadata(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to parse config repo metadata")
	}
	nsMap := make(map[string]bool)
	for _, ns := range rootMD.Namespaces {
		nsMap[ns] = true
	}
	nativeFiles := try.To1(native.ListNativeConfigFiles(lekkoPath, nlProject.Language))
	for _, f := range nativeFiles {
		if _, ok := nsMap[try.To1(nlProject.Language.GetNamespace(f))]; ok {
			updatesExistingNamespace = true
		}
	}

	if !force {
		// no lock and there is a potential conflict
		if len(dot.LockSHA) == 0 && updatesExistingNamespace {
			return errors.New("No Lekko lock information found, please run with --force flag to push anyway")
		}
		if len(dot.LockSHA) > 0 && head.Hash().String() != dot.LockSHA {
			return repo.ErrRemoteHasChanges
		}
	}

	// Generate native configs from remote main.
	// Will be used below to detect and print changes.
	remoteDir, err := os.MkdirTemp("", "lekko-push-")
	if err != nil {
		return errors.Wrap(err, "create temp dir")
	}
	defer os.RemoveAll(remoteDir)
	namespaces := try.To1(native.ListNamespaces(lekkoPath, nlProject.Language))
	repoOwner, repoName := dot.GetRepoInfo()
	try.To(gen.GenNative(ctx, nlProject, lekkoPath, repoOwner, repoName, repoPath, gen.GenOptions{
		CodeRepoPath: remoteDir,
		Namespaces:   namespaces,
	}))
	switch nlProject.Language {
	case native.LangTypeScript:
		_, err = BisyncTS(ctx, lekkoPath, repoPath)
		if err != nil {
			return err
		}
	case native.LangGo:
		_, err = BisyncGo(ctx, lekkoPath, lekkoPath, repoOwner, repoName, repoPath)
		if err != nil {
			return err
		}
	default:
		return fmt.Errorf("unsupported language: %s", nlProject.Language)
	}

	// Print diff between local and remote
	hasChanges := false
	for _, f := range nativeFiles {
		gitDiffCmd := exec.Command("git", "diff", "--no-index", "--src-prefix=remote/", "--dst-prefix=local/", filepath.Join(remoteDir, f), f) // #nosec G204
		gitDiffCmd.Stdout = os.Stdout
		err := gitDiffCmd.Run()
		if err != nil {
			exitErr, ok := err.(*exec.ExitError)
			if !ok {
				return errors.Wrap(err, "git diff")
			}
			if exitErr.ExitCode() > 0 {
				hasChanges = true
				fmt.Println()
			}
		}
	}
	if !hasChanges && !force {
		fmt.Println("Already up to date.")
		return nil
	}

	doIt := false
	fmt.Println()
	if err := survey.AskOne(&survey.Confirm{
		Message: "Continue?",
		Default: false,
	}, &doIt); err != nil {
		return errors.Wrap(err, "prompt")
	}
	if !doIt {
		return errors.New("Aborted")
	}

	rootMD, _, err = configRepo.ParseMetadata(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to parse config repo metadata")
	}
	// re-build proto
	registry, err := configRepo.ReBuildDynamicTypeRegistry(ctx, rootMD.ProtoDirectory, rootMD.UseExternalTypes)
	if err != nil {
		return errors.Wrap(err, "rebuild type registry")
	}
	_, err = configRepo.Compile(ctx, &repo.CompileRequest{
		Registry: registry,
	})
	if err != nil {
		return errors.Wrap(err, "compile before push")
	}
	fmt.Printf("Compiled successfully\n\n")

	worktree, err := gitRepo.Worktree()
	if err != nil {
		return err
	}
	status, err := worktree.Status()
	if err != nil {
		return err
	}
	headBefore, err := gitRepo.Head()
	if err != nil {
		return err
	}
	if !status.IsClean() {
		_, err = worktree.Add(".")
		if err != nil {
			return err
		}
		if len(commitMessage) == 0 {
			commitMessage = "Configs commit"
		}
		_, err = worktree.Commit(commitMessage, &git.CommitOptions{
			All: true,
		})
		if err != nil {
			return err
		}
	}
	// push to GitHub
	// assuming that there is only one remote and one URL
	fmt.Printf("Pushing to %s\n", remotes[0].Config().URLs[0])
	pushOutput, err := gitcli.Push(repoPath)
	fmt.Println(string(pushOutput))
	if err != nil {
		err = errors.Wrap(err, "failed to push")
		// Undo commit that we made before push.
		// Soft reset will keep changes as staged.
		errReset := worktree.Reset(&git.ResetOptions{
			Commit: headBefore.Hash(),
			Mode:   git.SoftReset,
		})
		if errReset != nil {
			return errors.Wrap(stderrors.Join(err, errReset), "failed to reset changes")
		}
		return err
	}

	// Take commit SHA for synchronizing with code repo
	head, err = gitRepo.Head()
	if err != nil {
		return err
	}
	headSHA := head.Hash().String()
	dot.LockSHA = headSHA
	if err := dot.WriteBack(); err != nil {
		return errors.Wrap(err, "write back .lekko file")
	}

	return nil
}
