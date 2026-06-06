package git

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// SynthHookMarkerStart marks the beginning of the Synth-managed block.
const SynthHookMarkerStart = "# SYNTH HOOK START"

// SynthHookMarkerEnd marks the end of the Synth-managed block.
const SynthHookMarkerEnd = "# SYNTH HOOK END"

// synthHookBlock is the Synth-managed block content (without shebang).
const synthHookBlock = `# SYNTH HOOK START
# Managed by Synth — do not edit this block manually
# To remove: synth hooks uninstall
if ! command -v synth >/dev/null 2>&1; then
    exit 0
fi
SYNTH_CONFIG="$(git rev-parse --show-toplevel)/.synth/config.toml"
if [ ! -f "$SYNTH_CONFIG" ]; then
    exit 0
fi
synth _post-commit \
  --hash="$(git rev-parse HEAD)" \
  --quiet 2>/dev/null || true
# SYNTH HOOK END`

// fullHookTemplate is the complete hook file when no prior hook exists.
const fullHookTemplate = `#!/bin/sh
# SYNTH HOOK START
# Managed by Synth — do not edit this block manually
# To remove: synth hooks uninstall
if ! command -v synth >/dev/null 2>&1; then
    exit 0
fi
SYNTH_CONFIG="$(git rev-parse --show-toplevel)/.synth/config.toml"
if [ ! -f "$SYNTH_CONFIG" ]; then
    exit 0
fi
synth _post-commit \
  --hash="$(git rev-parse HEAD)" \
  --quiet 2>/dev/null || true
# SYNTH HOOK END
`

// InstallPostCommitHook installs or updates the Synth post-commit hook
// in the git repository at gitRoot. It handles three cases:
//   - No hook file exists: write the full template.
//   - Hook file exists without Synth markers: append Synth block.
//   - Hook file exists with Synth markers: replace the Synth block (upgrade).
func InstallPostCommitHook(gitRoot string) error {
	hookDir := filepath.Join(gitRoot, ".git", "hooks")
	hookPath := filepath.Join(hookDir, "post-commit")

	// Ensure hooks directory exists.
	if err := os.MkdirAll(hookDir, 0o755); err != nil {
		return fmt.Errorf("creating hooks directory: %w", err)
	}

	existing, err := os.ReadFile(hookPath)
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("reading existing hook: %w", err)
	}

	var content string

	if os.IsNotExist(err) {
		// Case 1: No existing hook — write full template.
		content = fullHookTemplate
	} else if !strings.Contains(string(existing), SynthHookMarkerStart) {
		// Case 2: Existing hook without Synth block — append.
		content = string(existing)
		if !strings.HasSuffix(content, "\n") {
			content += "\n"
		}
		content += "\n" + synthHookBlock + "\n"
	} else {
		// Case 3: Existing hook with Synth block — replace (upgrade).
		content = replaceSynthBlock(string(existing), synthHookBlock)
	}

	if err := os.WriteFile(hookPath, []byte(content), 0o755); err != nil {
		return fmt.Errorf("writing hook file: %w", err)
	}

	if err := os.Chmod(hookPath, 0o755); err != nil {
		return fmt.Errorf("setting hook permissions: %w", err)
	}

	return nil
}

// IsHookInstalled returns true if the post-commit hook exists and
// contains the Synth hook marker.
func IsHookInstalled(gitRoot string) bool {
	hookPath := filepath.Join(gitRoot, ".git", "hooks", "post-commit")
	data, err := os.ReadFile(hookPath)
	if err != nil {
		return false
	}
	return strings.Contains(string(data), SynthHookMarkerStart)
}

// UninstallHook removes the Synth-managed block from the post-commit hook.
// If the file only contained the Synth block, the file is deleted entirely.
func UninstallHook(gitRoot string) error {
	hookPath := filepath.Join(gitRoot, ".git", "hooks", "post-commit")

	data, err := os.ReadFile(hookPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil // Nothing to uninstall.
		}
		return fmt.Errorf("reading hook file: %w", err)
	}

	content := string(data)
	if !strings.Contains(content, SynthHookMarkerStart) {
		return nil // No Synth block to remove.
	}

	remaining := removeSynthBlock(content)

	// Check if only whitespace/shebang remains.
	trimmed := strings.TrimSpace(remaining)
	if trimmed == "" || trimmed == "#!/bin/sh" {
		// File only had Synth content — delete entirely.
		if err := os.Remove(hookPath); err != nil {
			return fmt.Errorf("removing hook file: %w", err)
		}
		return nil
	}

	if err := os.WriteFile(hookPath, []byte(remaining), 0o755); err != nil {
		return fmt.Errorf("writing hook file: %w", err)
	}

	return nil
}

// replaceSynthBlock replaces the content between markers (inclusive) with
// the new block content.
func replaceSynthBlock(content, newBlock string) string {
	startIdx := strings.Index(content, SynthHookMarkerStart)
	endIdx := strings.Index(content, SynthHookMarkerEnd)
	if startIdx < 0 || endIdx < 0 {
		return content
	}

	endIdx += len(SynthHookMarkerEnd)
	// Include trailing newline if present.
	if endIdx < len(content) && content[endIdx] == '\n' {
		endIdx++
	}

	return content[:startIdx] + newBlock + "\n" + content[endIdx:]
}

// removeSynthBlock removes the Synth block (between markers, inclusive)
// from the content, cleaning up surrounding blank lines.
func removeSynthBlock(content string) string {
	startIdx := strings.Index(content, SynthHookMarkerStart)
	endIdx := strings.Index(content, SynthHookMarkerEnd)
	if startIdx < 0 || endIdx < 0 {
		return content
	}

	endIdx += len(SynthHookMarkerEnd)
	// Include trailing newline if present.
	if endIdx < len(content) && content[endIdx] == '\n' {
		endIdx++
	}

	before := content[:startIdx]
	after := content[endIdx:]

	// Clean up extra blank lines at the junction.
	before = strings.TrimRight(before, "\n")
	after = strings.TrimLeft(after, "\n")

	if before != "" && after != "" {
		return before + "\n" + after
	}
	if before != "" {
		return before + "\n"
	}
	if after != "" {
		return after
	}
	return ""
}
