package daemon

import (
	"context"
	"encoding/json"
	"sort"
	"time"

	"github.com/shyamsundaravssb/synth/internal/store"
)

// LowContextFile represents a file's low-context state.
type LowContextFile struct {
	FilePath         string
	SaveCount        int
	LastNoteTime     time.Time
	HasEverBeenNoted bool
	DaysSinceNote    int
}

// LowContextScorer computes and caches low-context file analysis.
type LowContextScorer struct {
	store     store.Store
	projectID string
	threshold int
	log       *Logger
}

// LowContextSummary provides an overview of the low-context files.
type LowContextSummary struct {
	TotalFiles       int
	NeverNoted       int
	HighestSaveCount int
	MostNeglected    *LowContextFile
}

// NewLowContextScorer creates a new LowContextScorer.
func NewLowContextScorer(s store.Store, projectID string, threshold int, log *Logger) *LowContextScorer {
	return &LowContextScorer{
		store:     s,
		projectID: projectID,
		threshold: threshold,
		log:       log,
	}
}

// ComputeLowContextFiles analyzes the project to find files that have been saved
// many times but lack recent intent notes.
func (lc *LowContextScorer) ComputeLowContextFiles(ctx context.Context) ([]LowContextFile, error) {
	saveCounts, err := lc.store.GetSaveCountsSinceLastNote(ctx, lc.projectID)
	if err != nil {
		return nil, err
	}

	lastNoteTimes, err := lc.store.GetLastNoteTimePerFile(ctx, lc.projectID)
	if err != nil {
		return nil, err
	}

	var results []LowContextFile
	now := time.Now()

	for filePath, saveCount := range saveCounts {
		if saveCount < lc.threshold {
			continue
		}

		lastNote, hasNote := lastNoteTimes[filePath]
		daysSinceNote := 0

		if hasNote {
			daysSinceNote = int(now.Sub(lastNote).Hours() / 24)
		}

		results = append(results, LowContextFile{
			FilePath:         filePath,
			SaveCount:        saveCount,
			LastNoteTime:     lastNote,
			HasEverBeenNoted: hasNote,
			DaysSinceNote:    daysSinceNote,
		})
	}

	sort.Slice(results, func(i, j int) bool {
		if results[i].SaveCount != results[j].SaveCount {
			return results[i].SaveCount > results[j].SaveCount
		}
		return results[i].DaysSinceNote > results[j].DaysSinceNote
	})

	return results, nil
}

// ComputeSummary generates a summary from a slice of LowContextFile.
func ComputeSummary(files []LowContextFile) LowContextSummary {
	if len(files) == 0 {
		return LowContextSummary{}
	}

	var summary LowContextSummary
	summary.TotalFiles = len(files)
	summary.HighestSaveCount = files[0].SaveCount
	summary.MostNeglected = &files[0]

	for _, f := range files {
		if !f.HasEverBeenNoted {
			summary.NeverNoted++
		}
		if f.SaveCount > summary.HighestSaveCount {
			summary.HighestSaveCount = f.SaveCount
		}
	}

	return summary
}

// CacheResults stores the analyzed files as a JSON string in the daemon state.
func (lc *LowContextScorer) CacheResults(ctx context.Context, files []LowContextFile) error {
	data, err := json.Marshal(files)
	if err != nil {
		return err
	}

	return lc.store.SetDaemonState(ctx, lc.projectID, "low_context_files", string(data))
}

// LoadCachedResults retrieves the parsed files from the daemon state.
func (lc *LowContextScorer) LoadCachedResults(ctx context.Context) ([]LowContextFile, error) {
	value, err := lc.store.GetDaemonState(ctx, lc.projectID, "low_context_files")
	if err != nil {
		return nil, err
	}
	if value == "" {
		return []LowContextFile{}, nil
	}

	var files []LowContextFile
	if err := json.Unmarshal([]byte(value), &files); err != nil {
		return nil, err
	}

	return files, nil
}
