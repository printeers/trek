package cmd

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/golang-migrate/migrate/v4"
	"github.com/spf13/cobra"
	"github.com/thecodeteam/goodbye"

	"github.com/stack11/trek/internal"
)

var (
	//nolint:gochecknoglobals
	flagDiffInitial bool
	//nolint:gochecknoglobals
	flagOnce bool
)

//nolint:gochecknoinits
func init() {
	generateCmd.Flags().BoolVarP(
		&flagDiffInitial,
		"initial",
		"i",
		false,
		"Directly copy the diff to the migrations. Used for first time setup",
	)
	generateCmd.Flags().BoolVar(
		&flagOnce,
		"once",
		false,
		"Run only once and don't watch files",
	)
}

//nolint:gochecknoglobals
var generateCmd = &cobra.Command{
	Use:   "generate",
	Short: "Generate the migrations for a pgModeler file",
	Args: func(cmd *cobra.Command, args []string) error {
		if !flagDiffInitial && len(args) != 1 {
			//nolint:goerr113
			return errors.New("pass the name of the migration or use the -i/--initial flag for the initial migration")
		}

		return nil
	},
	Run: func(cmd *cobra.Command, args []string) {
		internal.AssertGenerateToolsAvailable()

		config, err := internal.ReadConfig()
		if err != nil {
			log.Fatalf("Failed to read config: %v\n", err)
		}

		ctx := context.Background()
		defer goodbye.Exit(ctx, -1)
		goodbye.Notify(ctx)
		goodbye.Register(func(ctx context.Context, sig os.Signal) {
			internal.DockerKillContainer(targetContainerID)
			internal.DockerKillContainer(migrateContainerID)
		})

		if len(args) == 0 {
			args = append(args, "")
		}

		updateDiff(*config, args[0], flagDiffInitial)

		if !flagOnce {
			for {
				time.Sleep(time.Millisecond * 100)
				updateDiff(*config, args[0], flagDiffInitial)
			}
		}
	},
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
func updateDiff(config internal.Config, migrationName string, initial bool) {
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
		return
	}
	modelContent = mStr

	migrationsDir := filepath.Join(wd, "migrations")
	if _, err = os.Stat(migrationsDir); os.IsNotExist(err) {
		err = os.Mkdir(migrationsDir, 0o755)
		if err != nil {
			log.Fatalf("Failed to create migrations directory: %v\n", err)
		}
	}

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
		// If we are developing the schema initially, there will be no diffs,
		// and we want to copy over the schema file to the initial migration file
		var input []byte
		input, err = os.ReadFile(filepath.Join(wd, fmt.Sprintf("%s.sql", config.ModelName)))
		if err != nil {
			log.Panicln(err)
		}

		//nolint:gosec
		err = os.WriteFile(filepath.Join(wd, "migrations", "001_init.up.sql"), input, 0o644)
		if err != nil {
			log.Panicln(err)
		}

		return
	}

	migrateDSN, err := setupMigrateDatabase(wd, config, migrationName, migrateContainerID)
	if err != nil {
		log.Panicln(err)
	}

	targetDSN, err := setupTargetDatabase(wd, config, targetContainerID)
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
		filepath.Join(wd, "migrations", fmt.Sprintf("%s.up.sql", migrationName)),
		[]byte(diff),
		0o644,
	)
	if err != nil {
		log.Panicln(err)
	}
}

func setupMigrateDatabase(wd string, config internal.Config, migrationName, migrateContainerID string) (string, error) {
	targetIP, err := setupDatabase(migrateContainerID, config)
	if err != nil {
		return "", err
	}

	migrationFile := filepath.Join(wd, "migrations", fmt.Sprintf("%s.up.sql", migrationName))
	if _, err = os.Stat(migrationFile); err == nil {
		err = os.Remove(migrationFile)
		if err != nil {
			return "", fmt.Errorf("failed to delete existing migration file: %w", err)
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

func setupTargetDatabase(wd string, config internal.Config, targetContainerID string) (string, error) {
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

func setupDatabase(containerName string, config internal.Config) (containerIP string, err error) {
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
