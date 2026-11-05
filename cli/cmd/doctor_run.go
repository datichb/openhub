package cmd

import (
	"context"
	"sync"

	"github.com/datichb/openhub/cli/internal/tracker"
)

// A run of oh doctor (the command, the Doctor view) puts a doctorRun in the
// context of its checks, so that what two of them ask of the network is asked
// once: tracker_connection and tracker_identity both test the connection of
// the same tracker with the same token. Outside a run (the tests of a check)
// nothing is shared.

type doctorRunKey struct{}

type connOutcome struct {
	username string
	err      error
}

type doctorRun struct {
	mu   sync.Mutex
	conn map[string]connOutcome
}

// withDoctorRun starts a run of checks.
func withDoctorRun(ctx context.Context) context.Context {
	return context.WithValue(ctx, doctorRunKey{}, &doctorRun{conn: map[string]connOutcome{}})
}

func doctorRunOf(ctx context.Context) *doctorRun {
	r, _ := ctx.Value(doctorRunKey{}).(*doctorRun)
	return r
}

// testTrackerConnection is t.TestConnection, answered from the run when the
// same tracker (type, server, token) was already tested in it.
func testTrackerConnection(ctx context.Context, t tracker.Tracker, creds tracker.Config) (string, error) {
	run := doctorRunOf(ctx)
	if run == nil {
		return t.TestConnection(ctx)
	}
	key := string(creds.Type) + "\x00" + creds.BaseURL + "\x00" + creds.Token
	run.mu.Lock()
	if o, ok := run.conn[key]; ok {
		run.mu.Unlock()
		return o.username, o.err
	}
	run.mu.Unlock()
	username, err := t.TestConnection(ctx)
	run.mu.Lock()
	run.conn[key] = connOutcome{username, err}
	run.mu.Unlock()
	return username, err
}
