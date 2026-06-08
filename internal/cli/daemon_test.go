package cli

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/shyamsundaravssb/synth/internal/daemon"
)

func TestRunDaemonStop_NotRunning(t *testing.T) {
	daemon.PIDFile = filepath.Join(t.TempDir(), "daemon.pid")

	err := runDaemonStop()
	if err != nil {
		t.Fatalf("expected nil, got %v", err)
	}
}

func TestRunDaemonStart_AlreadyRunning(t *testing.T) {
	daemon.PIDFile = filepath.Join(t.TempDir(), "daemon.pid")
	err := daemon.WritePID(daemon.PIDFile)
	if err != nil {
		t.Fatalf("failed to write pid file: %v", err)
	}

	err = runDaemonStart()
	if err != nil {
		t.Fatalf("expected nil, got %v", err)
	}
}

func TestRunDaemonStop_StaleProcess(t *testing.T) {
	daemon.PIDFile = filepath.Join(t.TempDir(), "daemon.pid")
	err := os.WriteFile(daemon.PIDFile, []byte("99999999"), 0644)
	if err != nil {
		t.Fatalf("failed to write stale pid: %v", err)
	}

	err = runDaemonStop()
	if err != nil {
		t.Fatalf("expected nil, got %v", err)
	}
}
