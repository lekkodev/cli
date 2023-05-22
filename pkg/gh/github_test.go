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

package gh

import (
	"context"
	"testing"

	"github.com/google/go-github/v52/github"
	"github.com/migueleliasweb/go-github-mock/src/mock"
	"github.com/stretchr/testify/require"
)

func TestParseOwnerRepo(t *testing.T) {
	o, r, err := ParseOwnerRepo("git@github.com:lekkodev/kudos-backend-config.git")
	require.NoError(t, err)
	require.Equal(t, o, "lekkodev")
	require.Equal(t, r, "kudos-backend-config")

	o, r, err = ParseOwnerRepo("https://github.com/lekkodev/config-test.git")
	require.NoError(t, err)
	require.Equal(t, o, "lekkodev")
	require.Equal(t, r, "config-test")
}

func TestGetUserOrganizations(t *testing.T) {
	mockedHTTPClient := mock.NewMockedHTTPClient(
		mock.WithRequestMatch(
			mock.GetUserOrgs,
			[]github.Organization{
				{
					Login: github.String("lekkodev1"),
				},
				{
					Login: github.String("lekkodev2"),
				},
				{
					Login: github.String("lekkodev3"),
				},
			},
		),
	)
	f := NewGithubClient(mockedHTTPClient)
	result, err := f.GetUserOrganizations(context.Background())
	require.NoError(t, err)
	require.NotNil(t, result)
	require.Len(t, result, 3)
	require.Equal(t, "lekkodev1", result[0])
	require.Equal(t, "lekkodev2", result[1])
	require.Equal(t, "lekkodev3", result[2])
}

func TestGetAllUserRepositories(t *testing.T) {
	mockedHTTPClient := mock.NewMockedHTTPClient(
		mock.WithRequestMatch(
			mock.GetUserRepos,
			[]github.Repository{
				{
					FullName: github.String("lekkodev/hello-world"),
				},
				{
					FullName: github.String("lekkodev/foobar"),
				},
				{
					FullName: github.String("lekkodev/baz"),
				},
			},
		),
	)
	mgh := NewGithubClient(mockedHTTPClient)
	repos, err := mgh.GetAllUserRepositories(context.Background(), "")
	require.NoError(t, err)
	require.Len(t, repos, 3)
}
