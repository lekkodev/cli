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
	"os"
	"testing"

	"github.com/lekkodev/cli/pkg/feature"
	"github.com/lekkodev/go-sdk/pkg/eval"
	"github.com/stretchr/testify/require"
)

func TestGetTemplate(t *testing.T) {
	template, err := GetTemplate(eval.ConfigTypeBool, feature.NamespaceVersionV1Beta5, nil)

	require.NoError(t, err)
	goldenFile, err := os.ReadFile("./testdata/test_get_template.star")
	require.NoError(t, err)
	require.Equal(t, string(goldenFile), string(template))
}

func TestGetTemplateV1Beta6(t *testing.T) {
	template, err := GetTemplate(eval.ConfigTypeBool, feature.NamespaceVersionV1Beta6, nil)

	require.NoError(t, err)
	goldenFile, err := os.ReadFile("./testdata/test_get_template_config.star")
	require.NoError(t, err)
	require.Equal(t, string(goldenFile), string(template))
}

func TestRenderExistingProtoTemplate(t *testing.T) {
	template, err := RenderExistingProtoTemplate(ProtoStarInputs{
		Message: "internal_config_v1beta1.ProductMetadata",
		Packages: map[string]string{
			"google.protobuf":         "google_protobuf",
			"internal.config.v1beta1": "internal_config_v1beta1",
		},
		Fields: []string{
			`state = internal_config_v1beta1.ProductState.PRODUCT_STATE_UNSPECIFIED`,
			`description = ""`,
			`time = google_protobuf.Timestamp()`,
			`friend = internal_config_v1beta1.Friend()`,
			`build = internal_config_v1beta1.Build()`,
			`sell = internal_config_v1beta1.Sell()`,
		},
	}, feature.NamespaceVersionV1Beta5)

	require.NoError(t, err)
	goldenFile, err := os.ReadFile("./testdata/test_render_existing_proto_template.star")
	require.NoError(t, err)
	require.Equal(t, string(goldenFile), string(template))
}

func TestRenderExistingProtoTemplateV1Beta6(t *testing.T) {
	template, err := RenderExistingProtoTemplate(ProtoStarInputs{
		Message: "internal_config_v1beta1.ProductMetadata",
		Packages: map[string]string{
			"google.protobuf":         "google_protobuf",
			"internal.config.v1beta1": "internal_config_v1beta1",
		},
		Fields: []string{
			`state = internal_config_v1beta1.ProductState.PRODUCT_STATE_UNSPECIFIED`,
			`description = ""`,
			`time = google_protobuf.Timestamp()`,
			`friend = internal_config_v1beta1.Friend()`,
			`build = internal_config_v1beta1.Build()`,
			`sell = internal_config_v1beta1.Sell()`,
		},
	}, feature.NamespaceVersionV1Beta6)

	require.NoError(t, err)
	goldenFile, err := os.ReadFile("./testdata/test_render_existing_proto_template_config.star")
	require.NoError(t, err)
	require.Equal(t, string(goldenFile), string(template))
}
