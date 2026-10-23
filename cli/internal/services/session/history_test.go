package session

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/domain"
	"github.com/datichb/openhub/cli/internal/storage/sqlite"
	"github.com/datichb/openhub/cli/internal/storage/sqlite/sqlitetest"
)

func seedHistory(t *testing.T) *sqlite.SessionStore {
	t.Helper()
	st := sqlitetest.New(t)
	started := time.Date(2026, 10, 1, 9, 30, 0, 0, time.UTC)
	ended := started.Add(12 * time.Minute)
	alice := "alice"
	sqlitetest.Seed(t, st).Project("p1", "/p1").
		Session(&domain.Session{ID: "ses_done", ProjectID: "p1", StartedAt: started, EndedAt: &ended, Status: domain.SessionStatusCompleted,
			Provider: "bedrock", Model: "sonnet", TokensIn: 120, TokensOut: 80, Cost: 0.42, MemberID: &alice}).
		Session(&domain.Session{ID: "ses_run", ProjectID: "p1", StartedAt: started.Add(time.Hour), Status: domain.SessionStatusRunning})
	return sqlite.NewSessionStore(st)
}

func memberOf(t *testing.T, s domain.SessionStore, id string) string {
	t.Helper()
	got, err := s.Get(context.Background(), id)
	require.NoError(t, err, id)
	if got.MemberID == nil {
		return ""
	}
	return *got.MemberID
}

// The round trip: what an export writes, an import into another database
// brings back (same sessions, same fields).
func TestHistoryExportThenImportRoundTrip(t *testing.T) {
	ctx := context.Background()
	src := seedHistory(t)
	path := filepath.Join(t.TempDir(), "history.json")

	n, err := ExportHistory(ctx, src, path, ExportOptions{MemberID: "bob", Now: time.Date(2026, 10, 23, 8, 0, 0, 0, time.UTC)})
	require.NoError(t, err)
	assert.Equal(t, 2, n)

	file, err := ReadHistory(path)
	require.NoError(t, err)
	assert.Equal(t, "bob", file.MemberID, "the member is written in the file")
	assert.Equal(t, time.Date(2026, 10, 23, 8, 0, 0, 0, time.UTC), file.ExportedAt.UTC())
	assert.Len(t, file.Sessions, 2)

	st2 := sqlitetest.New(t)
	sqlitetest.Seed(t, st2).Project("p1", "/p1")
	dst := sqlite.NewSessionStore(st2)

	res, err := ImportHistoryFile(ctx, dst, path, ImportOptions{})
	require.NoError(t, err)
	assert.Equal(t, ImportResult{Imported: 2, Skipped: 0}, res)

	want, err := src.Get(ctx, "ses_done")
	require.NoError(t, err)
	got, err := dst.Get(ctx, "ses_done")
	require.NoError(t, err)
	assert.Equal(t, want.StartedAt.UTC(), got.StartedAt.UTC())
	require.NotNil(t, got.EndedAt)
	assert.Equal(t, want.EndedAt.UTC(), got.EndedAt.UTC())
	assert.Equal(t, want.Status, got.Status)
	assert.Equal(t, want.Model, got.Model)
	assert.Equal(t, want.TokensIn, got.TokensIn)
	assert.InDelta(t, want.Cost, got.Cost, 1e-9)
	assert.Equal(t, "bob", memberOf(t, dst, "ses_done"), "the member of the file is given to every session")
	assert.Equal(t, "bob", memberOf(t, dst, "ses_run"))

	// Second import: everything exists.
	res, err = ImportHistoryFile(ctx, dst, path, ImportOptions{})
	require.NoError(t, err)
	assert.Equal(t, ImportResult{Imported: 0, Skipped: 2}, res, "replayable: nothing is imported twice")
}

func TestHistoryImportMemberPrecedence(t *testing.T) {
	ctx := context.Background()
	src := seedHistory(t)
	dir := t.TempDir()
	withMember := filepath.Join(dir, "with.json")
	without := filepath.Join(dir, "without.json")
	_, err := ExportHistory(ctx, src, withMember, ExportOptions{MemberID: "from-file"})
	require.NoError(t, err)
	_, err = ExportHistory(ctx, src, without, ExportOptions{})
	require.NoError(t, err)
	raw, err := os.ReadFile(without)
	require.NoError(t, err)
	assert.NotContains(t, string(raw), `"member_id": "`, "no member in the file when none is given")

	fresh := func() domain.SessionStore {
		st := sqlitetest.New(t)
		sqlitetest.Seed(t, st).Project("p1", "/p1")
		return sqlite.NewSessionStore(st)
	}

	cases := []struct {
		name string
		file string
		opts ImportOptions
		want string
	}{
		{"override wins over the file", withMember, ImportOptions{MemberID: "flag", DefaultMemberID: "cfg"}, "flag"},
		{"the file wins over the default", withMember, ImportOptions{DefaultMemberID: "cfg"}, "from-file"},
		{"the default fills in", without, ImportOptions{DefaultMemberID: "cfg"}, "cfg"},
		{"nothing: sessions keep their member", without, ImportOptions{}, ""},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			dst := fresh()
			_, err := ImportHistoryFile(ctx, dst, c.file, c.opts)
			require.NoError(t, err)
			assert.Equal(t, c.want, memberOf(t, dst, "ses_run"))
			if c.want == "" {
				assert.Equal(t, "alice", memberOf(t, dst, "ses_done"), "the member recorded in the session is kept")
			}
		})
	}
}

// The file is written whole or not at all: a failed export leaves the
// previous file and no temporary file.
func TestHistoryExportIsAtomic(t *testing.T) {
	ctx := context.Background()
	src := seedHistory(t)
	dir := t.TempDir()
	path := filepath.Join(dir, "history.json")
	require.NoError(t, os.WriteFile(path, []byte("previous"), 0o600))

	_, err := ExportHistory(ctx, src, path, ExportOptions{})
	require.NoError(t, err)
	entries, err := os.ReadDir(dir)
	require.NoError(t, err)
	assert.Len(t, entries, 1, "no temporary file is left next to the export")
	info, err := os.Stat(path)
	require.NoError(t, err)
	if runtime.GOOS != "windows" {
		assert.Equal(t, HistoryFilePerm, info.Mode().Perm(), "the mode is the one of the former export")
	}
	_, err = ReadHistory(path)
	require.NoError(t, err, "the previous content was replaced as a whole")

	missing := filepath.Join(dir, "no-such-dir", "history.json")
	_, err = ExportHistory(ctx, src, missing, ExportOptions{})
	var he *HistoryError
	require.ErrorAs(t, err, &he)
	assert.Equal(t, HistoryWrite, he.Stage)
	entries, err = os.ReadDir(dir)
	require.NoError(t, err)
	assert.Len(t, entries, 1, "a failed export leaves nothing behind")
}

func TestHistoryErrorsTellTheStage(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()

	_, err := ReadHistory(filepath.Join(dir, "absent.json"))
	var he *HistoryError
	require.ErrorAs(t, err, &he)
	assert.Equal(t, HistoryRead, he.Stage)
	assert.ErrorIs(t, err, os.ErrNotExist)

	bad := filepath.Join(dir, "bad.json")
	require.NoError(t, os.WriteFile(bad, []byte("not json"), 0o600))
	_, err = ImportHistoryFile(ctx, seedHistory(t), bad, ImportOptions{})
	require.ErrorAs(t, err, &he)
	assert.Equal(t, HistoryParse, he.Stage)

	st := sqlitetest.New(t)
	ss := sqlite.NewSessionStore(st)
	require.NoError(t, st.Close())
	_, err = ExportHistory(ctx, ss, filepath.Join(dir, "x.json"), ExportOptions{})
	require.ErrorAs(t, err, &he)
	assert.Equal(t, HistoryList, he.Stage)
	assert.NoFileExists(t, filepath.Join(dir, "x.json"))
}
