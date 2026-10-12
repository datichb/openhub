package daemon

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/domain"
	"github.com/datichb/openhub/cli/internal/gateway"
	"github.com/datichb/openhub/cli/internal/storage/sqlite"
)

// The leases of the tickets an open session claimed are refreshed with the
// person of the session (owner of the lease); a closed ticket or an ended
// session is no longer refreshed.
func TestDaemonRefreshesBeadsLeases(t *testing.T) {
	ctx := context.Background()
	st, err := sqlite.Open(filepath.Join(t.TempDir(), "oh.db"))
	require.NoError(t, err)
	t.Cleanup(func() { st.Close() })
	project, err := filepath.EvalSymlinks(t.TempDir())
	require.NoError(t, err)
	_, err = st.DB().Exec(`INSERT INTO projects (id, name, path) VALUES ('p1','p1',?)`, project)
	require.NoError(t, err)
	sessions := sqlite.NewSessionStore(st)
	require.NoError(t, sessions.Create(ctx, &domain.Session{ID: "ses_a", ProjectID: "p1", Status: domain.SessionStatusRunning, GroupKey: "g1", State: domain.RunActive}))

	calls := filepath.Join(t.TempDir(), "calls")
	bd := filepath.Join(t.TempDir(), "bd")
	require.NoError(t, os.WriteFile(bd, []byte(fakeWhere+"printf '%s\\n' \"$*\" >> "+calls+"\n"), 0o755))
	require.NoError(t, os.Mkdir(filepath.Join(project, ".beads"), 0o700))
	store, err := gateway.OpenStore("")
	require.NoError(t, err)
	gw := &gateway.Beads{Store: store, Binary: bd}
	store.ResolveBase = gw.ResolveBase
	_, err = store.Issue(gateway.Grant{SessionID: "ses_a", GroupKey: "g1", Location: project, Human: "Ada"})
	require.NoError(t, err)
	g, ok := store.Root("ses_a")
	require.True(t, ok)
	d := &Daemon{opts: Options{Sessions: sessions}, gateway: store, beadsGW: gw}

	d.trackLease(g, gateway.BeadsOp{Kind: gateway.OpClaim, IDs: []string{"pt-1", "pt-2"}})
	d.trackLease(g, gateway.BeadsOp{Kind: gateway.OpClose, IDs: []string{"pt-2"}})
	d.refreshLeases(ctx)
	read := func() string { b, _ := os.ReadFile(calls); return strings.TrimSpace(string(b)) }
	assert.Equal(t, "--actor=Ada heartbeat pt-1", read())

	s, _ := sessions.Get(ctx, "ses_a")
	s.State = domain.RunStopped
	require.NoError(t, sessions.Update(ctx, s))
	d.refreshLeases(ctx)
	assert.Equal(t, "--actor=Ada heartbeat pt-1", read(), "an ended session is no longer refreshed")
	assert.Empty(t, d.leases["ses_a"])
}
