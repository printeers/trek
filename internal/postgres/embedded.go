package postgres

import (
	"bytes"
	"fmt"
	"os"

	embeddedpostgres "github.com/fergusstrange/embedded-postgres"
	"github.com/printeers/trek/internal"
)

var _ Database = &postgresDatabaseEmbedded{}

type postgresDatabaseEmbedded struct {
	port   uint32
	db     *embeddedpostgres.EmbeddedPostgres
	tmpDir string
}

func (p *postgresDatabaseEmbedded) Start(port uint32) error {
	p.port = port

	var buf bytes.Buffer

	tmpDir, err := os.MkdirTemp("", "postgres-")
	if err != nil {
		return fmt.Errorf("failed to create temporary directory: %w", err)
	}

	p.tmpDir = tmpDir

	p.db = embeddedpostgres.NewDatabase(
		embeddedpostgres.
			DefaultConfig().
			Logger(&buf).
			Version(internal.PgversionEmbeddedpostgres). // keep in sync with pgmodeler.go
			RuntimePath(tmpDir).
			Username("postgres").
			Password("postgres").
			Port(port).
			Database("postgres"),
	)

	err = p.db.Start()
	if err != nil {
		return fmt.Errorf("failed to start database: %w", err)
	}

	return nil
}

func (p *postgresDatabaseEmbedded) Stop() error {
	err := p.db.Stop()
	if err != nil {
		return fmt.Errorf("failed to stop database: %w", err)
	}

	p.db = nil
	p.port = 0

	err = os.RemoveAll(p.tmpDir)
	if err != nil {
		return fmt.Errorf("failed to delete temporary directory: %w", err)
	}

	p.tmpDir = ""

	return nil
}

func (p *postgresDatabaseEmbedded) DSN(database string) string {
	return fmt.Sprintf("postgres://postgres:postgres@127.0.0.1:%d/%s?sslmode=disable", p.port, database)
}
