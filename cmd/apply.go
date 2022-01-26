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

//nolint:gochecknoglobals
var applyCmd = &cobra.Command{
	Use:   "apply",
	Short: "Apply the migrations to a running database",
	Run: func(cmd *cobra.Command, args []string) {
		config, err := internal.ReadConfig()
		if err != nil {
			log.Fatalf("Failed to read config: %v\n", err)
		}

		wd, err := os.Getwd()
		if err != nil {
			log.Fatalf("Failed to get working directory: %v\n", err)
		}

		pgHost := os.Getenv("PGHOST")
		pgPort := os.Getenv("PGPORT")
		pgUser := os.Getenv("PGUSER")
		pgPassword := os.Getenv("PGPASSWORD")
		resetDB := os.Getenv("RESET_DB") == "true"
		insertTestData := os.Getenv("INSERT_TEST_DATA") == "true"
		sslMode := internal.GetSSLMode()
		conn, err := pgx.Connect(context.Background(), fmt.Sprintf(
			"postgres://%s:%s@%s:%s/%s?sslmode=%s",
			pgUser,
			pgPassword,
			pgHost,
			pgPort,
			"postgres", // We need to connect to the default database in order to drop and create the actual database
			sslMode,
		))
		if err != nil {
			log.Fatalf("Unable to connect to database: %v", err)
		}

		if resetDB {
			log.Println("Resetting database")
			_, err = conn.Exec(
				context.Background(),
				fmt.Sprintf("DROP DATABASE IF EXISTS %q WITH (FORCE)", config.DatabaseName),
			)
			if err != nil {
				log.Fatalf("Failed to drop database: %v", err)
			}
			_, err = conn.Exec(
				context.Background(),
				fmt.Sprintf("DROP TABLE IF EXISTS %q", "schema_migrations"),
			)
			if err != nil {
				log.Fatalf("Failed to drop table: %v", err)
			}
		}

		databaseExists, err := internal.CheckDatabaseExists(conn, config.DatabaseName)
		if err != nil {
			log.Fatalf("Failed to check if database exists: %v", err)
		}
		if !databaseExists {
			_, err = conn.Exec(context.Background(), fmt.Sprintf("CREATE DATABASE %q;", config.DatabaseName))
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
				_, err = conn.Exec(context.Background(), fmt.Sprintf("CREATE ROLE %q WITH LOGIN;", u))
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
			"postgres://%s:%s@%s:%s/%s?sslmode=%s",
			pgUser,
			pgPassword,
			pgHost,
			pgPort,
			config.DatabaseName,
			sslMode,
		)

		m, err := migrate.New(fmt.Sprintf("file://%s", filepath.Join(wd, "migrations")), dsn)
		if err != nil {
			log.Fatalln(err)
		}

		if resetDB {
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

		log.Println("Successfully migrated database")
	},
}
