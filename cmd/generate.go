package cmd

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"text/template"
	"time"

	"github.com/golang-migrate/migrate/v4"
	"github.com/spf13/cobra"
	"github.com/thecodeteam/goodbye"

	"github.com/stack11/trek/internal"
)

var (
	//nolint:gochecknoglobals
	flagDev bool
	//nolint:gochecknoglobals
	flagCleanup bool
)

const (
	regexpPartialLowerKebabCase = `[a-z][a-z0-9\-]*[a-z]`
)

//nolint:gochecknoinits
func init() {
	generateCmd.Flags().BoolVar(
		&flagDev,
		"dev",
		false,
		"Watch for file changes and automatically regenerate the migration file",
	)
	generateCmd.Flags().BoolVar(
		&flagCleanup,
		"cleanup",
		true,
		"Remove the generated migrations file. Only works with --dev",
	)
}

//nolint:gochecknoglobals
var generateCmd = &cobra.Command{
	Use:   "generate [migration-name]",
	Short: "Generate the migrations for a pgModeler file",
	Args: func(cmd *cobra.Command, args []string) error {
		if len(args) == 0 {
			//nolint:goerr113
			return errors.New("pass the name of the migration")
		} else if len(args) > 1 {
			//nolint:goerr113
			return errors.New("expecting one migration name, use lower-kebab-case for the migration name")
		}

		var regexpLowerKebabCase = regexp.MustCompile(`^` + regexpPartialLowerKebabCase + `$`)
		if !regexpLowerKebabCase.MatchString(args[0]) {
			//nolint:goerr113
			return errors.New("migration name must be lower-kebab-case and must not start or end with a number or dash")
		}

		return nil
	},
	Run: func(cmd *cobra.Command, args []string) {
		internal.AssertGenerateToolsAvailable()

		config, err := internal.ReadConfig()
		if err != nil {
			log.Fatalf("Failed to read config: %v\n", err)
		}

		migrationName := args[0]
		newMigrationFilePath, migrationNumber := getNewMigrationFilePath(migrationName)
		initial := migrationNumber == 0

		ctx := context.Background()
		defer goodbye.Exit(ctx, -1)
		goodbye.Notify(ctx)
		goodbye.Register(func(ctx context.Context, sig os.Signal) {
			internal.DockerKillContainer(targetContainerID)
			internal.DockerKillContainer(migrateContainerID)

			if flagDev && flagCleanup {
				if _, err = os.Stat(newMigrationFilePath); err == nil {
					err = os.Remove(newMigrationFilePath)
					if err != nil {
						log.Fatalf("failed to delete new migration file: %v\n", err)
					}
				}
			}
		})

		if updateDiff(config, newMigrationFilePath, initial) {
			err = writeTemplateFiles(config, migrationNumber)
			if err != nil {
				log.Fatalf("Failed to write template files: %v\n", err)
			}
		}

		if flagDev {
			for {
				time.Sleep(time.Millisecond * 100)
				if updateDiff(config, newMigrationFilePath, initial) {
					err = writeTemplateFiles(config, migrationNumber)
					if err != nil {
						log.Fatalf("Failed to write template files: %v\n", err)
					}
				}
			}
		}
	},
}

func writeTemplateFiles(config *internal.Config, newVersion uint) error {
	for _, ts := range config.Templates {
		t, err := template.New(ts.Path).Parse(ts.Content)
		if err != nil {
			return fmt.Errorf("failed to parse template: %w", err)
		}

		dir := filepath.Dir(ts.Path)
		err = os.MkdirAll(dir, 0o755)
		if err != nil {
			return fmt.Errorf("failed to create %s: %w", dir, err)
		}

		f, err := os.Create(ts.Path)
		if err != nil {
			return fmt.Errorf("failed to create file %s: %w", ts.Path, err)
		}

		err = t.Execute(f, map[string]interface{}{"NewVersion": newVersion})
		if err != nil {
			return fmt.Errorf("failed to execute template: %w", err)
		}
	}

	return nil
}

func getNewMigrationFilePath(migrationName string) (path string, migrationNumber uint) {
	wd, err := os.Getwd()
	if err != nil {
		log.Fatalf("Failed to get working directory: %v\n", err)
	}
	migrationsDir := filepath.Join(wd, "migrations")
	if _, err = os.Stat(migrationsDir); os.IsNotExist(err) {
		err = os.Mkdir(migrationsDir, 0o755)
		if err != nil {
			log.Fatalf("Failed to create migrations directory: %v\n", err)
		}
	}

	migrationsCount, err := inspectMigrations(migrationsDir)
	if err != nil {
		log.Fatalf("Error when inspecting the migrations directory: %v\n", err)
	}
	var newMigrationFileName = fmt.Sprintf("%03d_%s.up.sql", migrationsCount+1, migrationName)
	var newMigrationFilePath = filepath.Join(wd, "migrations", newMigrationFileName)

	return newMigrationFilePath, migrationsCount
}

var (
	//nolint:gochecknoglobals
	modelContent = ""
	//nolint:gochecknoglobals
	targetContainerID = ""
	//nolint:gochecknoglobals
	migrateContainerID = ""
)

//nolint:cyclop
func updateDiff(config *internal.Config, newMigrationFilePath string, initial bool) bool {
	wd, err := os.Getwd()
	if err != nil {
		log.Fatalf("Failed to get working directory: %v\n", err)
	}

	m, err := os.ReadFile(filepath.Join(wd, fmt.Sprintf("%s.dbm", config.ModelName)))
	if err != nil {
		log.Fatalln(err)
	}
	mStr := strings.TrimSuffix(string(m), "\n")
	if mStr == "" || mStr == modelContent {
		return false
	}
	modelContent = mStr

	targetContainerID, err = internal.DockerRunPostgresContainer()
	if err != nil {
		log.Fatalf("Failed to create target container: %v\n", err)
	}
	migrateContainerID, err = internal.DockerRunPostgresContainer()
	if err != nil {
		log.Fatalf("Failed to create migrate container: %v\n", err)
	}

	defer func() {
		internal.DockerKillContainer(targetContainerID)
		internal.DockerKillContainer(migrateContainerID)
	}()

	go func() {
		err = internal.PgModelerExportToPng(
			filepath.Join(wd, fmt.Sprintf("%s.dbm", config.ModelName)),
			filepath.Join(wd, fmt.Sprintf("%s.png", config.ModelName)),
		)
		if err != nil {
			log.Panicln(err)
		}
	}()

	err = internal.PgModelerExportToFile(
		filepath.Join(wd, fmt.Sprintf("%s.dbm", config.ModelName)),
		filepath.Join(wd, fmt.Sprintf("%s.sql", config.ModelName)),
	)
	if err != nil {
		log.Panicln(err)
	}

	if initial {
		// Verify the schema is correct by applying it to the database
		_, err = setupTargetDatabase(config, targetContainerID)
		if err != nil {
			log.Panicln(fmt.Errorf("failed to validate initial schema: %w", err))
		}

		// If we are developing the schema initially, there will be no diffs,
		// and we want to copy over the schema file to the initial migration file
		var input []byte
		input, err = os.ReadFile(filepath.Join(wd, fmt.Sprintf("%s.sql", config.ModelName)))
		if err != nil {
			log.Panicln(err)
		}

		//nolint:gosec
		err = os.WriteFile(newMigrationFilePath, input, 0o644)
		if err != nil {
			log.Panicln(err)
		}

		return true
	}

	migrateDSN, err := setupMigrateDatabase(config, newMigrationFilePath, migrateContainerID)
	if err != nil {
		log.Panicln(err)
	}

	targetDSN, err := setupTargetDatabase(config, targetContainerID)
	if err != nil {
		log.Panicln(err)
	}

	diff, err := internal.Migra(migrateDSN, targetDSN)
	if err != nil {
		log.Panicln(err)
	}

	// Filter stuff from go-migrate that doesn't exist in the target db, and we don't have and need anyway
	diff = strings.ReplaceAll(
		diff,
		"alter table \"public\".\"schema_migrations\" drop constraint \"schema_migrations_pkey\";",
		"",
	)
	diff = strings.ReplaceAll(
		diff,
		"drop index if exists \"public\".\"schema_migrations_pkey\";",
		"",
	)
	diff = strings.ReplaceAll(
		diff,
		"drop table \"public\".\"schema_migrations\";",
		"",
	)
	diff = strings.Trim(diff, "\n")

	var lines []string
	for _, line := range strings.Split(diff, "\n") {
		if line != "" {
			lines = append(lines, line)
		}
	}
	diff = strings.Join(lines, "\n") + "\n"

	//nolint:gosec
	err = os.WriteFile(
		newMigrationFilePath,
		[]byte(diff),
		0o644,
	)
	if err != nil {
		log.Panicln(err)
	}

	log.Println("Wrote migration file")

	return true
}

func setupMigrateDatabase(config *internal.Config, newMigrationFilePath, migrateContainerID string) (string, error) {
	wd, err := os.Getwd()
	if err != nil {
		log.Fatalf("Failed to get working directory: %v\n", err)
	}

	targetIP, err := setupDatabase(migrateContainerID, config)
	if err != nil {
		return "", err
	}

	if _, err = os.Stat(newMigrationFilePath); err == nil {
		err = os.Remove(newMigrationFilePath)
		if err != nil {
			return "", fmt.Errorf("failed to delete generated migration file: %w", err)
		}
	}

	migrateDSN := fmt.Sprintf("postgresql://postgres:postgres@%s:5432/%s?sslmode=disable", targetIP, config.DatabaseName)
	m, err := migrate.New(fmt.Sprintf("file://%s", filepath.Join(wd, "migrations")), migrateDSN)
	if err != nil {
		return "", fmt.Errorf("failed to create migrate: %w", err)
	}
	err = m.Up()
	if err != nil {
		return "", fmt.Errorf("failed to up migrations: %w", err)
	}

	return migrateDSN, nil
}

func setupTargetDatabase(config *internal.Config, targetContainerID string) (string, error) {
	wd, err := os.Getwd()
	if err != nil {
		log.Fatalf("Failed to get working directory: %v\n", err)
	}

	targetIP, err := setupDatabase(targetContainerID, config)
	if err != nil {
		return "", fmt.Errorf("failed to setup database: %w", err)
	}

	err = internal.PsqlFile(
		targetIP,
		internal.PGDefaultUsername,
		internal.PGDefaultPassword,
		"disable",
		config.DatabaseName,
		filepath.Join(wd, fmt.Sprintf("%s.sql", config.ModelName)),
	)
	if err != nil {
		return "", fmt.Errorf("failed to apply model: %w", err)
	}

	return fmt.Sprintf("postgresql://postgres:postgres@%s:5432/%s?sslmode=disable", targetIP, config.DatabaseName), nil
}

func setupDatabase(containerName string, config *internal.Config) (containerIP string, err error) {
	ip, err := internal.DockerGetContainerIP(containerName)
	if err != nil {
		return "", fmt.Errorf("failed to get container IP: %w", err)
	}

	internal.PsqlWaitDatabaseUp(ip, internal.PGDefaultUsername, internal.PGDefaultPassword, "disable")
	err = internal.PsqlHelperSetupDatabaseAndUsers(
		ip,
		internal.PGDefaultUsername,
		internal.PGDefaultPassword,
		"disable",
		config.DatabaseName,
		config.DatabaseUsers,
	)
	if err != nil {
		return "", fmt.Errorf("failed to setup database: %w", err)
	}

	return ip, nil
}

func inspectMigrations(migrationsDir string) (migrationsCount uint, err error) {
	err = filepath.WalkDir(migrationsDir, func(path string, d fs.DirEntry, err error) error {
		if path == migrationsDir {
			return nil
		}
		if d.IsDir() {
			return filepath.SkipDir
		}
		var regexpValidMigrationFilename = regexp.MustCompile(`^\d{3}_` + regexpPartialLowerKebabCase + `\.up\.sql$`)
		if !regexpValidMigrationFilename.MatchString(d.Name()) {
			//nolint:goerr113
			return fmt.Errorf("invalid existing migration filename %q", d.Name())
		}
		migrationsCount++

		return nil
	})

	//nolint:wrapcheck
	return migrationsCount, err
}
