package embed

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func TestDownloadFiles_DownloadsAllFiles(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/fileA":
			w.Write([]byte("content A"))
		case "/fileB":
			w.Write([]byte("content B, slightly longer"))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	files := []ModelFile{
		{Name: "fileA", URL: server.URL + "/fileA"},
		{Name: "fileB", URL: server.URL + "/fileB"},
	}

	destDir := t.TempDir()
	ctx := context.Background()
	err := downloadFiles(ctx, files, destDir, false, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	contentA, _ := os.ReadFile(filepath.Join(destDir, "fileA"))
	if string(contentA) != "content A" {
		t.Errorf("expected 'content A', got %q", contentA)
	}

	contentB, _ := os.ReadFile(filepath.Join(destDir, "fileB"))
	if string(contentB) != "content B, slightly longer" {
		t.Errorf("expected 'content B, slightly longer', got %q", contentB)
	}
}

func TestDownloadFiles_SkipsExistingUnlessForce(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("new content"))
	}))
	defer server.Close()

	files := []ModelFile{
		{Name: "fileA", URL: server.URL + "/fileA"},
	}

	destDir := t.TempDir()
	pathA := filepath.Join(destDir, "fileA")
	_ = os.WriteFile(pathA, []byte("old content"), 0o644)

	ctx := context.Background()
	err := downloadFiles(ctx, files, destDir, false, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	contentA, _ := os.ReadFile(pathA)
	if string(contentA) != "old content" {
		t.Errorf("expected 'old content', got %q", contentA)
	}

	err = downloadFiles(ctx, files, destDir, true, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	contentA, _ = os.ReadFile(pathA)
	if string(contentA) != "new content" {
		t.Errorf("expected 'new content', got %q", contentA)
	}
}

func TestDownloadFiles_ServerError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.NotFound(w, r)
	}))
	defer server.Close()

	files := []ModelFile{
		{Name: "fileA", URL: server.URL + "/fileA"},
	}

	destDir := t.TempDir()
	ctx := context.Background()
	err := downloadFiles(ctx, files, destDir, false, nil)
	if err == nil {
		t.Fatalf("expected error, got nil")
	}
	if err.Error() == "" || !containsStr(err.Error(), "fileA") {
		t.Errorf("expected error to mention 'fileA', got: %v", err)
	}
}

func TestDownloadFiles_ProgressCallbackInvoked(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("1234567890"))
	}))
	defer server.Close()

	files := []ModelFile{
		{Name: "fileA", URL: server.URL + "/fileA"},
	}

	destDir := t.TempDir()
	ctx := context.Background()

	var lastDownloaded int64
	var calls int
	progress := func(fileName string, downloaded, total int64) {
		calls++
		lastDownloaded = downloaded
	}

	err := downloadFiles(ctx, files, destDir, false, progress)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if calls == 0 {
		t.Errorf("expected progress to be called")
	}
	if lastDownloaded != 10 {
		t.Errorf("expected lastDownloaded 10, got %d", lastDownloaded)
	}
}

func TestIsModelComplete_AllFilesPresent(t *testing.T) {
	destDir := t.TempDir()
	for _, file := range ModelFileList() {
		_ = os.WriteFile(filepath.Join(destDir, file.Name), []byte("content"), 0o644)
	}
	if !IsModelComplete(destDir) {
		t.Errorf("expected true")
	}
}

func TestIsModelComplete_MissingFile(t *testing.T) {
	destDir := t.TempDir()
	files := ModelFileList()
	for i, file := range files {
		if i == 0 {
			continue
		}
		_ = os.WriteFile(filepath.Join(destDir, file.Name), []byte("content"), 0o644)
	}
	if IsModelComplete(destDir) {
		t.Errorf("expected false")
	}
}

func TestIsModelComplete_EmptyFile(t *testing.T) {
	destDir := t.TempDir()
	files := ModelFileList()
	for i, file := range files {
		content := []byte("content")
		if i == 0 {
			content = []byte("")
		}
		_ = os.WriteFile(filepath.Join(destDir, file.Name), content, 0o644)
	}
	if IsModelComplete(destDir) {
		t.Errorf("expected false")
	}
}

func TestIsModelComplete_EmptyDir(t *testing.T) {
	destDir := t.TempDir()
	if IsModelComplete(destDir) {
		t.Errorf("expected false")
	}
}

func containsStr(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
