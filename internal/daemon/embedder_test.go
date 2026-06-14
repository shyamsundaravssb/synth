package daemon

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
	"time"

	"github.com/shyamsundaravssb/synth/internal/store"
	"github.com/shyamsundaravssb/synth/pkg/types"
)

type mockEmbedEngine struct {
	modelPresent bool
	embedResult  [][]float32
	embedErr     error
	embedCalled  int
}

func (m *mockEmbedEngine) IsModelPresent() bool {
	return m.modelPresent
}

func (m *mockEmbedEngine) EmbedBatch(ctx context.Context, texts []string) ([][]float32, error) {
	m.embedCalled++
	if m.embedErr != nil {
		return nil, m.embedErr
	}
	result := make([][]float32, len(texts))
	for i := range texts {
		if i < len(m.embedResult) {
			result[i] = m.embedResult[i]
		} else {
			result[i] = make([]float32, 384)
		}
	}
	return result, nil
}



func setupEmbedderTest(t *testing.T) (*Embedder, store.Store, *mockEmbedEngine, chan struct{}) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "intent.db")
	db, err := store.Open(dbPath)
	if err != nil {
		t.Fatalf("failed to open store: %v", err)
	}

	synthStore := store.NewSQLiteStore(db)
	projectID := "proj_123"

	mock := &mockEmbedEngine{
		modelPresent: true,
		embedResult:  [][]float32{make([]float32, 384)},
	}
	shutdownCh := make(chan struct{})

	// we use a real logger to test it without panic, writing to temp dir
	logFile := filepath.Join(dir, "test.log")
	log := NewLogger(logFile)

	embedder := NewEmbedder(synthStore, mock, projectID, log, shutdownCh)

	return embedder, synthStore, mock, shutdownCh
}

func TestEmbedder_SkipsWhenModelNotPresent(t *testing.T) {
	embedder, s, mock, _ := setupEmbedderTest(t)
	mock.modelPresent = false

	err := s.InsertIntent(context.Background(), types.Intent{
		ID: "int_1", ProjectID: "proj_123", FilePath: "a.go", Type: "change", Context: "normal", What: "what", Why: "why", Timestamp: time.Now(),
	})
	if err != nil {
		t.Fatalf("insert failed: %v", err)
	}

	embedder.embedPending()

	if mock.embedCalled != 0 {
		t.Fatalf("expected embedCalled 0, got %d", mock.embedCalled)
	}
}

func TestEmbedder_EmbedsUnembeddedIntents(t *testing.T) {
	embedder, s, mock, _ := setupEmbedderTest(t)
	mock.embedResult = [][]float32{make([]float32, 384), make([]float32, 384)}

	ctx := context.Background()
	_ = s.InsertIntent(ctx, types.Intent{ID: "int_1", ProjectID: "proj_123", FilePath: "a.go", Type: "change", Context: "normal", What: "what1", Why: "why1", Timestamp: time.Now()})
	_ = s.InsertIntent(ctx, types.Intent{ID: "int_2", ProjectID: "proj_123", FilePath: "b.go", Type: "change", Context: "normal", What: "what2", Why: "why2", Timestamp: time.Now()})

	embedder.embedPending()

	if mock.embedCalled != 1 {
		t.Fatalf("expected embedCalled 1, got %d", mock.embedCalled)
	}

	emb1, err := s.GetEmbedding(ctx, "int_1")
	if err != nil || emb1 == nil {
		t.Fatalf("failed to get embedding 1: %v", err)
	}

	emb2, err := s.GetEmbedding(ctx, "int_2")
	if err != nil || emb2 == nil {
		t.Fatalf("failed to get embedding 2: %v", err)
	}

	unembedded, err := s.ListIntentsWithoutEmbeddings(ctx, "proj_123", 10)
	if err != nil {
		t.Fatalf("ListIntentsWithoutEmbeddings error: %v", err)
	}
	if len(unembedded) != 0 {
		t.Fatalf("expected 0 unembedded, got %d", len(unembedded))
	}
}

func TestEmbedder_SkipsAlreadyEmbedded(t *testing.T) {
	embedder, s, mock, _ := setupEmbedderTest(t)

	_ = s.InsertIntent(context.Background(), types.Intent{ID: "int_1", ProjectID: "proj_123", FilePath: "a.go", Type: "change", Context: "normal", What: "what1", Why: "why1", Timestamp: time.Now()})

	embedder.embedPending()
	if mock.embedCalled != 1 {
		t.Fatalf("expected embedCalled 1, got %d", mock.embedCalled)
	}

	embedder.embedPending()
	if mock.embedCalled != 1 {
		t.Fatalf("expected embedCalled 1 still, got %d", mock.embedCalled)
	}
}

func TestEmbedder_HandlesBatchEmbedError(t *testing.T) {
	embedder, s, mock, _ := setupEmbedderTest(t)
	mock.embedErr = errors.New("model error")

	_ = s.InsertIntent(context.Background(), types.Intent{ID: "int_1", ProjectID: "proj_123", FilePath: "a.go", Type: "change", Context: "normal", What: "what1", Why: "why1", Timestamp: time.Now()})

	embedder.embedPending()

	unembedded, _ := s.ListIntentsWithoutEmbeddings(context.Background(), "proj_123", 10)
	if len(unembedded) != 1 {
		t.Fatalf("expected 1 unembedded intent after error, got %d", len(unembedded))
	}
}

func TestEmbedder_UpdatesDaemonState(t *testing.T) {
	embedder, s, _, _ := setupEmbedderTest(t)

	_ = s.InsertIntent(context.Background(), types.Intent{ID: "int_1", ProjectID: "proj_123", FilePath: "a.go", Type: "change", Context: "normal", What: "what1", Why: "why1", Timestamp: time.Now()})

	embedder.embedPending()

	val, err := s.GetDaemonState(context.Background(), "proj_123", "last_embed_run")
	if err != nil {
		t.Fatalf("GetDaemonState error: %v", err)
	}
	if val == "" {
		t.Fatalf("expected non-empty daemon state")
	}

	_, err = time.Parse(time.RFC3339, val)
	if err != nil {
		t.Fatalf("failed to parse daemon state time: %v", err)
	}
}

func TestEmbedder_StopsOnShutdown(t *testing.T) {
	embedder, _, _, shutdownCh := setupEmbedderTest(t)

	embedder.Start()
	time.Sleep(100 * time.Millisecond)

	close(shutdownCh)
	time.Sleep(200 * time.Millisecond)
}
