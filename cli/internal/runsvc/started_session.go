package runsvc

import "context"

type startedSessionKey struct{}

// WithStartedSession returns a context that carries the ID of the session
// that has just started: OnTicketsStarted receives it, so that the team
// claims it takes are tied to that session.
func WithStartedSession(ctx context.Context, sessionID string) context.Context {
	return context.WithValue(ctx, startedSessionKey{}, sessionID)
}

// StartedSession is the session carried by ctx ("" = none).
func StartedSession(ctx context.Context) string {
	id, _ := ctx.Value(startedSessionKey{}).(string)
	return id
}
