package cli

import (
	"context"
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/oklog/ulid/v2"

	"github.com/shyamsundaravssb/synth/internal/config"
	"github.com/shyamsundaravssb/synth/internal/git"
	"github.com/shyamsundaravssb/synth/internal/store"
	"github.com/shyamsundaravssb/synth/internal/ui"
	"github.com/shyamsundaravssb/synth/pkg/types"
	"github.com/spf13/cobra"
)

// newNoteCmd creates the cobra command for 'synth note'.
func newNoteCmd() *cobra.Command {
	var fileFlag string
	var quickFlag bool
	var whatFlag string
	var whyFlag string

	cmd := &cobra.Command{
		Use:   "note",
		Short: "Capture an intent note for a file change",
		Long: `Record what you changed and why.
Synth stores this intent alongside your code
so merges can be made intelligently.`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runNoteInteractive(fileFlag, quickFlag, whatFlag, whyFlag)
		},
	}

	cmd.Flags().StringVar(&fileFlag, "file", "", "file to note (skips file selection prompt)")
	cmd.Flags().BoolVar(&quickFlag, "quick", false, "skip the optional impact prompt")
	cmd.Flags().StringVar(&whatFlag, "what", "", "what changed (skips prompt, for scripting)")
	cmd.Flags().StringVar(&whyFlag, "why", "", "why it changed (skips prompt, for scripting)")

	return cmd
}

// runNoteInteractive handles the interactive prompts and delegates to core logic.
func runNoteInteractive(fileFlag string, quick bool, whatFlag, whyFlag string) error {
	// Step 1 — Find git root and load config.
	cwd, err := os.Getwd()
	if err != nil {
		ui.ShowError("could not determine current directory: " + err.Error())
		return err
	}

	gitRoot, err := git.FindGitRoot(cwd)
	if err != nil {
		ui.ShowError("not a git repository")
		return err
	}

	configPath := filepath.Join(gitRoot, ".synth", "config.toml")
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		ui.ShowError("synth is not initialized in this repository")
		ui.ShowInfo("run 'synth init' first")
		return fmt.Errorf("not initialized")
	}

	cfg, err := config.LoadProjectConfig(gitRoot)
	if err != nil {
		ui.ShowError("could not load config: " + err.Error())
		return err
	}

	// Step 2 — Determine target file.
	var relPath string

	if fileFlag != "" {
		// Resolve --file flag to an absolute path, then make relative.
		absPath, err := filepath.Abs(fileFlag)
		if err != nil {
			ui.ShowError("invalid file path: " + err.Error())
			return err
		}
		if _, err := os.Stat(absPath); os.IsNotExist(err) {
			ui.ShowError("file does not exist: " + fileFlag)
			return fmt.Errorf("file not found: %s", fileFlag)
		}
		relPath, err = makeRelativePath(gitRoot, absPath)
		if err != nil {
			ui.ShowError(err.Error())
			return err
		}
	} else {
		if whatFlag != "" && whyFlag != "" {
			ui.ShowError("--file is required when using --what and --why")
			return fmt.Errorf("--file is required when using --what and --why")
		}

		// Interactive file selection.
		files, err := git.GetRecentlyModifiedFiles(gitRoot, 2*time.Hour)
		if err != nil {
			ui.ShowError("could not list modified files: " + err.Error())
			return err
		}
		options := buildFileOptions(files, gitRoot)
		selected, err := ui.SelectFile(options)
		if err != nil {
			ui.ShowInfo("cancelled")
			return nil // Clean exit on cancel.
		}

		// The selected path might be absolute (manual entry) or relative.
		relPath, err = makeRelativePath(gitRoot, selected)
		if err != nil {
			ui.ShowError(err.Error())
			return err
		}
	}

	// Step 3 — Determine note type.
	dbPath := store.DBPath(cfg.Project.ID)
	db, err := store.Open(dbPath)
	if err != nil {
		ui.ShowError("could not open database: " + err.Error())
		return err
	}
	defer func() { _ = db.Close() }()

	synthStore := store.NewSQLiteStore(db)
	ctx := context.Background()

	existing, err := synthStore.GetFileRegistry(ctx, cfg.Project.ID, relPath)
	if err != nil {
		ui.ShowError("could not check file registry: " + err.Error())
		return err
	}

	if existing != nil {
		// Existing file flow — Day 3.
		// Non-interactive check for existing file
		if whatFlag != "" && whyFlag != "" {
			branch, _ := git.GetCurrentBranch(gitRoot)
			diff, _ := git.GetFileDiff(gitRoot, relPath)

			var impact string
			if diff != "" {
				impact = "[diff captured]"
			}

			return saveExistingFileNote(ctx, synthStore, cfg, relPath, branch, whatFlag, whyFlag, impact)
		}

		ui.ShowInfo("change — " + relPath)

		// Prompt 1: What did you change?
		what := whatFlag
		var err error
		if what == "" {
			what, err = ui.AskSingleLine("What did you change?", "")
			if err != nil {
				return nil
			}
		}

		// Prompt 2: Why did you make this change?
		why := whyFlag
		if why == "" {
			why, err = ui.AskSingleLine("Why did you make this change?", "")
			if err != nil {
				return nil
			}
		}

		// Prompt 3: Impact (optional, skip if --quick).
		var impact string
		if !quick {
			impact, err = ui.AskOptionalLine("Does this affect anything else?")
			if err != nil {
				return nil
			}
		}

		// Step 4 — Enrich with git context
		branch, _ := git.GetCurrentBranch(gitRoot)
		diff, _ := git.GetFileDiff(gitRoot, relPath)

		if impact == "" && diff != "" {
			impact = "[diff captured]"
		}

		return saveExistingFileNote(ctx, synthStore, cfg, relPath, branch, what, why, impact)
	}

	// Step 3A — New file prompt flow.
	// Non-interactive check for new file
	if whatFlag != "" && whyFlag != "" {
		return saveNewFileNote(synthStore, cfg, gitRoot, relPath, whatFlag, "", "", "")
	}

	ui.ShowInfo("new file — " + relPath)

	// Prompt 1: Purpose.
	purpose, err := ui.AskSingleLine("What is the purpose of this file?", "")
	if err != nil {
		return nil // Cancelled — exit cleanly.
	}

	// Prompt 2: Owns.
	owns, err := ui.AskSingleLine("What does this file own?", "its responsibilities")
	if err != nil {
		return nil // Cancelled — exit cleanly.
	}

	// Prompt 3: Boundary.
	boundary, err := ui.AskSingleLine("What does this file NOT own?", "its boundaries")
	if err != nil {
		return nil // Cancelled — exit cleanly.
	}

	// Prompt 4: Impact (optional, skip if --quick).
	var impact string
	if !quick {
		impact, err = ui.AskOptionalLine("Does this affect anything else?")
		if err != nil {
			return nil // Cancelled — exit cleanly.
		}
	}

	// Delegate to testable function.
	return saveNewFileNote(
		synthStore, cfg, gitRoot, relPath,
		purpose, owns, boundary, impact,
	)
}

// saveNewFileNote writes a new file intent and registry entry to the database.
// This is the testable core logic separated from interactive prompts.
func saveNewFileNote(
	synthStore *store.SQLiteStore,
	cfg *config.ProjectConfig,
	gitRoot, relPath,
	purpose, owns, boundary, impact string,
) error {
	ctx := context.Background()

	// Step 4 — Enrich with git context.
	branch, _ := git.GetCurrentBranch(gitRoot)

	// Step 5 — Build and write records.
	intent := buildNewFileIntent(
		cfg.Project.ID, relPath, branch,
		cfg.Developer.Name, purpose, impact,
	)

	entry := buildFileEntry(
		cfg.Project.ID, relPath, cfg.Developer.Name,
		purpose, owns, boundary,
	)

	if err := synthStore.InsertIntent(ctx, intent); err != nil {
		ui.ShowError("failed to save note: " + err.Error())
		return err
	}

	if err := synthStore.UpsertFileRegistry(ctx, entry); err != nil {
		ui.ShowError("failed to save note: " + err.Error())
		return err
	}

	// Step 6 — Confirmation.
	ui.ShowSuccess("note saved  ·  " + relPath + "  ·  " + branch)

	return nil
}

// saveExistingFileNote writes a change intent to the database.
func saveExistingFileNote(
	ctx context.Context,
	synthStore store.Store,
	cfg *config.ProjectConfig,
	filePath, branch,
	what, why, impact string,
) error {
	intent := buildExistingFileIntent(
		cfg.Project.ID, filePath, branch,
		cfg.Developer.Name, what, why, impact,
	)

	if err := synthStore.InsertIntent(ctx, intent); err != nil {
		ui.ShowError("failed to save note: " + err.Error())
		return err
	}

	ui.ShowSuccess("note saved  ·  " + filePath + "  ·  " + branch)

	return nil
}

// ---------------------------------------------------------------------------
// Testable pure functions
// ---------------------------------------------------------------------------

// buildFileOptions converts a []string of relative file paths into
// []ui.FileOption with human-readable ModifiedAgo by checking file ModTime.
func buildFileOptions(files []string, gitRoot string) []ui.FileOption {
	options := make([]ui.FileOption, 0, len(files))
	now := time.Now()

	for _, f := range files {
		absPath := filepath.Join(gitRoot, f)
		info, err := os.Stat(absPath)

		var modAgo string
		if err == nil {
			modAgo = formatDurationAgo(now.Sub(info.ModTime()))
		} else {
			modAgo = "unknown"
		}

		options = append(options, ui.FileOption{
			Path:        f,
			ModifiedAgo: modAgo,
		})
	}

	return options
}

// formatDurationAgo returns a human-readable duration string for file ages.
//
//	< 60 seconds: "just now"
//	< 60 minutes: "4m ago"
//	< 24 hours:   "2h ago"
//	>= 24 hours:  "1d ago"
func formatDurationAgo(d time.Duration) string {
	if d < time.Minute {
		return "just now"
	}
	if d < time.Hour {
		return fmt.Sprintf("%dm ago", int(d.Minutes()))
	}
	if d < 24*time.Hour {
		return fmt.Sprintf("%dh ago", int(d.Hours()))
	}
	return fmt.Sprintf("%dd ago", int(d.Hours()/24))
}

// makeRelativePath resolves filePath to an absolute path, then makes it
// relative to gitRoot. Returns an error if the file is outside gitRoot.
func makeRelativePath(gitRoot, filePath string) (string, error) {
	absPath, err := filepath.Abs(filePath)
	if err != nil {
		return "", fmt.Errorf("resolving path: %w", err)
	}

	absRoot, err := filepath.Abs(gitRoot)
	if err != nil {
		return "", fmt.Errorf("resolving git root: %w", err)
	}

	rel, err := filepath.Rel(absRoot, absPath)
	if err != nil {
		return "", fmt.Errorf("making path relative: %w", err)
	}

	// Check if the file is outside gitRoot (relative path starts with "..").
	if strings.HasPrefix(rel, "..") {
		return "", fmt.Errorf("file %s is outside the git repository", filePath)
	}

	return rel, nil
}

// buildNewFileIntent constructs an Intent struct for a new file declaration.
// Pure function — no I/O.
func buildNewFileIntent(
	projectID, filePath, branch, developer, purpose, impact string,
) types.Intent {
	entropy := rand.New(rand.NewSource(time.Now().UnixNano()))
	id := ulid.MustNew(ulid.Timestamp(time.Now()), entropy)

	return types.Intent{
		ID:         id.String(),
		ProjectID:  projectID,
		FilePath:   filePath,
		Branch:     branch,
		CommitHash: "",
		Developer:  developer,
		Timestamp:  time.Now(),
		Type:       types.IntentNewFile,
		What:       purpose,
		Why:        "new file declaration",
		Impact:     impact,
		Context:    types.ContextNormal,
	}
}

// buildExistingFileIntent constructs an Intent struct for an existing file change.
// Pure function — no I/O.
func buildExistingFileIntent(
	projectID, filePath, branch, developer,
	what, why, impact string,
) types.Intent {
	entropy := rand.New(rand.NewSource(time.Now().UnixNano()))
	id := ulid.MustNew(ulid.Timestamp(time.Now()), entropy)

	return types.Intent{
		ID:         id.String(),
		ProjectID:  projectID,
		FilePath:   filePath,
		Branch:     branch,
		CommitHash: "",
		Developer:  developer,
		Timestamp:  time.Now(),
		Type:       types.IntentChange,
		What:       what,
		Why:        why,
		Impact:     impact,
		Context:    types.ContextNormal,
	}
}

// buildFileEntry constructs a FileEntry struct for a new file.
// Pure function — no I/O.
func buildFileEntry(
	projectID, filePath, developer, purpose, owns, boundary string,
) types.FileEntry {
	return types.FileEntry{
		FilePath:  filePath,
		ProjectID: projectID,
		Purpose:   purpose,
		Owns:      owns,
		Boundary:  boundary,
		CreatedBy: developer,
		CreatedAt: time.Now(),
	}
}
