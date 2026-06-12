package store

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"

	_ "github.com/mattn/go-sqlite3"
)

// pragmas are applied immediately after opening a connection.
var pragmas = []string{
	"PRAGMA journal_mode=WAL",
	"PRAGMA synchronous=NORMAL",
	"PRAGMA foreign_keys=ON",
	"PRAGMA busy_timeout=5000",
}

// Open creates or opens a SQLite database at dbPath, applies pragmas for
// optimal performance, and runs any pending migrations. The caller is
// responsible for calling db.Close() when finished.
func Open(dbPath string) (retDB *sql.DB, retErr error) {
	// Create parent directories if they do not exist.
	if err := os.MkdirAll(filepath.Dir(dbPath), 0o755); err != nil {
		return nil, fmt.Errorf("creating db directory: %w", err)
	}

	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return nil, fmt.Errorf("opening database: %w", err)
	}

	// Ensure cleanup on any subsequent error.
	defer func() {
		if retErr != nil {
			_ = db.Close()
		}
	}()

	// SQLite does not support concurrent writers.
	db.SetMaxOpenConns(1)

	if err := applyPragmas(db); err != nil {
		return nil, err
	}

	if err := RunMigrations(db); err != nil {
		return nil, fmt.Errorf("running migrations: %w", err)
	}

	return db, nil
}

// applyPragmas sets SQLite connection pragmas for WAL mode, performance,
// and safety.
func applyPragmas(db *sql.DB) error {
	for _, p := range pragmas {
		if _, err := db.Exec(p); err != nil {
			return fmt.Errorf("setting pragma: %w", err)
		}
	}
	return nil
}

// DBPath returns the canonical database file path for a given project:
//
//	~/.synth/projects/<projectID>/intent.db
func DBPath(projectID string) string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".synth", "projects", projectID, "intent.db")
}
