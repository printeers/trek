package internal

import (
	"fmt"
	"os"
	"os/exec"
)

const PgmodelerCliDockerImage = "geertjohan/pgmodeler-cli:latest"

func PgModelerExportToFile(input, output string) error {
	//nolint:gosec
	err := os.WriteFile(output, []byte{}, 0o644)
	if err != nil {
		return fmt.Errorf("failed to create output file: %w", err)
	}
	//nolint:gosec
	cmdPgModeler := exec.Command(
		"docker",
		"run",
		"--rm",
		"-v",
		fmt.Sprintf("%s:%s", input, input),
		"-v",
		fmt.Sprintf("%s:%s:Z", output, output),
		PgmodelerCliDockerImage,
		"pgmodeler-cli",
		"--input",
		input,
		"--export-to-file",
		"--output",
		output,
	)
	cmdPgModeler.Stderr = os.Stderr
	cmdPgModeler.Stdout = os.Stdout

	err = cmdPgModeler.Run()
	if err != nil {
		return fmt.Errorf("failed to run pgmodeler: %w", err)
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
		"docker",
		"run",
		"--rm",
		"-v",
		fmt.Sprintf("%s:%s", input, input),
		"-v",
		fmt.Sprintf("%s:%s:Z", output, output),
		PgmodelerCliDockerImage,
		"pgmodeler-cli",
		"--input",
		input,
		"--export-to-png",
		"--output",
		output,
	)
	cmdPgModeler.Stderr = os.Stderr
	cmdPgModeler.Stdout = os.Stdout

	err = cmdPgModeler.Run()
	if err != nil {
		return fmt.Errorf("failed to run pgmodeler: %w", err)
	}

	return nil
}
