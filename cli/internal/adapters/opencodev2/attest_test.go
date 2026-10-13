package opencodev2

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/datichb/openhub/cli/internal/sessionspec"
)

func TestSkillVisibleOnServerUsesServerRules(t *testing.T) {
	b := sessionspec.BundleSpec{
		Agents: []sessionspec.AgentDef{{ID: "lead", Mode: "primary"}, {ID: "helper", Mode: "subagent"}, {ID: "quiet", Mode: "subagent"}},
		Skills: []sessionspec.SkillDef{{ID: "alpha"}},
	}
	closed := []sessionspec.PermissionRule{{Action: "*", Resource: "*", Effect: sessionspec.EffectAllow},
		{Action: "skill", Resource: "*", Effect: sessionspec.EffectDeny}, {Action: "skill", Resource: "alpha", Effect: sessionspec.EffectAllow}}
	reopened := append(append([]sessionspec.PermissionRule(nil), closed...), sessionspec.PermissionRule{Action: "skill", Resource: "report", Effect: sessionspec.EffectAllow})
	agents := []Agent{
		{ID: "lead", Permissions: closed},
		{ID: "helper", Permissions: reopened}, // what the server applies, whatever was rendered
		{ID: "quiet"},                         // no rules listed: judged on the rendered rules
	}
	assert.Equal(t, []string{"helper"}, skillVisibleOnServer(agents, b, "report"))
	assert.Empty(t, skillVisibleOnServer(agents[:1], b, "report"))
}

// L2: a bundle skill the server exposes to an agent outside its catalogue
// breaks the closed world of that agent.
func TestOutsideCatalogue(t *testing.T) {
	b := sessionspec.BundleSpec{
		Agents: []sessionspec.AgentDef{{ID: "lead", Skills: []string{"alpha"}}, {ID: "helper", Skills: []string{}}, {ID: "old"}},
		Skills: []sessionspec.SkillDef{{ID: "alpha"}},
	}
	open := []sessionspec.PermissionRule{{Action: "skill", Resource: "alpha", Effect: sessionspec.EffectAllow}}
	agents := []Agent{{ID: "lead", Permissions: open}, {ID: "helper", Permissions: open}, {ID: "old", Permissions: open}}
	assert.Equal(t, []string{"helper"}, outsideCatalogue(agents, b, "alpha"), "old: no catalogue, every bundle skill")
	assert.Empty(t, outsideCatalogue(agents[:1], b, "alpha"))
	// Rendered rules (server listing no rules) follow the catalogue.
	assert.Empty(t, outsideCatalogue([]Agent{{ID: "lead"}, {ID: "helper"}}, b, "alpha"))
}
