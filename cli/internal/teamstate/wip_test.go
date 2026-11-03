package teamstate

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/i18n/i18ntest"
)

func claimOf(ticket, owner, status string, labels ...string) Claim {
	return Claim{TicketID: ticket, Project: "p", ClaimedBy: owner, Status: status, Labels: labels}
}

// A claim counts when its owner works on it: in progress, a claim in review
// (status or label) apart, and planned, blocked, validation, done or pool
// claims do not count.
func TestCountsInProgress(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name  string
		claim Claim
		want  bool
	}{
		{"in progress", claimOf("1", "alice", ClaimStatusInProgress), true},
		{"planned", claimOf("2", "alice", ClaimStatusPlanned), false},
		{"review status", claimOf("3", "alice", ClaimStatusReview), false},
		{"in progress with the ready-for-review label", claimOf("4", "alice", ClaimStatusInProgress, LabelReadyForReview), false},
		{"in progress with the review label", claimOf("5", "alice", ClaimStatusInProgress, LabelReview), false},
		{"validation", claimOf("6", "alice", ClaimStatusValidation), false},
		{"blocked", claimOf("7", "alice", ClaimStatusBlocked), false},
		{"done", claimOf("8", "alice", ClaimStatusDone), false},
		{"pool", claimOf("9", "", ClaimStatusInProgress), false},
		{"another label", claimOf("10", "alice", ClaimStatusInProgress, "bug"), true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tc.want, tc.claim.CountsInProgress(BoardConfig{}))
		})
	}
}

// On a board with its own columns, the active ones count, the review apart.
func TestCountsInProgressOnACustomBoard(t *testing.T) {
	t.Parallel()
	board := BoardConfig{Columns: []BoardColumnConfig{
		{ID: "todo", Name: "To do", Role: ColumnRoleInitial},
		{ID: "dev", Name: "Dev", Role: ColumnRoleActive},
		{ID: "qa", Name: "QA", Role: ColumnRoleActive},
		{ID: "stuck", Name: "Stuck", Role: ColumnRoleBlocked},
		{ID: "shipped", Name: "Shipped", Role: ColumnRoleTerminal},
	}}
	for status, want := range map[string]bool{"todo": false, "dev": true, "qa": true, "stuck": false, "shipped": false, ClaimStatusInProgress: false} {
		c := claimOf("1", "alice", status)
		assert.Equal(t, want, c.CountsInProgress(board), status)
	}
	review := claimOf("1", "alice", "dev", LabelReadyForReview)
	assert.False(t, review.CountsInProgress(board), "in review by label")
}

func TestCheckInProgressRoom(t *testing.T) {
	t.Parallel()
	claims := []Claim{
		claimOf("1", "alice", ClaimStatusInProgress),
		claimOf("2", "alice", ClaimStatusInProgress),
		claimOf("3", "alice", ClaimStatusReview),                          // in review: not counted
		claimOf("4", "alice", ClaimStatusInProgress, LabelReadyForReview), // in review by label: not counted
		claimOf("5", "alice", ClaimStatusPlanned),                         // not started
		claimOf("6", "bob", ClaimStatusInProgress),                        // someone else's
		claimOf("7", "", ClaimStatusInProgress),                           // pool
	}
	ref := func(id string) ClaimRef { return ClaimRef{Project: "p", TicketID: id} }
	board := BoardConfig{}

	// Two in progress, limit 3: one more is fine, two more are not.
	require.NoError(t, CheckInProgressRoom(claims, "alice", board, 3, []ClaimRef{ref("20")}))
	err := CheckInProgressRoom(claims, "alice", board, 3, []ClaimRef{ref("20"), ref("21")})
	var lim *InProgressLimitError
	require.ErrorAs(t, err, &lim)
	assert.Equal(t, InProgressLimitError{Member: "alice", Count: 2, Limit: 3, Adding: 2}, *lim)

	// At the limit: one more is refused.
	err = CheckInProgressRoom(claims, "alice", board, 2, []ClaimRef{ref("20")})
	require.ErrorAs(t, err, &lim)
	assert.Equal(t, 2, lim.Count)
	assert.Equal(t, 2, lim.Limit)

	// What the member already has in progress does not count twice, a ticket
	// named twice counts once.
	require.NoError(t, CheckInProgressRoom(claims, "alice", board, 2, []ClaimRef{ref("1"), ref("2")}))
	require.NoError(t, CheckInProgressRoom(claims, "alice", board, 3, []ClaimRef{ref("20"), ref("20")}))

	// A claim in review, planned or released that comes back in progress does
	// count as one more.
	require.ErrorAs(t, CheckInProgressRoom(claims, "alice", board, 2, []ClaimRef{ref("3")}), &lim)
	require.ErrorAs(t, CheckInProgressRoom(claims, "alice", board, 2, []ClaimRef{ref("5")}), &lim)

	// 0 is no limit; nothing to add is never refused.
	require.NoError(t, CheckInProgressRoom(claims, "alice", board, 0, []ClaimRef{ref("20"), ref("21"), ref("22")}))
	require.NoError(t, CheckInProgressRoom(claims, "alice", board, 1, nil))

	// The count is the member's own.
	require.NoError(t, CheckInProgressRoom(claims, "carol", board, 1, []ClaimRef{ref("20")}))
	require.ErrorAs(t, CheckInProgressRoom(claims, "bob", board, 1, []ClaimRef{ref("20")}), &lim)
}

func TestInProgressLimitErrorSaysTheCountAndTheKey(t *testing.T) {
	for _, lang := range []string{"en", "fr"} {
		i18ntest.Set(t, lang)
		msg := (&InProgressLimitError{Member: "alice", Count: 5, Limit: 5, Adding: 1}).Error()
		assert.Contains(t, msg, "alice", lang)
		assert.Contains(t, msg, "5", lang)
		assert.Contains(t, msg, "max_in_progress_per_member", lang)
		msg = (&InProgressLimitError{Member: "alice", Count: 4, Limit: 5, Adding: 3}).Error()
		assert.Contains(t, msg, "3", lang)
		assert.Contains(t, msg, "max_in_progress_per_member", lang)
	}
	var err error = &InProgressLimitError{}
	var lim *InProgressLimitError
	assert.True(t, errors.As(err, &lim))
}

// max_in_progress_per_member wins; the former max_auto_plan_per_member is read
// when the new key is absent; neither: 5; 0 is no limit.
func TestInProgressLimitOfTheTeam(t *testing.T) {
	t.Parallel()
	n := func(v int) *int { return &v }
	assert.Equal(t, 5, TrackerConfig{}.InProgressLimit(), "default")
	assert.Equal(t, 8, TrackerConfig{MaxInProgressPerMember: n(8)}.InProgressLimit())
	assert.Equal(t, 0, TrackerConfig{MaxInProgressPerMember: n(0)}.InProgressLimit(), "0 is no limit")
	assert.Equal(t, 12, TrackerConfig{MaxAutoPlanPerMember: 12}.InProgressLimit(), "former key as a fallback")
	assert.Equal(t, 8, TrackerConfig{MaxInProgressPerMember: n(8), MaxAutoPlanPerMember: 12}.InProgressLimit(), "the new key wins")
	assert.Equal(t, 0, TrackerConfig{MaxInProgressPerMember: n(0), MaxAutoPlanPerMember: 12}.InProgressLimit(), "an explicit 0 wins over the former key")
	assert.Equal(t, 5, TrackerConfig{MaxInProgressPerMember: n(-1)}.InProgressLimit(), "a negative value is no limit: default")
	assert.True(t, TrackerConfig{MaxAutoPlanPerMember: 12}.UsesFormerInProgressKey())
	assert.False(t, TrackerConfig{MaxInProgressPerMember: n(8)}.UsesFormerInProgressKey())
}

func TestLoadConfigReadsTheFormerAutoPlanLimit(t *testing.T) {
	t.Parallel()
	repo := setupTestRepo(t)
	require.NoError(t, os.WriteFile(filepath.Join(repo.path, "config.toml"), []byte("[tracker]\nmax_auto_plan_per_member = 9\n"), 0o644))
	cfg, err := repo.LoadConfig(t.Context())
	require.NoError(t, err)
	assert.Equal(t, 9, cfg.Tracker.InProgressLimit())
	assert.True(t, cfg.Tracker.UsesFormerInProgressKey())

	require.NoError(t, os.WriteFile(filepath.Join(repo.path, "config.toml"), []byte("[tracker]\nmax_in_progress_per_member = 0\n"), 0o644))
	cfg, err = repo.LoadConfig(t.Context())
	require.NoError(t, err)
	require.NotNil(t, cfg.Tracker.MaxInProgressPerMember)
	assert.Zero(t, cfg.Tracker.InProgressLimit())

	require.NoError(t, os.WriteFile(filepath.Join(repo.path, "config.toml"), []byte("[tracker]\ntype = \"gitlab\"\n"), 0o644))
	cfg, err = repo.LoadConfig(t.Context())
	require.NoError(t, err)
	assert.Equal(t, 5, cfg.Tracker.InProgressLimit())
	assert.False(t, cfg.Tracker.UsesFormerInProgressKey())
}

// Writing config.toml carries the former key to the new one and drops it; an
// explicit 0 (no limit) is written.
func TestSaveConfigWritesTheInProgressKey(t *testing.T) {
	t.Parallel()
	repo, _ := setupGitTestRepo(t)
	cfg := &TeamConfig{Tracker: TrackerConfig{MaxAutoPlanPerMember: 9}}
	require.NoError(t, repo.SaveConfig(t.Context(), cfg))
	data, err := os.ReadFile(filepath.Join(repo.path, "config.toml"))
	require.NoError(t, err)
	assert.Contains(t, string(data), "max_in_progress_per_member = 9")
	assert.NotContains(t, string(data), "max_auto_plan_per_member")
	assert.Equal(t, 9, cfg.Tracker.MaxAutoPlanPerMember, "the caller's config is left as it is")

	zero := 0
	require.NoError(t, repo.SaveConfig(t.Context(), &TeamConfig{Tracker: TrackerConfig{MaxInProgressPerMember: &zero, MaxAutoPlanPerMember: 9}}))
	data, err = os.ReadFile(filepath.Join(repo.path, "config.toml"))
	require.NoError(t, err)
	assert.Contains(t, string(data), "max_in_progress_per_member = 0")
	assert.NotContains(t, string(data), "max_auto_plan_per_member")
	got, err := repo.LoadConfig(t.Context())
	require.NoError(t, err)
	assert.Zero(t, got.Tracker.InProgressLimit())

	require.NoError(t, repo.SaveConfig(t.Context(), &TeamConfig{}))
	data, err = os.ReadFile(filepath.Join(repo.path, "config.toml"))
	require.NoError(t, err)
	assert.NotContains(t, string(data), "max_in_progress_per_member", "absent stays absent")
}
