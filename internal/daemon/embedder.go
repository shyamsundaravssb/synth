package daemon

import (
	"context"
	"fmt"
	"time"

	"github.com/shyamsundaravssb/synth/internal/embed"
	"github.com/shyamsundaravssb/synth/internal/store"
)

const (
	EmbedPollInterval = 5 * time.Second
	EmbedBatchSize    = 10
)

type EmbedEngine interface {
	IsModelPresent() bool
	EmbedBatch(ctx context.Context, texts []string) ([][]float32, error)
}

type Embedder struct {
	store      store.Store
	engine     EmbedEngine
	projectID  string
	log        *Logger
	shutdownCh <-chan struct{}
}

func NewEmbedder(
	s store.Store,
	engine EmbedEngine,
	projectID string,
	log *Logger,
	shutdownCh <-chan struct{},
) *Embedder {
	return &Embedder{
		store:      s,
		engine:     engine,
		projectID:  projectID,
		log:        log,
		shutdownCh: shutdownCh,
	}
}

func (e *Embedder) Start() {
	go e.runLoop()
	e.log.Info("embedding loop started")
}

func (e *Embedder) runLoop() {
	ticker := time.NewTicker(EmbedPollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-e.shutdownCh:
			e.log.Info("embedding loop stopped")
			return

		case <-ticker.C:
			e.embedPending()
		}
	}
}

func (e *Embedder) embedPending() {
	if !e.engine.IsModelPresent() {
		return
	}

	ctx := context.Background()
	intents, err := e.store.ListIntentsWithoutEmbeddings(ctx, e.projectID, EmbedBatchSize)
	if err != nil {
		e.log.Error("failed to list unembedded intents", err.Error())
		return
	}
	if len(intents) == 0 {
		return
	}

	texts := make([]string, len(intents))
	for i, intent := range intents {
		text := intent.What + " " + intent.Why
		if intent.Impact != "" {
			text += " " + intent.Impact
		}
		texts[i] = text
	}

	vectors, err := e.engine.EmbedBatch(ctx, texts)
	if err != nil {
		e.log.Error("batch embedding failed", err.Error())
		return
	}

	for i, intent := range intents {
		record := store.EmbeddingRecord{
			IntentID:  intent.ID,
			ProjectID: e.projectID,
			Embedding: vectors[i],
			Model:     embed.ModelName,
		}
		err = e.store.InsertEmbedding(ctx, record)
		if err != nil {
			e.log.Error("failed to store embedding", err.Error())
			continue
		}
	}

	e.log.Info(fmt.Sprintf("embedded %d intent(s)", len(intents)))

	_ = e.store.SetDaemonState(
		ctx,
		e.projectID,
		"last_embed_run",
		time.Now().UTC().Format(time.RFC3339),
	)
}
