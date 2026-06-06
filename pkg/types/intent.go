package types

import "time"

// IntentType represents the category of a developer intent.
type IntentType string

const (
	IntentChange   IntentType = "change"
	IntentNewFile  IntentType = "new_file"
	IntentDelete   IntentType = "delete"
	IntentRefactor IntentType = "refactor"
)

// ContextLevel represents the confidence level of captured context.
type ContextLevel string

const (
	ContextNormal   ContextLevel = "normal"
	ContextLow      ContextLevel = "low"
	ContextInferred ContextLevel = "inferred"
)

// Intent represents a single developer intent capture.
type Intent struct {
	ID         string       `json:"id"`
	ProjectID  string       `json:"project_id"`
	FilePath   string       `json:"file_path"`
	Branch     string       `json:"branch"`
	CommitHash string       `json:"commit_hash"`
	Developer  string       `json:"developer"`
	Timestamp  time.Time    `json:"timestamp"`
	Type       IntentType   `json:"type"`
	What       string       `json:"what"`
	Why        string       `json:"why"`
	Impact     string       `json:"impact"`
	Context    ContextLevel `json:"context"`
}

// FileEntry represents metadata about a file tracked by Synth.
type FileEntry struct {
	FilePath  string    `json:"file_path"`
	ProjectID string    `json:"project_id"`
	Purpose   string    `json:"purpose"`
	Owns      string    `json:"owns"`
	Boundary  string    `json:"boundary"`
	CreatedBy string    `json:"created_by"`
	CreatedAt time.Time `json:"created_at"`
}
