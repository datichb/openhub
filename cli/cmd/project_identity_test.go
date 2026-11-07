package cmd

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/domain"
	"github.com/datichb/openhub/cli/internal/i18n/i18ntest"
	"github.com/datichb/openhub/cli/internal/teamstate"
)

// writeOrigin gives the folder a .git/config with the origin remote.
func writeOrigin(t *testing.T, dir, url string) {
	t.Helper()
	require.NoError(t, os.MkdirAll(filepath.Join(dir, ".git"), 0o755))
	cfg := "[core]\n\tbare = false\n[remote \"origin\"]\n\turl = " + url + "\n"
	require.NoError(t, os.WriteFile(filepath.Join(dir, ".git", "config"), []byte(cfg), 0o644))
}

func TestLocalProjectIdentity(t *testing.T) {
	dir := t.TempDir()
	p := &domain.Project{ID: "app-1a2b3c4d", Name: "app", Path: dir}
	_, ok := localProjectIdentity(p, "alice", time.Now())
	assert.False(t, ok, "no origin")

	writeOrigin(t, dir, "https://oauth2:tok@git.example.com/acme/app.git")
	id, ok := localProjectIdentity(p, "alice", time.Now())
	require.True(t, ok)
	assert.Equal(t, "git.example.com/acme/app", id.Remote, "the credentials of the URL are not recorded")
	assert.Equal(t, "alice", id.RecordedBy)

	writeOrigin(t, dir, "/srv/git/app.git")
	_, ok = localProjectIdentity(p, "alice", time.Now())
	assert.False(t, ok, "a local path is no identity")
	_, ok = localProjectIdentity(nil, "alice", time.Now())
	assert.False(t, ok)
}

// The launch records the remote of the project in the commit of its first
// claim.
func TestStartTicketClaimsRecordTheProjectIdentity(t *testing.T) {
	if testing.Short() {
		t.Skip("integration test (git)")
	}
	m := newMultiTeam(t, true)
	writeOrigin(t, m.projB.Path, "git@git.example.com:acme/proj-b.git")
	hook := startTicketClaims(m.a, &m.projB)
	require.NotNil(t, hook)
	hook(t.Context(), "proj-b", []string{"T-7"})

	id, err := m.repoB.ReadProjectIdentity(t.Context(), "proj-b")
	require.NoError(t, err)
	assert.Equal(t, "git.example.com/acme/proj-b", id.Remote)
	assert.Equal(t, "bob", id.RecordedBy)

	// a project without origin leaves the claim without identity
	_, err = m.repoA.ReadProjectIdentity(t.Context(), "proj-a")
	assert.ErrorIs(t, err, teamstate.ErrProjectIdentityNotFound)
}

func TestProjectIdentityDoctor(t *testing.T) {
	if testing.Short() {
		t.Skip("integration test (git)")
	}
	i18ntest.Set(t, "en")
	m := newMultiTeam(t, true)
	origin := "git@git.example.com:acme/pb.git"
	originOf := func(string) (string, bool) { return origin, true }
	ctx := t.Context()

	// no identity yet: nothing to say
	assert.Empty(t, projectIdentityResults(ctx, m.a, originOf))

	id, err := teamstate.NewProjectIdentity("pb", "proj-b", origin, "bob", time.Now())
	require.NoError(t, err)
	_, err = m.repoB.WriteProjectIdentity(ctx, id)
	require.NoError(t, err)
	assert.Empty(t, projectIdentityResults(ctx, m.a, originOf), "same repository, whatever the spelling")
	origin = "https://git.example.com/acme/pb"
	assert.Empty(t, projectIdentityResults(ctx, m.a, originOf))

	// the folder now points to another repository
	origin = "git@git.example.com:acme/elsewhere.git"
	rs := projectIdentityResults(ctx, m.a, originOf)
	require.Len(t, rs, 1)
	assert.True(t, rs[0].Warn)
	assert.Equal(t, doctorIDProjectIdentity, rs[0].ID)
	assert.Contains(t, rs[0].Detail, "git.example.com/acme/pb")
	assert.Contains(t, rs[0].Detail, "git.example.com/acme/elsewhere")

	// no origin to compare with: nothing
	assert.Empty(t, projectIdentityResults(ctx, m.a, func(string) (string, bool) { return "", false }))

	// an identity file that cannot be read
	require.NoError(t, os.WriteFile(filepath.Join(m.repoB.Path(), "projects", "pb", teamstate.ProjectIdentityFile), []byte("remote = [\n"), 0o644))
	rs = projectIdentityResults(ctx, m.a, originOf)
	require.Len(t, rs, 1)
	assert.Contains(t, rs[0].Detail, "cannot be read")
}
