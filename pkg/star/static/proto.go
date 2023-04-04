package static

import (
	"fmt"

	"github.com/bazelbuild/buildtools/build"
	"github.com/stripe/skycfg"
	"github.com/stripe/skycfg/go/protomodule"
	"go.starlark.net/starlark"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/reflect/protoregistry"
)

func ProtoToStatic(packageStr string, msg proto.Message) build.Expr {
	msgDesc := msg.ProtoReflect()
	constructorName := fmt.Sprintf("%s.%s", packageStr, msgDesc.Descriptor().Name())
	res := &build.CallExpr{X: &build.Ident{Name: constructorName}}
	// todo defaults
	msgDesc.Range(func(fieldDesc protoreflect.FieldDescriptor, val protoreflect.Value) bool {
		res.List = append(res.List, &build.AssignExpr{LHS: &build.Ident{Name: string(fieldDesc.Name())}, Op: "=", RHS: &build.Ident{Name: "True"}})
		return true
	})
	return res
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
