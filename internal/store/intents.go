package store

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/shyamsundaravssb/synth/pkg/types"
)

// IntentFilter controls which intents are returned by ListIntents.
type IntentFilter struct {
	ProjectID string
	FilePath  string
	Branch    string
	Developer string
	Since     time.Time
	Limit     int
}

// Store defines the data-access contract for the Synth intent store.
type Store interface {
	InsertIntent(ctx context.Context, intent types.Intent) error
	UpsertFileRegistry(ctx context.Context, entry types.FileEntry) error
	UpdateIntentCommitHash(ctx context.Context, id, hash string) error
	ListIntents(ctx context.Context, filter IntentFilter) ([]types.Intent, error)
	GetFileRegistry(ctx context.Context, projectID, filePath string) (*types.FileEntry, error)
	GetRecentIntents(ctx context.Context, projectID string, limit int) ([]types.Intent, error)
	CountIntents(ctx context.Context, projectID string) (int, error)
	GetLowContextFiles(ctx context.Context, projectID string, threshold int) ([]string, error)
	UpdateUncommittedIntents(ctx context.Context, projectID, commitHash string) (int, error)
}

// SQLiteStore implements Store backed by a SQLite database.
type SQLiteStore struct {
	db *sql.DB
}

// NewSQLiteStore creates a new SQLiteStore from an already-opened database.
func NewSQLiteStore(db *sql.DB) *SQLiteStore {
	return &SQLiteStore{db: db}
}

// InsertIntent stores a single intent record.
func (s *SQLiteStore) InsertIntent(ctx context.Context, intent types.Intent) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	_, err := s.db.ExecContext(ctx,
		`INSERT INTO intents
		 (id, project_id, file_path, branch, commit_hash, developer,
		  timestamp, type, what, why, impact, context)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		intent.ID,
		intent.ProjectID,
		intent.FilePath,
		intent.Branch,
		intent.CommitHash,
		intent.Developer,
		intent.Timestamp.UnixMilli(),
		string(intent.Type),
		intent.What,
		intent.Why,
		intent.Impact,
		string(intent.Context),
	)
	return err
}

// UpsertFileRegistry inserts or replaces a file registry entry.
// The composite key is (file_path, project_id).
func (s *SQLiteStore) UpsertFileRegistry(ctx context.Context, entry types.FileEntry) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	_, err := s.db.ExecContext(ctx,
		`INSERT OR REPLACE INTO file_registry
		 (file_path, project_id, purpose, owns, boundary, created_by, created_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?)`,
		entry.FilePath,
		entry.ProjectID,
		entry.Purpose,
		entry.Owns,
		entry.Boundary,
		entry.CreatedBy,
		entry.CreatedAt.UnixMilli(),
	)
	return err
}

// UpdateIntentCommitHash sets the commit hash on an existing intent.
func (s *SQLiteStore) UpdateIntentCommitHash(ctx context.Context, id, hash string) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	_, err := s.db.ExecContext(ctx,
		`UPDATE intents SET commit_hash = ? WHERE id = ?`,
		hash, id,
	)
	return err
}

// UpdateUncommittedIntents assigns the commitHash to all intents for projectID that do not have one yet.
// Returns the number of intents updated.
func (s *SQLiteStore) UpdateUncommittedIntents(ctx context.Context, projectID, commitHash string) (int, error) {
	if err := ctx.Err(); err != nil {
		return 0, err
	}

	res, err := s.db.ExecContext(ctx,
		`UPDATE intents
		 SET commit_hash = ?
		 WHERE project_id = ?
		 AND (commit_hash = '' OR commit_hash IS NULL)`,
		commitHash, projectID,
	)
	if err != nil {
		return 0, err
	}
	
	rows, err := res.RowsAffected()
	return int(rows), err
}

// ListIntents returns intents matching the filter, ordered by timestamp DESC.
// A Limit of 0 defaults to 20.
func (s *SQLiteStore) ListIntents(ctx context.Context, filter IntentFilter) ([]types.Intent, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	var clauses []string
	var args []interface{}

	if filter.ProjectID != "" {
		clauses = append(clauses, "project_id = ?")
		args = append(args, filter.ProjectID)
	}
	if filter.FilePath != "" {
		clauses = append(clauses, "file_path = ?")
		args = append(args, filter.FilePath)
	}
	if filter.Branch != "" {
		clauses = append(clauses, "branch = ?")
		args = append(args, filter.Branch)
	}
	if filter.Developer != "" {
		clauses = append(clauses, "developer = ?")
		args = append(args, filter.Developer)
	}
	if !filter.Since.IsZero() {
		clauses = append(clauses, "timestamp >= ?")
		args = append(args, filter.Since.UnixMilli())
	}

	query := `SELECT id, project_id, file_path, branch, commit_hash,
	                 developer, timestamp, type, what, why, impact, context
	          FROM intents`
	if len(clauses) > 0 {
		query += " WHERE " + strings.Join(clauses, " AND ")
	}
	query += " ORDER BY timestamp DESC"

	limit := filter.Limit
	if limit <= 0 {
		limit = 20
	}
	query += fmt.Sprintf(" LIMIT %d", limit)

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return scanIntents(rows)
}

// GetFileRegistry retrieves a single file registry entry.
// Returns (nil, nil) if the entry does not exist.
func (s *SQLiteStore) GetFileRegistry(ctx context.Context, projectID, filePath string) (*types.FileEntry, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	var entry types.FileEntry
	var createdAtMs int64

	err := s.db.QueryRowContext(ctx,
		`SELECT file_path, project_id, purpose, owns, boundary, created_by, created_at
		 FROM file_registry
		 WHERE project_id = ? AND file_path = ?`,
		projectID, filePath,
	).Scan(
		&entry.FilePath, &entry.ProjectID, &entry.Purpose,
		&entry.Owns, &entry.Boundary, &entry.CreatedBy, &createdAtMs,
	)

	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	entry.CreatedAt = time.UnixMilli(createdAtMs)
	return &entry, nil
}

// GetRecentIntents returns the most recent intents for a project.
// A limit of 0 defaults to 20.
func (s *SQLiteStore) GetRecentIntents(ctx context.Context, projectID string, limit int) ([]types.Intent, error) {
	if limit <= 0 {
		limit = 20
	}
	return s.ListIntents(ctx, IntentFilter{
		ProjectID: projectID,
		Limit:     limit,
	})
}

// CountIntents returns the total number of intents for a project.
func (s *SQLiteStore) CountIntents(ctx context.Context, projectID string) (int, error) {
	if err := ctx.Err(); err != nil {
		return 0, err
	}

	var count int
	err := s.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM intents WHERE project_id = ?`,
		projectID,
	).Scan(&count)
	return count, err
}

// GetLowContextFiles retrieves files saved without notes more than threshold times.
func (s *SQLiteStore) GetLowContextFiles(ctx context.Context, projectID string, threshold int) ([]string, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	rows, err := s.db.QueryContext(ctx,
		`SELECT file_path FROM file_saves
		 WHERE project_id = ? AND has_note = 0
		 GROUP BY file_path
		 HAVING COUNT(*) >= ?`,
		projectID, threshold,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var files []string
	for rows.Next() {
		var f string
		if err := rows.Scan(&f); err != nil {
			return nil, err
		}
		files = append(files, f)
	}
	return files, rows.Err()
}

// scanIntents reads intent rows into a slice. It handles nullable
// commit_hash and converts Unix-millisecond timestamps back to time.Time.
func scanIntents(rows *sql.Rows) ([]types.Intent, error) {
	var intents []types.Intent
	for rows.Next() {
		var i types.Intent
		var tsMs int64
		var commitHash sql.NullString
		var intentType, contextLevel string

		if err := rows.Scan(
			&i.ID, &i.ProjectID, &i.FilePath, &i.Branch,
			&commitHash, &i.Developer, &tsMs,
			&intentType, &i.What, &i.Why, &i.Impact, &contextLevel,
		); err != nil {
			return nil, err
		}

		i.Timestamp = time.UnixMilli(tsMs)
		i.Type = types.IntentType(intentType)
		i.Context = types.ContextLevel(contextLevel)
		if commitHash.Valid {
			i.CommitHash = commitHash.String
		}

		intents = append(intents, i)
	}
	return intents, rows.Err()
}
