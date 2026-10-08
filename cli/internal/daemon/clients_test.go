package daemon

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/termlaunch"
)

// A43: the daemon tells where the client of a session runs, so that oh brings
// its window back instead of opening another client.
func TestAttachedClientsOfASession(t *testing.T) {
	e := startLifecycle(t, time.Hour, "g1", "ses_a")
	where := &termlaunch.Location{Program: "iTerm.app", TTY: "/dev/ttys007"}
	require.NoError(t, e.client.Heartbeat(e.ctx, HeartbeatRequest{ClientID: "c1", Kind: ClientAttach, Group: "g1", SessionID: "ses_a", Where: where}))
	require.NoError(t, e.client.Heartbeat(e.ctx, HeartbeatRequest{ClientID: "c2", Kind: ClientAttach, Group: "g1", SessionID: "ses_b"}))
	require.NoError(t, e.client.Heartbeat(e.ctx, HeartbeatRequest{ClientID: "tui", Kind: ClientPresence}))

	got, err := e.client.AttachedClients(e.ctx, "ses_a")
	require.NoError(t, err)
	require.Len(t, got, 1)
	assert.Equal(t, "c1", got[0].ClientID)
	assert.Equal(t, where, got[0].Where)

	require.NoError(t, e.client.ClientGone(e.ctx, "c1"))
	got, err = e.client.AttachedClients(e.ctx, "ses_a")
	require.NoError(t, err)
	assert.Empty(t, got)
}
