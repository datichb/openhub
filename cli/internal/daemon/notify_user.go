package daemon

import (
	"context"

	"github.com/datichb/openhub/cli/internal/notifypolicy"
)

// The jobs the daemon runs on behalf of the commands (the tracker sync, the
// claims) can tell the user something that is not a decision: a text of their
// own, notified as the user chose for its kind (notifypolicy). The daemon puts
// itself in the context of its tasks, with the alerts.

type userNotifier interface {
	NotifyUser(kind notifypolicy.Kind, title, message string)
}

// NotifyUser notifies the user, through the daemon whose job ctx belongs to.
// It reports false when there is no daemon around (a command run by hand): the
// caller then has nothing to fall back on but its own output.
func NotifyUser(ctx context.Context, kind notifypolicy.Kind, title, message string) bool {
	n, ok := ctx.Value(alertRaiserKey{}).(userNotifier)
	if ok {
		n.NotifyUser(kind, title, message)
	}
	return ok
}

// NotifyUser queues a system notification of the given kind (shown when the
// user's settings say « system » for it).
func (d *Daemon) NotifyUser(kind notifypolicy.Kind, title, message string) {
	d.notes.direct(kind, title, message)
}
