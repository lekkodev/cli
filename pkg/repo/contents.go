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

package repo

import (
	"encoding/base64"

	featurev1beta1 "buf.build/gen/go/lekkodev/cli/protocolbuffers/go/lekko/feature/v1beta1"
	protoutils "github.com/lekkodev/cli/pkg/proto"
	"github.com/pkg/errors"
	"google.golang.org/protobuf/proto"
)

// TODO: Diffing method

// Take the namespace contents of `other` into `dst`.
// Namespaces and features in `other` that are not in `dst` are added.
// Conflicting features are overwritten with ones from `other`.
// Namespaces in `dst` that are not in `other` are NOT removed.
// Returns the modified `dst`. `other` should not be used after being passed.
func TakeNamespaceContents(dst *featurev1beta1.RepositoryContents, other *featurev1beta1.RepositoryContents) *featurev1beta1.RepositoryContents {
	for _, otherNs := range other.Namespaces {
		replaced := false
		for i, dstNs := range dst.Namespaces {
			if dstNs.Name == otherNs.Name {
				dst.Namespaces[i] = otherNs
				replaced = true
			}
		}
		if !replaced {
			dst.Namespaces = append(dst.Namespaces, otherNs)
		}
	}
	// Need to also overwrite appropriate file descriptors
	// Assumes that file descriptors corresponding to namespaces have deterministic names
	for _, otherFD := range other.FileDescriptorSet.File {
		replaced := false
		for i, dstFD := range dst.FileDescriptorSet.File {
			if dstFD.Name == otherFD.Name {
				dst.FileDescriptorSet.File[i] = otherFD
				replaced = true
			}
		}
		if !replaced {
			dst.FileDescriptorSet.File = append(dst.FileDescriptorSet.File, otherFD)
		}
	}
	return dst
}

// Expects base64 encoded serialized RepositoryContents message
func DecodeRepositoryContents(encoded []byte) (*featurev1beta1.RepositoryContents, error) {
	// Because Protobuf is not self-describing, we have to jump through some hoops here for deserialization.
	// The RepositoryContents message contains the FDS which we want to use as the resolver for unmarshalling
	// the rest of the contents.
	decoded, err := base64.StdEncoding.DecodeString(string(encoded))
	if err != nil {
		return nil, errors.Wrap(err, "base64 decode")
	}
	// First pass unmarshal to get the FDS while ignoring any unresolvable Anys
	tempRepoContents := &featurev1beta1.RepositoryContents{}
	err = proto.UnmarshalOptions{DiscardUnknown: true, Resolver: &protoutils.IgnoreAnyResolver{}}.Unmarshal(decoded, tempRepoContents)
	if err != nil {
		return nil, errors.Wrap(err, "shallow unmarshal")
	}
	typeRegistry, err := protoutils.FileDescriptorSetToTypeRegistry(tempRepoContents.FileDescriptorSet)
	if err != nil {
		return nil, errors.Wrap(err, "get type registry")
	}
	// Re-unmarshal using type registry this time - we can resolve Anys correctly
	repoContents := &featurev1beta1.RepositoryContents{}
	err = proto.UnmarshalOptions{Resolver: typeRegistry}.Unmarshal(decoded, repoContents)
	if err != nil {
		return nil, errors.Wrap(err, "full unmarshal")
	}
	return repoContents, nil
}
