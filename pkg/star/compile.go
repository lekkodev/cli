package star

import (
	"fmt"
	"io/ioutil"
	"log"
	"os"

	protov1 "github.com/golang/protobuf/proto"
	"github.com/jhump/protoreflect/desc/protoparse"
	"github.com/jhump/protoreflect/dynamic"
	lekkov1beta1 "github.com/lekkodev/cli/pkg/gen/proto/go/lekko/feature/v1beta1"
	"github.com/lekkodev/cli/pkg/star/starproto"
	"github.com/pkg/errors"
	"go.starlark.net/starlark"
	"google.golang.org/protobuf/types/known/anypb"
)

func Compile(name, starfile string) (*lekkov1beta1.Feature, error) {
	thread := &starlark.Thread{
		Name: "compile thread",
		Load: load,
	}
	reader, err := os.Open(starfile)
	if err != nil {
		return nil, errors.Wrap(err, "open starfile")
	}
	defer reader.Close()
	moduleSource, err := ioutil.ReadAll(reader)
	if err != nil {
		return nil, errors.Wrap(err, "read starfile")
	}
	sd, err := starlark.ExecFile(thread, starfile, moduleSource, nil)
	if err != nil {
		return nil, errors.Wrap(err, "get globals")
	}

	defaultProto, err := getDefault(sd)
	if err != nil {
		return nil, errors.Wrap(err, "get default")
	}

	rules, err := getRules(sd)
	if err != nil {
		return nil, errors.Wrap(err, "get rules")
	}

	log.Printf("Got final values: \n\tdefault: %v\n\trules: %v\n", defaultProto, rules)

	tree, err := constructTree(defaultProto, rules)
	if err != nil {
		return nil, errors.Wrap(err, "construct tree")
	}

	return &lekkov1beta1.Feature{
		Key:         name,
		Description: "",
		Tree:        tree,
	}, nil
}

func load(thread *starlark.Thread, moduleName string) (starlark.StringDict, error) {
	// parse the proto into descriptors
	p := protoparse.Parser{}
	results, err := p.ParseFiles(moduleName)
	if err != nil {
		return nil, errors.Wrap(err, "parse files")
	}
	globals := starlark.StringDict{}
	for _, message := range results[0].GetMessageTypes() {
		globals[message.GetName()] = starproto.NewMessageType(message)
	}
	for _, enum := range results[0].GetEnumTypes() {
		globals[enum.GetName()] = starproto.NewEnumType(enum)
	}
	return globals, nil
}

func getDefault(globals starlark.StringDict) (*dynamic.Message, error) {
	sd, ok := globals["default"]
	if !ok {
		return nil, fmt.Errorf("no `default` function found")
	}
	def, ok := sd.(starlark.Callable)
	if !ok {
		return nil, fmt.Errorf("`default` must be a function (got a %s)", sd.Type())
	}

	thread := &starlark.Thread{}

	defaultVal, err := starlark.Call(thread, def, nil, nil)
	if err != nil {
		return nil, errors.Wrap(err, "call default func")
	}
	message, ok := starproto.ToProtoMessage(defaultVal)
	if !ok {
		return nil, fmt.Errorf("default returned something that is not proto, got %s", defaultVal.Type())
	}
	return message, nil
}

type Rule struct {
	rule  string
	value *dynamic.Message
}

func getRules(globals starlark.StringDict) ([]Rule, error) {
	sd, ok := globals["rules"]
	if !ok {
		return nil, fmt.Errorf("no `rules` function found ")
	}
	rules, ok := sd.(starlark.Callable)
	if !ok {
		return nil, fmt.Errorf("`rules` must be a function (got a %s)", sd.Type())
	}

	thread := &starlark.Thread{}

	rulesVal, err := starlark.Call(thread, rules, nil, nil)
	if err != nil {
		return nil, errors.Wrap(err, "call rules func")
	}

	seq, ok := rulesVal.(starlark.Sequence)
	if !ok {
		return nil, fmt.Errorf("rules: did not get back a starlark sequence: %v", rulesVal)
	}
	var rulesArr []Rule
	it := seq.Iterate()
	defer it.Done()
	var val starlark.Value
	for it.Next(&val) {
		if val == starlark.None {
			return nil, fmt.Errorf("type error: [%v] %v", val.Type(), val.String())
		}
		tuple, ok := val.(starlark.Tuple)
		if !ok {
			return nil, fmt.Errorf("type error: expecting tuple, got %v", val.Type())
		}
		if tuple.Len() != 2 {
			return nil, fmt.Errorf("expecting tuple of length 2, got length %d: %v", tuple.Len(), tuple)
		}
		conditionStr, ok := tuple.Index(0).(starlark.String)
		if !ok {
			return nil, fmt.Errorf("type error: expecting string, got %v: %v", tuple.Index(0).Type(), tuple.Index(0))
		}
		// TODO: parse into ruleslang
		if conditionStr.GoString() == "" {
			return nil, fmt.Errorf("expecting valid ruleslang, got %s", conditionStr.GoString())
		}
		message, ok := starproto.ToProtoMessage(tuple.Index(1))
		if !ok {
			return nil, fmt.Errorf("condition val returned something that is not proto, got %s", tuple.Index(1).Type())
		}
		rulesArr = append(rulesArr, Rule{
			rule:  conditionStr.GoString(),
			value: message,
		})
	}

	return rulesArr, nil
}

func constructTree(defaultValue *dynamic.Message, rules []Rule) (*lekkov1beta1.Tree, error) {
	// var defaultProto protoiface.MessageV1

	// if err := defaultValue.ConvertTo(defaultProto); err != nil {
	// 	return nil, errors.Wrap(err, "default value convert to proto")
	// }
	fqn := defaultValue.GetMessageDescriptor().GetFullyQualifiedName()
	reflectMessage := protov1.MessageReflect(defaultValue)
	// proto.Marshal(reflectMessage.Interface())

	// defaultProtoV2 := protov1.MessageV2(defaultValue)
	any, err := anypb.New(reflectMessage.Interface())
	if err != nil {
		return nil, errors.Wrap(err, "default to any")
	}
	any.TypeUrl = fmt.Sprintf("type.googleapis.com/%s", fqn)
	tree := &lekkov1beta1.Tree{
		Default: any,
	}
	for _, rule := range rules {
		// var valueProto protoiface.MessageV1
		// if err := rule.value.ConvertTo(valueProto); err != nil {
		// 	return nil, errors.Wrap(err, "rule value convert to proto")
		// }
		fqn := defaultValue.GetMessageDescriptor().GetFullyQualifiedName()
		valueProtoV2 := protov1.MessageReflect(rule.value)
		any, err := anypb.New(valueProtoV2.Interface())
		if err != nil {
			return nil, errors.Wrap(err, "rule value to any")
		}
		any.TypeUrl = fmt.Sprintf("type.googleapis.com/%s", fqn)
		tree.Constraints = append(tree.Constraints, &lekkov1beta1.Constraint{
			Rule:  rule.rule,
			Value: any,
		})
	}
	return tree, nil
}
