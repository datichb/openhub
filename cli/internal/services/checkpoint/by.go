package checkpoint

import (
	"log/slog"
	"slices"
	"strings"

	"github.com/datichb/openhub/cli/internal/i18n"
	"github.com/datichb/openhub/cli/internal/sessionspec"
)

// byWarning warns when an agent passes a checkpoint its workflow gives to
// other agents (L3, `by:`): the call goes on (oh never blocks on it), the
// agent and the log are told. "" when the checkpoint names no agent, the
// caller is unknown or is one of them.
func byWarning(sessionID string, c sessionspec.CheckpointDef, agent string) string {
	if agent == "" || len(c.By) == 0 || slices.Contains(c.By, agent) {
		return ""
	}
	by := strings.Join(c.By, ", ")
	slog.Warn("checkpoint: passed by an agent the workflow does not name", "session", sessionID, "checkpoint", c.ID, "agent", agent, "by", by)
	return i18n.Tf("cmd.daemon.workflow.by_other_agent", c.ID, agent, by)
}
