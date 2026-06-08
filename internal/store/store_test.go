package store

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/shyamsundaravssb/synth/pkg/types"
)

// setupTestStore creates a temporary SQLite database with migrations applied
// and returns a ready-to-use *SQLiteStore.
func setupTestStore(t *testing.T) *SQLiteStore {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "test.db")
	db, err := Open(dbPath)
	if err != nil {
		t.Fatalf("Open(%q) error = %v", dbPath, err)
	}
	t.Cleanup(func() { db.Close() })
	return NewSQLiteStore(db)
}

// makeIntent is a test helper that builds an Intent with sensible defaults.
func makeIntent(id, projectID, filePath string) types.Intent {
	return types.Intent{
		ID:        id,
		ProjectID: projectID,
		FilePath:  filePath,
		Branch:    "main",
		Developer: "tester",
		Timestamp: time.Now(),
		Type:      types.IntentChange,
		What:      "changed " + filePath,
		Why:       "testing",
		Impact:    "none",
		Context:   types.ContextNormal,
	}
}

// ---------------------------------------------------------------------------
// Tests
// ---------------------------------------------------------------------------

func TestInsertAndRetrieveIntent(t *testing.T) {
	s := setupTestStore(t)
	ctx := context.Background()

	want := makeIntent("i-1", "proj-1", "main.go")
	want.Branch = "feature/auth"
	want.CommitHash = "abc123"
	want.Impact = "high"
	want.Context = types.ContextLow
	// Truncate to milliseconds since we store as UnixMilli.
	want.Timestamp = want.Timestamp.Truncate(time.Millisecond)

	if err := s.InsertIntent(ctx, want); err != nil {
		t.Fatalf("InsertIntent() error = %v", err)
	}

	got, err := s.ListIntents(ctx, IntentFilter{ProjectID: "proj-1"})
	if err != nil {
		t.Fatalf("ListIntents() error = %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("got %d intents, want 1", len(got))
	}

	g := got[0]
	if g.ID != want.ID {
		t.Errorf("ID = %q, want %q", g.ID, want.ID)
	}
	if g.ProjectID != want.ProjectID {
		t.Errorf("ProjectID = %q, want %q", g.ProjectID, want.ProjectID)
	}
	if g.FilePath != want.FilePath {
		t.Errorf("FilePath = %q, want %q", g.FilePath, want.FilePath)
	}
	if g.Branch != want.Branch {
		t.Errorf("Branch = %q, want %q", g.Branch, want.Branch)
	}
	if g.CommitHash != want.CommitHash {
		t.Errorf("CommitHash = %q, want %q", g.CommitHash, want.CommitHash)
	}
	if g.Developer != want.Developer {
		t.Errorf("Developer = %q, want %q", g.Developer, want.Developer)
	}
	if !g.Timestamp.Equal(want.Timestamp) {
		t.Errorf("Timestamp = %v, want %v", g.Timestamp, want.Timestamp)
	}
	if g.Type != want.Type {
		t.Errorf("Type = %q, want %q", g.Type, want.Type)
	}
	if g.What != want.What {
		t.Errorf("What = %q, want %q", g.What, want.What)
	}
	if g.Why != want.Why {
		t.Errorf("Why = %q, want %q", g.Why, want.Why)
	}
	if g.Impact != want.Impact {
		t.Errorf("Impact = %q, want %q", g.Impact, want.Impact)
	}
	if g.Context != want.Context {
		t.Errorf("Context = %q, want %q", g.Context, want.Context)
	}
}

func TestListIntentsFilterByFile(t *testing.T) {
	s := setupTestStore(t)
	ctx := context.Background()

	files := []string{"main.go", "utils.go", "main.go"}
	for i, f := range files {
		intent := makeIntent(fmt.Sprintf("f-%d", i), "proj-1", f)
		if err := s.InsertIntent(ctx, intent); err != nil {
			t.Fatalf("InsertIntent(%d) error = %v", i, err)
		}
	}

	got, err := s.ListIntents(ctx, IntentFilter{
		ProjectID: "proj-1",
		FilePath:  "main.go",
	})
	if err != nil {
		t.Fatalf("ListIntents() error = %v", err)
	}
	if len(got) != 2 {
		t.Errorf("got %d intents for main.go, want 2", len(got))
	}
	for _, g := range got {
		if g.FilePath != "main.go" {
			t.Errorf("unexpected FilePath %q in filtered results", g.FilePath)
		}
	}
}

func TestListIntentsFilterByBranch(t *testing.T) {
	s := setupTestStore(t)
	ctx := context.Background()

	branches := []string{"main", "feature/x", "main", "feature/y"}
	for i, b := range branches {
		intent := makeIntent(fmt.Sprintf("b-%d", i), "proj-1", "app.go")
		intent.Branch = b
		if err := s.InsertIntent(ctx, intent); err != nil {
			t.Fatalf("InsertIntent(%d) error = %v", i, err)
		}
	}

	got, err := s.ListIntents(ctx, IntentFilter{
		ProjectID: "proj-1",
		Branch:    "main",
	})
	if err != nil {
		t.Fatalf("ListIntents() error = %v", err)
	}
	if len(got) != 2 {
		t.Errorf("got %d intents for branch main, want 2", len(got))
	}
}

func TestListIntentsDefaultLimit(t *testing.T) {
	s := setupTestStore(t)
	ctx := context.Background()

	for i := 0; i < 25; i++ {
		intent := makeIntent(fmt.Sprintf("l-%d", i), "proj-1", "bulk.go")
		intent.Timestamp = time.Now().Add(time.Duration(i) * time.Second)
		if err := s.InsertIntent(ctx, intent); err != nil {
			t.Fatalf("InsertIntent(%d) error = %v", i, err)
		}
	}

	got, err := s.ListIntents(ctx, IntentFilter{
		ProjectID: "proj-1",
		Limit:     0, // Should default to 20.
	})
	if err != nil {
		t.Fatalf("ListIntents() error = %v", err)
	}
	if len(got) != 20 {
		t.Errorf("got %d intents with zero limit, want 20", len(got))
	}
}

func TestUpsertFileRegistry(t *testing.T) {
	s := setupTestStore(t)
	ctx := context.Background()
	now := time.Now().Truncate(time.Millisecond)

	entry := types.FileEntry{
		FilePath:  "pkg/api/handler.go",
		ProjectID: "proj-1",
		Purpose:   "HTTP handler",
		Owns:      "routes",
		Boundary:  "api",
		CreatedBy: "alice",
		CreatedAt: now,
	}

	// Initial insert.
	if err := s.UpsertFileRegistry(ctx, entry); err != nil {
		t.Fatalf("UpsertFileRegistry() error = %v", err)
	}

	got, err := s.GetFileRegistry(ctx, "proj-1", "pkg/api/handler.go")
	if err != nil {
		t.Fatalf("GetFileRegistry() error = %v", err)
	}
	if got == nil {
		t.Fatal("GetFileRegistry() returned nil, want entry")
	}
	if got.Purpose != "HTTP handler" {
		t.Errorf("Purpose = %q, want %q", got.Purpose, "HTTP handler")
	}
	if got.CreatedBy != "alice" {
		t.Errorf("CreatedBy = %q, want %q", got.CreatedBy, "alice")
	}
	if !got.CreatedAt.Equal(now) {
		t.Errorf("CreatedAt = %v, want %v", got.CreatedAt, now)
	}

	// Upsert with changed values — same composite key.
	entry.Purpose = "REST handler"
	entry.CreatedBy = "bob"
	if err := s.UpsertFileRegistry(ctx, entry); err != nil {
		t.Fatalf("UpsertFileRegistry() upsert error = %v", err)
	}

	got, err = s.GetFileRegistry(ctx, "proj-1", "pkg/api/handler.go")
	if err != nil {
		t.Fatalf("GetFileRegistry() after upsert error = %v", err)
	}
	if got.Purpose != "REST handler" {
		t.Errorf("Purpose after upsert = %q, want %q", got.Purpose, "REST handler")
	}
	if got.CreatedBy != "bob" {
		t.Errorf("CreatedBy after upsert = %q, want %q", got.CreatedBy, "bob")
	}
}

func TestUpdateIntentCommitHash(t *testing.T) {
	s := setupTestStore(t)
	ctx := context.Background()

	intent := makeIntent("hash-1", "proj-1", "main.go")
	intent.CommitHash = "" // Empty initially.
	if err := s.InsertIntent(ctx, intent); err != nil {
		t.Fatalf("InsertIntent() error = %v", err)
	}

	// Update the commit hash.
	if err := s.UpdateIntentCommitHash(ctx, "hash-1", "deadbeef"); err != nil {
		t.Fatalf("UpdateIntentCommitHash() error = %v", err)
	}

	got, err := s.ListIntents(ctx, IntentFilter{ProjectID: "proj-1"})
	if err != nil {
		t.Fatalf("ListIntents() error = %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("got %d intents, want 1", len(got))
	}
	if got[0].CommitHash != "deadbeef" {
		t.Errorf("CommitHash = %q, want %q", got[0].CommitHash, "deadbeef")
	}
}

func TestCountIntents(t *testing.T) {
	s := setupTestStore(t)
	ctx := context.Background()

	const n = 7
	for i := 0; i < n; i++ {
		intent := makeIntent(fmt.Sprintf("c-%d", i), "proj-1", "count.go")
		if err := s.InsertIntent(ctx, intent); err != nil {
			t.Fatalf("InsertIntent(%d) error = %v", i, err)
		}
	}

	count, err := s.CountIntents(ctx, "proj-1")
	if err != nil {
		t.Fatalf("CountIntents() error = %v", err)
	}
	if count != n {
		t.Errorf("CountIntents() = %d, want %d", count, n)
	}

	// Different project should have zero.
	count, err = s.CountIntents(ctx, "proj-other")
	if err != nil {
		t.Fatalf("CountIntents(other) error = %v", err)
	}
	if count != 0 {
		t.Errorf("CountIntents(other) = %d, want 0", count)
	}
}

func TestRunMigrationsIdempotent(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "idem.db")

	// First open runs migrations.
	db1, err := Open(dbPath)
	if err != nil {
		t.Fatalf("first Open() error = %v", err)
	}
	db1.Close()

	// Second open runs migrations again — must not error.
	db2, err := Open(dbPath)
	if err != nil {
		t.Fatalf("second Open() error = %v", err)
	}
	defer db2.Close()

	// Verify exactly one migration entry (no duplicates).
	var count int
	if err := db2.QueryRow("SELECT COUNT(*) FROM schema_migrations").Scan(&count); err != nil {
		t.Fatalf("counting migrations error = %v", err)
	}
	if count != 1 {
		t.Errorf("schema_migrations rows = %d, want 1", count)
	}
}

func TestTransactionRollbackOnError(t *testing.T) {
	s := setupTestStore(t)
	ctx := context.Background()

	// Insert a valid intent.
	intent := makeIntent("txn-1", "proj-1", "main.go")
	if err := s.InsertIntent(ctx, intent); err != nil {
		t.Fatalf("InsertIntent() error = %v", err)
	}

	// Attempt to insert a duplicate ID — PRIMARY KEY violation.
	dup := makeIntent("txn-1", "proj-1", "other.go")
	err := s.InsertIntent(ctx, dup)
	if err == nil {
		t.Fatal("expected error inserting duplicate ID, got nil")
	}

	// Verify the database state is unchanged.
	count, err := s.CountIntents(ctx, "proj-1")
	if err != nil {
		t.Fatalf("CountIntents() error = %v", err)
	}
	if count != 1 {
		t.Errorf("count after failed insert = %d, want 1", count)
	}

	// Verify the original intent is intact.
	got, err := s.ListIntents(ctx, IntentFilter{ProjectID: "proj-1"})
	if err != nil {
		t.Fatalf("ListIntents() error = %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("got %d intents, want 1", len(got))
	}
	if got[0].FilePath != "main.go" {
		t.Errorf("FilePath = %q, want %q (original should be unchanged)", got[0].FilePath, "main.go")
	}
}

func TestGetRecentIntents(t *testing.T) {
	s := setupTestStore(t)
	ctx := context.Background()

	for i := 0; i < 5; i++ {
		intent := makeIntent(fmt.Sprintf("r-%d", i), "proj-1", "recent.go")
		intent.Timestamp = time.Now().Add(time.Duration(i) * time.Second)
		if err := s.InsertIntent(ctx, intent); err != nil {
			t.Fatalf("InsertIntent(%d) error = %v", i, err)
		}
	}

	got, err := s.GetRecentIntents(ctx, "proj-1", 3)
	if err != nil {
		t.Fatalf("GetRecentIntents() error = %v", err)
	}
	if len(got) != 3 {
		t.Errorf("got %d intents, want 3", len(got))
	}

	// Results should be in descending timestamp order.
	for i := 1; i < len(got); i++ {
		if got[i].Timestamp.After(got[i-1].Timestamp) {
			t.Error("results not in descending timestamp order")
		}
	}
}

func TestGetFileRegistryNotFound(t *testing.T) {
	s := setupTestStore(t)
	ctx := context.Background()

	got, err := s.GetFileRegistry(ctx, "proj-1", "nonexistent.go")
	if err != nil {
		t.Fatalf("GetFileRegistry() error = %v", err)
	}
	if got != nil {
		t.Errorf("expected nil for nonexistent entry, got %+v", got)
	}
}

func TestDBPath(t *testing.T) {
	path := DBPath("my-project")
	if !filepath.IsAbs(path) {
		t.Errorf("DBPath returned relative path: %q", path)
	}
	want := filepath.Join(".synth", "projects", "my-project", "intent.db")
	if !contains(path, want) {
		t.Errorf("DBPath = %q, want it to contain %q", path, want)
	}
}

// contains checks if s contains substr.
func contains(s, substr string) bool {
	return len(s) >= len(substr) && searchString(s, substr)
}

func searchString(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}

func TestCancelledContext(t *testing.T) {
	s := setupTestStore(t)
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately.

	intent := makeIntent("ctx-1", "proj-1", "main.go")
	if err := s.InsertIntent(ctx, intent); err == nil {
		t.Error("InsertIntent with cancelled context should error")
	}
	if _, err := s.ListIntents(ctx, IntentFilter{ProjectID: "proj-1"}); err == nil {
		t.Error("ListIntents with cancelled context should error")
	}
	if _, err := s.CountIntents(ctx, "proj-1"); err == nil {
		t.Error("CountIntents with cancelled context should error")
	}
	if _, err := s.GetFileRegistry(ctx, "proj-1", "x.go"); err == nil {
		t.Error("GetFileRegistry with cancelled context should error")
	}
	if err := s.UpsertFileRegistry(ctx, types.FileEntry{
		FilePath: "x.go", ProjectID: "proj-1", CreatedBy: "a", CreatedAt: time.Now(),
	}); err == nil {
		t.Error("UpsertFileRegistry with cancelled context should error")
	}
	if err := s.UpdateIntentCommitHash(ctx, "ctx-1", "abc"); err == nil {
		t.Error("UpdateIntentCommitHash with cancelled context should error")
	}
	if _, err := s.GetLowContextFiles(ctx, "proj-1", 3); err == nil {
		t.Error("GetLowContextFiles with cancelled context should error")
	}
	if _, err := s.UpdateUncommittedIntents(ctx, "proj-1", "abc"); err == nil {
		t.Error("UpdateUncommittedIntents with cancelled context should error")
	}
}

func TestListIntentsFilterByDeveloper(t *testing.T) {
	s := setupTestStore(t)
	ctx := context.Background()

	devs := []string{"alice", "bob", "alice"}
	for i, d := range devs {
		intent := makeIntent(fmt.Sprintf("d-%d", i), "proj-1", "dev.go")
		intent.Developer = d
		if err := s.InsertIntent(ctx, intent); err != nil {
			t.Fatalf("InsertIntent(%d) error = %v", i, err)
		}
	}

	got, err := s.ListIntents(ctx, IntentFilter{
		ProjectID: "proj-1",
		Developer: "alice",
	})
	if err != nil {
		t.Fatalf("ListIntents() error = %v", err)
	}
	if len(got) != 2 {
		t.Errorf("got %d intents for alice, want 2", len(got))
	}
}

func TestListIntentsFilterBySince(t *testing.T) {
	s := setupTestStore(t)
	ctx := context.Background()

	base := time.Now().Add(-10 * time.Minute)
	for i := 0; i < 5; i++ {
		intent := makeIntent(fmt.Sprintf("s-%d", i), "proj-1", "since.go")
		intent.Timestamp = base.Add(time.Duration(i) * 2 * time.Minute)
		if err := s.InsertIntent(ctx, intent); err != nil {
			t.Fatalf("InsertIntent(%d) error = %v", i, err)
		}
	}

	// Filter: only intents from 5 minutes ago onward (should get the last ~3).
	cutoff := base.Add(5 * time.Minute)
	got, err := s.ListIntents(ctx, IntentFilter{
		ProjectID: "proj-1",
		Since:     cutoff,
	})
	if err != nil {
		t.Fatalf("ListIntents() error = %v", err)
	}
	for _, g := range got {
		if g.Timestamp.Before(cutoff.Truncate(time.Millisecond)) {
			t.Errorf("got intent with timestamp %v, before cutoff %v", g.Timestamp, cutoff)
		}
	}
}

func TestListIntentsNoFilter(t *testing.T) {
	s := setupTestStore(t)
	ctx := context.Background()

	for i := 0; i < 3; i++ {
		intent := makeIntent(fmt.Sprintf("nf-%d", i), "proj-1", "nofilter.go")
		if err := s.InsertIntent(ctx, intent); err != nil {
			t.Fatalf("InsertIntent(%d) error = %v", i, err)
		}
	}

	// Empty filter — no WHERE clause.
	got, err := s.ListIntents(ctx, IntentFilter{})
	if err != nil {
		t.Fatalf("ListIntents() error = %v", err)
	}
	if len(got) != 3 {
		t.Errorf("got %d intents, want 3", len(got))
	}
}

func TestGetRecentIntentsDefaultLimit(t *testing.T) {
	s := setupTestStore(t)
	ctx := context.Background()

	for i := 0; i < 25; i++ {
		intent := makeIntent(fmt.Sprintf("rd-%d", i), "proj-1", "recentdef.go")
		intent.Timestamp = time.Now().Add(time.Duration(i) * time.Second)
		if err := s.InsertIntent(ctx, intent); err != nil {
			t.Fatalf("InsertIntent(%d) error = %v", i, err)
		}
	}

	// Zero limit should default to 20.
	got, err := s.GetRecentIntents(ctx, "proj-1", 0)
	if err != nil {
		t.Fatalf("GetRecentIntents() error = %v", err)
	}
	if len(got) != 20 {
		t.Errorf("got %d intents with zero limit, want 20", len(got))
	}
}

func TestRunMigrationsDirectIdempotent(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "direct.db")
	db, err := Open(dbPath)
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer db.Close()

	// Call RunMigrations again directly — should be a no-op.
	if err := RunMigrations(db); err != nil {
		t.Fatalf("RunMigrations() second call error = %v", err)
	}

	// Verify still exactly one migration entry.
	var count int
	if err := db.QueryRow("SELECT COUNT(*) FROM schema_migrations").Scan(&count); err != nil {
		t.Fatalf("counting migrations error = %v", err)
	}
	if count != 1 {
		t.Errorf("schema_migrations rows = %d, want 1", count)
	}
}

func TestRunMigrationsBadSQL(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "bad.db")

	// Open a raw database without running the standard migrations.
	db, err := openRawDB(dbPath)
	if err != nil {
		t.Fatalf("openRawDB() error = %v", err)
	}
	defer db.Close()

	// Temporarily override the package-level migrations with bad SQL.
	orig := migrations
	migrations = []Migration{
		{Version: 1, Description: "Bad migration", SQL: "THIS IS NOT VALID SQL;"},
	}
	defer func() { migrations = orig }()

	err = RunMigrations(db)
	if err == nil {
		t.Fatal("RunMigrations() with bad SQL should error")
	}

	// schema_migrations table should not exist (migration 1 creates all tables,
	// and it failed before the INSERT).
	var count int
	row := db.QueryRow("SELECT count(*) FROM sqlite_master WHERE type='table' AND name='schema_migrations'")
	if scanErr := row.Scan(&count); scanErr != nil {
		t.Fatalf("scan error = %v", scanErr)
	}
	if count != 0 {
		t.Errorf("schema_migrations should not exist after failed migration, count = %d", count)
	}
}

func TestOpenInvalidPath(t *testing.T) {
	// Try to open a database at a path that cannot be created.
	_, err := Open("/dev/null/impossible/path/db.sqlite")
	if err == nil {
		t.Fatal("Open() with invalid path should error")
	}
}

func TestOpenFreshDB(t *testing.T) {
	// This exercises the getCurrentVersion path where schema_migrations
	// table does not exist yet (count == 0 in sqlite_master check).
	dbPath := filepath.Join(t.TempDir(), "fresh.db")
	db, err := Open(dbPath)
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer db.Close()

	// Verify migrations ran and schema_migrations has exactly one entry.
	var version int
	if err := db.QueryRow("SELECT MAX(version) FROM schema_migrations").Scan(&version); err != nil {
		t.Fatalf("query version error = %v", err)
	}
	if version != 1 {
		t.Errorf("version = %d, want 1", version)
	}
}

func TestListIntentsWithAllFilters(t *testing.T) {
	s := setupTestStore(t)
	ctx := context.Background()

	base := time.Now().Truncate(time.Millisecond)

	// Create intents with varied attributes.
	intent := makeIntent("af-1", "proj-1", "target.go")
	intent.Branch = "feature/x"
	intent.Developer = "alice"
	intent.Timestamp = base
	if err := s.InsertIntent(ctx, intent); err != nil {
		t.Fatalf("InsertIntent() error = %v", err)
	}

	intent2 := makeIntent("af-2", "proj-1", "other.go")
	intent2.Branch = "main"
	intent2.Developer = "bob"
	intent2.Timestamp = base.Add(-time.Hour)
	if err := s.InsertIntent(ctx, intent2); err != nil {
		t.Fatalf("InsertIntent() error = %v", err)
	}

	// Use all filters simultaneously.
	got, err := s.ListIntents(ctx, IntentFilter{
		ProjectID: "proj-1",
		FilePath:  "target.go",
		Branch:    "feature/x",
		Developer: "alice",
		Since:     base.Add(-time.Minute),
		Limit:     10,
	})
	if err != nil {
		t.Fatalf("ListIntents() error = %v", err)
	}
	if len(got) != 1 {
		t.Errorf("got %d intents, want 1", len(got))
	}
	if len(got) > 0 && got[0].ID != "af-1" {
		t.Errorf("ID = %q, want %q", got[0].ID, "af-1")
	}
}

// openRawDB opens a SQLite database with pragmas but without running migrations.
// Used for testing migration error paths.
func openRawDB(dbPath string) (*sql.DB, error) {
	if err := os.MkdirAll(filepath.Dir(dbPath), 0o755); err != nil {
		return nil, err
	}
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)
	if _, err := db.Exec("PRAGMA journal_mode=WAL"); err != nil {
		db.Close()
		return nil, err
	}
	return db, nil
}

func TestApplyPragmas(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "pragma.db")
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		t.Fatalf("sql.Open() error = %v", err)
	}
	defer db.Close()
	db.SetMaxOpenConns(1)

	if err := applyPragmas(db); err != nil {
		t.Fatalf("applyPragmas() error = %v", err)
	}

	// Verify WAL mode was set.
	var mode string
	if err := db.QueryRow("PRAGMA journal_mode").Scan(&mode); err != nil {
		t.Fatalf("query journal_mode error = %v", err)
	}
	if mode != "wal" {
		t.Errorf("journal_mode = %q, want %q", mode, "wal")
	}
}

func TestOpenMigrationFailure(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "migfail.db")

	// Temporarily override migrations with bad SQL.
	orig := migrations
	migrations = []Migration{
		{Version: 1, Description: "Bad", SQL: "INVALID SQL STATEMENT;"},
	}
	defer func() { migrations = orig }()

	_, err := Open(dbPath)
	if err == nil {
		t.Fatal("Open() with bad migration should error")
	}
}

func TestGetCurrentVersionEmptyTable(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "empty_ver.db")
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		t.Fatalf("sql.Open() error = %v", err)
	}
	defer db.Close()
	db.SetMaxOpenConns(1)

	// Create schema_migrations table but leave it empty.
	_, err = db.Exec(`CREATE TABLE schema_migrations (
		version INTEGER PRIMARY KEY, applied_at INTEGER NOT NULL, description TEXT NOT NULL)`)
	if err != nil {
		t.Fatalf("create table error = %v", err)
	}

	ver, err := getCurrentVersion(db)
	if err != nil {
		t.Fatalf("getCurrentVersion() error = %v", err)
	}
	if ver != 0 {
		t.Errorf("version = %d, want 0", ver)
	}
}

func TestApplyMigrationSuccess(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "applymig.db")
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		t.Fatalf("sql.Open() error = %v", err)
	}
	defer db.Close()
	db.SetMaxOpenConns(1)

	// Apply the real initial migration directly.
	m := Migration{Version: 1, Description: "Initial schema", SQL: initialSchemaSQL}
	if err := applyMigration(db, m); err != nil {
		t.Fatalf("applyMigration() error = %v", err)
	}

	// Verify the migration was recorded.
	var ver int
	if err := db.QueryRow("SELECT MAX(version) FROM schema_migrations").Scan(&ver); err != nil {
		t.Fatalf("query version error = %v", err)
	}
	if ver != 1 {
		t.Errorf("version = %d, want 1", ver)
	}
}

func TestApplyMigrationBadSQL(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "badapply.db")
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		t.Fatalf("sql.Open() error = %v", err)
	}
	defer db.Close()
	db.SetMaxOpenConns(1)

	m := Migration{Version: 1, Description: "Bad", SQL: "TOTALLY INVALID;"}
	err = applyMigration(db, m)
	if err == nil {
		t.Fatal("applyMigration() with bad SQL should error")
	}
}

func TestGetLowContextFiles(t *testing.T) {
	s := setupTestStore(t)
	ctx := context.Background()

	for i := 0; i < 4; i++ {
		_, err := s.db.ExecContext(ctx, "INSERT INTO file_saves (file_path, project_id, saved_at, has_note) VALUES (?, ?, ?, ?)", "main.go", "proj-1", time.Now().UnixMilli(), 0)
		if err != nil {
			t.Fatal(err)
		}
	}

	files, err := s.GetLowContextFiles(ctx, "proj-1", 3)
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 1 || files[0] != "main.go" {
		t.Errorf("expected [main.go], got %v", files)
	}
}

func TestGetLowContextFiles_BelowThreshold(t *testing.T) {
	s := setupTestStore(t)
	ctx := context.Background()

	for i := 0; i < 2; i++ {
		_, err := s.db.ExecContext(ctx, "INSERT INTO file_saves (file_path, project_id, saved_at, has_note) VALUES (?, ?, ?, ?)", "utils.go", "proj-1", time.Now().UnixMilli(), 0)
		if err != nil {
			t.Fatal(err)
		}
	}

	files, err := s.GetLowContextFiles(ctx, "proj-1", 3)
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 0 {
		t.Errorf("expected empty, got %v", files)
	}
}

func TestGetLowContextFiles_WithNotes(t *testing.T) {
	s := setupTestStore(t)
	ctx := context.Background()

	for i := 0; i < 4; i++ {
		_, err := s.db.ExecContext(ctx, "INSERT INTO file_saves (file_path, project_id, saved_at, has_note) VALUES (?, ?, ?, ?)", "auth.go", "proj-1", time.Now().UnixMilli(), 1)
		if err != nil {
			t.Fatal(err)
		}
	}

	files, err := s.GetLowContextFiles(ctx, "proj-1", 3)
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 0 {
		t.Errorf("expected empty, got %v", files)
	}
}

func TestUpdateUncommittedIntents_InStore(t *testing.T) {
	s := setupTestStore(t)
	ctx := context.Background()

	// Insert intents with empty commit hashes.
	for i := 0; i < 3; i++ {
		intent := makeIntent(fmt.Sprintf("uc-%d", i), "proj-1", "file.go")
		intent.CommitHash = ""
		if err := s.InsertIntent(ctx, intent); err != nil {
			t.Fatal(err)
		}
	}

	// Insert an intent that already has a commit hash.
	intent := makeIntent("uc-already", "proj-1", "file.go")
	intent.CommitHash = "abc1234"
	if err := s.InsertIntent(ctx, intent); err != nil {
		t.Fatal(err)
	}

	// Insert an intent for another project with an empty commit hash.
	intent2 := makeIntent("uc-other", "proj-2", "file.go")
	intent2.CommitHash = ""
	if err := s.InsertIntent(ctx, intent2); err != nil {
		t.Fatal(err)
	}

	// Run update
	updated, err := s.UpdateUncommittedIntents(ctx, "proj-1", "newhash")
	if err != nil {
		t.Fatal(err)
	}
	if updated != 3 {
		t.Errorf("expected 3 updated, got %d", updated)
	}

	// Verify the 3 were updated
	intents, err := s.ListIntents(ctx, IntentFilter{ProjectID: "proj-1"})
	if err != nil {
		t.Fatal(err)
	}

	countNewHash := 0
	for _, in := range intents {
		if in.CommitHash == "newhash" {
			countNewHash++
		}
	}
	if countNewHash != 3 {
		t.Errorf("expected 3 intents with 'newhash', got %d", countNewHash)
	}
}
