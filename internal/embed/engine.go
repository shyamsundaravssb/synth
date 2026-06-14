// Package embed provides the local embedding engine for Synth.
// It wraps hugot v0.7.5 and the all-MiniLM-L6-v2 ONNX model to produce
// 384-dimensional float32 vectors from intent note text.
//
// The engine is lazy-loading: the model is not loaded until the first
// call to Embed or EmbedBatch. After IdleTimeout of inactivity the model
// is automatically unloaded to reclaim memory, and will be transparently
// reloaded on the next call.
package embed

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/knights-analytics/hugot"
	"github.com/knights-analytics/hugot/pipelines"
)

// ─── Constants ────────────────────────────────────────────────────────────────

const (
	// ModelName is the canonical name of the embedding model.
	ModelName = "all-MiniLM-L6-v2"

	// EmbedDimension is the number of float32 values in each output vector.
	EmbedDimension = 384

	// IdleTimeout is how long the engine waits after the last embed call
	// before automatically unloading the model to free memory.
	IdleTimeout = 60 * time.Second
)

// DefaultModelDir returns the path to the directory where the model files
// should be installed. It uses the current user's home directory.
// Falls back to a relative path if home resolution fails.
func DefaultModelDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return filepath.Join(".synth", "models", ModelName)
	}
	return filepath.Join(home, ".synth", "models", ModelName)
}

// ─── Logger interface ─────────────────────────────────────────────────────────

// EngineLogger is the logging interface the embedding engine expects from its
// caller. This matches the same shape used by the daemon and IPC packages.
type EngineLogger interface {
	Info(msg string)
	Warn(msg string)
	Error(msg, errStr string)
}

// ─── Engine ───────────────────────────────────────────────────────────────────

// Engine is the production embedding engine. It is safe for concurrent use.
// Load is lazy and unload is automatic after IdleTimeout.
type Engine struct {
	modelDir  string
	session   *hugot.Session
	pipeline  *pipelines.FeatureExtractionPipeline
	mu        sync.Mutex
	lastUsed  time.Time
	idleTimer *time.Timer
	log       EngineLogger
}

// New creates a new Engine. The model is NOT loaded immediately; it will be
// loaded on the first call to Embed or EmbedBatch.
func New(modelDir string, log EngineLogger) *Engine {
	return &Engine{
		modelDir: modelDir,
		log:      log,
	}
}

// ─── IsModelPresent ───────────────────────────────────────────────────────────

// IsModelPresent reports whether the model directory exists and contains the
// minimum required files: model.onnx and tokenizer.json.
func (e *Engine) IsModelPresent() bool {
	required := []string{"model.onnx", "tokenizer.json"}
	for _, f := range required {
		path := filepath.Join(e.modelDir, f)
		if _, err := os.Stat(path); err != nil {
			return false
		}
	}
	return true
}

// ─── load (internal) ──────────────────────────────────────────────────────────

// load initialises the hugot session and feature extraction pipeline.
// Caller MUST hold e.mu.
func (e *Engine) load(ctx context.Context) error {
	// Step 1 — Verify model files are present.
	if !e.IsModelPresent() {
		return fmt.Errorf("model not found at %s — run synth daemon start to download it", e.modelDir)
	}

	// Step 2 — Create session. Try ORT first; fall back to pure-Go backend.
	session, err := hugot.NewORTSession(ctx)
	if err != nil {
		e.log.Info("ORT backend unavailable, falling back to Go backend")
		session, err = hugot.NewGoSession(ctx)
		if err != nil {
			return fmt.Errorf("failed to create hugot session: %w", err)
		}
	}
	e.session = session

	// Step 3 — Create the feature extraction pipeline.
	cfg := hugot.FeatureExtractionConfig{
		ModelPath: e.modelDir,
		Name:      "embedder",
	}
	pipeline, err := hugot.NewPipeline(e.session, cfg)
	if err != nil {
		_ = e.session.Destroy()
		e.session = nil
		return fmt.Errorf("failed to create pipeline: %w", err)
	}
	e.pipeline = pipeline

	// Step 4 — Log success.
	e.log.Info("embedding engine loaded: " + e.modelDir)
	return nil
}

// ─── unload (internal) ────────────────────────────────────────────────────────

// unload destroys the session and clears the pipeline reference.
// Caller MUST hold e.mu.
func (e *Engine) unload() {
	if e.session != nil {
		_ = e.session.Destroy()
		e.session = nil
		e.pipeline = nil
		e.log.Info("embedding engine unloaded (idle timeout)")
	}
}

// ─── resetIdleTimer (internal) ────────────────────────────────────────────────

// resetIdleTimer restarts the idle countdown after a successful embed.
// Caller MUST hold e.mu.
func (e *Engine) resetIdleTimer() {
	if e.idleTimer != nil {
		e.idleTimer.Stop()
	}
	e.idleTimer = time.AfterFunc(IdleTimeout, func() {
		e.mu.Lock()
		defer e.mu.Unlock()
		e.unload()
	})
}

// ─── Embed ────────────────────────────────────────────────────────────────────

// Embed encodes a single text string into a 384-dimensional float32 vector.
// The model is loaded lazily on the first call and unloaded after IdleTimeout
// of inactivity. Embed is safe for concurrent use.
func (e *Engine) Embed(ctx context.Context, text string) ([]float32, error) {
	// Step 1 — Exclusive lock for the entire operation.
	e.mu.Lock()
	defer e.mu.Unlock()

	// Step 2 — Load the model if it is not already in memory.
	if e.pipeline == nil {
		if err := e.load(ctx); err != nil {
			return nil, err
		}
	}

	// Step 3 — Record the time of this access.
	e.lastUsed = time.Now()

	// Step 4 — Run the pipeline.
	output, err := e.pipeline.RunPipeline(ctx, []string{text})
	if err != nil {
		return nil, fmt.Errorf("embedding failed: %w", err)
	}

	// Step 5 — Validate output.
	if len(output.Embeddings) == 0 {
		return nil, fmt.Errorf("no embeddings returned")
	}
	if len(output.Embeddings[0]) != EmbedDimension {
		return nil, fmt.Errorf("unexpected embedding dimension: %d", len(output.Embeddings[0]))
	}

	// Step 6 — Reset the idle timer.
	e.resetIdleTimer()

	// Step 7 — Return the single embedding vector.
	return output.Embeddings[0], nil
}

// ─── EmbedBatch ───────────────────────────────────────────────────────────────

// EmbedBatch encodes multiple texts in a single pipeline call, returning one
// 384-dimensional vector per input string. An empty input slice returns an
// empty slice with no error.
func (e *Engine) EmbedBatch(ctx context.Context, texts []string) ([][]float32, error) {
	// Step 1 — Exclusive lock for the entire operation.
	e.mu.Lock()
	defer e.mu.Unlock()

	// Step 2 — Short-circuit on empty input (no load required).
	if len(texts) == 0 {
		return [][]float32{}, nil
	}

	// Step 3 — Load the model if it is not already in memory.
	if e.pipeline == nil {
		if err := e.load(ctx); err != nil {
			return nil, err
		}
	}

	// Step 4 — Run the pipeline on the full batch.
	output, err := e.pipeline.RunPipeline(ctx, texts)
	if err != nil {
		return nil, fmt.Errorf("batch embedding failed: %w", err)
	}

	// Step 5 — Validate output count.
	if len(output.Embeddings) != len(texts) {
		return nil, fmt.Errorf(
			"embedding count mismatch: expected %d got %d",
			len(texts), len(output.Embeddings),
		)
	}

	// Step 6 — Update last-used timestamp and reset idle timer.
	e.lastUsed = time.Now()
	e.resetIdleTimer()

	// Step 7 — Return all embedding vectors.
	return output.Embeddings, nil
}

// ─── IsLoaded ─────────────────────────────────────────────────────────────────

// IsLoaded reports whether the model is currently loaded in memory.
func (e *Engine) IsLoaded() bool {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.pipeline != nil
}

// ─── Shutdown ─────────────────────────────────────────────────────────────────

// Shutdown stops the idle timer and immediately unloads the model. This should
// be called when the daemon is stopping. It is safe to call on an engine that
// has never been loaded.
func (e *Engine) Shutdown() {
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.idleTimer != nil {
		e.idleTimer.Stop()
	}
	e.unload()
}
