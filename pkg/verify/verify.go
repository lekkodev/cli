package verify

import (
	"os"
	"path/filepath"

	"github.com/lekkodev/cli/pkg/config"
	"github.com/lekkodev/cli/pkg/encoding"
)

// Verifies that a configuration from a root is properly formatted.
func Verify(rootPath string) error {
	_, nsNameToNsConfigs, err := config.ParseFullConfigStrict(rootPath)
	if err != nil {
		return err
	}
	for ns, nsConfig := range nsNameToNsConfigs {
		files, err := os.ReadDir(filepath.Join(rootPath, ns))
		if err != nil {
			return err
		}
		for _, file := range files {
			if file.Name() == config.DefaultNamespaceConfigFileName {
				// Do not parse the yaml config.
				continue
			}

			contents, err := os.ReadFile(filepath.Join(rootPath, ns, file.Name()))
			if err != nil {
				return err
			}
			if _, err := encoding.ParseFeature(contents, nsConfig.Version); err != nil {
				return err
			}
		}
	}
	return nil
}
