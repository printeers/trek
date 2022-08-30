package cmd

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"text/template"
	"time"

	embeddedpostgres "github.com/fergusstrange/embedded-postgres"
	"github.com/golang-migrate/migrate/v4"
	"github.com/jackc/pgx/v4"
	"github.com/spf13/cobra"

	"github.com/stack11/trek/internal"
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

				if !internal.RegexpMigrationName.MatchString(args[0]) {
					//nolint:goerr113
					return errors.New("migration name must be lower-kebab-case and must not start or end with a number or dash")
				}
			}

			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := context.Background()

			wd, err := os.Getwd()
			if err != nil {
				return fmt.Errorf("failed to get working directory: %w", err)
			}

			config, err := internal.ReadConfig(wd)
			if err != nil {
				return fmt.Errorf("failed to read config: %w", err)
			}

			var migrationsDir string
			migrationsDir, err = internal.GetMigrationsDir(wd)
			if err != nil {
				return fmt.Errorf("failed to get migrations directory: %w", err)
			}

			var runnerFunc func() error

			if stdout {
				var migrationsCount uint
				migrationsCount, err = internal.InspectMigrations(migrationsDir)
				if err != nil {
					return fmt.Errorf("failed to get inspect migrations directory: %w", err)
				}

				initial := migrationsCount == 0

				runnerFunc = func() error {
					return runWithStdout(ctx, config, wd, migrationsDir, initial)
				}
			} else {
				migrationName := args[0]
				var newMigrationFilePath string
				var migrationNumber uint
				newMigrationFilePath, migrationNumber, err = internal.GetNewMigrationFilePath(
					migrationsDir,
					migrationName,
					overwrite,
				)
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
					return runWithFile(ctx, config, wd, migrationsDir, newMigrationFilePath, migrationNumber)
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
	tmpDir,
	name string,
	port uint32,
) (
	*embeddedpostgres.EmbeddedPostgres,
	*pgx.Conn,
	error,
) {
	postgres, dsn := internal.NewPostgresDatabase(filepath.Join(tmpDir, name), port)
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
	wd,
	migrationsDir string,
	initial bool,
) error {
	updated, err := checkIfUpdated(config, wd)
	if err != nil {
		return fmt.Errorf("failed to check if model has been updated: %w", err)
	}
	if updated {
		tmpDir, err := os.MkdirTemp("", "trek-")
		if err != nil {
			return fmt.Errorf("failed to create temporary directory: %w", err)
		}
		defer os.RemoveAll(tmpDir)

		targetPostgres, targetConn, err := setupDatabase(ctx, tmpDir, "target", 5432)
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

		migratePostgres, migrateConn, err := setupDatabase(ctx, tmpDir, "migrate", 5433)
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
			migrationsDir,
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
	wd,
	migrationsDir,
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

		tmpDir, err := os.MkdirTemp("", "trek-")
		if err != nil {
			return fmt.Errorf("failed to create temporary directory: %w", err)
		}
		defer os.RemoveAll(tmpDir)

		targetPostgres, targetConn, err := setupDatabase(ctx, tmpDir, "target", 5432)
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

		migratePostgres, migrateConn, err := setupDatabase(ctx, tmpDir, "migrate", 5433)
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
			migrationsDir,
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

		updated, err = generateDiffLockFile(ctx, wd, tmpDir, newMigrationFilePath, targetConn, migrateConn)
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
	wd,
	tmpDir string,
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
	diff, err = diffSchemaDumps(tmpDir, targetConn, migrateConn)
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

func diffSchemaDumps(tmpDir string, targetConn, migrateConn *pgx.Conn) (string, error) {
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

	targetDumpFile := filepath.Join(tmpDir, "target.sql")
	err = os.WriteFile(targetDumpFile, []byte(cleanDump(targetDump)), 0o600)
	if err != nil {
		return "", fmt.Errorf("failed to write target.sql file: %w", err)
	}

	migrateDumpFile := filepath.Join(tmpDir, "migrate.sql")
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

var (
	//nolint:gochecknoglobals
	modelContent = ""
)

//nolint:cyclop
func generateMigrationStatements(
	ctx context.Context,
	config *internal.Config,
	wd,
	migrationsDir string,
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

	err = executeMigrateSQL(migrationsDir, migrateConn)
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

func executeMigrateSQL(migrationsDir string, migrateConn *pgx.Conn) error {
	m, err := migrate.New(fmt.Sprintf("file://%s", migrationsDir), internal.DSN(migrateConn, "disable"))
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
