package sync

import (
	"fmt"
	"os/exec"
	"strings"

	"github.com/pkg/errors"
)

func BisyncTS(lekkoPath, repoPath string) error {
	tsSyncCmd := exec.Command("npx", "lekko-repo-sync", "--lekko-dir", lekkoPath, "--repo-path", repoPath)
	output, err := tsSyncCmd.CombinedOutput()
	outputStr := strings.TrimSpace(string(output))
	if len(outputStr) > 0 {
		fmt.Println(outputStr)
	}
	if err != nil {
		return errors.Wrap(err, "Lekko Typescript tools not found, please make sure that you are inside a node project and have up to date Lekko packages.")
	}
	return nil
}
