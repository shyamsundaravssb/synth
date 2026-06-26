package cli

import (
	"context"
	"fmt"

	"github.com/shyamsundaravssb/synth/internal/embed"
	"github.com/shyamsundaravssb/synth/internal/ui"
	"github.com/spf13/cobra"
)

func newModelCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "model",
		Short: "Manage the local embedding model",
	}

	var force bool
	downloadCmd := &cobra.Command{
		Use:   "download",
		Short: "Download the embedding model for semantic search",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runModelDownload(embed.DefaultModelDir(), force)
		},
	}
	downloadCmd.Flags().BoolVar(&force, "force", false, "re-download even if already present")

	statusCmd := &cobra.Command{
		Use:   "status",
		Short: "Check whether the embedding model is installed",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runModelStatus(embed.DefaultModelDir())
		},
	}

	cmd.AddCommand(downloadCmd)
	cmd.AddCommand(statusCmd)
	return cmd
}

func runModelDownload(modelDir string, force bool) error {
	if !force && embed.IsModelComplete(modelDir) {
		ui.ShowSuccess("model already downloaded at " + modelDir)
		ui.ShowInfo("use --force to re-download")
		return nil
	}

	ui.ShowInfo("downloading embedding model (~87MB, one-time download)...")

	lastPrintedPercent := make(map[string]int)

	progressFn := func(fileName string, downloaded, total int64) {
		if total > 0 {
			pct := int((downloaded * 100) / total)
			last, exists := lastPrintedPercent[fileName]
			if !exists || pct-last >= 10 || pct == 100 {
				lastPrintedPercent[fileName] = pct
				fmt.Printf("  %-25s %3d%% (%.1f MB / %.1f MB)\n", fileName, pct, float64(downloaded)/(1024*1024), float64(total)/(1024*1024))
			}
		} else {
			last, exists := lastPrintedPercent[fileName]
			kb := int(downloaded / 1024)
			if !exists || kb-last >= 1024 {
				lastPrintedPercent[fileName] = kb
				fmt.Printf("  %-25s %d KB downloaded\n", fileName, kb)
			}
		}
	}

	ctx := context.Background()
	if err := embed.DownloadModel(ctx, modelDir, force, progressFn); err != nil {
		ui.ShowError("model download failed: " + err.Error())
		return err
	}

	ui.ShowSuccess("model downloaded successfully — semantic search is now available")
	ui.ShowInfo("restart the daemon to use it: synth daemon restart")
	return nil
}

func runModelStatus(modelDir string) error {
	if embed.IsModelComplete(modelDir) {
		ui.ShowSuccess("model installed at " + modelDir)
	} else {
		ui.ShowInfo("model not installed")
		ui.ShowInfo("run 'synth model download' to enable semantic search")
	}
	return nil
}
