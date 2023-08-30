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
	"bytes"
	"fmt"
	"text/template"

	"github.com/lekkodev/cli/pkg/feature"
	"github.com/lekkodev/go-sdk/pkg/eval"
)

const featureTemplate = `result = feature(
    description = "my config description",
    default = %s,
)
`

const configTemplate = `export(
    Config(
        description = "my config description",
        default = %s,
    ),
)
`

const protoFeatureTemplate = `{{- range $name, $alias := .Packages }}
{{$alias}} = proto.package("{{$name}}")
{{- end}}

result = feature(
    description = "my config description",
    default = {{.Message}}(
        {{- range .Fields}}
        {{. -}},
        {{- end}}
    ),
)
`

const protoConfigTemplate = `{{- range $name, $alias := .Packages }}
{{$alias}} = proto.package("{{$name}}")
{{- end}}

export(
    Config(
        description = "my config description",
        default = {{.Message}}(
            {{- range .Fields}}
            {{. -}},
            {{- end}}
        ),
    ),
)
`

type ProtoStarInputs struct {
	Message  string
	Packages map[string]string
	Fields   []string
}

// RenderExistingProtoTemplate will render the parsed Proto message descriptor into a Starlark feature model
func RenderExistingProtoTemplate(inputs ProtoStarInputs, nv feature.NamespaceVersion) ([]byte, error) {
	var templateBody string
	if nv >= feature.NamespaceVersionV1Beta6 {
		templateBody = protoConfigTemplate
	} else {
		templateBody = protoFeatureTemplate
	}
	var buf bytes.Buffer
	templ := template.New("protobuf starlark")
	templ, err := templ.Parse(templateBody)
	if err != nil {
		return nil, err
	}
	err = templ.Execute(&buf, inputs)
	if err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func GetTemplate(fType eval.ConfigType, nv feature.NamespaceVersion) ([]byte, error) {
	var templateBody string
	if nv >= feature.NamespaceVersionV1Beta6 {
		templateBody = configTemplate
	} else {
		templateBody = featureTemplate
	}
	switch fType {
	case eval.ConfigTypeBool:
		return []byte(fmt.Sprintf(templateBody, "False")), nil
	case eval.ConfigTypeInt:
		return []byte(fmt.Sprintf(templateBody, "1")), nil
	case eval.ConfigTypeFloat:
		return []byte(fmt.Sprintf(templateBody, "1.0")), nil
	case eval.ConfigTypeString:
		return []byte(fmt.Sprintf(templateBody, "''")), nil
	case eval.ConfigTypeJSON:
		return []byte(fmt.Sprintf(templateBody, "{}")), nil
	default:
		return nil, fmt.Errorf("templating is not supported for config type %s", fType)
	}
}
