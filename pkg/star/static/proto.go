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
	"fmt"
	"sort"
	"strings"

	featurev1beta1 "buf.build/gen/go/lekkodev/cli/protocolbuffers/go/lekko/feature/v1beta1"
	"github.com/bazelbuild/buildtools/build"
	"github.com/pkg/errors"
	"github.com/stripe/skycfg"
	"github.com/stripe/skycfg/go/protomodule"
	"go.starlark.net/starlark"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/reflect/protoregistry"
)

// Given a proto message and the import statement that the type of the message is provided by,
// constructs a starlark expression defining the proto message.
func ProtoToStatic(imports []*featurev1beta1.ImportStatement, msg protoreflect.Message, meta *featurev1beta1.StarMeta) (*build.CallExpr, error) {
	imp, err := findImport(imports, msg.Descriptor().FullName())
	if err != nil {
		return nil, err
	}
	suffix := strings.TrimPrefix(strings.TrimPrefix(string(msg.Descriptor().FullName()), imp.Rhs.GetArgs()[0]), ".")
	res := &build.CallExpr{
		X: &build.DotExpr{
			X:    &build.Ident{Name: imp.GetLhs().Token},
			Name: suffix,
		},
		ForceMultiLine: meta.GetMultiline(),
	}
	var retErr error
	// Note: Default values are not set in the proto spec.
	// Thus, the following Range doesn't iterate over them.
	// This can lead to behavior where, after a round trip
	// of static parsing,
	// 		pb.BoolValue(value = False)
	// will be overwritten as
	// 		pb.BoolValue()
	msg.Range(func(fieldDesc protoreflect.FieldDescriptor, val protoreflect.Value) bool {
		starExpr, err := ReflectValueToExpr(imports, fieldDesc, &val, meta)
		if err != nil {
			retErr = err
			return false
		}
		res.List = append(res.List, &build.AssignExpr{LHS: &build.Ident{Name: string(fieldDesc.Name())}, Op: "=", RHS: starExpr})
		return true
	})
	// Since Range operates in undefined order, we need to introduce order to the output
	// so that the round-trip is stable.
	sort.Slice(res.List, func(i, j int) bool {
		return build.FormatString(res.List[i]) < build.FormatString(res.List[j])
	})
	return res, retErr
}

func ReflectValueToExpr(imports []*featurev1beta1.ImportStatement, fieldDesc protoreflect.FieldDescriptor, val *protoreflect.Value, meta *featurev1beta1.StarMeta) (build.Expr, error) {
	// There is a strict enum definition here:
	/*
		    ╔════════════╤═════════════════════════════════════╗
			║ Go type    │ Protobuf kind                       ║
			╠════════════╪═════════════════════════════════════╣
			║ bool       │ BoolKind                            ║
			║ int32      │ Int32Kind, Sint32Kind, Sfixed32Kind ║
			║ int64      │ Int64Kind, Sint64Kind, Sfixed64Kind ║
			║ uint32     │ Uint32Kind, Fixed32Kind             ║
			║ uint64     │ Uint64Kind, Fixed64Kind             ║
			║ float32    │ FloatKind                           ║
			║ float64    │ DoubleKind                          ║
			║ string     │ StringKind                          ║
			║ []byte     │ BytesKind                           ║
			║ EnumNumber │ EnumKind                            ║
			║ Message    │ MessageKind, GroupKind              ║
			╚════════════╧═════════════════════════════════════╝
	*/
	goValInterface := val.Interface()
	switch goVal := goValInterface.(type) {
	case bool:
		if goVal {
			return &build.Ident{Name: "True"}, nil
		}
		return &build.Ident{Name: "False"}, nil
	case string:
		return &build.StringExpr{Value: goVal}, nil
	case int32:
		return &build.LiteralExpr{
			Token: starlark.MakeInt(int(goVal)).String(),
		}, nil
	case int64:
		return &build.LiteralExpr{
			Token: starlark.MakeInt64(goVal).String(),
		}, nil
	case uint32:
		return &build.LiteralExpr{
			Token: starlark.MakeUint(uint(goVal)).String(),
		}, nil
	case uint64:
		return &build.LiteralExpr{
			Token: starlark.MakeUint64(goVal).String(),
		}, nil
	case float32:
		return &build.LiteralExpr{
			Token: fmt.Sprintf("%v", goVal),
		}, nil
	case float64:
		return &build.LiteralExpr{
			Token: fmt.Sprintf("%v", goVal),
		}, nil
	case []byte:
		return &build.StringExpr{Value: string(goVal)}, nil
	case protoreflect.EnumNumber:
		enumDesc := fieldDesc.Enum()
		imp, err := findImport(imports, enumDesc.FullName())
		if err != nil {
			return nil, err
		}
		suffix := strings.TrimPrefix(strings.TrimPrefix(string(enumDesc.FullName()), imp.Rhs.GetArgs()[0]), ".")
		return &build.DotExpr{
			X:    &build.Ident{Name: imp.Lhs.GetToken()},
			Name: fmt.Sprintf("%s.%s", suffix, enumDesc.Values().ByNumber(goVal).Name()),
		}, nil
	case protoreflect.Message:
		return ProtoToStatic(imports, goVal, meta) // recurse
	default:
		return nil, errors.Wrapf(ErrUnsupportedStaticParsing, "static mutate proto val %v", val)
	}
}

// Returns (nil, err) if the message is not protobuf.
func ExprToProto(expr build.Expr, f *featurev1beta1.StaticFeature, registry *protoregistry.Types) (proto.Message, error) {
	thread := &starlark.Thread{Name: "parse_proto"}
	protoModule := protomodule.NewModule(registry)
	globals, err := starlark.ExecFile(thread, "", genMiniStar(f.Imports, expr), starlark.StringDict{
		"proto": protoModule,
	})
	if err != nil {
		return nil, err
	}
	proto, ok := skycfg.AsProtoMessage(globals["res"])
	if !ok {
		return nil, fmt.Errorf("no proto message found %T %v", globals["res"], globals["res"])
	}
	return proto, nil
}

func genMiniStar(imps []*featurev1beta1.ImportStatement, expr build.Expr) (ret []byte) {
	var imports []string
	for _, imp := range imps {
		var args []string
		for _, arg := range imp.Rhs.Args {
			args = append(args, fmt.Sprintf("\"%s\"", arg))
		}
		rhs := fmt.Sprintf("%s.%s(%s)", imp.Rhs.Dot.X, imp.Rhs.Dot.Name, strings.Join(args, ","))
		imports = append(imports, fmt.Sprintf("%s %s %s", imp.Lhs.Token, imp.Operator, rhs))
	}
	return []byte(fmt.Sprintf("%s\nres = %s\n", strings.Join(imports, "\n"), build.FormatString(expr)))
}

// Given the list of proto imports that were defined by the starlark,
// find the specific import that declared the protobuf package that
// contains the schema for the provided message. Note: the message may be
// dynamic.
func findImport(imports []*featurev1beta1.ImportStatement, fullName protoreflect.FullName) (*featurev1beta1.ImportStatement, error) {
	for _, imp := range imports {
		if len(imp.Rhs.Args) == 0 {
			return nil, errors.Errorf("import statement found with no args: %v", imp.Rhs.String())
		}
		packagePrefix := imp.Rhs.Args[0]
		if strings.HasPrefix(string(fullName), packagePrefix) {
			return imp, nil
		}
	}
	return nil, errors.New("no proto import statements found")
}
