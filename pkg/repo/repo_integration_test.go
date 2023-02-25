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
	"os"
	"path/filepath"
	"testing"

	"github.com/go-git/go-git/v5/plumbing"
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
		user: "lekkoci",
		// token: os.Getenv("GITHUB_TOKEN"),
		token: "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",
	}
}

func TestRepoIntegration(t *testing.T) {
	path, err := os.MkdirTemp("", integrationTestRepoName)
	require.NoError(t, err, "make tmp directory")
	defer func() {
		assert.NoErrorf(t, os.RemoveAll(path), "failed to remove path %s", path)
	}()
	ap := newAuthProvider()
	require.NotEmpty(t, ap.GetUsername(), "no auth username")
	require.NotEmpty(t, ap.GetToken(), "no auth token")
	r, err := NewLocalClone(filepath.Join(path, integrationTestRepoName), integrationTestURL, ap)
	require.NoError(t, err)
	t.Run("Constructor", func(t *testing.T) { testConstructor(t, r) })
}

func testConstructor(t *testing.T, r *Repo) {
	branch, err := r.BranchName()
	require.NoError(t, err)
	require.Equal(t, MainBranchName, branch, "should be on main branch after construction")
	// ensure the remote is set up correctly
	remote, err := r.Repo.Remote(RemoteName)
	require.NoError(t, err)
	require.NotNil(t, remote)
	remoteCfg := remote.Config()
	require.NotNil(t, remoteCfg)
	require.Len(t, remoteCfg.URLs, 1)
	assert.Equal(t, remoteCfg.URLs[0], integrationTestURL)
	localRef, err := r.Repo.Reference(plumbing.NewBranchReferenceName(MainBranchName), false)
	require.NoError(t, err)
	remoteRef, err := r.Repo.Reference(plumbing.NewRemoteReferenceName(RemoteName, MainBranchName), false)
	require.NoError(t, err)
	assert.Equal(t, remoteRef.Hash(), localRef.Hash(), "local branch must be up to date")
	t.Fatal("making sure this works")
}
