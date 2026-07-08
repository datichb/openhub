package teamstate

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

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
