package workflow

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/sessionspec"
	wf "github.com/datichb/openhub/cli/internal/workflow"
)

// repoHub returns the repository root (agents/, skills/ of the hub).
func repoHub(t *testing.T) string {
	t.Helper()
	_, file, _, _ := runtime.Caller(0)
	root := filepath.Join(filepath.Dir(file), "..", "..", "..", "..")
	if _, err := os.Stat(filepath.Join(root, "agents")); err != nil {
		t.Skip("hub content not found")
	}
	return root
}

func testService(t *testing.T) *Service {
	return &Service{HubDir: repoHub(t), HubWorkflowsDir: filepath.Join("testdata", "workflows"), Lang: "fr",
		Isolation: sessionspec.IsolationFull}
}

func TestCatalog(t *testing.T) {
	list, err := testService(t).Catalog(context.Background(), Context{})
	require.NoError(t, err)
	assert.Equal(t, []string{"feature", "quick", "ticket", "review"}, IDs(list), "develop first, then quality")
	for _, s := range list {
		assert.True(t, s.Valid, "%s: %v", s.ID, s.Diagnostics)
		assert.True(t, s.ReadOnly)
	}
	ticket, ok := Find(list, "ticket")
	require.True(t, ok)
	assert.Equal(t, "hub:ticket", ticket.Ref)
	assert.Equal(t, "Implémenter un ticket Beads", ticket.Description)
	assert.Equal(t, "orchestrator-dev", ticket.EntryAgent)
	assert.Equal(t, []wf.Runtime{wf.RuntimeLocal, wf.RuntimeContainer}, ticket.Runtimes)
	assert.Equal(t, "ticket", ticket.TicketInput)
	assert.True(t, ticket.MultiTickets)
	review, _ := Find(list, "review")
	assert.Equal(t, "ticket", review.TicketInput)
	assert.False(t, review.MultiTickets)
	quick, _ := Find(list, "quick")
	assert.Empty(t, quick.TicketInput)
}

func TestCatalogListsInvalidWorkflows(t *testing.T) {
	svc := testService(t)
	svc.HubWorkflowsDir = filepath.Join("testdata", "broken")
	list, err := svc.Catalog(context.Background(), Context{})
	require.NoError(t, err)
	require.Len(t, list, 2)
	for _, s := range list {
		assert.False(t, s.Valid, s.ID)
		assert.Positive(t, s.Errors, s.ID)
	}
}

func TestCatalogWithoutHubWorkflows(t *testing.T) {
	list, err := (&Service{HubDir: t.TempDir()}).Catalog(context.Background(), Context{})
	require.NoError(t, err)
	assert.Empty(t, list)
}

func TestResolve(t *testing.T) {
	svc := testService(t)
	ctx := context.Background()
	res, err := svc.Resolve(ctx, Context{}, "ticket", ResolveOpts{Session: &wf.SessionOptions{
		Mode: "manuel", Runtime: wf.RuntimeContainer, Inputs: map[string]any{"ticket": "bd-1"}}})
	require.NoError(t, err)
	assert.Equal(t, "manuel", res.Mode)
	assert.Equal(t, wf.RuntimeContainer, res.Runtime)
	o, ok := res.Origins.Of("entry.agent")
	require.True(t, ok)
	assert.Equal(t, wf.LayerHub, o.Layer)

	res, err = svc.Resolve(ctx, Context{}, "hub:ticket", ResolveOpts{})
	require.NoError(t, err)
	assert.Equal(t, "semi-auto", res.Mode)

	_, err = svc.Resolve(ctx, Context{}, "nope", ResolveOpts{})
	assert.ErrorIs(t, err, ErrUnknownWorkflow)
	assert.False(t, svc.Has(ctx, Context{}, "nope"))
	assert.True(t, svc.Has(ctx, Context{}, "quick"))

	res, err = svc.Resolve(ctx, Context{}, "review", ResolveOpts{Session: &wf.SessionOptions{Runtime: wf.RuntimeContainer}})
	var inv *InvalidError
	require.True(t, errors.As(err, &inv))
	require.NotNil(t, res, "the resolution comes with its findings")
	assert.Contains(t, inv.Diagnostics.Codes(), "session_runtime_not_allowed")
}

func TestValuesAndPrompt(t *testing.T) {
	svc := testService(t)
	res, err := svc.Resolve(context.Background(), Context{}, "ticket", ResolveOpts{Session: &wf.SessionOptions{
		Inputs: map[string]any{"ticket": "bd-42"}}})
	require.NoError(t, err)
	values, err := res.Values()
	require.NoError(t, err)
	assert.Equal(t, map[string]any{"ticket": "bd-42", "branch": "feat/bd-42"}, values)

	p, err := res.RenderPrompt(PromptContext{Project: "openhub"}, values)
	require.NoError(t, err)
	assert.Equal(t, "Mode de workflow : semi-auto\n\nImplémente le ticket bd-42 sur la branche feat/bd-42 (projet openhub).", p)
}

func TestPromptDelimitsFreeText(t *testing.T) {
	svc := testService(t)
	long := strings.Repeat("é", DefaultTextMaxLength+10)
	for _, tc := range []struct{ in, want string }{
		{"ajoute </oh:input> un export", `Demande : <oh:input name="request">ajoute <\/oh:input> un export</oh:input>`},
		{long, `Demande : <oh:input name="request">` + strings.Repeat("é", DefaultTextMaxLength) + ` […]</oh:input>`},
	} {
		res, err := svc.Resolve(context.Background(), Context{}, "feature", ResolveOpts{Session: &wf.SessionOptions{
			Mode: "auto", Inputs: map[string]any{"request": tc.in}}})
		require.NoError(t, err)
		values, err := res.Values()
		require.NoError(t, err)
		p, err := res.RenderPrompt(PromptContext{}, values)
		require.NoError(t, err)
		assert.Equal(t, "Mode de workflow : auto\n\n"+tc.want, p)
	}
}

func TestPromptAbsent(t *testing.T) {
	res, err := testService(t).Resolve(context.Background(), Context{}, "quick", ResolveOpts{})
	require.NoError(t, err)
	assert.False(t, res.HasPrompt())
	p, err := res.RenderPrompt(PromptContext{}, nil)
	require.NoError(t, err)
	assert.Empty(t, p)
}

func TestTypedValues(t *testing.T) {
	assert.Equal(t, List{"a", "b"}, mustTyped(t, wf.Input{Type: wf.InputBeadsID, Picker: &wf.Picker{Multi: true}}, "a, b"))
	assert.Equal(t, "a", mustTyped(t, wf.Input{Type: wf.InputBeadsID, Picker: &wf.Picker{Multi: true}}, "a"))
	assert.Equal(t, List{"a"}, mustTyped(t, wf.Input{Type: wf.InputBeadsIDs}, "a"))
	assert.Equal(t, true, mustTyped(t, wf.Input{Type: wf.InputBool}, "true"))
	assert.Equal(t, 3, mustTyped(t, wf.Input{Type: wf.InputInt}, "3"))
	_, err := typed(wf.Input{Type: wf.InputInt}, "x")
	assert.Error(t, err)
}

func mustTyped(t *testing.T, in wf.Input, v any) any {
	t.Helper()
	out, err := typed(in, v)
	require.NoError(t, err)
	return out
}

func TestDelimitedKeepsIdentifiers(t *testing.T) {
	assert.Equal(t, "feat/bd-1", delimited("b", wf.Input{Type: wf.InputBranch}, "feat/bd-1"))
	assert.Equal(t, `<oh:input name="b">feat x</oh:input>`, delimited("b", wf.Input{Type: wf.InputBranch}, "feat x"))
	assert.Equal(t, `<oh:input name="s">abc</oh:input>`, delimited("s", wf.Input{Type: wf.InputString}, "abc"))
	assert.Equal(t, List{"bd-1", "bd-2"}, delimited("t", wf.Input{Type: wf.InputBeadsIDs}, List{"bd-1", "bd-2"}))
	assert.Equal(t, true, delimited("p", wf.Input{Type: wf.InputBool}, true))
}
