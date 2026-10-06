package limits

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestResolveOffByDefault(t *testing.T) {
	r := Resolve(Input{})
	assert.True(t, r.IsZero())
	assert.Empty(t, r.Origins)
}

func TestResolveCascade(t *testing.T) {
	team := &TeamLimits{
		Recommended: Limits{SessionBudgetUSD: 3, DailyBudgetUSD: 50, MaxActiveSessions: 4},
		Enforced:    Limits{SessionBudgetUSD: 10, MemoryMB: 8000, Models: []string{"eu.anthropic.*"}},
	}
	r := Resolve(Input{
		Hub:       Limits{DailyBudgetUSD: 20, MemoryMB: 12000},
		Team:      team,
		Project:   Limits{MaxActiveSessions: 2, MemoryMB: 1}, // memory is a machine setting
		Workflow:  Limits{SessionBudgetUSD: 15, Models: []string{"eu.anthropic.claude-haiku-*", "us.openai.*"}},
		ProjectID: "p1",
	})
	assert.Equal(t, 10.0, r.SessionBudgetUSD, "the team ceiling bounds the workflow")
	assert.Equal(t, OriginTeamEnforced, r.Origins[FieldSessionBudget])
	assert.Equal(t, 20.0, r.DailyBudgetUSD, "hub before the team recommendation")
	assert.Equal(t, OriginHub, r.Origins[FieldDailyBudget])
	assert.Equal(t, 2, r.MaxActiveSessions)
	assert.Equal(t, 8000, r.MemoryMB, "enforced ceiling below the hub value")
	assert.Equal(t, []string{"eu.anthropic.claude-haiku-*"}, r.Models, "only what the team allows")
	assert.Equal(t, ScopeGlobal, r.DailyScope())
	assert.Equal(t, "project:p1", r.ActiveScope())
	assert.Equal(t, "p1", ScopeProject(r.ActiveScope()))
	assert.Equal(t, "", ScopeProject(ScopeGlobal))

	// A lower value than the ceiling is kept; nothing set but a ceiling = the ceiling.
	r = Resolve(Input{Workflow: Limits{SessionBudgetUSD: 2}, Team: team})
	assert.Equal(t, 2.0, r.SessionBudgetUSD)
	assert.Equal(t, OriginWorkflow, r.Origins[FieldSessionBudget])
	assert.Equal(t, []string{"eu.anthropic.*"}, r.Models)
	r = Resolve(Input{Workflow: Limits{Models: []string{"us.*"}}, Team: team})
	assert.Equal(t, []string{"eu.anthropic.*"}, r.Models, "nothing covered: the team list")
}

func TestSaveLoad(t *testing.T) {
	dir := t.TempDir()
	r := Resolve(Input{Hub: Limits{SessionBudgetUSD: 1.5}, ProjectID: "p"})
	require.NoError(t, Save(dir, "s1", r))
	got, err := Load(dir, "s1")
	require.NoError(t, err)
	assert.Equal(t, 1.5, got.SessionBudgetUSD)
	assert.Equal(t, OriginHub, got.Origins[FieldSessionBudget])
	none, err := Load(dir, "s2")
	require.NoError(t, err)
	assert.True(t, none.IsZero())
	require.NoError(t, Save(dir, "s1", Resolve(Input{})))
	got, _ = Load(dir, "s1")
	assert.True(t, got.IsZero())
}

func TestSetAndValue(t *testing.T) {
	var l Limits
	require.NoError(t, l.Set(FieldSessionBudget, "2.5"))
	require.NoError(t, l.Set(FieldMaxActive, "3"))
	require.NoError(t, l.Set(FieldModels, "a, b*"))
	assert.Equal(t, "2.5", l.Value(FieldSessionBudget))
	assert.Equal(t, "3", l.Value(FieldMaxActive))
	assert.Equal(t, []string{"a", "b*"}, l.Models)
	require.NoError(t, l.Set(FieldModels, ""))
	assert.Nil(t, l.Models)
	require.NoError(t, l.Set(FieldMaxActive, "off"))
	assert.Zero(t, l.MaxActiveSessions)
	assert.Error(t, l.Set(FieldDailyBudget, "-1"))
	assert.Error(t, l.Set(FieldMemory, "lots"))
	assert.Error(t, l.Set("nope", "1"))
}
