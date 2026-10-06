package views

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/config"
)

func TestSettingsBudgetFields(t *testing.T) {
	v := &SettingsView{live: &config.Config{}}
	v.buildFields()
	idx := map[string]int{}
	for i, f := range v.fields {
		if f.Key != "" {
			idx[f.Key] = i
		}
	}
	require.Contains(t, idx, "limits_session_budget_usd")
	assert.Greater(t, idx["limits_max_active_sessions"], idx["session_idle_sleep"], "after the Sessions settings")
	f := v.fields[idx["limits_session_budget_usd"]]
	f.Set("2.5")
	assert.Equal(t, 2.5, v.live.Limits.SessionBudgetUSD)
	f.Set("lots") // ignored
	assert.Equal(t, "2.5", f.Get())
	m := v.fields[idx["limits_models"]]
	m.Set("eu.anthropic.*, us.*")
	assert.Equal(t, []string{"eu.anthropic.*", "us.*"}, v.live.Limits.Models)
	m.Set("")
	assert.Empty(t, v.live.Limits.Models)
}
