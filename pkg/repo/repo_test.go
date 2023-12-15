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

package repo

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type noopAuthProvider struct{}

func (noopAuthProvider) GetUsername() string {
	return ""
}

func (noopAuthProvider) GetToken() string {
	return ""
}

func TestGetCommitSignatureNoGitHub(t *testing.T) {
	ctx := context.Background()

	lekkoUser := "test@lekko.com"
	sign, err := GetCommitSignature(ctx, &noopAuthProvider{}, lekkoUser)
	require.NoError(t, err)
	assert.Equal(t, lekkoUser, sign.Email)
	assert.Equal(t, "test", sign.Name)
}

func TestGetCoauthorInformation(t *testing.T) {
	for i, tc := range []struct {
		commitMsg     string
		expectedName  string
		expectedEmail string
	}{
		{
			commitMsg: "commit (#464)",
		},
		{
			commitMsg:     "test commit (#51)\n\nCo-authored-by: lekko-app[bot] <108442683+lekko-app[bot]@users.noreply.github.com>\nCo-authored-by: Average Joe <12345+joe@users.noreply.github.com>",
			expectedName:  "Average Joe",
			expectedEmail: "12345+joe@users.noreply.github.com",
		},
		{
			commitMsg: "test commit (#51)\n\nCo-authored-by: lekko-app[bot] <108442683+lekko-app[bot]@users.noreply.github.com>",
		},
		{
			commitMsg:     "test commit (#51)\n\nCo-authored-by: Average Joe <12345+joe@users.noreply.github.com>",
			expectedName:  "Average Joe",
			expectedEmail: "12345+joe@users.noreply.github.com",
		},
		{
			commitMsg: "test commit (#51)\n\nother unrelated things",
		},
		{
			commitMsg:     "test commit (#51)\n\nCo-authored-by: lekko-app[bot] <108442683+lekko-app[bot]@users.noreply.github.com>\nCo-authored-by: Average Joe <12345+joe@users.noreply.github.com>\nCo-authored-by: Steve <12345+steve@users.noreply.github.com>",
			expectedName:  "Average Joe",
			expectedEmail: "12345+joe@users.noreply.github.com",
		},
		{
			commitMsg:     "test commit (#51)\n\nCo-authored-by: Steve <12345+steve@users.noreply.github.com>",
			expectedName:  "Steve",
			expectedEmail: "12345+steve@users.noreply.github.com",
		},
		{
			commitMsg:    "test commit (#51)\n\nCo-authored-by: Steve",
			expectedName: "Steve",
		},
	} {
		t.Run(fmt.Sprintf("%d|%s", i, tc.commitMsg), func(t *testing.T) {
			name, email := GetCoauthorInformation(tc.commitMsg)
			assert.Equal(t, tc.expectedName, name)
			assert.Equal(t, tc.expectedEmail, email)
		})
	}
}
