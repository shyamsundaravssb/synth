package cli

import (
	"strings"
	"testing"
	"time"

	"github.com/shyamsundaravssb/synth/pkg/types"
)

func TestParseSince_MinutesAgo(t *testing.T) {
	now := time.Now()
	parsed, err := parseSince("30 minutes ago")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	expected := now.Add(-30 * time.Minute)

	// Check within 1 second tolerance
	if parsed.Before(expected.Add(-1*time.Second)) || parsed.After(expected.Add(1*time.Second)) {
		t.Errorf("parsed = %v, want approx %v", parsed, expected)
	}
}

func TestParseSince_HoursAgo(t *testing.T) {
	now := time.Now()
	parsed, err := parseSince("2 hours ago")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	expected := now.Add(-2 * time.Hour)
	if parsed.Before(expected.Add(-1*time.Second)) || parsed.After(expected.Add(1*time.Second)) {
		t.Errorf("parsed = %v, want approx %v", parsed, expected)
	}
}

func TestParseSince_DaysAgo(t *testing.T) {
	now := time.Now()
	parsed, err := parseSince("3 days ago")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	expected := now.Add(-3 * 24 * time.Hour)
	if parsed.Before(expected.Add(-1*time.Second)) || parsed.After(expected.Add(1*time.Second)) {
		t.Errorf("parsed = %v, want approx %v", parsed, expected)
	}
}

func TestParseSince_Yesterday(t *testing.T) {
	now := time.Now()
	parsed, err := parseSince("yesterday")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	y, m, d := now.Date()
	expected := time.Date(y, m, d-1, 0, 0, 0, 0, now.Location())
	if !parsed.Equal(expected) {
		t.Errorf("parsed = %v, want %v", parsed, expected)
	}
}

func TestParseSince_Today(t *testing.T) {
	now := time.Now()
	parsed, err := parseSince("today")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	y, m, d := now.Date()
	expected := time.Date(y, m, d, 0, 0, 0, 0, now.Location())
	if !parsed.Equal(expected) {
		t.Errorf("parsed = %v, want %v", parsed, expected)
	}
}

func TestParseSince_InvalidFormat(t *testing.T) {
	_, err := parseSince("last week")
	if err == nil {
		t.Error("expected error for 'last week', got nil")
	}
	if !strings.Contains(err.Error(), "unrecognized --since format") {
		t.Errorf("unexpected error msg: %v", err)
	}
}

func TestParseSince_EmptyString(t *testing.T) {
	parsed, err := parseSince("")
	if err != nil {
		t.Fatalf("unexpected error for empty string: %v", err)
	}
	if !parsed.IsZero() {
		t.Errorf("expected zero time, got %v", parsed)
	}
}

func TestBuildFileSummary(t *testing.T) {
	intents := []types.Intent{
		{FilePath: "main.go"},
		{FilePath: "main.go"},
		{FilePath: "utils.go"},
		{FilePath: "main.go"},
		{FilePath: "auth.go"},
		{FilePath: "utils.go"},
	}

	summary := buildFileSummary(intents)
	if len(summary) != 3 {
		t.Fatalf("expected 3 files, got %d", len(summary))
	}

	if summary[0].FilePath != "main.go" || summary[0].NoteCount != 3 {
		t.Errorf("expected first to be main.go (3 notes), got %v", summary[0])
	}
	if summary[1].FilePath != "utils.go" || summary[1].NoteCount != 2 {
		t.Errorf("expected second to be utils.go (2 notes), got %v", summary[1])
	}
	if summary[2].FilePath != "auth.go" || summary[2].NoteCount != 1 {
		t.Errorf("expected third to be auth.go (1 note), got %v", summary[2])
	}
}
