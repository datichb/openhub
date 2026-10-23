package tracker

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNeedsTokenDecl(t *testing.T) {
	def := DefaultTokenKey("gitlab")
	assert.Equal(t, "openhub.tracker.gitlab.token", def)

	tests := []struct {
		name   string
		eff    EffectiveTrackerConfig
		key    string
		server string
		want   bool
	}{
		{"nothing declared, default key kept", EffectiveTrackerConfig{Type: "gitlab"}, def, "", false},
		{"nothing declared, another key", EffectiveTrackerConfig{Type: "gitlab"}, "forge.token", "", true},
		{"nothing declared, default key for a server", EffectiveTrackerConfig{Type: "gitlab"}, def, "https://gitlab.forge.example", true},
		{"declared key and server kept", EffectiveTrackerConfig{Type: "gitlab", TrackerTokenKey: "k", TrackerTokenURL: "https://s"}, "k", "https://s", false},
		{"declared key, server changed", EffectiveTrackerConfig{Type: "gitlab", TrackerTokenKey: "k", TrackerTokenURL: "https://s"}, "k", "https://other", true},
		{"declared key, server cleared", EffectiveTrackerConfig{Type: "gitlab", TrackerTokenKey: "k", TrackerTokenURL: "https://s"}, "k", "", true},
		{"declared key, back to the default", EffectiveTrackerConfig{Type: "gitlab", TrackerTokenKey: "k"}, def, "", true},
		{"jira nothing declared", EffectiveTrackerConfig{Type: "jira"}, DefaultTokenKey("jira"), "", true},
		{"jira declared key kept, server ignored", EffectiveTrackerConfig{Type: "jira", TrackerTokenKey: "k"}, "k", "https://x", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, tt.eff.NeedsTokenDecl(tt.key, tt.server))
		})
	}
}
