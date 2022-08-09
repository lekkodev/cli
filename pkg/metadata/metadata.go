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
	"fmt"
	"io"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// The type for
type RootConfigRepoMetadata struct {
	// This version refers to the version of the metadata.
	Version    string   `json:"version,omitempty" yaml:"version,omitempty"`
	Namespaces []string `json:"namespaces,omitempty" yaml:"namespaces,omitempty"`
}

type NamespaceConfigRepoMetadata struct {
	// This version refers to the version of the configuration in the repo itself.
	// TODO we should move this to a separate version number.
	Version string `json:"version,omitempty" yaml:"version,omitempty"`
	Name    string `json:"name,omitempty" yaml:"name,omitempty"`
}

const DefaultRootConfigRepoMetadataFileName = "lekko.root.yaml"
const DefaultNamespaceConfigRepoMetadataFileName = "lekko.ns.yaml"

// Parses the lekko metadata from a configuration repo in a strict way, returning a user error on failure.
//
// This looks in the default locations and names for now, and a custom extension
// can be written later.
//
// This returns the root metadata of a repository and a map from namespace name to namespace metadata.
func ParseFullConfigRepoMetadataStrict(path string) (*RootConfigRepoMetadata, map[string]*NamespaceConfigRepoMetadata, error) {
	contents, err := os.ReadFile(filepath.Join(path, DefaultRootConfigRepoMetadataFileName))
	if err != nil {
		return nil, nil, fmt.Errorf("could not open root metadata: %v", err)
	}

	var rootMetadata RootConfigRepoMetadata
	if err := UnmarshalYAMLStrict(contents, &rootMetadata); err != nil {
		return nil, nil, fmt.Errorf("could not parse root metadata: %v", err)
	}
	nsMDs := make(map[string]*NamespaceConfigRepoMetadata)
	for _, namespace := range rootMetadata.Namespaces {
		contents, err := os.ReadFile(filepath.Join(path, namespace, DefaultNamespaceConfigRepoMetadataFileName))
		if err != nil {
			return nil, nil, fmt.Errorf("could not open root metadata: %v", err)
		}
		var nsConfig NamespaceConfigRepoMetadata
		if err := UnmarshalYAMLStrict(contents, &nsConfig); err != nil {
			return nil, nil, fmt.Errorf("could not parse namespace metadata: %v", err)
		}
		if nsConfig.Name != namespace {
			return nil, nil, fmt.Errorf("invalid configuration, namespace in root metadata: %s does not match in-namespace metadata: %s", namespace, nsConfig.Name)
		}
		nsMDs[namespace] = &nsConfig
	}
	return &rootMetadata, nsMDs, nil
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

// NewYAMLDecoderStrict creates a new YAML decoder from the reader.
func NewYAMLDecoderStrict(reader io.Reader) *yaml.Decoder {
	yamlDecoder := yaml.NewDecoder(reader)
	yamlDecoder.KnownFields(true)
	return yamlDecoder
}
