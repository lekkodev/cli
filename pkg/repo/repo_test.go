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
