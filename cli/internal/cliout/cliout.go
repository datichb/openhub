// Package cliout is where the commands of oh write what they print: a table
// of aligned columns, a JSON document, and the one-line messages that start
// with a status mark (success, warning, information, error).
//
// Every function writes to the io.Writer it is given (cmd.OutOrStdout(),
// cmd.ErrOrStderr(), a.IO.Out…), never to os.Stdout, so that a test can read
// the output and a command can be run in-process. The marks follow the theme
// (colors, NO_COLOR) when they are written, not when the program starts.
package cliout

import (
	"encoding/json"
	"fmt"
	"io"
	"text/tabwriter"

	"github.com/datichb/openhub/cli/internal/tui/theme"
)

// Table returns the writer of a table: the cells of a row are separated by a
// tab, a row ends with a newline, and the columns are aligned (two spaces
// between columns) once Flush is called.
func Table(w io.Writer) *tabwriter.Writer {
	return tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
}

// JSON writes v as an indented JSON document (two spaces) followed by a
// newline: the output of the --json options.
func JSON(w io.Writer, v any) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(v)
}

// JSONCompact writes v as a JSON document on one line followed by a newline.
// It is the output of the --json options that have always been compact (and
// of the HTTP responses); a script that reads them does not depend on the
// indentation, but their text is kept as it was.
func JSONCompact(w io.Writer, v any) error {
	return json.NewEncoder(w).Encode(v)
}

// SuccessIcon is the mark of a success, colored like the theme's success.
func SuccessIcon() string { return theme.SuccessStyle.Render(theme.IconSuccess) }

// WarnIcon is the mark of a warning.
func WarnIcon() string { return theme.WarningStyle.Render(theme.IconWarning) }

// InfoIcon is the mark of an information.
func InfoIcon() string { return theme.InfoStyle.Render(theme.IconInfo) }

// ErrorIcon is the mark of an error.
func ErrorIcon() string { return theme.ErrorStyle.Render(theme.IconError) }

// Success writes the success mark, a space and msg, on one line.
func Success(w io.Writer, msg string) { line(w, SuccessIcon(), msg) }

// Warn writes the warning mark, a space and msg, on one line.
func Warn(w io.Writer, msg string) { line(w, WarnIcon(), msg) }

// Info writes the information mark, a space and msg, on one line.
func Info(w io.Writer, msg string) { line(w, InfoIcon(), msg) }

// Error writes the error mark, a space and msg, on one line.
func Error(w io.Writer, msg string) { line(w, ErrorIcon(), msg) }

func line(w io.Writer, icon, msg string) {
	fmt.Fprintf(w, "%s %s\n", icon, msg)
}
