package repo

import (
	"encoding/base64"

	featurev1beta1 "buf.build/gen/go/lekkodev/cli/protocolbuffers/go/lekko/feature/v1beta1"
	protoutils "github.com/lekkodev/cli/pkg/proto"
	"github.com/pkg/errors"
	"google.golang.org/protobuf/proto"
)

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
