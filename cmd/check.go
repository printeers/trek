package cmd

import (
	"context"
	"encoding/xml"
	"errors"
	"fmt"
	"io/fs"
	"log"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/golang-migrate/migrate/v4"
	"github.com/jackc/pgx/v5"
	internalpostgres "github.com/printeers/trek/internal/postgres"

	// needed driver.
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"github.com/spf13/cobra"

	"github.com/printeers/trek/internal"
	"github.com/printeers/trek/internal/dbm"
)

func NewCheckCommand() *cobra.Command {
	checkCmd := &cobra.Command{
		Use:   "check",
		Short: "Validate all files",
		PersistentPreRun: func(cmd *cobra.Command, _ []string) {
			internal.InitializeFlags(cmd)
		},
		RunE: func(_ *cobra.Command, _ []string) error {
			ctx := context.Background()

			wd, err := os.Getwd()
			if err != nil {
				return fmt.Errorf("failed to get working directory: %w", err)
			}

			config, err := internal.ReadConfig(wd)
			if err != nil {
				return fmt.Errorf("failed to read config: %w", err)
			}

			migrationsDir, err := internal.GetMigrationsDir(wd)
			if err != nil {
				return fmt.Errorf("failed to get migrations directory: %w", err)
			}

			return checkAll(ctx, config, wd, migrationsDir)
		},
	}

	return checkCmd
}

//nolint:cyclop
func checkAll(
	ctx context.Context,
	config *internal.Config,
	wd,
	migrationsDir string,
) error {
	postgres, err := setupDatabase(5434)
	if err != nil {
		return fmt.Errorf("failed to setup database: %w", err)
	}
	defer postgres.Stop() //nolint:errcheck

	dsn := postgres.DSN("postgres")

	conn, err := pgx.Connect(ctx, dsn)
	if err != nil {
		return fmt.Errorf("failed to connect to database: %w", err)
	}
	defer conn.Close(ctx)

	for _, u := range config.DatabaseUsers {
		var userExists bool
		userExists, err = internalpostgres.CheckUserExists(ctx, conn, u)
		if err != nil {
			return fmt.Errorf("failed to check if user exists: %w", err)
		}
		if !userExists {
			_, err = conn.Exec(ctx, fmt.Sprintf("CREATE ROLE %q WITH LOGIN PASSWORD 'postgres'", u))
			if err != nil {
				return fmt.Errorf("failed to create user: %w", err)
			}
		}
	}

	migrationFiles, err := internal.FindMigrations(migrationsDir, true)
	if err != nil {
		return fmt.Errorf("failed to find migrations: %w", err)
	}

	hookOptions := &internal.HookOptions{
		Env: map[string]string{
			"TREK_POSTGRES_HOST":     "localhost",
			"TREK_POSTGRES_PORT":     "5434",
			"TREK_POSTGRES_USER":     "postgres",
			"TREK_POSTGRES_PASSWORD": "postgres",
			"TREK_POSTGRES_DATABASE": "postgres",
			"TREK_POSTGRES_SSLMODE":  "disable",
		},
	}

	err = internal.RunHook(ctx, wd, "check-pre", hookOptions)
	if err != nil {
		return fmt.Errorf("failed to run hook: %w", err)
	}

	log.Println("Checking dbm file")

	err = checkDBM(config, wd)
	if err != nil {
		return fmt.Errorf("failed to check dbm: %w", err)
	}

	log.Println("Checking migration file names")

	err = checkMigrationFileNames(migrationFiles)
	if err != nil {
		return fmt.Errorf("failed to check migration file names: %w", err)
	}

	log.Println("Checking templates")

	err = checkTemplates(config, uint(len(migrationFiles)))
	if err != nil {
		return fmt.Errorf("failed to check templates: %w", err)
	}

	log.Println("Checking migrations and testdata")

	err = checkMigrationsAndTestdata(ctx, wd, migrationsDir, dsn, migrationFiles)
	if err != nil {
		return fmt.Errorf("failed to check migrations and testdata: %w", err)
	}

	for _, u := range config.DatabaseUsers {
		_, err = conn.Exec(ctx, fmt.Sprintf("GRANT SELECT ON public.schema_migrations TO %q", u))
		if err != nil {
			return fmt.Errorf("failed to grant select permission on schema_migrations to %q: %w", u, err)
		}
	}

	err = internal.RunHook(ctx, wd, "check-post", hookOptions)
	if err != nil {
		return fmt.Errorf("failed to run hook: %w", err)
	}

	return nil
}

func checkDBM(config *internal.Config, wd string) error {
	model := dbm.DBModel{}

	m, err := os.ReadFile(filepath.Join(wd, fmt.Sprintf("%s.dbm", config.ModelName)))
	if err != nil {
		return fmt.Errorf("failed to read model file: %w", err)
	}

	err = xml.Unmarshal(m, &model)
	if err != nil {
		return fmt.Errorf("failed to parse model: %w", err)
	}

	modelRoles := map[string]struct{}{}
	for _, role := range model.Roles {
		if !role.SQLDisabled {
			//nolint:err113
			return fmt.Errorf("role %q has sql enabled", role.Name)
		}
		modelRoles[role.Name] = struct{}{}
	}

	configRoles := map[string]struct{}{}
	for _, role := range config.DatabaseUsers {
		configRoles[role] = struct{}{}
	}

	for role := range modelRoles {
		if _, ok := configRoles[role]; !ok {
			//nolint:err113
			return fmt.Errorf("role %q defined in the model not defined in the config", role)
		}
	}

	for role := range configRoles {
		if _, ok := modelRoles[role]; !ok {
			//nolint:err113
			return fmt.Errorf("role %q defined in the config not defined in the model", role)
		}
	}

	if len(model.Databases) > 1 {
		//nolint:err113
		return fmt.Errorf("only one database allowed in the model")
	}
	if len(model.Databases) == 0 {
		//nolint:err113
		return fmt.Errorf("no database defined in the model")
	}
	if model.Databases[0].Name != config.DatabaseName {
		//nolint:err113
		return fmt.Errorf(
			"database defined in model should be named %q but is named %q",
			config.DatabaseName,
			model.Databases[0].Name,
		)
	}

	return nil
}

func checkMigrationFileNames(migrationFiles []string) error {
	for _, migrationFile := range migrationFiles {
		if !internal.RegexpMigrationFileName.MatchString(migrationFile) {
			//nolint:err113
			return fmt.Errorf("invalid migration file name %q", migrationFile)
		}
	}

	existingMigrations := map[int]struct{}{}
	for _, migrationFile := range migrationFiles {
		index, err := strconv.Atoi(strings.Split(migrationFile, "_")[0])
		if err != nil {
			//nolint:err113
			return fmt.Errorf("failed to parse migration index of file %q", migrationFile)
		}

		if _, ok := existingMigrations[index]; ok {
			//nolint:err113
			return fmt.Errorf("migration with index %d exists more than once", index)
		}

		if len(existingMigrations) != index-1 {
			//nolint:err113
			return fmt.Errorf("migration after index %d missing", len(existingMigrations))
		}

		existingMigrations[index] = struct{}{}
	}

	return nil
}

func checkTemplates(config *internal.Config, migrationsCount uint) error {
	for _, ts := range config.Templates {
		if _, err := os.Stat(ts.Path); errors.Is(err, os.ErrNotExist) {
			//nolint:err113
			return fmt.Errorf("templated file %q does not exist", ts.Path)
		}

		data, err := internal.ExecuteConfigTemplate(ts, migrationsCount)
		if err != nil {
			return fmt.Errorf("failed to execute template: %w", err)
		}

		writtenData, err := os.ReadFile(ts.Path)
		if err != nil {
			return fmt.Errorf("failed to read file %q: %w", ts.Path, err)
		}

		if string(writtenData) != *data {
			//nolint:err113
			return fmt.Errorf("templated file %q not up to date", ts.Path)
		}
	}

	return nil
}

func checkMigrationsAndTestdata(ctx context.Context, wd, migrationsDir, dsn string, migrationFiles []string) error {
	m, err := migrate.New(fmt.Sprintf("file://%s", migrationsDir), dsn)
	if err != nil {
		return fmt.Errorf("failed to initialize go-migrate: %w", err)
	}

	for index, file := range migrationFiles {
		err = m.Steps(1)
		if errors.Is(err, migrate.ErrNoChange) {
			continue
		} else if err != nil {
			return fmt.Errorf("failed to apply migration %q: %w", file, err)
		}
		err = filepath.Walk(filepath.Join(wd, "testdata"), func(p string, _ fs.FileInfo, err error) error {
			if err != nil {
				return err
			}

			if strings.HasPrefix(path.Base(p), fmt.Sprintf("%03d", index+1)) {
				// We have to use psql, because users might use commands like "\copy"
				// which don't work by directly connecting to the database
				err := internalpostgres.PsqlFile(ctx, dsn, p)
				if err != nil {
					//nolint:err113
					return fmt.Errorf("failed to apply testdata: %w", err)
				}

				return nil
			}

			return nil
		})
		if err != nil {
			//nolint:err113
			return fmt.Errorf("failed to run testdata: %w", err)
		}
	}

	return nil
}
