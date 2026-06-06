package git

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestInstallHook_NoExistingHook(t *testing.T) {
	repo := setupTestGitRepo(t)

	if err := InstallPostCommitHook(repo); err != nil {
		t.Fatalf("InstallPostCommitHook: %v", err)
	}

	hookPath := filepath.Join(repo, ".git", "hooks", "post-commit")

	// Verify file exists.
	info, err := os.Stat(hookPath)
	if err != nil {
		t.Fatalf("hook file does not exist: %v", err)
	}

	// Verify is executable.
	if info.Mode()&0o111 == 0 {
		t.Error("hook file is not executable")
	}

	// Verify contains marker.
	data, err := os.ReadFile(hookPath)
	if err != nil {
		t.Fatal(err)
	}
	content := string(data)
	if !strings.Contains(content, SynthHookMarkerStart) {
		t.Error("hook does not contain start marker")
	}
	if !strings.Contains(content, SynthHookMarkerEnd) {
		t.Error("hook does not contain end marker")
	}
	if !strings.HasPrefix(content, "#!/bin/sh\n") {
		t.Error("hook does not start with shebang")
	}
}

func TestInstallHook_ExistingNonSynthHook(t *testing.T) {
	repo := setupTestGitRepo(t)

	hookDir := filepath.Join(repo, ".git", "hooks")
	if err := os.MkdirAll(hookDir, 0o755); err != nil {
		t.Fatal(err)
	}

	// Write a custom hook first.
	hookPath := filepath.Join(hookDir, "post-commit")
	customContent := "#!/bin/sh\necho 'custom hook'\n"
	if err := os.WriteFile(hookPath, []byte(customContent), 0o755); err != nil {
		t.Fatal(err)
	}

	if err := InstallPostCommitHook(repo); err != nil {
		t.Fatalf("InstallPostCommitHook: %v", err)
	}

	data, err := os.ReadFile(hookPath)
	if err != nil {
		t.Fatal(err)
	}
	content := string(data)

	// Verify original content is preserved.
	if !strings.Contains(content, "echo 'custom hook'") {
		t.Error("original hook content not preserved")
	}

	// Verify Synth block is appended.
	if !strings.Contains(content, SynthHookMarkerStart) {
		t.Error("Synth block not appended")
	}

	// Verify custom content comes before Synth block.
	customIdx := strings.Index(content, "echo 'custom hook'")
	synthIdx := strings.Index(content, SynthHookMarkerStart)
	if customIdx > synthIdx {
		t.Error("Synth block should come after original content")
	}
}

func TestInstallHook_AlreadyInstalled(t *testing.T) {
	repo := setupTestGitRepo(t)

	// Install hook twice.
	if err := InstallPostCommitHook(repo); err != nil {
		t.Fatalf("first install: %v", err)
	}
	if err := InstallPostCommitHook(repo); err != nil {
		t.Fatalf("second install: %v", err)
	}

	hookPath := filepath.Join(repo, ".git", "hooks", "post-commit")
	data, err := os.ReadFile(hookPath)
	if err != nil {
		t.Fatal(err)
	}
	content := string(data)

	// Verify marker appears exactly once (not duplicated).
	count := strings.Count(content, SynthHookMarkerStart)
	if count != 1 {
		t.Errorf("SynthHookMarkerStart appears %d times, expected 1", count)
	}
}

func TestIsHookInstalled_True(t *testing.T) {
	repo := setupTestGitRepo(t)

	if err := InstallPostCommitHook(repo); err != nil {
		t.Fatal(err)
	}

	if !IsHookInstalled(repo) {
		t.Error("IsHookInstalled: expected true after install")
	}
}

func TestIsHookInstalled_False(t *testing.T) {
	repo := setupTestGitRepo(t)

	if IsHookInstalled(repo) {
		t.Error("IsHookInstalled: expected false in fresh repo")
	}
}

func TestUninstallHook_OnlySynthContent(t *testing.T) {
	repo := setupTestGitRepo(t)

	// Install then uninstall.
	if err := InstallPostCommitHook(repo); err != nil {
		t.Fatal(err)
	}
	if err := UninstallHook(repo); err != nil {
		t.Fatalf("UninstallHook: %v", err)
	}

	hookPath := filepath.Join(repo, ".git", "hooks", "post-commit")
	if _, err := os.Stat(hookPath); !os.IsNotExist(err) {
		t.Error("hook file should be deleted when only Synth content was present")
	}
}

func TestUninstallHook_MixedContent(t *testing.T) {
	repo := setupTestGitRepo(t)

	// Write a custom hook first.
	hookDir := filepath.Join(repo, ".git", "hooks")
	if err := os.MkdirAll(hookDir, 0o755); err != nil {
		t.Fatal(err)
	}
	hookPath := filepath.Join(hookDir, "post-commit")
	customContent := "#!/bin/sh\necho 'my custom hook'\n"
	if err := os.WriteFile(hookPath, []byte(customContent), 0o755); err != nil {
		t.Fatal(err)
	}

	// Install Synth hook on top.
	if err := InstallPostCommitHook(repo); err != nil {
		t.Fatal(err)
	}

	// Verify both exist.
	data, _ := os.ReadFile(hookPath)
	if !strings.Contains(string(data), SynthHookMarkerStart) {
		t.Fatal("Synth block not found before uninstall")
	}

	// Uninstall.
	if err := UninstallHook(repo); err != nil {
		t.Fatalf("UninstallHook: %v", err)
	}

	// Verify file still exists.
	data, err := os.ReadFile(hookPath)
	if err != nil {
		t.Fatal("hook file should still exist with custom content")
	}
	content := string(data)

	// Verify Synth block removed.
	if strings.Contains(content, SynthHookMarkerStart) {
		t.Error("Synth block should be removed after uninstall")
	}

	// Verify original content preserved.
	if !strings.Contains(content, "echo 'my custom hook'") {
		t.Error("original hook content should be preserved after uninstall")
	}
}

func TestUninstallHook_NotInstalled(t *testing.T) {
	repo := setupTestGitRepo(t)

	// Uninstalling when no hook exists should not error.
	if err := UninstallHook(repo); err != nil {
		t.Fatalf("UninstallHook (not installed): %v", err)
	}
}
