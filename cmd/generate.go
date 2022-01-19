package cmd

import (
	"errors"
	"fmt"
	"io/fs"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"text/template"
	"time"

	"github.com/golang-migrate/migrate/v4"
	"github.com/manifoldco/promptui"
	"github.com/spf13/cobra"
	"github.com/stack11/trek/internal"
)

var (
	//nolint:gochecknoglobals
	flagDev bool
	//nolint:gochecknoglobals
	flagCleanup bool
	//nolint:gochecknoglobals
	flagOverwriteYes bool
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
	generateCmd.Flags().BoolVar(
		&flagOverwriteYes,
		"overwrite-yes",
		false,
		"Overwrite existing files",
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
		newMigrationFilePath, migrationNumber, err := getNewMigrationFilePath(migrationName)
		if err != nil {
			log.Fatalf("Failed to get new migration file path: %v\n", err)
		}

		defer func() {
			if flagDev && flagCleanup {
				if _, err = os.Stat(newMigrationFilePath); err == nil {
					err = os.Remove(newMigrationFilePath)
					if err != nil {
						log.Printf("failed to delete new migration file: %v\n", err)
					}
				}
			}
		}()

		err = run(config, newMigrationFilePath, migrationNumber)
		if err != nil {
			log.Fatalf("Failed to run: %v\n", err)
		}

		if flagDev {
			for {
				time.Sleep(time.Millisecond * 100)
				err = run(config, newMigrationFilePath, migrationNumber)
				if err != nil {
					log.Fatalf("Failed to run: %v\n", err)
				}
			}
		}
	},
}

type PostgresFunction = func(targetContainerID, migrateContainerID string) error

func run(
	config *internal.Config,
	newMigrationFilePath string,
	migrationNumber uint,
) error {
	targetContainerID, err := internal.DockerRunPostgresContainer()
	if err != nil {
		return fmt.Errorf("failed to create target container: %w", err)
	}
	migrateContainerID, err := internal.DockerRunPostgresContainer()
	if err != nil {
		return fmt.Errorf("failed to create migrate container: %w", err)
	}

	defer func() {
		internal.DockerKillContainer(targetContainerID)
		internal.DockerKillContainer(migrateContainerID)
	}()

	updated, err := generateMigrationFile(
		config,
		newMigrationFilePath,
		migrationNumber == 1,
		targetContainerID,
		migrateContainerID,
	)
	if err != nil && !errors.Is(err, ErrInvalidModel) {
		return fmt.Errorf("failed to generate migration file: %w", err)
	}

	if updated {
		log.Println("Wrote migration file")

		err = writeTemplateFiles(config, migrationNumber)
		if err != nil {
			return fmt.Errorf("failed to write template files: %w", err)
		}

		updated, err = generateDiffLockFile(config, newMigrationFilePath, targetContainerID, migrateContainerID)
		if err != nil {
			return fmt.Errorf("failed to generate diff lock file: %w", err)
		}

		if updated {
			log.Println("Wrote diff lock file")
		}
	}

	return nil
}

func generateDiffLockFile(
	config *internal.Config,
	newMigrationFilePath,
	targetContainerID,
	migrateContainerID string,
) (bool, error) {
	migrateIP, err := internal.DockerGetContainerIP(migrateContainerID)
	if err != nil {
		//nolint:wrapcheck
		return false, err
	}

	err = internal.PsqlFile(
		migrateIP,
		internal.PGDefaultUsername,
		internal.PGDefaultPassword,
		"disable",
		config.DatabaseName,
		newMigrationFilePath,
	)
	if err != nil {
		return false, fmt.Errorf("failed to apply generated migration: %w", err)
	}

	var diff string
	diff, err = diffSchemaDumps(config, targetContainerID, migrateContainerID)
	if err != nil {
		return false, fmt.Errorf("failed to diff schema dumps: %w", err)
	}

	var wd string
	wd, err = os.Getwd()
	if err != nil {
		return false, fmt.Errorf("failed to get working directory: %w", err)
	}

	lockfile := filepath.Join(wd, "diff.lock")
	hasStoredDiff := false
	var storedDiff string

	if _, err = os.Stat(lockfile); err == nil {
		hasStoredDiff = true
		var s []byte
		s, err = os.ReadFile(lockfile)
		if err != nil {
			return false, fmt.Errorf("failed to read diff.lock file: %w", err)
		}
		storedDiff = string(s)
	}

	if !hasStoredDiff || diff != storedDiff {
		err = os.WriteFile(lockfile, []byte(diff), 0o600)
		if err != nil {
			return false, fmt.Errorf("failed to write diff.lock file: %w", err)
		}

		return true, nil
	}

	return false, nil
}

func diffSchemaDumps(config *internal.Config, targetContainerID, migrateContainerID string) (string, error) {
	targetIP, err := internal.DockerGetContainerIP(targetContainerID)
	if err != nil {
		//nolint:wrapcheck
		return "", err
	}

	migrateIP, err := internal.DockerGetContainerIP(migrateContainerID)
	if err != nil {
		//nolint:wrapcheck
		return "", err
	}

	pgDumpOptions := []string{
		"--schema-only",
		"--exclude-table=public.schema_migrations",
	}

	targetDump, err := internal.PgDump(targetIP,
		internal.PGDefaultUsername,
		internal.PGDefaultPassword,
		"disable",
		config.DatabaseName,
		pgDumpOptions,
	)
	if err != nil {
		//nolint:wrapcheck
		return "", err
	}

	migrateDump, err := internal.PgDump(migrateIP,
		internal.PGDefaultUsername,
		internal.PGDefaultPassword,
		"disable",
		config.DatabaseName,
		pgDumpOptions,
	)
	if err != nil {
		//nolint:wrapcheck
		return "", err
	}

	targetDumpFile := "/tmp/target.sql"
	err = os.WriteFile(targetDumpFile, []byte(cleanDump(targetDump)), 0o600)
	if err != nil {
		return "", fmt.Errorf("failed to write target.sql file: %w", err)
	}

	migrateDumpFile := "/tmp/migrate.sql"
	err = os.WriteFile(migrateDumpFile, []byte(cleanDump(migrateDump)), 0o600)
	if err != nil {
		return "", fmt.Errorf("failed to write migrate.sql file: %w", err)
	}

	gitDiffCmd := exec.Command(
		"git",
		"diff",
		migrateDumpFile,
		targetDumpFile,
	)
	gitDiffCmd.Stderr = os.Stderr

	output, err := gitDiffCmd.Output()
	if err != nil {
		var ee *exec.ExitError
		if !(errors.As(err, &ee) && ee.ExitCode() <= 1) {
			return "", fmt.Errorf("failed to run git diff: %w %v", err, string(output))
		}
	}

	return string(output), nil
}

func cleanDump(dump string) string {
	var lines []string
	for _, line := range strings.Split(dump, "\n") {
		if line != "" && !strings.HasPrefix(line, "--") {
			lines = append(lines, line)
		}
	}

	return strings.Join(lines, "\n")
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

func getNewMigrationFilePath(migrationName string) (path string, migrationNumber uint, err error) {
	wd, err := os.Getwd()
	if err != nil {
		return "", 0, fmt.Errorf("failed to get working directory: %w", err)
	}
	migrationsDir := filepath.Join(wd, "migrations")
	if _, err = os.Stat(migrationsDir); os.IsNotExist(err) {
		err = os.Mkdir(migrationsDir, 0o755)
		if err != nil {
			return "", 0, fmt.Errorf("failed to create migrations directory: %w", err)
		}
	}

	migrationsCount, err := inspectMigrations(migrationsDir)
	if err != nil {
		return "", 0, fmt.Errorf("failed to inspect migrations directory: %w", err)
	}

	var migrationsNumber uint
	if _, err = os.Stat(
		filepath.Join(wd, "migrations", getMigrationFileName(migrationsCount, migrationName)),
	); err == nil {
		if flagOverwriteYes {
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

	return filepath.Join(wd, "migrations", getMigrationFileName(migrationsNumber, migrationName)), migrationsNumber, nil
}

func getMigrationFileName(migrationNumber uint, migrationName string) string {
	return fmt.Sprintf("%03d_%s.up.sql", migrationNumber, migrationName)
}

var (
	//nolint:gochecknoglobals
	modelContent = ""
)

func exportToPng(config *internal.Config, wd string) {
	err := internal.PgModelerExportToPng(
		filepath.Join(wd, fmt.Sprintf("%s.dbm", config.ModelName)),
		filepath.Join(wd, fmt.Sprintf("%s.png", config.ModelName)),
	)
	if err != nil {
		log.Println(err)
	}
}

var ErrInvalidModel = errors.New("invalid model")

//nolint:cyclop
func generateMigrationFile(
	config *internal.Config,
	newMigrationFilePath string,
	initial bool,
	targetContainerID,
	migrateContainerID string,
) (updated bool, err error) {
	wd, err := os.Getwd()
	if err != nil {
		return false, fmt.Errorf("failed to get working directory: %w", err)
	}

	m, err := os.ReadFile(filepath.Join(wd, fmt.Sprintf("%s.dbm", config.ModelName)))
	if err != nil {
		return false, fmt.Errorf("failed to read model file: %w", err)
	}
	mStr := strings.TrimSuffix(string(m), "\n")
	if mStr == "" || mStr == modelContent {
		return false, nil
	}
	log.Println("Generating migration file")
	modelContent = mStr

	go exportToPng(config, wd)

	err = internal.PgModelerExportToFile(
		filepath.Join(wd, fmt.Sprintf("%s.dbm", config.ModelName)),
		filepath.Join(wd, fmt.Sprintf("%s.sql", config.ModelName)),
	)
	if err != nil {
		log.Println(err)

		return false, ErrInvalidModel
	}

	if initial {
		// Verify the schema is correct by applying it to the database
		_, err = setupTargetDatabase(config, targetContainerID)
		if err != nil {
			log.Println(err)

			return false, ErrInvalidModel
		}
		_, err = setupDatabase(migrateContainerID, config)
		if err != nil {
			return false, fmt.Errorf("failed to setup database: %w", err)
		}

		// If we are developing the schema initially, there will be no diffs,
		// and we want to copy over the schema file to the initial migration file
		var input []byte
		input, err = os.ReadFile(filepath.Join(wd, fmt.Sprintf("%s.sql", config.ModelName)))
		if err != nil {
			return false, fmt.Errorf("failed to read sql file: %w", err)
		}

		//nolint:gosec
		err = os.WriteFile(newMigrationFilePath, input, 0o644)
		if err != nil {
			return false, fmt.Errorf("failed to write migration file: %w", err)
		}

		return true, nil
	}

	migrateDSN, err := setupMigrateDatabase(config, newMigrationFilePath, migrateContainerID)
	if err != nil {
		return false, fmt.Errorf("failed to setup migrate database: %w", err)
	}

	targetDSN, err := setupTargetDatabase(config, targetContainerID)
	if err != nil {
		log.Println(err)

		return false, ErrInvalidModel
	}

	diff, err := internal.Migra(migrateDSN, targetDSN)
	if err != nil {
		log.Println(err)

		return false, ErrInvalidModel
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
		return false, fmt.Errorf("failed to write migration file: %w", err)
	}

	return true, nil
}

func setupMigrateDatabase(config *internal.Config, newMigrationFilePath, migrateContainerID string) (string, error) {
	wd, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("failed to get working directory: %w", err)
	}

	targetIP, err := setupDatabase(migrateContainerID, config)
	if err != nil {
		return "", fmt.Errorf("failed to setup database: %w", err)
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
		return "", fmt.Errorf("failed to get working directory: %w", err)
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
