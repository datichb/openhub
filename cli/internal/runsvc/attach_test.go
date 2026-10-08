package runsvc

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/adapters"
	"github.com/datichb/openhub/cli/internal/daemon"
	"github.com/datichb/openhub/cli/internal/termlaunch"
)

type clientsDaemon struct {
	*fakeDaemon
	clients map[string][]daemon.AttachedClient
	asked   []string
}

func (d *clientsDaemon) AttachedClients(_ context.Context, sessionID string) ([]daemon.AttachedClient, error) {
	d.asked = append(d.asked, sessionID)
	return d.clients[sessionID], nil
}

func withFocus(t *testing.T, fn func(context.Context, termlaunch.Location) error) {
	t.Helper()
	prev := focusWindow
	t.Cleanup(func() { focusWindow = prev })
	focusWindow = fn
}

// A43: opening a session that already has a client brings its window back
// instead of opening another client.
func TestAttachFocusesTheOpenClient(t *testing.T) {
	where := termlaunch.Location{Program: "iTerm.app", TTY: "/dev/ttys009"}
	dc := &clientsDaemon{fakeDaemon: &fakeDaemon{}, clients: map[string][]daemon.AttachedClient{
		"ses_a": {{ClientID: "old", SessionID: "ses_a"}, {ClientID: "c1", SessionID: "ses_a", Where: &where}},
	}}
	svc := &Service{Daemon: func(context.Context) (DaemonClient, error) { return dc, nil }}
	var focused []termlaunch.Location
	withFocus(t, func(_ context.Context, l termlaunch.Location) error {
		focused = append(focused, l)
		return nil
	})
	m, err := svc.Attach(context.Background(), "ses_a", "/p", termlaunch.PrefAuto, termlaunch.ITermTab, "")
	require.NoError(t, err)
	assert.Equal(t, termlaunch.MethodFocused, m)
	assert.Equal(t, []termlaunch.Location{where}, focused, "a client without location is skipped")
}

func TestFocusAttachedWithoutWindow(t *testing.T) {
	dc := &clientsDaemon{fakeDaemon: &fakeDaemon{}, clients: map[string][]daemon.AttachedClient{
		"ses_a": {{ClientID: "c1", SessionID: "ses_a", Where: &termlaunch.Location{TTY: "/dev/ttys009"}}},
	}}
	svc := &Service{Daemon: func(context.Context) (DaemonClient, error) { return dc, nil }}
	withFocus(t, func(context.Context, termlaunch.Location) error { return termlaunch.ErrNotFound })
	assert.False(t, svc.FocusAttached(context.Background(), "ses_a"), "window closed: a client is opened")
	assert.False(t, svc.FocusAttached(context.Background(), "ses_b"), "no client")

	plain := &Service{Daemon: func(context.Context) (DaemonClient, error) { return &fakeDaemon{}, nil }}
	assert.False(t, plain.FocusAttached(context.Background(), "ses_a"), "daemon without the client list")
	assert.False(t, (&Service{}).FocusAttached(context.Background(), "ses_a"))
}

type singleSessionAdapter struct{ *rtAdapter }

func (a singleSessionAdapter) SingleSessionAttachCommand(_ adapters.ServerHandle, sessionID string, base []string) ([]string, []string) {
	return []string{"tool", sessionID}, []string{"SINGLE=1", "BASE=" + base[0]}
}

func TestAttachCommandUsesSingleSessionClient(t *testing.T) {
	f := newRTFixture(t)
	ctx := context.Background()
	res, err := f.svc.StartSession(ctx, f.request(f.project))
	require.NoError(t, err)

	argv, env, err := f.svc.AttachCommand(ctx, res.SessionID)
	require.NoError(t, err)
	assert.Nil(t, argv, "plain adapter: AttachCommand")
	assert.Nil(t, env)

	f.svc.Adapter = singleSessionAdapter{f.ad}
	argv, env, err = f.svc.AttachCommand(ctx, res.SessionID)
	require.NoError(t, err)
	assert.Equal(t, []string{"tool", res.SessionID}, argv)
	assert.Equal(t, "SINGLE=1", env[0])
	assert.NotEqual(t, "BASE=", env[1], "the inherited environment is passed")
}
