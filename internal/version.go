package internal

import embeddedpostgres "github.com/fergusstrange/embedded-postgres"

// IMPORTANT: Keep these in sync so that the major versions match.
const (
	pgversionEmbeddedpostgres = embeddedpostgres.V18
	pgversionPgmodeler        = "18.0"
)
