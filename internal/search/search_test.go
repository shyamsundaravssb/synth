package search

import (
	"context"
	"fmt"
	"path/filepath"
	"testing"
	"time"

	"github.com/shyamsundaravssb/synth/internal/store"
	"github.com/shyamsundaravssb/synth/pkg/types"
)

func setupSearcher(t *testing.T) (*Searcher, store.Store) {
	t.Helper()

	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "intent.db")

	db, err := store.Open(dbPath)
	if err != nil {
		t.Fatalf("failed to open store: %v", err)
	}
	t.Cleanup(func() { db.Close() })

	s := store.NewSQLiteStore(db)

	// Use a non-existent socket path to guarantee IPC fails
	sockPath := filepath.Join(tempDir, "does-not-exist.sock")
	searcher := New(sockPath, "proj_123", s)

	return searcher, s
}

func TestSearch_FallsBackToFTS5WhenDaemonOffline(t *testing.T) {
	searcher, s := setupSearcher(t)
	ctx := context.Background()

	err := s.InsertIntent(ctx, types.Intent{
		ID:        "int_1",
		ProjectID: "proj_123",
		FilePath:  "auth.go",
		Type:      "change",
		Context:   "normal",
		What:      "added login endpoint",
		Timestamp: time.Now(),
	})
	if err != nil {
		t.Fatalf("failed to insert intent: %v", err)
	}

	err = s.InsertIntent(ctx, types.Intent{
		ID:        "int_2",
		ProjectID: "proj_123",
		FilePath:  "db.go",
		Type:      "change",
		Context:   "normal",
		What:      "fixed database timeout",
		Timestamp: time.Now(),
	})
	if err != nil {
		t.Fatalf("failed to insert intent: %v", err)
	}

	resp, err := searcher.Search(ctx, SearchRequest{
		Query: "login",
		Limit: 10,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !resp.IsFallback {
		t.Errorf("expected IsFallback to be true")
	}
	if resp.Mode != ModeFTS5 {
		t.Errorf("expected Mode to be ModeFTS5, got %v", resp.Mode)
	}
	if len(resp.Results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(resp.Results))
	}
	if resp.Results[0].What != "added login endpoint" {
		t.Errorf("unexpected what: %s", resp.Results[0].What)
	}
}

func TestSearch_NoFallbackReturnsErrorWhenOffline(t *testing.T) {
	searcher, _ := setupSearcher(t)
	ctx := context.Background()

	_, err := searcher.Search(ctx, SearchRequest{
		Query:      "test",
		NoFallback: true,
	})
	if err == nil {
		t.Fatalf("expected error when NoFallback is true and daemon is offline")
	}
}

func TestSearch_DefaultLimit(t *testing.T) {
	searcher, s := setupSearcher(t)
	ctx := context.Background()

	for i := 0; i < 15; i++ {
		err := s.InsertIntent(ctx, types.Intent{
			ID:        fmt.Sprintf("int_%d", i),
			ProjectID: "proj_123",
			FilePath:  "main.go",
			Type:      "change",
			Context:   "normal",
			What:      "common update",
			Timestamp: time.Now(),
		})
		if err != nil {
			t.Fatalf("failed to insert intent: %v", err)
		}
	}

	resp, err := searcher.Search(ctx, SearchRequest{
		Query: "common",
		Limit: 0,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(resp.Results) != 10 { // Default limit is 10
		t.Fatalf("expected exactly 10 results due to default limit, got %d", len(resp.Results))
	}
}

func TestSearchFTS5_FileFilter(t *testing.T) {
	searcher, s := setupSearcher(t)
	ctx := context.Background()

	intents := []types.Intent{
		{ID: "i1", ProjectID: "proj_123", FilePath: "auth.go", Type: "change", Context: "normal", What: "added login", Timestamp: time.Now()},
		{ID: "i2", ProjectID: "proj_123", FilePath: "users.go", Type: "change", Context: "normal", What: "added profile", Timestamp: time.Now()},
		{ID: "i3", ProjectID: "proj_123", FilePath: "auth.go", Type: "change", Context: "normal", What: "added logout", Timestamp: time.Now()},
	}

	for _, intent := range intents {
		if err := s.InsertIntent(ctx, intent); err != nil {
			t.Fatalf("failed to insert: %v", err)
		}
	}

	resp, err := searcher.Search(ctx, SearchRequest{
		Query:    "added",
		FilePath: "auth.go",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(resp.Results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(resp.Results))
	}
	for _, res := range resp.Results {
		if res.FilePath != "auth.go" {
			t.Errorf("expected FilePath to be auth.go, got %s", res.FilePath)
		}
	}
}

func TestSearchFTS5_DeveloperFilter(t *testing.T) {
	searcher, s := setupSearcher(t)
	ctx := context.Background()

	intents := []types.Intent{
		{ID: "i1", ProjectID: "proj_123", FilePath: "main.go", Type: "change", Context: "normal", What: "added cache", Developer: "Alice", Timestamp: time.Now()},
		{ID: "i2", ProjectID: "proj_123", FilePath: "main.go", Type: "change", Context: "normal", What: "added cache", Developer: "Bob", Timestamp: time.Now()},
	}

	for _, intent := range intents {
		if err := s.InsertIntent(ctx, intent); err != nil {
			t.Fatalf("failed to insert: %v", err)
		}
	}

	resp, err := searcher.Search(ctx, SearchRequest{
		Query:     "cache",
		Developer: "Alice",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(resp.Results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(resp.Results))
	}
	if resp.Results[0].Developer != "Alice" {
		t.Errorf("expected developer Alice, got %s", resp.Results[0].Developer)
	}
}

func TestSearchResult_ModeIsFTS5(t *testing.T) {
	searcher, s := setupSearcher(t)
	ctx := context.Background()

	err := s.InsertIntent(ctx, types.Intent{
		ID:        "i1",
		ProjectID: "proj_123",
		FilePath:  "main.go",
		Type:      "change",
		Context:   "normal",
		What:      "testing mode",
		Timestamp: time.Now(),
	})
	if err != nil {
		t.Fatalf("failed to insert: %v", err)
	}

	resp, err := searcher.Search(ctx, SearchRequest{
		Query: "testing",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(resp.Results) == 0 {
		t.Fatalf("expected results")
	}

	for _, res := range resp.Results {
		if res.Mode != ModeFTS5 {
			t.Errorf("expected ModeFTS5, got %v", res.Mode)
		}
		if res.Score != 0.0 {
			t.Errorf("expected Score 0.0, got %f", res.Score)
		}
	}
}
