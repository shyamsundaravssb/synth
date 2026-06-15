// Package store — embeddings.go
//
// Implements embedding-specific store operations: writing and reading
// float32 vectors serialised as little-endian binary BLOBs, daemon state
// key-value persistence, and FTS5 full-text search over intent notes.
package store

import (
	"context"
	"database/sql"
	"encoding/binary"
	"math"
	"strings"
	"time"

	"github.com/shyamsundaravssb/synth/pkg/types"
)

// ─── EmbeddingRecord ──────────────────────────────────────────────────────────

// EmbeddingRecord holds a single embedding vector and its metadata.
type EmbeddingRecord struct {
	IntentID  string
	ProjectID string
	Embedding []float32
	Model     string
	CreatedAt time.Time
}

// ─── Float32 serialisation helpers ───────────────────────────────────────────

// float32SliceToBytes encodes a []float32 to a little-endian binary []byte.
// Each float32 occupies exactly 4 bytes (IEEE 754 representation).
func float32SliceToBytes(v []float32) []byte {
	buf := make([]byte, len(v)*4)
	for i, f := range v {
		binary.LittleEndian.PutUint32(buf[i*4:], math.Float32bits(f))
	}
	return buf
}

// bytesToFloat32Slice decodes a little-endian binary []byte back to []float32.
// len(b) must be a multiple of 4; extra trailing bytes are silently ignored.
func bytesToFloat32Slice(b []byte) []float32 {
	v := make([]float32, len(b)/4)
	for i := range v {
		v[i] = math.Float32frombits(binary.LittleEndian.Uint32(b[i*4:]))
	}
	return v
}

// ─── InsertEmbedding ─────────────────────────────────────────────────────────

// InsertEmbedding stores or replaces the embedding vector for an intent.
// The []float32 slice is serialised to a little-endian binary BLOB before
// storage. INSERT OR REPLACE semantics allow re-embedding the same intent.
func (s *SQLiteStore) InsertEmbedding(ctx context.Context, record EmbeddingRecord) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	blob := float32SliceToBytes(record.Embedding)

	_, err := s.db.ExecContext(ctx,
		`INSERT OR REPLACE INTO intent_embeddings
		 (intent_id, project_id, embedding, model, created_at)
		 VALUES (?, ?, ?, ?, ?)`,
		record.IntentID,
		record.ProjectID,
		blob,
		record.Model,
		time.Now().UnixMilli(),
	)
	return err
}

// ─── GetEmbedding ────────────────────────────────────────────────────────────

// GetEmbedding retrieves the embedding record for the given intent ID.
// Returns (nil, nil) if no embedding exists for that intent.
func (s *SQLiteStore) GetEmbedding(ctx context.Context, intentID string) (*EmbeddingRecord, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	var rec EmbeddingRecord
	var blob []byte
	var createdAtMs int64

	err := s.db.QueryRowContext(ctx,
		`SELECT intent_id, project_id, embedding, model, created_at
		 FROM intent_embeddings
		 WHERE intent_id = ?`,
		intentID,
	).Scan(&rec.IntentID, &rec.ProjectID, &blob, &rec.Model, &createdAtMs)

	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	rec.Embedding = bytesToFloat32Slice(blob)
	rec.CreatedAt = time.UnixMilli(createdAtMs)
	return &rec, nil
}

// ─── ListIntentsWithoutEmbeddings ────────────────────────────────────────────

// ListIntentsWithoutEmbeddings returns intents for a project that have not yet
// been embedded. Results are ordered oldest-first so the embedding loop
// processes them in chronological order. A limit of 0 defaults to 100.
func (s *SQLiteStore) ListIntentsWithoutEmbeddings(
	ctx context.Context,
	projectID string,
	limit int,
) ([]types.Intent, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	if limit <= 0 {
		limit = 100
	}

	rows, err := s.db.QueryContext(ctx,
		`SELECT i.id, i.project_id, i.file_path,
		        i.branch, i.commit_hash, i.developer,
		        i.timestamp, i.type, i.what, i.why,
		        i.impact, i.context
		 FROM intents i
		 LEFT JOIN intent_embeddings e ON i.id = e.intent_id
		 WHERE i.project_id = ?
		   AND e.intent_id IS NULL
		 ORDER BY i.timestamp ASC
		 LIMIT ?`,
		projectID, limit,
	)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	return scanIntents(rows)
}

// ─── GetDaemonState ──────────────────────────────────────────────────────────

// GetDaemonState retrieves a persisted daemon state value by project and key.
// Returns ("", nil) if the key does not exist.
func (s *SQLiteStore) GetDaemonState(ctx context.Context, projectID, key string) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}

	var value string
	err := s.db.QueryRowContext(ctx,
		`SELECT value FROM daemon_state
		 WHERE project_id = ? AND key = ?`,
		projectID, key,
	).Scan(&value)

	if err == sql.ErrNoRows {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	return value, nil
}

// ─── SetDaemonState ──────────────────────────────────────────────────────────

// SetDaemonState upserts a key-value pair for a project into daemon_state.
// The updated_at timestamp is set to the current time in milliseconds.
func (s *SQLiteStore) SetDaemonState(ctx context.Context, projectID, key, value string) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	_, err := s.db.ExecContext(ctx,
		`INSERT OR REPLACE INTO daemon_state
		 (key, project_id, value, updated_at)
		 VALUES (?, ?, ?, ?)`,
		key,
		projectID,
		value,
		time.Now().UnixMilli(),
	)
	return err
}

// ─── SearchFTS ───────────────────────────────────────────────────────────────

// SearchFTS performs a full-text search over intent notes using SQLite FTS5.
// The query string is passed directly to the FTS5 MATCH operator and supports
// standard FTS5 query syntax (keywords, phrases, prefix queries, etc.).
// Results are ordered by FTS5 relevance rank (most relevant first).
// A limit of 0 defaults to 20.
func (s *SQLiteStore) SearchFTS(
	ctx context.Context,
	projectID, query string,
	limit int,
) ([]types.Intent, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	if limit <= 0 {
		limit = 20
	}

	safeQuery := sanitizeFTSQuery(query)

	rows, err := s.db.QueryContext(ctx,
		`SELECT i.id, i.project_id, i.file_path,
		        i.branch, i.commit_hash, i.developer,
		        i.timestamp, i.type, i.what, i.why,
		        i.impact, i.context
		 FROM intents i
		 JOIN intents_fts f ON i.id = f.intent_id
		 WHERE intents_fts MATCH ?
		   AND i.project_id = ?
		 ORDER BY rank
		 LIMIT ?`,
		safeQuery, projectID, limit,
	)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	return scanIntents(rows)
}

// ─── CountEmbeddings ─────────────────────────────────────────────────────────

// CountEmbeddings returns the total number of embeddings for a project.
func (s *SQLiteStore) CountEmbeddings(ctx context.Context, projectID string) (int, error) {
	if err := ctx.Err(); err != nil {
		return 0, err
	}

	var count int
	err := s.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM intent_embeddings WHERE project_id = ?`,
		projectID,
	).Scan(&count)
	return count, err
}

func sanitizeFTSQuery(query string) string {
	var sb strings.Builder
	for _, r := range query {
		switch r {
		case '"', '\'', '*', '^', '(', ')', '[', ']', '{', '}':
			continue
		default:
			sb.WriteRune(r)
		}
	}
	s := strings.TrimSpace(sb.String())
	if s == "" {
		return query
	}
	return s
}

// ─── GetAllEmbeddings ────────────────────────────────────────────────────────

// GetAllEmbeddings retrieves all embedding records for the specified project.
func (s *SQLiteStore) GetAllEmbeddings(ctx context.Context, projectID string) ([]EmbeddingRecord, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	rows, err := s.db.QueryContext(ctx,
		`SELECT intent_id, project_id, embedding, model, created_at
		 FROM intent_embeddings
		 WHERE project_id = ?`,
		projectID,
	)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var results []EmbeddingRecord
	for rows.Next() {
		var rec EmbeddingRecord
		var blob []byte
		var createdAtMs int64

		if err := rows.Scan(&rec.IntentID, &rec.ProjectID, &blob, &rec.Model, &createdAtMs); err != nil {
			return nil, err
		}

		rec.Embedding = bytesToFloat32Slice(blob)
		rec.CreatedAt = time.UnixMilli(createdAtMs)
		results = append(results, rec)
	}
	return results, rows.Err()
}
