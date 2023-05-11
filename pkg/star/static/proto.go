package static

import (
	"fmt"

	"github.com/bazelbuild/buildtools/build"
	"github.com/pkg/errors"
	"github.com/stripe/skycfg"
	"github.com/stripe/skycfg/go/protomodule"
	"go.starlark.net/starlark"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/reflect/protoregistry"
)

func ProtoToStatic(packageStr string, msg proto.Message) (build.Expr, error) {
	msgDesc := msg.ProtoReflect()
	constructorName := fmt.Sprintf("%s.%s", packageStr, msgDesc.Descriptor().Name())
	res := &build.CallExpr{X: &build.Ident{Name: constructorName}}
	var retErr error
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
	default:
		return nil, errors.Wrapf(ErrUnsupportedStaticParsing, "static mutate proto val %v", val)
	}
}

// Returns (nil, err) if the message is not protobuf.
func CallExprToProto(ce *build.CallExpr) (proto.Message, error) {
	thread := &starlark.Thread{
		Name: "compile",
	}
	// TODO: pipe in user defined protos
	protoModule := protomodule.NewModule(protoregistry.GlobalTypes)
	// TODO figure out what the user's imported proto modules are more elegantly
	globals, err := starlark.ExecFile(thread, "", fmt.Sprintf("pb = proto.package(\"google.protobuf\")\nres =%s", build.FormatString(ce)), starlark.StringDict{
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
