package cli

import (
	"context"
	"fmt"
	"os"

	"github.com/shyamsundaravssb/synth/internal/config"
	"github.com/shyamsundaravssb/synth/internal/git"
	"github.com/shyamsundaravssb/synth/internal/store"
	"github.com/spf13/cobra"
)

func newPostCommitCmd() *cobra.Command {
	var hashFlag string
	var quietFlag bool

	cmd := &cobra.Command{
		Use:    "_post-commit",
		Short:  "Internal: called by git post-commit hook",
		Hidden: true,
		Run: func(cmd *cobra.Command, args []string) {
			// This command must NEVER exit with a non-zero code.
			_ = runPostCommit(hashFlag, quietFlag)
		},
	}

	cmd.Flags().StringVar(&hashFlag, "hash", "", "commit hash from git")
	cmd.Flags().BoolVar(&quietFlag, "quiet", false, "suppress all output")

	return cmd
}

func runPostCommit(hash string, quiet bool) error {
	if hash == "" {
		return nil
	}

	cwd, err := os.Getwd()
	if err != nil {
		return nil // exit 0 silently
	}

	gitRoot, err := git.FindGitRoot(cwd)
	if err != nil {
		return nil
	}

	cfg, err := config.LoadProjectConfig(gitRoot)
	if err != nil || cfg == nil || cfg.Project.ID == "" {
		return nil
	}

	dbPath := store.DBPath(cfg.Project.ID)
	db, err := store.Open(dbPath)
	if err != nil {
		return nil
	}
	defer func() { _ = db.Close() }()

	synthStore := store.NewSQLiteStore(db)
	ctx := context.Background()

	rows, err := linkCommitHash(ctx, synthStore, cfg.Project.ID, hash)
	if err != nil {
		return nil
	}

	if rows > 0 && !quiet {
		shortHash := hash
		if len(hash) >= 8 {
			shortHash = hash[:8]
		}
		fmt.Printf("synth: linked %d note(s) to commit %s\n", rows, shortHash)
	}

	return nil
}

func linkCommitHash(ctx context.Context, synthStore store.Store, projectID, commitHash string) (int, error) {
	return synthStore.UpdateUncommittedIntents(ctx, projectID, commitHash)
}
