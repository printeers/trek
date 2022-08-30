package cmd

import (
	"context"
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

	embeddedpostgres "github.com/fergusstrange/embedded-postgres"
	"github.com/golang-migrate/migrate/v4"
	"github.com/jackc/pgx/v4"
	"github.com/manifoldco/promptui"
	"github.com/spf13/cobra"

	"github.com/stack11/trek/internal"
)

const (
	regexpPartialLowerKebabCase = `[a-z][a-z0-9\-]*[a-z]`
)

//nolint:gocognit,cyclop
func NewGenerateCommand() *cobra.Command {
	var (
		dev       bool
		cleanup   bool
		overwrite bool
		stdout    bool
	)

	generateCmd := &cobra.Command{
		Use:   "generate [migration-name]",
		Short: "Generate the migrations for a pgModeler file",
		PersistentPreRun: func(cmd *cobra.Command, args []string) {
			internal.InitializeFlags(cmd)
		},
		Args: func(cmd *cobra.Command, args []string) error {
			if stdout {
				if len(args) != 0 {
					//nolint:goerr113
					return errors.New("pass no name for stdout generation")
				}
			} else {
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
			}

			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := context.Background()

			config, err := internal.ReadConfig()
			if err != nil {
				return fmt.Errorf("failed to read config: %w", err)
			}

			wd, err := os.Getwd()
			if err != nil {
				return fmt.Errorf("failed to get working directory: %w", err)
			}

			var runnerFunc func() error

			if stdout {
				var migrationsDir string
				migrationsDir, err = getMigrationsDir(wd)
				if err != nil {
					return fmt.Errorf("failed to get migrations directory: %w", err)
				}
				var migrationsCount uint
				migrationsCount, err = inspectMigrations(migrationsDir)
				if err != nil {
					return fmt.Errorf("failed to get inspect migrations directory: %w", err)
				}

				initial := migrationsCount == 0

				runnerFunc = func() error {
					return runWithStdout(ctx, config, wd, initial)
				}
			} else {
				migrationName := args[0]
				var newMigrationFilePath string
				var migrationNumber uint
				newMigrationFilePath, migrationNumber, err = getNewMigrationFilePath(wd, migrationName, overwrite)
				if err != nil {
					return fmt.Errorf("failed to get new migration file path: %w", err)
				}

				defer func() {
					if dev && cleanup {
						if _, err = os.Stat(newMigrationFilePath); err == nil {
							err = os.Remove(newMigrationFilePath)
							if err != nil {
								log.Printf("Failed to delete new migration file: %v\n", err)
							}
						}
					}
				}()

				runnerFunc = func() error {
					return runWithFile(ctx, config, wd, newMigrationFilePath, migrationNumber)
				}
			}

			err = runnerFunc()
			if err != nil {
				log.Printf("Failed to run: %v\n", err)
			}

			if dev {
				for {
					time.Sleep(time.Millisecond * 100)
					err = runnerFunc()
					if err != nil {
						log.Printf("Failed to run: %v\n", err)
					}
				}
			}

			return nil
		},
	}

	generateCmd.Flags().BoolVar(&dev, "dev", false, "Watch for file changes and automatically regenerate the migration file") //nolint:lll
	generateCmd.Flags().BoolVar(&cleanup, "cleanup", true, "Remove the generated migrations file. Only works with --dev")
	generateCmd.Flags().BoolVar(&overwrite, "overwrite", false, "Overwrite existing files")
	generateCmd.Flags().BoolVar(&stdout, "stdout", false, "Output migration statements to stdout")

	return generateCmd
}

func setupDatabase(
	ctx context.Context,
	name string,
	port uint32,
) (
	*embeddedpostgres.EmbeddedPostgres,
	*pgx.Conn,
	error,
) {
	postgres, dsn := internal.NewPostgresDatabase(fmt.Sprintf("/tmp/trek/%s", name), port)
	err := postgres.Start()
	if err != nil {
		return nil, nil, fmt.Errorf("failed to start %q database: %w", name, err)
	}
	conn, err := pgx.Connect(ctx, dsn)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to connect to %q database: %w", name, err)
	}

	return postgres, conn, nil
}

//nolint:gocognit,cyclop
func runWithStdout(
	ctx context.Context,
	config *internal.Config,
	wd string,
	initial bool,
) error {
	updated, err := checkIfUpdated(config, wd)
	if err != nil {
		return fmt.Errorf("failed to check if model has been updated: %w", err)
	}
	if updated {
		targetPostgres, targetConn, err := setupDatabase(ctx, "target", 5432)
		defer func() {
			if targetConn != nil {
				_ = targetConn.Close(ctx)
			}
			if targetPostgres != nil {
				_ = targetPostgres.Stop()
			}
		}()
		if err != nil {
			return fmt.Errorf("failed to setup target database: %w", err)
		}

		migratePostgres, migrateConn, err := setupDatabase(ctx, "migrate", 5433)
		defer func() {
			if migrateConn != nil {
				_ = migrateConn.Close(ctx)
			}
			if migratePostgres != nil {
				_ = migratePostgres.Stop()
			}
		}()
		if err != nil {
			return fmt.Errorf("failed to setup migrate database: %w", err)
		}

		var statements *string
		statements, err = generateMigrationStatements(
			ctx,
			config,
			wd,
			initial,
			targetConn,
			migrateConn,
		)
		if err != nil {
			return fmt.Errorf("failed to generate migration statements: %w", err)
		}

		file, err := os.CreateTemp("", "migration")
		if err != nil {
			return fmt.Errorf("failed get temporary migration file: %w", err)
		}

		err = os.WriteFile(
			file.Name(),
			[]byte(*statements),
			0o600,
		)
		if err != nil {
			return fmt.Errorf("failed to write temporary migration file: %w", err)
		}

		err = internal.RunHook(wd, "generate-migration-post", file.Name())
		if err != nil {
			return fmt.Errorf("failed to run hook: %w", err)
		}

		tmpStatementBytes, err := os.ReadFile(file.Name())
		if err != nil {
			return fmt.Errorf("failed to read temporary migration file: %w", err)
		}
		tmpStatementStr := string(tmpStatementBytes)
		statements = &tmpStatementStr

		err = os.Remove(file.Name())
		if err != nil {
			return fmt.Errorf("failed to delete temporary migration file: %w", err)
		}

		fmt.Println("")
		fmt.Println("--")
		fmt.Println(*statements)
		fmt.Println("--")
	}

	return nil
}

//nolint:gocognit,cyclop
func runWithFile(
	ctx context.Context,
	config *internal.Config,
	wd string,
	newMigrationFilePath string,
	migrationNumber uint,
) error {
	updated, err := checkIfUpdated(config, wd)
	if err != nil {
		return fmt.Errorf("failed to check if model has been updated: %w", err)
	}
	if updated {
		if _, err = os.Stat(newMigrationFilePath); err == nil {
			err = os.Remove(newMigrationFilePath)
			if err != nil {
				return fmt.Errorf("failed to delete generated migration file: %w", err)
			}
		}

		targetPostgres, targetConn, err := setupDatabase(ctx, "target", 5432)
		defer func() {
			if targetConn != nil {
				_ = targetConn.Close(ctx)
			}
			if targetPostgres != nil {
				_ = targetPostgres.Stop()
			}
		}()
		if err != nil {
			return fmt.Errorf("failed to setup target database: %w", err)
		}

		migratePostgres, migrateConn, err := setupDatabase(ctx, "migrate", 5433)
		defer func() {
			if migrateConn != nil {
				_ = migrateConn.Close(ctx)
			}
			if migratePostgres != nil {
				_ = migratePostgres.Stop()
			}
		}()
		if err != nil {
			return fmt.Errorf("failed to setup migrate database: %w", err)
		}

		var statements *string
		statements, err = generateMigrationStatements(
			ctx,
			config,
			wd,
			migrationNumber == 1,
			targetConn,
			migrateConn,
		)
		if err != nil {
			return fmt.Errorf("failed to generate migration statements: %w", err)
		}

		//nolint:gosec
		err = os.WriteFile(
			newMigrationFilePath,
			[]byte(*statements),
			0o644,
		)
		if err != nil {
			return fmt.Errorf("failed to write migration file: %w", err)
		}
		log.Println("Wrote migration file")

		err = internal.RunHook(wd, "generate-migration-post", newMigrationFilePath)
		if err != nil {
			return fmt.Errorf("failed to run hook: %w", err)
		}

		err = writeTemplateFiles(config, migrationNumber)
		if err != nil {
			return fmt.Errorf("failed to write template files: %w", err)
		}

		updated, err = generateDiffLockFile(ctx, wd, newMigrationFilePath, targetConn, migrateConn)
		if err != nil {
			return fmt.Errorf("failed to generate diff lock file: %w", err)
		}

		if updated {
			log.Println("Wrote diff lock file")
		}
	}

	return nil
}

func checkIfUpdated(config *internal.Config, wd string) (bool, error) {
	m, err := os.ReadFile(filepath.Join(wd, fmt.Sprintf("%s.dbm", config.ModelName)))
	if err != nil {
		return false, fmt.Errorf("failed to read model file: %w", err)
	}
	mStr := strings.TrimSuffix(string(m), "\n")
	if mStr == "" || mStr == modelContent {
		return false, nil
	}
	modelContent = mStr

	log.Println("Changes detected")

	return true, nil
}

func generateDiffLockFile(
	ctx context.Context,
	wd string,
	newMigrationFilePath string,
	targetConn,
	migrateConn *pgx.Conn,
) (bool, error) {
	newMigrationFileContent, err := os.ReadFile(newMigrationFilePath)
	if err != nil {
		return false, fmt.Errorf("failed to read new migratio file: %w", err)
	}
	_, err = migrateConn.Exec(ctx, string(newMigrationFileContent))
	if err != nil {
		return false, fmt.Errorf("failed to apply generated migration: %w", err)
	}

	var diff string
	diff, err = diffSchemaDumps(targetConn, migrateConn)
	if err != nil {
		return false, fmt.Errorf("failed to diff schema dumps: %w", err)
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

func diffSchemaDumps(targetConn, migrateConn *pgx.Conn) (string, error) {
	pgDumpOptions := []string{
		"--schema-only",
		"--exclude-table=public.schema_migrations",
	}

	targetDump, err := internal.PgDump(internal.DSN(targetConn, "disable"), pgDumpOptions)
	if err != nil {
		//nolint:wrapcheck
		return "", err
	}

	migrateDump, err := internal.PgDump(internal.DSN(migrateConn, "disable"), pgDumpOptions)
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
			return "", fmt.Errorf("failed to run git diff: %w %s", err, string(output))
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
			return fmt.Errorf("failed to create %q: %w", dir, err)
		}

		f, err := os.Create(ts.Path)
		if err != nil {
			return fmt.Errorf("failed to create file %q: %w", ts.Path, err)
		}

		err = t.Execute(f, map[string]interface{}{"NewVersion": newVersion})
		if err != nil {
			return fmt.Errorf("failed to execute template: %w", err)
		}
	}

	return nil
}

func getMigrationsDir(wd string) (string, error) {
	migrationsDir := filepath.Join(wd, "migrations")
	if _, err := os.Stat(migrationsDir); os.IsNotExist(err) {
		err = os.Mkdir(migrationsDir, 0o755)
		if err != nil {
			return "", fmt.Errorf("failed to create migrations directory: %w", err)
		}
	}

	return migrationsDir, nil
}

func getNewMigrationFilePath(
	wd string,
	migrationName string,
	overwrite bool,
) (
	path string,
	migrationNumber uint,
	err error,
) {
	migrationsDir, err := getMigrationsDir(wd)
	if err != nil {
		return "", 0, fmt.Errorf("failed to get migrations directory: %w", err)
	}
	migrationsCount, err := inspectMigrations(migrationsDir)
	if err != nil {
		return "", 0, fmt.Errorf("failed to inspect migrations directory: %w", err)
	}

	var migrationsNumber uint
	if _, err = os.Stat(
		filepath.Join(wd, "migrations", getMigrationFileName(migrationsCount, migrationName)),
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

	return filepath.Join(wd, "migrations", getMigrationFileName(migrationsNumber, migrationName)), migrationsNumber, nil
}

func getMigrationFileName(migrationNumber uint, migrationName string) string {
	return fmt.Sprintf("%03d_%s.up.sql", migrationNumber, migrationName)
}

var (
	//nolint:gochecknoglobals
	modelContent = ""
)

//nolint:cyclop
func generateMigrationStatements(
	ctx context.Context,
	config *internal.Config,
	wd string,
	initial bool,
	targetConn,
	migrateConn *pgx.Conn,
) (*string, error) {
	log.Println("Generating migration statements")

	err := internal.PgModelerExportToFile(
		filepath.Join(wd, fmt.Sprintf("%s.dbm", config.ModelName)),
		filepath.Join(wd, fmt.Sprintf("%s.sql", config.ModelName)),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to export model: %w", err)
	}

	go func() {
		err = internal.PgModelerExportToPng(
			filepath.Join(wd, fmt.Sprintf("%s.dbm", config.ModelName)),
			filepath.Join(wd, fmt.Sprintf("%s.png", config.ModelName)),
		)
		if err != nil {
			log.Printf("Failed to export png: %v\n", err)
		}
	}()

	err = internal.CreateUsers(ctx, migrateConn, config.DatabaseUsers)
	if err != nil {
		return nil, fmt.Errorf("failed to create migrate users: %w", err)
	}

	err = internal.CreateUsers(ctx, targetConn, config.DatabaseUsers)
	if err != nil {
		return nil, fmt.Errorf("failed to create target users: %w", err)
	}

	err = executeTargetSQL(ctx, config, wd, targetConn)
	if err != nil {
		return nil, fmt.Errorf("failed to execute target sql: %w", err)
	}

	if initial {
		// If we are developing the schema initially, there will be no diffs,
		// and we want to copy over the schema file to the initial migration file
		var input []byte
		input, err = os.ReadFile(filepath.Join(wd, fmt.Sprintf("%s.sql", config.ModelName)))
		if err != nil {
			return nil, fmt.Errorf("failed to read sql file: %w", err)
		}

		str := string(input)

		return &str, nil
	}

	err = executeMigrateSQL(wd, migrateConn)
	if err != nil {
		return nil, fmt.Errorf("failed to execute migrate sql: %w", err)
	}

	statements, err := internal.Migra(internal.DSN(migrateConn, "disable"), internal.DSN(targetConn, "disable"))
	if err != nil {
		return nil, fmt.Errorf("failed to run migra: %w", err)
	}

	// Filter stuff from go-migrate that doesn't exist in the target db, and we don't have and need anyway
	statements = strings.ReplaceAll(
		statements,
		"alter table \"public\".\"schema_migrations\" drop constraint \"schema_migrations_pkey\";",
		"",
	)
	statements = strings.ReplaceAll(
		statements,
		"drop index if exists \"public\".\"schema_migrations_pkey\";",
		"",
	)
	statements = strings.ReplaceAll(
		statements,
		"drop table \"public\".\"schema_migrations\";",
		"",
	)
	statements = strings.Trim(statements, "\n")

	var lines []string
	for _, line := range strings.Split(statements, "\n") {
		if line != "" {
			lines = append(lines, line)
		}
	}
	statements = strings.Join(lines, "\n") + "\n"

	return &statements, nil
}

func executeMigrateSQL(wd string, migrateConn *pgx.Conn) error {
	m, err := migrate.New(fmt.Sprintf("file://%s", filepath.Join(wd, "migrations")), internal.DSN(migrateConn, "disable"))
	if err != nil {
		return fmt.Errorf("failed to create migrate: %w", err)
	}
	err = m.Up()
	if err != nil {
		return fmt.Errorf("failed to up migrations: %w", err)
	}

	return nil
}

func executeTargetSQL(ctx context.Context, config *internal.Config, wd string, targetConn *pgx.Conn) error {
	targetSQL, err := os.ReadFile(filepath.Join(wd, fmt.Sprintf("%s.sql", config.ModelName)))
	if err != nil {
		return fmt.Errorf("failed to read target sql: %w", err)
	}

	_, err = targetConn.Exec(ctx, string(targetSQL))
	if err != nil {
		return fmt.Errorf("failed to execute target sql: %w", err)
	}

	return nil
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
