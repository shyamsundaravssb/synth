package git

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// binaryExtensions lists file extensions to exclude from modified file results.
var binaryExtensions = map[string]bool{
	".exe":   true,
	".bin":   true,
	".so":    true,
	".dylib": true,
	".png":   true,
	".jpg":   true,
	".jpeg":  true,
	".gif":   true,
	".pdf":   true,
	".zip":   true,
	".tar":   true,
	".gz":    true,
}

// FindGitRoot walks up the directory tree from startPath looking for a .git
// directory. Returns the absolute path of the directory containing .git.
func FindGitRoot(startPath string) (string, error) {
	absPath, err := filepath.Abs(startPath)
	if err != nil {
		return "", fmt.Errorf("resolving path: %w", err)
	}

	current := absPath
	for {
		gitDir := filepath.Join(current, ".git")
		info, err := os.Stat(gitDir)
		if err == nil && info.IsDir() {
			return current, nil
		}

		parent := filepath.Dir(current)
		if parent == current {
			// Reached filesystem root without finding .git.
			return "", fmt.Errorf("not a git repository (or any parent directory)")
		}
		current = parent
	}
}

// IsGitRepo returns true if path is inside a git repository.
func IsGitRepo(path string) bool {
	_, err := FindGitRoot(path)
	return err == nil
}

// GetCurrentBranch returns the current branch name for the repository
// at gitRoot. If HEAD is detached, returns "HEAD". If the git command
// fails, returns "unknown", nil — never fails a Synth operation.
func GetCurrentBranch(gitRoot string) (string, error) {
	output, err := runGitCommand(gitRoot, "rev-parse", "--abbrev-ref", "HEAD")
	if err != nil {
		return "unknown", nil
	}
	if output == "" {
		return "unknown", nil
	}
	return output, nil
}

// GetCurrentCommit returns the current commit hash for the repository
// at gitRoot. If no commits exist or the command fails, returns "", nil.
func GetCurrentCommit(gitRoot string) (string, error) {
	output, err := runGitCommand(gitRoot, "rev-parse", "HEAD")
	if err != nil {
		return "", nil
	}
	return output, nil
}

// GetRecentlyModifiedFiles returns files recently modified in the git
// repository at gitRoot. It combines two sources: git status output and
// filesystem walk checking file modification times within the 'since' duration.
// Results are deduplicated and returned relative to gitRoot.
func GetRecentlyModifiedFiles(gitRoot string, since time.Duration) ([]string, error) {
	seen := make(map[string]bool)
	var result []string

	addFile := func(relPath string) {
		// Normalize path separators.
		relPath = filepath.ToSlash(relPath)
		relPath = filepath.FromSlash(relPath)

		if seen[relPath] {
			return
		}
		if shouldExclude(relPath) {
			return
		}
		seen[relPath] = true
		result = append(result, relPath)
	}

	// Source 1: git status --porcelain
	statusOutput, err := runGitCommand(gitRoot, "status", "--porcelain")
	if err == nil && statusOutput != "" {
		scanner := bufio.NewScanner(strings.NewReader(statusOutput))
		for scanner.Scan() {
			line := scanner.Text()
			if len(line) < 4 {
				continue
			}
			// Porcelain format: XY filename
			// The first two characters are status codes, then a space.
			filePath := strings.TrimSpace(line[3:])
			// Handle renamed files: "old -> new"
			if idx := strings.Index(filePath, " -> "); idx >= 0 {
				filePath = filePath[idx+4:]
			}
			if filePath != "" {
				addFile(filePath)
			}
		}
	}

	// Source 2: Walk filesystem, check ModTime.
	cutoff := time.Now().Add(-since)
	_ = filepath.Walk(gitRoot, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // skip errors
		}

		relPath, relErr := filepath.Rel(gitRoot, path)
		if relErr != nil {
			return nil
		}

		// Skip .git and .synth directories entirely.
		if info.IsDir() {
			name := filepath.Base(relPath)
			if name == ".git" || name == ".synth" {
				return filepath.SkipDir
			}
			return nil
		}

		if info.ModTime().After(cutoff) {
			addFile(relPath)
		}

		return nil
	})

	return result, nil
}

// GetFileDiff returns the diff of a file against HEAD. If no diff exists
// (file untracked or no changes), returns "", nil. Output is limited to
// the first 100 lines.
func GetFileDiff(gitRoot, filePath string) (string, error) {
	output, err := runGitCommand(gitRoot, "diff", "HEAD", "--", filePath)
	if err != nil {
		return "", nil
	}
	if output == "" {
		return "", nil
	}

	// Limit to first 100 lines.
	lines := strings.SplitN(output, "\n", 101)
	if len(lines) > 100 {
		lines = lines[:100]
	}

	return strings.Join(lines, "\n"), nil
}

// shouldExclude returns true if the file should be excluded from results.
func shouldExclude(relPath string) bool {
	// Exclude .git/ and .synth/ directories.
	parts := strings.Split(filepath.ToSlash(relPath), "/")
	for _, part := range parts {
		if part == ".git" || part == ".synth" {
			return true
		}
	}

	// Exclude binary files by extension.
	ext := strings.ToLower(filepath.Ext(relPath))
	return binaryExtensions[ext]
}

// runGitCommand executes git with the given args in gitRoot directory.
// Returns trimmed stdout. Returns stderr content in error if command fails.
func runGitCommand(gitRoot string, args ...string) (string, error) {
	fullArgs := append([]string{"-C", gitRoot}, args...)
	cmd := exec.Command("git", fullArgs...)

	var stdout, stderr strings.Builder
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		errMsg := strings.TrimSpace(stderr.String())
		if errMsg == "" {
			errMsg = err.Error()
		}
		return "", fmt.Errorf("git %s: %s", strings.Join(args, " "), errMsg)
	}

	return strings.TrimSpace(stdout.String()), nil
}
