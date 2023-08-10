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

	"github.com/lekkodev/go-sdk/pkg/eval"
)

const starFmt = `result = feature(
    description="my config description",
    default=%s
)
`

// TODO: v1beta6 version to support this
// const starFmt = `export(
//     Config(
//         description="my config description",
//         default=%s
//     ),
// )
// `

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

// TODO: v1beta6 version to support this
// const protoFeatureTemplate = `{{- range $name, $alias := .Packages }}
// {{$alias}} = proto.package("{{$name}}")
// {{- end}}

// export(
//     Config(
//         description = "my config description",
//         default = {{.Message}}(
//             {{- range .Fields}}
//             {{. -}},
//             {{- end}}
//         ),
//     ),
// )
// `

type ProtoStarInputs struct {
	Message  string
	Packages map[string]string
	Fields   []string
}

// RenderExistingProtoTemplate will render the parsed Proto message descriptor into a Starlark feature model
func RenderExistingProtoTemplate(inputs ProtoStarInputs) ([]byte, error) {
	var buf bytes.Buffer
	templ := template.New("protobuf starlark")
	templ, err := templ.Parse(protoFeatureTemplate)
	if err != nil {
		return nil, err
	}
	err = templ.Execute(&buf, inputs)
	if err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func GetTemplate(fType eval.FeatureType) ([]byte, error) {
	switch fType {
	case eval.FeatureTypeBool:
		return []byte(fmt.Sprintf(starFmt, "False")), nil
	case eval.FeatureTypeInt:
		return []byte(fmt.Sprintf(starFmt, "1")), nil
	case eval.FeatureTypeFloat:
		return []byte(fmt.Sprintf(starFmt, "1.0")), nil
	case eval.FeatureTypeString:
		return []byte(fmt.Sprintf(starFmt, "''")), nil
	case eval.FeatureTypeJSON:
		return []byte(fmt.Sprintf(starFmt, "{}")), nil
	default:
		return nil, fmt.Errorf("templating is not supported for config type %s", fType)
	}
}
