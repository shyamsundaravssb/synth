package cli

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/shyamsundaravssb/synth/internal/config"
	"github.com/shyamsundaravssb/synth/internal/daemon"
	"github.com/shyamsundaravssb/synth/internal/git"
	"github.com/shyamsundaravssb/synth/internal/search"
	"github.com/shyamsundaravssb/synth/internal/store"
	"github.com/shyamsundaravssb/synth/internal/ui"
	"github.com/spf13/cobra"
)

type searchFlags struct {
	limit      int
	file       string
	since      string
	developer  string
	json       bool
	noFallback bool
}

func newSearchCmd() *cobra.Command {
	flags := searchFlags{}

	cmd := &cobra.Command{
		Use:   "search <query>",
		Short: "Search intent notes by meaning",
		Long: `Search your intent notes using natural
          language. When the daemon is running,
          uses semantic similarity. Falls back to
          keyword search when daemon is offline.`,
		Args: cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			query := strings.Join(args, " ")
			return runSearch(query, flags)
		},
	}

	cmd.Flags().IntVar(&flags.limit, "limit", 10, "max results to return")
	cmd.Flags().StringVar(&flags.file, "file", "", "filter by file path")
	cmd.Flags().StringVar(&flags.since, "since", "", "time filter (same format as synth log)")
	cmd.Flags().StringVar(&flags.developer, "developer", "", "filter by developer")
	cmd.Flags().BoolVar(&flags.json, "json", false, "JSON output")
	cmd.Flags().BoolVar(&flags.noFallback, "no-fallback", false, "fail if semantic unavailable")

	return cmd
}

func runSearch(query string, flags searchFlags) error {
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

	sinceTime, err := parseSince(flags.since)
	if err != nil {
		ui.ShowError("unrecognized --since format")
		os.Exit(1)
	}

	dbPath := store.DBPath(cfg.Project.ID)
	db, err := store.Open(dbPath)
	if err != nil {
		return fmt.Errorf("could not open database: %w", err)
	}
	defer func() { _ = db.Close() }()

	synthStore := store.NewSQLiteStore(db)

	req := buildSearchRequest(query, flags, sinceTime)

	searcher := search.New(
		daemon.SockFile,
		cfg.Project.ID,
		synthStore,
	)

	response, err := searcher.Search(context.Background(), req)
	if err != nil {
		ui.ShowError(err.Error())
		return err
	}

	if flags.json {
		return ui.RenderSearchResultsJSON(response)
	}

	ui.RenderSearchResults(response.Results, response, cfg.Project.Name)
	return nil
}

func buildSearchRequest(query string, flags searchFlags, sinceTime time.Time) search.SearchRequest {
	return search.SearchRequest{
		Query:      query,
		Limit:      flags.limit,
		FilePath:   flags.file,
		Since:      sinceTime,
		Developer:  flags.developer,
		NoFallback: flags.noFallback,
	}
}
