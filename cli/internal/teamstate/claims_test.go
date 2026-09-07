package teamstate

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCreateClaimBasic(t *testing.T) {
	repo := setupTestRepo(t)

	// Create claims directory and file manually (no git needed for unit test)
	dir := filepath.Join(repo.path, "projects", "T-SRU", "claims")
	require.NoError(t, os.MkdirAll(dir, 0o755))

	claim := `claimed_by = "benjamin"
claimed_at = 2026-07-07T14:30:00Z
worktree = "feat/SRU-142-user-auth"
status = "in_progress"
`
	path := repo.claimFilePath("T-SRU", "SRU-142")
	require.NoError(t, os.WriteFile(path, []byte(claim), 0o644))

	// GetClaim should find it
	got, err := repo.GetClaim("T-SRU", "SRU-142")
	require.NoError(t, err)
	assert.Equal(t, "benjamin", got.ClaimedBy)
	assert.Equal(t, "feat/SRU-142-user-auth", got.Worktree)
	assert.Equal(t, "in_progress", got.Status)
	assert.Equal(t, "SRU-142", got.TicketID)
	assert.Equal(t, "T-SRU", got.Project)
}

func TestGetClaimNotFound(t *testing.T) {
	repo := setupTestRepo(t)

	_, err := repo.GetClaim("T-SRU", "NONEXISTENT")
	assert.ErrorIs(t, err, ErrClaimNotFound)
}

func TestListClaimsForProject(t *testing.T) {
	repo := setupTestRepo(t)

	dir := filepath.Join(repo.path, "projects", "T-SRU", "claims")
	require.NoError(t, os.MkdirAll(dir, 0o755))

	claim1 := `claimed_by = "benjamin"
claimed_at = 2026-07-07T14:30:00Z
status = "in_progress"
`
	claim2 := `claimed_by = "alice"
claimed_at = 2026-07-07T15:00:00Z
status = "review"
`
	require.NoError(t, os.WriteFile(filepath.Join(dir, "SRU-142.toml"), []byte(claim1), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "SRU-155.toml"), []byte(claim2), 0o644))

	claims, err := repo.ListClaims("T-SRU")
	require.NoError(t, err)
	assert.Len(t, claims, 2)

	byTicket := map[string]Claim{}
	for _, c := range claims {
		byTicket[c.TicketID] = c
	}
	assert.Equal(t, "benjamin", byTicket["SRU-142"].ClaimedBy)
	assert.Equal(t, "alice", byTicket["SRU-155"].ClaimedBy)
	assert.Equal(t, "review", byTicket["SRU-155"].Status)
}

func TestListClaimsAllProjects(t *testing.T) {
	repo := setupTestRepo(t)

	dir1 := filepath.Join(repo.path, "projects", "T-SRU", "claims")
	dir2 := filepath.Join(repo.path, "projects", "OTHER", "claims")
	require.NoError(t, os.MkdirAll(dir1, 0o755))
	require.NoError(t, os.MkdirAll(dir2, 0o755))

	claim := `claimed_by = "benjamin"
claimed_at = 2026-07-07T14:30:00Z
status = "in_progress"
`
	require.NoError(t, os.WriteFile(filepath.Join(dir1, "SRU-142.toml"), []byte(claim), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(dir2, "OTHER-10.toml"), []byte(claim), 0o644))

	claims, err := repo.ListClaims("")
	require.NoError(t, err)
	assert.Len(t, claims, 2)
}

func TestListClaimsEmptyProject(t *testing.T) {
	repo := setupTestRepo(t)

	claims, err := repo.ListClaims("NONEXISTENT")
	require.NoError(t, err)
	assert.Empty(t, claims)
}

func TestClaimFilePath(t *testing.T) {
	repo := NewRepo("", "/repo")

	tests := []struct {
		project  string
		ticketID string
		expected string
	}{
		{"T-SRU", "SRU-142", "/repo/projects/T-SRU/claims/SRU-142.toml"},
		{"T-SRU", "feat/123", "/repo/projects/T-SRU/claims/feat_123.toml"},
	}
	for _, tt := range tests {
		got := repo.claimFilePath(tt.project, tt.ticketID)
		assert.Equal(t, tt.expected, got)
	}
}

func TestClaimWithMetadata(t *testing.T) {
	repo := setupTestRepo(t)

	dir := filepath.Join(repo.path, "projects", "T-SRU", "claims")
	require.NoError(t, os.MkdirAll(dir, 0o755))

	claim := `claimed_by = "benjamin"
claimed_at = 2026-07-07T14:30:00Z
status = "in_progress"
title = "Implement user authentication"
description = "As a user, I want to login with my email and password so that I can access my account."
tracker_status = "In Progress"
`
	path := repo.claimFilePath("T-SRU", "SRU-142")
	require.NoError(t, os.WriteFile(path, []byte(claim), 0o644))

	got, err := repo.GetClaim("T-SRU", "SRU-142")
	require.NoError(t, err)
	assert.Equal(t, "benjamin", got.ClaimedBy)
	assert.Equal(t, "in_progress", got.Status)
	assert.Equal(t, "Implement user authentication", got.Title)
	assert.Equal(t, "As a user, I want to login with my email and password so that I can access my account.", got.Description)
	assert.Equal(t, "In Progress", got.TrackerStatus)
}

func TestClaimWithMetadata_BackwardCompat(t *testing.T) {
	// Old TOML format without title/description/tracker_status should still work.
	repo := setupTestRepo(t)

	dir := filepath.Join(repo.path, "projects", "T-SRU", "claims")
	require.NoError(t, os.MkdirAll(dir, 0o755))

	claim := `claimed_by = "alice"
claimed_at = 2026-07-07T15:00:00Z
status = "review"
`
	path := repo.claimFilePath("T-SRU", "SRU-155")
	require.NoError(t, os.WriteFile(path, []byte(claim), 0o644))

	got, err := repo.GetClaim("T-SRU", "SRU-155")
	require.NoError(t, err)
	assert.Equal(t, "alice", got.ClaimedBy)
	assert.Equal(t, "review", got.Status)
	assert.Equal(t, "", got.Title)
	assert.Equal(t, "", got.Description)
	assert.Equal(t, "", got.TrackerStatus)
}

func TestUpdateClaimStatusFromTracker_SkipsTransitionValidation(t *testing.T) {
	repo, _ := setupGitTestRepo(t)
	ctx := context.Background()

	// Create a claim in "planned" status
	_, err := repo.CreateClaim(ctx, Claim{
		TicketID:  "SRU-300",
		Project:   "T-SRU",
		ClaimedBy: "benjamin",
		Status:    ClaimStatusPlanned,
	})
	require.NoError(t, err)

	// planned → review would be rejected by UpdateClaimStatus (invalid transition)
	err = repo.UpdateClaimStatus(ctx, "T-SRU", "SRU-300", ClaimStatusReview)
	assert.ErrorIs(t, err, ErrInvalidTransition, "normal UpdateClaimStatus should reject planned → review")

	// But UpdateClaimStatusFromTracker should allow it
	err = repo.UpdateClaimStatusFromTracker(ctx, "T-SRU", "SRU-300", ClaimStatusReview)
	require.NoError(t, err, "UpdateClaimStatusFromTracker should allow planned → review")

	got, err := repo.GetClaim("T-SRU", "SRU-300")
	require.NoError(t, err)
	assert.Equal(t, ClaimStatusReview, got.Status)
}

func TestUpdateClaimStatusFromTracker_Noop(t *testing.T) {
	repo, _ := setupGitTestRepo(t)
	ctx := context.Background()

	_, err := repo.CreateClaim(ctx, Claim{
		TicketID:  "SRU-301",
		Project:   "T-SRU",
		ClaimedBy: "benjamin",
		Status:    ClaimStatusInProgress,
	})
	require.NoError(t, err)

	// Same status → no-op
	err = repo.UpdateClaimStatusFromTracker(ctx, "T-SRU", "SRU-301", ClaimStatusInProgress)
	require.NoError(t, err)
}

func TestUpdateClaimMetadata(t *testing.T) {
	repo, _ := setupGitTestRepo(t)
	ctx := context.Background()

	_, err := repo.CreateClaim(ctx, Claim{
		TicketID:  "SRU-310",
		Project:   "T-SRU",
		ClaimedBy: "benjamin",
		Status:    ClaimStatusInProgress,
	})
	require.NoError(t, err)

	// Update metadata
	err = repo.UpdateClaimMetadata(ctx, "T-SRU", "SRU-310", "New Title", "A description", "Code Review")
	require.NoError(t, err)

	got, err := repo.GetClaim("T-SRU", "SRU-310")
	require.NoError(t, err)
	assert.Equal(t, "New Title", got.Title)
	assert.Equal(t, "A description", got.Description)
	assert.Equal(t, "Code Review", got.TrackerStatus)
	// Status should NOT have changed
	assert.Equal(t, ClaimStatusInProgress, got.Status)
}

func TestUpdateClaimMetadata_Noop(t *testing.T) {
	repo, _ := setupGitTestRepo(t)
	ctx := context.Background()

	_, err := repo.CreateClaim(ctx, Claim{
		TicketID:  "SRU-311",
		Project:   "T-SRU",
		ClaimedBy: "benjamin",
		Status:    ClaimStatusInProgress,
		Title:     "Existing Title",
	})
	require.NoError(t, err)

	// Same values → no-op (no error, no unnecessary commit)
	err = repo.UpdateClaimMetadata(ctx, "T-SRU", "SRU-311", "Existing Title", "", "")
	require.NoError(t, err)
}

// Integration tests — require git

func setupGitTestRepo(t *testing.T) (*Repo, string) {
	t.Helper()
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	// Create a bare repo
	bare := t.TempDir()
	gitCmd(t, bare, "init", "--bare")

	// Clone it
	clone := filepath.Join(t.TempDir(), "clone")
	gitCmd(t, t.TempDir(), "clone", bare, clone)

	// Initial commit
	require.NoError(t, os.WriteFile(filepath.Join(clone, "README.md"), []byte("init"), 0o644))
	gitCmd(t, clone, "add", ".")
	gitCmd(t, clone, "commit", "-m", "init")
	gitCmd(t, clone, "push")

	repo := NewRepo(bare, clone)
	return repo, bare
}

func TestCreateClaimIntegration(t *testing.T) {
	repo, _ := setupGitTestRepo(t)
	ctx := context.Background()

	_, err := repo.CreateClaim(ctx, Claim{
		TicketID:  "SRU-142",
		Project:   "T-SRU",
		ClaimedBy: "benjamin",
		Status:    "in_progress",
	})
	require.NoError(t, err)

	// Verify claim exists
	got, err := repo.GetClaim("T-SRU", "SRU-142")
	require.NoError(t, err)
	assert.Equal(t, "benjamin", got.ClaimedBy)
}

func TestCreateClaimConflictIntegration(t *testing.T) {
	repo, _ := setupGitTestRepo(t)
	ctx := context.Background()

	// First claim
	_, err := repo.CreateClaim(ctx, Claim{
		TicketID:  "SRU-142",
		Project:   "T-SRU",
		ClaimedBy: "benjamin",
		Status:    "in_progress",
	})
	require.NoError(t, err)

	// Second claim on same ticket
	existing, err := repo.CreateClaim(ctx, Claim{
		TicketID:  "SRU-142",
		Project:   "T-SRU",
		ClaimedBy: "alice",
		Status:    "in_progress",
	})
	assert.ErrorIs(t, err, ErrClaimExists)
	require.NotNil(t, existing)
	assert.Equal(t, "benjamin", existing.ClaimedBy)
}

func TestReleaseClaimIntegration(t *testing.T) {
	repo, _ := setupGitTestRepo(t)
	ctx := context.Background()

	// Create then release
	_, err := repo.CreateClaim(ctx, Claim{
		TicketID:  "SRU-142",
		Project:   "T-SRU",
		ClaimedBy: "benjamin",
		Status:    "in_progress",
	})
	require.NoError(t, err)

	err = repo.ReleaseClaim(ctx, "T-SRU", "SRU-142")
	require.NoError(t, err)

	// Should be gone
	_, err = repo.GetClaim("T-SRU", "SRU-142")
	assert.ErrorIs(t, err, ErrClaimNotFound)
}

func TestReleaseClaimNotFound(t *testing.T) {
	repo, _ := setupGitTestRepo(t)
	ctx := context.Background()

	err := repo.ReleaseClaim(ctx, "T-SRU", "NONEXISTENT")
	assert.ErrorIs(t, err, ErrClaimNotFound)
}

func TestTransferClaimIntegration(t *testing.T) {
	repo, _ := setupGitTestRepo(t)
	ctx := context.Background()

	_, err := repo.CreateClaim(ctx, Claim{
		TicketID:  "SRU-142",
		Project:   "T-SRU",
		ClaimedBy: "benjamin",
		Worktree:  "feat/SRU-142",
		Status:    "in_progress",
	})
	require.NoError(t, err)

	err = repo.TransferClaim(ctx, "T-SRU", "SRU-142", "alice")
	require.NoError(t, err)

	got, err := repo.GetClaim("T-SRU", "SRU-142")
	require.NoError(t, err)
	assert.Equal(t, "alice", got.ClaimedBy)
	// Worktree should be preserved
	assert.Equal(t, "feat/SRU-142", got.Worktree)
}

// gitCmd runs a git command in the given directory.
func gitCmd(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(),
		"GIT_AUTHOR_NAME=Test",
		"GIT_AUTHOR_EMAIL=test@test.com",
		"GIT_COMMITTER_NAME=Test",
		"GIT_COMMITTER_EMAIL=test@test.com",
	)
	out, err := cmd.CombinedOutput()
	require.NoError(t, err, "git %v failed: %s", args, string(out))
}

func TestIsValidTransition_Allowed(t *testing.T) {
	allowed := []struct {
		from string
		to   string
	}{
		{ClaimStatusPlanned, ClaimStatusInProgress},
		{ClaimStatusInProgress, ClaimStatusReview},
		{ClaimStatusInProgress, ClaimStatusBlocked},
		{ClaimStatusReview, ClaimStatusDone},
		{ClaimStatusReview, ClaimStatusInProgress},
		{ClaimStatusBlocked, ClaimStatusInProgress},
		{ClaimStatusDone, ClaimStatusInProgress}, // reopen
	}
	for _, tt := range allowed {
		assert.True(t, IsValidTransition(tt.from, tt.to),
			"transition %s → %s should be allowed", tt.from, tt.to)
	}
}

func TestIsValidTransition_Rejected(t *testing.T) {
	rejected := []struct {
		from string
		to   string
	}{
		{ClaimStatusPlanned, ClaimStatusReview},
		{ClaimStatusPlanned, ClaimStatusDone},
		{ClaimStatusPlanned, ClaimStatusBlocked},
		{ClaimStatusInProgress, ClaimStatusPlanned},
		{ClaimStatusInProgress, ClaimStatusDone},
		{ClaimStatusReview, ClaimStatusBlocked},
		{ClaimStatusReview, ClaimStatusPlanned},
		{ClaimStatusBlocked, ClaimStatusDone},
		{ClaimStatusBlocked, ClaimStatusReview},
		{ClaimStatusDone, ClaimStatusPlanned},
		{ClaimStatusDone, ClaimStatusReview},
		{ClaimStatusDone, ClaimStatusBlocked},
	}
	for _, tt := range rejected {
		assert.False(t, IsValidTransition(tt.from, tt.to),
			"transition %s → %s should be rejected", tt.from, tt.to)
	}
}

func TestUpdateClaimStatus_InvalidTransition(t *testing.T) {
	repo, _ := setupGitTestRepo(t)
	ctx := context.Background()

	// Create a claim in "planned" status
	_, err := repo.CreateClaim(ctx, Claim{
		TicketID:  "SRU-200",
		Project:   "T-SRU",
		ClaimedBy: "benjamin",
		Status:    ClaimStatusPlanned,
	})
	require.NoError(t, err)

	// Try an invalid transition: planned → done
	err = repo.UpdateClaimStatus(ctx, "T-SRU", "SRU-200", ClaimStatusDone)
	assert.ErrorIs(t, err, ErrInvalidTransition)

	// Verify status unchanged
	got, err := repo.GetClaim("T-SRU", "SRU-200")
	require.NoError(t, err)
	assert.Equal(t, ClaimStatusPlanned, got.Status)
}

func TestUpdateClaimStatus_ValidTransition(t *testing.T) {
	repo, _ := setupGitTestRepo(t)
	ctx := context.Background()

	// Create a claim in "planned" status
	_, err := repo.CreateClaim(ctx, Claim{
		TicketID:  "SRU-201",
		Project:   "T-SRU",
		ClaimedBy: "benjamin",
		Status:    ClaimStatusPlanned,
	})
	require.NoError(t, err)

	// Valid transition: planned → in_progress
	err = repo.UpdateClaimStatus(ctx, "T-SRU", "SRU-201", ClaimStatusInProgress)
	require.NoError(t, err)

	got, err := repo.GetClaim("T-SRU", "SRU-201")
	require.NoError(t, err)
	assert.Equal(t, ClaimStatusInProgress, got.Status)
}

// --- Path traversal security tests ---

func TestGetClaim_PathTraversal(t *testing.T) {
	repo := setupTestRepo(t)

	tests := []struct {
		name    string
		project string
		ticket  string
	}{
		{"traversal project", "../../../etc", "passwd"},
		{"traversal ticket", "T-SRU", "../../../etc/passwd"},
		{"slash in project", "foo/bar", "ticket"},
		{"slash in ticket", "T-SRU", "foo/bar"},
		{"backslash project", "foo\\bar", "ticket"},
		{"null byte project", "foo\x00bar", "ticket"},
		{"null byte ticket", "T-SRU", "foo\x00bar"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := repo.GetClaim(tt.project, tt.ticket)
			assert.Error(t, err)
			assert.ErrorIs(t, err, ErrUnsafeName)
		})
	}
}

func TestCreateClaim_PathTraversal(t *testing.T) {
	repo := setupTestRepo(t)
	ctx := context.Background()

	tests := []struct {
		name    string
		project string
		ticket  string
	}{
		{"traversal project", "../../../tmp", "ticket"},
		{"traversal ticket", "T-SRU", "../../../tmp/evil"},
		{"slash in project", "foo/bar", "ticket"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := repo.CreateClaim(ctx, Claim{
				Project:   tt.project,
				TicketID:  tt.ticket,
				ClaimedBy: "attacker",
			})
			assert.Error(t, err)
			assert.ErrorIs(t, err, ErrUnsafeName)
		})
	}
}

func TestReleaseClaim_PathTraversal(t *testing.T) {
	repo := setupTestRepo(t)
	ctx := context.Background()

	err := repo.ReleaseClaim(ctx, "../../../tmp", "evil")
	assert.Error(t, err)
	assert.ErrorIs(t, err, ErrUnsafeName)

	err = repo.ReleaseClaim(ctx, "T-SRU", "../../../tmp/evil")
	assert.Error(t, err)
	assert.ErrorIs(t, err, ErrUnsafeName)
}

func TestTransferClaim_PathTraversal(t *testing.T) {
	repo := setupTestRepo(t)
	ctx := context.Background()

	err := repo.TransferClaim(ctx, "../../../tmp", "evil", "alice")
	assert.Error(t, err)
	assert.ErrorIs(t, err, ErrUnsafeName)

	err = repo.TransferClaim(ctx, "T-SRU", "../../../tmp/evil", "alice")
	assert.Error(t, err)
	assert.ErrorIs(t, err, ErrUnsafeName)
}

func TestUpdateClaimStatus_PathTraversal(t *testing.T) {
	repo := setupTestRepo(t)
	ctx := context.Background()

	err := repo.UpdateClaimStatus(ctx, "../../../tmp", "evil", ClaimStatusDone)
	assert.Error(t, err)
	assert.ErrorIs(t, err, ErrUnsafeName)

	err = repo.UpdateClaimStatus(ctx, "T-SRU", "../../../tmp/evil", ClaimStatusDone)
	assert.Error(t, err)
	assert.ErrorIs(t, err, ErrUnsafeName)
}

func TestAddClaimLabel_PathTraversal(t *testing.T) {
	repo := setupTestRepo(t)
	ctx := context.Background()

	err := repo.AddClaimLabel(ctx, "../../../tmp", "evil", "label")
	assert.Error(t, err)
	assert.ErrorIs(t, err, ErrUnsafeName)
}

func TestRemoveClaimLabel_PathTraversal(t *testing.T) {
	repo := setupTestRepo(t)
	ctx := context.Background()

	err := repo.RemoveClaimLabel(ctx, "../../../tmp", "evil", "label")
	assert.Error(t, err)
	assert.ErrorIs(t, err, ErrUnsafeName)
}

func TestSetClaimExternalIID_PathTraversal(t *testing.T) {
	repo := setupTestRepo(t)
	ctx := context.Background()

	err := repo.SetClaimExternalIID(ctx, "../../../tmp", "evil", 42)
	assert.Error(t, err)
	assert.ErrorIs(t, err, ErrUnsafeName)
}

// ─── CleanupDoneClaims tests ────────────────────────────────────────────────

func writeClaimFile(t *testing.T, repo *Repo, project, ticketID string, c Claim) {
	t.Helper()
	dir := filepath.Join(repo.path, "projects", project, "claims")
	require.NoError(t, os.MkdirAll(dir, 0o755))
	data := fmt.Sprintf("claimed_by = %q\nclaimed_at = %s\nstatus = %q\n",
		c.ClaimedBy, c.ClaimedAt.Format(time.RFC3339), c.Status)
	if !c.LastActivity.IsZero() {
		data += fmt.Sprintf("last_activity = %s\n", c.LastActivity.Format(time.RFC3339))
	}
	require.NoError(t, os.WriteFile(filepath.Join(dir, ticketID+".toml"), []byte(data), 0o644))
}

func TestCleanupDoneClaims_RemovesOld(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}
	repo, _ := setupGitTestRepo(t)
	ctx := context.Background()

	// Old done claim (10 days ago)
	writeClaimFile(t, repo, "proj", "OLD-1", Claim{
		ClaimedBy: "alice", ClaimedAt: time.Now().Add(-10 * 24 * time.Hour),
		Status: ClaimStatusDone,
	})
	// Recent done claim (1 day ago)
	writeClaimFile(t, repo, "proj", "NEW-1", Claim{
		ClaimedBy: "bob", ClaimedAt: time.Now().Add(-1 * 24 * time.Hour),
		Status: ClaimStatusDone,
	})
	gitCmd(t, repo.path, "add", ".")
	gitCmd(t, repo.path, "commit", "-m", "add claims")
	gitCmd(t, repo.path, "push")

	released, err := repo.CleanupDoneClaims(ctx, 3)
	require.NoError(t, err)
	assert.Len(t, released, 1)
	assert.Equal(t, "OLD-1", released[0].TicketID)

	// Verify OLD-1 file is gone
	_, err = os.Stat(filepath.Join(repo.path, "projects", "proj", "claims", "OLD-1.toml"))
	assert.True(t, os.IsNotExist(err))

	// Verify NEW-1 file still exists
	_, err = os.Stat(filepath.Join(repo.path, "projects", "proj", "claims", "NEW-1.toml"))
	assert.NoError(t, err)
}

func TestCleanupDoneClaims_KeepsActive(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}
	repo, _ := setupGitTestRepo(t)
	ctx := context.Background()

	// Old active claim (should NOT be cleaned)
	writeClaimFile(t, repo, "proj", "ACTIVE-1", Claim{
		ClaimedBy: "alice", ClaimedAt: time.Now().Add(-30 * 24 * time.Hour),
		Status: ClaimStatusInProgress,
	})
	// Old done claim (should be cleaned)
	writeClaimFile(t, repo, "proj", "DONE-OLD", Claim{
		ClaimedBy: "bob", ClaimedAt: time.Now().Add(-10 * 24 * time.Hour),
		Status: ClaimStatusDone,
	})
	gitCmd(t, repo.path, "add", ".")
	gitCmd(t, repo.path, "commit", "-m", "add claims")
	gitCmd(t, repo.path, "push")

	released, err := repo.CleanupDoneClaims(ctx, 3)
	require.NoError(t, err)
	assert.Len(t, released, 1)
	assert.Equal(t, "DONE-OLD", released[0].TicketID)

	// Active claim file still exists
	_, err = os.Stat(filepath.Join(repo.path, "projects", "proj", "claims", "ACTIVE-1.toml"))
	assert.NoError(t, err)
}

func TestCleanupDoneClaims_Empty(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}
	repo, _ := setupGitTestRepo(t)
	ctx := context.Background()

	released, err := repo.CleanupDoneClaims(ctx, 3)
	require.NoError(t, err)
	assert.Empty(t, released)
}

func TestCleanupDoneClaims_AllRecent(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}
	repo, _ := setupGitTestRepo(t)
	ctx := context.Background()

	writeClaimFile(t, repo, "proj", "DONE-1", Claim{
		ClaimedBy: "alice", ClaimedAt: time.Now().Add(-1 * 24 * time.Hour),
		Status: ClaimStatusDone,
	})
	writeClaimFile(t, repo, "proj", "DONE-2", Claim{
		ClaimedBy: "bob", ClaimedAt: time.Now().Add(-2 * 24 * time.Hour),
		Status: ClaimStatusDone,
	})
	gitCmd(t, repo.path, "add", ".")
	gitCmd(t, repo.path, "commit", "-m", "add claims")
	gitCmd(t, repo.path, "push")

	released, err := repo.CleanupDoneClaims(ctx, 3)
	require.NoError(t, err)
	assert.Empty(t, released, "all claims are recent, none should be cleaned")
}
