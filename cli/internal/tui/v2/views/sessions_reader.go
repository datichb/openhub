package views

import (
	"fmt"
	"strings"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"

	"github.com/datichb/openhub/cli/internal/i18n"
	"github.com/datichb/openhub/cli/internal/tui/theme"
	"github.com/datichb/openhub/cli/internal/tui/v2/keys"
)

// textReader is the simple reader of the session tabs (trace, reports):
// plain text that scrolls, / to search (case-insensitive), n/N to the
// next/previous matching line, g/G to the beginning/end. The text comes
// from agents and tools: always escaped.
type textReader struct {
	root  *tview.Flex
	head  *tview.TextView
	text  *tview.TextView
	lines []string
	query string
	hits  []int // matching line indexes
	cur   int   // index in hits (-1: none)
	empty string
	state stateKind // what empty says: loading, nothing to show, or the failure
	// ask prompts for the search (ShowInputModal; tests replace it).
	ask func(title, current string, done func(string))
}

func newTextReader() *textReader {
	r := &textReader{cur: -1}
	r.head = tview.NewTextView().SetDynamicColors(true)
	r.head.SetBackgroundColor(theme.BgPanel)
	r.text = tview.NewTextView().SetDynamicColors(true).SetRegions(true).SetWrap(true).SetScrollable(true)
	r.text.SetBackgroundColor(theme.BgPanel)
	r.root = tview.NewFlex().SetDirection(tview.FlexRow).
		AddItem(r.head, 1, 0, false).
		AddItem(r.text, 0, 1, true)
	r.root.SetBackgroundColor(theme.BgPanel)
	r.render()
	return r
}

// SetText replaces the text (search kept, back to the beginning).
func (r *textReader) SetText(raw string) {
	raw = strings.TrimRight(strings.ReplaceAll(raw, "\r\n", "\n"), "\n")
	r.lines = nil
	if raw != "" {
		r.lines = strings.Split(raw, "\n")
	}
	r.search(r.query)
	r.text.ScrollToBeginning()
}

// SetEmpty is the text shown without lines when there is nothing to read.
func (r *textReader) SetEmpty(msg string) { r.SetState(stateEmpty, msg) }

// SetState is the text shown without lines: loading, nothing to read, or the
// reason the text could not be read (stateText).
func (r *textReader) SetState(kind stateKind, msg string) {
	r.state, r.empty = kind, msg
	r.render()
}

func (r *textReader) search(q string) {
	r.query = strings.TrimSpace(q)
	r.hits, r.cur = nil, -1
	if r.query != "" {
		low := strings.ToLower(r.query)
		for i, l := range r.lines {
			if strings.Contains(strings.ToLower(l), low) {
				r.hits = append(r.hits, i)
			}
		}
		if len(r.hits) > 0 {
			r.cur = 0
		}
	}
	r.render()
	r.showCurrent()
}

func (r *textReader) move(delta int) {
	if len(r.hits) == 0 {
		return
	}
	r.cur = (r.cur + delta + len(r.hits)) % len(r.hits)
	r.render()
	r.showCurrent()
}

func (r *textReader) showCurrent() {
	if r.cur < 0 {
		r.text.Highlight()
		return
	}
	r.text.Highlight(fmt.Sprintf("h%d", r.cur))
	r.text.ScrollToHighlight()
}

// HandleKey takes the keys of the reader; moves go to the text view.
func (r *textReader) HandleKey(ev *tcell.EventKey) *tcell.EventKey {
	switch {
	case keys.Search.Matches(ev):
		if r.ask != nil {
			r.ask(i18n.T("tui.sessions.reader.search_title"), r.query, r.search)
		}
		return nil
	case keys.NextMatch.Matches(ev):
		r.move(1)
		return nil
	case keys.PrevMatch.Matches(ev):
		r.move(-1)
		return nil
	case keys.First.Matches(ev):
		r.text.ScrollToBeginning()
		return nil
	case keys.Last.Matches(ev):
		r.text.ScrollToEnd()
		return nil
	}
	return ev
}

func (r *textReader) render() {
	switch {
	case r.query == "":
		r.head.SetText(muted(keys.Hints(keys.Search.Hint(), keys.NextMatch.Keys()+"/"+keys.PrevMatch.Keys()+" "+
			keys.NextMatch.Label()+"/"+keys.PrevMatch.Label(), keys.Ends.Hint())))
	case len(r.hits) == 0:
		r.head.SetText(tview.Escape("/"+r.query) + "  " + muted(i18n.T("tui.sessions.reader.no_match")))
	default:
		r.head.SetText(tview.Escape("/"+r.query) + "  " + muted(i18n.Tf("tui.sessions.reader.matches", r.cur+1, len(r.hits))))
	}
	if len(r.lines) == 0 {
		if r.empty == "" && r.state != stateError {
			r.text.SetText("")
			return
		}
		r.text.SetText(stateText(r.state, r.empty))
		return
	}
	hit := map[int]int{}
	for i, l := range r.hits {
		hit[l] = i
	}
	var b strings.Builder
	for i, l := range r.lines {
		if i > 0 {
			b.WriteByte('\n')
		}
		h, ok := hit[i]
		if !ok {
			b.WriteString(tview.Escape(l))
			continue
		}
		fmt.Fprintf(&b, `["h%d"]%s[""]`, h, markQuery(l, r.query))
	}
	r.text.SetText(b.String())
}

// markQuery escapes a line and colours the occurrences of q.
func markQuery(line, q string) string {
	low, lq := strings.ToLower(line), strings.ToLower(q)
	if len(low) != len(line) {
		return tview.Escape(line) // lower case of another length: no marks
	}
	var b strings.Builder
	for {
		i := strings.Index(low, lq)
		if i < 0 || lq == "" {
			b.WriteString(tview.Escape(line))
			return b.String()
		}
		b.WriteString(tview.Escape(line[:i]))
		b.WriteString(theme.Paint(theme.WarningHex, tview.Escape(line[i:i+len(lq)])))
		line, low = line[i+len(lq):], low[i+len(lq):]
	}
}
