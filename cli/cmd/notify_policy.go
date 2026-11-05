package cmd

import (
	"context"
	"log/slog"
	"os"
	"sync"
	"time"

	"github.com/datichb/openhub/cli/internal/app"
	"github.com/datichb/openhub/cli/internal/config"
	"github.com/datichb/openhub/cli/internal/daemon"
	"github.com/datichb/openhub/cli/internal/i18n"
	"github.com/datichb/openhub/cli/internal/notifypolicy"
	"github.com/datichb/openhub/cli/internal/sysnotify"
	"github.com/datichb/openhub/cli/internal/tui/v2/shell"
)

// What the user chose about being told ([notifications] of hub.toml) is read
// again when the file changes: the daemon and the TUI live for days, a setting
// applies at the next notification, without a restart.

// policyReader gives the policy of hub.toml as it is now. The file is read
// again only when its date or size changed.
type policyReader struct {
	read func() (*config.Config, error)
	path func() string

	mu    sync.Mutex
	mtime time.Time
	size  int64
	have  bool
	cur   notifypolicy.Settings
}

func newPolicyReader(initial notifypolicy.Settings) *policyReader {
	return &policyReader{read: func() (*config.Config, error) { return readHubConfig() }, path: config.ConfigPath, cur: initial}
}

// Get is the policy now; if hub.toml cannot be read, the last one.
func (r *policyReader) Get() notifypolicy.Settings {
	r.mu.Lock()
	defer r.mu.Unlock()
	fi, err := os.Stat(r.path())
	if err != nil {
		return r.cur
	}
	if r.have && fi.ModTime().Equal(r.mtime) && fi.Size() == r.size {
		return r.cur
	}
	cfg, err := r.read()
	if err != nil || cfg == nil {
		return r.cur
	}
	r.mtime, r.size, r.have = fi.ModTime(), fi.Size(), true
	r.cur = cfg.NotifyPolicy()
	return r.cur
}

// hubNotifyPolicy is the policy of the process; started with the configuration
// of the start-up.
var hubNotifyPolicy *policyReader

func notifyPolicyOf(a *app.App) *policyReader {
	if hubNotifyPolicy == nil {
		hubNotifyPolicy = newPolicyReader(a.Config.NotifyPolicy())
	}
	return hubNotifyPolicy
}

// systemNotifier is the backend of the system notifications (a test replaces it).
var systemNotifier = sysnotify.New

// daemonNotifier returns the desktop notifier of the daemon: whether it
// notifies is decided by the policy, at each notification.
func daemonNotifier() daemon.NotifyFunc {
	n := systemNotifier()
	return func(ctx context.Context, note daemon.Notification) error {
		return n.Notify(ctx, sysnotify.Note{Title: note.Title, Message: note.Message, Group: note.Group, Sound: note.Sound})
	}
}

// lastSyncLine is the last line the notification centre got from a sync, by team.
var (
	lastSyncMu   sync.Mutex
	lastSyncLine = map[string]string{}
)

// tellTrackerSync tells the user what an automatic sync of the daemon did: a
// line in the notification centre (the same text twice in a row is not
// repeated), and a system notification when the user chose « system » for the
// tracker sync (the default is the toast of the TUI, shown by the board
// itself). A failure is told once: the caller is silent while it repeats.
func tellTrackerSync(ctx context.Context, team, text string, failed bool) {
	title := i18n.Tf("v54n.sync.title", team)
	level := shell.ToastSuccess
	if failed {
		title, level = i18n.Tf("v54n.sync.failed_title", team), shell.ToastError
	}
	lastSyncMu.Lock()
	same := lastSyncLine[team] == title+text
	lastSyncLine[team] = title + text
	lastSyncMu.Unlock()
	if same {
		return
	}
	_ = shell.AppendToFile(shell.NotificationsFilePath(), level, title+" · "+text)
	daemon.NotifyUser(ctx, notifypolicy.KindTrackerSync, title, text)
}

// notifyOutboxEvery is how often the daemon tries again the messages that
// could not reach a webhook (each one has its own growing pause).
const notifyOutboxEvery = time.Minute

// notifyOutboxDaemonOptions adds the retry of the undelivered webhook messages
// to the tasks of the daemon.
func notifyOutboxDaemonOptions(opts *daemon.Options) {
	box := notifyOutbox()
	opts.Tasks = append(opts.Tasks, daemon.Task{Name: "notify-outbox", Every: notifyOutboxEvery, Run: func(ctx context.Context) {
		if res := box.Flush(ctx); res.Sent+res.GaveUp > 0 {
			slog.Info("ohd: webhook messages tried again", "sent", res.Sent, "given_up", res.GaveUp, "waiting", res.Retry)
		}
	}})
}
