package internal

import (
	"errors"
	"log"
	"os"
	"os/exec"
	"path/filepath"
)

func RunHook(wd, hookName string, args ...string) error {
	hooksDir := filepath.Join(wd, "hooks")
	filePath := filepath.Join(hooksDir, hookName)
	if _, err := os.Stat(filePath); errors.Is(err, os.ErrNotExist) {
		log.Printf("Skipping hook %s", hookName)

		return nil
	}

	log.Printf("Running hook %s", hookName)

	cmd := exec.Command(filePath, args...)
	cmd.Dir = hooksDir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	err := cmd.Run()
	if err != nil {
		//nolint:wrapcheck
		return err
	}

	return nil
}
