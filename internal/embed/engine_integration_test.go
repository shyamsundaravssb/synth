//go:build integration

package embed

import (
	"context"
	"testing"
	"time"
)

// Integration tests use the REAL all-MiniLM-L6-v2 model on disk.
// They are slow (2–5 seconds each) and are excluded from the default
// test run. Run them with:
//
//	go test -tags=integration ./internal/embed/... -v -timeout 60s

// TestEmbed_RealModel_ProducesVector verifies the full embed pipeline:
//   - model loads without error
//   - output has exactly 384 dimensions
//   - output is not all zeros (sanity check)
//   - IsLoaded() reports true after a successful embed
func TestEmbed_RealModel_ProducesVector(t *testing.T) {
	modelDir := DefaultModelDir()
	e := New(modelDir, &noopLogger{})

	if !e.IsModelPresent() {
		t.Skip("model not present at " + modelDir + ", skipping integration test")
	}

	ctx := context.Background()
	vector, err := e.Embed(ctx, "removed email verification from auth flow")
	if err != nil {
		t.Fatalf("Embed returned an unexpected error: %v", err)
	}

	// Dimension check.
	if len(vector) != EmbedDimension {
		t.Fatalf("expected vector length %d, got %d", EmbedDimension, len(vector))
	}

	// Sanity check: at least one non-zero value.
	allZero := true
	for _, v := range vector {
		if v != 0 {
			allZero = false
			break
		}
	}
	if allZero {
		t.Fatal("all embedding values are zero — model output is suspicious")
	}

	// Engine should now report as loaded.
	if !e.IsLoaded() {
		t.Fatal("expected IsLoaded() == true after a successful Embed call")
	}

	t.Logf("Dimensions:     %d", len(vector))
	t.Logf("First 5 values: %v", vector[:5])
	t.Log("IsLoaded:       true ✓")
}

// TestEmbedBatch_RealModel_MultipleTexts verifies that EmbedBatch:
//   - returns one vector per input text
//   - each vector has exactly 384 dimensions
//   - different texts produce different vectors
func TestEmbedBatch_RealModel_MultipleTexts(t *testing.T) {
	modelDir := DefaultModelDir()
	e := New(modelDir, &noopLogger{})

	if !e.IsModelPresent() {
		t.Skip("model not present at " + modelDir + ", skipping integration test")
	}

	texts := []string{
		"added authentication middleware",
		"removed rate limiting for performance",
		"refactored database connection pool",
	}

	ctx := context.Background()
	results, err := e.EmbedBatch(ctx, texts)
	if err != nil {
		t.Fatalf("EmbedBatch returned an unexpected error: %v", err)
	}

	// Count check.
	if len(results) != len(texts) {
		t.Fatalf("expected %d embeddings, got %d", len(texts), len(results))
	}

	// Dimension check for each vector.
	for i, vec := range results {
		if len(vec) != EmbedDimension {
			t.Errorf("result[%d]: expected length %d, got %d", i, EmbedDimension, len(vec))
		}
	}

	// Sanity check: different texts should produce different vectors.
	if results[0][0] == results[1][0] {
		t.Error("first elements of different embeddings are identical — this is suspicious")
	}

	t.Logf("Embedded %d texts, all %d-dimensional ✓", len(results), EmbedDimension)
}

// TestEmbed_RealModel_IdleUnload verifies the automatic idle-unload behaviour.
// The test overrides the idle timer after the first embed to a very short
// duration (200 ms) to keep the test fast.
func TestEmbed_RealModel_IdleUnload(t *testing.T) {
	modelDir := DefaultModelDir()
	e := New(modelDir, &noopLogger{})

	if !e.IsModelPresent() {
		t.Skip("model not present at " + modelDir + ", skipping integration test")
	}

	ctx := context.Background()

	// Trigger the first embed so the model is loaded.
	_, err := e.Embed(ctx, "initial load")
	if err != nil {
		t.Fatalf("first Embed failed: %v", err)
	}

	if !e.IsLoaded() {
		t.Fatal("expected IsLoaded() == true after first Embed")
	}

	// Override the idle timer to fire after 200 ms.
	e.mu.Lock()
	if e.idleTimer != nil {
		e.idleTimer.Stop()
	}
	e.idleTimer = time.AfterFunc(200*time.Millisecond, func() {
		e.mu.Lock()
		defer e.mu.Unlock()
		e.unload()
	})
	e.mu.Unlock()

	// Wait long enough for the timer to fire.
	time.Sleep(400 * time.Millisecond)

	if e.IsLoaded() {
		t.Fatal("expected IsLoaded() == false after idle timeout, model is still loaded")
	}
	t.Log("model unloaded after idle timeout ✓")
}

// TestEmbed_RealModel_ReloadsAfterUnload verifies that the engine transparently
// reloads the model after an idle-timeout unload.
func TestEmbed_RealModel_ReloadsAfterUnload(t *testing.T) {
	modelDir := DefaultModelDir()
	e := New(modelDir, &noopLogger{})

	if !e.IsModelPresent() {
		t.Skip("model not present at " + modelDir + ", skipping integration test")
	}

	ctx := context.Background()

	// First embed — loads the model.
	_, err := e.Embed(ctx, "first embed")
	if err != nil {
		t.Fatalf("first Embed failed: %v", err)
	}

	// Override the idle timer to a very short duration.
	e.mu.Lock()
	if e.idleTimer != nil {
		e.idleTimer.Stop()
	}
	e.idleTimer = time.AfterFunc(200*time.Millisecond, func() {
		e.mu.Lock()
		defer e.mu.Unlock()
		e.unload()
	})
	e.mu.Unlock()

	// Wait for unload.
	time.Sleep(400 * time.Millisecond)

	if e.IsLoaded() {
		t.Fatal("model should be unloaded at this point")
	}

	// Second embed — should reload transparently.
	vector, err := e.Embed(ctx, "second embed after reload")
	if err != nil {
		t.Fatalf("second Embed (after reload) failed: %v", err)
	}
	if len(vector) != EmbedDimension {
		t.Fatalf("expected %d dimensions after reload, got %d", EmbedDimension, len(vector))
	}

	t.Logf("model reloaded transparently, got %d-dimensional vector ✓", len(vector))
}
