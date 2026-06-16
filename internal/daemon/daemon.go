package daemon

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"

	"github.com/adrg/xdg"
	"github.com/shyamsundaravssb/synth/internal/config"
	"github.com/shyamsundaravssb/synth/internal/embed"
	"github.com/shyamsundaravssb/synth/internal/git"
	"github.com/shyamsundaravssb/synth/internal/ipc"
	"github.com/shyamsundaravssb/synth/internal/store"
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
	watcher      *Watcher
	embedder         *Embedder
	searchHandler    *SearchHandler
	lowContextScorer *LowContextScorer
	lowContextLoop   *LowContextLoop
	startTime        time.Time
	synthStore    store.Store
	projectID     string
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
	d.startTime = time.Now()

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

	var projCfg *config.ProjectConfig
	if cwd, err := os.Getwd(); err == nil {
		if gitRoot, err := git.FindGitRoot(cwd); err != nil {
			d.log.Warn("no git repository found — file watching disabled")
		} else {
			if cfg, err := config.LoadProjectConfig(gitRoot); err != nil {
				d.log.Warn("no synth config found — file watching disabled")
			} else {
				projCfg = cfg
				if db, err := store.Open(store.DBPath(cfg.Project.ID)); err != nil {
					d.log.Error("failed to open store for watcher", err.Error())
				} else {
					synthStore := store.NewSQLiteStore(db)
					d.synthStore = synthStore
					d.projectID = cfg.Project.ID
					d.watcher, err = NewWatcher(gitRoot, cfg.Project.ID, synthStore, d.log, d.ShutdownCh())
					if err != nil {
						d.log.Error("failed to create watcher", err.Error())
					} else {
						if err := d.watcher.Start(); err != nil {
							d.log.Error("failed to start watcher", err.Error())
						}
					}
				}
			}
		}
	}

	if d.synthStore != nil {
		modelDir := embed.DefaultModelDir()
		embEngine := embed.New(modelDir, d.log)
		d.embedder = NewEmbedder(
			d.synthStore,
			embEngine,
			d.projectID,
			d.log,
			d.ShutdownCh(),
		)
		d.embedder.Start()

		d.searchHandler = NewSearchHandler(
			d.synthStore,
			embEngine,
			d.projectID,
			d.log,
		)
		d.ipcServer.Handle(ipc.TypeSearch, d.searchHandler.Handle)
		d.log.Info("search handler registered")

		threshold := 5
		if projCfg != nil {
			threshold = projCfg.Behavior.LowContextThreshold
		}
		d.lowContextScorer = NewLowContextScorer(
			d.synthStore,
			d.projectID,
			threshold,
			d.log,
		)
		d.lowContextLoop = NewLowContextLoop(
			d.lowContextScorer,
			d.log,
			d.ShutdownCh(),
		)
		d.lowContextLoop.Start()
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
	ctx := context.Background()
	notesCount := 0
	fileSavesCount := 0
	embeddingsCount := 0
	lowContextCount := 0
	var lowContextFiles []ipc.LowContextFileItem

	if d.synthStore != nil && d.projectID != "" {
		if c, err := d.synthStore.CountIntents(ctx, d.projectID); err == nil {
			notesCount = c
		}
		if sc, err := d.synthStore.CountFileSaves(ctx, d.projectID); err == nil {
			fileSavesCount = sc
		}
		if ec, err := d.synthStore.CountEmbeddings(ctx, d.projectID); err == nil {
			embeddingsCount = ec
		}
		if d.lowContextScorer != nil {
			if cached, err := d.lowContextScorer.LoadCachedResults(ctx); err == nil {
				lowContextCount = len(cached)
				items := []ipc.LowContextFileItem{}
				for _, f := range cached {
					items = append(items, ipc.LowContextFileItem{
						FilePath:         f.FilePath,
						SaveCount:        f.SaveCount,
						HasEverBeenNoted: f.HasEverBeenNoted,
						DaysSinceNote:    f.DaysSinceNote,
					})
				}
				lowContextFiles = items
			}
		}
	}

	data := ipc.StatusData{
		Running:         true,
		PID:             os.Getpid(),
		UptimeS:         int64(time.Since(d.startTime).Seconds()),
		NotesCount:      notesCount,
		FileSavesCount:  fileSavesCount,
		EmbeddingsCount: embeddingsCount,
		LowContextCount: lowContextCount,
		LowContextFiles: lowContextFiles,
		LogFile:         d.LogFile,
		SockFile:        d.SockFile,
	}
	resp, _ := ipc.NewOKResponse(data)
	return resp
}
