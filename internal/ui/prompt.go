package ui

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"strings"
)

// ErrCancelled is returned when the user cancels an interactive prompt
// via Escape or Ctrl+C.
var ErrCancelled = errors.New("cancelled")

// FileOption represents a file choice in the interactive file selector.
type FileOption struct {
	Path        string
	ModifiedAgo string // human-readable: "4m ago", "1h ago"
}

// SelectFile renders an interactive file selection list. The last item is
// always "Type a path manually..." which falls through to AskSingleLine.
// If files is empty, goes directly to manual entry.
// Returns "", ErrCancelled if the user cancels.
func SelectFile(files []FileOption) (string, error) {
	if len(files) == 0 {
		return AskSingleLine("Enter file path:", "path/to/file.go")
	}

	fmt.Println()
	dim := "\033[2m"
	reset := "\033[0m"
	cyan := "\033[36m"
	bold := "\033[1m"

	for i, f := range files {
		fmt.Printf("  %s[%d]%s %s%s%s  %s%s%s\n",
			cyan, i+1, reset,
			bold, f.Path, reset,
			dim, f.ModifiedAgo, reset)
	}
	manualIdx := len(files) + 1
	fmt.Printf("  %s[%d]%s Type a path manually...\n", cyan, manualIdx, reset)
	fmt.Println()

	for {
		fmt.Print("  Select [1-" + fmt.Sprint(manualIdx) + "]: ")
		reader := bufio.NewReader(os.Stdin)
		input, err := reader.ReadString('\n')
		if err != nil {
			return "", ErrCancelled
		}
		input = strings.TrimSpace(input)

		if input == "" {
			continue
		}

		// Check for cancel signals.
		if input == "q" || input == "Q" {
			return "", ErrCancelled
		}

		var choice int
		if _, scanErr := fmt.Sscanf(input, "%d", &choice); scanErr != nil {
			ShowError("Please enter a number between 1 and " + fmt.Sprint(manualIdx))
			continue
		}

		if choice < 1 || choice > manualIdx {
			ShowError("Please enter a number between 1 and " + fmt.Sprint(manualIdx))
			continue
		}

		if choice == manualIdx {
			return AskSingleLine("Enter file path:", "path/to/file.go")
		}

		return files[choice-1].Path, nil
	}
}

// AskSingleLine renders a single-line text input. The placeholder is shown
// as a hint. Input must be at least 3 characters. Returns "", ErrCancelled
// if the user cancels.
func AskSingleLine(prompt, placeholder string) (string, error) {
	dim := "\033[2m"
	reset := "\033[0m"

	reader := bufio.NewReader(os.Stdin)

	for {
		if placeholder != "" {
			fmt.Printf("  %s %s(%s)%s: ", prompt, dim, placeholder, reset)
		} else {
			fmt.Printf("  %s: ", prompt)
		}

		input, err := reader.ReadString('\n')
		if err != nil {
			return "", ErrCancelled
		}
		input = strings.TrimSpace(input)

		if input == "" {
			ShowError("Input must be at least 3 characters")
			continue
		}

		if len(input) < 3 {
			ShowError("Input must be at least 3 characters")
			continue
		}

		return input, nil
	}
}

// AskOptionalLine is like AskSingleLine but with no minimum length validation.
// Empty input is valid and returns "". Returns "", ErrCancelled if the user
// cancels.
func AskOptionalLine(prompt string) (string, error) {
	dim := "\033[2m"
	reset := "\033[0m"

	fmt.Printf("  %s %s(optional, Enter to skip)%s: ", prompt, dim, reset)

	reader := bufio.NewReader(os.Stdin)
	input, err := reader.ReadString('\n')
	if err != nil {
		return "", ErrCancelled
	}
	return strings.TrimSpace(input), nil
}

// Confirm renders a Y/n or y/N confirmation prompt. Enter with no input
// uses the default. Returns false, ErrCancelled if the user cancels.
func Confirm(question string, defaultYes bool) (bool, error) {
	hint := "y/N"
	if defaultYes {
		hint = "Y/n"
	}

	reader := bufio.NewReader(os.Stdin)

	for {
		fmt.Printf("  %s [%s]: ", question, hint)

		input, err := reader.ReadString('\n')
		if err != nil {
			return false, ErrCancelled
		}
		input = strings.TrimSpace(strings.ToLower(input))

		switch input {
		case "":
			return defaultYes, nil
		case "y", "yes":
			return true, nil
		case "n", "no":
			return false, nil
		default:
			ShowError("Please answer y or n")
		}
	}
}

// ShowSuccess prints a green checkmark line to stdout.
func ShowSuccess(message string) {
	green := "\033[32m"
	reset := "\033[0m"
	fmt.Printf("  %s✓%s %s\n", green, reset, message)
}

// ShowError prints a red ✗ line to stderr.
func ShowError(message string) {
	red := "\033[31m"
	reset := "\033[0m"
	fmt.Fprintf(os.Stderr, "  %s✗%s %s\n", red, reset, message)
}

// ShowInfo prints a dimmed info line to stdout.
func ShowInfo(message string) {
	dim := "\033[2m"
	reset := "\033[0m"
	fmt.Printf("  %s·%s %s\n", dim, reset, message)
}
