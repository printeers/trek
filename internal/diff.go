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

func Diff(ctx context.Context, postgresConn, targetConn, migrateConn *pgx.Conn) (string, error) {
	fromDB := stdlib.OpenDB(*targetConn.Config())
	fromDB.SetMaxOpenConns(diffMaxOpenConns)
	defer fromDB.Close()
	err := fromDB.Ping()
	if err != nil {
		return "", fmt.Errorf("failed to open 'from database' connection: %w", err)
	}

	toDB := stdlib.OpenDB(*migrateConn.Config())
	toDB.SetMaxOpenConns(diffMaxOpenConns)
	defer toDB.Close()
	err = toDB.Ping()
	if err != nil {
		return "", fmt.Errorf("failed to open 'to database' connection: %w", err)
	}

	var deferredCloseFuncs []func() error
	defer func() {
		// call close funcs in inverted order (last first)
		for i := len(deferredCloseFuncs) - 1; i >= 0; i-- {
			if err := deferredCloseFuncs[i](); err != nil {
				log.Printf("failed to close resource: %v", err)
			}
		}
	}()
	tempFactory, err := tempdb.NewOnInstanceFactory(ctx, func(ctx context.Context, dbName string) (*sql.DB, error) {
		config := postgresConn.Config()
		config.Database = dbName
		tempDB := stdlib.OpenDB(*config)
		tempDB.SetMaxOpenConns(diffMaxOpenConns)
		err := tempDB.Ping()
		if err != nil {
			tempDB.Close()
			return nil, fmt.Errorf("failed to connect to temp database: %w", err)
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
		diff.WithNoConcurrentIndexOps(),     // Concurrent index creation is not available in transactions that are used by go-migrate.
	)
	if err != nil {
		return "", fmt.Errorf("failed to generate diff plan: %w", err)
	}

	// Ignore the proposed deletion of the schema_migrations table.
	plan.Statements = slices.DeleteFunc(plan.Statements, func(statement diff.Statement) bool {
		if statement.DDL == `DROP TABLE "public"."schema_migrations"` {
			return true
		}
		return false
	})

	return planToSql(plan), nil
}

// planToSql converts the plan to one large runnable SQL script.
//
// This function is copied verbatim from the pg-schema-diff cmd (package main)
// https://github.com/stripe/pg-schema-diff/blob/v1.0.2/cmd/pg-schema-diff/plan_cmd.go#L607C1-L628C2
// TODO: We could open an issue to ask them to expose this function, although it
// could be beneficial for trek to write our own custom sql output format.
// Licensed MIT by pg-schema-diff
func planToSql(plan diff.Plan) string {
	sb := strings.Builder{}
	for i, stmt := range plan.Statements {
		sb.WriteString("/*\n")
		sb.WriteString(fmt.Sprintf("Statement %d\n", i))
		if len(stmt.Hazards) > 0 {
			for _, hazard := range stmt.Hazards {
				sb.WriteString(fmt.Sprintf("  - %s\n", hazardToPrettyS(hazard)))
			}
		}
		sb.WriteString("*/\n")
		sb.WriteString(fmt.Sprintf("SET SESSION statement_timeout = %d;\n", stmt.Timeout.Milliseconds()))
		sb.WriteString(fmt.Sprintf("SET SESSION lock_timeout = %d;\n", stmt.LockTimeout.Milliseconds()))
		sb.WriteString(fmt.Sprintf("%s;", stmt.DDL))
		if i < len(plan.Statements)-1 {
			sb.WriteString("\n\n")
		}
	}
	return sb.String()
}

// hazardToPrettyS converts a migration hazard to a pretty string.
//
// Copied verbatim from the pg-schema-diff cmd, together with planToSql.
// Licensed MIT by pg-schema-diff
func hazardToPrettyS(hazard diff.MigrationHazard) string {
	if len(hazard.Message) > 0 {
		return fmt.Sprintf("%s: %s", hazard.Type, hazard.Message)
	} else {
		return hazard.Type
	}
}
