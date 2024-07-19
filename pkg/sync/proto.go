package sync

import (
	"fmt"
	"strings"

	"github.com/pkg/errors"
	"google.golang.org/protobuf/reflect/protodesc"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/reflect/protoregistry"
	"google.golang.org/protobuf/types/descriptorpb"
	"google.golang.org/protobuf/types/dynamicpb"
)

func FileRegistryToTypeRegistry(fr *protoregistry.Files) (*protoregistry.Types, error) {
	tr := &protoregistry.Types{}
	var retErr error
	fr.RangeFiles(func(fd protoreflect.FileDescriptor) bool {
		for i := range fd.Enums().Len() {
			if err := tr.RegisterEnum(dynamicpb.NewEnumType(fd.Enums().Get(i))); err != nil {
				retErr = errors.Wrapf(err, "register enum %s", fd.Enums().Get(i).FullName())
				return false
			}
		}
		for i := range fd.Messages().Len() {
			if err := tr.RegisterMessage(dynamicpb.NewMessageType(fd.Messages().Get(i))); err != nil {
				retErr = errors.Wrapf(err, "register message %s", fd.Messages().Get(i).FullName())
				return false
			}
		}
		for i := range fd.Extensions().Len() {
			if err := tr.RegisterExtension(dynamicpb.NewExtensionType(fd.Extensions().Get(i))); err != nil {
				retErr = errors.Wrapf(err, "register extension %s", fd.Extensions().Get(i).FullName())
				return false
			}
		}
		return true
	})
	if retErr != nil {
		return nil, retErr
	}
	return tr, nil
}

func FileRegistryToFileDescriptorSet(registry *protoregistry.Files) *descriptorpb.FileDescriptorSet {
	fds := &descriptorpb.FileDescriptorSet{}
	registry.RangeFiles(func(fd protoreflect.FileDescriptor) bool {
		fds.File = append(fds.File, protodesc.ToFileDescriptorProto(fd))
		return true
	})
	return fds
}

// TODO: Canonical proto formatter that doesn't rely on `buf format`, which we should use for all languages

func FileDescriptorToProtoString(fd protoreflect.FileDescriptor) (string, error) {
	var sb strings.Builder
	// Preamble
	sb.WriteString("syntax = \"proto3\";\n\n")
	sb.WriteString(fmt.Sprintf("package %s;\n\n", fd.Package()))
	// Imports
	for i := range fd.Imports().Len() {
		sb.WriteString(fmt.Sprintf("import \"%s\";\n\n", fd.Imports().Get(i).Path()))
	}
	// TODO: Enums
	// Messages
	for i := range fd.Messages().Len() {
		md := fd.Messages().Get(i)
		mds, err := MessageDescriptorToProtoString(md, 0)
		if err != nil {
			return "", errors.Wrapf(err, "stringify message descriptor %s", md.FullName())
		}
		sb.WriteString(mds)
		sb.WriteString("\n")
	}
	return sb.String(), nil
}

func MessageDescriptorToProtoString(md protoreflect.MessageDescriptor, indentLevel int) (string, error) {
	indent := strings.Repeat("  ", indentLevel)
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("%smessage %s {\n", indent, md.Name()))
	// Fields
	var fieldSb strings.Builder
	mapFieldTypes := make(map[string]struct{})
	for i := range md.Fields().Len() {
		fd := md.Fields().Get(i)
		// TODO: Handle maps/repeated that reference non-primitive types
		if fd.IsMap() {
			// Proto map fields use implicitly nested messages
			mapFieldTypes[string(fd.Message().FullName())] = struct{}{}
			keyType := fd.MapKey().Kind().String()
			valueType := fd.MapValue().Kind().String()
			fieldSb.WriteString(fmt.Sprintf("%s  map<%s, %s> %s = %d;\n", indent, keyType, valueType, fd.Name(), fd.Number()))
		} else if fd.IsList() {
			fieldType := fd.Kind().String()
			fieldSb.WriteString(fmt.Sprintf("%s  repeated %s %s = %d;\n", indent, fieldType, fd.Name(), fd.Number()))
		} else if fd.Kind() == protoreflect.MessageKind {
			fieldType := fd.Message().FullName()
			fieldSb.WriteString(fmt.Sprintf("%s  %s %s = %d;\n", indent, fieldType, fd.Name(), fd.Number()))
		} else if fd.Kind() == protoreflect.EnumKind {
			return "", fmt.Errorf("stringify %s: enum fields are not supported", fd.FullName())
		} else {
			fieldType := fd.Kind().String()
			fieldSb.WriteString(fmt.Sprintf("%s  %s %s = %d;\n", indent, fieldType, fd.Name(), fd.Number()))
		}
	}
	// Nested messages
	nestedMds := md.Messages()
	for i := range nestedMds.Len() {
		// Ignore "implicit" nested message types that are used for maps
		if _, ok := mapFieldTypes[string(nestedMds.Get(i).FullName())]; ok {
			continue
		}
		s, err := MessageDescriptorToProtoString(nestedMds.Get(i), indentLevel+1)
		if err != nil {
			return "", errors.Wrapf(err, "stringify nested message %s", nestedMds.Get(i).FullName())
		}
		sb.WriteString(s)
		sb.WriteString("\n")
	}
	// TODO: nested enums

	sb.WriteString(fieldSb.String())
	sb.WriteString(fmt.Sprintf("%s}\n", indent))
	return sb.String(), nil
}
