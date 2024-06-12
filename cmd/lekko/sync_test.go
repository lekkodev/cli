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

package main

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"testing"

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/descriptorpb"
)

func GenerateDescriptorSet(protoFilePath string) (*descriptorpb.FileDescriptorSet, error) {
	var out bytes.Buffer
	cmd := exec.Command("protoc", "--descriptor_set_out=/dev/stdout", protoFilePath)
	cmd.Stdout = &out

	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("failed to run protoc: %v", err)
	}

	// Unmarshal the data into a FileDescriptorSet
	fdSet := &descriptorpb.FileDescriptorSet{}
	if err := proto.Unmarshal(out.Bytes(), fdSet); err != nil {
		return nil, fmt.Errorf("failed to unmarshal descriptor data: %v", err)
	}

	return fdSet, nil
}

func Test_writeProtoFiles(t *testing.T) {
	tests := []struct {
		fileName string
	}{
		{fileName: "simple.proto"},
		{fileName: "default.proto"},
	}
	for _, tt := range tests {
		home, err := os.Getwd()
		if err != nil {
			panic(err)
		}
		t.Run(tt.fileName, func(t *testing.T) {
			err = os.Chdir("./testdata")
			if err != nil {
				panic(err)
			}
			fds, err := GenerateDescriptorSet(tt.fileName)
			if err != nil {
				panic(err)
			}
			fmt.Printf("%+v\n", fds)
			files := writeProtoFiles(fds)
			err = os.Chdir("./out")
			if err != nil {
				panic(err)
			}
			for fn, contents := range files {
				err := os.WriteFile(fn, []byte(contents), 0600)
				if err != nil {
					panic(err)
				}
				fds2, err := GenerateDescriptorSet(fn)
				if err != nil {
					panic(err)
				}
				fmt.Printf("%+v\n", fds2)
				if !proto.Equal(fds, fds2) {
					t.Errorf("writeProtoFiles() got = %v, want %v", fds2, fds)
				}
			}
			err = os.Chdir(home)
			if err != nil {
				panic(err)
			}
		})
	}
}

func Test_goToGo(t *testing.T) {
	t.Run("simple", func(t *testing.T) {
		ctx := context.Background()
		f, err := os.ReadFile("./testdata/simple.go")
		if err != nil {
			panic(err)
		}
		if got := goToGo(ctx, f); got != string(f) {
			t.Errorf("goToGo() = %v, want %v", got, string(f))
		}
	})
	t.Run("withcontext", func(t *testing.T) {
		ctx := context.Background()
		f, err := os.ReadFile("./testdata/withcontext.go")
		if err != nil {
			panic(err)
		}
		if got := goToGo(ctx, f); got != string(f) {
			t.Errorf("goToGo() = \n===\n%v+++, want \n===\n%v+++", got, string(f))
		}
	})
}
