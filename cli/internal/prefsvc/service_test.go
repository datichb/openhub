package prefsvc

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/domain"
	"github.com/datichb/openhub/cli/internal/i18n"
	"github.com/datichb/openhub/cli/internal/storage/sqlite"
)

type fakeUsage struct {
	uses    []domain.WorkflowUse
	project string
}

func (f *fakeUsage) RecentWorkflows(_ context.Context, projectID string, _ int) ([]domain.WorkflowUse, error) {
	f.project = projectID
	return f.uses, nil
}

func newService(t *testing.T, usage domain.WorkflowUsageReader) *Service {
	t.Helper()
	st, err := sqlite.Open(filepath.Join(t.TempDir(), "oh.db"))
	require.NoError(t, err)
	t.Cleanup(func() { st.Close() })
	return New(sqlite.NewPreferenceStore(st), usage)
}

func TestPinUnpinToggle(t *testing.T) {
	s := newService(t, nil)
	ctx := context.Background()
	g := domain.PreferenceScopeGlobal

	require.NoError(t, s.Pin(ctx, g, "ticket"))
	require.NoError(t, s.Pin(ctx, g, "ticket"), "idempotent")
	require.NoError(t, s.Pin(ctx, g, "review"))
	ids, err := s.Pins(ctx, g)
	require.NoError(t, err)
	assert.Equal(t, []string{"ticket", "review"}, ids)

	pinned, err := s.TogglePin(ctx, g, "ticket")
	require.NoError(t, err)
	assert.False(t, pinned)
	pinned, err = s.TogglePin(ctx, g, "audit")
	require.NoError(t, err)
	assert.True(t, pinned)
	ids, _ = s.Pins(ctx, g)
	assert.Equal(t, []string{"review", "audit"}, ids)

	require.NoError(t, s.Unpin(ctx, g, "ghost"))
	require.NoError(t, s.Unpin(ctx, g, "review"))
	require.NoError(t, s.Unpin(ctx, g, "audit"))
	ids, _ = s.Pins(ctx, g)
	assert.Empty(t, ids)

	assert.Error(t, s.Pin(ctx, g, ""))
}

func TestPinLimitPerScope(t *testing.T) {
	s := newService(t, nil)
	ctx := context.Background()
	g := domain.PreferenceScopeGlobal
	for _, id := range []string{"a", "b", "c", "d", "e"} {
		require.NoError(t, s.Pin(ctx, g, id))
	}
	assert.ErrorIs(t, s.Pin(ctx, g, "f"), ErrPinLimit)
	_, err := s.TogglePin(ctx, g, "f")
	assert.ErrorIs(t, err, ErrPinLimit)
	require.NoError(t, s.Pin(ctx, domain.ProjectPreferenceScope("p1"), "f"), "the limit is per scope")
}

func TestStartMergesScopesRecentsAndSuggestions(t *testing.T) {
	now := time.Now()
	usage := &fakeUsage{uses: []domain.WorkflowUse{
		{WorkflowID: "orchestrator", LastUsed: now}, // phase 0 session: agent id, not a workflow
		{WorkflowID: "ticket", LastUsed: now},       // pinned: not repeated
		{WorkflowID: "review", LastUsed: now.Add(-time.Hour)},
		{WorkflowID: "audit", LastUsed: now.Add(-2 * time.Hour)},
		{WorkflowID: "debug", LastUsed: now.Add(-3 * time.Hour)},
		{WorkflowID: "quick", LastUsed: now.Add(-4 * time.Hour)},
	}}
	s := newService(t, usage)
	ctx := context.Background()
	catalog := []string{"ticket", "cadrage", "review", "audit", "debug", "quick", "feature", "sweep", "onboarding"}

	require.NoError(t, s.Pin(ctx, domain.ProjectPreferenceScope("p1"), "cadrage"))
	require.NoError(t, s.Pin(ctx, domain.ProjectPreferenceScope("p1"), "removed-workflow"))
	for _, id := range []string{"ticket", "cadrage", "feature", "sweep", "onboarding"} {
		require.NoError(t, s.Pin(ctx, domain.PreferenceScopeGlobal, id))
	}

	e, err := s.Start(ctx, Context{ProjectID: "p1"}, catalog)
	require.NoError(t, err)
	assert.Equal(t, []PinnedEntry{
		{WorkflowID: "cadrage", Scope: "project:p1"},
		{WorkflowID: "ticket", Scope: "global"},
		{WorkflowID: "feature", Scope: "global"},
		{WorkflowID: "sweep", Scope: "global"},
		{WorkflowID: "onboarding", Scope: "global"},
	}, e.Pinned, "project pins first, no duplicate, unknown skipped, at most 5")
	assert.Equal(t, "p1", usage.project, "recents of the project")
	require.Len(t, e.Recent, MaxRecents)
	assert.Equal(t, []string{"review", "audit", "debug"}, recentIDs(e.Recent))
	assert.Empty(t, e.Suggested)

	// Hub mode, nothing pinned nor used: default suggestions from the catalogue.
	empty := newService(t, &fakeUsage{})
	e, err = empty.Start(ctx, Context{}, []string{"review", "ticket"})
	require.NoError(t, err)
	assert.Empty(t, e.Pinned)
	assert.Equal(t, []string{"ticket", "review"}, e.Suggested)

	e, err = New(empty.store, nil).Start(ctx, Context{TeamID: "core"}, nil)
	require.NoError(t, err)
	assert.Equal(t, DefaultSuggestions, e.Suggested, "nil catalogue: no filtering, nil usage: no recents")
}

func TestContextScopes(t *testing.T) {
	assert.Equal(t, []string{"global"}, Context{}.Scopes())
	assert.Equal(t, []string{"project:p", "team:t", "global"}, Context{ProjectID: "p", TeamID: "t"}.Scopes())
}

func TestUISettings(t *testing.T) {
	s := newService(t, nil)
	ctx := context.Background()
	var v struct{ Collapsed []string }
	ok, err := s.Get(ctx, "global", "start.collapsed", &v)
	require.NoError(t, err)
	assert.False(t, ok)
	require.NoError(t, s.Set(ctx, "global", "start.collapsed", map[string][]string{"Collapsed": {"quality"}}))
	ok, err = s.Get(ctx, "global", "start.collapsed", &v)
	require.NoError(t, err)
	assert.True(t, ok)
	assert.Equal(t, []string{"quality"}, v.Collapsed)
}

func TestMessages(t *testing.T) {
	prev := i18n.Locale()
	i18n.SetLocale("fr")
	t.Cleanup(func() { i18n.SetLocale(prev) })
	assert.Equal(t, "★ ticket épinglé (ce projet)", PinMessage("ticket", "project:p1", true))
	assert.Equal(t, "ticket désépinglé (tout le hub)", PinMessage("ticket", "global", false))
	assert.Contains(t, ErrorMessage(ErrPinLimit), "5")
	assert.Contains(t, ErrorMessage(errors.New("boom")), "boom")
	assert.Equal(t, "cette équipe", ScopeLabel("team:core"))
}

func recentIDs(rs []RecentEntry) []string {
	out := make([]string, 0, len(rs))
	for _, r := range rs {
		out = append(out, r.WorkflowID)
	}
	return out
}
