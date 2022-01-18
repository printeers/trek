package internal

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
)

const MigraDockerImage = "ghcr.io/stack11/trek/migra:latest"

func Migra(from, to string) (string, error) {
	cmdMigra := exec.Command("docker", "run", "--rm", MigraDockerImage, "migra", "--unsafe", "--with-privileges", from, to)
	cmdMigra.Stderr = os.Stderr
	output, err := cmdMigra.Output()
	if err != nil && cmdMigra.ProcessState.ExitCode() != 2 {
		return "", fmt.Errorf("failed to run migra: %w %v", err, string(output))
	}

	return strings.TrimSuffix(string(output), "\n"), nil
}
