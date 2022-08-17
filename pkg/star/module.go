package star

import (
	"os"

	"github.com/pkg/errors"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protodesc"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/reflect/protoregistry"
	"google.golang.org/protobuf/types/descriptorpb"
	"google.golang.org/protobuf/types/dynamicpb"
)

// Takes a path to a buf-generated image, which is compatible with
// protobuf's native FileDescriptorSet type. This allows us to build a
// registry of protobuf types, starting with the Global registry as our
// baseline, and adding any user-defined types residing in the given
// buf image.
func buildTypes(imageFileName string) (*protoregistry.Types, error) {
	bytes, err := os.ReadFile(imageFileName)
	if err != nil {
		return nil, errors.Wrap(err, "os.readfile image bin")
	}
	fds := &descriptorpb.FileDescriptorSet{}

	if err := proto.Unmarshal(bytes, fds); err != nil {
		return nil, errors.Wrap(err, "proto unmarshal")
	}
	files, err := protodesc.NewFiles(fds)

	if err != nil {
		return nil, errors.Wrap(err, "protodesc.NewFiles")
	}
	return filesToTypes(files), nil
}

func filesToTypes(files *protoregistry.Files) *protoregistry.Types {
	ret := protoregistry.GlobalTypes
	files.RangeFiles(func(fd protoreflect.FileDescriptor) bool {
		for i := 0; i < fd.Enums().Len(); i++ {
			ed := fd.Enums().Get(i)
			ret.RegisterEnum(dynamicpb.NewEnumType(ed))
		}
		for i := 0; i < fd.Messages().Len(); i++ {
			md := fd.Messages().Get(i)
			ret.RegisterMessage(dynamicpb.NewMessageType(md))
		}
		for i := 0; i < fd.Extensions().Len(); i++ {
			exd := fd.Extensions().Get(i)
			ret.RegisterExtension(dynamicpb.NewExtensionType(exd))
		}
		return true
	})
	return ret
}
