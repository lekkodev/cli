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
	"strings"

	"github.com/AlecAivazis/survey/v2"
	"github.com/go-git/go-git/v5"
	"github.com/lekkodev/cli/pkg/dotlekko"
	"github.com/lekkodev/cli/pkg/gen"
	"github.com/lekkodev/cli/pkg/gitcli"
	"github.com/lekkodev/cli/pkg/repo"
	"github.com/pkg/errors"
	"golang.org/x/mod/modfile"
)

type NativeLang string

var (
	GO NativeLang = "go"
	TS NativeLang = "ts"
)

func DetectNativeLang() (NativeLang, error) {
	// naive check for "known" project types
	if _, err := os.Stat("go.mod"); err == nil {
		return GO, nil
	} else if _, err = os.Stat("package.json"); err == nil {
		return TS, nil
	}
	return "", errors.New("Unknown project type, Lekko currently supports Go and NPM projects.")
}

func NativeLangFromExt(filename string) (NativeLang, error) {
	ext := filepath.Ext(filename)
	switch ext {
	case ".go":
		return GO, nil
	case ".ts":
		return TS, nil
	}
	return "", errors.New("Unsupported language")
}

func (l *NativeLang) Ext() string {
	switch *l {
	case GO:
		return ".go"
	case TS:
		return ".ts"
	}
	return ""
}

func (l *NativeLang) GetNamespace(filename string) string {
	switch *l {
	case GO:
		return filepath.Base(filepath.Dir(filename))
	case TS:
		base := filepath.Base(filename)
		return strings.TrimSuffix(base, l.Ext())
	}
	return ""
}

func Push(ctx context.Context, commitMessage string, forceLock bool, dot *dotlekko.DotLekko) error {
	nativeLang, err := DetectNativeLang()
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
	err = gitcli.Pull(repoPath)
	if err != nil {
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
	nativeFiles, err := repo.ListNativeConfigFiles(lekkoPath, nativeLang.Ext())
	if err != nil {
		return errors.Wrap(err, "list native config files")
	}
	for _, f := range nativeFiles {
		ns := strings.TrimSuffix(filepath.Base(f), nativeLang.Ext())
		if _, ok := nsMap[ns]; ok {
			updatesExistingNamespace = true
		}
	}

	if !forceLock {
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
	for _, f := range nativeFiles {
		ns := nativeLang.GetNamespace(f)
		err = GenNative(ctx, nativeLang, lekkoPath, repoPath, ns, remoteDir)
		if err != nil {
			return errors.Wrap(err, "generate native config for remote")
		}
	}

	// run 2-way sync
	switch nativeLang {
	case TS:
		tsSyncCmd := exec.Command("npx", "lekko-repo-sync", "--lekko-dir", lekkoPath)
		output, err := tsSyncCmd.CombinedOutput()
		outputStr := strings.TrimSpace(string(output))
		if len(outputStr) > 0 {
			fmt.Println(string(output))
		}
		if err != nil {
			return errors.Wrap(err, "Lekko Typescript tools not found, please make sure that you are inside a node project and have up to date Lekko packages.")
		}
	case GO:
		_, err = Bisync(ctx, lekkoPath, lekkoPath, repoPath)
		if err != nil {
			return err
		}
	default:
		return fmt.Errorf("unsupported language: %s", nativeLang)
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
	if !hasChanges {
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
	// auth, err := repo.GitAuthForRemote(gitRepo, "origin", rs)
	// if err != nil {
	// 	return err
	// }
	// err = gitRepo.Push(&git.PushOptions{
	// 	Auth: auth,
	// })
	if err != nil {
		// if strings.Contains(err.Error(), "non-fast-forward update") {
		// 	return repo.ErrRemoteHasChanges
		// }
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
	// if errors.Is(err, git.NoErrAlreadyUpToDate) {
	// 	fmt.Println("Already up to date.")
	// 	return nil
	// }
	head, err = gitRepo.Head()
	if err != nil {
		return err
	}
	headSHA := head.Hash().String()
	fmt.Printf("Successfully pushed changes as %s\n", headSHA)

	// Take commit SHA for synchronizing with code repo
	dot.LockSHA = headSHA
	if err := dot.WriteBack(); err != nil {
		return errors.Wrap(err, "write back .lekko file")
	}

	// Pull to get remote branches
	return gitcli.Pull(repoPath)
}

func GenNative(ctx context.Context, nativeLang NativeLang, lekkoPath, repoPath, ns, dir string) error {
	switch nativeLang {
	case TS:
		err := os.MkdirAll(filepath.Join(dir, lekkoPath), 0770)
		if err != nil {
			return errors.Wrap(err, "create output dir")
		}
		outFilename := filepath.Join(dir, lekkoPath, ns+nativeLang.Ext())
		return gen.GenFormattedTS(ctx, repoPath, ns, outFilename)
	case GO:
		outDir := filepath.Join(dir, lekkoPath)
		return genFormattedGo(ctx, ns, repoPath, outDir, lekkoPath)
	default:
		return errors.New("Unsupported language")
	}
}

func genFormattedGo(ctx context.Context, namespace, repoPath, outDir, lekkoPath string) error {
	b, err := os.ReadFile("go.mod")
	if err != nil {
		return errors.Wrap(err, "find go.mod in working directory")
	}
	mf, err := modfile.ParseLax("go.mod", b, nil)
	if err != nil {
		return err
	}

	generator := gen.NewGoGenerator(mf.Module.Mod.Path, outDir, lekkoPath, repoPath, namespace)
	if err := generator.Gen(ctx); err != nil {
		return errors.Wrapf(err, "generate code for %s", namespace)
	}
	return nil
}
