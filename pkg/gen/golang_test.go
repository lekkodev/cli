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

package gen

import (
	"testing"

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protodesc"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/reflect/protoregistry"
	"google.golang.org/protobuf/types/descriptorpb"
)

func TestDescriptorToStructDeclaration(t *testing.T) {
	// Define the field descriptor for "foo"
	fieldDesc := &descriptorpb.FieldDescriptorProto{
		Name:   proto.String("foo"),
		Number: proto.Int32(1),
		Label:  descriptorpb.FieldDescriptorProto_LABEL_OPTIONAL.Enum(),
		Type:   descriptorpb.FieldDescriptorProto_TYPE_STRING.Enum(),
	}

	// Define the message descriptor
	msgDesc := &descriptorpb.DescriptorProto{
		Name:  proto.String("Moo"),
		Field: []*descriptorpb.FieldDescriptorProto{fieldDesc},
	}

	// Create a FileDescriptorProto
	fileDesc := &descriptorpb.FileDescriptorProto{
		Name:    proto.String("test.proto"),
		Package: proto.String("test"),
		MessageType: []*descriptorpb.DescriptorProto{
			msgDesc,
		},
	}
	file, _ := protodesc.NewFile(fileDesc, nil)
	err := protoregistry.GlobalFiles.RegisterFile(file)
	if err != nil {
		panic(err)
	}

	// Find the message descriptor within the file descriptor
	message, err := protoregistry.GlobalFiles.FindDescriptorByName(protoreflect.FullName("test.Moo"))
	if err != nil {
		panic(err)
	}
	md, ok := message.(protoreflect.MessageDescriptor)
	if !ok {
		panic("Not an md")
	}

	type args struct {
		d protoreflect.MessageDescriptor
	}
	tests := []struct {
		name string
		args args
		want string
	}{
		{
			name: "basic test",
			args: args{
				d: md,
			},
			want: `type Moo struct {
	Foo string;
}
`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := DescriptorToStructDeclaration(tt.args.d); got != tt.want {
				t.Errorf("DescriptorToStructDeclaration() = %v, want %v", got, tt.want)
			}
		})
	}
}
