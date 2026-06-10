package daemon

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/shyamsundaravssb/synth/internal/store"
)

type WatcherLogger interface {
	Info(msg string)
	Warn(msg string)
	Error(msg, errStr string)
	InfoFile(msg, filePath string)
}

type Watcher struct {
	gitRoot    string
	projectID  string
	store      store.Store
	fsWatcher  *fsnotify.Watcher
	log        WatcherLogger
	shutdownCh <-chan struct{}
	debounce   map[string]time.Time
	debounceMu sync.Mutex
	debounceD  time.Duration
}

func NewWatcher(gitRoot, projectID string, s store.Store, log WatcherLogger, shutdownCh <-chan struct{}) (*Watcher, error) {
	fsWatcher, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, err
	}

	return &Watcher{
		gitRoot:    gitRoot,
		projectID:  projectID,
		store:      s,
		fsWatcher:  fsWatcher,
		log:        log,
		shutdownCh: shutdownCh,
		debounce:   make(map[string]time.Time),
		debounceD:  500 * time.Millisecond,
	}, nil
}

func (w *Watcher) Start() error {
	if err := w.watchDirectory(w.gitRoot); err != nil {
		return err
	}

	go w.eventLoop()

	w.log.Info("file watcher started: " + w.gitRoot)
	return nil
}

func (w *Watcher) watchDirectory(dir string) error {
	_ = filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil
		}

		if d.IsDir() {
			if d.Name() == ".git" {
				return filepath.SkipDir
			}
			if strings.HasPrefix(d.Name(), ".") && d.Name() != "." && d.Name() != ".." {
				return filepath.SkipDir
			}

			if err := w.fsWatcher.Add(path); err != nil {
				w.log.Error("failed to watch directory", err.Error())
			}
		}
		return nil
	})

	return nil
}

func (w *Watcher) eventLoop() {
	for {
		select {
		case event, ok := <-w.fsWatcher.Events:
			if !ok {
				return
			}
			w.handleEvent(event)

		case err, ok := <-w.fsWatcher.Errors:
			if !ok {
				return
			}
			w.log.Error("watcher error", err.Error())

		case <-w.shutdownCh:
			w.Stop()
			return
		}
	}
}

func (w *Watcher) handleEvent(event fsnotify.Event) {
	if !event.Has(fsnotify.Write) {
		return
	}

	filePath := event.Name

	if !w.shouldTrack(filePath) {
		return
	}

	w.debounceMu.Lock()
	last, exists := w.debounce[filePath]
	now := time.Now()
	if exists && now.Sub(last) < w.debounceD {
		w.debounceMu.Unlock()
		return
	}
	w.debounce[filePath] = now
	w.debounceMu.Unlock()

	relPath, err := filepath.Rel(w.gitRoot, filePath)
	if err != nil {
		return
	}

	ctx := context.Background()
	err = w.store.RecordFileSave(ctx, w.projectID, relPath, false)
	if err != nil {
		w.log.Error("failed to record save", err.Error())
	} else {
		w.log.InfoFile("file saved", relPath)
	}
}

func (w *Watcher) shouldTrack(filePath string) bool {
	if strings.Contains(filePath, "/.git/") || strings.HasSuffix(filePath, "/.git") {
		return false
	}
	if strings.Contains(filePath, "/.synth/") || strings.HasSuffix(filePath, "/.synth") {
		return false
	}

	baseName := filepath.Base(filePath)
	if strings.HasPrefix(baseName, ".") {
		return false
	}

	ext := strings.ToLower(filepath.Ext(filePath))
	switch ext {
	case ".exe", ".bin", ".so", ".dylib", ".a", ".o",
		".png", ".jpg", ".jpeg", ".gif", ".webp", ".ico",
		".pdf", ".zip", ".tar", ".gz", ".bz2", ".xz",
		".mp4", ".mp3", ".wav", ".avi", ".mov",
		".db", ".sqlite", ".sqlite3":
		return false
	}

	return strings.HasPrefix(filePath, w.gitRoot)
}

func (w *Watcher) Stop() {
	_ = w.fsWatcher.Close()
	w.log.Info("file watcher stopped")
}
