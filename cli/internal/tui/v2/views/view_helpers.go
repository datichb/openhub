package views

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/rivo/tview"

	"github.com/datichb/openhub/cli/internal/i18n"
	"github.com/datichb/openhub/cli/internal/teamstate"
	"github.com/datichb/openhub/cli/internal/tui/theme"
	"github.com/datichb/openhub/cli/internal/tui/v2/keys"
)

// tagged colours s with a theme colour (hex), then resets the foreground
// (theme.Paint: nothing under NO_COLOR).
func tagged(hex, s string) string {
	return theme.Paint(hex, s)
}

// bracketed is s between square brackets as plain text: « [Détail] », « [f] ».
// The brackets are escaped so that tview does not take them for a tag.
func bracketed(s string) string {
	return tview.Escape(fmt.Sprintf("%c%s%c", '[', s, ']'))
}

// errorText is the toast of a failed operation (« Erreur : … »).
func errorText(err error) string {
	return i18n.Tf("tui.settings.error_detail", err.Error())
}

// confirmDiscard runs reload at once when nothing is unsaved, and after a
// confirmation otherwise (r drops the unsaved changes of a view).
func confirmDiscard(sh ShellAccess, dirty bool, reload func()) {
	if !dirty || sh == nil {
		reload()
		return
	}
	sh.Confirm(i18n.T("tui.config.discard_title"), i18n.T("tui.config.discard_detail"), true, reload)
}

// shellContext is the context of the shell, which ends with it (so that the
// reads of a view stop with it); Background without a shell.
func shellContext(sh ShellAccess) context.Context {
	if sh != nil {
		return sh.Context()
	}
	return context.Background()
}

// readTeamConfig reads the configuration of the team from the clone of its
// team-state. A team-state that cannot be read (no clone, config.toml that does
// not parse) is an error and never an empty configuration: a view that showed
// one would let the user save it over the real one. nil, nil when there is no
// team-state to read. It reads a file: call it off the event loop.
func readTeamConfig(ctx context.Context, tc TeamResolution) (*teamstate.TeamConfig, error) {
	if !tc.hasState() {
		return nil, nil
	}
	repo := teamstate.NewRepo(tc.StateRepo, tc.StatePath)
	if !repo.IsCloned() {
		return nil, errors.New(i18n.T("v533b.team_config.not_cloned"))
	}
	return repo.LoadConfig(ctx)
}

// unreadableConfigText is what a view says when the team config cannot be read.
func unreadableConfigText(err error) string {
	return i18n.Tf("v533b.team_config.unreadable", err.Error())
}

// saveRefusedText is the reason a save is refused when the team config is not
// there: it could not be read, or the read has not ended.
func saveRefusedText(err error) string {
	if err == nil {
		err = errors.New(i18n.T("v533b.team_config.not_loaded"))
		return i18n.Tf("v533b.team_config.save_refused", err.Error())
	}
	return i18n.Tf("v533b.team_config.save_refused", unreadableConfigText(err))
}

// unreadableDetail is the detail pane of a view whose team config cannot be
// read: what happened, that nothing can be saved, how to get out.
func unreadableDetail(err error, extra ...string) string {
	var d detailText
	d.title(i18n.T("v533b.team_config.unreadable_title"))
	d.para(tview.Escape(unreadableConfigText(err)))
	d.para(i18n.T("v533b.team_config.unreadable_detail"))
	d.keys(append(keys.Pairs(keys.Refresh, keys.Back), extra...)...)
	return d.String()
}

// confirmReloadOnConflict handles a save refused because a colleague changed
// the same settings meanwhile (teamstate.ErrConfigConflict): nothing was
// written; the user is asked to reload (which drops the edits). It reports
// whether err was such a conflict.
func confirmReloadOnConflict(sh ShellAccess, err error, reload func()) bool {
	c, ok := teamstate.IsConfigConflict(err)
	if !ok {
		return false
	}
	if sh != nil {
		sh.Confirm(i18n.T("v533b.team_config.conflict_title"),
			i18n.Tf("v533b.team_config.conflict_detail", strings.Join(c.Keys, ", ")), true, reload)
	}
	return true
}

// modeBarRefresher is implemented by a shell that can redraw its mode bar
// from the current view (mocks need not).
type modeBarRefresher interface{ RefreshModeBar() }

// refreshModeBar redraws the mode bar after a view got data that it shows
// there (ModeInfo). Event loop only.
func refreshModeBar(sh ShellAccess) {
	if r, ok := sh.(modeBarRefresher); ok {
		r.RefreshModeBar()
	}
}

// refreshHints redraws the hints line after a view changed what its
// StatusHints says (a board that finds Beads is not set up). Event loop only.
func refreshHints(sh ShellAccess) {
	if r, ok := sh.(HintsRefresher); ok {
		r.RefreshHints()
	}
}
