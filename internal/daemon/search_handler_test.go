package daemon

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
	"time"

	"github.com/shyamsundaravssb/synth/internal/ipc"
	"github.com/shyamsundaravssb/synth/internal/store"
	"github.com/shyamsundaravssb/synth/pkg/types"
)

func TestCosineSimilarity_IdenticalVectors(t *testing.T) {
	a := make([]float32, 384)
	a[0] = 1.0
	b := make([]float32, 384)
	b[0] = 1.0

	sim := cosineSimilarity(a, b)
	if sim != 1.0 {
		t.Errorf("expected exactly 1.0, got %f", sim)
	}
}

func TestCosineSimilarity_OrthogonalVectors(t *testing.T) {
	a := make([]float32, 384)
	a[0] = 1.0
	b := make([]float32, 384)
	b[1] = 1.0

	sim := cosineSimilarity(a, b)
	if sim != 0.0 {
		t.Errorf("expected 0.0, got %f", sim)
	}
}

func TestCosineSimilarity_EmptyVectors(t *testing.T) {
	sim := cosineSimilarity([]float32{}, []float32{})
	if sim != 0.0 {
		t.Errorf("expected 0.0, got %f", sim)
	}
}

func TestCosineSimilarity_MismatchedLengths(t *testing.T) {
	a := make([]float32, 10)
	b := make([]float32, 5)

	sim := cosineSimilarity(a, b)
	if sim != 0.0 {
		t.Errorf("expected 0.0, got %f", sim)
	}
}

func setupSearchHandlerTest(t *testing.T) (*SearchHandler, store.Store, *mockEmbedEngine) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "intent.db")
	db, err := store.Open(dbPath)
	if err != nil {
		t.Fatalf("failed to open store: %v", err)
	}
	synthStore := store.NewSQLiteStore(db)

	mock := &mockEmbedEngine{
		modelPresent: true,
	}
	log := NewLogger(filepath.Join(dir, "test.log"))
	handler := NewSearchHandler(synthStore, mock, "proj_123", log)

	return handler, synthStore, mock
}

func TestSearchHandler_EmptyQuery(t *testing.T) {
	handler, _, _ := setupSearchHandlerTest(t)

	req, _ := ipc.NewRequest(ipc.TypeSearch, ipc.SearchPayload{Query: ""})
	resp := handler.Handle(req)

	if resp.Status != ipc.StatusError {
		t.Errorf("expected error status, got %s", resp.Status)
	}

	errData, _ := ipc.ParseErrorData(resp)
	if errData.Code != "ERR_EMPTY" {
		t.Errorf("expected ERR_EMPTY code, got %s", errData.Code)
	}
}

func TestSearchHandler_EmbedError(t *testing.T) {
	handler, _, mock := setupSearchHandlerTest(t)
	mock.embedErr = errors.New("model error")

	req, _ := ipc.NewRequest(ipc.TypeSearch, ipc.SearchPayload{Query: "test"})
	resp := handler.Handle(req)

	if resp.Status != ipc.StatusError {
		t.Errorf("expected error status, got %s", resp.Status)
	}

	errData, _ := ipc.ParseErrorData(resp)
	if errData.Code != "ERR_EMBED" {
		t.Errorf("expected ERR_EMBED code, got %s", errData.Code)
	}
}

func TestSearchHandler_ReturnsRankedResults(t *testing.T) {
	handler, s, mock := setupSearchHandlerTest(t)
	ctx := context.Background()

	intents := []types.Intent{
		{ID: "iA", ProjectID: "proj_123", FilePath: "a.go", Type: "change", Context: "normal", What: "intent A", Timestamp: time.Now()},
		{ID: "iB", ProjectID: "proj_123", FilePath: "b.go", Type: "change", Context: "normal", What: "intent B", Timestamp: time.Now()},
		{ID: "iC", ProjectID: "proj_123", FilePath: "c.go", Type: "change", Context: "normal", What: "intent C", Timestamp: time.Now()},
	}

	for _, intent := range intents {
		if err := s.InsertIntent(ctx, intent); err != nil {
			t.Fatalf("InsertIntent failed: %v", err)
		}
	}

	vecA := make([]float32, 384)
	vecA[0] = 1.0

	vecB := make([]float32, 384)
	vecB[1] = 1.0

	vecC := make([]float32, 384)
	vecC[0] = 0.9
	vecC[1] = 0.1

	_ = s.InsertEmbedding(ctx, store.EmbeddingRecord{IntentID: "iA", ProjectID: "proj_123", Embedding: vecA, Model: "test"})
	_ = s.InsertEmbedding(ctx, store.EmbeddingRecord{IntentID: "iB", ProjectID: "proj_123", Embedding: vecB, Model: "test"})
	_ = s.InsertEmbedding(ctx, store.EmbeddingRecord{IntentID: "iC", ProjectID: "proj_123", Embedding: vecC, Model: "test"})

	// Query is exactly vecA
	mock.embedResult = [][]float32{vecA}

	req, _ := ipc.NewRequest(ipc.TypeSearch, ipc.SearchPayload{Query: "find A"})
	resp := handler.Handle(req)

	if resp.Status != ipc.StatusOK {
		t.Fatalf("expected OK, got %s", resp.Status)
	}

	data, err := ipc.ParseSearchData(resp)
	if err != nil {
		t.Fatalf("ParseSearchData failed: %v", err)
	}

	if len(data.Results) != 3 {
		t.Fatalf("expected 3 results, got %d", len(data.Results))
	}

	// Order should be A, C, B
	if data.Results[0].ID != "iA" {
		t.Errorf("expected first result to be iA, got %s", data.Results[0].ID)
	}
	if data.Results[1].ID != "iC" {
		t.Errorf("expected second result to be iC, got %s", data.Results[1].ID)
	}
	if data.Results[2].ID != "iB" {
		t.Errorf("expected third result to be iB, got %s", data.Results[2].ID)
	}

	// Scores should be descending
	if data.Results[0].Score < data.Results[1].Score || data.Results[1].Score < data.Results[2].Score {
		t.Errorf("scores are not descending: %v", data.Results)
	}
}

func TestSearchHandler_AppliesFileFilter(t *testing.T) {
	handler, s, mock := setupSearchHandlerTest(t)
	ctx := context.Background()

	_ = s.InsertIntent(ctx, types.Intent{ID: "i1", ProjectID: "proj_123", FilePath: "auth.go", Type: "change", Context: "normal", What: "auth", Timestamp: time.Now()})
	_ = s.InsertIntent(ctx, types.Intent{ID: "i2", ProjectID: "proj_123", FilePath: "users.go", Type: "change", Context: "normal", What: "users", Timestamp: time.Now()})

	_ = s.InsertEmbedding(ctx, store.EmbeddingRecord{IntentID: "i1", ProjectID: "proj_123", Embedding: make([]float32, 384), Model: "test"})
	_ = s.InsertEmbedding(ctx, store.EmbeddingRecord{IntentID: "i2", ProjectID: "proj_123", Embedding: make([]float32, 384), Model: "test"})

	mock.embedResult = [][]float32{make([]float32, 384)}

	req, _ := ipc.NewRequest(ipc.TypeSearch, ipc.SearchPayload{Query: "test", FilePath: "auth.go"})
	resp := handler.Handle(req)

	data, err := ipc.ParseSearchData(resp)
	if err != nil {
		t.Fatalf("ParseSearchData failed: %v", err)
	}

	if len(data.Results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(data.Results))
	}
	if data.Results[0].FilePath != "auth.go" {
		t.Errorf("expected auth.go, got %s", data.Results[0].FilePath)
	}
}
