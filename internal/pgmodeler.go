package internal

import (
	"fmt"
	"os"
	"os/exec"
)

func PgModelerExportToFile(input, output string) error {
	//nolint:gosec
	err := os.WriteFile(output, []byte{}, 0o644)
	if err != nil {
		return fmt.Errorf("failed to create output file: %w", err)
	}
	//nolint:gosec
	cmdPgModeler := exec.Command(
		"pgmodeler-cli",
		"--input",
		input,
		"--export-to-file",
		"--output",
		output,
	)
	cmdPgModeler.Stderr = os.Stderr

	out, err := cmdPgModeler.Output()
	if err != nil {
		return fmt.Errorf("failed to run pgmodeler: %w %v", err, string(out))
	}

	return nil
}

func PgModelerExportToPng(input, output string) error {
	//nolint:gosec
	err := os.WriteFile(output, []byte{}, 0o644)
	if err != nil {
		return fmt.Errorf("failed to create output png: %w", err)
	}
	//nolint:gosec
	cmdPgModeler := exec.Command(
		"pgmodeler-cli",
		"--input",
		input,
		"--export-to-png",
		"--output",
		output,
	)
	cmdPgModeler.Stderr = os.Stderr

	out, err := cmdPgModeler.Output()
	if err != nil {
		return fmt.Errorf("failed to run pgmodeler: %w %v", err, string(out))
	}

	return nil
}
