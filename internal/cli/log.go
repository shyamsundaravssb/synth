package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/shyamsundaravssb/synth/internal/config"
	"github.com/shyamsundaravssb/synth/internal/daemon"
	"github.com/shyamsundaravssb/synth/internal/git"
	"github.com/shyamsundaravssb/synth/internal/ipc"
	"github.com/shyamsundaravssb/synth/internal/store"
	"github.com/shyamsundaravssb/synth/internal/ui"
	"github.com/shyamsundaravssb/synth/pkg/types"
	"github.com/spf13/cobra"
)

func newLogCmd() *cobra.Command {
	var allFlag bool
	var fileFlag string
	var branchFlag string
	var developerFlag string
	var sinceFlag string
	var jsonFlag bool
	var limitFlag int

	cmd := &cobra.Command{
		Use:   "log",
		Short: "Show intent history for this repository",
		Long: `Display captured intent notes in reverse
chronological order. Filter by file, branch,
developer, or time.`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runLog(allFlag, fileFlag, branchFlag, developerFlag, sinceFlag, jsonFlag, limitFlag)
		},
	}

	cmd.Flags().BoolVar(&allFlag, "all", false, "show all entries (no limit)")
	cmd.Flags().StringVar(&fileFlag, "file", "", "filter by file path")
	cmd.Flags().StringVar(&branchFlag, "branch", "", "filter by branch name")
	cmd.Flags().StringVar(&developerFlag, "developer", "", "filter by developer name")
	cmd.Flags().StringVar(&sinceFlag, "since", "", "time filter e.g. '2 days ago' '1 hour ago' 'yesterday'")
	cmd.Flags().BoolVar(&jsonFlag, "json", false, "output as JSON")
	cmd.Flags().IntVarP(&limitFlag, "n", "n", 20, "number of entries to show")

	return cmd
}

func parseSince(since string) (time.Time, error) {
	if since == "" {
		return time.Time{}, nil
	}

	since = strings.ToLower(strings.TrimSpace(since))
	now := time.Now()

	if since == "yesterday" {
		y, m, d := now.Date()
		t := time.Date(y, m, d-1, 0, 0, 0, 0, now.Location())
		return t, nil
	}
	if since == "today" {
		y, m, d := now.Date()
		t := time.Date(y, m, d, 0, 0, 0, 0, now.Location())
		return t, nil
	}

	parts := strings.Fields(since)
	if len(parts) == 3 && parts[2] == "ago" {
		n, err := strconv.Atoi(parts[0])
		if err != nil {
			return time.Time{}, fmt.Errorf("invalid number: %s", parts[0])
		}

		switch parts[1] {
		case "minute", "minutes":
			return now.Add(-time.Duration(n) * time.Minute), nil
		case "hour", "hours":
			return now.Add(-time.Duration(n) * time.Hour), nil
		case "day", "days":
			return now.Add(-time.Duration(n) * 24 * time.Hour), nil
		}
	}

	return time.Time{}, fmt.Errorf("unrecognized --since format")
}

func runLog(all bool, file, branch, developer, since string, asJSON bool, limit int) error {
	cwd, err := os.Getwd()
	if err != nil {
		ui.ShowError("could not determine current directory: " + err.Error())
		return err
	}

	gitRoot, err := git.FindGitRoot(cwd)
	if err != nil {
		ui.ShowError("not a git repository")
		os.Exit(1)
	}

	configPath := filepath.Join(gitRoot, ".synth", "config.toml")
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		ui.ShowError("synth is not initialized in this repository")
		ui.ShowInfo("run 'synth init' first")
		os.Exit(1)
	}

	cfg, err := config.LoadProjectConfig(gitRoot)
	if err != nil {
		return fmt.Errorf("could not load project config: %w", err)
	}

	sinceTime, err := parseSince(since)
	if err != nil {
		ui.ShowError("unrecognized --since format")
		ui.ShowInfo("examples: '2 days ago', '1 hour ago', 'yesterday'")
		os.Exit(1)
	}

	if all {
		limit = 0
	}

	filter := store.IntentFilter{
		ProjectID: cfg.Project.ID,
		FilePath:  file,
		Branch:    branch,
		Developer: developer,
		Since:     sinceTime,
		Limit:     limit,
	}

	dbPath := store.DBPath(cfg.Project.ID)
	db, err := store.Open(dbPath)
	if err != nil {
		return fmt.Errorf("could not open database: %w", err)
	}
	defer func() { _ = db.Close() }()

	synthStore := store.NewSQLiteStore(db)
	ctx := context.Background()

	intents, err := synthStore.ListIntents(ctx, filter)
	if err != nil {
		return fmt.Errorf("could not list intents: %w", err)
	}

	registryMap := make(map[string]types.FileEntry)
	for _, intent := range intents {
		if intent.Type == types.IntentNewFile {
			entry, err := synthStore.GetFileRegistry(ctx, cfg.Project.ID, intent.FilePath)
			if err == nil && entry != nil {
				registryMap[intent.FilePath] = *entry
			}
		}
	}

	if asJSON {
		if err := ui.RenderIntentJSON(intents); err != nil {
			return err
		}
	} else {
		ui.RenderIntentLog(intents, registryMap, cfg.Project.Name, cfg.Developer.Name, all)
	}

	return nil
}

func newStatusCmd() *cobra.Command {
	var lowContextFlag bool
	var jsonFlag bool

	cmd := &cobra.Command{
		Use:   "status",
		Short: "Show local project summary",
		Long: `Display a summary of intent notes recorded
in this repository including files worked on
and low context files.`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runStatus(lowContextFlag, jsonFlag)
		},
	}
	
	cmd.Flags().BoolVar(&lowContextFlag, "low-context", false, "show only low context files")
	cmd.Flags().BoolVar(&jsonFlag, "json", false, "output as JSON")
	return cmd
}

func runStatus(lowContext bool, asJSON bool) error {
	cwd, err := os.Getwd()
	if err != nil {
		ui.ShowError("could not determine current directory: " + err.Error())
		return err
	}

	gitRoot, err := git.FindGitRoot(cwd)
	if err != nil {
		ui.ShowError("not a git repository")
		os.Exit(1)
	}

	configPath := filepath.Join(gitRoot, ".synth", "config.toml")
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		ui.ShowError("synth is not initialized in this repository")
		ui.ShowInfo("run 'synth init' first")
		os.Exit(1)
	}

	cfg, err := config.LoadProjectConfig(gitRoot)
	if err != nil {
		return fmt.Errorf("could not load project config: %w", err)
	}

	dbPath := store.DBPath(cfg.Project.ID)
	db, err := store.Open(dbPath)
	if err != nil {
		return fmt.Errorf("could not open database: %w", err)
	}
	defer func() { _ = db.Close() }()

	synthStore := store.NewSQLiteStore(db)
	ctx := context.Background()

	var lowContextFiles []ipc.LowContextFileItem

	running, _, _ := daemon.IsDaemonRunning(daemon.PIDFile)
	var statusData *ipc.StatusData

	if running {
		client := ipc.NewClient(daemon.SockFile)
		req, _ := ipc.NewRequest(ipc.TypeStatus, ipc.StatusPayload{})
		resp, err := client.Send(req)
		if err == nil && resp.Status == ipc.StatusOK {
			sd, _ := ipc.ParseStatusData(resp)
			statusData = sd
		}
	}

	if lowContext {
		if running && statusData != nil {
			ui.RenderLowContextFiles(statusData.LowContextFiles, cfg.Project.Name, asJSON)
		} else {
			threshold := cfg.Behavior.LowContextThreshold
			files, _ := synthStore.GetLowContextFiles(ctx, cfg.Project.ID, threshold)
			
			counts, _ := synthStore.GetSaveCountsSinceLastNote(ctx, cfg.Project.ID)
			times, _ := synthStore.GetLastNoteTimePerFile(ctx, cfg.Project.ID)
			
			var items []ipc.LowContextFileItem
			for _, f := range files {
				saveCount := counts[f]
				lastTime, hasNote := times[f]
				var daysSinceNote int
				if hasNote {
					daysSinceNote = int(time.Since(lastTime).Hours() / 24)
				}
				items = append(items, ipc.LowContextFileItem{
					FilePath:         f,
					SaveCount:        saveCount,
					HasEverBeenNoted: hasNote,
					DaysSinceNote:    daysSinceNote,
				})
			}
			ui.RenderLowContextFiles(items, cfg.Project.Name, asJSON)
			if !asJSON {
				ui.ShowInfo("daemon offline — showing cached data only")
			}
		}
		return nil
	}

	if running && statusData != nil {
		lowContextFiles = statusData.LowContextFiles
	} else {
		threshold := cfg.Behavior.LowContextThreshold
		files, _ := synthStore.GetLowContextFiles(ctx, cfg.Project.ID, threshold)
		
		counts, _ := synthStore.GetSaveCountsSinceLastNote(ctx, cfg.Project.ID)
		times, _ := synthStore.GetLastNoteTimePerFile(ctx, cfg.Project.ID)
		
		for _, f := range files {
			saveCount := counts[f]
			lastTime, hasNote := times[f]
			var daysSinceNote int
			if hasNote {
				daysSinceNote = int(time.Since(lastTime).Hours() / 24)
			}
			lowContextFiles = append(lowContextFiles, ipc.LowContextFileItem{
				FilePath:         f,
				SaveCount:        saveCount,
				HasEverBeenNoted: hasNote,
				DaysSinceNote:    daysSinceNote,
			})
		}
	}

	totalNotes, err := synthStore.CountIntents(ctx, cfg.Project.ID)
	if err != nil {
		return fmt.Errorf("could not count intents: %w", err)
	}

	recent, err := synthStore.GetRecentIntents(ctx, cfg.Project.ID, 1)
	if err != nil {
		return fmt.Errorf("could not get recent intents: %w", err)
	}
	var lastNote *types.Intent
	if len(recent) > 0 {
		lastNote = &recent[0]
	}

	allIntents, err := synthStore.ListIntents(ctx, store.IntentFilter{
		ProjectID: cfg.Project.ID,
		Limit:     1000000,
	})
	if err != nil {
		return fmt.Errorf("could not list all intents: %w", err)
	}

	filesWithNotes := buildFileSummary(allIntents)

	statusResult := ui.StatusData{
		ProjectName:     cfg.Project.Name,
		Developer:       cfg.Developer.Name,
		TotalNotes:      totalNotes,
		FilesWithNotes:  filesWithNotes,
		LowContextFiles: lowContextFiles,
		LastNote:        lastNote,
	}

	if asJSON {
		// Output StatusResult as JSON
		out, _ := json.MarshalIndent(statusResult, "", "  ")
		fmt.Println(string(out))
	} else {
		ui.RenderStatus(statusResult)
	}

	return nil
}

// buildFileSummary computes file summaries from a list of intents and returns them sorted by count descending.
// Exposed for testing.
func buildFileSummary(intents []types.Intent) []ui.FileNoteSummary {
	filesMap := make(map[string]int)
	for _, intent := range intents {
		filesMap[intent.FilePath]++
	}

	var filesWithNotes []ui.FileNoteSummary
	for f, c := range filesMap {
		filesWithNotes = append(filesWithNotes, ui.FileNoteSummary{FilePath: f, NoteCount: c})
	}

	// Sort by count descending
	sort.Slice(filesWithNotes, func(i, j int) bool {
		return filesWithNotes[i].NoteCount > filesWithNotes[j].NoteCount
	})
	return filesWithNotes
}
