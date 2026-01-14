package internal

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"slices"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/stripe/pg-schema-diff/pkg/diff"
	"github.com/stripe/pg-schema-diff/pkg/tempdb"
)

const diffMaxOpenConns = 100

// Diff generates a SQL script to migrate the schema from the 'from' database to match the 'to' database.
// nolint:gocognit,cyclop
func Diff(ctx context.Context, postgresConn, fromConn, toConn *pgx.Conn) (string, error) {
	fromDB := stdlib.OpenDB(*fromConn.Config())
	fromDB.SetMaxOpenConns(diffMaxOpenConns)
	defer fromDB.Close()
	err := fromDB.PingContext(ctx)
	if err != nil {
		return "", fmt.Errorf("failed to open 'from database' connection: %w", err)
	}

	toDB := stdlib.OpenDB(*toConn.Config())
	toDB.SetMaxOpenConns(diffMaxOpenConns)
	defer toDB.Close()
	err = toDB.PingContext(ctx)
	if err != nil {
		return "", fmt.Errorf("failed to open 'to database' connection: %w", err)
	}

	var deferredCloseFuncs []func() error
	defer func() {
		// call close funcs in inverted order (last first)
		for i := len(deferredCloseFuncs) - 1; i >= 0; i-- {
			errClose := deferredCloseFuncs[i]()
			if errClose != nil {
				log.Printf("failed to close resource: %v", errClose)
			}
		}
	}()
	tempFactory, err := tempdb.NewOnInstanceFactory(ctx, func(ctx context.Context, dbName string) (*sql.DB, error) {
		config := postgresConn.Config()
		config.Database = dbName
		tempDB := stdlib.OpenDB(*config)
		tempDB.SetMaxOpenConns(diffMaxOpenConns)
		errPing := tempDB.PingContext(ctx)
		if errPing != nil {
			tempDB.Close()

			return nil, fmt.Errorf("failed to connect to temp database: %w", errPing)
		}
		deferredCloseFuncs = append(deferredCloseFuncs, tempDB.Close)

		return tempDB, nil
	})
	if err != nil {
		return "", fmt.Errorf("failed to create temp database factory: %w", err)
	}

	plan, err := diff.Generate(ctx,
		diff.DBSchemaSource(fromDB),
		diff.DBSchemaSource(toDB),
		diff.WithTempDbFactory(tempFactory), // Required to validate the generated diff statements.
		diff.WithNoConcurrentIndexOps(),     // Concurrent index creation is not available in transactions.
		diff.WithDoNotValidatePlan(),        // See https://github.com/stripe/pg-schema-diff/issues/266
	)
	if err != nil {
		return "", fmt.Errorf("failed to generate diff plan: %w", err)
	}

	// Ignore the proposed deletion of the schema_migrations table.
	plan.Statements = slices.DeleteFunc(plan.Statements, func(statement diff.Statement) bool {
		return statement.DDL == `DROP TABLE "public"."schema_migrations"`
	})

	sb := strings.Builder{}
	var lastStatementTimeout int64
	var lastLockTimeout int64
	for i, stmt := range plan.Statements {
		statementTimeout := stmt.Timeout.Milliseconds()
		lockTimeout := stmt.LockTimeout.Milliseconds()
		if lastStatementTimeout != statementTimeout || lastLockTimeout != lockTimeout {
			if lastStatementTimeout != statementTimeout {
				lastStatementTimeout = statementTimeout
				sb.WriteString(fmt.Sprintf("SET SESSION statement_timeout = %d;\n", statementTimeout))
			}
			if lastLockTimeout != lockTimeout {
				lastLockTimeout = lockTimeout
				sb.WriteString(fmt.Sprintf("SET SESSION lock_timeout = %d;\n", lockTimeout))
			}
			sb.WriteString("\n")
		}
		if len(stmt.Hazards) > 0 {
			sb.WriteString("/* Hazards:\n")
			for _, hazard := range stmt.Hazards {
				if hazard.Message != "" {
					sb.WriteString(fmt.Sprintf(" - %s: %s\n", hazard.Type, hazard.Message))
				} else {
					sb.WriteString(fmt.Sprintf(" - %s\n", hazard.Type))
				}
			}
			sb.WriteString("*/\n")
		}
		sb.WriteString(fmt.Sprintf("%s;\n", stmt.DDL))
		if i < len(plan.Statements)-1 {
			sb.WriteString("\n")
		}
	}

	return sb.String(), nil
}
