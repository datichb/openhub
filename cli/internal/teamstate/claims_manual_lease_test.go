package teamstate

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/gitutil/gittest"
)

func TestClaimConfigManualLeaseAndRenewalCeilings(t *testing.T) {
	t.Parallel()
	load := func(t *testing.T, content string) ClaimConfig {
		t.Helper()
		r := setupTestRepo(t)
		require.NoError(t, os.WriteFile(filepath.Join(r.path, "config.toml"), []byte(content), 0o644))
		cfg, err := r.LoadConfig(t.Context())
		require.NoError(t, err)
		return cfg.Claim
	}

	t.Run("defaults", func(t *testing.T) {
		c := load(t, "[claim]\nlease_hours = 24\n")
		assert.Equal(t, 72*time.Hour, c.ManualLeaseDuration())
		assert.Equal(t, 3*24*time.Hour, c.SleepingRenewalWindow())
		assert.Equal(t, 7*24*time.Hour, c.RemoteRenewalWindow())
		assert.Equal(t, 72*time.Hour, ClaimConfig{}.ManualLeaseDuration(), "a config that did not go through LoadConfig")
	})
	t.Run("explicit values", func(t *testing.T) {
		c := load(t, "[claim]\nlease_hours_manual = 100\nsleeping_renewal_days = 5\nremote_renewal_days = 14\n")
		assert.Equal(t, 100*time.Hour, c.ManualLeaseDuration())
		assert.Equal(t, 5*24*time.Hour, c.SleepingRenewalWindow())
		assert.Equal(t, 14*24*time.Hour, c.RemoteRenewalWindow())
	})
	t.Run("a long session lease lifts the default", func(t *testing.T) {
		assert.Equal(t, 120*time.Hour, load(t, "[claim]\nlease_hours = 120\n").ManualLeaseDuration())
	})
	t.Run("a value out of its bounds is brought back", func(t *testing.T) {
		assert.Equal(t, MaxLeaseHoursManual, load(t, "[claim]\nlease_hours_manual = 99999\n").LeaseHoursManual)
		assert.Equal(t, 48, load(t, "[claim]\nlease_hours = 48\nlease_hours_manual = 10\n").LeaseHoursManual, "never shorter than the session lease")
		assert.Equal(t, MaxRenewalDays, load(t, "[claim]\nsleeping_renewal_days = 500\n").SleepingRenewalDays)
		assert.Equal(t, 0, load(t, "[claim]\nremote_renewal_days = -2\n").RemoteRenewalDays)
	})
}

func TestClaimConfigValidateManualLeaseAndCeilings(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		cfg     ClaimConfig
		wantErr string
	}{
		{"unset", ClaimConfig{LeaseHours: 24}, ""},
		{"manual lease at the default", ClaimConfig{LeaseHours: 24, LeaseHoursManual: 72}, ""},
		{"manual lease equal to the lease", ClaimConfig{LeaseHours: 24, LeaseHoursManual: 24}, ""},
		{"manual lease at the maximum", ClaimConfig{LeaseHoursManual: MaxLeaseHoursManual}, ""},
		{"manual lease negative", ClaimConfig{LeaseHoursManual: -1}, "lease_hours_manual"},
		{"manual lease too long", ClaimConfig{LeaseHoursManual: MaxLeaseHoursManual + 1}, "lease_hours_manual"},
		{"manual lease shorter than the lease", ClaimConfig{LeaseHours: 48, LeaseHoursManual: 24}, "shorter than lease_hours"},
		{"manual lease shorter than the default lease", ClaimConfig{LeaseHoursManual: 12}, "shorter than lease_hours"},
		{"sleeping ceiling", ClaimConfig{SleepingRenewalDays: 3, RemoteRenewalDays: 7}, ""},
		{"sleeping ceiling negative", ClaimConfig{SleepingRenewalDays: -1}, "sleeping_renewal_days"},
		{"sleeping ceiling too long", ClaimConfig{SleepingRenewalDays: MaxRenewalDays + 1}, "sleeping_renewal_days"},
		{"remote ceiling too long", ClaimConfig{RemoteRenewalDays: MaxRenewalDays + 1}, "remote_renewal_days"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			err := tt.cfg.Validate()
			if tt.wantErr == "" {
				assert.NoError(t, err)
				return
			}
			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.wantErr)
		})
	}
}

// A claim taken by hand (no session works on the ticket) is held for
// lease_hours_manual, nothing renews it on its own; a claim of a session keeps
// lease_hours.
func TestClaimTakenByHandHasTheLongLease(t *testing.T) {
	t.Parallel()
	repo, _ := setupGitTestRepo(t)
	ctx := context.Background()
	require.NoError(t, repo.SaveConfig(ctx, &TeamConfig{Claim: ClaimConfig{LeaseHours: 24, LeaseHoursManual: 100}}))
	leaseOf := func(id string) time.Duration {
		c, err := repo.GetClaim(ctx, "p", id)
		require.NoError(t, err)
		require.False(t, c.LeaseUntil.IsZero(), id)
		return time.Until(c.LeaseUntil.Time()).Round(time.Hour)
	}

	_, err := repo.CreateClaim(ctx, Claim{TicketID: "by-hand", Project: "p", ClaimedBy: "alice", Status: ClaimStatusInProgress})
	require.NoError(t, err)
	_, err = repo.CreateClaim(ctx, Claim{TicketID: "session", Project: "p", ClaimedBy: "alice", Status: ClaimStatusInProgress, Session: "ses_1"})
	require.NoError(t, err)
	assert.Equal(t, 100*time.Hour, leaseOf("by-hand"))
	assert.Equal(t, 24*time.Hour, leaseOf("session"))

	_, err = repo.CreateClaim(ctx, Claim{TicketID: "pool", Project: "p", Status: ClaimStatusInProgress})
	require.NoError(t, err)
	require.NoError(t, repo.ClaimPoolTicket(ctx, "p", "pool", "alice", ""))
	assert.Equal(t, 100*time.Hour, leaseOf("pool"), "a pool ticket claimed from the board")

	require.NoError(t, repo.TransferClaim(ctx, "p", "session", "bob"))
	assert.Equal(t, 100*time.Hour, leaseOf("session"), "taken over by hand")
	require.NoError(t, repo.TransferClaim(WithClaimSession(ctx, "ses_2"), "p", "by-hand", "bob"))
	assert.Equal(t, 24*time.Hour, leaseOf("by-hand"), "taken over by a session")
}

func TestRenewClaimByHand(t *testing.T) {
	t.Parallel()
	repo, bare := setupGitTestRepo(t)
	ctx := context.Background()
	now := time.Now()
	soon := InstantOf(now.Add(time.Hour))
	for _, c := range []Claim{
		{TicketID: "by-hand", ClaimedBy: "alice", Status: ClaimStatusInProgress, LeaseUntil: soon},
		{TicketID: "session", ClaimedBy: "alice", Status: ClaimStatusInProgress, LeaseUntil: soon, Session: "ses_1"},
		{TicketID: "expired", ClaimedBy: "alice", Status: ClaimStatusInProgress, LeaseUntil: InstantOf(now.Add(-time.Hour))},
		{TicketID: "bobs", ClaimedBy: "bob", Status: ClaimStatusInProgress, LeaseUntil: soon},
		{TicketID: "planned", ClaimedBy: "alice", Status: ClaimStatusPlanned},
	} {
		createUnder(t, repo, "p", c)
	}
	before := commitCount(t, bare)

	end, err := repo.RenewClaim(ctx, "p", "by-hand", "alice", now)
	require.NoError(t, err)
	assert.WithinDuration(t, now.Add(72*time.Hour), end, 2*time.Second, "not expired yet, renewed for the long lease")
	assert.Equal(t, before+1, commitCount(t, bare))
	assert.Equal(t, "lease: alice renews p/by-hand", gittest.Run(t, repo.Path(), "log", "-1", "--format=%s"))
	got, err := repo.GetClaim(ctx, "p", "by-hand")
	require.NoError(t, err)
	assert.Equal(t, InstantOf(end), got.LeaseUntil)
	assert.True(t, got.LastActivity.IsZero(), "a renewal is not activity")

	end, err = repo.RenewClaim(ctx, "p", "session", "alice", now)
	require.NoError(t, err)
	assert.WithinDuration(t, now.Add(24*time.Hour), end, 2*time.Second, "a claim of a session: the lease of a session")
	_, err = repo.RenewClaim(ctx, "p", "expired", "alice", now)
	require.NoError(t, err)

	_, err = repo.RenewClaim(ctx, "p", "bobs", "alice", now)
	assert.ErrorIs(t, err, ErrClaimNotOwned)
	_, err = repo.RenewClaim(ctx, "p", "bobs", "", now)
	assert.ErrorIs(t, err, ErrClaimNotOwned, "nobody is not the owner")
	_, err = repo.RenewClaim(ctx, "p", "planned", "alice", now)
	assert.ErrorIs(t, err, ErrNoLease)
	_, err = repo.RenewClaim(ctx, "p", "nope", "alice", now)
	assert.ErrorIs(t, err, ErrClaimNotFound)
	_, err = repo.RenewClaim(ctx, "p", "../x", "alice", now)
	require.Error(t, err)
	assert.Equal(t, before+3, commitCount(t, bare), "refusals commit nothing")
}
