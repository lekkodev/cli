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

package star

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/pkg/errors"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protodesc"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/reflect/protoregistry"
	"google.golang.org/protobuf/types/descriptorpb"
	"google.golang.org/protobuf/types/dynamicpb"
	"google.golang.org/protobuf/types/known/wrapperspb"
)

// Takes a path to the protobuf directory in the config repo, and generates
// a registry of user-defined types. This registry implements the Resolver
// interface, which is useful for compiling to json.
func BuildDynamicTypeRegistry(protoDir string) (*protoregistry.Types, error) {
	image, err := newBufImage(protoDir)
	if err != nil {
		return nil, errors.Wrap(err, "new buf image")
	}
	bytes, err := os.ReadFile(image.filename)
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
	return filesToTypes(files)
}

func filesToTypes(files *protoregistry.Files) (*protoregistry.Types, error) {
	// Start from an empty type registry. All user-defined types
	// should be explicitly imported in their .proto files, which will end up
	// getting included since we're including imports in our file descriptor set.
	ret := &protoregistry.Types{}
	ret.RangeMessages(func(mt protoreflect.MessageType) bool {
		log.Printf("existing global message type: %v\n", mt.Descriptor().FullName())
		return true
	})
	var rangeErr error
	files.RangeFiles(func(fd protoreflect.FileDescriptor) bool {
		if err := registerTypes(ret, fd, false); err != nil {
			rangeErr = errors.Wrap(err, "registering user-defined types")
			return false
		}
		return true
	})
	if rangeErr != nil {
		return nil, rangeErr
	}
	// Since we're internally converting starlark primitive types to google.protobuf.*Value,
	// we need to ensure that the wrapperspb types exist in the type registry in order for
	// json marshaling to work. However, we also need to ensure that type registration does
	// not panic in the event that the user also imported wrappers.proto
	if err := registerTypes(ret, wrapperspb.File_google_protobuf_wrappers_proto, true); err != nil {
		return nil, errors.Wrap(err, "registering wrapperspb")
	}
	return ret, nil
}

func registerTypes(t *protoregistry.Types, fd protoreflect.FileDescriptor, checkNotExists bool) error {
	existingTypes := make(map[string]struct{})
	if checkNotExists {
		t.RangeEnums(func(et protoreflect.EnumType) bool {
			existingTypes[string(et.Descriptor().FullName())] = struct{}{}
			return true
		})
		t.RangeMessages(func(et protoreflect.MessageType) bool {
			existingTypes[string(et.Descriptor().FullName())] = struct{}{}
			return true
		})
		t.RangeExtensions(func(et protoreflect.ExtensionType) bool {
			existingTypes[string(et.TypeDescriptor().FullName())] = struct{}{}
			return true
		})
	}
	for i := 0; i < fd.Enums().Len(); i++ {
		ed := fd.Enums().Get(i)
		if _, ok := existingTypes[string(ed.FullName())]; ok {
			log.Printf("skipping registration of type %s, already exists", ed.FullName())
			continue
		}
		if err := t.RegisterEnum(dynamicpb.NewEnumType(ed)); err != nil {
			return errors.Wrap(err, "register enum")
		}
	}
	for i := 0; i < fd.Messages().Len(); i++ {
		md := fd.Messages().Get(i)
		if _, ok := existingTypes[string(md.FullName())]; ok {
			log.Printf("skipping registration of type %s, already exists", md.FullName())
			continue
		}
		if err := t.RegisterMessage(dynamicpb.NewMessageType(md)); err != nil {
			return errors.Wrap(err, "register message")
		}
	}
	for i := 0; i < fd.Extensions().Len(); i++ {
		exd := fd.Extensions().Get(i)
		if _, ok := existingTypes[string(exd.FullName())]; ok {
			log.Printf("skipping registration of type %s, already exists", exd.FullName())
			continue
		}
		if err := t.RegisterExtension(dynamicpb.NewExtensionType(exd)); err != nil {
			return errors.Wrap(err, "register extension")
		}
	}
	return nil
}

type bufImage struct {
	filename string
}

// Generates a buf image, which is compatible with
// protobuf's native FileDescriptorSet type. This allows us to build a
// registry of protobuf types, adding any user-defined types.
func newBufImage(protoDir string) (*bufImage, error) {
	outputFile := filepath.Join(protoDir, "image.bin")
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
