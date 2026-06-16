package daemon

import (
	"context"
	"testing"
	"time"
)

func TestLowContextLoop_RunsImmediately(t *testing.T) {
	scorer, s := setupLowContextScorer(t, 5)
	insertFileSaves(t, s, "proj-1", "auth.go", 6)

	shutdownCh := make(chan struct{})
	loop := NewLowContextLoop(scorer, scorer.log, shutdownCh)

	loop.tick()

	cached, err := scorer.LoadCachedResults(context.Background())
	if err != nil {
		t.Fatalf("LoadCachedResults error: %v", err)
	}

	if len(cached) != 1 {
		t.Fatalf("expected 1 file, got %d", len(cached))
	}
	if cached[0].FilePath != "auth.go" {
		t.Errorf("expected auth.go, got %s", cached[0].FilePath)
	}
	if cached[0].SaveCount != 6 {
		t.Errorf("expected 6 saves, got %d", cached[0].SaveCount)
	}
}

func TestLowContextLoop_CachesResults(t *testing.T) {
	scorer, s := setupLowContextScorer(t, 5)
	insertFileSaves(t, s, "proj-1", "file1.go", 5)
	insertFileSaves(t, s, "proj-1", "file2.go", 8)

	shutdownCh := make(chan struct{})
	loop := NewLowContextLoop(scorer, scorer.log, shutdownCh)

	loop.tick()

	cached, err := scorer.LoadCachedResults(context.Background())
	if err != nil {
		t.Fatalf("LoadCachedResults error: %v", err)
	}

	if len(cached) != 2 {
		t.Fatalf("expected 2 files, got %d", len(cached))
	}
}

func TestLowContextLoop_StopsOnShutdown(t *testing.T) {
	scorer, _ := setupLowContextScorer(t, 5)

	shutdownCh := make(chan struct{})
	loop := NewLowContextLoop(scorer, scorer.log, shutdownCh)

	loop.Start()
	time.Sleep(100 * time.Millisecond)

	close(shutdownCh)
	time.Sleep(200 * time.Millisecond)
	// If it doesn't leak or panic, we consider it a pass.
}

func TestLowContextLoop_HandlesEmptyResults(t *testing.T) {
	scorer, _ := setupLowContextScorer(t, 5)

	shutdownCh := make(chan struct{})
	loop := NewLowContextLoop(scorer, scorer.log, shutdownCh)

	loop.tick()

	cached, err := scorer.LoadCachedResults(context.Background())
	if err != nil {
		t.Fatalf("LoadCachedResults error: %v", err)
	}

	if len(cached) != 0 {
		t.Fatalf("expected 0 files, got %d", len(cached))
	}
}
