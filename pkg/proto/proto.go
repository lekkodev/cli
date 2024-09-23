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

package proto

import (
	"fmt"
	"strings"

	"github.com/pkg/errors"
	"google.golang.org/protobuf/reflect/protodesc"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/reflect/protoregistry"
	"google.golang.org/protobuf/types/descriptorpb"
	"google.golang.org/protobuf/types/dynamicpb"
	"google.golang.org/protobuf/types/known/anypb"
	"google.golang.org/protobuf/types/known/durationpb"
	"google.golang.org/protobuf/types/known/structpb"
	"google.golang.org/protobuf/types/known/wrapperspb"

	featurev1beta1 "buf.build/gen/go/lekkodev/cli/protocolbuffers/go/lekko/feature/v1beta1"
	rulesv1beta2 "buf.build/gen/go/lekkodev/cli/protocolbuffers/go/lekko/rules/v1beta2"
	rulesv1beta3 "buf.build/gen/go/lekkodev/cli/protocolbuffers/go/lekko/rules/v1beta3"
)

// Initializes an empty file descriptor set and starts with well-known types.
// TODO: currently used for sync, but consider if this should be used in all other places (e.g. repo init, compile, etc.)
func NewDefaultFileDescriptorSet() *descriptorpb.FileDescriptorSet {
	fds := &descriptorpb.FileDescriptorSet{}
	fds.File = append(fds.File, protodesc.ToFileDescriptorProto(descriptorpb.File_google_protobuf_descriptor_proto))
	fds.File = append(fds.File, protodesc.ToFileDescriptorProto(wrapperspb.File_google_protobuf_wrappers_proto))
	fds.File = append(fds.File, protodesc.ToFileDescriptorProto(structpb.File_google_protobuf_struct_proto))
	fds.File = append(fds.File, protodesc.ToFileDescriptorProto(durationpb.File_google_protobuf_duration_proto))
	fds.File = append(fds.File, protodesc.ToFileDescriptorProto(anypb.File_google_protobuf_any_proto))
	fds.File = append(fds.File, protodesc.ToFileDescriptorProto(rulesv1beta2.File_lekko_rules_v1beta2_rules_proto))
	fds.File = append(fds.File, protodesc.ToFileDescriptorProto(rulesv1beta3.File_lekko_rules_v1beta3_rules_proto))
	fds.File = append(fds.File, protodesc.ToFileDescriptorProto(featurev1beta1.File_lekko_feature_v1beta1_feature_proto))
	return fds
}

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

func FileDescriptorSetToTypeRegistry(fds *descriptorpb.FileDescriptorSet) (*protoregistry.Types, error) {
	fr, err := protodesc.NewFiles(fds)
	if err != nil {
		return nil, errors.Wrap(err, "convert to file registry")
	}
	tr, err := FileRegistryToTypeRegistry(fr)
	return tr, errors.Wrap(err, "get type registry from file registry")
}

// TODO: Canonical proto formatter that doesn't rely on `buf format`, which we should use for all languages

// Print the contents of a file descriptor in a format suitable for e.g. writing to a .proto file
func PrintFileDescriptor(fd protoreflect.FileDescriptor) (string, error) {
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
		mds, err := PrintMessageDescriptor(md, 0)
		if err != nil {
			return "", errors.Wrapf(err, "stringify message descriptor %s", md.FullName())
		}
		sb.WriteString(mds)
		sb.WriteString("\n")
	}
	return sb.String(), nil
}

func PrintMessageDescriptor(md protoreflect.MessageDescriptor, indentLevel int) (string, error) {
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
		s, err := PrintMessageDescriptor(nestedMds.Get(i), indentLevel+1)
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
