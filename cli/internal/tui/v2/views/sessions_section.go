package views

import (
	"strconv"

	"github.com/datichb/openhub/cli/internal/i18n"
)

// Sessions section of the landings (P3-T19, 10 §1–§3): a short summary read
// from the cache kept by the wiring layer (no I/O on the event loop).

// SessionsSectionConfig is shared by the home, project and team landings.
type SessionsSectionConfig struct {
	// Summary returns the cached sessions summary of a scope (projectID "",
	// or a project; team: the team projects). Nil hides the section.
	Summary func(scope SessionsScope) SessionsSummary
	// Open shows the Sessions view, focused on a session ("" = none).
	Open func(sessionID string)
}

// SessionsScope selects the sessions of a landing.
type SessionsScope struct {
	ProjectID string
	TeamID    string
}

type sessionsSectionItem struct {
	Icon, Label, Desc string
	Action            func()
}

// sessionsSection returns the header and the items of the section.
func sessionsSection(cfg SessionsSectionConfig, scope SessionsScope, headerKey string) (header string, items []sessionsSectionItem, ok bool) {
	if cfg.Summary == nil {
		return "", nil, false
	}
	sum := cfg.Summary(scope)
	open := func(id string) func() {
		return func() {
			if cfg.Open != nil {
				cfg.Open(id)
			}
		}
	}
	header = i18n.T(headerKey)
	if sum.Running > 0 || sum.Decisions > 0 {
		header += "  " + SessionsBadge(sum.Running, sum.Decisions)
	}
	for _, l := range sum.Lines {
		items = append(items, sessionsSectionItem{Icon: l.Icon, Label: l.Label, Desc: l.Desc, Action: open(l.ID)})
	}
	label := i18n.T("tui.sessions.all")
	if len(sum.Lines) == 0 {
		label = i18n.T("tui.sessions.none_open")
	}
	items = append(items, sessionsSectionItem{Icon: "▸", Label: label, Desc: i18n.T("tui.sessions.all_desc"), Action: open("")})
	return header, items, true
}

// SessionsBadge renders the mode bar badge: "● 2 ⏸ 1".
func SessionsBadge(running, decisions int) string {
	if running == 0 && decisions == 0 {
		return ""
	}
	s := "● " + strconv.Itoa(running)
	if decisions > 0 {
		s += "  ⏸ " + strconv.Itoa(decisions)
	}
	return s
}
