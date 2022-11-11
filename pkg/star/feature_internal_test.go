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

	"github.com/stretchr/testify/require"
)

// TODO add more tests, and rewrite during ruleslang rewrite.
func TestValidateRulesLang(t *testing.T) {
	testCases := []struct {
		rule    string
		context map[string]interface{}
		testErr bool
	}{
		{
			"h in [1] AND blah in [1, 2, 3]",
			map[string]interface{}{},
			true,
		},
		{
			// Test that not pr doesn't short circuit our error handling.
			"h not pr and blah in 7",
			map[string]interface{}{},
			true,
		},
	}
	for _, tc := range testCases {
		err := validateRulesLang(tc.rule)
		if tc.testErr {
			require.Error(t, err)
		} else {
			require.NoError(t, err)
		}
	}
}
