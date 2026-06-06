package cli

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/shyamsundaravssb/synth/internal/store"
)

// setupTestGitRepo creates a temporary directory, initializes a git repo
// inside it, and configures user.email and user.name. Returns the repo path.
func setupTestGitRepo(t *testing.T) string {
	t.Helper()

	dir := t.TempDir()

	run := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("git %v failed: %v\n%s", args, err, out)
		}
	}

	run("init")
	run("config", "user.email", "test@synth.dev")
	run("config", "user.name", "Synth Test")

	return dir
}

func TestRunInit_Success(t *testing.T) {
	repo := setupTestGitRepo(t)

	// Override HOME so global dirs go to a temp location.
	origHome := os.Getenv("HOME")
	tmpHome := t.TempDir()
	os.Setenv("HOME", tmpHome)
	defer os.Setenv("HOME", origHome)

	err := runInit(repo, "test-project", "Test Dev", false)
	if err != nil {
		t.Fatalf("runInit: unexpected error: %v", err)
	}

	// Verify .synth/config.toml exists.
	configPath := filepath.Join(repo, ".synth", "config.toml")
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		t.Fatal("config.toml not created")
	}

	// Verify config contains correct values.
	data, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("reading config: %v", err)
	}
	content := string(data)
	if !strings.Contains(content, "test-project") {
		t.Errorf("config missing project name, got:\n%s", content)
	}
	if !strings.Contains(content, "Test Dev") {
		t.Errorf("config missing developer name, got:\n%s", content)
	}

	// Extract project ID from config to check database.
	projectID := extractProjectID(t, content)
	if projectID == "" {
		t.Fatal("could not extract project ID from config")
	}

	// Verify database file exists.
	dbPath := store.DBPath(projectID)
	if _, err := os.Stat(dbPath); os.IsNotExist(err) {
		t.Fatalf("database not created at %s", dbPath)
	}

	// Verify git hook is installed.
	hookPath := filepath.Join(repo, ".git", "hooks", "post-commit")
	if _, err := os.Stat(hookPath); os.IsNotExist(err) {
		t.Fatal("post-commit hook not installed")
	}
	hookData, _ := os.ReadFile(hookPath)
	if !strings.Contains(string(hookData), "SYNTH HOOK START") {
		t.Error("hook does not contain Synth marker")
	}
}

func TestRunInit_NotAGitRepo(t *testing.T) {
	dir := t.TempDir() // Plain directory, no git init.

	origHome := os.Getenv("HOME")
	tmpHome := t.TempDir()
	os.Setenv("HOME", tmpHome)
	defer os.Setenv("HOME", origHome)

	err := runInit(dir, "test-project", "Test Dev", false)
	if err == nil {
		t.Fatal("runInit: expected error for non-git-repo, got nil")
	}
	if !strings.Contains(err.Error(), "not a git repository") {
		t.Errorf("error = %q, want to contain 'not a git repository'", err.Error())
	}
}

func TestRunInit_AlreadyInitialized(t *testing.T) {
	repo := setupTestGitRepo(t)

	origHome := os.Getenv("HOME")
	tmpHome := t.TempDir()
	os.Setenv("HOME", tmpHome)
	defer os.Setenv("HOME", origHome)

	// First init should succeed.
	if err := runInit(repo, "test-project", "Test Dev", false); err != nil {
		t.Fatalf("first runInit: unexpected error: %v", err)
	}

	// Second init should fail.
	err := runInit(repo, "test-project", "Test Dev", false)
	if err == nil {
		t.Fatal("second runInit: expected error, got nil")
	}
	if !strings.Contains(err.Error(), "already initialized") {
		t.Errorf("error = %q, want to contain 'already initialized'", err.Error())
	}
}

func TestRunInit_CreatesGitignoreEntry(t *testing.T) {
	repo := setupTestGitRepo(t)

	origHome := os.Getenv("HOME")
	tmpHome := t.TempDir()
	os.Setenv("HOME", tmpHome)
	defer os.Setenv("HOME", origHome)

	// No .gitignore exists initially.
	gitignorePath := filepath.Join(repo, ".gitignore")
	if _, err := os.Stat(gitignorePath); err == nil {
		t.Fatal(".gitignore should not exist before init")
	}

	err := runInit(repo, "test-project", "Test Dev", true)
	if err != nil {
		t.Fatalf("runInit: unexpected error: %v", err)
	}

	// Verify .gitignore was created with .synth/ entry.
	data, err := os.ReadFile(gitignorePath)
	if err != nil {
		t.Fatalf("reading .gitignore: %v", err)
	}
	if !strings.Contains(string(data), ".synth/") {
		t.Errorf(".gitignore missing .synth/ entry, got:\n%s", string(data))
	}
}

func TestRunInit_AppendsToExistingGitignore(t *testing.T) {
	repo := setupTestGitRepo(t)

	origHome := os.Getenv("HOME")
	tmpHome := t.TempDir()
	os.Setenv("HOME", tmpHome)
	defer os.Setenv("HOME", origHome)

	// Create .gitignore with existing content.
	gitignorePath := filepath.Join(repo, ".gitignore")
	originalContent := "node_modules/\n*.log\n"
	if err := os.WriteFile(gitignorePath, []byte(originalContent), 0o644); err != nil {
		t.Fatal(err)
	}

	err := runInit(repo, "test-project", "Test Dev", true)
	if err != nil {
		t.Fatalf("runInit: unexpected error: %v", err)
	}

	// Verify original content is preserved and .synth/ is appended.
	data, err := os.ReadFile(gitignorePath)
	if err != nil {
		t.Fatalf("reading .gitignore: %v", err)
	}
	content := string(data)
	if !strings.Contains(content, "node_modules/") {
		t.Error(".gitignore lost original 'node_modules/' entry")
	}
	if !strings.Contains(content, "*.log") {
		t.Error(".gitignore lost original '*.log' entry")
	}
	if !strings.Contains(content, ".synth/") {
		t.Error(".gitignore missing .synth/ entry")
	}
}

func TestRunInit_NoPartialStateOnFailure(t *testing.T) {
	repo := setupTestGitRepo(t)

	// Override HOME to a non-writable location to force database creation
	// to fail. We create a directory structure and make it read-only.
	origHome := os.Getenv("HOME")
	tmpHome := t.TempDir()

	// Create the .synth/projects dir but make it non-writable so
	// store.Open fails when trying to create the project subdir.
	projectsDir := filepath.Join(tmpHome, ".synth", "projects")
	if err := os.MkdirAll(projectsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	// Make projects dir non-writable to force failure on db creation.
	if err := os.Chmod(projectsDir, 0o444); err != nil {
		t.Fatal(err)
	}
	// Restore permissions for cleanup.
	defer os.Chmod(projectsDir, 0o755)

	os.Setenv("HOME", tmpHome)
	defer os.Setenv("HOME", origHome)

	err := runInit(repo, "test-project", "Test Dev", false)
	if err == nil {
		t.Fatal("runInit: expected error due to read-only projects dir, got nil")
	}

	// Verify .synth/ directory was cleaned up.
	synthDir := filepath.Join(repo, ".synth")
	if _, err := os.Stat(synthDir); !os.IsNotExist(err) {
		t.Errorf(".synth/ directory still exists after failed init (should have been cleaned up)")
	}
}

func TestRunInit_SkipsGitignoreWhenFalse(t *testing.T) {
	repo := setupTestGitRepo(t)

	origHome := os.Getenv("HOME")
	tmpHome := t.TempDir()
	os.Setenv("HOME", tmpHome)
	defer os.Setenv("HOME", origHome)

	err := runInit(repo, "test-project", "Test Dev", false)
	if err != nil {
		t.Fatalf("runInit: unexpected error: %v", err)
	}

	// Verify .gitignore was NOT created.
	gitignorePath := filepath.Join(repo, ".gitignore")
	if _, err := os.Stat(gitignorePath); err == nil {
		t.Error(".gitignore should not exist when addToGitignore is false")
	}
}

func TestRunInit_DoesNotDuplicateGitignoreEntry(t *testing.T) {
	repo := setupTestGitRepo(t)

	origHome := os.Getenv("HOME")
	tmpHome := t.TempDir()
	os.Setenv("HOME", tmpHome)
	defer os.Setenv("HOME", origHome)

	// Create .gitignore that already has .synth/.
	gitignorePath := filepath.Join(repo, ".gitignore")
	if err := os.WriteFile(gitignorePath, []byte(".synth/\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	err := runInit(repo, "test-project", "Test Dev", true)
	if err != nil {
		t.Fatalf("runInit: unexpected error: %v", err)
	}

	// Verify .synth/ appears exactly once.
	data, _ := os.ReadFile(gitignorePath)
	count := strings.Count(string(data), ".synth/")
	if count != 1 {
		t.Errorf(".synth/ appears %d times in .gitignore, expected 1", count)
	}
}

// extractProjectID parses a project ID from TOML config content.
func extractProjectID(t *testing.T, content string) string {
	t.Helper()
	for _, line := range strings.Split(content, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "id") {
			parts := strings.SplitN(line, "=", 2)
			if len(parts) == 2 {
				id := strings.TrimSpace(parts[1])
				id = strings.Trim(id, "\"")
				return id
			}
		}
	}
	return ""
}
