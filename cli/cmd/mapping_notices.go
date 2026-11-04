package cmd

import (
	"log/slog"
	"os"
	"sync"

	"github.com/datichb/openhub/cli/internal/cliout"
	"github.com/datichb/openhub/cli/internal/i18n"
	"github.com/datichb/openhub/cli/internal/teamstate"
)

// What reading the config.toml of a team clone has to say about its tracker
// mappings: one sentence per clone and per process, with the command to run
// (the teamstate package deduplicates and logs the detail of each value at
// the Info level, as `oh -v` shows it).

func init() {
	teamstate.SetMappingNoticeHandler(warnMappingNotice)
}

// mappingNoticeText is the translated sentence of a notice.
func mappingNoticeText(n teamstate.MappingNotice) string {
	switch {
	case n.Migrated > 0 && n.Invalid > 0:
		return i18n.Tf("v531.mapping.both", n.Team, n.Migrated, n.Example.Value, n.Example.Status, n.Invalid)
	case n.Migrated > 0:
		return i18n.Tf("v531.mapping.migrated", n.Team, n.Migrated, n.Example.Value, n.Example.Status)
	default:
		return i18n.Tf("v531.mapping.invalid", n.Team, n.Invalid)
	}
}

// heldNotices keeps the sentences raised while the TUI starts: the terminal
// is then not yet on the alternate screen and a line printed would sit above
// the TUI. They are shown as toasts once the shell is up.
var heldNotices struct {
	sync.Mutex
	holding bool
	texts   []string
}

// holdNoticesForTUI keeps the notices back until the shell is up
// (releaseHeldNotices).
func holdNoticesForTUI() {
	heldNotices.Lock()
	heldNotices.holding = true
	heldNotices.Unlock()
}

// releaseHeldNotices ends the hold and hands the kept sentences to show.
func releaseHeldNotices(show func(string)) {
	heldNotices.Lock()
	texts := heldNotices.texts
	heldNotices.texts, heldNotices.holding = nil, false
	heldNotices.Unlock()
	for _, t := range texts {
		show(t)
	}
}

// warnMappingNotice shows the sentence like the other warnings of the
// startup: on stderr for a command; as a toast (and an entry of the
// notifications) in the TUI, which slog does; with --log-format json through
// slog only.
func warnMappingNotice(n teamstate.MappingNotice) {
	text := mappingNoticeText(n)
	heldNotices.Lock()
	if heldNotices.holding {
		heldNotices.texts = append(heldNotices.texts, text)
		heldNotices.Unlock()
		return
	}
	heldNotices.Unlock()
	if tuiShell != nil || logJSON {
		slog.Warn(text)
		return
	}
	cliout.Warn(os.Stderr, text)
}
