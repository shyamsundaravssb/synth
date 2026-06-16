package config

import (
	"os"
	"path/filepath"

	"github.com/BurntSushi/toml"
)

// ---------------------------------------------------------------------------
// Project config — lives at <git-root>/.synth/config.toml
// ---------------------------------------------------------------------------

// ProjectConfig holds per-project configuration.
type ProjectConfig struct {
	Project   ProjectSection   `toml:"project"`
	Developer DeveloperSection `toml:"developer"`
	Behavior  BehaviorSection  `toml:"behavior"`
	Sync      SyncSection      `toml:"sync"`
}

// ProjectSection identifies the project.
type ProjectSection struct {
	ID      string `toml:"id"`
	Name    string `toml:"name"`
	Created string `toml:"created"`
}

// DeveloperSection holds per-project developer identity.
type DeveloperSection struct {
	Name string `toml:"name"`
}

// BehaviorSection controls runtime heuristics.
type BehaviorSection struct {
	LowContextThreshold int `toml:"low_context_threshold"`
}

// SyncSection controls team sync behavior.
type SyncSection struct {
	ServerURL     string `toml:"server_url"`
	IntervalHours int    `toml:"interval_hours"`
}

// DefaultProjectConfig returns a ProjectConfig with sensible defaults.
func DefaultProjectConfig() *ProjectConfig {
	return &ProjectConfig{
		Behavior: BehaviorSection{
			LowContextThreshold: 5,
		},
		Sync: SyncSection{
			IntervalHours: 6,
		},
	}
}

// projectConfigPath returns the path to the project config file.
func projectConfigPath(gitRoot string) string {
	return filepath.Join(gitRoot, ".synth", "config.toml")
}

// LoadProjectConfig loads the project configuration from
// <gitRoot>/.synth/config.toml. If the file does not exist, it returns
// a default config with no error.
func LoadProjectConfig(gitRoot string) (*ProjectConfig, error) {
	cfg := DefaultProjectConfig()
	path := projectConfigPath(gitRoot)

	if _, err := os.Stat(path); os.IsNotExist(err) {
		return cfg, nil
	}

	if _, err := toml.DecodeFile(path, cfg); err != nil {
		return nil, err
	}

	return cfg, nil
}

// SaveProjectConfig writes the project configuration to
// <gitRoot>/.synth/config.toml, creating the .synth directory if needed.
func SaveProjectConfig(gitRoot string, cfg *ProjectConfig) error {
	dir := filepath.Join(gitRoot, ".synth")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}

	f, err := os.Create(projectConfigPath(gitRoot))
	if err != nil {
		return err
	}
	defer func() { _ = f.Close() }()

	return toml.NewEncoder(f).Encode(cfg)
}

// ---------------------------------------------------------------------------
// Global config — lives at <globalDir>/global.toml  (default: ~/.synth/)
// ---------------------------------------------------------------------------

// GlobalConfig holds user-level configuration.
type GlobalConfig struct {
	User UserSection `toml:"user"`
}

// UserSection holds global developer identity.
type UserSection struct {
	Name  string `toml:"name"`
	Email string `toml:"email"`
}

// DefaultGlobalConfig returns a GlobalConfig with sensible defaults.
func DefaultGlobalConfig() *GlobalConfig {
	return &GlobalConfig{}
}

// DefaultGlobalDir returns the default global config directory (~/.synth).
func DefaultGlobalDir() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".synth")
}

// resolveGlobalDir returns the first non-empty override, falling back to
// DefaultGlobalDir(). This makes every global function testable without
// touching the real home directory.
func resolveGlobalDir(overrides []string) string {
	if len(overrides) > 0 && overrides[0] != "" {
		return overrides[0]
	}
	return DefaultGlobalDir()
}

// LoadGlobalConfig loads the global configuration from
// <globalDir>/global.toml. If the file does not exist, it returns a
// default config with no error.
//
// Pass an optional globalDir to override the default ~/.synth directory
// (useful for testing).
func LoadGlobalConfig(globalDir ...string) (*GlobalConfig, error) {
	dir := resolveGlobalDir(globalDir)
	cfg := DefaultGlobalConfig()
	path := filepath.Join(dir, "global.toml")

	if _, err := os.Stat(path); os.IsNotExist(err) {
		return cfg, nil
	}

	if _, err := toml.DecodeFile(path, cfg); err != nil {
		return nil, err
	}

	return cfg, nil
}

// SaveGlobalConfig writes the global configuration to
// <globalDir>/global.toml, creating the directory if needed.
//
// Pass an optional globalDir to override the default ~/.synth directory
// (useful for testing).
func SaveGlobalConfig(cfg *GlobalConfig, globalDir ...string) error {
	dir := resolveGlobalDir(globalDir)

	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}

	f, err := os.Create(filepath.Join(dir, "global.toml"))
	if err != nil {
		return err
	}
	defer func() { _ = f.Close() }()

	return toml.NewEncoder(f).Encode(cfg)
}

// EnsureGlobalDirs creates the global config directory and its projects
// subdirectory. This function is idempotent — safe to call multiple times.
//
// Pass an optional globalDir to override the default ~/.synth directory
// (useful for testing).
func EnsureGlobalDirs(globalDir ...string) error {
	dir := resolveGlobalDir(globalDir)

	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	return os.MkdirAll(filepath.Join(dir, "projects"), 0o755)
}
