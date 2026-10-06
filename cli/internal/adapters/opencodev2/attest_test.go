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
