package config_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/datichb/openhub/cli/internal/config"
)

func TestResolveBool(t *testing.T) {
	t.Parallel()
	b := func(v bool) *bool { return &v }
	tests := []struct {
		name                      string
		team, enforced, hub, proj *bool
		teamID                    string
		def                       bool
		want                      bool
		wantSource                config.ConfigSource
		wantLocked                bool
	}{
		{"an enforced team value wins over everything", b(true), b(true), b(false), b(false), "acme", false, true, config.SourceTeamEnforced, true},
		{"the project overrides the hub and the team recommendation", b(true), nil, b(true), b(false), "acme", true, false, config.SourceProject, false},
		{"the hub wins over the team recommendation", b(true), nil, b(false), nil, "acme", true, false, config.SourceHub, false},
		{"the team recommendation is the fallback", b(true), nil, nil, nil, "acme", false, true, config.SourceTeamRecommended, false},
		{"the default when nothing is set", nil, nil, nil, nil, "", true, true, config.SourceDefault, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := config.ResolveBool(tt.team, tt.enforced, tt.hub, tt.proj, tt.teamID, tt.def)
			assert.Equal(t, tt.want, got.Value, "value")
			assert.Equal(t, tt.wantSource, got.Source, "source")
			assert.Equal(t, tt.wantLocked, got.Locked, "locked")
			if tt.wantSource == config.SourceTeamEnforced {
				assert.Equal(t, tt.teamID, got.TeamID, "team id")
			}
		})
	}
}

func TestResolveString(t *testing.T) {
	t.Parallel()
	enforced := true
	tests := []struct {
		name           string
		team           string
		enforced       *bool
		hub, proj, def string
		want           string
		wantSource     config.ConfigSource
		wantLocked     bool
	}{
		{"an enforced team value wins", "https://gitlab.team.com", &enforced, "https://gitlab.mine.com", "", "", "https://gitlab.team.com", config.SourceTeamEnforced, true},
		{"the project overrides the others", "team-url", nil, "hub-url", "project-url", "default", "project-url", config.SourceProject, false},
		{"the team recommendation comes before the default", "team-model", nil, "", "", "default-model", "team-model", config.SourceTeamRecommended, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := config.ResolveString(tt.team, tt.enforced, tt.hub, tt.proj, "acme", tt.def)
			assert.Equal(t, tt.want, got.Value, "value")
			assert.Equal(t, tt.wantSource, got.Source, "source")
			assert.Equal(t, tt.wantLocked, got.Locked, "locked")
		})
	}
}
