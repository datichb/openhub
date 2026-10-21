// Package sqlitetest gives the tests a throw-away SQLite store and a way to
// fill it. It is meant for the tests outside internal/storage/sqlite (the
// package itself cannot import it: that would be a cycle).
//
//	st := sqlitetest.New(t)
//	sqlitetest.Seed(t, st).
//		Project("p1", "/p1").
//		Session(&domain.Session{ID: "ses_a", ProjectID: "p1", Status: domain.SessionStatusRunning})
package sqlitetest

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/domain"
	"github.com/datichb/openhub/cli/internal/storage/sqlite"
)

// New opens a store on oh.db in a temporary directory, with the migrations
// applied. It is closed when the test ends.
func New(t testing.TB) *sqlite.Store {
	t.Helper()
	st, err := sqlite.Open(filepath.Join(t.TempDir(), "oh.db"))
	require.NoError(t, err, "opening the test store")
	t.Cleanup(func() { _ = st.Close() })
	return st
}

// Seeder fills a store through its public stores; each step fails the test
// on error and returns the seeder so the calls chain.
type Seeder struct {
	t  testing.TB
	st *sqlite.Store
}

// Seed starts filling st.
func Seed(t testing.TB, st *sqlite.Store) *Seeder {
	t.Helper()
	return &Seeder{t: t, st: st}
}

// Project adds an active project; its name is its id.
func (s *Seeder) Project(id, path string) *Seeder {
	s.t.Helper()
	err := sqlite.NewProjectStore(s.st).Create(context.Background(),
		&domain.Project{ID: id, Name: id, Path: path, Status: domain.ProjectStatusActive})
	require.NoError(s.t, err, "seeding project %s", id)
	return s
}

// Session adds a session (its project must have been seeded).
func (s *Seeder) Session(sess *domain.Session) *Seeder {
	s.t.Helper()
	require.NoError(s.t, sqlite.NewSessionStore(s.st).Create(context.Background(), sess),
		"seeding session %s", sess.ID)
	return s
}
