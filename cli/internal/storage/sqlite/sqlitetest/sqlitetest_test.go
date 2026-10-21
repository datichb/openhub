package sqlitetest_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/domain"
	"github.com/datichb/openhub/cli/internal/storage/sqlite"
	"github.com/datichb/openhub/cli/internal/storage/sqlite/sqlitetest"
)

func TestNewAndSeed(t *testing.T) {
	ctx := context.Background()
	st := sqlitetest.New(t)
	other := sqlitetest.New(t)

	version, err := st.SchemaVersion()
	require.NoError(t, err, "schema version")
	assert.Positive(t, version, "migrations are applied")

	seed := sqlitetest.Seed(t, st)
	chained := seed.
		Project("p1", "/p1").
		Project("p2", "/p2").
		Session(&domain.Session{ID: "ses_a", ProjectID: "p1", Status: domain.SessionStatusRunning, GroupKey: "g1"})
	assert.Same(t, seed, chained, "the calls chain on the same seeder")

	projects := sqlite.NewProjectStore(st)
	p1, err := projects.Get(ctx, "p1")
	require.NoError(t, err, "seeded project is found")
	assert.Equal(t, "p1", p1.Name, "the name is the id")
	assert.Equal(t, "/p1", p1.Path, "the path is kept")
	assert.Equal(t, domain.ProjectStatusActive, p1.Status, "the project is active")
	active, err := projects.List(ctx, domain.ProjectStatusActive)
	require.NoError(t, err, "listing active projects")
	assert.Len(t, active, 2, "both projects are listed")

	sess, err := sqlite.NewSessionStore(st).Get(ctx, "ses_a")
	require.NoError(t, err, "seeded session is found")
	assert.Equal(t, "p1", sess.ProjectID, "the session belongs to its project")
	assert.Equal(t, "g1", sess.GroupKey, "the session fields are kept")

	_, err = sqlite.NewProjectStore(other).Get(ctx, "p1")
	assert.ErrorIs(t, err, domain.ErrNotFound, "each New is its own database")
}
