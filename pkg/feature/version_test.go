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

package feature

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestPriorVersionsSupported(t *testing.T) {
	supported := SupportedNamespaceVersions()
	assert.NotContains(t, supported, NamespaceVersionV1Beta1)
	assert.NotContains(t, supported, NamespaceVersionV1Beta2)
	assert.Contains(t, supported, NamespaceVersionV1Beta3)
	assert.Contains(t, supported, NamespaceVersionV1Beta4)
	assert.Contains(t, supported, NamespaceVersionV1Beta5)
	assert.Contains(t, supported, NamespaceVersionV1Beta6)
}

func TestVersionOrder(t *testing.T) {
	assert.True(t, NamespaceVersionV1Beta1.Before(NamespaceVersionV1Beta2))
	assert.True(t, NamespaceVersionV1Beta1.Before(NamespaceVersionV1Beta3))
	assert.True(t, NamespaceVersionV1Beta1.Before(NamespaceVersionV1Beta4))
	assert.True(t, !NamespaceVersionV1Beta1.Before(NamespaceVersionV1Beta1))
	assert.True(t, !NamespaceVersionV1Beta4.Before(NamespaceVersionV1Beta1))
}
