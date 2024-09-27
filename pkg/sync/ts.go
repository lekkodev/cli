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
	"strings"

	"github.com/lekkodev/cli/pkg/gen"
	"github.com/lekkodev/cli/pkg/native"
	"github.com/lekkodev/cli/pkg/repo"
	"github.com/pkg/errors"

	featurev1beta1 "buf.build/gen/go/lekkodev/cli/protocolbuffers/go/lekko/feature/v1beta1"
)

// TODO: consider consolidating bisync methods
func BisyncTS(ctx context.Context, lekkoPath, repoPath string) error {
	repoContents, err := SyncTS(lekkoPath)
	if err != nil {
		return errors.Wrap(err, "sync")
	}
	if err := WriteContentsToLocalRepo(ctx, repoContents, repoPath); err != nil {
		return errors.Wrap(err, "write to repository")
	}
	// Generate only for namespaces that were synced
	for _, namespace := range repoContents.Namespaces {
		outFilename := filepath.Join(lekkoPath, namespace.Name+native.LangTypeScript.Ext())
		if err := gen.GenFormattedTS(ctx, repoPath, namespace.Name, outFilename); err != nil {
			return errors.Wrapf(err, "generate code for %s", namespace.Name)
		}
	}
	return nil
}

func SyncTS(lekkoPath string) (*featurev1beta1.RepositoryContents, error) {
	// TODO: If command not found, give helpful error message prompting user to update dependencies
	tsSyncCmd := exec.Command("npx", "lekko-sync", "--lekko-dir", lekkoPath)
	output, err := tsSyncCmd.CombinedOutput()
	if err != nil {
		return nil, errors.Wrapf(err, "sync ts: %s", output)
	}
<<<<<<< HEAD
	return SyncTSOutputToRepositoryContents(output)
=======
	repoContents, err := repo.DecodeRepositoryContents(output)
	if err != nil {
		return nil, errors.Wrap(err, "decode sync ts output")
	}
	return repoContents, nil
>>>>>>> origin/main
}

func SyncTSFiles(lekkoFiles ...string) (*featurev1beta1.RepositoryContents, error) {
	commafied := strings.Join(lekkoFiles, ",")
	tsSyncCmd := exec.Command("npx", "lekko-sync", "--lekko-files", commafied)
	output, err := tsSyncCmd.CombinedOutput()
	if err != nil {
		return nil, errors.Wrapf(err, "sync ts: %s", output)
	}
<<<<<<< HEAD
	return SyncTSOutputToRepositoryContents(output)
}

// Output is expected to be base64 encoded serialized RepositoryContents message
func SyncTSOutputToRepositoryContents(output []byte) (*featurev1beta1.RepositoryContents, error) {
	// Because Protobuf is not self-describing, we have to jump through some hoops here for deserialization.
	// The RepositoryContents message contains the FDS which we want to use as the resolver for unmarshalling
	// the rest of the contents.
	decoded, err := base64.StdEncoding.DecodeString(string(output))
=======
	repoContents, err := repo.DecodeRepositoryContents(output)
>>>>>>> origin/main
	if err != nil {
		return nil, errors.Wrap(err, "decode sync ts output")
	}
	return repoContents, nil
}
