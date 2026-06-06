package cli

import (
	"fmt"
	"math/rand"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/oklog/ulid/v2"

	"github.com/shyamsundaravssb/synth/internal/config"
	"github.com/shyamsundaravssb/synth/internal/git"
	"github.com/shyamsundaravssb/synth/internal/store"
	"github.com/shyamsundaravssb/synth/internal/ui"
	"github.com/spf13/cobra"
)

// newInitCmd creates the cobra command for 'synth init'.
func newInitCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "init",
		Short: "Initialize Synth in a Git repository",
		Long: `Initialize Synth in the current Git repository.
Creates a local intent store and installs
a Git post-commit hook.`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runInitInteractive()
		},
	}
}

// runInitInteractive handles the interactive prompts and delegates to runInit.
func runInitInteractive() error {
	// Step 1 — Find git root.
	cwd, err := os.Getwd()
	if err != nil {
		ui.ShowError("could not determine current directory: " + err.Error())
		return err
	}

	gitRoot, err := git.FindGitRoot(cwd)
	if err != nil {
		ui.ShowError("not a git repository")
		ui.ShowInfo("run 'git init' first to initialize a git repository")
		return err
	}

	// Step 2 — Check not already initialized.
	configPath := filepath.Join(gitRoot, ".synth", "config.toml")
	if _, err := os.Stat(configPath); err == nil {
		ui.ShowError("synth is already initialized in this repository")
		ui.ShowInfo("run 'synth status' to see current project state")
		return fmt.Errorf("already initialized")
	}

	// Step 3 — Collect project name.
	defaultName := filepath.Base(gitRoot)
	projectName, err := ui.AskSingleLine("Project name", defaultName)
	if err != nil {
		return nil // User cancelled — exit cleanly.
	}

	// Step 4 — Collect developer name.
	defaultDev := getGitUserName(gitRoot)
	var devPrompt string
	if defaultDev != "" {
		devPrompt = defaultDev
	} else {
		devPrompt = ""
	}
	developerName, err := ui.AskSingleLine("Your name", devPrompt)
	if err != nil {
		return nil // User cancelled — exit cleanly.
	}

	// Step 11 — Handle .gitignore (ask before making changes).
	addToGitignore := false
	if !gitignoreContainsSynth(gitRoot) {
		confirmed, confirmErr := ui.Confirm(
			"Add .synth/ to .gitignore? (config.toml will still be committed)",
			true,
		)
		if confirmErr != nil {
			return nil // User cancelled — exit cleanly.
		}
		addToGitignore = confirmed
	}

	// Delegate to testable function.
	return runInit(gitRoot, projectName, developerName, addToGitignore)
}

// runInit performs the actual initialization with known values.
// This function is separated from the interactive prompts so it can be
// tested directly without stdin interaction.
func runInit(gitRoot, projectName, developerName string, addToGitignore bool) error {
	// Step 1 — Verify git root.
	if _, err := git.FindGitRoot(gitRoot); err != nil {
		ui.ShowError("not a git repository")
		ui.ShowInfo("run 'git init' first to initialize a git repository")
		return fmt.Errorf("not a git repository: %w", err)
	}

	// Step 2 — Check not already initialized.
	configPath := filepath.Join(gitRoot, ".synth", "config.toml")
	if _, err := os.Stat(configPath); err == nil {
		ui.ShowError("synth is already initialized in this repository")
		ui.ShowInfo("run 'synth status' to see current project state")
		return fmt.Errorf("already initialized")
	}

	// Step 5 — Generate project ID.
	projectID := generateProjectID()

	// Step 6 — Ensure global dirs.
	if err := config.EnsureGlobalDirs(); err != nil {
		ui.ShowError("could not create global directories: " + err.Error())
		return err
	}

	// Step 7 — Create .synth/ directory.
	synthDir := filepath.Join(gitRoot, ".synth")
	if err := os.MkdirAll(synthDir, 0o755); err != nil {
		ui.ShowError("could not create .synth directory: " + err.Error())
		return err
	}

	// From this point, any failure must clean up .synth/.
	cleanup := func() {
		os.RemoveAll(synthDir)
	}

	// Step 8 — Write .synth/config.toml.
	cfg := &config.ProjectConfig{
		Project: config.ProjectSection{
			ID:      projectID,
			Name:    projectName,
			Created: time.Now().Format("2006-01-02"),
		},
		Developer: config.DeveloperSection{
			Name: developerName,
		},
		Behavior: config.BehaviorSection{
			LowContextThreshold: 3,
		},
		Sync: config.SyncSection{
			ServerURL:     "",
			IntervalHours: 6,
		},
	}
	if err := config.SaveProjectConfig(gitRoot, cfg); err != nil {
		ui.ShowError("could not write config: " + err.Error())
		cleanup()
		return err
	}

	// Step 9 — Open and migrate database.
	dbPath := store.DBPath(projectID)
	db, err := store.Open(dbPath)
	if err != nil {
		ui.ShowError("could not create database: " + err.Error())
		cleanup()
		return err
	}
	db.Close()

	// Step 10 — Install git hook.
	if err := git.InstallPostCommitHook(gitRoot); err != nil {
		ui.ShowInfo("warning: could not install git hook: " + err.Error())
		// Continue — do not abort init for a hook failure.
	}

	// Step 11 — Handle .gitignore.
	if addToGitignore {
		if err := ensureGitignoreEntry(gitRoot); err != nil {
			// Non-fatal: warn but continue.
			ui.ShowInfo("warning: could not update .gitignore: " + err.Error())
		}
	}

	// Step 12 — Print success block.
	fmt.Println()
	ui.ShowSuccess("Synth initialized — " + projectName)
	ui.ShowInfo("Developer:  " + developerName)
	ui.ShowInfo("Project ID: " + projectID)
	ui.ShowInfo("Database:   ~/.synth/projects/" + projectID + "/intent.db")
	ui.ShowInfo("run 'synth note' to capture your first intent.")
	fmt.Println()

	return nil
}

// generateProjectID creates a unique project identifier using ULID.
// Produces a 26-character, uppercase, time-sortable string
// (e.g. 01HXYZ123456789ABCDEFGHJKM).
func generateProjectID() string {
	entropy := rand.New(rand.NewSource(time.Now().UnixNano()))
	id := ulid.MustNew(ulid.Timestamp(time.Now()), entropy)
	return id.String()
}

// getGitUserName retrieves the git config user.name for the repo at gitRoot.
// Returns empty string if not set or on error.
func getGitUserName(gitRoot string) string {
	cmd := exec.Command("git", "-C", gitRoot, "config", "user.name")
	out, err := cmd.Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

// gitignoreContainsSynth checks if the .gitignore already contains ".synth/".
func gitignoreContainsSynth(gitRoot string) bool {
	data, err := os.ReadFile(filepath.Join(gitRoot, ".gitignore"))
	if err != nil {
		return false
	}
	for _, line := range strings.Split(string(data), "\n") {
		if strings.TrimSpace(line) == ".synth/" {
			return true
		}
	}
	return false
}

// ensureGitignoreEntry appends ".synth/" to .gitignore if not already present.
func ensureGitignoreEntry(gitRoot string) error {
	gitignorePath := filepath.Join(gitRoot, ".gitignore")

	data, err := os.ReadFile(gitignorePath)
	if err != nil && !os.IsNotExist(err) {
		return err
	}

	content := string(data)

	// Check if already present.
	for _, line := range strings.Split(content, "\n") {
		if strings.TrimSpace(line) == ".synth/" {
			return nil // Already present.
		}
	}

	// Append .synth/ entry.
	if content != "" && !strings.HasSuffix(content, "\n") {
		content += "\n"
	}
	content += ".synth/\n"

	return os.WriteFile(gitignorePath, []byte(content), 0o644)
}
