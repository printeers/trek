package internal

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/printeers/trek/internal/embedded/migra"
)

func Migra(from, to string) (string, error) {
	migraPath, err := migra.Path()
	if err != nil {
		return "", fmt.Errorf("failed to get migra path: %w", err)
	}
	cmdMigra := exec.Command(migraPath, "--unsafe", "--with-privileges", from, to)
	cmdMigra.Stderr = os.Stderr
	output, err := cmdMigra.Output()
	if err != nil && cmdMigra.ProcessState.ExitCode() != 2 {
		return "", fmt.Errorf("failed to run migra: %w %s", err, string(output))
	}

	return strings.TrimSuffix(string(output), "\n"), nil
}
