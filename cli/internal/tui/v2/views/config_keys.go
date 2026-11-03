package views

import (
	"github.com/datichb/openhub/cli/internal/i18n"
	"github.com/datichb/openhub/cli/internal/tui/v2/keys"
)

// The keys that the configuration views share (Settings, team detail, the MCP
// views, the models views, the project configuration): the same letter does
// the same thing in each of them, and each view reads it through these
// actions instead of a letter written in its code.
var (
	// cfgToggleKey is the space bar: it switches the setting under the cursor.
	cfgToggleKey = keys.Toggle
	cfgUndoKey   = keys.Own("config.undo", 'u', "tui.hints.undo")
	cfgSyncKey   = keys.Own("config.sync", 's', "tui.hints.sync")
	cfgTestKey   = keys.Own("config.test", 't', "tui.hints.test")
)

// toggleHint is the hint of the space bar (it has a name, not a letter).
func toggleHint() string { return keys.SpaceName() + " " + i18n.T("tui.hints.toggle") }
