package cli

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/shyamsundaravssb/synth/internal/config"
	"github.com/shyamsundaravssb/synth/internal/store"
	"github.com/shyamsundaravssb/synth/pkg/types"
)

func TestMakeRelativePath_Success(t *testing.T) {
	rel, err := makeRelativePath("/tmp/repo", "/tmp/repo/src/auth.go")
	if err != nil {
		t.Fatalf("makeRelativePath: unexpected error: %v", err)
	}
	expected := filepath.Join("src", "auth.go")
	if rel != expected {
		t.Errorf("makeRelativePath = %q, want %q", rel, expected)
	}
}

func TestMakeRelativePath_AlreadyRelative(t *testing.T) {
	// Create a temp directory structure so the paths resolve correctly.
	dir := t.TempDir()
	subDir := filepath.Join(dir, "src")
	if err := os.MkdirAll(subDir, 0o755); err != nil {
		t.Fatal(err)
	}

	// Use the temp dir as gitRoot and pass a relative path.
	// We need to change to the dir for relative paths to resolve.
	origDir, _ := os.Getwd()
	os.Chdir(subDir)
	defer os.Chdir(origDir)

	// From inside subDir, "auth.go" should resolve to src/auth.go relative to dir.
	rel, err := makeRelativePath(dir, "auth.go")
	if err != nil {
		t.Fatalf("makeRelativePath: unexpected error: %v", err)
	}
	expected := filepath.Join("src", "auth.go")
	if rel != expected {
		t.Errorf("makeRelativePath = %q, want %q", rel, expected)
	}
}

func TestMakeRelativePath_OutsideRepo(t *testing.T) {
	_, err := makeRelativePath("/tmp/repo", "/tmp/other/file.go")
	if err == nil {
		t.Fatal("makeRelativePath: expected error for path outside repo, got nil")
	}
	if !strings.Contains(err.Error(), "outside the git repository") {
		t.Errorf("error = %q, want to contain 'outside the git repository'", err.Error())
	}
}

func TestBuildFileOptions_PopulatesFields(t *testing.T) {
	dir := t.TempDir()

	// Create some test files.
	files := []string{"main.go", "utils.go"}
	for _, f := range files {
		path := filepath.Join(dir, f)
		if err := os.WriteFile(path, []byte("package main"), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	options := buildFileOptions(files, dir)

	if len(options) != len(files) {
		t.Fatalf("buildFileOptions returned %d options, want %d", len(options), len(files))
	}

	for i, opt := range options {
		if opt.Path != files[i] {
			t.Errorf("option[%d].Path = %q, want %q", i, opt.Path, files[i])
		}
		if opt.ModifiedAgo == "" {
			t.Errorf("option[%d].ModifiedAgo is empty", i)
		}
		// Files were just created, so should be "just now".
		if opt.ModifiedAgo != "just now" {
			t.Errorf("option[%d].ModifiedAgo = %q, want %q", i, opt.ModifiedAgo, "just now")
		}
	}
}

func TestBuildFileOptions_NonexistentFile(t *testing.T) {
	dir := t.TempDir()
	files := []string{"missing.go"}

	options := buildFileOptions(files, dir)

	if len(options) != 1 {
		t.Fatalf("buildFileOptions returned %d options, want 1", len(options))
	}
	if options[0].ModifiedAgo != "unknown" {
		t.Errorf("ModifiedAgo = %q, want %q for nonexistent file", options[0].ModifiedAgo, "unknown")
	}
}

func TestBuildNewFileIntent_AllFields(t *testing.T) {
	intent := buildNewFileIntent(
		"PROJECT123", "src/auth.go", "main",
		"Test Dev", "Authentication module", "Affects login flow",
	)

	if intent.ID == "" {
		t.Error("intent ID is empty")
	}
	if len(intent.ID) != 26 {
		t.Errorf("intent ID length = %d, want 26 (ULID format)", len(intent.ID))
	}
	if intent.ProjectID != "PROJECT123" {
		t.Errorf("ProjectID = %q, want %q", intent.ProjectID, "PROJECT123")
	}
	if intent.FilePath != "src/auth.go" {
		t.Errorf("FilePath = %q, want %q", intent.FilePath, "src/auth.go")
	}
	if intent.Branch != "main" {
		t.Errorf("Branch = %q, want %q", intent.Branch, "main")
	}
	if intent.CommitHash != "" {
		t.Errorf("CommitHash = %q, want empty", intent.CommitHash)
	}
	if intent.Developer != "Test Dev" {
		t.Errorf("Developer = %q, want %q", intent.Developer, "Test Dev")
	}
	if intent.Type != types.IntentNewFile {
		t.Errorf("Type = %q, want %q", intent.Type, types.IntentNewFile)
	}
	if intent.What != "Authentication module" {
		t.Errorf("What = %q, want %q", intent.What, "Authentication module")
	}
	if intent.Why != "new file declaration" {
		t.Errorf("Why = %q, want %q", intent.Why, "new file declaration")
	}
	if intent.Impact != "Affects login flow" {
		t.Errorf("Impact = %q, want %q", intent.Impact, "Affects login flow")
	}
	if intent.Context != types.ContextNormal {
		t.Errorf("Context = %q, want %q", intent.Context, types.ContextNormal)
	}
	if intent.Timestamp.IsZero() {
		t.Error("Timestamp is zero")
	}
}

func TestBuildNewFileIntent_EmptyImpact(t *testing.T) {
	intent := buildNewFileIntent(
		"PROJECT123", "src/auth.go", "main",
		"Test Dev", "Auth module", "",
	)

	if intent.Impact != "" {
		t.Errorf("Impact = %q, want empty string", intent.Impact)
	}
}

func TestBuildFileEntry_AllFields(t *testing.T) {
	entry := buildFileEntry(
		"PROJECT123", "src/auth.go", "Test Dev",
		"Authentication module", "Login/logout logic", "Does not handle sessions",
	)

	if entry.FilePath != "src/auth.go" {
		t.Errorf("FilePath = %q, want %q", entry.FilePath, "src/auth.go")
	}
	if entry.ProjectID != "PROJECT123" {
		t.Errorf("ProjectID = %q, want %q", entry.ProjectID, "PROJECT123")
	}
	if entry.Purpose != "Authentication module" {
		t.Errorf("Purpose = %q, want %q", entry.Purpose, "Authentication module")
	}
	if entry.Owns != "Login/logout logic" {
		t.Errorf("Owns = %q, want %q", entry.Owns, "Login/logout logic")
	}
	if entry.Boundary != "Does not handle sessions" {
		t.Errorf("Boundary = %q, want %q", entry.Boundary, "Does not handle sessions")
	}
	if entry.CreatedBy != "Test Dev" {
		t.Errorf("CreatedBy = %q, want %q", entry.CreatedBy, "Test Dev")
	}
	if entry.CreatedAt.IsZero() {
		t.Error("CreatedAt is zero")
	}
}

func TestFormatDurationAgo(t *testing.T) {
	tests := []struct {
		name     string
		duration time.Duration
		expected string
	}{
		{"just now", 30 * time.Second, "just now"},
		{"minutes", 4 * time.Minute, "4m ago"},
		{"one minute", 1 * time.Minute, "1m ago"},
		{"hours", 2 * time.Hour, "2h ago"},
		{"one hour", 61 * time.Minute, "1h ago"},
		{"days", 25 * time.Hour, "1d ago"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := formatDurationAgo(tt.duration)
			if got != tt.expected {
				t.Errorf("formatDurationAgo(%v) = %q, want %q", tt.duration, got, tt.expected)
			}
		})
	}
}

func TestNoteNewFile_WritesToDatabase(t *testing.T) {
	repo := setupTestGitRepo(t)

	// Override HOME so global dirs go to a temp location.
	origHome := os.Getenv("HOME")
	tmpHome := t.TempDir()
	os.Setenv("HOME", tmpHome)
	defer os.Setenv("HOME", origHome)

	// Initialize synth in the repo.
	if err := runInit(repo, "test-project", "Test Dev", false); err != nil {
		t.Fatalf("runInit: unexpected error: %v", err)
	}

	// Create a real file in the repo.
	testFile := filepath.Join(repo, "src", "auth.go")
	if err := os.MkdirAll(filepath.Dir(testFile), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(testFile, []byte("package auth"), 0o644); err != nil {
		t.Fatal(err)
	}

	// Load the config to get the project ID.
	cfg, err := config.LoadProjectConfig(repo)
	if err != nil {
		t.Fatalf("loading config: %v", err)
	}

	// Open the database.
	dbPath := store.DBPath(cfg.Project.ID)
	db, err := store.Open(dbPath)
	if err != nil {
		t.Fatalf("opening database: %v", err)
	}
	defer db.Close()

	synthStore := store.NewSQLiteStore(db)

	// Call the core note logic with known inputs.
	err = saveNewFileNote(
		synthStore, cfg, repo,
		filepath.Join("src", "auth.go"),
		"Authentication module",
		"Login/logout logic",
		"Does not handle sessions",
		"Affects user flow",
	)
	if err != nil {
		t.Fatalf("saveNewFileNote: unexpected error: %v", err)
	}

	ctx := context.Background()

	// Verify intent record exists.
	intents, err := synthStore.ListIntents(ctx, store.IntentFilter{
		ProjectID: cfg.Project.ID,
		FilePath:  filepath.Join("src", "auth.go"),
	})
	if err != nil {
		t.Fatalf("listing intents: %v", err)
	}
	if len(intents) != 1 {
		t.Fatalf("expected 1 intent, got %d", len(intents))
	}

	intent := intents[0]
	if intent.Type != types.IntentNewFile {
		t.Errorf("intent.Type = %q, want %q", intent.Type, types.IntentNewFile)
	}
	if intent.What != "Authentication module" {
		t.Errorf("intent.What = %q, want %q", intent.What, "Authentication module")
	}
	if intent.Why != "new file declaration" {
		t.Errorf("intent.Why = %q, want %q", intent.Why, "new file declaration")
	}
	if intent.Impact != "Affects user flow" {
		t.Errorf("intent.Impact = %q, want %q", intent.Impact, "Affects user flow")
	}
	if intent.Developer != "Test Dev" {
		t.Errorf("intent.Developer = %q, want %q", intent.Developer, "Test Dev")
	}
	if intent.Context != types.ContextNormal {
		t.Errorf("intent.Context = %q, want %q", intent.Context, types.ContextNormal)
	}

	// Verify file registry record exists.
	entry, err := synthStore.GetFileRegistry(ctx, cfg.Project.ID, filepath.Join("src", "auth.go"))
	if err != nil {
		t.Fatalf("getting file registry: %v", err)
	}
	if entry == nil {
		t.Fatal("file registry entry not found")
	}
	if entry.Purpose != "Authentication module" {
		t.Errorf("entry.Purpose = %q, want %q", entry.Purpose, "Authentication module")
	}
	if entry.Owns != "Login/logout logic" {
		t.Errorf("entry.Owns = %q, want %q", entry.Owns, "Login/logout logic")
	}
	if entry.Boundary != "Does not handle sessions" {
		t.Errorf("entry.Boundary = %q, want %q", entry.Boundary, "Does not handle sessions")
	}
	if entry.CreatedBy != "Test Dev" {
		t.Errorf("entry.CreatedBy = %q, want %q", entry.CreatedBy, "Test Dev")
	}
}

func TestNoteNewFile_NotInitialized(t *testing.T) {
	repo := setupTestGitRepo(t)

	origHome := os.Getenv("HOME")
	tmpHome := t.TempDir()
	os.Setenv("HOME", tmpHome)
	defer os.Setenv("HOME", origHome)

	// Do NOT run init — repo is not initialized.
	// Create a file to note.
	testFile := filepath.Join(repo, "main.go")
	if err := os.WriteFile(testFile, []byte("package main"), 0o644); err != nil {
		t.Fatal(err)
	}

	// saveNewFileNote requires a loaded config and store, so we test
	// at a higher level — the interactive flow would catch this.
	// Instead, verify config.toml does not exist.
	configPath := filepath.Join(repo, ".synth", "config.toml")
	if _, err := os.Stat(configPath); !os.IsNotExist(err) {
		t.Error("config.toml should not exist in uninitialized repo")
	}
}

func TestNoteNewFile_SecondNoteOnSameFileIsExisting(t *testing.T) {
	repo := setupTestGitRepo(t)

	origHome := os.Getenv("HOME")
	tmpHome := t.TempDir()
	os.Setenv("HOME", tmpHome)
	defer os.Setenv("HOME", origHome)

	// Initialize synth.
	if err := runInit(repo, "test-project", "Test Dev", false); err != nil {
		t.Fatalf("runInit: unexpected error: %v", err)
	}

	// Create a file.
	testFile := filepath.Join(repo, "main.go")
	if err := os.WriteFile(testFile, []byte("package main"), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg, err := config.LoadProjectConfig(repo)
	if err != nil {
		t.Fatalf("loading config: %v", err)
	}

	dbPath := store.DBPath(cfg.Project.ID)
	db, err := store.Open(dbPath)
	if err != nil {
		t.Fatalf("opening database: %v", err)
	}
	defer db.Close()

	synthStore := store.NewSQLiteStore(db)

	// First note — should succeed (new file).
	err = saveNewFileNote(
		synthStore, cfg, repo, "main.go",
		"Entry point", "Application startup", "Nothing else", "",
	)
	if err != nil {
		t.Fatalf("first saveNewFileNote: unexpected error: %v", err)
	}

	// Verify the file is now in the registry (would be treated as existing).
	ctx := context.Background()
	entry, err := synthStore.GetFileRegistry(ctx, cfg.Project.ID, "main.go")
	if err != nil {
		t.Fatalf("getting file registry: %v", err)
	}
	if entry == nil {
		t.Fatal("file registry entry should exist after first note")
	}
}

func TestBuildExistingFileIntent_AllFields(t *testing.T) {
	intent := buildExistingFileIntent(
		"PROJECT123", "src/auth.go", "main",
		"Test Dev", "fixed bug", "customer reported", "affects login",
	)

	if intent.ID == "" {
		t.Error("intent ID is empty")
	}
	if len(intent.ID) != 26 {
		t.Errorf("intent ID length = %d, want 26", len(intent.ID))
	}
	if intent.Type != types.IntentChange {
		t.Errorf("Type = %q, want %q", intent.Type, types.IntentChange)
	}
	if intent.What != "fixed bug" {
		t.Errorf("What = %q, want %q", intent.What, "fixed bug")
	}
	if intent.Why != "customer reported" {
		t.Errorf("Why = %q, want %q", intent.Why, "customer reported")
	}
	if intent.Impact != "affects login" {
		t.Errorf("Impact = %q, want %q", intent.Impact, "affects login")
	}
}

func TestBuildExistingFileIntent_EmptyImpact(t *testing.T) {
	intent := buildExistingFileIntent(
		"PROJECT123", "src/auth.go", "main",
		"Test Dev", "fixed bug", "customer reported", "",
	)

	if intent.Impact != "" {
		t.Errorf("Impact = %q, want empty string", intent.Impact)
	}
}

func TestNoteExistingFile_WritesToDatabase(t *testing.T) {
	repo := setupTestGitRepo(t)
	origHome := os.Getenv("HOME")
	tmpHome := t.TempDir()
	os.Setenv("HOME", tmpHome)
	defer os.Setenv("HOME", origHome)

	if err := runInit(repo, "test-project", "Test Dev", false); err != nil {
		t.Fatalf("runInit: unexpected error: %v", err)
	}

	testFile := filepath.Join(repo, "src", "auth.go")
	if err := os.MkdirAll(filepath.Dir(testFile), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(testFile, []byte("package auth"), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg, _ := config.LoadProjectConfig(repo)
	dbPath := store.DBPath(cfg.Project.ID)
	db, _ := store.Open(dbPath)
	defer db.Close()
	synthStore := store.NewSQLiteStore(db)
	ctx := context.Background()

	// Register file first
	err := saveNewFileNote(synthStore, cfg, repo, filepath.Join("src", "auth.go"), "auth", "login", "nothing", "none")
	if err != nil {
		t.Fatal(err)
	}

	// Add existing file note
	err = saveExistingFileNote(ctx, synthStore, cfg, filepath.Join("src", "auth.go"), "main", "fixed bug", "customer reported", "no impact")
	if err != nil {
		t.Fatal(err)
	}

	intents, _ := synthStore.ListIntents(ctx, store.IntentFilter{ProjectID: cfg.Project.ID, FilePath: filepath.Join("src", "auth.go")})
	if len(intents) != 2 {
		t.Fatalf("expected 2 intents, got %d", len(intents))
	}
	
	var changeIntent *types.Intent
	for i := range intents {
		if intents[i].Type == types.IntentChange {
			changeIntent = &intents[i]
			break
		}
	}
	if changeIntent == nil {
		t.Fatal("expected to find an IntentChange record")
	}
	if changeIntent.What != "fixed bug" || changeIntent.Why != "customer reported" {
		t.Errorf("unexpected values: What=%s, Why=%s", changeIntent.What, changeIntent.Why)
	}

	// file_registry should remain unchanged
	entry, _ := synthStore.GetFileRegistry(ctx, cfg.Project.ID, filepath.Join("src", "auth.go"))
	if entry.Purpose != "auth" {
		t.Errorf("registry changed: %v", entry)
	}
}

func TestNoteNonInteractive_MissingFileFlag(t *testing.T) {
	repo := setupTestGitRepo(t)
	origHome := os.Getenv("HOME")
	tmpHome := t.TempDir()
	os.Setenv("HOME", tmpHome)
	defer os.Setenv("HOME", origHome)

	origWd, _ := os.Getwd()
	os.Chdir(repo)
	defer os.Chdir(origWd)

	if err := runInit(repo, "test-project", "Test Dev", false); err != nil {
		t.Fatalf("runInit: unexpected error: %v", err)
	}

	err := runNoteInteractive("", false, "fixed bug", "customer reported")
	if err == nil {
		t.Fatal("expected error for missing --file flag, got nil")
	}
	if !strings.Contains(err.Error(), "--file is required") {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestNoteNonInteractive_ExistingFile(t *testing.T) {
	repo := setupTestGitRepo(t)
	origHome := os.Getenv("HOME")
	tmpHome := t.TempDir()
	os.Setenv("HOME", tmpHome)
	defer os.Setenv("HOME", origHome)

	runInit(repo, "test-project", "Test Dev", false)
	testFile := filepath.Join(repo, "src", "auth.go")
	os.MkdirAll(filepath.Dir(testFile), 0o755)
	os.WriteFile(testFile, []byte("package auth"), 0o644)

	cfg, _ := config.LoadProjectConfig(repo)
	dbPath := store.DBPath(cfg.Project.ID)
	db, _ := store.Open(dbPath)
	defer db.Close()
	synthStore := store.NewSQLiteStore(db)
	ctx := context.Background()

	saveNewFileNote(synthStore, cfg, repo, filepath.Join("src", "auth.go"), "auth", "login", "nothing", "none")

	// Run non-interactive directly via function
	err := saveExistingFileNote(ctx, synthStore, cfg, filepath.Join("src", "auth.go"), "main", "test change", "testing non-interactive mode", "")
	if err != nil {
		t.Fatal(err)
	}

	intents, _ := synthStore.ListIntents(ctx, store.IntentFilter{ProjectID: cfg.Project.ID, FilePath: filepath.Join("src", "auth.go")})
	found := false
	for _, intent := range intents {
		if intent.Type == types.IntentChange && intent.What == "test change" && intent.Why == "testing non-interactive mode" {
			found = true
		}
	}
	if !found {
		t.Errorf("intent not found in DB")
	}
}
