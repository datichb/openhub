// Package common provides terminal capability detection for TUI rendering decisions.
// Design System: Aurum v2 "Floating Panels" — see docs/design/aurum.md
package common

import (
	"os"
	"sync"

	"github.com/mattn/go-isatty"
)

var (
	noTUI     bool
	noTUIOnce sync.Once
)

// SetNoTUI is called by root.go PersistentPreRunE to propagate the --no-tui flag.
func SetNoTUI(v bool) {
	noTUI = v
}

// UseRichTUI returns true if the CLI should use alt-screen BubbleTea wizards
// and rich TUI components. It returns false if:
//   - --no-tui flag was passed
//   - stdin is not a terminal (piped input)
//   - CI=true environment variable is set
//   - TERM=dumb
//   - OH_RICH_TUI=0 environment variable is set
func UseRichTUI() bool {
	if noTUI {
		return false
	}
	noTUIOnce.Do(func() {
		// Detect non-interactive environments
		if os.Getenv("CI") != "" {
			noTUI = true
			return
		}
		if os.Getenv("TERM") == "dumb" {
			noTUI = true
			return
		}
		if os.Getenv("OH_RICH_TUI") == "0" {
			noTUI = true
			return
		}
		if !isatty.IsTerminal(os.Stdin.Fd()) && !isatty.IsCygwinTerminal(os.Stdin.Fd()) {
			noTUI = true
			return
		}
	})
	return !noTUI
}
