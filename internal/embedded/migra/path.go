package migra

import (
	_ "embed"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

const commandName = "migra"

//nolint:gochecknoglobals
var ForceEmbedded bool

//go:embed migra
var MigraBinary []byte

// Path will try to locate the command in the path. If it is not found,
// it will install the binary into the trek cache folder and return the path to
// the binary. If an environment variable is set, it will force install the
// binary.
func Path() (string, error) {
	if !ForceEmbedded {
		// Try to find the command in the path, but only if the environment
		// variable to force using the embedded binary is not set.
		path, err := exec.LookPath(commandName)
		if err == nil {
			return path, nil
		}
	}

	// Install the embedded binary and return the path to the binary.
	return installAndGetPath()
}

// installAndGetPath installs the embedded migra binary into the trek cache
// folder and returns the path to the binary.
func installAndGetPath() (string, error) {
	cachePath, err := os.UserCacheDir()
	if err != nil {
		return "", fmt.Errorf("failed to get user cache dir: %w", err)
	}

	trekCachePath := filepath.Join(cachePath, "trek")
	_, err = os.Stat(trekCachePath)
	if errors.Is(err, os.ErrNotExist) {
		err = os.Mkdir(trekCachePath, 0o775)
		if err != nil {
			return "", fmt.Errorf("failed to create trek cache dir: %w", err)
		}
	} else if err != nil {
		return "", fmt.Errorf("failed to stat trek cache dir: %w", err)
	}

	executablePath := filepath.Join(trekCachePath, commandName)
	_, err = os.Stat(executablePath)
	if errors.Is(err, os.ErrNotExist) {
		//nolint:gosec
		err = os.WriteFile(executablePath, MigraBinary, 0o775)
		if err != nil {
			return "", fmt.Errorf("failed to install executable %s: %w", commandName, err)
		}
	} else if err != nil {
		return "", fmt.Errorf("failed to stat executable %s: %w", commandName, err)
	}

	return executablePath, nil
}
