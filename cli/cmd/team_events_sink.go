package cmd

import (
	"context"
	"log/slog"
	"path/filepath"
	"time"

	"github.com/datichb/openhub/cli/internal/config"
	"github.com/datichb/openhub/cli/internal/i18n"
	"github.com/datichb/openhub/cli/internal/notify"
	"github.com/datichb/openhub/cli/internal/teamstate"
	"github.com/datichb/openhub/cli/internal/tui/v2/shell"
)

// Every event a committed transaction wrote to the activity of a team is
// announced to the webhooks of that team (and mentions the person it is for):
// teamstate hands it to teamEventSink, whoever wrote it — a command, the
// daemon, an agent through the MCP server, the TUI. The destinations choose
// the events they receive (notification.events, destinations[].events).

func init() { teamstate.SetEventSink(teamEventSink) }

// eventSinkShutdownWait is how long a command waits, when it ends, for the
// announcements of its events to be delivered.
const eventSinkShutdownWait = 15 * time.Second

// notifyOutboxDir holds the messages that could not be delivered, until the
// daemon tries them again.
func notifyOutboxDir() string { return notify.OutboxDir(config.HubDir()) }

// notifyOutbox is the queue of the messages to deliver later; a message given
// up is said in the notification centre.
func notifyOutbox() *notify.Outbox {
	box := notify.NewOutbox(notifyOutboxDir())
	box.OnGiveUp = func(team, text string, err error) {
		_ = shell.AppendToFile(shell.NotificationsFilePath(), shell.ToastError, i18n.Tf("v54n.notify.gave_up", team, text, err))
	}
	return box
}

// newTeamDispatcher is the dispatcher of a team: the names of its members and
// of the hub projects in the text, the handles for the mentions, and the
// outbox for what cannot be delivered now.
func newTeamDispatcher(ctx context.Context, repo teamstate.EventSource, cfg *teamstate.TeamConfig) *notify.Dispatcher {
	projectName := func(id string) string {
		a := TryApp()
		if a == nil || a.Projects == nil {
			return ""
		}
		if p, err := a.Projects.Get(ctx, id); err == nil && p != nil {
			return p.Name
		}
		return ""
	}
	return notify.ForTeam(ctx, repo, cfg, filepath.Base(repo.Path()), notifyOutbox(), projectName)
}

func teamEventSink(ctx context.Context, repo teamstate.EventSource, e teamstate.Event) {
	cfg, err := repo.LoadConfig(ctx)
	if err != nil || !cfg.Notification.Enabled {
		return
	}
	if rep := newTeamDispatcher(ctx, repo, cfg).Send(ctx, e); rep.Err != nil {
		slog.Warn("notify.event.failed", "event", e.Type, "error", rep.Err)
	}
}
