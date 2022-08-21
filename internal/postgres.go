package internal

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"

	embeddedpostgres "github.com/fergusstrange/embedded-postgres"
	"github.com/jackc/pgx/v4"
)

func NewPostgresDatabase(runtimePath string, port uint32) (postgres *embeddedpostgres.EmbeddedPostgres, dsn string) {
	var buf bytes.Buffer

	return embeddedpostgres.NewDatabase(
		embeddedpostgres.
			DefaultConfig().
			Logger(&buf).
			Version(embeddedpostgres.V13).
			RuntimePath(runtimePath).
			Username("postgres").
			Password("postgres").
			Port(port).
			Database("postgres"),
	), fmt.Sprintf("postgres://postgres:postgres@127.0.0.1:%d/postgres", port)
}

func PgDump(dsn string, args []string) (string, error) {
	cmd := []string{
		"pg_dump",
		"--dbname",
		dsn,
	}
	cmd = append(cmd, args...)

	//nolint:gosec
	cmdPgDump := exec.Command(cmd[0], cmd[1:]...)
	cmdPgDump.Stderr = os.Stderr

	stdout, err := cmdPgDump.Output()
	if err != nil {
		return "", fmt.Errorf("failed to run pg_dump: %w", err)
	}

	return string(stdout), nil
}

func PsqlFile(dsn, file string) error {
	cmdPsql := exec.Command(
		"psql",
		"--echo-errors",
		"--variable",
		"ON_ERROR_STOP=1",
		"--dbname",
		dsn,
		"--file",
		file,
	)
	cmdPsql.Stderr = os.Stderr

	err := cmdPsql.Run()
	if err != nil {
		return fmt.Errorf("failed to run psql: %w", err)
	}

	return nil
}

func CreateUsers(conn *pgx.Conn, users []string) error {
	for _, u := range users {
		_, err := conn.Exec(context.Background(), fmt.Sprintf("CREATE ROLE %q WITH LOGIN;", u))
		if err != nil {
			return fmt.Errorf("failed to create user: %w", err)
		}
	}

	return nil
}

func CheckDatabaseExists(conn *pgx.Conn, user string) (bool, error) {
	a := conn.QueryRow(
		context.Background(),
		fmt.Sprintf("SELECT EXISTS(SELECT 1 FROM pg_database WHERE datname='%s');", user),
	)

	var b bool
	err := a.Scan(&b)
	if err != nil {
		return false, fmt.Errorf("failed to decode row: %w", err)
	}

	return b, nil
}

func CheckUserExists(conn *pgx.Conn, user string) (bool, error) {
	a := conn.QueryRow(
		context.Background(),
		fmt.Sprintf("SELECT EXISTS(SELECT 1 FROM pg_roles WHERE rolname='%s');", user),
	)

	var b bool
	err := a.Scan(&b)
	if err != nil {
		return false, fmt.Errorf("failed to decode row: %w", err)
	}

	return b, nil
}

func CheckTableExists(conn *pgx.Conn, schema, name string) (bool, error) {
	a := conn.QueryRow(
		context.Background(),
		fmt.Sprintf("SELECT EXISTS(SELECT FROM pg_tables WHERE schemaname = '%s' AND tablename = '%s');", schema, name),
	)

	var b bool
	err := a.Scan(&b)
	if err != nil {
		return false, fmt.Errorf("failed to decode row: %w", err)
	}

	return b, nil
}

func DSN(conn *pgx.Conn, sslmode string) string {
	config := conn.Config()

	return fmt.Sprintf(
		"postgresql://%s:%s@%s:%d/%s?sslmode=%s",
		config.User,
		config.Password,
		config.Host,
		config.Port,
		config.Database,
		sslmode,
	)
}
