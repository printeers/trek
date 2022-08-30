package internal

import (
	"fmt"
	"os"
	"strings"

	"github.com/amenzhinsky/go-memexec"

	"github.com/stack11/trek/internal/embed"
)

func Migra(from, to string) (string, error) {
	exe, err := memexec.New(embed.MigraBinary)
	if err != nil {
		return "", fmt.Errorf("failed to load migra binary: %w", err)
	}
	defer exe.Close()

	cmdMigra := exe.Command("--unsafe", "--with-privileges", from, to)
	cmdMigra.Stderr = os.Stderr
	output, err := cmdMigra.Output()
	if err != nil && cmdMigra.ProcessState.ExitCode() != 2 {
		return "", fmt.Errorf("failed to run migra: %w %s", err, string(output))
	}

	return strings.TrimSuffix(string(output), "\n"), nil
}
