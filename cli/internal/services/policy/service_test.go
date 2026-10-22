package policy

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	toml "github.com/pelletier/go-toml/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/teamstate"
)

func newService(t *testing.T) (*Service, *teamstate.Repo) {
	t.Helper()
	repo, err := teamstate.InitSolo(context.Background(), filepath.Join(t.TempDir(), "state"), teamstate.Member{ID: "alice"})
	require.NoError(t, err)
	return New(repo), repo
}

func load(t *testing.T, repo *teamstate.Repo) map[string]teamstate.Policy {
	t.Helper()
	list, err := repo.LoadPolicies("")
	require.NoError(t, err)
	m := make(map[string]teamstate.Policy, len(list))
	for _, p := range list {
		m[p.Name] = p
	}
	return m
}

func lastCommit(t *testing.T, repo *teamstate.Repo) string {
	t.Helper()
	out, err := exec.Command("git", "-C", repo.Path(), "log", "-1", "--format=%s").Output()
	require.NoError(t, err)
	return strings.TrimSpace(string(out))
}

func TestAddUpdateRemove(t *testing.T) {
	svc, repo := newService(t)
	ctx := t.Context()

	require.NoError(t, svc.Add(ctx, "branch-naming", teamstate.Policy{
		Type: teamstate.PolicyTypeRegex, Target: "branch_name", Rule: `^(feat|fix)/`,
		Enforcement: teamstate.EnforcementRefuse, Message: "feat/*",
	}))
	assert.Equal(t, "policies: add branch-naming", lastCommit(t, repo))
	require.NoError(t, svc.Add(ctx, "no-todo", teamstate.Policy{
		Type: teamstate.PolicyTypeForbiddenPattern, Patterns: []string{"TODO", "FIXME"},
		Scope: "diff_only", Enforcement: teamstate.EnforcementWarn,
	}))
	require.NoError(t, svc.Add(ctx, "max-wip", teamstate.Policy{
		Type: teamstate.PolicyTypeLimit, Target: "wip_tickets", Max: 3, Enforcement: teamstate.EnforcementWarn,
	}))
	require.NoError(t, svc.Add(ctx, "require-tests", teamstate.Policy{
		Type: teamstate.PolicyTypeBoolean, Target: "tests", Enabled: true, Enforcement: teamstate.EnforcementWarn,
	}))

	got := load(t, repo)
	require.Len(t, got, 4)
	assert.Equal(t, `^(feat|fix)/`, got["branch-naming"].Rule)
	assert.Equal(t, []string{"TODO", "FIXME"}, got["no-todo"].Patterns)
	assert.Equal(t, 3, got["max-wip"].Max)
	assert.True(t, got["require-tests"].Enabled)

	// Update in place.
	p := got["branch-naming"]
	p.Name = ""
	p.Enforcement = teamstate.EnforcementWarn
	p.Message = "autre message"
	require.NoError(t, svc.Update(ctx, "branch-naming", p))
	assert.Equal(t, "policies: update branch-naming", lastCommit(t, repo))
	got = load(t, repo)
	require.Len(t, got, 4)
	assert.Equal(t, teamstate.EnforcementWarn, got["branch-naming"].Enforcement)
	assert.Equal(t, "autre message", got["branch-naming"].Message)

	// Rename.
	p.Name = "branches"
	require.NoError(t, svc.Update(ctx, "branch-naming", p))
	got = load(t, repo)
	require.Len(t, got, 4)
	assert.NotContains(t, got, "branch-naming")
	assert.Contains(t, got, "branches")

	// Enforcement only.
	require.NoError(t, svc.SetEnforcement(ctx, "no-todo", teamstate.EnforcementDisabled))
	got = load(t, repo)
	assert.Equal(t, teamstate.EnforcementDisabled, got["no-todo"].Enforcement)
	assert.Equal(t, []string{"TODO", "FIXME"}, got["no-todo"].Patterns)

	// Remove, down to an empty file that still loads.
	for _, n := range []string{"branches", "no-todo", "max-wip", "require-tests"} {
		require.NoError(t, svc.Remove(ctx, n))
	}
	assert.Equal(t, "policies: remove require-tests", lastCommit(t, repo))
	assert.Empty(t, load(t, repo))
}

func TestSpecialCharacters(t *testing.T) {
	svc, repo := newService(t)
	ctx := t.Context()
	names := []string{`say "hi"`, `a.b`, "tab\tname", `back\slash`, "é accentué", "nom avec espaces", `[crochets]`}
	for _, n := range names {
		require.NoError(t, svc.Add(ctx, n, teamstate.Policy{
			Type:        teamstate.PolicyTypeForbiddenPattern,
			Patterns:    []string{`password="x"`, `C:\dir`, "ligne\nsuivante", `'quote'`},
			Scope:       "diff_only",
			Enforcement: teamstate.EnforcementRefuse,
			Message:     "Interdit : \"secret\" \\ tabulation\t fin",
		}), n)
	}

	data, err := os.ReadFile(filepath.Join(repo.Path(), "policies.toml"))
	require.NoError(t, err)
	var raw map[string]map[string]map[string]any
	require.NoError(t, toml.Unmarshal(data, &raw), "policies.toml is valid TOML:\n%s", data)
	assert.Len(t, raw["policies"], len(names))

	got := load(t, repo)
	require.Len(t, got, len(names))
	for _, n := range names {
		p, ok := got[n]
		require.True(t, ok, n)
		assert.Equal(t, []string{`password="x"`, `C:\dir`, "ligne\nsuivante", `'quote'`}, p.Patterns)
		assert.Equal(t, "Interdit : \"secret\" \\ tabulation\t fin", p.Message)
	}
}

func TestErrors(t *testing.T) {
	svc, repo := newService(t)
	ctx := t.Context()
	ok := teamstate.Policy{Type: teamstate.PolicyTypeBoolean, Enabled: true, Enforcement: teamstate.EnforcementWarn}

	require.NoError(t, svc.Add(ctx, "p", ok))
	assert.ErrorIs(t, svc.Add(ctx, "p", ok), ErrExists)
	assert.ErrorIs(t, svc.Add(ctx, " ", ok), ErrNoName)
	assert.ErrorIs(t, svc.Add(ctx, "q", teamstate.Policy{Type: "nope", Enforcement: teamstate.EnforcementWarn}), ErrBadType)
	assert.ErrorIs(t, svc.Add(ctx, "q", teamstate.Policy{Type: teamstate.PolicyTypeBoolean, Enforcement: "loud"}), ErrBadEnforced)
	assert.ErrorIs(t, svc.Update(ctx, "absent", ok), ErrNotFound)
	assert.ErrorIs(t, svc.Remove(ctx, "absent"), ErrNotFound)
	assert.ErrorIs(t, svc.SetEnforcement(ctx, "absent", teamstate.EnforcementWarn), ErrNotFound)
	assert.ErrorIs(t, svc.SetEnforcement(ctx, "p", "loud"), ErrBadEnforced)

	require.NoError(t, svc.Add(ctx, "r", ok))
	renamed := ok
	renamed.Name = "p"
	assert.ErrorIs(t, svc.Update(ctx, "r", renamed), ErrExists)
	assert.Len(t, load(t, repo), 2, "failed edits change nothing")
}

func TestInvalidFileIsNotOverwritten(t *testing.T) {
	svc, repo := newService(t)
	path := filepath.Join(repo.Path(), "policies.toml")
	require.NoError(t, os.WriteFile(path, []byte("[policies.x\ntype = "), 0o644))
	err := svc.Add(t.Context(), "p", teamstate.Policy{Type: teamstate.PolicyTypeBoolean, Enforcement: teamstate.EnforcementWarn})
	assert.ErrorIs(t, err, teamstate.ErrPolicyFileInvalid)
	data, _ := os.ReadFile(path)
	assert.Equal(t, "[policies.x\ntype = ", string(data))
}

func TestNotCloned(t *testing.T) {
	svc := New(teamstate.NewRepo("", filepath.Join(t.TempDir(), "absent")))
	err := svc.Add(t.Context(), "p", teamstate.Policy{Type: teamstate.PolicyTypeBoolean, Enforcement: teamstate.EnforcementWarn})
	assert.ErrorIs(t, err, teamstate.ErrNotCloned)
}
