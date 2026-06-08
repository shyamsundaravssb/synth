package daemon

import (
	"bufio"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

func setupLogger(t *testing.T) (*Logger, string) {
	logFile := filepath.Join(t.TempDir(), "daemon.log")
	return NewLogger(logFile), logFile
}

func readLogLines(t *testing.T, path string) []map[string]interface{} {
	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		t.Fatalf("failed to open log file: %v", err)
	}
	defer f.Close()

	var lines []map[string]interface{}
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		var entry map[string]interface{}
		if err := json.Unmarshal(scanner.Bytes(), &entry); err != nil {
			t.Fatalf("invalid JSON in log file: %v\nLine: %s", err, scanner.Text())
		}
		lines = append(lines, entry)
	}
	if err := scanner.Err(); err != nil {
		t.Fatalf("error reading log file: %v", err)
	}
	return lines
}

func TestLogger_InfoWritesLine(t *testing.T) {
	l, path := setupLogger(t)
	l.Info("test message")

	lines := readLogLines(t, path)
	if len(lines) != 1 {
		t.Fatalf("expected 1 line, got %d", len(lines))
	}

	entry := lines[0]
	if entry["level"] != "info" {
		t.Fatalf("expected level info, got %v", entry["level"])
	}
	if entry["msg"] != "test message" {
		t.Fatalf("expected msg 'test message', got %v", entry["msg"])
	}
	pid, ok := entry["pid"].(float64)
	if !ok || pid <= 0 {
		t.Fatalf("expected positive numeric pid, got %v", entry["pid"])
	}
	ts, ok := entry["ts"].(string)
	if !ok || ts == "" {
		t.Fatalf("expected non-empty ts string, got %v", entry["ts"])
	}
}

func TestLogger_WarnWritesCorrectLevel(t *testing.T) {
	l, path := setupLogger(t)
	l.Warn("test warning")

	lines := readLogLines(t, path)
	if len(lines) != 1 {
		t.Fatalf("expected 1 line, got %d", len(lines))
	}
	if lines[0]["level"] != "warn" {
		t.Fatalf("expected level warn, got %v", lines[0]["level"])
	}
}

func TestLogger_ErrorWritesErrorField(t *testing.T) {
	l, path := setupLogger(t)
	l.Error("something failed", "file not found")

	lines := readLogLines(t, path)
	if len(lines) != 1 {
		t.Fatalf("expected 1 line, got %d", len(lines))
	}
	if lines[0]["level"] != "error" {
		t.Fatalf("expected level error, got %v", lines[0]["level"])
	}
	if lines[0]["error"] != "file not found" {
		t.Fatalf("expected error 'file not found', got %v", lines[0]["error"])
	}
}

func TestLogger_InfoFileWritesFileField(t *testing.T) {
	l, path := setupLogger(t)
	l.InfoFile("file saved", "src/auth.go")

	lines := readLogLines(t, path)
	if len(lines) != 1 {
		t.Fatalf("expected 1 line, got %d", len(lines))
	}
	if lines[0]["file"] != "src/auth.go" {
		t.Fatalf("expected file 'src/auth.go', got %v", lines[0]["file"])
	}
}

func TestLogger_MultipleEntries(t *testing.T) {
	l, path := setupLogger(t)
	l.Info("info msg")
	l.Warn("warn msg")
	l.Error("error msg", "err str")

	lines := readLogLines(t, path)
	if len(lines) != 3 {
		t.Fatalf("expected 3 lines, got %d", len(lines))
	}
	if lines[0]["msg"] != "info msg" {
		t.Fatalf("expected first line msg 'info msg', got %v", lines[0]["msg"])
	}
	if lines[1]["msg"] != "warn msg" {
		t.Fatalf("expected second line msg 'warn msg', got %v", lines[1]["msg"])
	}
	if lines[2]["msg"] != "error msg" {
		t.Fatalf("expected third line msg 'error msg', got %v", lines[2]["msg"])
	}
}

func TestLogger_CreatesParentDirs(t *testing.T) {
	nestedPath := filepath.Join(t.TempDir(), "a", "b", "daemon.log")
	l := NewLogger(nestedPath)
	l.Info("test")

	if _, err := os.Stat(nestedPath); err != nil {
		t.Fatalf("expected file to exist, got %v", err)
	}
}

func TestLogger_AppendsNotOverwrites(t *testing.T) {
	l, path := setupLogger(t)
	l.Info("first")
	l.Info("second")

	lines := readLogLines(t, path)
	if len(lines) != 2 {
		t.Fatalf("expected 2 lines, got %d", len(lines))
	}
	if lines[0]["msg"] != "first" || lines[1]["msg"] != "second" {
		t.Fatalf("expected lines in order, got %v", lines)
	}
}

func TestLogger_RotationOnSizeThreshold(t *testing.T) {
	l, path := setupLogger(t)
	l.maxBytes = 50

	l.Info("message one that is quite long to exceed fifty bytes easily")
	l.Info("message two")

	if _, err := os.Stat(path); err != nil {
		t.Fatalf("expected new log file to exist, got %v", err)
	}

	backupPath := path + ".1"
	if _, err := os.Stat(backupPath); err != nil {
		t.Fatalf("expected backup file to exist, got %v", err)
	}

	backupLines := readLogLines(t, backupPath)
	if len(backupLines) != 1 {
		t.Fatalf("expected 1 line in backup, got %d", len(backupLines))
	}
	if backupLines[0]["msg"] != "message one that is quite long to exceed fifty bytes easily" {
		t.Fatalf("expected first message in backup file, got %v", backupLines[0]["msg"])
	}
}

func TestLogger_RotationKeepsOnlyOneBackup(t *testing.T) {
	l, path := setupLogger(t)
	l.maxBytes = 50

	l.Info("message one that is quite long to exceed fifty bytes easily")
	l.Info("message two that is also long enough to exceed the threshold")
	l.Info("message three")

	backupPath := path + ".1"
	if _, err := os.Stat(backupPath); err != nil {
		t.Fatalf("expected backup .1 to exist, got %v", err)
	}

	if _, err := os.Stat(path + ".2"); !os.IsNotExist(err) {
		t.Fatalf("expected backup .2 to NOT exist")
	}

	backupLines := readLogLines(t, backupPath)
	if len(backupLines) != 1 {
		t.Fatalf("expected 1 line in backup, got %d", len(backupLines))
	}
	// Note: in go, map keys might have type interface{} internally representing float64 for numbers.
	// But the msg is string.
	msg, ok := backupLines[0]["msg"].(string)
	if !ok || !strings.Contains(msg, "message two") {
		t.Fatalf("expected second message in backup file, got %v", backupLines[0]["msg"])
	}
}

func TestLogger_ConcurrentWrites(t *testing.T) {
	l, path := setupLogger(t)

	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 10; j++ {
				l.Info("concurrent test message")
			}
		}()
	}
	wg.Wait()

	lines := readLogLines(t, path)
	if len(lines) != 100 {
		t.Fatalf("expected 100 lines, got %d", len(lines))
	}
}

func TestShouldRotate_FalseWhenFileSmall(t *testing.T) {
	l, _ := setupLogger(t)
	l.Info("tiny")
	if l.shouldRotate() {
		t.Fatalf("expected shouldRotate false for small file")
	}
}

func TestShouldRotate_TrueWhenFileExceedsMax(t *testing.T) {
	l, _ := setupLogger(t)
	l.maxBytes = 10
	l.Info("exceeds ten bytes easily")
	if !l.shouldRotate() {
		t.Fatalf("expected shouldRotate true")
	}
}

func TestShouldRotate_FalseWhenFileAbsent(t *testing.T) {
	l, _ := setupLogger(t)
	if l.shouldRotate() {
		t.Fatalf("expected shouldRotate false for non-existent file")
	}
}
