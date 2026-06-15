package search

import (
	"context"
	"fmt"
	"time"

	"github.com/shyamsundaravssb/synth/internal/ipc"
	"github.com/shyamsundaravssb/synth/internal/store"
	"github.com/shyamsundaravssb/synth/pkg/types"
)

type SearchMode string

const (
	ModeSemantic SearchMode = "semantic"
	ModeFTS5     SearchMode = "fts5"
)

type SearchRequest struct {
	Query      string
	Limit      int
	FilePath   string
	Since      time.Time
	Developer  string
	NoFallback bool
}

type SearchResult struct {
	ID        string
	FilePath  string
	Type      string
	Branch    string
	Developer string
	Timestamp time.Time
	What      string
	Why       string
	Impact    string
	Score     float64
	Mode      SearchMode
}

type SearchResponse struct {
	Results    []SearchResult
	Mode       SearchMode
	Query      string
	IsFallback bool
}

type Searcher struct {
	sockPath  string
	store     store.Store
	projectID string
}

func New(sockPath, projectID string, s store.Store) *Searcher {
	return &Searcher{
		sockPath:  sockPath,
		projectID: projectID,
		store:     s,
	}
}

func (s *Searcher) Search(ctx context.Context, req SearchRequest) (*SearchResponse, error) {
	if req.Limit <= 0 {
		req.Limit = 10
	}

	results, err := s.searchSemantic(ctx, req)
	if err == nil {
		return &SearchResponse{
			Results:    results,
			Mode:       ModeSemantic,
			Query:      req.Query,
			IsFallback: false,
		}, nil
	}

	if req.NoFallback {
		return nil, fmt.Errorf("semantic search unavailable: %w", err)
	}

	results, ftsErr := s.searchFTS5(ctx, req)
	if ftsErr != nil {
		return nil, fmt.Errorf("both semantic and keyword search failed: %w", ftsErr)
	}

	return &SearchResponse{
		Results:    results,
		Mode:       ModeFTS5,
		Query:      req.Query,
		IsFallback: true,
	}, nil
}

func (s *Searcher) searchSemantic(ctx context.Context, req SearchRequest) ([]SearchResult, error) {
	client := ipc.NewClient(s.sockPath)
	if !client.IsDaemonReachable() {
		return nil, fmt.Errorf("daemon not reachable")
	}

	payload := ipc.SearchPayload{
		Query: req.Query,
		Limit: req.Limit,
	}
	if req.FilePath != "" {
		payload.FilePath = req.FilePath
	}
	if req.Since != (time.Time{}) {
		payload.Since = req.Since.Format(time.RFC3339)
	}
	if req.Developer != "" {
		payload.Developer = req.Developer
	}

	ipcReq, err := ipc.NewRequest(ipc.TypeSearch, payload)
	if err != nil {
		return nil, err
	}

	resp, err := client.Send(ipcReq)
	if err != nil {
		return nil, err
	}

	if resp.Status == ipc.StatusError {
		errData, _ := ipc.ParseErrorData(resp)
		return nil, fmt.Errorf("search error: %s", errData.Message)
	}

	data, err := ipc.ParseSearchData(resp)
	if err != nil {
		return nil, err
	}

	results := make([]SearchResult, len(data.Results))
	for i, item := range data.Results {
		ts, _ := time.Parse(time.RFC3339, item.Timestamp)
		results[i] = SearchResult{
			ID:        item.ID,
			FilePath:  item.FilePath,
			Type:      item.Type,
			Branch:    item.Branch,
			Developer: item.Developer,
			Timestamp: ts,
			What:      item.What,
			Why:       item.Why,
			Impact:    item.Impact,
			Score:     item.Score,
			Mode:      ModeSemantic,
		}
	}
	return results, nil
}

func (s *Searcher) searchFTS5(ctx context.Context, req SearchRequest) ([]SearchResult, error) {
	intents, err := s.store.SearchFTS(ctx, s.projectID, req.Query, req.Limit)
	if err != nil {
		return nil, err
	}

	var filtered []types.Intent
	for _, intent := range intents {
		if req.FilePath != "" && intent.FilePath != req.FilePath {
			continue
		}
		if req.Since != (time.Time{}) && !intent.Timestamp.After(req.Since) {
			continue
		}
		if req.Developer != "" && intent.Developer != req.Developer {
			continue
		}
		filtered = append(filtered, intent)
	}

	results := make([]SearchResult, len(filtered))
	for i, intent := range filtered {
		results[i] = SearchResult{
			ID:        intent.ID,
			FilePath:  intent.FilePath,
			Type:      string(intent.Type),
			Branch:    intent.Branch,
			Developer: intent.Developer,
			Timestamp: intent.Timestamp,
			What:      intent.What,
			Why:       intent.Why,
			Impact:    intent.Impact,
			Score:     0.0,
			Mode:      ModeFTS5,
		}
	}

	return results, nil
}
