package star

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/pkg/errors"
)

type bufImage struct {
	filename string
}

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
