package daemon

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"

	"github.com/adrg/xdg"
	"os/signal"
	"sync"
	"time"
)

var ErrDaemonNotRunning = errors.New("daemon is not running")

var (
	PIDFile  string
	SockFile string
	LogFile  string
)

func init() {
	PIDFile = resolvePath("daemon.pid")
	SockFile = resolvePath("daemon.sock")
	LogFile = resolvePath("daemon.log")
}

func resolvePath(filename string) string {
	if xdg.DataHome != "" {
		return filepath.Join(xdg.DataHome, "synth", filename)
	}
	home, err := os.UserHomeDir()
	if err == nil {
		return filepath.Join(home, ".synth", filename)
	}
	return filepath.Join("/tmp/synth", filename)
}

func WritePID(pidFile string) error {
	dir := filepath.Dir(pidFile)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	pidStr := strconv.Itoa(os.Getpid())
	return os.WriteFile(pidFile, []byte(pidStr), 0644)
}

func ReadPID(pidFile string) (int, error) {
	data, err := os.ReadFile(pidFile)
	if err != nil {
		if os.IsNotExist(err) {
			return 0, ErrDaemonNotRunning
		}
		return 0, err
	}
	pidStr := strings.TrimSpace(string(data))
	pid, err := strconv.Atoi(pidStr)
	if err != nil {
		return 0, err
	}
	return pid, nil
}

func RemovePID(pidFile string) error {
	err := os.Remove(pidFile)
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

func IsProcessRunning(pid int) bool {
	process, err := os.FindProcess(pid)
	if err != nil {
		return false
	}
	err = process.Signal(syscall.Signal(0))
	return err == nil
}

func IsDaemonRunning(pidFile string) (bool, int, error) {
	pid, err := ReadPID(pidFile)
	if err != nil {
		if errors.Is(err, ErrDaemonNotRunning) {
			return false, 0, nil
		}
		return false, 0, err
	}

	if IsProcessRunning(pid) {
		return true, pid, nil
	}

	_ = RemovePID(pidFile)
	return false, 0, nil
}

type Daemon struct {
	PIDFile      string
	SockFile     string
	LogFile      string
	shutdownCh   chan struct{}
	done         chan struct{}
	shutdownOnce sync.Once
}

func New() *Daemon {
	return &Daemon{
		PIDFile:    PIDFile,
		SockFile:   SockFile,
		LogFile:    LogFile,
		shutdownCh: make(chan struct{}),
		done:       make(chan struct{}),
	}
}

func (d *Daemon) Run() error {
	err := WritePID(d.PIDFile)
	if err != nil {
		return fmt.Errorf("failed to write PID: %w", err)
	}

	defer func() {
		_ = RemovePID(d.PIDFile)
		close(d.done)
	}()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGTERM, syscall.SIGINT)
	defer signal.Stop(sigCh)

	_ = appendToLog(d.LogFile, "daemon started", os.Getpid())

	for {
		select {
		case sig := <-sigCh:
			_ = appendToLog(d.LogFile, fmt.Sprintf("received signal: %s", sig), os.Getpid())
			return d.Shutdown()
		case <-d.shutdownCh:
			return nil
		}
	}
}

func (d *Daemon) Shutdown() error {
	_ = appendToLog(d.LogFile, "daemon shutting down", os.Getpid())

	d.shutdownOnce.Do(func() {
		close(d.shutdownCh)
	})

	select {
	case <-d.done:
		// clean exit
	case <-time.After(5 * time.Second):
		// forced exit after timeout
		_ = appendToLog(d.LogFile, "shutdown timeout — forcing exit", os.Getpid())
	}

	return nil
}

func appendToLog(logFile, message string, pid int) error {
	f, err := os.OpenFile(logFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	defer f.Close()

	ts := time.Now().UTC().Format(time.RFC3339)
	line := fmt.Sprintf("%s pid=%d msg=%q\n", ts, pid, message)
	_, err = f.WriteString(line)
	return err
}

func (d *Daemon) ShutdownCh() <-chan struct{} {
	return d.shutdownCh
}
