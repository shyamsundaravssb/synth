package embed

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

// ─── noopLogger ───────────────────────────────────────────────────────────────

// noopLogger satisfies EngineLogger without producing any output.
// Used in all tests that do not need to observe log calls.
type noopLogger struct{}

func (n *noopLogger) Info(msg string)     {}
func (n *noopLogger) Warn(msg string)     {}
func (n *noopLogger) Error(msg, e string) {}

// ─── Unit tests (no build tag — always run) ───────────────────────────────────

// TestNew_DoesNotLoadOnCreation verifies that constructing an Engine does not
// trigger model loading. IsLoaded must return false immediately after New().
func TestNew_DoesNotLoadOnCreation(t *testing.T) {
	t.Parallel()
	e := New("/non/existent/model/dir", &noopLogger{})
	if e.IsLoaded() {
		t.Fatal("engine reported loaded immediately after construction, expected not loaded")
	}
}

// TestIsModelPresent_MissingDir verifies that IsModelPresent returns false when
// the model directory does not exist at all.
func TestIsModelPresent_MissingDir(t *testing.T) {
	t.Parallel()
	e := New("/this/path/does/not/exist", &noopLogger{})
	if e.IsModelPresent() {
		t.Fatal("expected IsModelPresent to return false for non-existent directory")
	}
}

// TestIsModelPresent_MissingFiles verifies that IsModelPresent returns false when
// the directory exists but does not contain the required model files.
func TestIsModelPresent_MissingFiles(t *testing.T) {
	t.Parallel()
	dir := t.TempDir() // empty temporary directory
	e := New(dir, &noopLogger{})
	if e.IsModelPresent() {
		t.Fatal("expected IsModelPresent to return false for directory without model files")
	}
}

// TestIsModelPresent_PresentWithRequiredFiles verifies that IsModelPresent
// returns true when both model.onnx and tokenizer.json are present in the
// model directory (file contents are irrelevant — presence is what matters).
func TestIsModelPresent_PresentWithRequiredFiles(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	// Create the two required files (empty is fine — we only check existence).
	for _, name := range []string{"model.onnx", "tokenizer.json"} {
		path := filepath.Join(dir, name)
		if err := os.WriteFile(path, []byte{}, 0600); err != nil {
			t.Fatalf("failed to create %s: %v", name, err)
		}
	}

	e := New(dir, &noopLogger{})
	if !e.IsModelPresent() {
		t.Fatal("expected IsModelPresent to return true when required files are present")
	}
}

// TestEmbed_ModelNotPresent verifies that Embed returns an error (not a panic)
// when the model directory does not exist, and that the error message includes
// "model not found".
func TestEmbed_ModelNotPresent(t *testing.T) {
	t.Parallel()
	e := New("/this/path/does/not/exist", &noopLogger{})
	ctx := context.Background()

	_, err := e.Embed(ctx, "test input")
	if err == nil {
		t.Fatal("expected an error from Embed when model is not present, got nil")
	}

	const want = "model not found"
	if !containsSubstring(err.Error(), want) {
		t.Errorf("expected error to contain %q, got: %v", want, err)
	}
}

// TestEmbedBatch_EmptyInput verifies that calling EmbedBatch with an empty
// slice is not an error — it should return an empty slice and a nil error.
func TestEmbedBatch_EmptyInput(t *testing.T) {
	t.Parallel()
	// Use a valid-looking directory — load will be triggered only after the
	// empty-input short-circuit, so no real model is needed.
	e := New("/non/existent", &noopLogger{})
	ctx := context.Background()

	results, err := e.EmbedBatch(ctx, []string{})
	if err != nil {
		t.Fatalf("expected no error for empty batch input, got: %v", err)
	}
	if len(results) != 0 {
		t.Fatalf("expected empty slice for empty batch input, got len=%d", len(results))
	}
}

// TestShutdown_WhenNotLoaded verifies that calling Shutdown on an engine that
// was never loaded does not panic or error.
func TestShutdown_WhenNotLoaded(t *testing.T) {
	t.Parallel()
	e := New("/non/existent", &noopLogger{})
	// Must not panic.
	e.Shutdown()
}

// TestIsLoaded_FalseBeforeLoad verifies that IsLoaded returns false before any
// embed call is made.
func TestIsLoaded_FalseBeforeLoad(t *testing.T) {
	t.Parallel()
	e := New(DefaultModelDir(), &noopLogger{})
	if e.IsLoaded() {
		t.Fatal("expected IsLoaded to return false before any Embed call")
	}
}

// ─── helpers ──────────────────────────────────────────────────────────────────

// containsSubstring is a simple check to avoid importing strings in tests.
func containsSubstring(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
