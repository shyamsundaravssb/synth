package daemon

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/shyamsundaravssb/synth/internal/store"
	"github.com/shyamsundaravssb/synth/pkg/types"
)

func setupLowContextScorer(t *testing.T, threshold int) (*LowContextScorer, store.Store) {
	t.Helper()

	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "intent.db")

	db, err := store.Open(dbPath)
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}

	s := store.NewSQLiteStore(db)

	logPath := filepath.Join(tempDir, "daemon.log")
	logger := NewLogger(logPath)

	scorer := NewLowContextScorer(s, "proj-1", threshold, logger)
	return scorer, s
}

func insertFileSaves(t *testing.T, s store.Store, projectID, filePath string, count int) {
	t.Helper()
	ctx := context.Background()
	for i := 0; i < count; i++ {
		if err := s.RecordFileSave(ctx, projectID, filePath, false); err != nil {
			t.Fatalf("failed to insert file save: %v", err)
		}
	}
}

func insertIntent(t *testing.T, s store.Store, projectID, filePath, what string, ts time.Time) {
	t.Helper()
	ctx := context.Background()
	in := types.Intent{
		ID:        "id-" + ts.Format(time.RFC3339Nano),
		ProjectID: projectID,
		FilePath:  filePath,
		What:      what,
		Timestamp: ts,
		Type:      "change",
		Context:   "normal",
	}
	if err := s.InsertIntent(ctx, in); err != nil {
		t.Fatalf("failed to insert intent: %v", err)
	}
}

func TestComputeLowContextFiles_BelowThreshold(t *testing.T) {
	scorer, s := setupLowContextScorer(t, 5)
	insertFileSaves(t, s, "proj-1", "auth.go", 4)

	res, err := scorer.ComputeLowContextFiles(context.Background())
	if err != nil {
		t.Fatalf("error: %v", err)
	}

	for _, f := range res {
		if f.FilePath == "auth.go" {
			t.Errorf("expected auth.go to not be in results (below threshold)")
		}
	}
}

func TestComputeLowContextFiles_AtThreshold(t *testing.T) {
	scorer, s := setupLowContextScorer(t, 5)
	insertFileSaves(t, s, "proj-1", "auth.go", 5)

	res, err := scorer.ComputeLowContextFiles(context.Background())
	if err != nil {
		t.Fatalf("error: %v", err)
	}

	found := false
	for _, f := range res {
		if f.FilePath == "auth.go" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected auth.go to be in results")
	}
}

func TestComputeLowContextFiles_NeverNoted(t *testing.T) {
	scorer, s := setupLowContextScorer(t, 5)
	insertFileSaves(t, s, "proj-1", "auth.go", 6)

	res, err := scorer.ComputeLowContextFiles(context.Background())
	if err != nil {
		t.Fatalf("error: %v", err)
	}

	if len(res) != 1 {
		t.Fatalf("expected 1 result, got %d", len(res))
	}

	f := res[0]
	if f.HasEverBeenNoted != false {
		t.Errorf("expected HasEverBeenNoted = false")
	}
	if f.DaysSinceNote != 0 {
		t.Errorf("expected DaysSinceNote = 0, got %d", f.DaysSinceNote)
	}
}

func TestComputeLowContextFiles_WithRecentNote(t *testing.T) {
	scorer, s := setupLowContextScorer(t, 5)
	insertFileSaves(t, s, "proj-1", "auth.go", 6)
	insertIntent(t, s, "proj-1", "auth.go", "recent", time.Now())

	res, err := scorer.ComputeLowContextFiles(context.Background())
	if err != nil {
		t.Fatalf("error: %v", err)
	}

	if len(res) != 1 {
		t.Fatalf("expected 1 result, got %d", len(res))
	}

	f := res[0]
	if f.HasEverBeenNoted != true {
		t.Errorf("expected HasEverBeenNoted = true")
	}
	if f.DaysSinceNote != 0 {
		t.Errorf("expected DaysSinceNote = 0, got %d", f.DaysSinceNote)
	}
}

func TestComputeLowContextFiles_WithOldNote(t *testing.T) {
	scorer, s := setupLowContextScorer(t, 5)
	insertFileSaves(t, s, "proj-1", "auth.go", 6)
	insertIntent(t, s, "proj-1", "auth.go", "old", time.Now().Add(-5*24*time.Hour))

	res, err := scorer.ComputeLowContextFiles(context.Background())
	if err != nil {
		t.Fatalf("error: %v", err)
	}

	if len(res) != 1 {
		t.Fatalf("expected 1 result, got %d", len(res))
	}

	f := res[0]
	if f.DaysSinceNote != 5 {
		t.Errorf("expected DaysSinceNote = 5, got %d", f.DaysSinceNote)
	}
}

func TestComputeLowContextFiles_SortedBySaveCount(t *testing.T) {
	scorer, s := setupLowContextScorer(t, 5)
	insertFileSaves(t, s, "proj-1", "auth.go", 8)
	insertFileSaves(t, s, "proj-1", "users.go", 5)
	insertFileSaves(t, s, "proj-1", "payments.go", 7)

	res, err := scorer.ComputeLowContextFiles(context.Background())
	if err != nil {
		t.Fatalf("error: %v", err)
	}

	if len(res) != 3 {
		t.Fatalf("expected 3 results, got %d", len(res))
	}

	if res[0].FilePath != "auth.go" {
		t.Errorf("expected res[0] to be auth.go, got %s", res[0].FilePath)
	}
	if res[1].FilePath != "payments.go" {
		t.Errorf("expected res[1] to be payments.go, got %s", res[1].FilePath)
	}
	if res[2].FilePath != "users.go" {
		t.Errorf("expected res[2] to be users.go, got %s", res[2].FilePath)
	}
}

func TestComputeSummary_EmptyInput(t *testing.T) {
	summary := ComputeSummary([]LowContextFile{})
	if summary.TotalFiles != 0 {
		t.Errorf("expected TotalFiles = 0, got %d", summary.TotalFiles)
	}
	if summary.NeverNoted != 0 {
		t.Errorf("expected NeverNoted = 0, got %d", summary.NeverNoted)
	}
}

func TestComputeSummary_WithFiles(t *testing.T) {
	files := []LowContextFile{
		{FilePath: "file1", SaveCount: 8, HasEverBeenNoted: false},
		{FilePath: "file2", SaveCount: 6, HasEverBeenNoted: true},
		{FilePath: "file3", SaveCount: 5, HasEverBeenNoted: false},
	}

	summary := ComputeSummary(files)
	if summary.TotalFiles != 3 {
		t.Errorf("expected TotalFiles = 3, got %d", summary.TotalFiles)
	}
	if summary.NeverNoted != 2 {
		t.Errorf("expected NeverNoted = 2, got %d", summary.NeverNoted)
	}
	if summary.HighestSaveCount != 8 {
		t.Errorf("expected HighestSaveCount = 8, got %d", summary.HighestSaveCount)
	}
	if summary.MostNeglected.FilePath != "file1" {
		t.Errorf("expected MostNeglected = file1, got %s", summary.MostNeglected.FilePath)
	}
}

func TestCacheAndLoadResults(t *testing.T) {
	scorer, _ := setupLowContextScorer(t, 5)

	files := []LowContextFile{
		{FilePath: "file1", SaveCount: 10, HasEverBeenNoted: true, DaysSinceNote: 2},
	}

	if err := scorer.CacheResults(context.Background(), files); err != nil {
		t.Fatalf("CacheResults error: %v", err)
	}

	loaded, err := scorer.LoadCachedResults(context.Background())
	if err != nil {
		t.Fatalf("LoadCachedResults error: %v", err)
	}

	if len(loaded) != len(files) {
		t.Fatalf("expected %d results, got %d", len(files), len(loaded))
	}

	if loaded[0].FilePath != files[0].FilePath {
		t.Errorf("FilePath mismatch")
	}
	if loaded[0].SaveCount != files[0].SaveCount {
		t.Errorf("SaveCount mismatch")
	}
	if loaded[0].HasEverBeenNoted != files[0].HasEverBeenNoted {
		t.Errorf("HasEverBeenNoted mismatch")
	}
}

func TestLoadCachedResults_Empty(t *testing.T) {
	scorer, _ := setupLowContextScorer(t, 5)

	loaded, err := scorer.LoadCachedResults(context.Background())
	if err != nil {
		t.Fatalf("LoadCachedResults error: %v", err)
	}

	if loaded == nil {
		t.Error("expected empty slice, got nil")
	}
	if len(loaded) != 0 {
		t.Errorf("expected 0 entries, got %d", len(loaded))
	}
}
