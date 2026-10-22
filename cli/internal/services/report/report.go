// Package report is the ReportService (modernisation L4, workflow_report):
// the reports the agents of a session file with the workflow MCP server.
// oh stores the body (a Markdown text) in the session folder
// (~/.oh/sessions/<session>/reports/rp-<n>.md, mode 0600) and its metadata
// in session_reports; the author is the agent oh saw calling the tool. The
// coordinators then only pass the id on; the checkpoint card shows the
// reports filed since the last checkpoint.
//
// Limits: 64 KiB per report, 100 reports and 4 MiB per session. Control
// characters are removed (a report is shown in terminals).
package report

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/datichb/openhub/cli/internal/domain"
	"github.com/datichb/openhub/cli/internal/fsutil"
)

// Limits of the reports.
const (
	MaxBytes        = 64 << 10 // body of a report
	MaxPerSession   = 100
	MaxSessionBytes = 4 << 20 // bodies of a session
	MaxTitle        = 120     // characters
)

// DirName is the folder of the reports in a session folder.
const DirName = "reports"

// Errors.
var (
	ErrEmpty    = errors.New("the report is empty")
	ErrTooLarge = fmt.Errorf("the report is larger than %d KiB", MaxBytes>>10)
	ErrFull     = fmt.Errorf("the session already holds %d reports or %d MiB of reports", MaxPerSession, MaxSessionBytes>>20)
	ErrBadID    = errors.New("a report id reads rp-<number>")
	// ErrAltered: the body on disk is not the one oh stored.
	ErrAltered = errors.New("the body of the report was changed outside oh")
	ErrNoStore = errors.New("report: no store")
)

// Service is the ReportService.
type Service struct {
	Store domain.ReportStore
	// SessionsDir is ~/.oh/sessions.
	SessionsDir string
	Now         func() time.Time
}

func (s *Service) now() time.Time {
	if s.Now != nil {
		return s.Now()
	}
	return time.Now()
}

// Dir is the reports folder of a session.
func (s *Service) Dir(sessionID string) string {
	return filepath.Join(s.SessionsDir, sessionID, DirName)
}

func (s *Service) path(sessionID string, seq int) string {
	return filepath.Join(s.Dir(sessionID), domain.ReportID(seq)+".md")
}

// escapes are the terminal sequences (CSI, OSC) removed whole.
var escapes = regexp.MustCompile(`\x1b\[[\x30-\x3f]*[\x20-\x2f]*[\x40-\x7e]|\x1b\][^\x07\x1b]*(?:\x07|\x1b\\)?`)

// Clean removes the terminal sequences and the control characters of a
// text (line feeds and tabs stay), ends of line become \n and invalid UTF-8
// is replaced.
func Clean(text string) string {
	text = strings.ToValidUTF8(text, "\uFFFD")
	text = escapes.ReplaceAllString(text, "")
	text = strings.ReplaceAll(text, "\r\n", "\n")
	return strings.Map(func(r rune) rune {
		switch {
		case r == '\n' || r == '\t':
			return r
		case r == '\r':
			return '\n'
		case unicode.IsControl(r), unicode.Is(unicode.Bidi_Control, r):
			return -1
		}
		return r
	}, text)
}

// cleanTitle returns a title on one line, bounded.
func cleanTitle(title, body string) string {
	t := strings.Join(strings.Fields(Clean(title)), " ")
	if t == "" {
		for _, line := range strings.Split(body, "\n") {
			if h, ok := heading(line); ok {
				t = h.text
				break
			}
		}
	}
	if t == "" {
		t = "Rapport"
	}
	if utf8.RuneCountInString(t) > MaxTitle {
		t = string([]rune(t)[:MaxTitle-1]) + "…"
	}
	return t
}

type headingLine struct {
	level int
	text  string
}

// heading reads a Markdown ATX heading ("## Tests").
func heading(line string) (headingLine, bool) {
	l := strings.TrimRight(line, " \t")
	n := 0
	for n < len(l) && n < 7 && l[n] == '#' {
		n++
	}
	if n == 0 || n > 6 || (len(l) > n && l[n] != ' ' && l[n] != '\t') {
		return headingLine{}, false
	}
	text := strings.TrimSpace(strings.TrimRight(strings.TrimSpace(l[n:]), "#"))
	if text == "" {
		return headingLine{}, false
	}
	return headingLine{level: n, text: text}, true
}

// Sections returns the headings of a body, in order (fenced code ignored).
func Sections(body string) []string {
	var out []string
	walk(body, func(_ int, h headingLine) { out = append(out, h.text) })
	return out
}

// walk calls fn for each heading outside fenced code, with its line index.
func walk(body string, fn func(line int, h headingLine)) {
	fence := false
	for i, line := range strings.Split(body, "\n") {
		t := strings.TrimSpace(line)
		if strings.HasPrefix(t, "```") || strings.HasPrefix(t, "~~~") {
			fence = !fence
			continue
		}
		if fence {
			continue
		}
		if h, ok := heading(line); ok {
			fn(i, h)
		}
	}
}

// Section returns a section of a body: its heading and what follows up to
// the next heading of the same or a higher level. name matches the heading
// text whatever its case; a heading containing name is taken when none is
// equal.
func Section(body, name string) (string, bool) {
	name = strings.ToLower(strings.TrimSpace(strings.Trim(strings.TrimSpace(name), "#")))
	if name == "" {
		return body, true
	}
	lines := strings.Split(body, "\n")
	type found struct{ line, level int }
	var exact, partial *found
	var heads []found
	walk(body, func(i int, h headingLine) {
		f := found{i, h.level}
		heads = append(heads, f)
		t := strings.ToLower(h.text)
		switch {
		case t == name && exact == nil:
			exact = &f
		case strings.Contains(t, name) && partial == nil:
			partial = &f
		}
	})
	hit := exact
	if hit == nil {
		hit = partial
	}
	if hit == nil {
		return "", false
	}
	end := len(lines)
	for _, h := range heads {
		if h.line > hit.line && h.level <= hit.level {
			end = h.line
			break
		}
	}
	return strings.TrimRight(strings.Join(lines[hit.line:end], "\n"), "\n"), true
}

func digest(body string) string {
	sum := sha256.Sum256([]byte(body))
	return hex.EncodeToString(sum[:])
}

// Add stores a report of a session. agent is the author seen by oh.
func (s *Service) Add(ctx context.Context, sessionID, agent, toolSession, title, body string) (domain.Report, error) {
	if s.Store == nil || s.SessionsDir == "" {
		return domain.Report{}, ErrNoStore
	}
	body = strings.TrimSpace(Clean(body))
	if body == "" {
		return domain.Report{}, ErrEmpty
	}
	if len(body) > MaxBytes {
		return domain.Report{}, fmt.Errorf("%w (%d KiB)", ErrTooLarge, (len(body)+1023)>>10)
	}
	r := domain.Report{SessionID: sessionID, Agent: strings.TrimSpace(Clean(agent)), ToolSession: toolSession,
		Title: cleanTitle(title, body), Bytes: len(body), SHA256: digest(body), Sections: Sections(body), CreatedAt: s.now().UTC()}
	if err := s.Store.AddReport(ctx, &r, MaxPerSession, MaxSessionBytes); err != nil {
		if errors.Is(err, domain.ErrReportLimit) {
			return domain.Report{}, ErrFull
		}
		return domain.Report{}, err
	}
	if err := s.write(sessionID, r.Seq, body); err != nil {
		_ = s.Store.DeleteReport(context.WithoutCancel(ctx), sessionID, r.Seq)
		return domain.Report{}, err
	}
	return r, nil
}

// write writes a body (0600, folder 0700), through a temporary file.
func (s *Service) write(sessionID string, seq int, body string) error {
	dir := s.Dir(sessionID)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	_ = os.Chmod(dir, 0o700)
	return fsutil.WriteFileAtomic(s.path(sessionID, seq), []byte(body), 0o600)
}

// List returns the reports of a session, oldest first.
func (s *Service) List(ctx context.Context, sessionID string) ([]domain.Report, error) {
	if s.Store == nil {
		return nil, ErrNoStore
	}
	return s.Store.ListReports(ctx, sessionID)
}

// Since returns the reports of a session filed after t.
func (s *Service) Since(ctx context.Context, sessionID string, t time.Time) ([]domain.Report, error) {
	all, err := s.List(ctx, sessionID)
	if err != nil {
		return nil, err
	}
	var out []domain.Report
	for _, r := range all {
		if r.CreatedAt.After(t) {
			out = append(out, r)
		}
	}
	return out, nil
}

// Get returns a report and its body (checked against the stored digest).
func (s *Service) Get(ctx context.Context, sessionID, id string) (domain.Report, string, error) {
	if s.Store == nil {
		return domain.Report{}, "", ErrNoStore
	}
	seq, ok := domain.ParseReportID(id)
	if !ok {
		return domain.Report{}, "", ErrBadID
	}
	r, err := s.Store.GetReport(ctx, sessionID, seq)
	if err != nil {
		return domain.Report{}, "", fmt.Errorf("report %s: %w", domain.ReportID(seq), err)
	}
	data, err := os.ReadFile(s.path(sessionID, seq))
	if err != nil {
		return *r, "", fmt.Errorf("report %s: %w", r.ID(), err)
	}
	body := string(data)
	if digest(body) != r.SHA256 {
		return *r, "", fmt.Errorf("report %s: %w", r.ID(), ErrAltered)
	}
	return *r, body, nil
}

// Exported is a report with its body (remote sessions: the artifact of the
// job, imported when the session is fetched).
type Exported struct {
	domain.Report
	Body string `json:"body"`
}

// Export returns the reports of a session with their bodies.
func (s *Service) Export(ctx context.Context, sessionID string) ([]Exported, error) {
	list, err := s.List(ctx, sessionID)
	if err != nil {
		return nil, err
	}
	out := make([]Exported, 0, len(list))
	for _, r := range list {
		_, body, err := s.Get(ctx, sessionID, r.ID())
		if err != nil {
			return nil, err
		}
		out = append(out, Exported{Report: r, Body: body})
	}
	return out, nil
}

// Import records the reports of a remote session (artifact of its job,
// not trusted: cleaned and bounded again). A report already there is kept.
// It returns the number of reports added.
func (s *Service) Import(ctx context.Context, sessionID string, list []Exported) (int, error) {
	if s.Store == nil || s.SessionsDir == "" {
		return 0, ErrNoStore
	}
	n, total := 0, 0
	for i, e := range list {
		if i >= MaxPerSession {
			break
		}
		body := strings.TrimSpace(Clean(e.Body))
		if body == "" || len(body) > MaxBytes || e.Seq <= 0 || total+len(body) > MaxSessionBytes {
			continue
		}
		total += len(body)
		at := e.CreatedAt
		if at.IsZero() {
			at = s.now().UTC()
		}
		r := domain.Report{SessionID: sessionID, Seq: e.Seq, Agent: strings.TrimSpace(Clean(e.Agent)), ToolSession: e.ToolSession,
			Title: cleanTitle(e.Title, body), Bytes: len(body), SHA256: digest(body), Sections: Sections(body), CreatedAt: at}
		if _, err := s.Store.GetReport(ctx, sessionID, r.Seq); err == nil {
			continue
		}
		if err := s.write(sessionID, r.Seq, body); err != nil {
			return n, err
		}
		added, err := s.Store.ImportReport(ctx, &r)
		if err != nil {
			return n, err
		}
		if added {
			n++
		}
	}
	return n, nil
}
