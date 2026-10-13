package report

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/domain"
)

// memStore is an in-memory domain.ReportStore.
type memStore struct {
	mu      sync.Mutex
	reports map[string][]domain.Report
}

func (m *memStore) AddReport(_ context.Context, r *domain.Report, maxCount int, maxBytes int64) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.reports == nil {
		m.reports = map[string][]domain.Report{}
	}
	list := m.reports[r.SessionID]
	var total int64
	last := 0
	for _, x := range list {
		total += int64(x.Bytes)
		last = max(last, x.Seq)
	}
	if (maxCount > 0 && len(list)+1 > maxCount) || (maxBytes > 0 && total+int64(r.Bytes) > maxBytes) {
		return domain.ErrReportLimit
	}
	r.Seq = last + 1
	m.reports[r.SessionID] = append(list, *r)
	return nil
}

func (m *memStore) ImportReport(_ context.Context, r *domain.Report) (bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.reports == nil {
		m.reports = map[string][]domain.Report{}
	}
	for _, x := range m.reports[r.SessionID] {
		if x.Seq == r.Seq {
			return false, nil
		}
	}
	m.reports[r.SessionID] = append(m.reports[r.SessionID], *r)
	return true, nil
}

func (m *memStore) DeleteReport(_ context.Context, sid string, seq int) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	list := m.reports[sid]
	for i, x := range list {
		if x.Seq == seq {
			m.reports[sid] = append(list[:i], list[i+1:]...)
			break
		}
	}
	return nil
}

func (m *memStore) ListReports(_ context.Context, sid string) ([]domain.Report, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return append([]domain.Report(nil), m.reports[sid]...), nil
}

func (m *memStore) GetReport(_ context.Context, sid string, seq int) (*domain.Report, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, x := range m.reports[sid] {
		if x.Seq == seq {
			return &x, nil
		}
	}
	return nil, domain.ErrNotFound
}

func newService(t *testing.T) *Service {
	t.Helper()
	at := time.Date(2026, 10, 13, 10, 0, 0, 0, time.UTC)
	return &Service{Store: &memStore{}, SessionsDir: t.TempDir(), Now: func() time.Time { at = at.Add(time.Second); return at }}
}

const body = "# Retour du developer\n\n## Changements par fichier\n- calc.go : Sub corrigée\n\n```\n# pas un titre\n```\n\n## Tests\ngo test ./... : ok\n"

func TestAddGetAndSections(t *testing.T) {
	s := newService(t)
	ctx := context.Background()
	r, err := s.Add(ctx, "ses_a", "developer", "ses_child", "", "\x1b[31m"+body+"\x07")
	require.NoError(t, err)
	assert.Equal(t, "rp-1", r.ID())
	assert.Equal(t, "developer", r.Agent)
	assert.Equal(t, "Retour du developer", r.Title, "title taken from the first heading")
	assert.Equal(t, []string{"Retour du developer", "Changements par fichier", "Tests"}, r.Sections)

	path := filepath.Join(s.SessionsDir, "ses_a", "reports", "rp-1.md")
	fi, err := os.Stat(path)
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0o600), fi.Mode().Perm())
	di, err := os.Stat(filepath.Dir(path))
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0o700), di.Mode().Perm())

	got, text, err := s.Get(ctx, "ses_a", "RP-1")
	require.NoError(t, err)
	assert.Equal(t, r.SHA256, got.SHA256)
	assert.NotContains(t, text, "\x1b")
	assert.NotContains(t, text, "\x07")
	assert.True(t, strings.HasPrefix(text, "# Retour"), text)

	sec, ok := Section(text, "tests")
	require.True(t, ok)
	assert.Equal(t, "## Tests\ngo test ./... : ok", sec)
	sec, ok = Section(text, "## Changements")
	require.True(t, ok)
	assert.Contains(t, sec, "calc.go")
	assert.NotContains(t, sec, "go test")
	_, ok = Section(text, "absente")
	assert.False(t, ok)
	all, ok := Section(text, "")
	assert.True(t, ok)
	assert.Equal(t, text, all)

	_, _, err = s.Get(ctx, "ses_a", "rp-9")
	assert.ErrorIs(t, err, domain.ErrNotFound)
	_, _, err = s.Get(ctx, "ses_a", "zz")
	assert.ErrorIs(t, err, ErrBadID)

	// A body changed on disk is refused (no agent forges a stored report).
	require.NoError(t, os.WriteFile(path, []byte("forgé"), 0o600))
	_, _, err = s.Get(ctx, "ses_a", "rp-1")
	assert.ErrorIs(t, err, ErrAltered)
}

func TestClean(t *testing.T) {
	assert.Equal(t, "a\nb\tc\nd", Clean("a\r\nb\tc\rd\x00\x1b"))
	assert.Equal(t, "rouge lien", Clean("\x1b[1;31mrouge\x1b[0m \x1b]8;;http://x\x07lien\x1b]8;;\x07"))
	assert.Equal(t, "ok", Clean("o\u202ek\u0085"), "bidi and C1 controls removed")
	assert.Equal(t, "x\uFFFDy", Clean("x\xffy"))
	assert.Equal(t, "Un titre sur une ligne", cleanTitle("  Un titre\nsur\x1b une   ligne ", ""))
	assert.Equal(t, "Rapport", cleanTitle("", "pas de titre"))
	long := cleanTitle(strings.Repeat("é", 300), "")
	assert.Equal(t, MaxTitle, len([]rune(long)))
}

func TestLimits(t *testing.T) {
	s := newService(t)
	ctx := context.Background()
	_, err := s.Add(ctx, "ses_a", "developer", "", "t", " \n\x1b ")
	assert.ErrorIs(t, err, ErrEmpty)
	_, err = s.Add(ctx, "ses_a", "developer", "", "t", strings.Repeat("a", MaxBytes+1))
	assert.ErrorIs(t, err, ErrTooLarge)
	_, err = s.Add(ctx, "ses_a", "developer", "", "t", strings.Repeat("a", MaxBytes))
	require.NoError(t, err, "64 KiB exactly")

	// 4 MiB per session.
	for i := 1; i < MaxSessionBytes/MaxBytes; i++ {
		_, err = s.Add(ctx, "ses_a", "developer", "", "t", strings.Repeat("a", MaxBytes))
		require.NoError(t, err)
	}
	_, err = s.Add(ctx, "ses_a", "developer", "", "t", "un octet de trop")
	assert.ErrorIs(t, err, ErrFull)

	// 100 reports per session.
	for i := 0; i < MaxPerSession; i++ {
		_, err = s.Add(ctx, "ses_b", "reviewer", "", "t", "court")
		require.NoError(t, err)
	}
	_, err = s.Add(ctx, "ses_b", "reviewer", "", "t", "court")
	assert.ErrorIs(t, err, ErrFull)
	list, err := s.List(ctx, "ses_b")
	require.NoError(t, err)
	assert.Len(t, list, MaxPerSession)
}

func TestWriteFailureRollsBack(t *testing.T) {
	s := newService(t)
	ctx := context.Background()
	// The session folder is a file: the body cannot be written.
	require.NoError(t, os.WriteFile(filepath.Join(s.SessionsDir, "ses_a"), nil, 0o600))
	_, err := s.Add(ctx, "ses_a", "developer", "", "t", "corps")
	require.Error(t, err)
	list, _ := s.List(ctx, "ses_a")
	assert.Empty(t, list, "no row without its body")
}

func TestSinceExportImport(t *testing.T) {
	s := newService(t)
	ctx := context.Background()
	r1, err := s.Add(ctx, "ses_a", "developer", "", "Dev", body)
	require.NoError(t, err)
	r2, err := s.Add(ctx, "ses_a", "reviewer", "", "Review", "## Verdict\ncorriger")
	require.NoError(t, err)
	since, err := s.Since(ctx, "ses_a", r1.CreatedAt)
	require.NoError(t, err)
	require.Len(t, since, 1)
	assert.Equal(t, r2.ID(), since[0].ID())

	ex, err := s.Export(ctx, "ses_a")
	require.NoError(t, err)
	require.Len(t, ex, 2)
	assert.Equal(t, "## Verdict\ncorriger", ex[1].Body)

	// Import on another machine: cleaned and bounded again; numbers kept.
	other := newService(t)
	ex[1].Body = "## Verdict\x1b[2J\ncorriger"
	ex = append(ex, Exported{Report: domain.Report{Seq: 3, Agent: "x"}, Body: strings.Repeat("a", MaxBytes+1)})
	n, err := other.Import(ctx, "ses_a", ex)
	require.NoError(t, err)
	assert.Equal(t, 2, n, "the oversized report is left out")
	_, text, err := other.Get(ctx, "ses_a", "rp-2")
	require.NoError(t, err)
	assert.Equal(t, "## Verdict\ncorriger", text)
	got, _, err := other.Get(ctx, "ses_a", "rp-1")
	require.NoError(t, err)
	assert.Equal(t, "developer", got.Agent)
	assert.True(t, got.CreatedAt.Equal(r1.CreatedAt))

	// Fetched again: nothing added, nothing replaced.
	ex[0].Body = "remplacé"
	n, err = other.Import(ctx, "ses_a", ex)
	require.NoError(t, err)
	assert.Zero(t, n)
	_, text, err = other.Get(ctx, "ses_a", "rp-1")
	require.NoError(t, err)
	assert.Contains(t, text, "Changements par fichier")
}

func TestHeading(t *testing.T) {
	for line, want := range map[string]string{"# A": "A", "### B ###": "B", "#C": "", "####### D": "", "#": "", "  # E": ""} {
		h, ok := heading(line)
		assert.Equal(t, want != "", ok, line)
		assert.Equal(t, want, h.text, line)
	}
}
