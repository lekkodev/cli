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

package oauth

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestMaskToken(t *testing.T) {
	assert.Equal(t, "lekko_oauth_abcd**********", maskToken("lekko_oauth_abcdefghijklmnopqrstuvw=", "lekko_oauth_"))
	assert.Equal(t, "lekko_1q2w**********", maskToken("lekko_1q2w3e4r-5t6y-7u8i-9old-1q2w3e4r5t6y_6y7u8i9o-1q2w-3e4r-4r5t-1q2w3e4r5t6y", "lekko_"))
	assert.Equal(t, "ghu_1Q2w**********", maskToken("ghu_1Q2w3E4r5T6y7U8i9O1q2S3e4R5t6Y7u8I9o", "ghu_"))
	assert.Equal(t, "lekko_oauth**********", maskToken("lekko_oauth", "lekko_oauth"))
	assert.Equal(t, "**********", maskToken("lekko_oauth", "lekko_oauth_"))
	assert.Equal(t, "lekko_oauth_1**********", maskToken("lekko_oauth_1", "lekko_oauth_"))
}
