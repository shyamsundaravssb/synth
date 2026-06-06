package store

import (
	"database/sql"
	"fmt"
	"time"
)

// Migration represents a single database schema migration.
type Migration struct {
	Version     int
	Description string
	SQL         string
}

// migrations is the ordered list of all schema migrations.
// New migrations are appended here with incrementing version numbers.
var migrations = []Migration{
	{Version: 1, Description: "Initial schema", SQL: initialSchemaSQL},
}

// RunMigrations applies any pending migrations to the database.
// It is idempotent — safe to call on an already-migrated database.
// Each migration runs inside its own transaction; a failure triggers
// an immediate rollback and returns the error.
func RunMigrations(db *sql.DB) error {
	current, err := getCurrentVersion(db)
	if err != nil {
		return fmt.Errorf("reading migration version: %w", err)
	}

	for _, m := range migrations {
		if m.Version <= current {
			continue
		}
		if err := applyMigration(db, m); err != nil {
			return err
		}
	}

	return nil
}

// applyMigration runs a single migration inside a transaction.
func applyMigration(db *sql.DB, m Migration) error {
	tx, err := db.Begin()
	if err != nil {
		return fmt.Errorf("begin migration %d: %w", m.Version, err)
	}

	if _, err := tx.Exec(m.SQL); err != nil {
		tx.Rollback()
		return fmt.Errorf("exec migration %d (%s): %w", m.Version, m.Description, err)
	}

	if _, err := tx.Exec(
		"INSERT INTO schema_migrations (version, applied_at, description) VALUES (?, ?, ?)",
		m.Version, time.Now().UnixMilli(), m.Description,
	); err != nil {
		tx.Rollback()
		return fmt.Errorf("record migration %d: %w", m.Version, err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit migration %d: %w", m.Version, err)
	}

	return nil
}

// getCurrentVersion returns the highest migration version that has been
// applied. If the schema_migrations table does not exist yet, it returns 0.
func getCurrentVersion(db *sql.DB) (int, error) {
	var count int
	err := db.QueryRow(
		"SELECT count(*) FROM sqlite_master WHERE type='table' AND name='schema_migrations'",
	).Scan(&count)
	if err != nil {
		return 0, err
	}
	if count == 0 {
		return 0, nil
	}

	var maxVersion sql.NullInt64
	if err := db.QueryRow("SELECT MAX(version) FROM schema_migrations").Scan(&maxVersion); err != nil {
		return 0, err
	}
	if !maxVersion.Valid {
		return 0, nil
	}
	return int(maxVersion.Int64), nil
}
