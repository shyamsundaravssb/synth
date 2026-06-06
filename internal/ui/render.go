package ui

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/shyamsundaravssb/synth/pkg/types"
)

// ANSI escape codes for terminal colors.
const (
	ansiReset   = "\033[0m"
	ansiBold    = "\033[1m"
	ansiDim     = "\033[2m"
	ansiRed     = "\033[31m"
	ansiGreen   = "\033[32m"
	ansiYellow  = "\033[33m"
	ansiCyan    = "\033[36m"
	ansiMagenta = "\033[35m"
	ansiWhite   = "\033[37m"
)

// StatusData contains all information needed to render a project status view.
type StatusData struct {
	ProjectName    string
	Developer      string
	TotalNotes     int
	FilesWithNotes []FileNoteSummary
	LowContextFiles []string
	LastNote       *types.Intent // nil if no notes yet
}

// FileNoteSummary summarizes notes for a single file.
type FileNoteSummary struct {
	FilePath  string
	NoteCount int
}

// RenderIntentLog renders a formatted intent log to stdout.
// If showAll is true, the footer hint is omitted.
func RenderIntentLog(intents []types.Intent, registry map[string]types.FileEntry, projectName string, developer string, showAll bool) {
	if len(intents) == 0 {
		fmt.Println()
		ShowInfo("no notes found. run 'synth note' to get started.")
		fmt.Println()
		return
	}

	// Header.
	fmt.Println()
	fmt.Printf("  %sSYNTH LOG%s  %s·%s  %s  %s·%s  %s\n",
		ansiBold, ansiReset,
		ansiDim, ansiReset,
		projectName,
		ansiDim, ansiReset,
		developer)
	fmt.Printf("  %s%s%s\n", ansiDim, strings.Repeat("─", 50), ansiReset)
	fmt.Println()

	for _, intent := range intents {
		renderIntentEntry(intent, registry)
	}

	// Footer.
	if !showAll {
		word := "notes"
		if len(intents) == 1 {
			word = "note"
		}
		fmt.Printf("  %s%d %s  ·  use --all to see everything%s\n",
			ansiDim, len(intents), word, ansiReset)
		fmt.Println()
	}
}

// renderIntentEntry renders a single intent block.
func renderIntentEntry(intent types.Intent, registry map[string]types.FileEntry) {
	relTime := RelativeTime(intent.Timestamp)
	typeColor := intentTypeColor(intent.Type)
	typeLabel := string(intent.Type)
	if intent.Type == types.IntentNewFile {
		typeLabel = "new file"
	}

	// File path + timestamp line.
	fmt.Printf("  %s%s%s%s%s%s\n",
		ansiBold+ansiWhite, intent.FilePath, ansiReset,
		strings.Repeat(" ", maxPad(50-len(intent.FilePath))),
		ansiDim, relTime+ansiReset)

	// Type badge + branch.
	fmt.Printf("  %s%s%s  %s·%s  %s branch\n",
		typeColor, typeLabel, ansiReset,
		ansiDim, ansiReset,
		intent.Branch)

	if intent.Type == types.IntentNewFile {
		// For new files, show registry info.
		if entry, ok := registry[intent.FilePath]; ok {
			fmt.Printf("  %sPurpose:%s  %s\n", ansiDim, ansiReset, entry.Purpose)
			fmt.Printf("  %sOwns:%s     %s\n", ansiDim, ansiReset, entry.Owns)
			fmt.Printf("  %sBoundary:%s %s\n", ansiDim, ansiReset, entry.Boundary)
		} else {
			fmt.Printf("  %sPurpose:%s  %s\n", ansiDim, ansiReset, intent.What)
		}
	} else {
		// For change/refactor/delete types.
		fmt.Printf("  %sWhat:%s   %s\n", ansiDim, ansiReset, intent.What)
		fmt.Printf("  %sWhy:%s    %s\n", ansiDim, ansiReset, intent.Why)
		if intent.Impact != "" {
			fmt.Printf("  %sImpact:%s %s\n", ansiDim, ansiReset, intent.Impact)
		}
	}

	fmt.Println()
}

// intentTypeColor returns the ANSI color code for an intent type.
func intentTypeColor(t types.IntentType) string {
	switch t {
	case types.IntentChange:
		return ansiCyan
	case types.IntentNewFile:
		return ansiYellow
	case types.IntentRefactor:
		return ansiMagenta
	case types.IntentDelete:
		return ansiRed
	default:
		return ansiWhite
	}
}

// maxPad ensures padding is non-negative.
func maxPad(n int) int {
	if n < 0 {
		return 0
	}
	return n
}

// intentJSONEnvelope is the versioned envelope for JSON output.
type intentJSONEnvelope struct {
	Version int            `json:"version"`
	Count   int            `json:"count"`
	Entries []types.Intent `json:"entries"`
}

// RenderIntentJSON marshals intents to a versioned JSON envelope and writes
// to stdout with 2-space indentation. Returns an error if marshaling fails.
func RenderIntentJSON(intents []types.Intent) error {
	if intents == nil {
		intents = []types.Intent{}
	}

	envelope := intentJSONEnvelope{
		Version: 1,
		Count:   len(intents),
		Entries: intents,
	}

	data, err := json.MarshalIndent(envelope, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling intent JSON: %w", err)
	}

	_, err = fmt.Fprintln(os.Stdout, string(data))
	return err
}

// RenderStatus renders a clean project status summary to stdout.
func RenderStatus(status StatusData) {
	fmt.Println()
	fmt.Printf("  %sPROJECT%s  %s\n", ansiBold, ansiReset, status.ProjectName)
	fmt.Printf("  %s%s%s\n", ansiDim, strings.Repeat("─", 40), ansiReset)

	fmt.Printf("  %sDeveloper:%s    %s\n", ansiDim, ansiReset, status.Developer)
	fmt.Printf("  %sTotal notes:%s  %d\n", ansiDim, ansiReset, status.TotalNotes)
	fmt.Println()

	// Files with notes.
	if len(status.FilesWithNotes) > 0 {
		fmt.Printf("  %s%sFILES WITH NOTES:%s\n", ansiBold, ansiDim, ansiReset)
		for _, f := range status.FilesWithNotes {
			word := "notes"
			if f.NoteCount == 1 {
				word = "note"
			}
			fmt.Printf("    %s  %s(%d %s)%s\n", f.FilePath, ansiDim, f.NoteCount, word, ansiReset)
		}
		fmt.Println()
	}

	// Low context files.
	if len(status.LowContextFiles) > 0 {
		fmt.Printf("  %s%sLOW CONTEXT FILES (saved without notes):%s\n",
			ansiBold, ansiDim, ansiReset)
		for _, f := range status.LowContextFiles {
			fmt.Printf("    %s\n", f)
		}
		fmt.Println()
	}

	// Last note.
	fmt.Printf("  %s%sLAST NOTE:%s\n", ansiBold, ansiDim, ansiReset)
	if status.LastNote == nil {
		fmt.Printf("    %sNo notes yet%s\n", ansiDim, ansiReset)
	} else {
		relTime := RelativeTime(status.LastNote.Timestamp)
		fmt.Printf("    %s%s%s  %s·%s  %s%s%s\n",
			ansiBold, status.LastNote.FilePath, ansiReset,
			ansiDim, ansiReset,
			ansiDim, relTime, ansiReset)
		fmt.Printf("    %s\n", status.LastNote.What)
	}
	fmt.Println()
}

// RelativeTime returns a human-readable relative time string.
//
//	< 1 minute:   "just now"
//	< 60 minutes: "42m ago"
//	< 24 hours:   "today, 3:45 PM"
//	< 48 hours:   "yesterday, 3:45 PM"
//	>= 48 hours:  "Jan 2, 3:45 PM"
func RelativeTime(t time.Time) string {
	return relativeTimeFrom(t, time.Now())
}

// relativeTimeFrom computes relative time against a reference point.
// Exported for testing via the package-level RelativeTime function.
func relativeTimeFrom(t, now time.Time) string {
	diff := now.Sub(t)

	if diff < time.Minute {
		return "just now"
	}

	if diff < time.Hour {
		mins := int(diff.Minutes())
		return fmt.Sprintf("%dm ago", mins)
	}

	// Use calendar days for "today" and "yesterday" checks.
	tYear, tMonth, tDay := t.Date()
	nowYear, nowMonth, nowDay := now.Date()

	sameDay := tYear == nowYear && tMonth == nowMonth && tDay == nowDay

	// Check yesterday: the day before now.
	yesterday := now.AddDate(0, 0, -1)
	yYear, yMonth, yDay := yesterday.Date()
	isYesterday := tYear == yYear && tMonth == yMonth && tDay == yDay

	timeStr := t.Format("3:04 PM")

	if sameDay {
		return "today, " + timeStr
	}

	if isYesterday {
		return "yesterday, " + timeStr
	}

	return t.Format("Jan 2, ") + timeStr
}
