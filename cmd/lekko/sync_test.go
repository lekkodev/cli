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
	"os"
	"os/exec"

	//"os/exec"
	"testing"
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

	t.Run("one_two", func(t *testing.T) {
		ctx := context.Background()
		f, err := os.ReadFile("./testdata/twostructs.go")
		if err != nil {
			panic(err)
		}
		if got := goToGo(ctx, f); got != string(f) {
			t.Errorf("goToGo() = \n===\n%v+++, want \n===\n%v+++", got, string(f))
		}
	})
}

func Test_Gertrude(t *testing.T) {
	t.Run("gertrude", func(t *testing.T) {
		ctx := context.Background()
		f, err := os.ReadFile("./testdata/gertrude.go")
		if err != nil {
			panic(err)
		}
		if got := goToGo(ctx, f); got != string(f) {
			diff, err := DiffStyleOutput(string(f), got)
			if err != nil {
				panic(err)
			}
			t.Errorf("Difference Found: %s\n", diff)
		}
	})
}

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

func TestDefault(t *testing.T) {
	t.Run("default", func(t *testing.T) {
		ctx := context.Background()
		f, err := os.ReadFile("./testdata/default.go")
		if err != nil {
			panic(err)
		}
		if got := goToGo(ctx, f); got != string(f) {
			diff, err := DiffStyleOutput(string(f), got)
			if err != nil {
				panic(err)
			}
			t.Errorf("Difference Found: %s\n", diff)
		}
	})
}

func TestDuration(t *testing.T) {
	t.Run("default", func(t *testing.T) {
		ctx := context.Background()
		f, err := os.ReadFile("./testdata/duration.go")
		if err != nil {
			panic(err)
		}
		if got := goToGo(ctx, f); got != string(f) {
			diff, err := DiffStyleOutput(string(f), got)
			if err != nil {
				panic(err)
			}
			t.Errorf("Difference Found: %s\n", diff)
		}
	})
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
