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
func ProtoToStatic(importStmt *featurev1beta1.ImportStatement, msg proto.Message) (*build.CallExpr, error) {
	msgDesc := msg.ProtoReflect()
	res := &build.CallExpr{X: &build.DotExpr{X: &build.Ident{Name: importStmt.GetLhs().Token}, Name: string(msgDesc.Descriptor().Name())}}
	var retErr error
	// Note: Default values are not set in the proto spec.
	// Thus, the following Range doesn't iterate over them.
	// This can lead to behavior where, after a round trip
	// of static parsing,
	// 		pb.BoolValue(value = False)
	// will be overwritten as
	// 		pb.BoolValue()
	msgDesc.Range(func(fieldDesc protoreflect.FieldDescriptor, val protoreflect.Value) bool {
		starExpr, err := ReflectValueToExpr(&val)
		if err != nil {
			retErr = err
			return false
		}
		res.List = append(res.List, &build.AssignExpr{LHS: &build.Ident{Name: string(fieldDesc.Name())}, Op: "=", RHS: starExpr})
		return true
	})
	return res, retErr
}

func ReflectValueToExpr(val *protoreflect.Value) (build.Expr, error) {
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
	// We need to implement this all.
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
	default:
		return nil, errors.Wrapf(ErrUnsupportedStaticParsing, "static mutate proto val %v", val)
	}
}

// Returns (nil, err) if the message is not protobuf.
func CallExprToProto(ce *build.CallExpr, f *featurev1beta1.StaticFeature, registry *protoregistry.Types) (proto.Message, error) {
	thread := &starlark.Thread{
		Name: "compile",
	}
	protoModule := protomodule.NewModule(registry)
	globals, err := starlark.ExecFile(thread, "", genMiniStar(f.Imports, ce), starlark.StringDict{
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

func genMiniStar(imps []*featurev1beta1.ImportStatement, ce *build.CallExpr) (ret []byte) {
	var imports []string
	for _, imp := range imps {
		var args []string
		for _, arg := range imp.Rhs.Args {
			args = append(args, fmt.Sprintf("\"%s\"", arg))
		}
		rhs := fmt.Sprintf("%s.%s(%s)", imp.Rhs.Dot.X, imp.Rhs.Dot.Name, strings.Join(args, ","))
		imports = append(imports, fmt.Sprintf("%s %s %s", imp.Lhs.Token, imp.Operator, rhs))
	}
	return []byte(fmt.Sprintf("%s\nres = %s\n", strings.Join(imports, "\n"), build.FormatString(ce)))
}
