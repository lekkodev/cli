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

package star

import (
	"testing"

	"github.com/bazelbuild/buildtools/build"
	butils "github.com/bazelbuild/buildtools/buildifier/utils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const testStar = `
result = feature(
    description = "this is a simple feature",
    default = True,
)
`

func testFile(t *testing.T) *build.File {
	p := butils.GetParser(inputTypeAuto)
	file, err := p("test.star", []byte(testStar))
	require.NoError(t, err, "failed to parse test star file")
	return file
}

func TestStaticBuilder(t *testing.T) {
	b := newStaticBuilder(testFile(t))
	f, err := b.build()
	assert.NoError(t, err)
	assert.NotNil(t, f)
}
