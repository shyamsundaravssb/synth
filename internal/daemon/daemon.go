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
	PIDFile  string
	SockFile string
	LogFile  string
}

func New() *Daemon {
	return &Daemon{
		PIDFile:  PIDFile,
		SockFile: SockFile,
		LogFile:  LogFile,
	}
}

func (d *Daemon) Run() error {
	_, _ = fmt.Fprintln(os.Stderr, "synth daemon: started (pid:", os.Getpid(), ")")
	select {}
}
