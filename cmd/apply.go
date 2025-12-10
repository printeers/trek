package cmd

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"log"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/golang-migrate/migrate/v4"
	internalpostgres "github.com/printeers/trek/internal/postgres"

	// needed driver.
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"github.com/jackc/pgx/v5"
	"github.com/spf13/cobra"

	"github.com/printeers/trek/internal"
)

//nolint:gocognit,cyclop
func NewApplyCommand() *cobra.Command {
	var (
		postgresHost     string
		postgresPort     int
		postgresUser     string
		postgresPassword string
		postgresSSLMode  string
		resetDatabase    bool
		insertTestData   bool
	)

	applyCmd := &cobra.Command{
		Use:   "apply",
		Short: "Apply the migrations to a running database",
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

			conn, err := pgx.Connect(ctx, fmt.Sprintf(
				"postgres://%s:%s@%s:%d/%s?sslmode=%s",
				postgresUser,
				postgresPassword,
				postgresHost,
				postgresPort,
				"postgres", // We need to connect to the default database in order to drop and create the actual database
				postgresSSLMode,
			))
			if err != nil {
				return fmt.Errorf("failed to connect to database: %w", err)
			}

			if resetDatabase {
				log.Println("Resetting database")

				err = internal.RunHook(ctx, wd, "apply-reset-pre", nil)
				if err != nil {
					return fmt.Errorf("failed to run hook: %w", err)
				}

				_, err = conn.Exec(
					ctx,
					fmt.Sprintf("DROP DATABASE IF EXISTS %q WITH (FORCE)", config.DatabaseName),
				)
				if err != nil {
					return fmt.Errorf("failed to drop database: %w", err)
				}
			}

			databaseExists, err := internalpostgres.CheckDatabaseExists(ctx, conn, config.DatabaseName)
			if err != nil {
				return fmt.Errorf("failed to check if database exists: %w", err)
			}
			if !databaseExists {
				_, err = conn.Exec(ctx, fmt.Sprintf("CREATE DATABASE %q", config.DatabaseName))
				if err != nil {
					return fmt.Errorf("failed to create database: %w", err)
				}
			}

			for _, u := range config.DatabaseUsers {
				var userExists bool
				userExists, err = internalpostgres.CheckUserExists(ctx, conn, u)
				if err != nil {
					return fmt.Errorf("failed to check if user exists: %w", err)
				}
				if !userExists {
					_, err = conn.Exec(ctx, fmt.Sprintf("CREATE ROLE %q WITH LOGIN", u))
					if err != nil {
						return fmt.Errorf("failed to create user: %w", err)
					}
				}
			}

			err = conn.Close(ctx)
			if err != nil {
				return fmt.Errorf("failed to close database connection: %w", err)
			}

			dsn := fmt.Sprintf(
				"postgres://%s:%s@%s:%d/%s?sslmode=%s",
				postgresUser,
				postgresPassword,
				postgresHost,
				postgresPort,
				config.DatabaseName,
				postgresSSLMode,
			)

			migrationsDir, err := internal.GetMigrationsDir(wd)
			if err != nil {
				return fmt.Errorf("failed to get migrations directory: %w", err)
			}

			m, err := migrate.New(fmt.Sprintf("file://%s", migrationsDir), dsn)
			if err != nil {
				return fmt.Errorf("failed to initialize go-migrate: %w", err)
			}

			if resetDatabase || !databaseExists {
				var migrationFiles []string
				migrationFiles, err = internal.FindMigrations(migrationsDir, true)
				if err != nil {
					return fmt.Errorf("failed to read migrations: %w", err)
				}

				for index, file := range migrationFiles {
					log.Printf("Applying migration %q\n", file)
					err = m.Steps(1)
					if errors.Is(err, migrate.ErrNoChange) {
						log.Println("No changes!")
					} else if err != nil {
						return fmt.Errorf("failed to apply migration %q: %w", file, err)
					}
					if insertTestData {
						err = filepath.Walk(filepath.Join(wd, "testdata"), func(p string, _ fs.FileInfo, err error) error {
							if err != nil {
								return err
							}

							if strings.HasPrefix(path.Base(p), fmt.Sprintf("%03d", index+1)) {
								log.Printf("Inserting testdata %q\n", path.Base(p))

								// We have to use psql, because users might use commands like "\copy"
								// which don't work by directly connecting to the database
								err := internalpostgres.PsqlFile(ctx, dsn, p)
								if err != nil {
									return fmt.Errorf("failed to insert testdata: %w", err)
								}

								return nil
							}

							return nil
						})
						if err != nil {
							return fmt.Errorf("failed to run testdata: %w", err)
						}
					}
				}

				err = internal.RunHook(ctx, wd, "apply-reset-post", nil)
				if err != nil {
					return fmt.Errorf("failed to run hook: %w", err)
				}
			} else {
				err = m.Up()
				if errors.Is(err, migrate.ErrNoChange) {
					log.Println("No changes!")
				} else if err != nil {
					return fmt.Errorf("failed to apply migrations: %w", err)
				}
			}

			conn, err = pgx.Connect(ctx, dsn)
			if err != nil {
				return fmt.Errorf("failed to connect to database: %w", err)
			}

			for _, u := range config.DatabaseUsers {
				_, err = conn.Exec(ctx, fmt.Sprintf("GRANT SELECT ON public.schema_migrations TO %q", u))
				if err != nil {
					return fmt.Errorf("failed to grant select permission on schema_migrations to %q: %w", u, err)
				}
			}

			err = conn.Close(ctx)
			if err != nil {
				return fmt.Errorf("failed to close database connection: %w", err)
			}

			log.Println("Successfully migrated database")

			return nil
		},
	}

	applyCmd.Flags().StringVar(&postgresHost, "postgres-host", "", "Host of the PostgreSQL database")
	applyCmd.Flags().IntVar(&postgresPort, "postgres-port", 0, "Port of the PostgreSQL database")
	applyCmd.Flags().StringVar(&postgresUser, "postgres-user", "", "User of the PostgreSQL database")
	applyCmd.Flags().StringVar(&postgresPassword, "postgres-password", "", "Password of the PostgreSQL database")
	applyCmd.Flags().StringVar(&postgresSSLMode, "postgres-sslmode", "disable", "SSL Mode of the PostgreSQL database")
	applyCmd.Flags().BoolVar(&resetDatabase, "reset-database", false, "Reset the database before applying migrations")
	applyCmd.Flags().BoolVar(&insertTestData, "insert-test-data", false, "Insert the testdata of each migration after the individual migrations has been applied") //nolint:lll
	internal.MarkFlagRequired(applyCmd, "postgres-host")
	internal.MarkFlagRequired(applyCmd, "postgres-port")
	internal.MarkFlagRequired(applyCmd, "postgres-user")
	internal.MarkFlagRequired(applyCmd, "postgres-password")

	return applyCmd
}
