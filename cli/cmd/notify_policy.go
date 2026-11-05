package cmd

import (
	"context"
	"os"
	"sync"
	"time"

	"github.com/datichb/openhub/cli/internal/app"
	"github.com/datichb/openhub/cli/internal/config"
	"github.com/datichb/openhub/cli/internal/daemon"
	"github.com/datichb/openhub/cli/internal/notifypolicy"
	"github.com/datichb/openhub/cli/internal/sysnotify"
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
