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
	"strconv"
	"strings"
	"text/template"

	"github.com/iancoleman/strcase"
	"github.com/lekkodev/cli/pkg/feature"
	"github.com/lekkodev/go-sdk/pkg/eval"
	"github.com/pkg/errors"
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

func GetTemplate(fType eval.ConfigType, nv feature.NamespaceVersion, defaultValue interface{}) ([]byte, error) {
	var templateBody string
	if nv >= feature.NamespaceVersionV1Beta6 {
		templateBody = configTemplate
	} else {
		templateBody = featureTemplate
	}
	var valStr string
	var err error
	if defaultValue == nil {
		valStr, err = getDefaultValStr(fType)
		if err != nil {
			return nil, err
		}
	} else {
		valStr, err = ValToStarStr(defaultValue)
		if err != nil {
			return nil, err
		}
	}
	return []byte(fmt.Sprintf(templateBody, valStr)), nil
}

// Converts arbitrary value to Starlark equivalent representation
func ValToStarStr(value interface{}) (string, error) {
	switch v := value.(type) {
	case bool:
		return strcase.ToCamel((strconv.FormatBool(v))), nil
	case int64:
		return strconv.FormatInt(v, 10), nil
	case float64:
		formatted := strconv.FormatFloat(v, 'f', -1, 64)
		if !strings.Contains(formatted, ".") {
			formatted += ".0" // For floats we need decimal points
		}
		return formatted, nil
	case string:
		return fmt.Sprintf("'%s'", v), nil
	default:
		return "", errors.Errorf("unsupported type %T", value)
	}
}

func getDefaultValStr(fType eval.ConfigType) (string, error) {
	switch fType {
	case eval.ConfigTypeBool:
		return "False", nil
	case eval.ConfigTypeInt:
		return "1", nil
	case eval.ConfigTypeFloat:
		return "1.0", nil
	case eval.ConfigTypeString:
		return "''", nil
	case eval.ConfigTypeJSON:
		return "{}", nil
	default:
		return "", errors.Errorf("unsupported config type %s", fType)
	}
}
