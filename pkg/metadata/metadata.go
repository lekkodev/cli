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

// This package governs metadata in configuration repos and their surrounding
// development repositories as well.
//
// Metadata refers to a yaml file that provides information about how to parse
// and interact with the referenced configuration files.
//
// ConfigRepo or configuration repository refers to the source of truth repository.
//
// DevRepo or development repository refers to the repository where lekko config
// is used, but not as a source of truth.
package metadata

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/lekkodev/cli/pkg/fs"
	"github.com/pkg/errors"

	"gopkg.in/yaml.v3"
)

// The type for
type RootConfigRepoMetadata struct {
	// This version refers to the version of the metadata.
	Version        string   `json:"version,omitempty" yaml:"version,omitempty"`
	Namespaces     []string `json:"namespaces,omitempty" yaml:"namespaces,omitempty"`
	ProtoDirectory string   `json:"protoDir,omitempty" yaml:"protoDir,omitempty"`
}

type NamespaceConfigRepoMetadata struct {
	// This version refers to the version of the configuration in the repo itself.
	// TODO we should move this to a separate version number.
	Version string `json:"version,omitempty" yaml:"version,omitempty"`
	Name    string `json:"name,omitempty" yaml:"name,omitempty"`
}

const DefaultRootConfigRepoMetadataFileName = "lekko.root.yaml"
const DefaultNamespaceConfigRepoMetadataFileName = "lekko.ns.yaml"
const GenFolderPathJSON = "gen/json"
const GenFolderPathProto = "gen/proto"

// Parses the lekko metadata from a configuration repo in a strict way, returning a user error on failure.
//
// This looks in the default locations and names for now, and a custom extension
// can be written later.
//
// This returns the root metadata of a repository and a map from namespace name to namespace metadata.
//
// This takes a provider so that we can use the same code on a local version on disk as well as in Github.
func ParseFullConfigRepoMetadataStrict(ctx context.Context, path string, provider fs.Provider) (*RootConfigRepoMetadata, map[string]*NamespaceConfigRepoMetadata, error) {
	contents, err := provider.GetFileContents(ctx, filepath.Join(path, DefaultRootConfigRepoMetadataFileName))
	if err != nil {
		return nil, nil, fmt.Errorf("could not open root metadata: %v", err)
	}

	var rootMetadata RootConfigRepoMetadata
	if err := UnmarshalYAMLStrict(contents, &rootMetadata); err != nil {
		return nil, nil, fmt.Errorf("could not parse root metadata: %v", err)
	}
	nsMDs := make(map[string]*NamespaceConfigRepoMetadata)
	for _, namespace := range rootMetadata.Namespaces {
		nsMD, err := ParseNamespaceMetadataStrict(ctx, path, namespace, provider)
		if err != nil {
			return nil, nil, errors.Wrap(err, "failed to parse namespace metadata")
		}
		nsMDs[namespace] = nsMD
	}
	return &rootMetadata, nsMDs, nil
}

func UpdateRootConfigRepoMetadata(ctx context.Context, path string, cw fs.ConfigWriter, f func(*RootConfigRepoMetadata)) error {
	contents, err := cw.GetFileContents(ctx, filepath.Join(path, DefaultRootConfigRepoMetadataFileName))
	if err != nil {
		return fmt.Errorf("could not open root metadata: %v", err)
	}
	var rootMetadata RootConfigRepoMetadata
	if err := UnmarshalYAMLStrict(contents, &rootMetadata); err != nil {
		return fmt.Errorf("could not parse root metadata: %v", err)
	}
	f(&rootMetadata)
	if err := writeYAML(cw, &rootMetadata, filepath.Join(path, DefaultRootConfigRepoMetadataFileName), 0644); err != nil {
		return errors.Wrap(err, "failed to write root md file")
	}
	return nil
}

func ParseNamespaceMetadataStrict(ctx context.Context, rootPath, namespaceName string, provider fs.Provider) (*NamespaceConfigRepoMetadata, error) {
	contents, err := provider.GetFileContents(ctx, filepath.Join(rootPath, namespaceName, DefaultNamespaceConfigRepoMetadataFileName))
	if err != nil {
		return nil, errors.Wrap(err, "could not open namespace metadata")
	}
	var nsConfig NamespaceConfigRepoMetadata
	if err := UnmarshalYAMLStrict(contents, &nsConfig); err != nil {
		return nil, errors.Wrap(err, "could not parse namespace metadata")
	}
	if nsConfig.Name != namespaceName {
		return nil, fmt.Errorf("invalid configuration, namespace in root metadata: %s does not match in-namespace metadata: %s", namespaceName, nsConfig.Name)
	}
	return &nsConfig, nil
}

func CreateNamespaceMetadata(ctx context.Context, rootPath, namespaceName string, cw fs.ConfigWriter, version string) error {
	if err := cw.MkdirAll(filepath.Join(rootPath, namespaceName), 0755); err != nil {
		return errors.Wrap(err, "failed to mkdir")
	}
	if err := writeYAML(cw, &NamespaceConfigRepoMetadata{
		Version: version,
		Name:    namespaceName,
	}, filepath.Join(rootPath, namespaceName, DefaultNamespaceConfigRepoMetadataFileName), 0600); err != nil {
		return errors.Wrap(err, "failed to write namespace metadata")
	}

	// now, add the new namespace to the root repo metadata
	rootMD, _, err := ParseFullConfigRepoMetadataStrict(ctx, rootPath, cw)
	if err != nil {
		return errors.Wrap(err, "failed to parse root config repo metadata")
	}
	rootMD.Namespaces = append(rootMD.Namespaces, namespaceName)
	if err := writeYAML(cw, rootMD, filepath.Join(rootPath, DefaultRootConfigRepoMetadataFileName), 0644); err != nil {
		return errors.Wrap(err, "failed to write root config repo metadata")
	}
	return nil
}

func UpdateNamespaceMetadata(ctx context.Context, rootPath, namespaceName string, cw fs.ConfigWriter, f func(*NamespaceConfigRepoMetadata)) error {
	existing, err := ParseNamespaceMetadataStrict(ctx, rootPath, namespaceName, cw)
	if err != nil {
		return errors.Wrap(err, "failed to parse namespace metadata")
	}
	f(existing)
	return writeYAML(cw, existing, filepath.Join(rootPath, namespaceName, DefaultNamespaceConfigRepoMetadataFileName), 0600)
}

func writeYAML(cw fs.ConfigWriter, goStruct interface{}, filename string, perm os.FileMode) error {
	bytes, err := MarshalYAML(goStruct)
	if err != nil {
		return errors.Wrap(err, "failed to marshal yaml")
	}
	if err := cw.WriteFile(filename, bytes, perm); err != nil {
		return errors.Wrap(err, "failed to write file")
	}
	return nil
}

// This is taken from github.com/bufbuild/buf/private/pkg/encoding/encoding.go
//
// UnmarshalYAMLStrict unmarshals the data as YAML, returning a user error on failure.
//
// If the data length is 0, this is a no-op.
func UnmarshalYAMLStrict(data []byte, v interface{}) error {
	if len(data) == 0 {
		return nil
	}
	yamlDecoder := NewYAMLDecoderStrict(bytes.NewReader(data))
	if err := yamlDecoder.Decode(v); err != nil {
		return fmt.Errorf("could not unmarshal as YAML: %v", err)
	}
	return nil
}

func MarshalYAML(v interface{}) ([]byte, error) {
	var b bytes.Buffer
	encoder := yaml.NewEncoder(&b)
	if err := encoder.Encode(v); err != nil {
		return nil, fmt.Errorf("failed to encode yaml: %v", err)
	}
	return b.Bytes(), nil
}

// NewYAMLDecoderStrict creates a new YAML decoder from the reader.
func NewYAMLDecoderStrict(reader io.Reader) *yaml.Decoder {
	yamlDecoder := yaml.NewDecoder(reader)
	yamlDecoder.KnownFields(true)
	return yamlDecoder
}

// NewYAMLDecoderStrict creates a new YAML decoder from the reader.
func NewYAMLEncoderStrict(writer io.Writer) *yaml.Encoder {
	return yaml.NewEncoder(writer)
}
