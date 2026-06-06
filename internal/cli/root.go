package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

// NewRootCmd creates and returns the root Cobra command for Synth.
// The version string is injected from main via ldflags.
func NewRootCmd(version string) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "synth",
		Short: "Synth — the intent layer for Git",
		Long: `Synth is a system-level developer CLI tool that sits on top of Git.
It captures developer intent — what changed, why it changed, by whom,
and in what context — and uses that information to make collaborative
development smarter.`,
		SilenceUsage:  true,
		SilenceErrors: true,
		Version:       version,
	}

	// Set custom version template.
	cmd.SetVersionTemplate(fmt.Sprintf("synth version %s\n", version))

	// Register subcommands.
	cmd.AddCommand(newInitCmd())
	cmd.AddCommand(newNoteCmd())
	cmd.AddCommand(newLogCmd())
	cmd.AddCommand(newStatusCmd())
	cmd.AddCommand(newPostCommitCmd())

	return cmd
}
