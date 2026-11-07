package teamstate

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/gitutil/gittest"
)

func TestNewProjectIdentityCanonicalizesTheRemote(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 11, 7, 12, 0, 0, 0, time.UTC)
	for _, remote := range []string{
		"https://oauth2:secret-token@git.example.com/acme/app.git",
		"git@git.example.com:acme/app.git",
		"ssh://git@git.example.com:2222/acme/app",
		"HTTPS://Git.Example.com/acme/app/",
	} {
		id, err := NewProjectIdentity("app-1a2b3c4d", "app", remote, "alice", now)
		require.NoError(t, err, remote)
		assert.Equal(t, "git.example.com/acme/app", id.Remote, remote)
		assert.Equal(t, InstantOf(now), id.RecordedAt)
	}

	_, err := NewProjectIdentity("app", "app", "/home/alice/app", "alice", now)
	require.ErrorIs(t, err, ErrProjectIdentityInvalid)
	_, err = NewProjectIdentity("../x", "app", "git@git.example.com:acme/app.git", "alice", now)
	require.ErrorIs(t, err, ErrProjectIdentityInvalid)
	_, err = NewProjectIdentity("app", "app", "https://user:hunter2@", "alice", now)
	require.Error(t, err)
	assert.NotContains(t, err.Error(), "hunter2", "a secret does not reach an error message")
}

func TestProjectIdentityMatchesAnySpelling(t *testing.T) {
	t.Parallel()
	id := ProjectIdentity{ID: "app", Remote: "git.example.com/acme/app"}
	assert.True(t, id.Matches("git@git.example.com:acme/app.git"))
	assert.True(t, id.Matches("https://git.example.com/acme/app"))
	assert.False(t, id.Matches("https://git.example.com/acme/other"))
	assert.False(t, ProjectIdentity{}.Matches(""))
}

func TestProjectIdentityReadWrite(t *testing.T) {
	t.Parallel()
	repo, bare := setupGitTestRepo(t)
	ctx := t.Context()
	_, err := repo.ReadProjectIdentity(ctx, "app-1")
	require.ErrorIs(t, err, ErrProjectIdentityNotFound)

	id, err := NewProjectIdentity("app-1", "app", "git@git.example.com:acme/app.git", "alice", time.Now())
	require.NoError(t, err)
	before := commitCount(t, bare)
	changed, err := repo.WriteProjectIdentity(ctx, id)
	require.NoError(t, err)
	assert.True(t, changed)
	assert.Equal(t, before+1, commitCount(t, bare), "one commit, pushed")
	assert.Contains(t, gittest.Run(t, bare, "log", "-1", "--format=%s", "main"), "project: identity of app-1")

	got, err := repo.ReadProjectIdentity(ctx, "app-1")
	require.NoError(t, err)
	assert.Equal(t, id, *got)
	raw, err := os.ReadFile(filepath.Join(repo.Path(), "projects", "app-1", ProjectIdentityFile))
	require.NoError(t, err)
	assert.Contains(t, string(raw), `remote = 'git.example.com/acme/app'`)

	// the same identity again: nothing to commit
	changed, err = repo.WriteProjectIdentity(ctx, id)
	require.NoError(t, err)
	assert.False(t, changed)
	assert.Equal(t, before+1, commitCount(t, bare))

	// Ensure keeps what is there; Write replaces it
	other, err := NewProjectIdentity("app-1", "app", "git@git.example.com:acme/moved.git", "bob", time.Now())
	require.NoError(t, err)
	created, err := repo.EnsureProjectIdentity(ctx, other)
	require.NoError(t, err)
	assert.False(t, created)
	got, _ = repo.ReadProjectIdentity(ctx, "app-1")
	assert.Equal(t, "git.example.com/acme/app", got.Remote)
	changed, err = repo.WriteProjectIdentity(ctx, other)
	require.NoError(t, err)
	assert.True(t, changed)
	got, _ = repo.ReadProjectIdentity(ctx, "app-1")
	assert.Equal(t, "git.example.com/acme/moved", got.Remote)
}

func TestProjectIdentityListAndFind(t *testing.T) {
	t.Parallel()
	repo, _ := setupGitTestRepo(t)
	ctx := t.Context()
	for _, p := range []struct{ id, remote string }{
		{"app-aaaa1111", "git@git.example.com:acme/app.git"},
		{"app-bbbb2222", "https://git.example.com/acme/app"},
		{"lib-cccc3333", "git@git.example.com:acme/lib.git"},
	} {
		id, err := NewProjectIdentity(p.id, p.id, p.remote, "alice", time.Now())
		require.NoError(t, err)
		_, err = repo.WriteProjectIdentity(ctx, id)
		require.NoError(t, err)
	}
	// a broken file is reported, not fatal
	bad := filepath.Join(repo.Path(), "projects", "bad-dddd4444")
	require.NoError(t, os.MkdirAll(bad, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(bad, ProjectIdentityFile), []byte("remote = [\n"), 0o644))
	// a project without identity (claims only) is just not listed
	require.NoError(t, os.MkdirAll(filepath.Join(repo.Path(), "projects", "old-eeee5555", "claims"), 0o755))

	all, unreadable, err := repo.ListProjectIdentities(ctx)
	require.NoError(t, err)
	require.Len(t, all, 3)
	assert.Equal(t, "app-aaaa1111", all[0].ID)
	require.Contains(t, unreadable, "bad-dddd4444")

	found, err := repo.FindProjectByRemote(ctx, "ssh://git@git.example.com/acme/app.git")
	require.NoError(t, err)
	require.Len(t, found, 2, "the IDs the repository has been known under")
	none, err := repo.FindProjectByRemote(ctx, "git@other.example.com:acme/app.git")
	require.NoError(t, err)
	assert.Empty(t, none)
}

// The identity goes in the commit of the first claim of a project, and an
// identity that is already there is left alone.
func TestWriteClaimRecordsTheProjectIdentityOnce(t *testing.T) {
	t.Parallel()
	repo, bare := setupGitTestRepo(t)
	ctx := t.Context()
	id, err := NewProjectIdentity("T-SRU", "sru", "git@git.example.com:acme/sru.git", "benjamin", time.Now())
	require.NoError(t, err)
	before := commitCount(t, bare)

	res, err := repo.WriteClaim(ctx, ClaimWrite{Project: "T-SRU", TicketID: "SRU-1", Member: "benjamin", Create: &Claim{}, Identity: &id})
	require.NoError(t, err)
	assert.True(t, res.Created)
	assert.True(t, res.IdentityRecorded)
	assert.Equal(t, before+1, commitCount(t, bare), "claim, event and identity: one commit")
	got, err := repo.ReadProjectIdentity(ctx, "T-SRU")
	require.NoError(t, err)
	assert.Equal(t, "git.example.com/acme/sru", got.Remote)

	res, err = repo.WriteClaim(ctx, ClaimWrite{Project: "T-SRU", TicketID: "SRU-2", Member: "benjamin", Create: &Claim{}, Identity: &id})
	require.NoError(t, err)
	assert.True(t, res.Created)
	assert.False(t, res.IdentityRecorded, "already recorded")

	// the claim of another member: nothing is recorded for it
	mustCreate(t, repo, Claim{TicketID: "SRU-3", ClaimedBy: "alice"})
	other, _ := NewProjectIdentity("T-SRU", "sru", "git@git.example.com:acme/elsewhere.git", "benjamin", time.Now())
	n := commitCount(t, bare)
	res, err = repo.WriteClaim(ctx, ClaimWrite{Project: "T-SRU", TicketID: "SRU-3", Member: "benjamin", Create: &Claim{}, Identity: &other})
	require.NoError(t, err)
	assert.NotNil(t, res.Held)
	assert.Equal(t, n, commitCount(t, bare))
	got, _ = repo.ReadProjectIdentity(ctx, "T-SRU")
	assert.Equal(t, "git.example.com/acme/sru", got.Remote, "an existing identity is never replaced by a claim")
}
