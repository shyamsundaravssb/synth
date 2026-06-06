package git

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

// setupTestGitRepo creates a temporary directory, initializes a git repo
// inside it, and configures user.email and user.name (required for commits
// in CI environments). Returns the repo path.
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

func TestFindGitRoot_FromRoot(t *testing.T) {
	repo := setupTestGitRepo(t)

	root, err := FindGitRoot(repo)
	if err != nil {
		t.Fatalf("FindGitRoot from root: unexpected error: %v", err)
	}

	// Resolve symlinks to handle macOS /tmp -> /private/tmp etc.
	expected, _ := filepath.EvalSymlinks(repo)
	got, _ := filepath.EvalSymlinks(root)
	if got != expected {
		t.Errorf("FindGitRoot = %q, want %q", got, expected)
	}
}

func TestFindGitRoot_FromSubdirectory(t *testing.T) {
	repo := setupTestGitRepo(t)

	subdir := filepath.Join(repo, "deep", "nested", "dir")
	if err := os.MkdirAll(subdir, 0o755); err != nil {
		t.Fatal(err)
	}

	root, err := FindGitRoot(subdir)
	if err != nil {
		t.Fatalf("FindGitRoot from subdir: unexpected error: %v", err)
	}

	expected, _ := filepath.EvalSymlinks(repo)
	got, _ := filepath.EvalSymlinks(root)
	if got != expected {
		t.Errorf("FindGitRoot = %q, want %q", got, expected)
	}
}

func TestFindGitRoot_NotARepo(t *testing.T) {
	dir := t.TempDir()

	_, err := FindGitRoot(dir)
	if err == nil {
		t.Fatal("FindGitRoot: expected error for non-repo, got nil")
	}
	if !contains(err.Error(), "not a git repository") {
		t.Errorf("error = %q, want to contain 'not a git repository'", err.Error())
	}
}

func TestIsGitRepo_True(t *testing.T) {
	repo := setupTestGitRepo(t)

	if !IsGitRepo(repo) {
		t.Error("IsGitRepo: expected true for git repo")
	}
}

func TestIsGitRepo_False(t *testing.T) {
	dir := t.TempDir()

	if IsGitRepo(dir) {
		t.Error("IsGitRepo: expected false for non-repo")
	}
}

func TestGetCurrentBranch(t *testing.T) {
	repo := setupTestGitRepo(t)

	// Create a file and commit so HEAD exists.
	testFile := filepath.Join(repo, "test.txt")
	if err := os.WriteFile(testFile, []byte("hello"), 0o644); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command("git", "-C", repo, "add", ".")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git add: %v\n%s", err, out)
	}
	cmd = exec.Command("git", "-C", repo, "commit", "-m", "initial")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git commit: %v\n%s", err, out)
	}

	branch, err := GetCurrentBranch(repo)
	if err != nil {
		t.Fatalf("GetCurrentBranch: unexpected error: %v", err)
	}
	// The default branch could be "main" or "master" depending on git config.
	if branch == "" || branch == "unknown" {
		t.Errorf("GetCurrentBranch = %q, expected a valid branch name", branch)
	}
}

func TestGetCurrentBranch_NoCommits(t *testing.T) {
	repo := setupTestGitRepo(t)

	// Fresh repo with no commits — should not error.
	branch, err := GetCurrentBranch(repo)
	if err != nil {
		t.Fatalf("GetCurrentBranch (no commits): unexpected error: %v", err)
	}
	// With no commits, git rev-parse --abbrev-ref HEAD may return "HEAD"
	// or fail gracefully with "unknown".
	if branch == "" {
		t.Error("GetCurrentBranch (no commits): expected non-empty result")
	}
}

func TestGetCurrentCommit(t *testing.T) {
	repo := setupTestGitRepo(t)

	// Create a commit.
	testFile := filepath.Join(repo, "test.txt")
	if err := os.WriteFile(testFile, []byte("hello"), 0o644); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command("git", "-C", repo, "add", ".")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git add: %v\n%s", err, out)
	}
	cmd = exec.Command("git", "-C", repo, "commit", "-m", "initial")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git commit: %v\n%s", err, out)
	}

	commit, err := GetCurrentCommit(repo)
	if err != nil {
		t.Fatalf("GetCurrentCommit: unexpected error: %v", err)
	}
	if len(commit) < 7 {
		t.Errorf("GetCurrentCommit = %q, expected a commit hash", commit)
	}
}

func TestGetCurrentCommit_NoCommits(t *testing.T) {
	repo := setupTestGitRepo(t)

	commit, err := GetCurrentCommit(repo)
	if err != nil {
		t.Fatalf("GetCurrentCommit (no commits): unexpected error: %v", err)
	}
	if commit != "" {
		t.Errorf("GetCurrentCommit (no commits) = %q, expected empty string", commit)
	}
}

func TestGetRecentlyModifiedFiles(t *testing.T) {
	repo := setupTestGitRepo(t)

	// Create and modify files.
	files := []string{"a.go", "b.txt", "sub/c.go"}
	for _, f := range files {
		full := filepath.Join(repo, f)
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte("content"), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	result, err := GetRecentlyModifiedFiles(repo, 1*time.Hour)
	if err != nil {
		t.Fatalf("GetRecentlyModifiedFiles: unexpected error: %v", err)
	}

	// All files should appear in results.
	for _, f := range files {
		found := false
		for _, r := range result {
			if filepath.ToSlash(r) == f {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("expected %q in results, got %v", f, result)
		}
	}
}

func TestGetRecentlyModifiedFiles_ExcludesDotGit(t *testing.T) {
	repo := setupTestGitRepo(t)

	// Create a regular file too.
	testFile := filepath.Join(repo, "test.go")
	if err := os.WriteFile(testFile, []byte("package main"), 0o644); err != nil {
		t.Fatal(err)
	}

	result, err := GetRecentlyModifiedFiles(repo, 1*time.Hour)
	if err != nil {
		t.Fatalf("GetRecentlyModifiedFiles: unexpected error: %v", err)
	}

	for _, r := range result {
		normalized := filepath.ToSlash(r)
		if contains(normalized, ".git") {
			t.Errorf("result contains .git path: %q", r)
		}
	}
}

func TestGetRecentlyModifiedFiles_ExcludesBinaries(t *testing.T) {
	repo := setupTestGitRepo(t)

	// Create binary and non-binary files.
	if err := os.WriteFile(filepath.Join(repo, "app.go"), []byte("package main"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repo, "image.png"), []byte("fake png"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repo, "lib.so"), []byte("fake so"), 0o644); err != nil {
		t.Fatal(err)
	}

	result, err := GetRecentlyModifiedFiles(repo, 1*time.Hour)
	if err != nil {
		t.Fatalf("GetRecentlyModifiedFiles: unexpected error: %v", err)
	}

	for _, r := range result {
		ext := filepath.Ext(r)
		if binaryExtensions[ext] {
			t.Errorf("result contains binary file: %q", r)
		}
	}
}

func TestGetRecentlyModifiedFiles_ExcludesSynthDir(t *testing.T) {
	repo := setupTestGitRepo(t)

	// Create a .synth directory with a file.
	synthDir := filepath.Join(repo, ".synth")
	if err := os.MkdirAll(synthDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(synthDir, "config.toml"), []byte(""), 0o644); err != nil {
		t.Fatal(err)
	}
	// Also a regular file.
	if err := os.WriteFile(filepath.Join(repo, "main.go"), []byte("package main"), 0o644); err != nil {
		t.Fatal(err)
	}

	result, err := GetRecentlyModifiedFiles(repo, 1*time.Hour)
	if err != nil {
		t.Fatalf("GetRecentlyModifiedFiles: unexpected error: %v", err)
	}

	for _, r := range result {
		normalized := filepath.ToSlash(r)
		if contains(normalized, ".synth") {
			t.Errorf("result contains .synth path: %q", r)
		}
	}
}

func TestGetFileDiff(t *testing.T) {
	repo := setupTestGitRepo(t)

	// Create a file, commit, then modify it.
	testFile := filepath.Join(repo, "test.txt")
	if err := os.WriteFile(testFile, []byte("original"), 0o644); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command("git", "-C", repo, "add", ".")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git add: %v\n%s", err, out)
	}
	cmd = exec.Command("git", "-C", repo, "commit", "-m", "initial")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git commit: %v\n%s", err, out)
	}

	// Modify the file.
	if err := os.WriteFile(testFile, []byte("modified"), 0o644); err != nil {
		t.Fatal(err)
	}

	diff, err := GetFileDiff(repo, "test.txt")
	if err != nil {
		t.Fatalf("GetFileDiff: unexpected error: %v", err)
	}
	if diff == "" {
		t.Error("GetFileDiff: expected non-empty diff")
	}
	if !contains(diff, "original") || !contains(diff, "modified") {
		t.Errorf("GetFileDiff: diff does not contain expected content: %s", diff)
	}
}

func TestGetFileDiff_NoChanges(t *testing.T) {
	repo := setupTestGitRepo(t)

	// Create a file and commit — no changes after commit.
	testFile := filepath.Join(repo, "test.txt")
	if err := os.WriteFile(testFile, []byte("content"), 0o644); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command("git", "-C", repo, "add", ".")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git add: %v\n%s", err, out)
	}
	cmd = exec.Command("git", "-C", repo, "commit", "-m", "initial")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git commit: %v\n%s", err, out)
	}

	diff, err := GetFileDiff(repo, "test.txt")
	if err != nil {
		t.Fatalf("GetFileDiff: unexpected error: %v", err)
	}
	if diff != "" {
		t.Errorf("GetFileDiff (no changes) = %q, expected empty string", diff)
	}
}

func TestGetFileDiff_UntrackedFile(t *testing.T) {
	repo := setupTestGitRepo(t)

	// Create an untracked file.
	if err := os.WriteFile(filepath.Join(repo, "untracked.txt"), []byte("new"), 0o644); err != nil {
		t.Fatal(err)
	}

	diff, err := GetFileDiff(repo, "untracked.txt")
	if err != nil {
		t.Fatalf("GetFileDiff (untracked): unexpected error: %v", err)
	}
	// Untracked files have no diff against HEAD.
	if diff != "" {
		t.Errorf("GetFileDiff (untracked) = %q, expected empty string", diff)
	}
}

// contains is a test helper to check if s contains substr.
func contains(s, substr string) bool {
	return len(s) >= len(substr) && containsStr(s, substr)
}

func containsStr(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
