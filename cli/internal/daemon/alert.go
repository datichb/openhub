package daemon

import (
	"context"
	"log/slog"

	"github.com/datichb/openhub/cli/internal/domain"
)

// Alerts for the jobs the daemon runs on behalf of the commands (the signal of
// life of the claims, its tasks): such a job knows nothing of the stores of the
// daemon, but it can tell the user something only they can fix (the token of
// the team-state expired). RaiseAlert puts it in « À traiter » of a session, as
// the alerts of the daemon itself (✗, dismissed by the user only). The daemon
// puts the raiser in the context it gives to OnClaimsHeartbeat and to the tasks.

// Alert is something only the user can fix, shown under a session.
type Alert struct {
	// SessionID is the session the alert is shown under.
	SessionID string
	// Kind says what the alert is about ("team_access"); Ref tells two alerts
	// of the same kind on the same session apart (the repository, and the
	// state of the credentials it was refused with): the same Ref is raised
	// once, even when the user dismissed it, a new one (a renewed token that is
	// refused again) is raised.
	Kind, Ref string
	// Title and Message are what the user reads, in their language.
	Title, Message string
}

// AlertRaiser raises the alerts of a running daemon.
type AlertRaiser interface {
	RaiseAlert(ctx context.Context, a Alert) bool
}

type alertRaiserKey struct{}

// WithAlerts returns ctx carrying r: RaiseAlert finds it there.
func WithAlerts(ctx context.Context, r AlertRaiser) context.Context {
	return context.WithValue(ctx, alertRaiserKey{}, r)
}

// RaiseAlert raises a through the daemon whose job ctx belongs to. It reports
// whether the alert was recorded now: false when there is no daemon around (a
// command run by hand), when the stores are missing, or when this alert was
// raised before.
func RaiseAlert(ctx context.Context, a Alert) bool {
	r, ok := ctx.Value(alertRaiserKey{}).(AlertRaiser)
	return ok && r.RaiseAlert(ctx, a)
}

// RaiseAlert records the alert under its session and shows it.
func (d *Daemon) RaiseAlert(ctx context.Context, a Alert) bool {
	if d.opts.Decisions == nil || d.opts.Sessions == nil || a.SessionID == "" || a.Kind == "" {
		return false
	}
	sess, err := d.opts.Sessions.Get(ctx, a.SessionID)
	if err != nil {
		return false
	}
	ref := a.Kind
	if a.Ref != "" {
		ref += ":" + a.Ref
	}
	dec := domain.Decision{SessionID: a.SessionID, GroupKey: sess.GroupKey, Kind: domain.DecisionError, ToolRef: ref,
		Payload: domain.DecisionPayload{Title: a.Title, Message: a.Message, Data: map[string]any{dataOhAlert: a.Kind}}}
	dec.ID = domain.DecisionID(dec.Kind, a.SessionID, dec.ToolRef)
	if prev, gerr := d.opts.Decisions.Get(ctx, dec.ID); gerr == nil && prev != nil {
		return false // already raised (and maybe dismissed)
	}
	if uerr := d.opts.Decisions.Upsert(ctx, &dec); uerr != nil {
		slog.Warn("ohd: alert not recorded", "kind", a.Kind, "session", a.SessionID, "error", uerr)
		return false
	}
	d.wmu.Lock()
	w := d.watchers[sess.GroupKey]
	d.wmu.Unlock()
	if w != nil {
		w.refreshAlerts(ctx, a.SessionID)
		return true
	}
	d.feed.publishChange(domain.SessionChange{SessionID: a.SessionID, GroupKey: sess.GroupKey, Decisions: true})
	d.notes.requestScan()
	return true
}
