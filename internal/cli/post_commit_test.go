package cli

import (
	"context"
	"fmt"
	"path/filepath"
	"testing"
	"time"

	"github.com/shyamsundaravssb/synth/internal/store"
	"github.com/shyamsundaravssb/synth/pkg/types"
)

func setupPostCommitTestStore(t *testing.T) *store.SQLiteStore {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "test.db")
	db, err := store.Open(dbPath)
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	t.Cleanup(func() { db.Close() })
	return store.NewSQLiteStore(db)
}

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

func TestLinkCommitHash_UpdatesUncommittedIntents(t *testing.T) {
	s := setupPostCommitTestStore(t)
	ctx := context.Background()

	for i := 0; i < 3; i++ {
		intent := makeIntent(fmt.Sprintf("uc-%d", i), "proj-1", "file.go")
		intent.CommitHash = ""
		if err := s.InsertIntent(ctx, intent); err != nil {
			t.Fatal(err)
		}
	}

	rows, err := linkCommitHash(ctx, s, "proj-1", "myhash")
	if err != nil {
		t.Fatalf("linkCommitHash error: %v", err)
	}
	if rows != 3 {
		t.Errorf("expected 3 rows updated, got %d", rows)
	}

	intents, _ := s.ListIntents(ctx, store.IntentFilter{ProjectID: "proj-1"})
	for _, in := range intents {
		if in.CommitHash != "myhash" {
			t.Errorf("expected commit hash 'myhash', got '%s'", in.CommitHash)
		}
	}
}

func TestLinkCommitHash_SkipsAlreadyCommitted(t *testing.T) {
	s := setupPostCommitTestStore(t)
	ctx := context.Background()

	for i := 0; i < 2; i++ {
		intent := makeIntent(fmt.Sprintf("uc-%d", i), "proj-1", "file.go")
		intent.CommitHash = ""
		if err := s.InsertIntent(ctx, intent); err != nil {
			t.Fatal(err)
		}
	}

	intent := makeIntent("uc-already", "proj-1", "file.go")
	intent.CommitHash = "abc1234"
	if err := s.InsertIntent(ctx, intent); err != nil {
		t.Fatal(err)
	}

	rows, err := linkCommitHash(ctx, s, "proj-1", "xyz789")
	if err != nil {
		t.Fatalf("linkCommitHash error: %v", err)
	}
	if rows != 2 {
		t.Errorf("expected 2 rows updated, got %d", rows)
	}

	intents, _ := s.ListIntents(ctx, store.IntentFilter{ProjectID: "proj-1"})
	for _, in := range intents {
		if in.ID == "uc-already" {
			if in.CommitHash != "abc1234" {
				t.Errorf("expected unchanged commit hash 'abc1234', got '%s'", in.CommitHash)
			}
		} else {
			if in.CommitHash != "xyz789" {
				t.Errorf("expected new commit hash 'xyz789', got '%s'", in.CommitHash)
			}
		}
	}
}

func TestLinkCommitHash_EmptyDatabase(t *testing.T) {
	s := setupPostCommitTestStore(t)
	ctx := context.Background()

	rows, err := linkCommitHash(ctx, s, "proj-1", "myhash")
	if err != nil {
		t.Fatalf("linkCommitHash error: %v", err)
	}
	if rows != 0 {
		t.Errorf("expected 0 rows updated, got %d", rows)
	}
}
