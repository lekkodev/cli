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
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestValidString(t *testing.T) {
	// valids
	assert.True(t, isValidName("anon.enabled"), ". is allowed")
	assert.True(t, isValidName("anon-enabled"), "- is allowed")
	assert.True(t, isValidName("anon_enabled"), "_ is allowed")
	assert.True(t, isValidName("s"), "single letter allowed")
	assert.True(t, isValidName("1"), "single digit allowed")
	assert.True(t, isValidName("anon123enabled"), "numbers are allowed")
	// invalids
	assert.True(t, !isValidName("anon enabled"), "no spaces allowed")
	assert.True(t, !isValidName("Anon.enabled"), "no uppercase allowed")
	assert.True(t, !isValidName(""), "cannot be empty")
	assert.True(t, !isValidName("anon@enabled"), "cannot have @")
	assert.True(t, !isValidName(".sblah"), "cannot start with special character")
	assert.True(t, !isValidName("blah-"), "cannot end with special character")
}
