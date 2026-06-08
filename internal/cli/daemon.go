package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"syscall"
	"time"

	"github.com/shyamsundaravssb/synth/internal/daemon"
	"github.com/shyamsundaravssb/synth/internal/ui"
	"github.com/spf13/cobra"
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
			if err := runDaemonStart(); err != nil {
				ui.ShowError(err.Error())
				os.Exit(1)
			}
		},
	}
}

func newDaemonStopCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "stop",
		Short: "Stop the Synth daemon",
		Run: func(cmd *cobra.Command, args []string) {
			if err := runDaemonStop(); err != nil {
				ui.ShowError(err.Error())
				os.Exit(1)
			}
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
				_, _ = fmt.Println(string(out))
			} else {
				if running {
					_, _ = fmt.Printf("● Synth daemon  ·  running  ·  pid %d\n", pid)
					_, _ = fmt.Printf("· socket: %s\n", daemon.SockFile)
					_, _ = fmt.Printf("· log:    %s\n", daemon.LogFile)
				} else {
					_, _ = fmt.Println("○ Synth daemon  ·  stopped")
					_, _ = fmt.Println("· Start with: synth daemon start")
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
			if err := runDaemonRestart(); err != nil {
				ui.ShowError(err.Error())
				os.Exit(1)
			}
		},
	}
}

func runDaemonStart() error {
	running, pid, err := daemon.IsDaemonRunning(daemon.PIDFile)
	if err != nil {
		return fmt.Errorf("failed to check daemon status: %w", err)
	}
	if running {
		ui.ShowError("daemon is already running (pid: " + strconv.Itoa(pid) + ")")
		return nil
	}

	execPath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("cannot find executable: %w", err)
	}

	cmd := exec.Command(execPath, "--daemon-child")
	cmd.Stdin = nil
	cmd.Stdout = nil
	cmd.Stderr = nil
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Setsid: true,
	}

	err = cmd.Start()
	if err != nil {
		return fmt.Errorf("failed to start daemon: %w", err)
	}

	_ = cmd.Process.Release()

	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		running, pid, err = daemon.IsDaemonRunning(daemon.PIDFile)
		if err == nil && running {
			ui.ShowSuccess("daemon started  ·  pid " + strconv.Itoa(pid))
			return nil
		}
		time.Sleep(100 * time.Millisecond)
	}

	return fmt.Errorf("daemon did not start within 3 seconds — check log: %s", daemon.LogFile)
}

func runDaemonStop() error {
	running, pid, err := daemon.IsDaemonRunning(daemon.PIDFile)
	if err != nil {
		return err
	}
	if !running {
		ui.ShowInfo("daemon is not running")
		return nil
	}

	process, err := os.FindProcess(pid)
	if err != nil {
		return fmt.Errorf("cannot find process %d: %w", pid, err)
	}

	err = process.Signal(syscall.SIGTERM)
	if err != nil {
		return fmt.Errorf("failed to send SIGTERM to pid %d: %w", pid, err)
	}

	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		running, _, _ = daemon.IsDaemonRunning(daemon.PIDFile)
		if !running {
			ui.ShowSuccess("daemon stopped")
			return nil
		}
		time.Sleep(100 * time.Millisecond)
	}

	_ = process.Signal(syscall.SIGKILL)
	_ = daemon.RemovePID(daemon.PIDFile)
	ui.ShowSuccess("daemon stopped (forced)")
	return nil
}

func runDaemonRestart() error {
	err := runDaemonStop()
	if err != nil {
		return err
	}

	time.Sleep(200 * time.Millisecond)

	return runDaemonStart()
}
