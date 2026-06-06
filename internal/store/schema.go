package store

import _ "embed"

// initialSchemaSQL contains the initial database schema embedded from the
// migrations directory. This keeps the SQL in a dedicated .sql file for
// readability while making it available at compile time.
//
//go:embed migrations/001_initial.sql
var initialSchemaSQL string
