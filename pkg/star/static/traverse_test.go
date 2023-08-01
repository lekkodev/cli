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
	"github.com/lekkodev/go-sdk/pkg/eval"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func testFile(t *testing.T, useExport bool) *build.File {
	_, starBytes := testStar(t, eval.FeatureTypeBool, useExport)
	return toFile(t, starBytes)
}

func toFile(t *testing.T, starBytes []byte) *build.File {
	p := butils.GetParser(InputTypeAuto)
	file, err := p("test.star", starBytes)
	require.NoError(t, err, "failed to parse test star file")
	return file
}

func strToFile(t *testing.T, contents string) *build.File {
	return toFile(t, []byte(contents))
}

func TestTraverse_noop(t *testing.T) {
	for _, useExport := range []bool{true, false} {
		_, starBytes := testStar(t, eval.FeatureTypeBool, useExport)
		f := testFile(t, useExport)
		tvs := newTraverser(f)
		err := tvs.traverse()
		require.NoError(t, err)

		// after noop traversal, the round trip bytes should be the same.
		assert.EqualValues(t, string(starBytes), string(tvs.format()))
	}
}

func TestTraverse_garbage(t *testing.T) {
	traverser := newTraverser(strToFile(t, "foo"))
	err := traverser.traverse()
	require.Error(t, err)
}
func TestTraverse_noExport(t *testing.T) {
	traverser := newTraverser(strToFile(t, "Config()"))
	err := traverser.traverse()
	require.Error(t, err)
}
func TestTraverse_noConfig(t *testing.T) {
	traverser := newTraverser(strToFile(t, "export()"))
	err := traverser.traverse()
	require.Error(t, err)
}
func TestTraverse_emptyConfig(t *testing.T) {
	traverser := newTraverser(strToFile(t, `export(Config())`))
	err := traverser.traverse()
	require.Error(t, err)
}
func TestTraverse_noDescription(t *testing.T) {
	starBytes := []byte(`export(Config(default=1))`)
	f := toFile(t, starBytes)
	traverser := newTraverser(f)
	err := traverser.traverse()
	require.Error(t, err)
}
func TestTraverse_noDefault(t *testing.T) {
	starBytes := []byte(`export(Config(description="test"))`)
	f := toFile(t, starBytes)
	traverser := newTraverser(f)
	err := traverser.traverse()
	require.Error(t, err)
}
func TestTraverse_valid(t *testing.T) {
	starBytes := []byte(`export(Config(description="test",default=1))`)
	f := toFile(t, starBytes)
	traverser := newTraverser(f)
	err := traverser.traverse()
	require.NoError(t, err)
}
