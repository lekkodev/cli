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

// Technically this file is specific to go, but the functions themselves
// are useful in general. We should generalize this to work in all langs
// but have the logic shared somehow. For now we are hacking shit across files.
package gen

import (
	"fmt"
	"strings"
)

type ProtoImport struct {
	PackageAlias string
	ImportPath   string
	Type         string
}

// This function handles both the google.protobuf.Any.TypeURL variable
// which has the format of `types.googleapis.com/fully.qualified.v1beta1.Proto`
// and purely `fully.qualified.v1beta1.Proto`
//
// return nil if typeURL is empty. Panics on any problems like the rest of the file.
func UnpackProtoType(moduleRoot string, typeURL string) *ProtoImport {
	if typeURL == "" {
		return nil
	}
	anyURLSplit := strings.Split(typeURL, "/")
	fqType := anyURLSplit[0]
	if len(anyURLSplit) > 1 {
		if anyURLSplit[0] != "type.googleapis.com" {
			panic("invalid any type url: " + typeURL)
		}
		fqType = anyURLSplit[1]
	}

	// turn default.config.v1beta1.DBConfig into:
	// moduleRoot/internal/lekko/proto/default/config/v1beta1
	typeParts := strings.Split(fqType, ".")
	// TODO: un-hardcode import path
	importPath := strings.Join(append([]string{moduleRoot + "/internal/lekko/proto"}, typeParts[:len(typeParts)-1]...), "/")
	// e.g. configv1beta1
	// TODO: shouldn't we be doing namespace + configv1beta1? what if there are multiple namespaces?
	prefix := fmt.Sprintf(`%s%s`, typeParts[len(typeParts)-3], typeParts[len(typeParts)-2])

	// TODO do google.protobuf.X
	switch fqType {
	case "google.protobuf.Duration":
		importPath = "google.golang.org/protobuf/types/known/durationpb"
		prefix = "durationpb"
	default:
	}
	return &ProtoImport{PackageAlias: prefix, ImportPath: importPath, Type: typeParts[len(typeParts)-1]}
}
