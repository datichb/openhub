package sqlite

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/domain"
	"github.com/datichb/openhub/cli/internal/testenv"
)

// The tests are isolated from the real hub and home (testenv).
func TestMain(m *testing.M) {
	finish := testenv.Isolate()
	os.Exit(finish(m.Run()))
}

// openTestStore opens a store on a fresh oh.db, closed when the test ends.
func openTestStore(t *testing.T) *Store {
	t.Helper()
	s, err := Open(filepath.Join(t.TempDir(), "oh.db"))
	require.NoError(t, err)
	t.Cleanup(func() { s.Close() })
	return s
}

// seedProject adds an active project named and placed after its id.
func seedProject(t *testing.T, s *Store, id string) {
	t.Helper()
	require.NoError(t, NewProjectStore(s).Create(context.Background(),
		&domain.Project{ID: id, Name: id, Path: "/" + id, Status: domain.ProjectStatusActive}), "seeding project %s", id)
}
