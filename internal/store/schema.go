package store

import _ "embed"

// initialSchemaSQL contains the Phase 0 initial database schema.
//
//go:embed migrations/001_initial.sql
var initialSchemaSQL string

// migration002SQL contains the Phase 1 schema additions: intent_embeddings,
// daemon_state, intents_fts virtual table, and FTS5 sync triggers.
//
//go:embed migrations/002_phase1.sql
var migration002SQL string
