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

package static

import (
	"testing"

	"github.com/bazelbuild/buildtools/build"
	butils "github.com/bazelbuild/buildtools/buildifier/utils"
	"github.com/lekkodev/cli/pkg/feature"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func testFile(t *testing.T) *build.File {
	_, starBytes := testStar(t, feature.FeatureTypeBool)
	p := butils.GetParser(InputTypeAuto)
	file, err := p("test.star", starBytes)
	require.NoError(t, err, "failed to parse test star file")
	return file
}

func TestTraverseNoop(t *testing.T) {
	_, starBytes := testStar(t, feature.FeatureTypeBool)
	f := testFile(t)
	tvs := newTraverser(f)
	err := tvs.traverse()
	require.NoError(t, err)

	// after noop traversal, the round trip bytes should be the same.
	assert.EqualValues(t, string(starBytes), string(tvs.format()))
}
