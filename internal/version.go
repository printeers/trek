package internal

import embeddedpostgres "github.com/fergusstrange/embedded-postgres"

// IMPORTANT: Keep these in sync so that the major versions match.
var (
	pgversionEmbeddedpostgres = embeddedpostgres.V15
	pgversionPgmodeler        = "15.0"
)
