package cmd

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/config"
	"github.com/datichb/openhub/cli/internal/daemon"
	"github.com/datichb/openhub/cli/internal/i18n/i18ntest"
)

func expiryOf(t *testing.T, now time.Time) (*claimExpiryWatch, *tellingDaemon, context.Context) {
	t.Helper()
	a, repo := setupBeat(t)
	claimsCreate(t, repo, "pt-manual", "testuser", "")
	claimsCreate(t, repo, "pt-other", "colleague", "")
	w := newClaimExpiryWatch(a)
	w.config = func() *config.Config { return a.Config }
	w.now = func() time.Time { return now }
	d := &tellingDaemon{}
	return w, d, daemon.WithAlerts(t.Context(), d)
}

// A claim of mine whose lease ends soon, then has ended, is announced once each
// time; a claim of a colleague or one with time left is not.
func TestClaimExpiryTellsTheOwnerOnce(t *testing.T) {
	i18ntest.Set(t, "fr")
	start := time.Now()
	w, d, ctx := expiryOf(t, start)

	w.run(ctx)
	assert.Empty(t, d.told, "the lease has time left")

	// 72 h of manual lease, minus one hour: it ends soon.
	w.now = func() time.Time { return start.Add(71 * time.Hour) }
	w.run(ctx)
	w.run(ctx)
	require.Len(t, d.told, 1, "once, and only mine")
	assert.Equal(t, "Réservation bientôt expirée", d.told[0][0])
	assert.Contains(t, d.told[0][1], "pt-manual")

	w.now = func() time.Time { return start.Add(80 * time.Hour) }
	w.run(ctx)
	w.run(ctx)
	require.Len(t, d.told, 2)
	assert.Equal(t, "Réservation expirée", d.told[1][0])
}

func TestClaimExpiryTaskIsRegisteredInTheDaemon(t *testing.T) {
	var opts daemon.Options
	claimExpiryDaemonOptions(nil, &opts)
	notifyOutboxDaemonOptions(&opts)
	var names []string
	for _, task := range opts.Tasks {
		names = append(names, task.Name)
	}
	assert.Equal(t, []string{"claims-expiry", "notify-outbox"}, names)
}
