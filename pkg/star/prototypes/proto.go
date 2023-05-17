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

package prototypes

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/lekkodev/cli/pkg/fs"
	"github.com/pkg/errors"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protodesc"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/reflect/protoregistry"
	"google.golang.org/protobuf/types/descriptorpb"
	"google.golang.org/protobuf/types/dynamicpb"
	"google.golang.org/protobuf/types/known/structpb"
	"google.golang.org/protobuf/types/known/wrapperspb"
)

const installBuf = `
Please install buf in order to use lekko cli.
Using homebrew, run 'brew install bufbuild/buf/buf'. 
For other options, see the installation page here: https://docs.buf.build/installation`

var requiredFileDescriptors = []protoreflect.FileDescriptor{
	// Since we're internally converting starlark primitive types to google.protobuf.*Value,
	// we need to ensure that the wrapperspb types exist in the type registry in order for
	// json marshaling to work. However, we also need to ensure that type registration does
	// not panic in the event that the user also imported wrappers.proto
	wrapperspb.File_google_protobuf_wrappers_proto,
	// In the case of json feature flags, we will internally represent the json object as a structpb
	// proto object.
	structpb.File_google_protobuf_struct_proto,
}

// Takes a path to the protobuf directory in the config repo, and generates
// a registry of user-defined types. This registry implements the Resolver
// interface, which is useful for compiling to json.
func BuildDynamicTypeRegistry(ctx context.Context, protoDir string, provider fs.Provider) (*SerializableTypes, error) {
	image, err := provider.GetFileContents(ctx, bufImageFilepath(protoDir))
	if err != nil {
		return nil, errors.Wrap(err, "read buf image")
	}
	return BuildDynamicTypeRegistryFromBufImage(image)
}

// Note: this method is not safe to be run on ephemeral repos, as it invokes the buf cmd line.
func ReBuildDynamicTypeRegistry(ctx context.Context, protoDir string, cw fs.ConfigWriter) (*SerializableTypes, error) {
	if err := checkBufExists(); err != nil {
		return nil, err
	}
	// Lint before generating the new buf image
	if err := lint(protoDir); err != nil {
		return nil, errors.Wrap(err, "buf lint")
	}
	_, err := newBufImage(protoDir)
	if err != nil {
		return nil, errors.Wrap(err, "new buf image")
	}
	return BuildDynamicTypeRegistry(ctx, protoDir, cw)
}

func BuildDynamicTypeRegistryFromBufImage(image []byte) (*SerializableTypes, error) {
	fds := &descriptorpb.FileDescriptorSet{}

	if err := proto.Unmarshal(image, fds); err != nil {
		return nil, errors.Wrap(err, "proto unmarshal")
	}
	files, err := protodesc.NewFiles(fds)

	if err != nil {
		return nil, errors.Wrap(err, "protodesc.NewFiles")
	}
	return RegisterDynamicTypes(files)
}

func bufImageFilepath(protoDir string) string {
	return filepath.Join(protoDir, "image.bin")
}

func RegisterDynamicTypes(files *protoregistry.Files) (*SerializableTypes, error) {
	// Start from an empty type registry.
	ret := NewSerializableTypes()
	// First, add required types
	for _, fd := range requiredFileDescriptors {
		if err := ret.AddFileDescriptor(fd, false); err != nil {
			return nil, errors.Wrapf(err, "registering file descriptor: %s", string(fd.FullName()))
		}
	}
	// Import user defined types, ignoring types that already exist. All user-defined types
	// should be explicitly imported in their .proto files, which will end up
	// getting included since we're including imports in our file descriptor set.
	if files != nil {
		var rangeErr error
		files.RangeFiles(func(fd protoreflect.FileDescriptor) bool {
			if err := ret.AddFileDescriptor(fd, true); err != nil {
				rangeErr = errors.Wrap(err, "registering user-defined types")
				return false
			}
			return true
		})
		if rangeErr != nil {
			return nil, rangeErr
		}
	}
	return ret, nil
}

// Wrapper around a user-defined registry that allows it to be serialized.
// Under the hood, the go-native types and the file descriptor set are kept
// in-sync. If types are registered in one, they are added to the serializable
// object as well. FileDescriptorSet is a protobuf object that can be serialized
// and shared with external systems (e.g. the frontend)
type SerializableTypes struct {
	Types             *protoregistry.Types
	FileDescriptorSet *descriptorpb.FileDescriptorSet
}

func NewSerializableTypes() *SerializableTypes {
	return &SerializableTypes{
		Types:             &protoregistry.Types{},
		FileDescriptorSet: &descriptorpb.FileDescriptorSet{},
	}
}

func (st *SerializableTypes) AddFileDescriptor(fd protoreflect.FileDescriptor, checkNotExists bool) error {
	existingTypes := make(map[string]struct{})
	if checkNotExists {
		st.Types.RangeEnums(func(et protoreflect.EnumType) bool {
			existingTypes[string(et.Descriptor().FullName())] = struct{}{}
			return true
		})
		st.Types.RangeMessages(func(et protoreflect.MessageType) bool {
			existingTypes[string(et.Descriptor().FullName())] = struct{}{}
			return true
		})
		st.Types.RangeExtensions(func(et protoreflect.ExtensionType) bool {
			existingTypes[string(et.TypeDescriptor().FullName())] = struct{}{}
			return true
		})
	}
	var numRegistered int
	for i := 0; i < fd.Enums().Len(); i++ {
		ed := fd.Enums().Get(i)
		if _, ok := existingTypes[string(ed.FullName())]; ok {
			continue
		}
		if err := st.Types.RegisterEnum(dynamicpb.NewEnumType(ed)); err != nil {
			return errors.Wrap(err, "register enum")
		}
		numRegistered++
	}
	for i := 0; i < fd.Messages().Len(); i++ {
		md := fd.Messages().Get(i)
		if _, ok := existingTypes[string(md.FullName())]; ok {
			continue
		}
		if err := st.Types.RegisterMessage(dynamicpb.NewMessageType(md)); err != nil {
			return errors.Wrap(err, "register message")
		}
		numRegistered++
	}
	for i := 0; i < fd.Extensions().Len(); i++ {
		exd := fd.Extensions().Get(i)
		if _, ok := existingTypes[string(exd.FullName())]; ok {
			continue
		}
		if err := st.Types.RegisterExtension(dynamicpb.NewExtensionType(exd)); err != nil {
			return errors.Wrap(err, "register extension")
		}
		numRegistered++
	}
	if numRegistered > 0 {
		st.FileDescriptorSet.File = append(st.FileDescriptorSet.File, protodesc.ToFileDescriptorProto(fd))
	}
	return nil
}

type bufImage struct {
	filename string
}

// Generates a buf image, which is compatible with
// protobuf's native FileDescriptorSet type. This allows us to build a
// registry of protobuf types, adding any user-defined types.
// Note: expects that buf cmd line exists.
func newBufImage(protoDir string) (*bufImage, error) {
	outputFile := bufImageFilepath(protoDir)
	args := []string{
		"build",
		protoDir,
		"--exclude-source-info",
		fmt.Sprintf("-o %s", outputFile),
	}
	cmd := exec.Command("buf", args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return nil, errors.Wrap(err, "buf build")
	}
	return &bufImage{
		filename: outputFile,
	}, nil
}

// Note: expects that buf cmd lint exists.
func lint(protoDir string) error {
	cmd := exec.Command("buf", "lint", protoDir)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return errors.Wrap(err, "buf lint")
	}
	return nil
}

func checkBufExists() error {
	cmd := exec.Command("buf", "--version")
	if err := cmd.Run(); err != nil {
		fmt.Println(installBuf)
		return errors.Wrap(err, "buf not found on the cmd line")
	}
	return nil
}
