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
	//"bytes"
	"context"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	//"os/exec"
	"testing"

	"github.com/lekkodev/cli/pkg/gen"
	protoutils "github.com/lekkodev/cli/pkg/proto"
	"github.com/lekkodev/cli/pkg/repo"
	"github.com/lekkodev/cli/pkg/sync"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
	//"google.golang.org/protobuf/types/descriptorpb"
)

/* Disabling so that I don't need to worry about protoc on gh right now
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
*/

func DiffStyleOutput(a, b string) (string, error) {
	// Create temporary files to hold the input strings
	fileA, err := os.CreateTemp("", "fileA")
	if err != nil {
		return "", err
	}
	defer os.Remove(fileA.Name())

	fileB, err := os.CreateTemp("", "fileB")
	if err != nil {
		return "", err
	}
	defer os.Remove(fileB.Name())

	// Write strings to temporary files
	if _, err := fileA.WriteString(a); err != nil {
		return "", err
	}
	if _, err := fileB.WriteString(b); err != nil {
		return "", err
	}

	// Call the diff command
	cmd := exec.Command("diff", "-u", fileA.Name(), fileB.Name()) //#nosec G204
	output, err := cmd.CombinedOutput()
	if err != nil {
		// diff command returns non-zero exit code when files differ, ignore the error
		if exitErr, ok := err.(*exec.ExitError); ok {
			if exitErr.ExitCode() != 1 {
				return "", err
			}
		} else {
			return "", err
		}
	}

	return string(output), nil
}

// Test code -> repo -> code (compare) -> repo (compare)
func TestGoSyncToGenToSync(t *testing.T) {
	if err := filepath.WalkDir("./testdata", func(path string, d fs.DirEntry, err error) error {
		if !d.IsDir() && strings.HasSuffix(d.Name(), ".go") {
			t.Run(strings.TrimSuffix(d.Name(), ".go"), func(t *testing.T) {
				ctx := context.Background()
				orig, err := os.ReadFile(path)
				if err != nil {
					t.Fatalf("read test file %s: %v", path, err)
				}

				s1 := sync.NewGoSyncer()
				r1, err := s1.Sync(path)
				if err != nil {
					t.Fatalf("sync 1: %v", err)
				}

				tmpd, err := os.MkdirTemp("", "test")
				if err != nil {
					t.Fatalf("tmp dir: %v", err)
				}
				defer os.RemoveAll(tmpd)

				g, err := gen.NewGoGenerator("test", tmpd, "", r1)
				if err != nil {
					t.Fatalf("initialize gen: %v", err)
				}
				namespace := r1.Namespaces[0]
				_, private, err := g.GenNamespaceFiles(ctx, namespace.Name, namespace.Features, nil)
				if err != nil {
					t.Fatalf("gen: %v", err)
				}
				privatePath := filepath.Join(tmpd, fmt.Sprintf("%s.go", namespace.Name))
				if err := os.WriteFile(privatePath, []byte(private), 0600); err != nil {
					t.Fatalf("write private %s: %v", privatePath, err)
				}

				if string(orig) != private {
					diff, err := DiffStyleOutput(string(orig), private)
					if err != nil {
						t.Fatalf("diff: %v", err)
					}
					t.Fatalf("mismatch in generated code: %s", diff)
				}

				s2 := sync.NewGoSyncer()
				r2, err := s2.Sync(privatePath)
				if err != nil {
					t.Fatalf("sync 2: %v", err)
				}
				// NOTE: Because Anys contained serialized values, their serialization needs to be
				// deterministic for this check to always pass even if their deserialized values are equal.
				// The problem is that the determinism is not canonical across languages.
				// We should keep this in mind.
				if !proto.Equal(r1, r2) {
					r1json := protojson.Format(r1)
					r2json := protojson.Format(r2)
					diff, _ := DiffStyleOutput(r1json, r2json)
					t.Fatalf("mismatch in repo contents: %s", diff)
				}
			})
		}
		return nil
	}); err != nil {
		t.Fatalf("walk: %v", err)
	}
}

/*
Here to just run the code, but I really don't want to have an unrelated node thing as a dep for the test
func Test_ProtoJsonToTs(t *testing.T) {
	t.Run("simple", func(t *testing.T) {
		nsStr := `{"namespaces":[{"name":"default","configs":[{"static_feature":{"key":"banner-config","description":"","tree":{"default":{"@type":"type.googleapis.com/default.config.v1beta1.BannerConfig"},"constraints":[{"value":{"cta":{"external":true,"text":"Learn more","url":"https://www.lekko.com/"},"text":"This is a development only example of a banner on the login page","@type":"type.googleapis.com/default.config.v1beta1.BannerConfig"},"ruleAstNew":{"logicalExpression":{"rules":[{"atom":{"contextKey":"pathname","comparisonValue":"/login","comparisonOperator":"COMPARISON_OPERATOR_EQUALS"}},{"atom":{"contextKey":"env","comparisonValue":"development","comparisonOperator":"COMPARISON_OPERATOR_EQUALS"}}],"logicalOperator":"LOGICAL_OPERATOR_AND"}}},{"value":{"cta":{"external":true,"text":"Learn more","url":"https://www.lekko.com/"},"text":"A test banner for a particular repo main page","@type":"type.googleapis.com/default.config.v1beta1.BannerConfig"},"ruleAstNew":{"atom":{"contextKey":"pathname","comparisonValue":"/teams/lekko-staging/repositories/lekkodev/plugins/branches/main","comparisonOperator":"COMPARISON_OPERATOR_EQUALS"}}}]},"type":"FEATURE_TYPE_PROTO"}}]}]}`
		fdStr := `{"name":"lekko.proto","package":"default.config.v1beta1","dependency":[],"messageType":[{"name":"BannerConfig","field":[{"name":"text","number":1,"type":"TYPE_STRING","typeName":""},{"name":"cta","number":2,"type":"TYPE_MESSAGE","typeName":"default.config.v1beta1.BannerConfig.Cta"},{"name":"permanent","number":3,"type":"TYPE_BOOL","typeName":""}],"nestedType":[{"name":"Cta","field":[{"name":"text","number":1,"type":"TYPE_STRING","typeName":""},{"name":"url","number":2,"type":"TYPE_STRING","typeName":""},{"name":"external","number":3,"type":"TYPE_BOOL","typeName":""}],"nestedType":[],"enumType":[],"extensionRange":[],"extension":[],"oneofDecl":[],"reservedRange":[],"reservedName":[]}],"enumType":[],"extensionRange":[],"extension":[],"oneofDecl":[],"reservedRange":[],"reservedName":[]},{"name":"RemoteTemplates","field":[{"name":"templates","number":1,"label":"LABEL_REPEATED","type":"TYPE_MESSAGE","typeName":"default.config.v1beta1.RemoteTemplates.Templates"}],"nestedType":[{"name":"Templates","field":[{"name":"code","number":1,"type":"TYPE_STRING","typeName":""},{"name":"feature_type","number":2,"type":"TYPE_STRING","typeName":""},{"name":"name","number":3,"type":"TYPE_STRING","typeName":""}],"nestedType":[],"enumType":[],"extensionRange":[],"extension":[],"oneofDecl":[],"reservedRange":[],"reservedName":[]}],"enumType":[],"extensionRange":[],"extension":[],"oneofDecl":[],"reservedRange":[],"reservedName":[]},{"name":"BannerConfigArgs","field":[{"name":"env","number":1,"type":"TYPE_STRING","typeName":""},{"name":"pathname","number":2,"type":"TYPE_STRING","typeName":""}],"nestedType":[],"enumType":[],"extensionRange":[],"extension":[],"oneofDecl":[],"reservedRange":[],"reservedName":[]}],"enumType":[],"service":[],"extension":[],"publicDependency":[],"weakDependency":[],"syntax":"proto3"}`
		out, err := ProtoJSONToTS([]byte(nsStr), []byte(fdStr))
		fmt.Println(err)
		fmt.Println(out)
	})
}
*/

func Test_blobToTs(t *testing.T) {
	repoContents, err := repo.DecodeRepositoryContents([]byte("Cp0CCgdkZWZhdWx0EnUKDWJhbm5lci1jb25maWcaYgovCi10eXBlLmdvb2dsZWFwaXMuY29tL2dvb2dsZS5wcm90b2J1Zi5Cb29sVmFsdWUaLwotdHlwZS5nb29nbGVhcGlzLmNvbS9nb29nbGUucHJvdG9idWYuQm9vbFZhbHVlIAESmgEKBXRoaW5nGo4BCkUKMnR5cGUuZ29vZ2xlYXBpcy5jb20vbGVra28ucnVsZXMudjFiZXRhMy5Db25maWdDYWxsEg8aDWJhbm5lci1jb25maWcaRQoydHlwZS5nb29nbGVhcGlzLmNvbS9sZWtrby5ydWxlcy52MWJldGEzLkNvbmZpZ0NhbGwSDxoNYmFubmVyLWNvbmZpZyABEuwLCvsBCh5nb29nbGUvcHJvdG9idWYvZHVyYXRpb24ucHJvdG8SD2dvb2dsZS5wcm90b2J1ZiI6CghEdXJhdGlvbhIYCgdzZWNvbmRzGAEgASgDUgdzZWNvbmRzEhQKBW5hbm9zGAIgASgFUgVuYW5vc0KDAQoTY29tLmdvb2dsZS5wcm90b2J1ZkINRHVyYXRpb25Qcm90b1ABWjFnb29nbGUuZ29sYW5nLm9yZy9wcm90b2J1Zi90eXBlcy9rbm93bi9kdXJhdGlvbnBi+AEBogIDR1BCqgIeR29vZ2xlLlByb3RvYnVmLldlbGxLbm93blR5cGVzYgZwcm90bzMK4gUKHGdvb2dsZS9wcm90b2J1Zi9zdHJ1Y3QucHJvdG8SD2dvb2dsZS5wcm90b2J1ZiKYAQoGU3RydWN0EjsKBmZpZWxkcxgBIAMoCzIjLmdvb2dsZS5wcm90b2J1Zi5TdHJ1Y3QuRmllbGRzRW50cnlSBmZpZWxkcxpRCgtGaWVsZHNFbnRyeRIQCgNrZXkYASABKAlSA2tleRIsCgV2YWx1ZRgCIAEoCzIWLmdvb2dsZS5wcm90b2J1Zi5WYWx1ZVIFdmFsdWU6AjgBIrICCgVWYWx1ZRI7CgpudWxsX3ZhbHVlGAEgASgOMhouZ29vZ2xlLnByb3RvYnVmLk51bGxWYWx1ZUgAUgludWxsVmFsdWUSIwoMbnVtYmVyX3ZhbHVlGAIgASgBSABSC251bWJlclZhbHVlEiMKDHN0cmluZ192YWx1ZRgDIAEoCUgAUgtzdHJpbmdWYWx1ZRIfCgpib29sX3ZhbHVlGAQgASgISABSCWJvb2xWYWx1ZRI8CgxzdHJ1Y3RfdmFsdWUYBSABKAsyFy5nb29nbGUucHJvdG9idWYuU3RydWN0SABSC3N0cnVjdFZhbHVlEjsKCmxpc3RfdmFsdWUYBiABKAsyGi5nb29nbGUucHJvdG9idWYuTGlzdFZhbHVlSABSCWxpc3RWYWx1ZUIGCgRraW5kIjsKCUxpc3RWYWx1ZRIuCgZ2YWx1ZXMYASADKAsyFi5nb29nbGUucHJvdG9idWYuVmFsdWVSBnZhbHVlcyobCglOdWxsVmFsdWUSDgoKTlVMTF9WQUxVRRAAQn8KE2NvbS5nb29nbGUucHJvdG9idWZCC1N0cnVjdFByb3RvUAFaL2dvb2dsZS5nb2xhbmcub3JnL3Byb3RvYnVmL3R5cGVzL2tub3duL3N0cnVjdHBi+AEBogIDR1BCqgIeR29vZ2xlLlByb3RvYnVmLldlbGxLbm93blR5cGVzYgZwcm90bzMKhgQKHmdvb2dsZS9wcm90b2J1Zi93cmFwcGVycy5wcm90bxIPZ29vZ2xlLnByb3RvYnVmIiMKC0RvdWJsZVZhbHVlEhQKBXZhbHVlGAEgASgBUgV2YWx1ZSIiCgpGbG9hdFZhbHVlEhQKBXZhbHVlGAEgASgCUgV2YWx1ZSIiCgpJbnQ2NFZhbHVlEhQKBXZhbHVlGAEgASgDUgV2YWx1ZSIjCgtVSW50NjRWYWx1ZRIUCgV2YWx1ZRgBIAEoBFIFdmFsdWUiIgoKSW50MzJWYWx1ZRIUCgV2YWx1ZRgBIAEoBVIFdmFsdWUiIwoLVUludDMyVmFsdWUSFAoFdmFsdWUYASABKA1SBXZhbHVlIiEKCUJvb2xWYWx1ZRIUCgV2YWx1ZRgBIAEoCFIFdmFsdWUiIwoLU3RyaW5nVmFsdWUSFAoFdmFsdWUYASABKAlSBXZhbHVlIiIKCkJ5dGVzVmFsdWUSFAoFdmFsdWUYASABKAxSBXZhbHVlQoMBChNjb20uZ29vZ2xlLnByb3RvYnVmQg1XcmFwcGVyc1Byb3RvUAFaMWdvb2dsZS5nb2xhbmcub3JnL3Byb3RvYnVmL3R5cGVzL2tub3duL3dyYXBwZXJzcGL4AQGiAgNHUEKqAh5Hb29nbGUuUHJvdG9idWYuV2VsbEtub3duVHlwZXNiBnByb3RvMw=="))
	if err != nil {
		panic(err)
	}

	typeRegistry, err := protoutils.FileDescriptorSetToTypeRegistry(repoContents.FileDescriptorSet)
	if err != nil {
		panic(err)
	}

	for _, namespace := range repoContents.Namespaces {
		for _, f := range namespace.Features {
			code, err := gen.GenTSForFeature(f, namespace.Name, "", typeRegistry)
			if err != nil {
				panic(err)
			}
			fmt.Println(code)
		}
	}
}
