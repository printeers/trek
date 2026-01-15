package internal

import (
	"context"
	"fmt"
	"os"
	"os/exec"
)

func PgmodelerExportSQL(ctx context.Context, input, output string) error {
	//nolint:gosec
	err := os.WriteFile(output, []byte{}, 0o644)
	if err != nil {
		return fmt.Errorf("failed to create output file: %w", err)
	}
	//nolint:gosec
	cmdPgModeler := exec.CommandContext(
		ctx,
		"pgmodeler-cli",
		"--input",
		input,
		"--export-to-file",
		"--output",
		output,
		"--pgsql-ver",
		pgversionPgmodeler,
	)
	cmdPgModeler.Stderr = os.Stderr

	out, err := cmdPgModeler.Output()
	if err != nil {
		return fmt.Errorf("failed to run pgmodeler: %w %s", err, string(out))
	}

	return nil
}

func PgmodelerExportPNG(ctx context.Context, input, output string) error {
	//nolint:gosec
	err := os.WriteFile(output, []byte{}, 0o644)
	if err != nil {
		return fmt.Errorf("failed to create output png: %w", err)
	}
	//nolint:gosec
	cmdPgModeler := exec.CommandContext(
		ctx,
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
		return fmt.Errorf("failed to run pgmodeler: %w %s", err, string(out))
	}

	return nil
}

func PgmodelerExportSVG(ctx context.Context, input, output string) error {
	//nolint:gosec
	err := os.WriteFile(output, []byte{}, 0o644)
	if err != nil {
		return fmt.Errorf("failed to create output svg: %w", err)
	}
	//nolint:gosec
	cmdPgModeler := exec.CommandContext(
		ctx,
		"pgmodeler-cli",
		"--input",
		input,
		"--export-to-svg",
		"--output",
		output,
	)
	cmdPgModeler.Stderr = os.Stderr

	out, err := cmdPgModeler.Output()
	if err != nil {
		return fmt.Errorf("failed to run pgmodeler: %w %s", err, string(out))
	}

	return nil
}
