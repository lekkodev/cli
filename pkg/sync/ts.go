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
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/lekkodev/cli/pkg/gen"
	"github.com/lekkodev/cli/pkg/native"
	"github.com/lekkodev/cli/pkg/repo"
	"github.com/pkg/errors"

	featurev1beta1 "buf.build/gen/go/lekkodev/cli/protocolbuffers/go/lekko/feature/v1beta1"
)

const syncTSExec = "lekko-sync"

var NoSyncTSError = errors.New("missing npx lekko-sync")

// TODO: consider consolidating bisync methods
func BisyncTS(ctx context.Context, lekkoPath, repoPath string) ([]string, error) {
	repoContents, err := SyncTS(lekkoPath)
	if err != nil {
		return nil, errors.Wrap(err, "sync")
	}
	if err := WriteContentsToLocalRepo(ctx, repoContents, repoPath); err != nil {
		return nil, errors.Wrap(err, "write to repository")
	}
	// Generate only for namespaces that were synced
	files := make([]string, len(repoContents.Namespaces))
	for i, namespace := range repoContents.Namespaces {
		files[i] = filepath.Join(lekkoPath, namespace.Name+native.LangTypeScript.Ext())
		if err := gen.GenFormattedTS(ctx, repoPath, namespace.Name, files[i]); err != nil {
			return nil, errors.Wrapf(err, "generate code for %s", namespace.Name)
		}
	}
	return files, nil
}

func SyncTS(lekkoPath string) (*featurev1beta1.RepositoryContents, error) {
	// TODO: If command not found, give helpful error message prompting user to update dependencies
	tsSyncCmd := exec.Command("npx", syncTSExec, "--lekko-dir", lekkoPath)
	output, err := tsSyncCmd.CombinedOutput()
	if err != nil {
		return nil, checkSyncTSError(output)
	}
	repoContents, err := repo.DecodeRepositoryContents(output)
	if err != nil {
		return nil, errors.Wrap(err, "decode")
	}
	return repoContents, nil
}

func SyncTSFiles(lekkoFiles ...string) (*featurev1beta1.RepositoryContents, error) {
	commafied := strings.Join(lekkoFiles, ",")
	tsSyncCmd := exec.Command("npx", syncTSExec, "--lekko-files", commafied)
	output, err := tsSyncCmd.CombinedOutput()
	if err != nil {
		return nil, checkSyncTSError(output)
	}
	repoContents, err := repo.DecodeRepositoryContents(output)
	if err != nil {
		return nil, errors.Wrap(err, "decode")
	}
	return repoContents, nil
}

func checkSyncTSError(output []byte) error {
	o := string(output)
	if strings.Contains(o, "404") && strings.Contains(o, syncTSExec) {
		return NoSyncTSError
	}
	r := regexp.MustCompile("LekkoParseError: (.*):(\\d+):(\\d+):(\\d+):(\\d+) - (.*)")
	matches := r.FindStringSubmatch(o)
	if matches == nil {
		return NewSyncError(errors.New(o))
	}
	startLine, err := strconv.Atoi(matches[2])
	if err != nil {
		return NewSyncError(errors.New(o))
	}
	startCol, err := strconv.Atoi(matches[3])
	if err != nil {
		return NewSyncError(errors.New(o))
	}
	endLine, err := strconv.Atoi(matches[4])
	if err != nil {
		return NewSyncError(errors.New(o))
	}
	endCol, err := strconv.Atoi(matches[5])
	if err != nil {
		return NewSyncError(errors.New(o))
	}
	return NewSyncPosError(errors.New(matches[4]), matches[1], startLine, startCol, endLine, endCol)
}
