package daemon

import (
	"context"
	"database/sql"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/shyamsundaravssb/synth/internal/store"
)

type noopLogger struct{}

func (n *noopLogger) Info(msg string)           {}
func (n *noopLogger) Warn(msg string)           {}
func (n *noopLogger) Error(msg, e string)       {}
func (n *noopLogger) InfoFile(msg, f string)    {}

func setupWatcher(t *testing.T) (*Watcher, string, *sql.DB, chan struct{}) {
	gitRoot := t.TempDir()

	cmd := exec.Command("git", "init")
	cmd.Dir = gitRoot
	if err := cmd.Run(); err != nil {
		t.Fatalf("failed to run git init: %v", err)
	}

	dbPath := filepath.Join(gitRoot, "intent.db")
	db, err := store.Open(dbPath)
	if err != nil {
		t.Fatalf("failed to open store: %v", err)
	}
	s := store.NewSQLiteStore(db)

	shutdownCh := make(chan struct{})
	projectID := "test_project"

	watcher, err := NewWatcher(gitRoot, projectID, s, &noopLogger{}, shutdownCh)
	if err != nil {
		t.Fatalf("failed to create watcher: %v", err)
	}

	return watcher, gitRoot, db, shutdownCh
}

func TestWatcher_DetectsFileWrite(t *testing.T) {
	watcher, gitRoot, db, shutdownCh := setupWatcher(t)
	err := watcher.Start()
	if err != nil {
		t.Fatalf("failed to start watcher: %v", err)
	}

	defer func() {
		close(shutdownCh)
		time.Sleep(50 * time.Millisecond)
	}()

	testFile := filepath.Join(gitRoot, "main.go")
	if err := os.WriteFile(testFile, []byte("package main"), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	deadline := time.Now().Add(2 * time.Second)
	ctx := context.Background()

	found := false
	for time.Now().Before(deadline) {
		type fileSave struct {
			FilePath string
			HasNote  int
		}

		rows, err := db.QueryContext(ctx, "SELECT file_path, has_note FROM file_saves WHERE project_id = 'test_project'")
		if err == nil {
			for rows.Next() {
				var fs fileSave
				if err := rows.Scan(&fs.FilePath, &fs.HasNote); err == nil {
					if fs.FilePath == "main.go" {
						if fs.HasNote != 0 {
							t.Errorf("expected has_note to be 0, got %d", fs.HasNote)
						}
						found = true
						break
					}
				}
			}
			rows.Close()
		}

		if found {
			break
		}
		time.Sleep(100 * time.Millisecond)
	}

	if !found {
		t.Errorf("expected to find file_saves record for main.go")
	}
}

func TestWatcher_IgnoresDotGit(t *testing.T) {
	watcher, gitRoot, db, shutdownCh := setupWatcher(t)
	_ = watcher.Start()
	defer func() {
		close(shutdownCh)
		time.Sleep(50 * time.Millisecond)
	}()

	gitDir := filepath.Join(gitRoot, ".git")
	_ = os.MkdirAll(gitDir, 0755)
	testFile := filepath.Join(gitDir, "test.txt")
	_ = os.WriteFile(testFile, []byte("ignore me"), 0644)

	time.Sleep(500 * time.Millisecond)

	ctx := context.Background()
	var count int
	err := db.QueryRowContext(ctx, "SELECT COUNT(*) FROM file_saves WHERE project_id = 'test_project'").Scan(&count)
	if err != nil {
		t.Fatalf("query failed: %v", err)
	}

	if count != 0 {
		t.Errorf("expected 0 saves for .git changes, got %d", count)
	}
}

func TestWatcher_IgnoresBinaryFiles(t *testing.T) {
	watcher, gitRoot, db, shutdownCh := setupWatcher(t)
	_ = watcher.Start()
	defer func() {
		close(shutdownCh)
		time.Sleep(50 * time.Millisecond)
	}()

	testFile := filepath.Join(gitRoot, "image.png")
	_ = os.WriteFile(testFile, []byte("fake image data"), 0644)

	time.Sleep(500 * time.Millisecond)

	ctx := context.Background()
	var count int
	_ = db.QueryRowContext(ctx, "SELECT COUNT(*) FROM file_saves WHERE project_id = 'test_project'").Scan(&count)

	if count != 0 {
		t.Errorf("expected 0 saves for binary file, got %d", count)
	}
}

func TestWatcher_IgnoresDotFiles(t *testing.T) {
	watcher, gitRoot, db, shutdownCh := setupWatcher(t)
	_ = watcher.Start()
	defer func() {
		close(shutdownCh)
		time.Sleep(50 * time.Millisecond)
	}()

	testFile := filepath.Join(gitRoot, ".hidden_file")
	_ = os.WriteFile(testFile, []byte("hidden"), 0644)

	time.Sleep(500 * time.Millisecond)

	ctx := context.Background()
	var count int
	_ = db.QueryRowContext(ctx, "SELECT COUNT(*) FROM file_saves WHERE project_id = 'test_project'").Scan(&count)

	if count != 0 {
		t.Errorf("expected 0 saves for dotfile, got %d", count)
	}
}

func TestWatcher_DebouncesRapidWrites(t *testing.T) {
	watcher, gitRoot, db, shutdownCh := setupWatcher(t)
	_ = watcher.Start()
	defer func() {
		close(shutdownCh)
		time.Sleep(50 * time.Millisecond)
	}()

	testFile := filepath.Join(gitRoot, "rapid.go")
	for i := 0; i < 5; i++ {
		_ = os.WriteFile(testFile, []byte("data"), 0644)
		time.Sleep(20 * time.Millisecond)
	}

	time.Sleep(1 * time.Second)

	ctx := context.Background()
	var count int
	_ = db.QueryRowContext(ctx, "SELECT COUNT(*) FROM file_saves WHERE project_id = 'test_project' AND file_path = 'rapid.go'").Scan(&count)

	if count != 1 && count != 2 {
		t.Errorf("expected 1 or 2 debounce events, got %d", count)
	}
}

func TestShouldTrack_Filters(t *testing.T) {
	watcher := &Watcher{gitRoot: "/tmp/repo"}

	falseCases := []string{
		"/tmp/repo/.git/config",
		"/tmp/repo/.synth/config.toml",
		"/tmp/repo/image.png",
		"/tmp/repo/.hidden",
		"/other/repo/file.go",
	}

	for _, c := range falseCases {
		if watcher.shouldTrack(c) {
			t.Errorf("expected false for %s", c)
		}
	}

	trueCases := []string{
		"/tmp/repo/main.go",
		"/tmp/repo/src/auth.go",
		"/tmp/repo/README.md",
	}

	for _, c := range trueCases {
		if !watcher.shouldTrack(c) {
			t.Errorf("expected true for %s", c)
		}
	}
}
