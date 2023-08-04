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

//go:build integration
// +build integration

package repo

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/go-git/go-git/v5/plumbing"
	"github.com/lekkodev/cli/pkg/gh"
	"github.com/lekkodev/cli/pkg/star/static"
	"github.com/lekkodev/go-sdk/pkg/eval"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	integrationTestOwnerName = "lekkodev"
	integrationTestRepoName  = "integration-test"
	integrationTestURL       = "https://github.com/" + integrationTestOwnerName + "/" + integrationTestRepoName
	// https://github.com/lekkodev/integration-test/commit/3954443a24b9053c3fb67ad453c4035e9f5aa4ed
	restoreCommitHash         = "3954443a24b9053c3fb67ad453c4035e9f5aa4ed"
	restoreFeatureDescription = "int-test-description"
)

type authProvider struct {
	user, token string
}

func (ap *authProvider) GetUsername() string { return ap.user }
func (ap *authProvider) GetToken() string    { return ap.token }

func newAuthProvider() *authProvider {
	return &authProvider{
		user:  "lekkoci",
		token: os.Getenv("GITHUB_TOKEN"),
	}
}

// Performs an end-to-end integration test on the Repo object. This test runs on ci.
// It can also be run locally, by running `GITHUB_TOKEN=ghu_**** make integration`.
func TestRepoIntegration(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	// Initialize dependencies
	ap := newAuthProvider()
	require.NoError(t, credentialsExist(ap))
	tmpDir := t.TempDir()
	ghCli := gh.NewGithubClientFromToken(ctx, ap.GetToken())
	user, err := ghCli.GetUser(ctx)
	require.NoError(t, err, "github token should be valid")
	t.Logf("Running integration test on behalf of '%s'\n", user.GetLogin())

	var r *repository
	t.Run("Constructor", func(t *testing.T) { r = testConstructor(t, tmpDir, ap) })
	require.NotNil(t, r)
	var branchName string
	t.Run("Review", func(t *testing.T) { branchName = testReview(ctx, t, r, ghCli, ap) })
	require.NotEmpty(t, branchName)
	t.Run("Ephemeral", func(t *testing.T) { testEphemeral(ctx, t, ap, branchName) })
	t.Run("Cleanup", func(t *testing.T) { testCleanup(ctx, t, r, ghCli, ap, branchName) })
	t.Run("Restore", func(t *testing.T) { testRestore(ctx, t, tmpDir, ap) })
}

func testConstructor(t *testing.T, tmpDir string, ap AuthProvider) *repository {
	path := filepath.Join(tmpDir, "test-constructor")
	cr, err := NewLocalClone(path, integrationTestURL, ap)
	require.NoError(t, err)
	r, ok := cr.(*repository)
	require.True(t, ok)
	branch, err := r.BranchName()
	require.NoError(t, err)
	require.Equal(t, r.DefaultBranchName(), branch, "should be on default branch after construction")
	// ensure the remote is set up correctly
	remote, err := r.repo.Remote(RemoteName)
	require.NoError(t, err)
	require.NotNil(t, remote)
	remoteCfg := remote.Config()
	require.NotNil(t, remoteCfg)
	require.Len(t, remoteCfg.URLs, 1)
	assert.Equal(t, remoteCfg.URLs[0], integrationTestURL)
	assertUpToDate(t, r, r.DefaultBranchName())
	return r
}

func testReview(ctx context.Context, t *testing.T, r *repository, ghCli *gh.GithubClient, ap AuthProvider) string {
	// Add feature, so we have some changes in our working directory.
	namespace := "default"
	getFeatureName := func(ft string) string {
		return fmt.Sprintf("integration_test_%s", ft)
	}
	for _, fType := range eval.FeatureTypes() {
		var protoMessageName string
		if eval.FeatureType(fType) == eval.FeatureTypeProto {
			protoMessageName = "google.protobuf.BoolValue"
		}
		path, err := r.AddFeature(ctx, namespace, getFeatureName(fType), eval.FeatureType(fType), protoMessageName)
		require.NoError(t, err)
		t.Logf("wrote config to path %s\n", path)
	}
	// Compile
	rootMD, _, err := r.ParseMetadata(ctx)
	require.NoError(t, err)
	registry, err := r.BuildDynamicTypeRegistry(ctx, rootMD.ProtoDirectory)
	require.NoError(t, err)
	var b bytes.Buffer
	clear := r.ConfigureLogger(&LoggingConfiguration{Writer: &b, ColorsDisabled: true})
	defer clear()
	result, err := r.Compile(ctx, &CompileRequest{
		Registry: registry,
		DryRun:   false,
	})
	require.NoError(t, err)
	assert.GreaterOrEqual(t, len(result), 6)
	for _, fcr := range result {
		require.NoErrorf(t, fcr.Err(), "%s/%s compilation result should have no error", fcr.NamespaceName, fcr.FeatureName)
	}
	assert.NotEmpty(t, b.String())
	// Eval
	for _, fType := range eval.FeatureTypes() {
		evalResult, evalType, resultPath, err := r.Eval(ctx, namespace, getFeatureName(fType), nil)
		require.NoError(t, err)
		require.NotNil(t, evalResult)
		require.Equal(t, fType, string(evalType))
		require.Len(t, resultPath, 0)
	}
	// Review
	url, err := r.Review(ctx, "Test PR", ghCli, ap)
	require.NoError(t, err)
	t.Logf("Created PR: %s\n", url)

	// check current branch
	branchName, err := r.BranchName()
	require.NoError(t, err)
	require.NotEqual(t, r.DefaultBranchName(), branchName)
	assertUpToDate(t, r, branchName)
	return branchName
}

func testEphemeral(ctx context.Context, t *testing.T, ap AuthProvider, branchName string) {
	cr, err := NewEphemeral(integrationTestURL, ap, &branchName)
	require.NoError(t, err)
	r, ok := cr.(*repository)
	require.True(t, ok)
	currentBranchName, err := r.BranchName()
	require.NoError(t, err)
	assert.Equal(t, branchName, currentBranchName)
	assertUpToDate(t, r, branchName)
}

func testCleanup(ctx context.Context, t *testing.T, r *repository, ghCli *gh.GithubClient, ap AuthProvider, branchName string) {
	require.NoError(t, r.Cleanup(ctx, &branchName, ap))
	_, resp, err := ghCli.Repositories.GetBranch(ctx, integrationTestOwnerName, integrationTestRepoName, branchName, false)
	require.Error(t, err)
	assert.Equal(t, 404, resp.StatusCode, "remote branch should be deleted")
	// check current branch
	currentBranchName, err := r.BranchName()
	require.NoError(t, err)
	assert.Equal(t, r.DefaultBranchName(), currentBranchName)
	localRef, err := r.repo.Reference(plumbing.NewBranchReferenceName(branchName), false)
	assert.Error(t, err, "local ref should no longer exist")
	assert.Nil(t, localRef)
}

func assertUpToDate(t *testing.T, r *repository, branchName string) {
	localRef, err := r.repo.Reference(plumbing.NewBranchReferenceName(branchName), false)
	require.NoError(t, err)
	remoteRef, err := r.repo.Reference(plumbing.NewRemoteReferenceName(RemoteName, branchName), false)
	require.NoError(t, err)
	assert.Equal(t, remoteRef.Hash(), localRef.Hash(), "local branch must be up to date")
}

func testRestore(ctx context.Context, t *testing.T, tmpDir string, ap AuthProvider) {
	path := filepath.Join(tmpDir, "test-restore")
	r, err := NewLocalClone(path, integrationTestURL, ap)
	require.NoError(t, err)
	require.NoError(t, r.RestoreWorkingDirectory(restoreCommitHash))
	fc, err := r.GetFeatureContents(ctx, "default", "example")
	require.NoError(t, err)
	f, err := static.NewWalker(fc.File.StarlarkFileName, fc.Star, nil).Build()
	require.NoError(t, err)
	assert.Equal(t, restoreFeatureDescription, f.FeatureOld.Description)
}
