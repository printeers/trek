package internal

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"

	"github.com/manifoldco/promptui"
)

const regexpPartialLowerKebabCase = `[a-z][a-z0-9\-]*[a-z]`

var (
	RegexpMigrationName     = regexp.MustCompile(`^` + regexpPartialLowerKebabCase + `$`)
	RegexpMigrationFileName = regexp.MustCompile(`^\d{3}_` + regexpPartialLowerKebabCase + `\.up\.sql$`)
)

func GetMigrationsDir(wd string) (string, error) {
	migrationsDir := filepath.Join(wd, "migrations")
	if _, err := os.Stat(migrationsDir); os.IsNotExist(err) {
		err = os.Mkdir(migrationsDir, 0o755)
		if err != nil {
			return "", fmt.Errorf("failed to create migrations directory: %w", err)
		}
	}

	return migrationsDir, nil
}

func GetMigrationFileName(migrationNumber uint, migrationName string) string {
	return fmt.Sprintf("%03d_%s.up.sql", migrationNumber, migrationName)
}

func InspectMigrations(migrationsDir string) (migrationsCount uint, err error) {
	err = filepath.WalkDir(migrationsDir, func(path string, d fs.DirEntry, err error) error {
		if path == migrationsDir {
			return nil
		}
		if d.IsDir() {
			return filepath.SkipDir
		}
		if !RegexpMigrationFileName.MatchString(d.Name()) {
			//nolint:goerr113
			return fmt.Errorf("invalid existing migration filename %q", d.Name())
		}
		migrationsCount++

		return nil
	})

	//nolint:wrapcheck
	return migrationsCount, err
}

func GetNewMigrationFilePath(
	migrationsDir string,
	migrationName string,
	overwrite bool,
) (
	path string,
	migrationNumber uint,
	err error,
) {
	migrationsCount, err := InspectMigrations(migrationsDir)
	if err != nil {
		return "", 0, fmt.Errorf("failed to inspect migrations directory: %w", err)
	}

	var migrationsNumber uint
	if _, err = os.Stat(
		filepath.Join(migrationsDir, GetMigrationFileName(migrationsCount, migrationName)),
	); err == nil {
		if overwrite {
			migrationsNumber = migrationsCount
		} else {
			prompt := promptui.Prompt{
				//nolint:lll
				Label:     "The previous migration has the same name. Overwrite the previous migration instead of creating a new one",
				IsConfirm: true,
				Default:   "y",
			}
			if _, err = prompt.Run(); err == nil {
				migrationsNumber = migrationsCount
			} else {
				migrationsNumber = migrationsCount + 1
			}
		}
	} else {
		migrationsNumber = migrationsCount + 1
	}

	return filepath.Join(migrationsDir, GetMigrationFileName(migrationsNumber, migrationName)), migrationsNumber, nil
}
