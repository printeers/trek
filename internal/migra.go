package internal

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/stack11/trek/internal/embed"
)

func Migra(from, to string) (string, error) {
	outBinary := "/tmp/migra"
	if _, err := os.Stat(outBinary); errors.Is(err, os.ErrNotExist) {
		//nolint:gosec
		err = os.WriteFile(outBinary, embed.MigraBinary, 0o700)
		if err != nil {
			return "", fmt.Errorf("failed to extract migra binary: %w", err)
		}
	}

	cmdMigra := exec.Command(outBinary, "--unsafe", "--with-privileges", from, to)
	cmdMigra.Stderr = os.Stderr
	output, err := cmdMigra.Output()
	if err != nil && cmdMigra.ProcessState.ExitCode() != 2 {
		return "", fmt.Errorf("failed to run migra: %w %v", err, string(output))
	}

	return strings.TrimSuffix(string(output), "\n"), nil
}
