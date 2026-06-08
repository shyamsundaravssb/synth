package daemon

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestLaunchdPlistContent_ContainsExecPath(t *testing.T) {
	cfg := ServiceConfig{
		ExecPath: "/usr/bin/synth",
		LogFile:  "/tmp/test.log",
	}
	content := LaunchdPlistContent(cfg)

	if !strings.Contains(content, "/usr/bin/synth") {
		t.Errorf("expected content to contain /usr/bin/synth")
	}
	if !strings.Contains(content, "--daemon-child") {
		t.Errorf("expected content to contain --daemon-child")
	}
	if !strings.Contains(content, "com.synth.daemon") {
		t.Errorf("expected content to contain com.synth.daemon")
	}
	if !strings.Contains(content, "<key>RunAtLoad</key>") {
		t.Errorf("expected content to contain RunAtLoad")
	}
	if !strings.Contains(content, "<key>KeepAlive</key>") {
		t.Errorf("expected content to contain KeepAlive")
	}
}

func TestLaunchdPlistContent_ContainsLogFile(t *testing.T) {
	cfg := ServiceConfig{
		ExecPath: "/usr/bin/synth",
		LogFile:  "/tmp/test.log",
	}
	content := LaunchdPlistContent(cfg)

	if !strings.Contains(content, "/tmp/test.log") {
		t.Errorf("expected content to contain /tmp/test.log")
	}
}

func TestSystemdUnitContent_ContainsExecPath(t *testing.T) {
	cfg := ServiceConfig{
		ExecPath: "/usr/bin/synth",
		LogFile:  "/tmp/test.log",
	}
	content := SystemdUnitContent(cfg)

	if !strings.Contains(content, "/usr/bin/synth") {
		t.Errorf("expected content to contain /usr/bin/synth")
	}
	if !strings.Contains(content, "--daemon-child") {
		t.Errorf("expected content to contain --daemon-child")
	}
	if !strings.Contains(content, "Restart=on-failure") {
		t.Errorf("expected content to contain Restart=on-failure")
	}
	if !strings.Contains(content, "WantedBy=default.target") {
		t.Errorf("expected content to contain WantedBy=default.target")
	}
}

func TestSystemdUnitContent_ContainsLogFile(t *testing.T) {
	cfg := ServiceConfig{
		ExecPath: "/usr/bin/synth",
		LogFile:  "/tmp/test.log",
	}
	content := SystemdUnitContent(cfg)

	if !strings.Contains(content, "/tmp/test.log") {
		t.Errorf("expected content to contain /tmp/test.log")
	}
}

func TestCurrentPlatform_ReturnsKnownValue(t *testing.T) {
	plat := CurrentPlatform()
	if plat != "linux" && plat != "darwin" && plat != "unsupported" {
		t.Errorf("expected linux, darwin, or unsupported, got %s", plat)
	}

	if runtime.GOOS == "linux" && plat != "linux" {
		t.Errorf("expected linux, got %s", plat)
	}
	if runtime.GOOS == "darwin" && plat != "darwin" {
		t.Errorf("expected darwin, got %s", plat)
	}
}

func TestNewServiceConfig_PopulatesFields(t *testing.T) {
	cfg, err := NewServiceConfig()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.ExecPath == "" {
		t.Errorf("expected non-empty ExecPath")
	}
	if !filepath.IsAbs(cfg.ExecPath) {
		t.Errorf("expected absolute ExecPath, got %s", cfg.ExecPath)
	}
	if cfg.User == "" {
		t.Errorf("expected non-empty User")
	}
	if cfg.LogFile != LogFile {
		t.Errorf("expected LogFile %s, got %s", LogFile, cfg.LogFile)
	}
	if cfg.PIDFile != PIDFile {
		t.Errorf("expected PIDFile %s, got %s", PIDFile, cfg.PIDFile)
	}
}

func TestInstallSystemdService_WritesFile(t *testing.T) {
	if runtime.GOOS == "darwin" {
		t.Skip("skipping systemd test on darwin")
	}

	tempPath := filepath.Join(t.TempDir(), "synth-daemon.service")
	content := "test content"

	err := writeServiceFile(tempPath, content)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	data, err := os.ReadFile(tempPath)
	if err != nil {
		t.Fatalf("failed to read file: %v", err)
	}

	if string(data) != content {
		t.Errorf("expected content %q, got %q", content, string(data))
	}
}

func TestInstallLaunchdService_WritesFile(t *testing.T) {
	if runtime.GOOS == "linux" {
		t.Skip("skipping launchd test on linux")
	}

	tempPath := filepath.Join(t.TempDir(), "com.synth.daemon.plist")
	content := "test content"

	err := writeServiceFile(tempPath, content)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	data, err := os.ReadFile(tempPath)
	if err != nil {
		t.Fatalf("failed to read file: %v", err)
	}

	if string(data) != content {
		t.Errorf("expected content %q, got %q", content, string(data))
	}
}
