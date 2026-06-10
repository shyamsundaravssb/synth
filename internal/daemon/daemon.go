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
	"github.com/shyamsundaravssb/synth/internal/ipc"
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
	log          *Logger
	ipcServer    *ipc.Server
}

func New() *Daemon {
	return &Daemon{
		PIDFile:    PIDFile,
		SockFile:   SockFile,
		LogFile:    LogFile,
		shutdownCh: make(chan struct{}),
		done:       make(chan struct{}),
		log:        NewLogger(LogFile),
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

	d.log.Info("daemon started")

	d.ipcServer = ipc.NewServer(d.SockFile, d.ShutdownCh(), d.log)
	d.ipcServer.Handle(ipc.TypePing, d.handlePing)
	d.ipcServer.Handle(ipc.TypeStatus, d.handleStatus)

	if err := d.ipcServer.Start(); err != nil {
		d.log.Error("failed to start IPC server", err.Error())
		return err
	}

	for {
		select {
		case sig := <-sigCh:
			d.log.Info("received signal: " + sig.String())
			go func() {
				if err := d.Shutdown(); err != nil {
					d.log.Error("shutdown failed", err.Error())
				}
			}()
			return nil
		case <-d.shutdownCh:
			return nil
		}
	}
}

func (d *Daemon) Shutdown() error {
	d.log.Info("daemon shutting down")

	d.shutdownOnce.Do(func() {
		close(d.shutdownCh)
	})

	select {
	case <-d.done:
		// clean exit
	case <-time.After(5 * time.Second):
		// forced exit after timeout
		d.log.Warn("shutdown timeout — forcing exit")
	}

	return nil
}

func (d *Daemon) ShutdownCh() <-chan struct{} {
	return d.shutdownCh
}

func (d *Daemon) handlePing(req *ipc.Request) *ipc.Response {
	resp, _ := ipc.NewOKResponse(ipc.PingData{
		PID:     os.Getpid(),
		Version: "0.1.0",
	})
	return resp
}

func (d *Daemon) handleStatus(req *ipc.Request) *ipc.Response {
	resp, _ := ipc.NewOKResponse(ipc.StatusData{
		Running:  true,
		PID:      os.Getpid(),
		LogFile:  d.LogFile,
		SockFile: d.SockFile,
	})
	return resp
}
