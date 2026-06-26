package cli

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/shyamsundaravssb/synth/internal/embed"
)

func TestRunModelStatus_NotInstalled(t *testing.T) {
	// Not installed
	dir := t.TempDir()
	err := runModelStatus(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRunModelStatus_Installed(t *testing.T) {
	dir := t.TempDir()
	for _, f := range embed.ModelFileList() {
		_ = os.WriteFile(filepath.Join(dir, f.Name), []byte("content"), 0o644)
	}

	err := runModelStatus(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRunModelDownload_InstalledNoForce(t *testing.T) {
	dir := t.TempDir()
	for _, f := range embed.ModelFileList() {
		_ = os.WriteFile(filepath.Join(dir, f.Name), []byte("content"), 0o644)
	}

	err := runModelDownload(dir, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}
