package daemon

import (
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"testing"
)

func TestWritePID_CreatesFile(t *testing.T) {
	pidFile := filepath.Join(t.TempDir(), "daemon.pid")
	err := WritePID(pidFile)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	data, err := os.ReadFile(pidFile)
	if err != nil {
		t.Fatalf("expected file to exist, got %v", err)
	}
	if string(data) != strconv.Itoa(os.Getpid()) {
		t.Fatalf("expected PID %d, got %s", os.Getpid(), string(data))
	}
}

func TestWritePID_CreatesParentDirs(t *testing.T) {
	pidFile := filepath.Join(t.TempDir(), "a", "b", "daemon.pid")
	err := WritePID(pidFile)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	_, err = os.Stat(pidFile)
	if err != nil {
		t.Fatalf("expected file to exist, got %v", err)
	}
}

func TestWritePID_OverwritesExisting(t *testing.T) {
	pidFile := filepath.Join(t.TempDir(), "daemon.pid")
	err := os.WriteFile(pidFile, []byte("123"), 0644)
	if err != nil {
		t.Fatalf("setup failed: %v", err)
	}
	err = WritePID(pidFile)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	data, err := os.ReadFile(pidFile)
	if err != nil {
		t.Fatalf("read failed: %v", err)
	}
	if string(data) != strconv.Itoa(os.Getpid()) {
		t.Fatalf("expected %d, got %s", os.Getpid(), string(data))
	}
}

func TestReadPID_ReadsCorrectly(t *testing.T) {
	pidFile := filepath.Join(t.TempDir(), "daemon.pid")
	expectedPID := 12345
	err := os.WriteFile(pidFile, []byte(strconv.Itoa(expectedPID)), 0644)
	if err != nil {
		t.Fatalf("setup failed: %v", err)
	}
	
	pid, err := ReadPID(pidFile)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if pid != expectedPID {
		t.Fatalf("expected %d, got %d", expectedPID, pid)
	}
}

func TestReadPID_FileNotFound(t *testing.T) {
	pidFile := filepath.Join(t.TempDir(), "daemon.pid")
	_, err := ReadPID(pidFile)
	if !errors.Is(err, ErrDaemonNotRunning) {
		t.Fatalf("expected ErrDaemonNotRunning, got %v", err)
	}
}

func TestReadPID_InvalidContent(t *testing.T) {
	pidFile := filepath.Join(t.TempDir(), "daemon.pid")
	err := os.WriteFile(pidFile, []byte("notanumber"), 0644)
	if err != nil {
		t.Fatalf("setup failed: %v", err)
	}
	_, err = ReadPID(pidFile)
	if err == nil || errors.Is(err, ErrDaemonNotRunning) {
		t.Fatalf("expected parsing error, got %v", err)
	}
}

func TestRemovePID_DeletesFile(t *testing.T) {
	pidFile := filepath.Join(t.TempDir(), "daemon.pid")
	err := os.WriteFile(pidFile, []byte("123"), 0644)
	if err != nil {
		t.Fatalf("setup failed: %v", err)
	}
	err = RemovePID(pidFile)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	_, err = os.Stat(pidFile)
	if !os.IsNotExist(err) {
		t.Fatalf("expected file to not exist, got %v", err)
	}
}

func TestRemovePID_NonExistentFile(t *testing.T) {
	pidFile := filepath.Join(t.TempDir(), "daemon.pid")
	err := RemovePID(pidFile)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
}

func TestIsProcessRunning_CurrentProcess(t *testing.T) {
	running := IsProcessRunning(os.Getpid())
	if !running {
		t.Fatal("expected current process to be running")
	}
}

func TestIsProcessRunning_InvalidPID(t *testing.T) {
	running := IsProcessRunning(99999999)
	if running {
		t.Fatal("expected invalid PID to not be running")
	}
}

func TestIsDaemonRunning_NoPIDFile(t *testing.T) {
	pidFile := filepath.Join(t.TempDir(), "daemon.pid")
	running, pid, err := IsDaemonRunning(pidFile)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if running {
		t.Fatal("expected not running")
	}
	if pid != 0 {
		t.Fatalf("expected pid 0, got %d", pid)
	}
}

func TestIsDaemonRunning_StalePIDFile(t *testing.T) {
	pidFile := filepath.Join(t.TempDir(), "daemon.pid")
	err := os.WriteFile(pidFile, []byte("99999999"), 0644)
	if err != nil {
		t.Fatalf("setup failed: %v", err)
	}
	
	running, pid, err := IsDaemonRunning(pidFile)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if running {
		t.Fatal("expected not running")
	}
	if pid != 0 {
		t.Fatalf("expected pid 0, got %d", pid)
	}
	
	_, err = os.Stat(pidFile)
	if !os.IsNotExist(err) {
		t.Fatalf("expected stale PID file to be removed")
	}
}

func TestIsDaemonRunning_RunningProcess(t *testing.T) {
	pidFile := filepath.Join(t.TempDir(), "daemon.pid")
	err := os.WriteFile(pidFile, []byte(strconv.Itoa(os.Getpid())), 0644)
	if err != nil {
		t.Fatalf("setup failed: %v", err)
	}
	
	running, pid, err := IsDaemonRunning(pidFile)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if !running {
		t.Fatal("expected running")
	}
	if pid != os.Getpid() {
		t.Fatalf("expected pid %d, got %d", os.Getpid(), pid)
	}
}
