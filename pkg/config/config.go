package config

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

type RootConfig struct {
	Version    string   `json:"version,omitempty" yaml:"version,omitempty"`
	Namespaces []string `json:"namespaces,omitempty" yaml:"namespaces,omitempty"`
}

type NamespaceConfig struct {
	Version string `json:"version,omitempty" yaml:"version,omitempty"`
	Name    string `json:"name,omitempty" yaml:"name,omitempty"`
}

const DefaultRootConfigFileName = "lekko.root.yaml"
const DefaultNamespaceConfigFileName = "lekko.ns.yaml"

// Parses the lekko configuration in a strict way, returning a user error on failure.
//
// This looks in the default locations and names for now, and a custom extension
// can be written later.
//
// This returns the root config of a repository and a map from namespace name to namespace.
func ParseFullConfigStrict(path string) (*RootConfig, map[string]*NamespaceConfig, error) {
	contents, err := os.ReadFile(filepath.Join(path, DefaultRootConfigFileName))
	if err != nil {
		return nil, nil, fmt.Errorf("could not open root config: %v", err)
	}

	var rootConfig RootConfig
	if err := UnmarshalYAMLStrict(contents, &rootConfig); err != nil {
		return nil, nil, fmt.Errorf("could not parse root config: %v", err)
	}
	nsConfigs := make(map[string]*NamespaceConfig)
	for _, namespace := range rootConfig.Namespaces {
		contents, err := os.ReadFile(filepath.Join(path, namespace, DefaultNamespaceConfigFileName))
		if err != nil {
			return nil, nil, fmt.Errorf("could not open root config: %v", err)
		}
		var nsConfig NamespaceConfig
		if err := UnmarshalYAMLStrict(contents, &nsConfig); err != nil {
			return nil, nil, fmt.Errorf("could not parse namespace config: %v", err)
		}
		if nsConfig.Name != namespace {
			return nil, nil, fmt.Errorf("invalid configuration, namespace in root config: %s does not match in-namespace config: %s", namespace, nsConfig.Name)
		}
		nsConfigs[namespace] = &nsConfig
	}
	return &rootConfig, nsConfigs, nil
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
