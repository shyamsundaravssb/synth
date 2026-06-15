package daemon

import (
	"context"
	"math"
	"sort"
	"time"

	"github.com/shyamsundaravssb/synth/internal/ipc"
	"github.com/shyamsundaravssb/synth/internal/store"
)

func cosineSimilarity(a, b []float32) float64 {
	if len(a) != len(b) || len(a) == 0 {
		return 0.0
	}
	var dot, magA, magB float64
	for i := range a {
		va := float64(a[i])
		vb := float64(b[i])
		dot += va * vb
		magA += va * va
		magB += vb * vb
	}
	if magA == 0 || magB == 0 {
		return 0.0
	}
	return dot / (math.Sqrt(magA) * math.Sqrt(magB))
}

type SearchHandler struct {
	store     store.Store
	engine    EmbedEngine
	projectID string
	log       *Logger
}

func NewSearchHandler(
	s store.Store,
	engine EmbedEngine,
	projectID string,
	log *Logger,
) *SearchHandler {
	return &SearchHandler{
		store:     s,
		engine:    engine,
		projectID: projectID,
		log:       log,
	}
}

func (h *SearchHandler) Handle(req *ipc.Request) *ipc.Response {
	payload, err := ipc.ParseSearchPayload(req)
	if err != nil {
		return ipc.NewErrorResponse("invalid search payload", "ERR_PARSE")
	}

	if payload.Query == "" {
		return ipc.NewErrorResponse("query cannot be empty", "ERR_EMPTY")
	}

	ctx := context.Background()

	queryVecs, err := h.engine.EmbedBatch(ctx, []string{payload.Query})
	if err != nil {
		return ipc.NewErrorResponse("failed to embed query: "+err.Error(), "ERR_EMBED")
	}
	if len(queryVecs) == 0 {
		return ipc.NewErrorResponse("failed to embed query: no vector returned", "ERR_EMBED")
	}
	queryVec := queryVecs[0]

	embeddings, err := h.store.GetAllEmbeddings(ctx, h.projectID)
	if err != nil {
		return ipc.NewErrorResponse("failed to load embeddings", "ERR_STORE")
	}

	type scoredID struct {
		id    string
		score float64
	}
	scores := make([]scoredID, 0, len(embeddings))
	for _, emb := range embeddings {
		sim := cosineSimilarity(queryVec, emb.Embedding)
		scores = append(scores, scoredID{
			id:    emb.IntentID,
			score: sim,
		})
	}

	sort.Slice(scores, func(i, j int) bool {
		return scores[i].score > scores[j].score
	})

	limit := payload.Limit
	if limit <= 0 {
		limit = 10
	}
	if len(scores) > limit {
		scores = scores[:limit]
	}

	ids := make([]string, len(scores))
	for i, s := range scores {
		ids[i] = s.id
	}

	intents, err := h.store.GetIntentsByIDs(ctx, ids)
	if err != nil {
		return ipc.NewErrorResponse("failed to fetch intents", "ERR_STORE")
	}

	scoreMap := make(map[string]float64)
	for _, s := range scores {
		scoreMap[s.id] = s.score
	}

	items := []ipc.SearchResultItem{}
	for _, intent := range intents {
		if payload.FilePath != "" && intent.FilePath != payload.FilePath {
			continue
		}
		if payload.Developer != "" && intent.Developer != payload.Developer {
			continue
		}
		if payload.Since != "" {
			since, err := time.Parse(time.RFC3339, payload.Since)
			if err == nil && intent.Timestamp.Before(since) {
				continue
			}
		}

		score := scoreMap[intent.ID]
		items = append(items, ipc.SearchResultItem{
			ID:         intent.ID,
			FilePath:   intent.FilePath,
			Type:       string(intent.Type),
			Branch:     intent.Branch,
			Developer:  intent.Developer,
			Timestamp:  intent.Timestamp.Format(time.RFC3339),
			What:       intent.What,
			Why:        intent.Why,
			Impact:     intent.Impact,
			Score:      score,
			SearchMode: "semantic",
		})
	}

	data := ipc.SearchData{
		Results:    items,
		Count:      len(items),
		SearchMode: "semantic",
		Query:      payload.Query,
	}

	resp, err := ipc.NewOKResponse(data)
	if err != nil {
		return ipc.NewErrorResponse("failed to build response", "ERR_RESPONSE")
	}
	return resp
}
