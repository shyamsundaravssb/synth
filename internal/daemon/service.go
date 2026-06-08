package daemon

import (
	"fmt"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"runtime"
	"strings"
)

func CurrentPlatform() string {
	if runtime.GOOS == "linux" || runtime.GOOS == "darwin" {
		return runtime.GOOS
	}
	return "unsupported"
}

type ServiceConfig struct {
	ExecPath string
	User     string
	LogFile  string
	PIDFile  string
}

func NewServiceConfig() (*ServiceConfig, error) {
	execPath, err := os.Executable()
	if err != nil {
		return nil, fmt.Errorf("failed to get executable path: %w", err)
	}

	username := os.Getenv("USER")
	if username == "" {
		u, err := user.Current()
		if err == nil {
			username = u.Username
		}
	}

	return &ServiceConfig{
		ExecPath: execPath,
		User:     username,
		LogFile:  LogFile,
		PIDFile:  PIDFile,
	}, nil
}

func LaunchdPlistContent(cfg ServiceConfig) string {
	template := `<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
    <key>Label</key>
    <string>com.synth.daemon</string>
    <key>ProgramArguments</key>
    <array>
        <string>{ExecPath}</string>
        <string>--daemon-child</string>
    </array>
    <key>RunAtLoad</key>
    <true/>
    <key>KeepAlive</key>
    <true/>
    <key>StandardOutPath</key>
    <string>{LogFile}</string>
    <key>StandardErrorPath</key>
    <string>{LogFile}</string>
</dict>
</plist>`

	content := strings.ReplaceAll(template, "{ExecPath}", cfg.ExecPath)
	content = strings.ReplaceAll(content, "{LogFile}", cfg.LogFile)
	return content
}

func writeServiceFile(path, content string) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	return os.WriteFile(path, []byte(content), 0644)
}

func InstallLaunchdService(cfg ServiceConfig) error {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return err
	}
	plistPath := filepath.Join(homeDir, "Library", "LaunchAgents", "com.synth.daemon.plist")

	content := LaunchdPlistContent(cfg)
	if err := writeServiceFile(plistPath, content); err != nil {
		return err
	}

	cmd := exec.Command("launchctl", "load", plistPath)
	return cmd.Run()
}

func UninstallLaunchdService() error {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return err
	}
	plistPath := filepath.Join(homeDir, "Library", "LaunchAgents", "com.synth.daemon.plist")

	if _, err := os.Stat(plistPath); os.IsNotExist(err) {
		return nil
	}

	_ = exec.Command("launchctl", "unload", plistPath).Run()

	return os.Remove(plistPath)
}

func SystemdUnitContent(cfg ServiceConfig) string {
	template := `[Unit]
Description=Synth Daemon
After=default.target

[Service]
Type=simple
ExecStart={ExecPath} --daemon-child
Restart=on-failure
RestartSec=5
StandardOutput=append:{LogFile}
StandardError=append:{LogFile}

[Install]
WantedBy=default.target`

	content := strings.ReplaceAll(template, "{ExecPath}", cfg.ExecPath)
	content = strings.ReplaceAll(content, "{LogFile}", cfg.LogFile)
	return content
}

func InstallSystemdService(cfg ServiceConfig) error {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return err
	}
	unitPath := filepath.Join(homeDir, ".config", "systemd", "user", "synth-daemon.service")

	content := SystemdUnitContent(cfg)
	if err := writeServiceFile(unitPath, content); err != nil {
		return err
	}

	if err := exec.Command("systemctl", "--user", "daemon-reload").Run(); err != nil {
		return err
	}

	return exec.Command("systemctl", "--user", "enable", "synth-daemon.service").Run()
}

func UninstallSystemdService() error {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return err
	}
	unitPath := filepath.Join(homeDir, ".config", "systemd", "user", "synth-daemon.service")

	if _, err := os.Stat(unitPath); os.IsNotExist(err) {
		return nil
	}

	_ = exec.Command("systemctl", "--user", "disable", "synth-daemon.service").Run()
	_ = exec.Command("systemctl", "--user", "daemon-reload").Run()

	return os.Remove(unitPath)
}

func InstallService() error {
	switch CurrentPlatform() {
	case "darwin":
		cfg, err := NewServiceConfig()
		if err != nil {
			return err
		}
		return InstallLaunchdService(*cfg)
	case "linux":
		cfg, err := NewServiceConfig()
		if err != nil {
			return err
		}
		return InstallSystemdService(*cfg)
	default:
		return fmt.Errorf("service installation is not supported on this platform (%s)", runtime.GOOS)
	}
}

func UninstallService() error {
	switch CurrentPlatform() {
	case "darwin":
		return UninstallLaunchdService()
	case "linux":
		return UninstallSystemdService()
	default:
		return fmt.Errorf("service uninstallation is not supported on this platform (%s)", runtime.GOOS)
	}
}
