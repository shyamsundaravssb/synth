package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"strconv"

	"github.com/spf13/cobra"
	"github.com/shyamsundaravssb/synth/internal/daemon"
	"github.com/shyamsundaravssb/synth/internal/ui"
)

func newDaemonCmd() *cobra.Command {
	daemonCmd := &cobra.Command{
		Use:   "daemon",
		Short: "Manage the Synth background daemon",
		Long:  "Start, stop, restart, and check the status\nof the Synth background daemon process.",
	}

	daemonCmd.AddCommand(newDaemonStartCmd())
	daemonCmd.AddCommand(newDaemonStopCmd())
	daemonCmd.AddCommand(newDaemonStatusCmd())
	daemonCmd.AddCommand(newDaemonRestartCmd())

	return daemonCmd
}

func newDaemonStartCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "start",
		Short: "Start the Synth daemon",
		Run: func(cmd *cobra.Command, args []string) {
			running, pid, _ := daemon.IsDaemonRunning(daemon.PIDFile)
			if running {
				ui.ShowError("daemon is already running (pid: " + strconv.Itoa(pid) + ")")
				os.Exit(1)
			}
			ui.ShowInfo("daemon start: not yet implemented")
			os.Exit(0)
		},
	}
}

func newDaemonStopCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "stop",
		Short: "Stop the Synth daemon",
		Run: func(cmd *cobra.Command, args []string) {
			ui.ShowInfo("daemon stop: not yet implemented")
			os.Exit(0)
		},
	}
}

func newDaemonStatusCmd() *cobra.Command {
	var jsonOutput bool

	cmd := &cobra.Command{
		Use:   "status",
		Short: "Show daemon status",
		Run: func(cmd *cobra.Command, args []string) {
			running, pid, _ := daemon.IsDaemonRunning(daemon.PIDFile)
			
			if jsonOutput {
				status := "stopped"
				if running {
					status = "running"
				}
				data := map[string]interface{}{
					"status": status,
					"pid":    pid,
					"socket": daemon.SockFile,
					"log":    daemon.LogFile,
				}
				out, _ := json.Marshal(data)
				fmt.Println(string(out))
			} else {
				if running {
					fmt.Printf("● Synth daemon  ·  running  ·  pid %d\n", pid)
					fmt.Printf("· socket: %s\n", daemon.SockFile)
					fmt.Printf("· log:    %s\n", daemon.LogFile)
				} else {
					fmt.Println("○ Synth daemon  ·  stopped")
					fmt.Println("· Start with: synth daemon start")
				}
			}
			
			if running {
				os.Exit(0)
			} else {
				os.Exit(1)
			}
		},
	}
	
	cmd.Flags().BoolVar(&jsonOutput, "json", false, "output as JSON")
	return cmd
}

func newDaemonRestartCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "restart",
		Short: "Restart the Synth daemon",
		Run: func(cmd *cobra.Command, args []string) {
			ui.ShowInfo("daemon restart: not yet implemented")
			os.Exit(0)
		},
	}
}
