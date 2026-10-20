package theme

import (
	"os"

	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"
)

// NoColor reports whether the user asked for no color: NO_COLOR set to any
// non-empty value (https://no-color.org). It is read at each call, so a
// process and its tests can change it.
func NoColor() bool { return os.Getenv("NO_COLOR") != "" }

// ApplyNoColor turns every color of the CLI and the TUI off when NO_COLOR is
// set: the lipgloss styles (CLI output, forms, help) render plain text, the
// tview color tags are empty (ColorTag) and the logs are not colored
// (prettylog). tcell also drops the colors of the screen by itself when
// NO_COLOR is set. Without NO_COLOR it changes nothing: the terminal and
// CLICOLOR decide, as before.
func ApplyNoColor() {
	if NoColor() {
		lipgloss.SetColorProfile(termenv.Ascii)
	}
}
