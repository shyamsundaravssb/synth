package daemon

import (
	"context"
	"fmt"
	"time"
)

const LowContextPollInterval = 60 * time.Second

type LowContextLoop struct {
	scorer     *LowContextScorer
	log        *Logger
	shutdownCh <-chan struct{}
}

func NewLowContextLoop(scorer *LowContextScorer, log *Logger, shutdownCh <-chan struct{}) *LowContextLoop {
	return &LowContextLoop{
		scorer:     scorer,
		log:        log,
		shutdownCh: shutdownCh,
	}
}

func (l *LowContextLoop) Start() {
	go l.runLoop()
	l.log.Info("low context loop started")
}

func (l *LowContextLoop) runLoop() {
	l.tick()

	ticker := time.NewTicker(LowContextPollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-l.shutdownCh:
			l.log.Info("low context loop stopped")
			return
		case <-ticker.C:
			l.tick()
		}
	}
}

func (l *LowContextLoop) tick() {
	ctx := context.Background()

	files, err := l.scorer.ComputeLowContextFiles(ctx)
	if err != nil {
		l.log.Error("low context compute failed", err.Error())
		return
	}

	err = l.scorer.CacheResults(ctx, files)
	if err != nil {
		l.log.Error("low context cache failed", err.Error())
		return
	}

	if len(files) > 0 {
		summary := ComputeSummary(files)
		l.log.Info(fmt.Sprintf("low context: %d file(s) need attention", summary.TotalFiles))
	}
}
