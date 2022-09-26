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
	"github.com/lekkodev/cli/pkg/star"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/reflect/protoregistry"
)

const testStar = `result = feature(
    description = "this is a simple feature",
    default = True,
    rules = [
        ("age == 10", False),
        ("city IN ['Rome', 'Milan']", False),
    ],
)
`

func testWalker() *walker {
	return &walker{
		filename:  "test.star",
		starBytes: []byte(testStar),
	}
}

func testFile(t *testing.T) *build.File {
	p := butils.GetParser(star.InputTypeAuto)
	file, err := p("test.star", []byte(testStar))
	require.NoError(t, err, "failed to parse test star file")
	return file
}

func TestWalkerBuild(t *testing.T) {
	b := testWalker()
	f, err := b.Build()
	require.NoError(t, err)
	require.NotNil(t, f)
	_, err = f.ToJSON(protoregistry.GlobalTypes)
	require.NoError(t, err)
}

func TestWalkerMutateNoop(t *testing.T) {
	b := testWalker()
	f, err := b.Build()
	require.NoError(t, err)
	require.NotNil(t, f)

	bytes, err := b.Mutate(f)
	require.NoError(t, err)
	assert.EqualValues(t, []byte(testStar), bytes)
}

func TestWalkerMutateDefault(t *testing.T) {
	b := testWalker()
	f, err := b.Build()
	require.NoError(t, err)
	require.NotNil(t, f)
	defaultVal, ok := f.Value.(bool)
	require.True(t, ok)
	require.True(t, defaultVal)

	f.Value = false
	bytes, err := b.Mutate(f)
	require.NoError(t, err)
	assert.NotEqualValues(t, []byte(testStar), bytes)
}

func TestWalkerMutateModifyRuleCondition(t *testing.T) {
	b := testWalker()
	f, err := b.Build()
	require.NoError(t, err)
	require.NotNil(t, f)

	f.Rules[0].Condition = "age == 12"

	bytes, err := b.Mutate(f)
	require.NoError(t, err)
	assert.Contains(t, string(bytes), "age == 12")
}

func TestWalkerMutateAddRule(t *testing.T) {
	b := testWalker()
	f, err := b.Build()
	require.NoError(t, err)
	require.NotNil(t, f)

	f.Rules = append(f.Rules, &feature.Rule{
		Condition: "age == 12",
		Value:     false,
	})

	bytes, err := b.Mutate(f)
	require.NoError(t, err)
	assert.Contains(t, string(bytes), "(\"age == 12\", False)")
}

func TestWalkerMutateRemoveRule(t *testing.T) {
	b := testWalker()
	f, err := b.Build()
	require.NoError(t, err)
	require.NotNil(t, f)

	f.Rules = f.Rules[1:]

	bytes, err := b.Mutate(f)
	require.NoError(t, err)
	assert.NotContains(t, string(bytes), "(\"age == 10\", False)")
}

func TestWalkerMutateDescription(t *testing.T) {
	b := testWalker()
	f, err := b.Build()
	require.NoError(t, err)
	require.NotNil(t, f)

	f.Description = "a NEW way to describe this feature."

	bytes, err := b.Mutate(f)
	require.NoError(t, err)
	assert.Contains(t, string(bytes), "a NEW way to describe this feature.")
}
