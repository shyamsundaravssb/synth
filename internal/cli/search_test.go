package cli

import (
	"bytes"
	"encoding/json"
	"io"
	"os"
	"testing"
	"time"

	"github.com/shyamsundaravssb/synth/internal/search"
	"github.com/shyamsundaravssb/synth/internal/ui"
)

func TestBuildSearchRequest_Defaults(t *testing.T) {
	flags := searchFlags{limit: 10}
	var zero time.Time

	req := buildSearchRequest("auth", flags, zero)

	if req.Query != "auth" {
		t.Errorf("expected Query auth, got %q", req.Query)
	}
	if req.Limit != 10 {
		t.Errorf("expected Limit 10, got %d", req.Limit)
	}
	if req.FilePath != "" {
		t.Errorf("expected empty FilePath, got %q", req.FilePath)
	}
	if !req.Since.IsZero() {
		t.Errorf("expected zero Since time, got %v", req.Since)
	}
	if req.NoFallback {
		t.Errorf("expected NoFallback false")
	}
}

func TestBuildSearchRequest_AllFlags(t *testing.T) {
	flags := searchFlags{
		limit:      5,
		file:       "auth.go",
		developer:  "shyam",
		noFallback: true,
	}
	sinceTime := time.Now()

	req := buildSearchRequest("auth", flags, sinceTime)

	if req.Limit != 5 {
		t.Errorf("expected Limit 5, got %d", req.Limit)
	}
	if req.FilePath != "auth.go" {
		t.Errorf("expected FilePath auth.go, got %q", req.FilePath)
	}
	if req.Developer != "shyam" {
		t.Errorf("expected Developer shyam, got %q", req.Developer)
	}
	if req.Since != sinceTime {
		t.Errorf("expected Since %v, got %v", sinceTime, req.Since)
	}
	if !req.NoFallback {
		t.Errorf("expected NoFallback true")
	}
}

func captureStdout(f func()) string {
	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	f()

	w.Close()
	os.Stdout = old
	var buf bytes.Buffer
	io.Copy(&buf, r)
	return buf.String()
}

func TestRenderSearchResultsJSON_ValidOutput(t *testing.T) {
	resp := &search.SearchResponse{
		Mode:       search.ModeSemantic,
		IsFallback: false,
		Query:      "test query",
		Results: []search.SearchResult{
			{ID: "r1", Score: 0.87, Mode: search.ModeSemantic},
			{ID: "r2", Score: 0.72, Mode: search.ModeSemantic},
		},
	}

	out := captureStdout(func() {
		_ = ui.RenderSearchResultsJSON(resp)
	})

	var parsed map[string]interface{}
	if err := json.Unmarshal([]byte(out), &parsed); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}

	if parsed["version"].(float64) != 1 {
		t.Errorf("expected version 1")
	}
	if parsed["count"].(float64) != 2 {
		t.Errorf("expected count 2")
	}
	if parsed["mode"].(string) != "semantic" {
		t.Errorf("expected mode semantic")
	}
	if parsed["is_fallback"].(bool) {
		t.Errorf("expected is_fallback false")
	}

	results := parsed["results"].([]interface{})
	r0 := results[0].(map[string]interface{})
	if r0["score_pct"].(float64) != 87 {
		t.Errorf("expected r0 score_pct 87, got %v", r0["score_pct"])
	}
	r1 := results[1].(map[string]interface{})
	if r1["score_pct"].(float64) != 72 {
		t.Errorf("expected r1 score_pct 72, got %v", r1["score_pct"])
	}
}

func TestRenderSearchResultsJSON_FTS5Mode(t *testing.T) {
	resp := &search.SearchResponse{
		Mode:       search.ModeFTS5,
		IsFallback: true,
		Query:      "test fts5",
		Results: []search.SearchResult{
			{ID: "r1", Score: 0.0, Mode: search.ModeFTS5},
		},
	}

	out := captureStdout(func() {
		_ = ui.RenderSearchResultsJSON(resp)
	})

	var parsed map[string]interface{}
	if err := json.Unmarshal([]byte(out), &parsed); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}

	if parsed["mode"].(string) != "fts5" {
		t.Errorf("expected mode fts5")
	}
	if !parsed["is_fallback"].(bool) {
		t.Errorf("expected is_fallback true")
	}
	results := parsed["results"].([]interface{})
	r0 := results[0].(map[string]interface{})
	if r0["score_pct"].(float64) != 0 {
		t.Errorf("expected score_pct 0, got %v", r0["score_pct"])
	}
}

func TestRenderSearchResultsJSON_EmptyResults(t *testing.T) {
	resp := &search.SearchResponse{
		Mode:       search.ModeSemantic,
		IsFallback: false,
		Query:      "test empty",
		Results:    []search.SearchResult{},
	}

	out := captureStdout(func() {
		_ = ui.RenderSearchResultsJSON(resp)
	})

	var parsed map[string]interface{}
	if err := json.Unmarshal([]byte(out), &parsed); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}

	if parsed["count"].(float64) != 0 {
		t.Errorf("expected count 0")
	}

	results, ok := parsed["results"].([]interface{})
	if !ok || len(results) != 0 {
		t.Errorf("expected empty results array, got %v", results)
	}
}
