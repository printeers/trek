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
	// needed driver.
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"github.com/jackc/pgx/v4"
	"github.com/spf13/cobra"

	"github.com/stack11/trek/internal"
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
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			initializeConfig(cmd)

			return nil
		},
		Run: func(cmd *cobra.Command, args []string) {
			config, err := internal.ReadConfig()
			if err != nil {
				log.Fatalf("Failed to read config: %v\n", err)
			}

			wd, err := os.Getwd()
			if err != nil {
				log.Fatalf("Failed to get working directory: %v\n", err)
			}

			conn, err := pgx.Connect(context.Background(), fmt.Sprintf(
				"postgres://%s:%s@%s:%d/%s?sslmode=%s",
				postgresUser,
				postgresPassword,
				postgresHost,
				postgresPort,
				"postgres", // We need to connect to the default database in order to drop and create the actual database
				postgresSSLMode,
			))
			if err != nil {
				log.Fatalf("Unable to connect to database: %v", err)
			}

			if resetDatabase {
				log.Println("Resetting database")
				_, err = conn.Exec(
					context.Background(),
					fmt.Sprintf("DROP DATABASE IF EXISTS %q WITH (FORCE)", config.DatabaseName),
				)
				if err != nil {
					log.Fatalf("Failed to drop database: %v", err)
				}
			}

			databaseExists, err := internal.CheckDatabaseExists(conn, config.DatabaseName)
			if err != nil {
				log.Fatalf("Failed to check if database exists: %v", err)
			}
			if !databaseExists {
				_, err = conn.Exec(context.Background(), fmt.Sprintf("CREATE DATABASE %q", config.DatabaseName))
				if err != nil {
					log.Fatalf("Failed to create database: %v", err)
				}
			}

			for _, u := range config.DatabaseUsers {
				var userExists bool
				userExists, err = internal.CheckUserExists(conn, u)
				if err != nil {
					log.Fatalf("Failed to check if user exists: %v", err)
				}
				if !userExists {
					_, err = conn.Exec(context.Background(), fmt.Sprintf("CREATE ROLE %q WITH LOGIN", u))
					if err != nil {
						log.Fatalf("Failed to create user: %v", err)
					}
				}
			}

			err = conn.Close(context.Background())
			if err != nil {
				log.Fatalf("Failed to close connection: %v", err)
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

			m, err := migrate.New(fmt.Sprintf("file://%s", filepath.Join(wd, "migrations")), dsn)
			if err != nil {
				log.Fatalln(err)
			}

			if resetDatabase {
				var files []os.DirEntry
				files, err = os.ReadDir(filepath.Join(wd, "migrations"))
				if err != nil {
					log.Fatalln(err)
				}

				for index, file := range files {
					log.Printf("Running migration %s\n", file.Name())
					err = m.Steps(1)
					if errors.Is(err, migrate.ErrNoChange) {
						log.Println("No changes!")
					} else if err != nil {
						log.Fatalln(err)
					}
					if insertTestData {
						err = filepath.Walk(filepath.Join(wd, "testdata"), func(p string, info fs.FileInfo, err error) error {
							if strings.HasPrefix(path.Base(p), fmt.Sprintf("%03d", index+1)) {
								log.Printf("Inserting test data %s\n", path.Base(p))

								// We have to use psql, because users might use commands like "\copy"
								// which don't work by directly connecting to the database
								err := internal.PsqlFile(dsn, p)
								if err != nil {
									return fmt.Errorf("failed to apply test data: %w", err)
								}

								return nil
							}

							return nil
						})
						if err != nil {
							log.Fatalf("Failed to run testdata: %v\n", err)
						}
					}
				}
			} else {
				err = m.Up()
				if errors.Is(err, migrate.ErrNoChange) {
					log.Println("No changes!")
				} else if err != nil {
					log.Fatalln(err)
				}
			}

			conn, err = pgx.Connect(context.Background(), dsn)
			if err != nil {
				log.Fatalf("Unable to connect to database: %v", err)
			}

			for _, u := range config.DatabaseUsers {
				_, err = conn.Exec(context.Background(), fmt.Sprintf("GRANT SELECT ON public.schema_migrations TO %q", u))
				if err != nil {
					log.Fatalf("Failed to grant select permission on schema_migrations to %q: %v", u, err)
				}
			}

			err = conn.Close(context.Background())
			if err != nil {
				log.Fatalf("Failed to close connection: %v", err)
			}

			log.Println("Successfully migrated database")
		},
	}

	applyCmd.Flags().StringVar(&postgresHost, "postgres-host", "", "Host of the PostgreSQL database")
	applyCmd.Flags().IntVar(&postgresPort, "postgres-port", 0, "Port of the PostgreSQL database")
	applyCmd.Flags().StringVar(&postgresUser, "postgres-user", "", "User of the PostgreSQL database")
	applyCmd.Flags().StringVar(&postgresPassword, "postgres-password", "", "Password of the PostgreSQL database")
	applyCmd.Flags().StringVar(&postgresSSLMode, "postgres-sslmode", "disable", "SSL Mode of the PostgreSQL database")
	applyCmd.Flags().BoolVar(&resetDatabase, "reset-database", false, "Reset the database before applying migrations")
	applyCmd.Flags().BoolVar(&insertTestData, "insert-test-data", false, "Insert the test data of each migration after the individual migrations has been applied") //nolint:lll
	markFlagRequired(applyCmd, "postgres-host")
	markFlagRequired(applyCmd, "postgres-port")
	markFlagRequired(applyCmd, "postgres-user")
	markFlagRequired(applyCmd, "postgres-password")

	return applyCmd
}
