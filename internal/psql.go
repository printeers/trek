package internal

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"time"
)

const (
	//nolint:gochecknoglobals
	PGDefaultUsername = "postgres"
	//nolint:gochecknoglobals
	PGDefaultPassword = "postgres"
	//nolint:gochecknoglobals
	PGDefaultDatabase = "postgres"
)

func getEnv(password, sslmode string) []string {
	return append(
		os.Environ(),
		fmt.Sprintf("PGPASSWORD=%s", password),
		fmt.Sprintf("PGSSLMODE=%s", sslmode),
	)
}

func PsqlIsDatabaseUp(ip, user, password, sslmode string) (up bool, out []byte) {
	cmdPsql := exec.Command(
		"psql",
		"--echo-errors",
		"--variable",
		"ON_ERROR_STOP=1",
		"--user",
		user,
		"--host",
		ip,
		"--command",
		"\\l",
		PGDefaultDatabase,
	)
	cmdPsql.Env = getEnv(password, sslmode)
	out, err := cmdPsql.CombinedOutput()

	return err == nil, out
}

func PsqlWaitDatabaseUp(ip, user, password, sslmode string) {
	var connected bool
	var out []byte
	count := 0
	for {
		if count == 10 {
			log.Fatalf("Failed to connect to database: %s\n", string(out))
		}
		if connected, out = PsqlIsDatabaseUp(ip, user, password, sslmode); connected {
			break
		} else {
			count++
			log.Printf("Waiting for %s\n", ip)
			time.Sleep(time.Second)
		}
	}
}

func PsqlCommand(ip, user, password, sslmode, database, command string) error {
	cmdPsql := exec.Command(
		"psql",
		"--echo-errors",
		"--variable",
		"ON_ERROR_STOP=1",
		"--user",
		user,
		"--host",
		ip,
		"--command",
		command,
		database,
	)
	cmdPsql.Env = getEnv(password, sslmode)
	cmdPsql.Stderr = os.Stderr
	cmdPsql.Stdout = os.Stdout

	err := cmdPsql.Run()
	if err != nil {
		return fmt.Errorf("failed to run psql: %w", err)
	}

	return nil
}

func PsqlFile(ip, user, password, sslmode, database, file string) error {
	cmdPsql := exec.Command(
		"psql",
		"--echo-errors",
		"--variable",
		"ON_ERROR_STOP=1",
		"--user",
		user,
		"--host",
		ip,
		"--file",
		file,
		database,
	)
	cmdPsql.Env = getEnv(password, sslmode)
	cmdPsql.Stderr = os.Stderr
	cmdPsql.Stdout = os.Stdout

	err := cmdPsql.Run()
	if err != nil {
		return fmt.Errorf("failed to run psql: %w", err)
	}

	return nil
}

func PsqlHelperSetupDatabaseAndUsers(ip, user, password, sslmode, database string, users []string) error {
	err := PsqlCommand(ip, user, password, sslmode, PGDefaultDatabase, fmt.Sprintf("CREATE DATABASE %q;", database))
	if err != nil {
		return err
	}
	for _, u := range users {
		err = PsqlCommand(ip, user, password, sslmode, PGDefaultDatabase, fmt.Sprintf("CREATE ROLE %q WITH LOGIN;", u))
		if err != nil {
			return err
		}
	}

	return nil
}

func PsqlHelperSetupDatabaseAndUsersDrop(ip, user, password, sslmode, database string, users []string) error {
	err := PsqlCommand(
		ip,
		user,
		password,
		sslmode,
		PGDefaultDatabase,
		fmt.Sprintf("DROP DATABASE IF EXISTS %q WITH (FORCE)", database),
	)
	if err != nil {
		return err
	}
	for _, u := range users {
		err = PsqlCommand(ip, user, password, sslmode, PGDefaultDatabase, fmt.Sprintf("DROP ROLE IF EXISTS %s", u))
		if err != nil {
			return err
		}
	}

	return nil
}
