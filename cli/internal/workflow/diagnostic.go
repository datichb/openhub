package workflow

import (
	"fmt"
	"sort"
	"strings"

	"github.com/datichb/openhub/cli/internal/i18n"
)

// Severity is the importance of a diagnostic.
type Severity string

const (
	SeverityError   Severity = "error"   // the workflow cannot be used
	SeverityWarning Severity = "warning" // usable, but probably not what the author meant
)

// Pos is a 1-based position in a YAML source. Col is 0 when unknown.
type Pos struct {
	Line int `json:"line,omitempty"`
	Col  int `json:"col,omitempty"`
}

// IsZero reports whether the position is unknown.
func (p Pos) IsZero() bool { return p.Line == 0 }

func (p Pos) String() string {
	switch {
	case p.Line == 0:
		return ""
	case p.Col == 0:
		return fmt.Sprintf("%d", p.Line)
	}
	return fmt.Sprintf("%d:%d", p.Line, p.Col)
}

// Diagnostic is one finding of the parser, the resolver or the validator.
//
// Code is stable (tests, TUI, JSON output); Message and Hint are localized
// from the i18n keys "workflow.diag.<code>" and "workflow.diag.<code>.hint".
type Diagnostic struct {
	Severity Severity `json:"severity"`
	Code     string   `json:"code"`
	// Path is the field path in the document ("agents.developer.after",
	// "outputs[1].type"); empty for document-level findings.
	Path string `json:"path,omitempty"`
	// Source identifies the file or layer the finding points to.
	Source string `json:"source,omitempty"`
	Pos    Pos    `json:"pos,omitzero"`
	// Message and Hint are localized when the diagnostic is created.
	Message string `json:"message"`
	Hint    string `json:"hint,omitempty"`
}

func (d Diagnostic) String() string {
	var b strings.Builder
	loc := d.Source
	if p := d.Pos.String(); p != "" {
		if loc != "" {
			loc += ":"
		}
		loc += p
	}
	if loc != "" {
		b.WriteString(loc)
		b.WriteString(": ")
	}
	b.WriteString(string(d.Severity))
	if d.Path != "" {
		b.WriteString(" ")
		b.WriteString(d.Path)
	}
	b.WriteString(": ")
	b.WriteString(d.Message)
	return b.String()
}

// Diagnostics is a list of findings.
type Diagnostics []Diagnostic

// HasErrors reports whether at least one finding is an error.
func (ds Diagnostics) HasErrors() bool {
	for _, d := range ds {
		if d.Severity == SeverityError {
			return true
		}
	}
	return false
}

// Errors returns the error findings only.
func (ds Diagnostics) Errors() Diagnostics {
	var out Diagnostics
	for _, d := range ds {
		if d.Severity == SeverityError {
			out = append(out, d)
		}
	}
	return out
}

// Codes returns the codes in order (handy in tests).
func (ds Diagnostics) Codes() []string {
	out := make([]string, len(ds))
	for i, d := range ds {
		out[i] = d.Code
	}
	return out
}

// Sort orders findings by source, line, column, then path.
func (ds Diagnostics) Sort() {
	sort.SliceStable(ds, func(i, j int) bool {
		a, b := ds[i], ds[j]
		if a.Source != b.Source {
			return a.Source < b.Source
		}
		if a.Pos.Line != b.Pos.Line {
			return a.Pos.Line < b.Pos.Line
		}
		if a.Pos.Col != b.Pos.Col {
			return a.Pos.Col < b.Pos.Col
		}
		return a.Path < b.Path
	})
}

// Err returns nil without errors, otherwise an error listing them.
func (ds Diagnostics) Err() error {
	errs := ds.Errors()
	if len(errs) == 0 {
		return nil
	}
	lines := make([]string, len(errs))
	for i, d := range errs {
		lines[i] = d.String()
	}
	return fmt.Errorf("%s", strings.Join(lines, "\n"))
}

// newDiag builds a localized diagnostic. args fill the message format.
func newDiag(sev Severity, code, path string, args ...any) Diagnostic {
	key := "workflow.diag." + code
	msg := i18n.T(key)
	if msg == key {
		msg = code // missing translation: keep the code readable
		for _, a := range args {
			msg += fmt.Sprintf(" %v", a)
		}
	} else if len(args) > 0 {
		msg = fmt.Sprintf(msg, args...)
	}
	d := Diagnostic{Severity: sev, Code: code, Path: path, Message: msg}
	if h := i18n.T(key + ".hint"); h != key+".hint" {
		d.Hint = h
	}
	return d
}

func errDiag(code, path string, args ...any) Diagnostic {
	return newDiag(SeverityError, code, path, args...)
}

func warnDiag(code, path string, args ...any) Diagnostic {
	return newDiag(SeverityWarning, code, path, args...)
}
