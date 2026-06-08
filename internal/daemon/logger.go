package daemon

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"time"
)

type Logger struct {
	filePath string
	pid      int
	mu       sync.Mutex
	maxBytes int64
}

func NewLogger(filePath string) *Logger {
	return &Logger{
		filePath: filePath,
		pid:      os.Getpid(),
		maxBytes: 10 * 1024 * 1024,
	}
}

type logEntry struct {
	TS    string `json:"ts"`
	Level string `json:"level"`
	PID   int    `json:"pid"`
	Msg   string `json:"msg"`
	Error string `json:"error,omitempty"`
	File  string `json:"file,omitempty"`
}

func (l *Logger) Info(msg string) {
	l.mu.Lock()
	defer l.mu.Unlock()

	entry := logEntry{
		TS:    time.Now().UTC().Format(time.RFC3339Nano),
		Level: "info",
		PID:   l.pid,
		Msg:   msg,
	}
	data, err := json.Marshal(entry)
	if err == nil {
		_ = l.writeEntry(data)
	}
}

func (l *Logger) Warn(msg string) {
	l.mu.Lock()
	defer l.mu.Unlock()

	entry := logEntry{
		TS:    time.Now().UTC().Format(time.RFC3339Nano),
		Level: "warn",
		PID:   l.pid,
		Msg:   msg,
	}
	data, err := json.Marshal(entry)
	if err == nil {
		_ = l.writeEntry(data)
	}
}

func (l *Logger) Error(msg, errStr string) {
	l.mu.Lock()
	defer l.mu.Unlock()

	entry := logEntry{
		TS:    time.Now().UTC().Format(time.RFC3339Nano),
		Level: "error",
		PID:   l.pid,
		Msg:   msg,
		Error: errStr,
	}
	data, err := json.Marshal(entry)
	if err == nil {
		_ = l.writeEntry(data)
	}
}

func (l *Logger) InfoFile(msg, filePath string) {
	l.mu.Lock()
	defer l.mu.Unlock()

	entry := logEntry{
		TS:    time.Now().UTC().Format(time.RFC3339Nano),
		Level: "info",
		PID:   l.pid,
		Msg:   msg,
		File:  filePath,
	}
	data, err := json.Marshal(entry)
	if err == nil {
		_ = l.writeEntry(data)
	}
}

func (l *Logger) writeEntry(data []byte) error {
	if l.shouldRotate() {
		_ = l.rotate()
	}

	dir := filepath.Dir(l.filePath)
	_ = os.MkdirAll(dir, 0755)

	f, err := os.OpenFile(l.filePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	defer f.Close()

	_, err = f.Write(append(data, '\n'))
	return err
}

func (l *Logger) shouldRotate() bool {
	info, err := os.Stat(l.filePath)
	if err != nil {
		return false
	}
	return info.Size() >= l.maxBytes
}

func (l *Logger) rotate() error {
	backupPath := l.filePath + ".1"
	if _, err := os.Stat(backupPath); err == nil {
		_ = os.Remove(backupPath)
	}
	return os.Rename(l.filePath, backupPath)
}
