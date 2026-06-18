package ui

import (
	"bytes"
	"encoding/json"
	"io"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/shyamsundaravssb/synth/internal/ipc"
	"github.com/shyamsundaravssb/synth/pkg/types"
)

func TestRelativeTime(t *testing.T) {
	// Fixed reference time: 2026-06-05 15:30:00 UTC
	now := time.Date(2026, 6, 5, 15, 30, 0, 0, time.UTC)

	tests := []struct {
		name     string
		input    time.Time
		contains string // substring expected in output
	}{
		{
			name:     "just now (30 seconds ago)",
			input:    now.Add(-30 * time.Second),
			contains: "just now",
		},
		{
			name:     "minutes ago (42m)",
			input:    now.Add(-42 * time.Minute),
			contains: "42m ago",
		},
		{
			name:     "minutes ago (1m)",
			input:    now.Add(-1 * time.Minute),
			contains: "1m ago",
		},
		{
			name:     "today (2 hours ago)",
			input:    now.Add(-2 * time.Hour),
			contains: "today,",
		},
		{
			name:     "yesterday",
			input:    now.Add(-26 * time.Hour),
			contains: "yesterday,",
		},
		{
			name:     "older than 48 hours",
			input:    now.Add(-72 * time.Hour),
			contains: "Jun 2,",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := relativeTimeFrom(tt.input, now)
			if !strings.Contains(result, tt.contains) {
				t.Errorf("relativeTimeFrom() = %q, want to contain %q", result, tt.contains)
			}
		})
	}
}

func TestRelativeTime_TimeFormat(t *testing.T) {
	// Verify the time portion format for "today" bucket.
	now := time.Date(2026, 6, 5, 15, 30, 0, 0, time.UTC)
	input := time.Date(2026, 6, 5, 10, 45, 0, 0, time.UTC) // same day, earlier

	result := relativeTimeFrom(input, now)
	if !strings.Contains(result, "today,") {
		t.Errorf("expected 'today,' prefix, got %q", result)
	}
	if !strings.Contains(result, "10:45 AM") {
		t.Errorf("expected '10:45 AM' in output, got %q", result)
	}
}

func TestRenderIntentJSON_ValidOutput(t *testing.T) {
	intents := []types.Intent{
		{
			ID:        "id-001",
			ProjectID: "synth",
			FilePath:  "internal/store/db.go",
			Branch:    "main",
			Developer: "alice",
			Timestamp: time.Date(2026, 6, 5, 10, 0, 0, 0, time.UTC),
			Type:      types.IntentChange,
			What:      "Added connection pooling",
			Why:       "Performance under load",
			Impact:    "All DB callers",
			Context:   types.ContextNormal,
		},
		{
			ID:        "id-002",
			ProjectID: "synth",
			FilePath:  "internal/ui/render.go",
			Branch:    "main",
			Developer: "alice",
			Timestamp: time.Date(2026, 6, 5, 11, 0, 0, 0, time.UTC),
			Type:      types.IntentNewFile,
			What:      "Terminal output renderer",
			Why:       "",
			Context:   types.ContextNormal,
		},
	}

	output := captureStdout(t, func() {
		if err := RenderIntentJSON(intents); err != nil {
			t.Fatalf("RenderIntentJSON: %v", err)
		}
	})

	// Verify output is valid JSON.
	var envelope map[string]interface{}
	if err := json.Unmarshal([]byte(output), &envelope); err != nil {
		t.Fatalf("output is not valid JSON: %v\noutput: %s", err, output)
	}

	// Verify version field.
	version, ok := envelope["version"].(float64)
	if !ok || int(version) != 1 {
		t.Errorf("version = %v, want 1", envelope["version"])
	}

	// Verify count matches slice length.
	count, ok := envelope["count"].(float64)
	if !ok || int(count) != 2 {
		t.Errorf("count = %v, want 2", envelope["count"])
	}

	// Verify entries array.
	entries, ok := envelope["entries"].([]interface{})
	if !ok {
		t.Fatal("entries is not an array")
	}
	if len(entries) != 2 {
		t.Errorf("entries length = %d, want 2", len(entries))
	}

	// Verify all intent fields present in first entry.
	first, ok := entries[0].(map[string]interface{})
	if !ok {
		t.Fatal("first entry is not an object")
	}
	requiredFields := []string{"id", "project_id", "file_path", "branch",
		"developer", "timestamp", "type", "what", "why", "impact", "context"}
	for _, field := range requiredFields {
		if _, exists := first[field]; !exists {
			t.Errorf("field %q missing from intent JSON", field)
		}
	}
}

func TestRenderIntentJSON_EmptySlice(t *testing.T) {
	output := captureStdout(t, func() {
		if err := RenderIntentJSON([]types.Intent{}); err != nil {
			t.Fatalf("RenderIntentJSON: %v", err)
		}
	})

	var envelope map[string]interface{}
	if err := json.Unmarshal([]byte(output), &envelope); err != nil {
		t.Fatalf("output is not valid JSON: %v\noutput: %s", err, output)
	}

	count, ok := envelope["count"].(float64)
	if !ok || int(count) != 0 {
		t.Errorf("count = %v, want 0", envelope["count"])
	}

	entries, ok := envelope["entries"].([]interface{})
	if !ok {
		t.Fatal("entries is not an array")
	}
	if len(entries) != 0 {
		t.Errorf("entries length = %d, want 0", len(entries))
	}
}

func TestRenderIntentJSON_NilSlice(t *testing.T) {
	output := captureStdout(t, func() {
		if err := RenderIntentJSON(nil); err != nil {
			t.Fatalf("RenderIntentJSON: %v", err)
		}
	})

	var envelope map[string]interface{}
	if err := json.Unmarshal([]byte(output), &envelope); err != nil {
		t.Fatalf("output is not valid JSON: %v\noutput: %s", err, output)
	}

	count := envelope["count"].(float64)
	if int(count) != 0 {
		t.Errorf("count = %v, want 0", envelope["count"])
	}
}

func TestRenderStatus_NoNotes(t *testing.T) {
	status := StatusData{
		ProjectName:     "synth",
		Developer:       "alice",
		TotalNotes:      0,
		FilesWithNotes:  nil,
		LowContextFiles: nil,
		LastNote:        nil,
	}

	output := captureStdout(t, func() {
		RenderStatus(status)
	})

	if !strings.Contains(output, "No notes yet") {
		t.Error("expected 'No notes yet' in output for nil LastNote")
	}
	if !strings.Contains(output, "synth") {
		t.Error("expected project name 'synth' in output")
	}
	if !strings.Contains(output, "alice") {
		t.Error("expected developer name 'alice' in output")
	}
}

func TestRenderStatus_WithNotes(t *testing.T) {
	lastNote := &types.Intent{
		ID:        "id-001",
		FilePath:  "internal/store/db.go",
		Timestamp: time.Now().Add(-5 * time.Minute),
		What:      "Added connection pooling",
	}
	status := StatusData{
		ProjectName: "synth",
		Developer:   "alice",
		TotalNotes:  3,
		FilesWithNotes: []FileNoteSummary{
			{FilePath: "db.go", NoteCount: 2},
			{FilePath: "store.go", NoteCount: 1},
		},
		LowContextFiles: []ipc.LowContextFileItem{{FilePath: "config.go", SaveCount: 5}},
		LastNote:        lastNote,
	}

	output := captureStdout(t, func() {
		RenderStatus(status)
	})

	if !strings.Contains(output, "internal/store/db.go") {
		t.Error("expected last note file path in output")
	}
	if !strings.Contains(output, "Added connection pooling") {
		t.Error("expected last note content in output")
	}
	if !strings.Contains(output, "LOW CONTEXT") {
		t.Error("expected LOW CONTEXT section in output")
	}
	if !strings.Contains(output, "config.go") {
		t.Error("expected low context file in output")
	}
}

func TestRenderIntentLog_EmptyIntents(t *testing.T) {
	output := captureStdout(t, func() {
		RenderIntentLog([]types.Intent{}, nil, "synth", "alice", false)
	})

	if !strings.Contains(output, "no notes found") {
		t.Error("expected 'no notes found' message for empty intents")
	}
}

func TestRenderIntentLog_WithIntents(t *testing.T) {
	intents := []types.Intent{
		{
			ID:        "id-001",
			ProjectID: "synth",
			FilePath:  "db.go",
			Branch:    "main",
			Developer: "alice",
			Timestamp: time.Now().Add(-5 * time.Minute),
			Type:      types.IntentChange,
			What:      "Added pooling",
			Why:       "Performance",
			Impact:    "All callers",
		},
	}

	output := captureStdout(t, func() {
		RenderIntentLog(intents, nil, "synth", "alice", false)
	})

	if !strings.Contains(output, "SYNTH LOG") {
		t.Error("expected 'SYNTH LOG' header")
	}
	if !strings.Contains(output, "db.go") {
		t.Error("expected file path in output")
	}
	if !strings.Contains(output, "Added pooling") {
		t.Error("expected 'What' content in output")
	}
}

func TestRenderLowContextFiles_Empty(t *testing.T) {
	output := captureStdout(t, func() {
		RenderLowContextFiles([]ipc.LowContextFileItem{}, "synth", false)
	})

	if !strings.Contains(output, "No low context files") {
		t.Error("expected 'No low context files' in output")
	}
}

func TestRenderLowContextFiles_NeverNoted(t *testing.T) {
	item := ipc.LowContextFileItem{
		FilePath:         "auth.go",
		SaveCount:        8,
		HasEverBeenNoted: false,
		DaysSinceNote:    0,
	}
	output := captureStdout(t, func() {
		RenderLowContextFiles([]ipc.LowContextFileItem{item}, "synth", false)
	})

	if !strings.Contains(output, "auth.go") {
		t.Error("expected 'auth.go' in output")
	}
	if !strings.Contains(output, "8 saves") {
		t.Error("expected '8 saves' in output")
	}
	if !strings.Contains(output, "never noted") {
		t.Error("expected 'never noted' in output")
	}
}

func TestRenderLowContextFiles_SingularGrammar(t *testing.T) {
	item := ipc.LowContextFileItem{
		FilePath:         "auth.go",
		SaveCount:        8,
		HasEverBeenNoted: false,
		DaysSinceNote:    0,
	}
	output := captureStdout(t, func() {
		RenderLowContextFiles([]ipc.LowContextFileItem{item}, "synth", false)
	})

	if !strings.Contains(output, "1 file needs attention") {
		t.Error("expected '1 file needs attention' in output")
	}
	if strings.Contains(output, "files need attention") {
		t.Error("expected output to NOT contain 'files need attention'")
	}
}

func TestRenderLowContextFiles_WithOldNote(t *testing.T) {
	item := ipc.LowContextFileItem{
		FilePath:         "users.go",
		SaveCount:        6,
		HasEverBeenNoted: true,
		DaysSinceNote:    5,
	}
	output := captureStdout(t, func() {
		RenderLowContextFiles([]ipc.LowContextFileItem{item}, "synth", false)
	})

	if !strings.Contains(output, "5 days ago") {
		t.Error("expected '5 days ago' in output")
	}
}

func TestRenderLowContextFilesJSON_ValidOutput(t *testing.T) {
	items := []ipc.LowContextFileItem{
		{
			FilePath:         "auth.go",
			SaveCount:        8,
			HasEverBeenNoted: false,
			DaysSinceNote:    0,
		},
		{
			FilePath:         "users.go",
			SaveCount:        6,
			HasEverBeenNoted: true,
			DaysSinceNote:    5,
		},
	}

	output := captureStdout(t, func() {
		RenderLowContextFiles(items, "synth", true)
	})

	var envelope map[string]interface{}
	if err := json.Unmarshal([]byte(output), &envelope); err != nil {
		t.Fatalf("output is not valid JSON: %v\noutput: %s", err, output)
	}

	version, ok := envelope["version"].(float64)
	if !ok || int(version) != 1 {
		t.Errorf("version = %v, want 1", envelope["version"])
	}

	count, ok := envelope["count"].(float64)
	if !ok || int(count) != 2 {
		t.Errorf("count = %v, want 2", envelope["count"])
	}
}

// captureStdout captures stdout output during fn execution.
func captureStdout(t *testing.T, fn func()) string {
	t.Helper()

	old := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("creating pipe: %v", err)
	}

	os.Stdout = w

	fn()

	w.Close()
	os.Stdout = old

	var buf bytes.Buffer
	if _, err := io.Copy(&buf, r); err != nil {
		t.Fatalf("reading captured output: %v", err)
	}

	return buf.String()
}
