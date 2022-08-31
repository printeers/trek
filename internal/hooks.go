package internal

import (
	"errors"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
)

func RunHook(wd, hookName string, options *HookOptions) error {
	hooksDir := filepath.Join(wd, "hooks")
	filePath := filepath.Join(hooksDir, hookName)
	if _, err := os.Stat(filePath); errors.Is(err, os.ErrNotExist) {
		log.Printf("Skipping hook %q", hookName)

		return nil
	}

	log.Printf("Running hook %q", hookName)

	var args []string
	env := os.Environ()
	if options != nil {
		args = append(args, options.Args...)

		var envValues []string
		for key, value := range options.Env {
			envValues = append(envValues, fmt.Sprintf("%s=%s", key, value))
		}
		env = append(env, envValues...)
	}

	cmd := exec.Command(filePath, args...)
	cmd.Env = env
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

type HookOptions struct {
	Args []string
	Env  map[string]string
}
