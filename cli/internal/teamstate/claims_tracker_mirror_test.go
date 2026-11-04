package teamstate

import (
	"testing"

	toml "github.com/pelletier/go-toml/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/gitutil/gittest"
)

// A claim written before 5.3.3 has no tracker_state or tracker_labels: it is
// read without them and rewritten identically.
func TestClaimWithoutTrackerMirrorIsRewrittenIdentically(t *testing.T) {
	t.Parallel()
	old := "claimed_by = 'benjamin'\nclaimed_at = 2026-07-08T14:30:00Z\nstatus = 'in_progress'\ntracker_status = 'opened'\nlabels = ['bug']\n"
	var c Claim
	require.NoError(t, toml.Unmarshal([]byte(old), &c))
	assert.Empty(t, c.TrackerState)
	assert.Empty(t, c.TrackerLabels)
	data, err := marshalClaim(&c)
	require.NoError(t, err)
	assert.Equal(t, old, string(data))
}

func TestSetClaimTrackerMirror(t *testing.T) {
	t.Parallel()
	r, _ := setupGitTestRepo(t)
	ctx := t.Context()
	createUnder(t, r, "p", Claim{TicketID: "1", ClaimedBy: "alice"})

	require.NoError(t, r.SetClaimTrackerMirror(ctx, "p", "1", "open", []string{"b", "a", "b"}))
	c, err := r.GetClaim(ctx, "p", "1")
	require.NoError(t, err)
	assert.Equal(t, "open", c.TrackerState)
	assert.Equal(t, []string{"a", "b"}, c.TrackerLabels, "sorted, once each")

	head := gittest.Run(t, r.Path(), "rev-parse", "HEAD")
	require.NoError(t, r.SetClaimTrackerMirror(ctx, "p", "1", "open", []string{"b", "a"}))
	assert.Equal(t, head, gittest.Run(t, r.Path(), "rev-parse", "HEAD"), "nothing changed: no commit")

	require.NoError(t, r.SetClaimTrackerMirror(ctx, "p", "1", "closed", nil))
	c, err = r.GetClaim(ctx, "p", "1")
	require.NoError(t, err)
	assert.Equal(t, "closed", c.TrackerState)
	assert.Empty(t, c.TrackerLabels)

	assert.ErrorIs(t, r.SetClaimTrackerMirror(ctx, "p", "404", "open", nil), ErrClaimNotFound)
}
