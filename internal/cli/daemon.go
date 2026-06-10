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
	"github.com/shyamsundaravssb/synth/internal/ipc"
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
	daemonCmd.AddCommand(newDaemonPingCmd())
	daemonCmd.AddCommand(newDaemonInstallServiceCmd())
	daemonCmd.AddCommand(newDaemonUninstallServiceCmd())

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
			var statusData *ipc.StatusData

			if running {
				client := ipc.NewClient(daemon.SockFile)
				req, _ := ipc.NewRequest(ipc.TypeStatus, ipc.StatusPayload{})
				resp, err := client.Send(req)
				if err == nil && resp.Status == ipc.StatusOK {
					sd, parseErr := ipc.ParseStatusData(resp)
					if parseErr == nil {
						statusData = sd
						pid = sd.PID
					}
				}
			}

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
				if statusData != nil {
					data["uptime_seconds"] = statusData.UptimeS
					data["notes_count"] = statusData.NotesCount
					data["file_saves_count"] = statusData.FileSavesCount
				}
				out, _ := json.Marshal(data)
				_, _ = fmt.Println(string(out))
			} else {
				if running {
					_, _ = fmt.Printf("● Synth daemon  ·  running  ·  pid %d\n", pid)
					if statusData != nil {
						_, _ = fmt.Printf("· uptime:      %d seconds\n", statusData.UptimeS)
						_, _ = fmt.Printf("· notes:       %d intent notes\n", statusData.NotesCount)
						_, _ = fmt.Printf("· file saves:  %d saves tracked\n", statusData.FileSavesCount)
					}
					_, _ = fmt.Printf("· socket:      %s\n", daemon.SockFile)
					_, _ = fmt.Printf("· log:         %s\n", daemon.LogFile)
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

func newDaemonInstallServiceCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "install-service",
		Short: "Install Synth daemon as a system service",
		Long: "Installs a launchd service (macOS) or systemd user service (Linux) " +
			"so the Synth daemon starts automatically on login and restarts on crash.",
		Args: cobra.NoArgs,
		Run: func(cmd *cobra.Command, args []string) {
			ui.ShowInfo("installing Synth daemon service...")
			if err := daemon.InstallService(); err != nil {
				ui.ShowError("failed to install service: " + err.Error())
				os.Exit(1)
			}
			ui.ShowSuccess("service installed")

			if daemon.CurrentPlatform() == "linux" {
				ui.ShowInfo("start now with: systemctl --user start synth-daemon.service")
				ui.ShowInfo("check status with: systemctl --user status synth-daemon.service")
			} else if daemon.CurrentPlatform() == "darwin" {
				ui.ShowInfo("the service will start automatically on next login")
				ui.ShowInfo("start now with: synth daemon start")
			}
		},
	}
}

func newDaemonUninstallServiceCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "uninstall-service",
		Short: "Remove the Synth daemon system service",
		Args:  cobra.NoArgs,
		Run: func(cmd *cobra.Command, args []string) {
			running, _, _ := daemon.IsDaemonRunning(daemon.PIDFile)
			if running {
				ui.ShowInfo("stopping daemon before uninstalling service...")
				_ = runDaemonStop()
			}

			if err := daemon.UninstallService(); err != nil {
				ui.ShowError("failed to uninstall service: " + err.Error())
				os.Exit(1)
			}
			ui.ShowSuccess("service uninstalled")
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

func newDaemonPingCmd() *cobra.Command {
	var jsonOutput bool

	cmd := &cobra.Command{
		Use:   "ping",
		Short: "Ping the Synth daemon",
		Long:  "Send a ping to the running daemon and report the round-trip time.",
		Args:  cobra.NoArgs,
		Run: func(cmd *cobra.Command, args []string) {
			client := ipc.NewClient(daemon.SockFile)

			start := time.Now()
			pingData, err := client.Ping()
			elapsed := time.Since(start)

			if err != nil {
				ui.ShowError("daemon is not reachable: " + err.Error())
				ui.ShowInfo("start the daemon with: synth daemon start")
				os.Exit(1)
			}

			if jsonOutput {
				data := map[string]interface{}{
					"status":     "ok",
					"pid":        pingData.PID,
					"version":    pingData.Version,
					"elapsed_ms": elapsed.Milliseconds(),
				}
				out, _ := json.Marshal(data)
				_, _ = fmt.Println(string(out))
			} else {
				_, _ = fmt.Printf("● pong  ·  pid %d  ·  %dms\n", pingData.PID, elapsed.Milliseconds())
				_, _ = fmt.Printf("· version: %s\n", pingData.Version)
			}
			os.Exit(0)
		},
	}

	cmd.Flags().BoolVar(&jsonOutput, "json", false, "output as JSON")
	return cmd
}
