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
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/go-git/go-git/v5/plumbing"
	"github.com/lekkodev/cli/pkg/feature"
	"github.com/lekkodev/cli/pkg/gh"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	integrationTestRepoName = "integration-test"
	integrationTestURL      = "https://github.com/lekkodev/" + integrationTestRepoName
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
func TestRepoIntegration(t *testing.T) {

	ap := newAuthProvider()
	require.NoError(t, credentialsExist(ap))

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	path := filepath.Join(t.TempDir(), integrationTestRepoName)
	var r *Repo
	t.Run("Constructor", func(t *testing.T) { r = testConstructor(t, path, ap) })
	require.NotNil(t, r)
	ghCli := gh.NewGithubClientFromToken(ctx, ap.GetToken())
	_, err := ghCli.GetUser(ctx)
	require.NoError(t, err)
	var branchName string
	t.Run("Review", func(t *testing.T) { branchName = testReview(ctx, t, r, ghCli, ap) })
	require.NotEmpty(t, branchName)
	// TODO: create an ephemeral repo and run CheckoutRemoteBranch 
	t.Run("Cleanup", func(t *testing.T) { testCleanup(ctx, t, r, ghCli, ap, branchName) })
}

func testConstructor(t *testing.T, path string, ap AuthProvider) *Repo {
	r, err := NewLocalClone(path, integrationTestURL, ap)
	require.NoError(t, err)
	branch, err := r.BranchName()
	require.NoError(t, err)
	require.Equal(t, MainBranchName, branch, "should be on main branch after construction")
	// ensure the remote is set up correctly
	remote, err := r.repo.Remote(RemoteName)
	require.NoError(t, err)
	require.NotNil(t, remote)
	remoteCfg := remote.Config()
	require.NotNil(t, remoteCfg)
	require.Len(t, remoteCfg.URLs, 1)
	assert.Equal(t, remoteCfg.URLs[0], integrationTestURL)
	assertUpToDate(t, r, MainBranchName)
	return r
}

func testReview(ctx context.Context, t *testing.T, r *Repo, ghCli *gh.GithubClient, ap AuthProvider) string {
	// Add feature, so we have some changes in our working directory.
	namespace, featureName := "default", "test"
	path, err := r.AddFeature(ctx, namespace, featureName, feature.FeatureTypeBool)
	require.NoError(t, err)
	t.Logf("wrote feature to path %s\n", path)
	// Compile
	rootMD, _, err := r.ParseMetadata(ctx)
	require.NoError(t, err)
	registry, err := r.BuildDynamicTypeRegistry(ctx, rootMD.ProtoDirectory)
	require.NoError(t, err)
	result, err := r.Compile(ctx, &CompileRequest{
		Registry:        registry,
		NamespaceFilter: namespace,
		FeatureFilter:   featureName,
		Persist:         true,
	})
	require.NoError(t, err)
	assert.Len(t, result, 1)
	require.NoError(t, result[0].Err())
	// Review
	url, err := r.Review(ctx, "Test PR", ghCli, ap)
	require.NoError(t, err)
	t.Logf("Created PR: %s\n", url)

	// check current branch
	branchName, err := r.BranchName()
	require.NoError(t, err)
	require.NotEqual(t, MainBranchName, branchName)
	assertUpToDate(t, r, branchName)
	return branchName
}

func testCleanup(ctx context.Context, t *testing.T, r *Repo, ghCli *gh.GithubClient, ap AuthProvider, branchName string) {
	require.NoError(t, r.Cleanup(ctx, &branchName, ap))
	_, resp, err := ghCli.Repositories.GetBranch(ctx, "lekkodev", integrationTestRepoName, branchName, false)
	require.Error(t, err)
	assert.Equal(t, 404, resp.StatusCode, "remote branch should be deleted")
	// check current branch
	currentBranchName, err := r.BranchName()
	require.NoError(t, err)
	assert.Equal(t, MainBranchName, currentBranchName)
	// TODO: check that local branch ref no longer exists
}

func assertUpToDate(t *testing.T, r *Repo, branchName string) {
	localRef, err := r.repo.Reference(plumbing.NewBranchReferenceName(branchName), false)
	require.NoError(t, err)
	remoteRef, err := r.repo.Reference(plumbing.NewRemoteReferenceName(RemoteName, branchName), false)
	require.NoError(t, err)
	assert.Equal(t, remoteRef.Hash(), localRef.Hash(), "local branch must be up to date")
}
