package views

import (
	"testing"

	"github.com/rivo/tview"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/config"
	"github.com/datichb/openhub/cli/internal/i18n"
	"github.com/datichb/openhub/cli/internal/tracker"
)

func teamDetailFor(res TeamResolution, promote func(string, string)) *TeamDetailView {
	return NewTeamDetailView(TeamDetailViewConfig{
		GetMCPConfig:          func() config.MCPConfig { return config.MCPConfig{} },
		GetTrackerLocalConfig: func() config.TrackerLocalConfig { return config.TrackerLocalConfig{} },
		ResolveTeam:           func() TeamResolution { return res },
		GetSecrets:            func() tracker.SecretGetter { return nil },
		GetHubConfig:          func() *config.Config { return &config.Config{} },
		PromoteSolo:           promote,
	})
}

func fieldByKey(v *TeamDetailView, key string) (configField, bool) {
	for _, f := range v.fields {
		if f.Key == key {
			return f, true
		}
	}
	return configField{}, false
}

// P2-T17 (governance, read only) and P2-T16 (« Passer en équipe »).
func TestTeamDetailGovernanceAndPromote(t *testing.T) {
	var promoted string
	v := teamDetailFor(TeamResolution{Solo: true, TeamID: "solo", StatePath: t.TempDir()}, func(id, remote string) { promoted = id + "→" + remote })
	sh := &formShell{}
	v.SetShell(sh)
	v.Mount(tview.NewFlex(), nil)

	gov, ok := fieldByKey(v, "governance.publish")
	require.True(t, ok)
	assert.Equal(t, CfgFieldReadonly, gov.Kind)
	assert.Equal(t, i18n.T("tui.team.governance.value_any_member"), gov.Get())
	_, ok = fieldByKey(v, "solo")
	assert.True(t, ok)
	act, ok := fieldByKey(v, "solo.promote")
	require.True(t, ok)

	act.Set("")
	require.NotNil(t, sh.input)
	sh.input(" git@example.com:me/ts.git ")
	require.NotEmpty(t, sh.modal)
	sh.modal[0].Callback()
	assert.Equal(t, "solo→git@example.com:me/ts.git", promoted)

	// A regular team: governance, no solo rows; without team-state: nothing.
	v = teamDetailFor(TeamResolution{Enabled: true, TeamID: "core", StatePath: t.TempDir()}, nil)
	v.Mount(tview.NewFlex(), nil)
	_, ok = fieldByKey(v, "governance.publish")
	assert.True(t, ok)
	_, ok = fieldByKey(v, "solo.promote")
	assert.False(t, ok)
	v = teamDetailFor(TeamResolution{}, nil)
	v.Mount(tview.NewFlex(), nil)
	_, ok = fieldByKey(v, "governance.publish")
	assert.False(t, ok)

	assert.Contains(t, GovernanceLabel("maintainers"), "maintainers")
}

// QB2: the team [parallel] settings have no effect in v5 (the session
// restrictions are the [limits] of I6): they are no longer editable.
func TestTeamDetailHasNoParallelSetting(t *testing.T) {
	v := teamDetailFor(TeamResolution{Enabled: true, TeamID: "core", StatePath: t.TempDir()}, nil)
	v.SetShell(&formShell{})
	v.Mount(tview.NewFlex(), nil)
	_, ok := fieldByKey(v, "stale_days")
	require.True(t, ok, "collaboration section built")
	_, ok = fieldByKey(v, "max_sessions")
	assert.False(t, ok)
}

// A42: the detail of a solo space hides what does not apply to it (tracker,
// notifications, collaboration, tracker overrides); a team keeps them.
func TestTeamDetailSoloHidesTeamFeatures(t *testing.T) {
	hidden := []string{"tracker_url", "webhook_url", "channel", "stale_days", "tracker_enabled", "auto_sync"}
	v := teamDetailFor(TeamResolution{Solo: true, TeamID: "solo", StatePath: t.TempDir()}, func(string, string) {})
	v.SetShell(&formShell{})
	v.Mount(tview.NewFlex(), nil)
	for _, k := range hidden {
		_, ok := fieldByKey(v, k)
		assert.False(t, ok, k)
	}
	for _, k := range []string{"governance.publish", "solo.promote", "mcp", "workflow"} {
		_, ok := fieldByKey(v, k)
		assert.True(t, ok, k)
	}
	for _, f := range v.fields {
		if f.Kind == CfgFieldSectionHeader {
			assert.NotEqual(t, i18n.T("tui.config.section.tracker"), f.Label)
			assert.NotEqual(t, i18n.T("tui.config.section.notifications"), f.Label)
		}
	}
	assert.NotContains(t, v.StatusHints(), "discovery")

	v = teamDetailFor(TeamResolution{Enabled: true, TeamID: "core", StatePath: t.TempDir()}, nil)
	v.SetShell(&formShell{})
	v.Mount(tview.NewFlex(), nil)
	for _, k := range hidden {
		_, ok := fieldByKey(v, k)
		assert.True(t, ok, k)
	}
}
