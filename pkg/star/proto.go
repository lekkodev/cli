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
)

// Takes a path to the protobuf directory in the config repo, and generates
// a registry of user-defined types. This registry implements the Resolver
// interface, which is useful for compiling to json.
func BuildDynamicTypeRegistry(protoDir string) (*protoregistry.Types, error) {
	image, err := newBufImage(protoDir)
	if err != nil {
		return nil, errors.Wrap(err, "new buf image")
	}
	defer func() {
		// Note: we choose not to check in the buf image to the config repo and instead
		// always regenerate on-the-fly. This decision can be reevaluated.
		if err := image.cleanup(); err != nil {
			fmt.Printf("Error encountered when cleaning up buf image: %v", err)
		}
	}()
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
	var retErr error
	ret.RangeMessages(func(mt protoreflect.MessageType) bool {
		log.Printf("existing global message type: %v\n", mt.Descriptor().FullName())
		return true
	})
	files.RangeFiles(func(fd protoreflect.FileDescriptor) bool {
		for i := 0; i < fd.Enums().Len(); i++ {
			ed := fd.Enums().Get(i)
			if err := ret.RegisterEnum(dynamicpb.NewEnumType(ed)); err != nil {
				retErr = errors.Wrap(err, "register enum")
				return false
			}
		}
		for i := 0; i < fd.Messages().Len(); i++ {
			md := fd.Messages().Get(i)
			if err := ret.RegisterMessage(dynamicpb.NewMessageType(md)); err != nil {
				retErr = errors.Wrap(err, "register message")
				return false
			}
		}
		for i := 0; i < fd.Extensions().Len(); i++ {
			exd := fd.Extensions().Get(i)
			if err := ret.RegisterExtension(dynamicpb.NewExtensionType(exd)); err != nil {
				retErr = errors.Wrap(err, "register extension")
				return false
			}
		}
		return true
	})
	return ret, retErr
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
	err := cmd.Run()
	if err != nil {
		return nil, errors.Wrap(err, "buf build")
	}
	return &bufImage{
		filename: outputFile,
	}, nil
}

func (bi *bufImage) cleanup() error {
	return os.Remove(bi.filename)
}
