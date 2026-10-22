package cmd

import (
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/teamstate"
)

// The policies step of the team wizards writes the chosen recommended
// policies through the policy service: one commit, the existing policies are
// kept, a recommended policy replaces the one with the same name.
func TestTeamPoliciesStepWritesThroughThePolicyService(t *testing.T) {
	repo, err := teamstate.InitSolo(t.Context(), filepath.Join(t.TempDir(), "state"), teamstate.Member{ID: "alice"})
	require.NoError(t, err)
	byName := func() map[string]teamstate.Policy {
		list, err := repo.LoadPolicies("")
		require.NoError(t, err)
		m := map[string]teamstate.Policy{}
		for _, p := range list {
			m[p.Name] = p
		}
		return m
	}
	lastCommit := func() string {
		out, err := exec.Command("git", "-C", repo.Path(), "log", "-1", "--format=%s").Output()
		require.NoError(t, err)
		return strings.TrimSpace(string(out))
	}
	require.NoError(t, repo.SavePolicies(t.Context(), map[string]teamstate.Policy{
		"mine":          {Type: teamstate.PolicyTypeBoolean, Enabled: true, Enforcement: teamstate.EnforcementWarn},
		"branch_naming": {Type: teamstate.PolicyTypeRegex, Rule: "^old/", Enforcement: teamstate.EnforcementWarn},
	}))
	require.NoError(t, repo.CommitAndPush(t.Context(), "seed", "policies.toml"))

	s := &teamStepState{Ctx: t.Context(), Repo: repo, HasPolicies: true,
		SelectedPolicies: []string{"branch_naming", "max_ticket_wip"}}
	step := buildTeamPoliciesStep(s, teamStepOpts{})
	require.NoError(t, step.OnDone())

	got := byName()
	require.Len(t, got, 3)
	assert.Equal(t, teamstate.PolicyTypeBoolean, got["mine"].Type, "the existing policy is kept")
	assert.Equal(t, teamstate.EnforcementRefuse, got["branch_naming"].Enforcement, "the recommended policy wins")
	assert.NotEqual(t, "^old/", got["branch_naming"].Rule)
	assert.Equal(t, 2, got["max_ticket_wip"].Max)
	assert.Equal(t, "policies: update", lastCommit())

	// Nothing chosen: nothing written.
	s.SelectedPolicies = nil
	require.NoError(t, buildTeamPoliciesStep(s, teamStepOpts{}).OnDone())
	assert.Equal(t, "policies: update", lastCommit())
	assert.Len(t, byName(), 3)
}
