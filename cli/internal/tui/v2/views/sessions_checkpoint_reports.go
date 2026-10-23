package views

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/rivo/tview"

	"github.com/datichb/openhub/cli/internal/i18n"
	"github.com/datichb/openhub/cli/internal/textfmt"
)

// Reports on the checkpoint card (modernisation L4, workflow_report): the
// reports filed since the previous checkpoint, each opened in a scrolling
// reader. Report texts come from agents: always escaped.

// checkpointCardReports is the number of reports with their own reader
// action on the card.
const checkpointCardReports = 4

// CheckpointReport is a report attached to a checkpoint card.
type CheckpointReport struct {
	ID    string
	Agent string
	Title string
	Bytes int
	At    time.Time
	// Session holds the report when it is not the session shown (the child
	// of a fan-out, on its parent); Child is that child's label. Ids are
	// per session: rp-1 exists in each child.
	Session string
	Child   string
}

// key tells reports of different sessions apart.
func (r CheckpointReport) key() string {
	if r.Session == "" {
		return r.ID
	}
	return r.Session + "/" + r.ID
}

// owner is the session that holds the report (shown is the one displayed).
func (r CheckpointReport) owner(shown string) string {
	return firstText(r.Session, shown)
}

// CheckpointReportReader is implemented by session backends that can read
// a stored report (optional: without it, the card only lists them).
type CheckpointReportReader interface {
	CheckpointReport(ctx context.Context, sessionID, id string) (title, body string, err error)
}

// writeCheckpointReports renders the reports section of the card.
func writeCheckpointReports(b *strings.Builder, reports []CheckpointReport) {
	if len(reports) == 0 {
		return
	}
	b.WriteString(muted("─ "+i18n.T("tui.checkpoint.reports")) + "\n")
	for _, r := range reports {
		line := fmt.Sprintf("%s · %s · %s", r.ID, firstText(r.Agent, "?"), textfmt.Clip(r.Title, 80))
		fmt.Fprintf(b, "  %s  %s\n", tview.Escape(line), muted(tview.Escape(reportMeta(r))))
	}
	b.WriteString("\n")
}

func reportMeta(r CheckpointReport) string {
	size := textfmt.Bytes(int64(r.Bytes))
	if r.At.IsZero() {
		return size
	}
	return size + " · " + textfmt.Clock(r.At)
}

// reportActions are the reader actions of the card (one per report, the
// first ones).
func (v *SessionsView) reportActions(sessionID, decisionID string, reports []CheckpointReport) []ModalAction {
	rd, ok := v.cfg.Backend.(CheckpointReportReader)
	if !ok || sessionID == "" {
		return nil
	}
	var out []ModalAction
	for i, r := range reports {
		if i == checkpointCardReports {
			break
		}
		rp := r
		out = append(out, ModalAction{Label: i18n.Tf("tui.checkpoint.read_report", rp.ID), Callback: func() {
			v.openReport(rd, sessionID, decisionID, rp)
		}})
	}
	return out
}

// openReport loads a report off the event loop, then shows it in a
// scrolling reader.
func (v *SessionsView) openReport(rd CheckpointReportReader, sessionID, decisionID string, r CheckpointReport) {
	var title, body string // written off the loop, read on it after
	v.async(func(ctx context.Context) (string, error) {
		var err error
		title, body, err = rd.CheckpointReport(ctx, sessionID, r.ID)
		return "", err
	}, func(string) {
		if v.shell == nil {
			return
		}
		head := strings.Join([]string{r.ID, firstText(r.Agent, "?"), firstText(title, r.Title)}, " · ")
		v.shell.ShowScrollableModal(tview.Escape(head), tview.Escape(body), nil)
		v.trackDecision(decisionID)
	})
}
