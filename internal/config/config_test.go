package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// ---------------------------------------------------------------------------
// Project config tests
// ---------------------------------------------------------------------------

func TestSaveProjectConfig_WritesValidFile(t *testing.T) {
	root := t.TempDir()

	cfg := &ProjectConfig{
		Project: ProjectSection{
			ID:      "proj-001",
			Name:    "test-project",
			Created: "2026-06-04T10:00:00Z",
		},
		Developer: DeveloperSection{Name: "alice"},
		Behavior:  BehaviorSection{LowContextThreshold: 5},
		Sync: SyncSection{
			ServerURL:     "http://localhost:9090",
			IntervalHours: 12,
		},
	}

	if err := SaveProjectConfig(root, cfg); err != nil {
		t.Fatalf("SaveProjectConfig() error = %v", err)
	}

	// Verify the file was actually written.
	path := filepath.Join(root, ".synth", "config.toml")
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("config file not found: %v", err)
	}
	if info.Size() == 0 {
		t.Fatal("config file is empty")
	}
}

func TestLoadProjectConfig_RoundTrip(t *testing.T) {
	root := t.TempDir()

	original := &ProjectConfig{
		Project: ProjectSection{
			ID:      "proj-002",
			Name:    "round-trip",
			Created: "2026-06-04T10:00:00Z",
		},
		Developer: DeveloperSection{Name: "bob"},
		Behavior:  BehaviorSection{LowContextThreshold: 7},
		Sync: SyncSection{
			ServerURL:     "https://sync.example.com",
			IntervalHours: 24,
		},
	}

	if err := SaveProjectConfig(root, original); err != nil {
		t.Fatalf("SaveProjectConfig() error = %v", err)
	}

	loaded, err := LoadProjectConfig(root)
	if err != nil {
		t.Fatalf("LoadProjectConfig() error = %v", err)
	}

	// Verify every field round-trips correctly.
	if loaded.Project.ID != original.Project.ID {
		t.Errorf("Project.ID = %q, want %q", loaded.Project.ID, original.Project.ID)
	}
	if loaded.Project.Name != original.Project.Name {
		t.Errorf("Project.Name = %q, want %q", loaded.Project.Name, original.Project.Name)
	}
	if loaded.Project.Created != original.Project.Created {
		t.Errorf("Project.Created = %q, want %q", loaded.Project.Created, original.Project.Created)
	}
	if loaded.Developer.Name != original.Developer.Name {
		t.Errorf("Developer.Name = %q, want %q", loaded.Developer.Name, original.Developer.Name)
	}
	if loaded.Behavior.LowContextThreshold != original.Behavior.LowContextThreshold {
		t.Errorf("Behavior.LowContextThreshold = %d, want %d",
			loaded.Behavior.LowContextThreshold, original.Behavior.LowContextThreshold)
	}
	if loaded.Sync.ServerURL != original.Sync.ServerURL {
		t.Errorf("Sync.ServerURL = %q, want %q", loaded.Sync.ServerURL, original.Sync.ServerURL)
	}
	if loaded.Sync.IntervalHours != original.Sync.IntervalHours {
		t.Errorf("Sync.IntervalHours = %d, want %d",
			loaded.Sync.IntervalHours, original.Sync.IntervalHours)
	}
}

func TestLoadProjectConfig_MissingFileReturnsDefaults(t *testing.T) {
	root := t.TempDir() // No config file written.

	cfg, err := LoadProjectConfig(root)
	if err != nil {
		t.Fatalf("LoadProjectConfig() error = %v, want nil", err)
	}

	defaults := DefaultProjectConfig()
	if cfg.Behavior.LowContextThreshold != defaults.Behavior.LowContextThreshold {
		t.Errorf("LowContextThreshold = %d, want default %d",
			cfg.Behavior.LowContextThreshold, defaults.Behavior.LowContextThreshold)
	}
	if cfg.Sync.IntervalHours != defaults.Sync.IntervalHours {
		t.Errorf("IntervalHours = %d, want default %d",
			cfg.Sync.IntervalHours, defaults.Sync.IntervalHours)
	}
	if cfg.Project.ID != "" {
		t.Errorf("Project.ID = %q, want empty string", cfg.Project.ID)
	}
}

// ---------------------------------------------------------------------------
// Global config tests
// ---------------------------------------------------------------------------

func TestSaveGlobalConfig_WritesCorrectly(t *testing.T) {
	dir := t.TempDir()

	cfg := &GlobalConfig{
		User: UserSection{
			Name:  "charlie",
			Email: "charlie@example.com",
		},
	}

	if err := SaveGlobalConfig(cfg, dir); err != nil {
		t.Fatalf("SaveGlobalConfig() error = %v", err)
	}

	path := filepath.Join(dir, "global.toml")
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("global config file not found: %v", err)
	}
	if info.Size() == 0 {
		t.Fatal("global config file is empty")
	}
}

func TestLoadGlobalConfig_RoundTrip(t *testing.T) {
	dir := t.TempDir()

	original := &GlobalConfig{
		User: UserSection{
			Name:  "dana",
			Email: "dana@example.com",
		},
	}

	if err := SaveGlobalConfig(original, dir); err != nil {
		t.Fatalf("SaveGlobalConfig() error = %v", err)
	}

	loaded, err := LoadGlobalConfig(dir)
	if err != nil {
		t.Fatalf("LoadGlobalConfig() error = %v", err)
	}

	if loaded.User.Name != original.User.Name {
		t.Errorf("User.Name = %q, want %q", loaded.User.Name, original.User.Name)
	}
	if loaded.User.Email != original.User.Email {
		t.Errorf("User.Email = %q, want %q", loaded.User.Email, original.User.Email)
	}
}

func TestLoadGlobalConfig_MissingFileReturnsDefaults(t *testing.T) {
	dir := t.TempDir() // No config file written.

	cfg, err := LoadGlobalConfig(dir)
	if err != nil {
		t.Fatalf("LoadGlobalConfig() error = %v, want nil", err)
	}

	if cfg.User.Name != "" {
		t.Errorf("User.Name = %q, want empty string", cfg.User.Name)
	}
	if cfg.User.Email != "" {
		t.Errorf("User.Email = %q, want empty string", cfg.User.Email)
	}
}

// ---------------------------------------------------------------------------
// EnsureGlobalDirs tests
// ---------------------------------------------------------------------------

func TestEnsureGlobalDirs_CreatesDirectories(t *testing.T) {
	base := t.TempDir()
	dir := filepath.Join(base, "fresh-synth") // Does not exist yet.

	if err := EnsureGlobalDirs(dir); err != nil {
		t.Fatalf("EnsureGlobalDirs() error = %v", err)
	}

	// Verify both directories were created.
	if info, err := os.Stat(dir); err != nil || !info.IsDir() {
		t.Errorf("expected %q to be a directory", dir)
	}
	projects := filepath.Join(dir, "projects")
	if info, err := os.Stat(projects); err != nil || !info.IsDir() {
		t.Errorf("expected %q to be a directory", projects)
	}
}

func TestEnsureGlobalDirs_Idempotent(t *testing.T) {
	base := t.TempDir()
	dir := filepath.Join(base, "idempotent-synth")

	// Call twice — second call must not error.
	if err := EnsureGlobalDirs(dir); err != nil {
		t.Fatalf("EnsureGlobalDirs() first call error = %v", err)
	}
	if err := EnsureGlobalDirs(dir); err != nil {
		t.Fatalf("EnsureGlobalDirs() second call error = %v", err)
	}

	// Directories should still exist.
	if _, err := os.Stat(filepath.Join(dir, "projects")); err != nil {
		t.Errorf("projects dir missing after second call: %v", err)
	}
}

// ---------------------------------------------------------------------------
// Error path and edge case tests
// ---------------------------------------------------------------------------

func TestLoadProjectConfig_MalformedTOML(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, ".synth")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}

	// Write invalid TOML content.
	if err := os.WriteFile(filepath.Join(dir, "config.toml"), []byte("{{{{not valid toml!"), 0o644); err != nil {
		t.Fatal(err)
	}

	_, err := LoadProjectConfig(root)
	if err == nil {
		t.Fatal("LoadProjectConfig() expected error for malformed TOML, got nil")
	}
}

func TestLoadGlobalConfig_MalformedTOML(t *testing.T) {
	dir := t.TempDir()

	// Write invalid TOML content.
	if err := os.WriteFile(filepath.Join(dir, "global.toml"), []byte("{{{{not valid toml!"), 0o644); err != nil {
		t.Fatal(err)
	}

	_, err := LoadGlobalConfig(dir)
	if err == nil {
		t.Fatal("LoadGlobalConfig() expected error for malformed TOML, got nil")
	}
}

func TestDefaultGlobalDir_ReturnsNonEmpty(t *testing.T) {
	dir := DefaultGlobalDir()
	if dir == "" {
		t.Fatal("DefaultGlobalDir() returned empty string")
	}
	if !filepath.IsAbs(dir) {
		t.Errorf("DefaultGlobalDir() = %q, expected absolute path", dir)
	}
}

func TestResolveGlobalDir_EmptyOverride(t *testing.T) {
	// Empty string override should fall back to default.
	result := resolveGlobalDir([]string{""})
	expected := DefaultGlobalDir()
	if result != expected {
		t.Errorf("resolveGlobalDir([\"\"]) = %q, want %q", result, expected)
	}
}

func TestResolveGlobalDir_NoOverride(t *testing.T) {
	// No overrides at all should fall back to default.
	result := resolveGlobalDir(nil)
	expected := DefaultGlobalDir()
	if result != expected {
		t.Errorf("resolveGlobalDir(nil) = %q, want %q", result, expected)
	}
}

func TestResolveGlobalDir_ValidOverride(t *testing.T) {
	result := resolveGlobalDir([]string{"/custom/path"})
	if result != "/custom/path" {
		t.Errorf("resolveGlobalDir([\"/custom/path\"]) = %q, want /custom/path", result)
	}
}

func TestEnsureGlobalDirs_PartialExists(t *testing.T) {
	base := t.TempDir()
	dir := filepath.Join(base, "partial-synth")

	// Create only the parent dir, not the projects subdir.
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}

	if err := EnsureGlobalDirs(dir); err != nil {
		t.Fatalf("EnsureGlobalDirs() error = %v", err)
	}

	// Verify projects subdir was created.
	projects := filepath.Join(dir, "projects")
	info, err := os.Stat(projects)
	if err != nil {
		t.Fatalf("projects dir not found: %v", err)
	}
	if !info.IsDir() {
		t.Errorf("%q is not a directory", projects)
	}
}

func TestLoadProjectConfig_PartialTOMLPreservesDefaults(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, ".synth")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}

	// Write TOML with only project section — behavior and sync should keep defaults.
	partialTOML := `[project]
id = "partial-proj"
name = "Partial"
`
	if err := os.WriteFile(filepath.Join(dir, "config.toml"), []byte(partialTOML), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg, err := LoadProjectConfig(root)
	if err != nil {
		t.Fatalf("LoadProjectConfig() error = %v", err)
	}

	if cfg.Project.ID != "partial-proj" {
		t.Errorf("Project.ID = %q, want %q", cfg.Project.ID, "partial-proj")
	}
	// Defaults should be preserved for unspecified sections.
	defaults := DefaultProjectConfig()
	if cfg.Behavior.LowContextThreshold != defaults.Behavior.LowContextThreshold {
		t.Errorf("LowContextThreshold = %d, want default %d",
			cfg.Behavior.LowContextThreshold, defaults.Behavior.LowContextThreshold)
	}
	if cfg.Sync.IntervalHours != defaults.Sync.IntervalHours {
		t.Errorf("IntervalHours = %d, want default %d",
			cfg.Sync.IntervalHours, defaults.Sync.IntervalHours)
	}
}

func TestSaveProjectConfig_CreatesNestedDirs(t *testing.T) {
	// Verify SaveProjectConfig creates the .synth dir if it doesn't exist.
	root := t.TempDir()

	cfg := DefaultProjectConfig()
	cfg.Project.ID = "nested-test"

	if err := SaveProjectConfig(root, cfg); err != nil {
		t.Fatalf("SaveProjectConfig() error = %v", err)
	}

	// Verify .synth/config.toml exists.
	path := filepath.Join(root, ".synth", "config.toml")
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("config.toml not created: %v", err)
	}

	// Round-trip verify.
	loaded, err := LoadProjectConfig(root)
	if err != nil {
		t.Fatalf("LoadProjectConfig() error = %v", err)
	}
	if loaded.Project.ID != "nested-test" {
		t.Errorf("Project.ID = %q, want %q", loaded.Project.ID, "nested-test")
	}
}

func TestSaveGlobalConfig_CreatesDir(t *testing.T) {
	base := t.TempDir()
	dir := filepath.Join(base, "new-global-dir")

	cfg := &GlobalConfig{
		User: UserSection{Name: "tester", Email: "tester@test.com"},
	}

	if err := SaveGlobalConfig(cfg, dir); err != nil {
		t.Fatalf("SaveGlobalConfig() error = %v", err)
	}

	// Verify global.toml exists in the newly created dir.
	path := filepath.Join(dir, "global.toml")
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("global.toml not created: %v", err)
	}
}

func TestLoadGlobalConfig_PartialTOMLPreservesDefaults(t *testing.T) {
	dir := t.TempDir()

	// Write TOML with only name, no email.
	partialTOML := `[user]
name = "just-name"
`
	if err := os.WriteFile(filepath.Join(dir, "global.toml"), []byte(partialTOML), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg, err := LoadGlobalConfig(dir)
	if err != nil {
		t.Fatalf("LoadGlobalConfig() error = %v", err)
	}

	if cfg.User.Name != "just-name" {
		t.Errorf("User.Name = %q, want %q", cfg.User.Name, "just-name")
	}
	if cfg.User.Email != "" {
		t.Errorf("User.Email = %q, want empty string", cfg.User.Email)
	}
}

func TestSaveProjectConfig_WritesCorrectDefaultThreshold(t *testing.T) {
	root := t.TempDir()

	cfg := DefaultProjectConfig()
	cfg.Project.ID = "test-init-001"

	if err := SaveProjectConfig(root, cfg); err != nil {
		t.Fatalf("SaveProjectConfig() error = %v", err)
	}

	// Read the written file back manually to check string
	path := filepath.Join(root, ".synth", "config.toml")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("Failed to read config file: %v", err)
	}
	content := string(data)
	if !strings.Contains(content, "low_context_threshold = 5") {
		t.Errorf("expected config file to contain 'low_context_threshold = 5', got:\n%s", content)
	}

	// Parse TOML back
	loaded, err := LoadProjectConfig(root)
	if err != nil {
		t.Fatalf("LoadProjectConfig() error = %v", err)
	}
	if loaded.Behavior.LowContextThreshold != 5 {
		t.Errorf("LowContextThreshold = %d, want 5", loaded.Behavior.LowContextThreshold)
	}
}
