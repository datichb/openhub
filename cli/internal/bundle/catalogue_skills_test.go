package bundle

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func shippedSkillFile(t *testing.T, b *Bundle, id string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(b.Dir, skillsDir, id, "SKILL.md"))
	require.NoError(t, err)
	return string(data)
}

// A callee that accepts the skills of its callers gets the skills an
// on-demand skill of its caller names (an orchestrator skill routing the
// developer to a standard): the sections addressed to another agent and the
// caller's natives aside.
func TestCalleeGetsSkillsNamedByCallerSkills(t *testing.T) {
	files := map[string]string{
		"agents/test/solo.md": delegatingAgentMD("solo", "primary", "", "a/routing, a/own", "helper", "Route le travail."),
		"skills/a/routing.md": skillMD("routing", "",
			"Le helper charge `a/std`, prompt [SKILL:a/tagged].\n\nVoir `a/own`.\n\n"+
				"## Pour le solo\n\nCharge `a/solo-only`.\n\n## Pour le helper\n\nCharge `a/addressed`."),
		"skills/a/own.md":       skillMD("own", "", "x"),
		"skills/a/std.md":       skillMD("std", "", "x"),
		"skills/a/tagged.md":    skillMD("tagged", "", "x"),
		"skills/a/solo-only.md": skillMD("solo-only", "", "x"),
		"skills/a/addressed.md": skillMD("addressed", "", "Puis `a/next`."),
		"skills/a/next.md":      skillMD("next", "", "x"),
	}
	files["agents/test/helper.md"] = delegatingAgentMD("helper", "subagent", "skills_from_callers: true\n", "", "", "Aide.")
	b, err := buildSolo(t, fakeHub(t, files))
	require.NoError(t, err)
	assert.Equal(t, []string{"addressed", "next", "std", "tagged"}, findAgent(b.Spec.Agents, "helper").Skills,
		"named by the caller's on-demand skill (its natives and the section for the caller aside), then transitively")
	assert.Equal(t, []string{"own", "routing", "solo-only", "std"}, findAgent(b.Spec.Agents, "solo").Skills)

	files["agents/test/helper.md"] = delegatingAgentMD("helper", "subagent", "", "", "", "Aide.")
	b, err = buildSolo(t, fakeHub(t, files))
	require.NoError(t, err)
	assert.Empty(t, findAgent(b.Spec.Agents, "helper").Skills, "without skills_from_callers")
}

// A skill named by an on-demand skill but left out of the session is named
// in a note closing that skill, where the model reads the mention; when it
// is absent for some readers of the skill only, the note goes in their
// prompt (the skill file is shared).
func TestAbsentSkillsNamedByOnDemandSkills(t *testing.T) {
	hub := fakeHub(t, map[string]string{
		"agents/test/solo.md":   delegatingAgentMD("solo", "primary", "", "a/lead, a/shared", "helper", "Travaille."),
		"agents/test/helper.md": delegatingAgentMD("helper", "subagent", "", "a/shared", "", "Aide."),
		"skills/a/lead.md":      skillMD("lead", "", "Si le plugin est chargé, charge `a/plugged`."),
		"skills/a/plugged.md":   skillMD("plugged", "plugin: rtk\n", "x"),
		"skills/a/shared.md":    skillMD("shared", "", "Charge `a/plugged`, puis `a/solo-only`."),
		"skills/a/solo-only.md": skillMD("solo-only", "when: agent=solo\n", "x"),
	})
	b, err := buildSolo(t, hub)
	require.NoError(t, err)
	solo, helper := findAgent(b.Spec.Agents, "solo"), findAgent(b.Spec.Agents, "helper")
	assert.Equal(t, []string{"lead", "shared", "solo-only"}, solo.Skills)
	assert.Equal(t, []string{"shared"}, helper.Skills)

	lead := shippedSkillFile(t, b, "lead")
	assert.Contains(t, lead, "charge `a/plugged`."+absentSkillsNote([]string{"plugged"}), "the note closes the skill")
	shared := shippedSkillFile(t, b, "shared")
	assert.Contains(t, shared, absentSkillsNote([]string{"plugged"}), "absent for both readers")
	assert.NotContains(t, shared, "`solo-only`.\n", "solo loads it: not in the shared note")

	assert.Equal(t, "", absentNote(solo.Body), "no absence of its own")
	assert.Contains(t, helper.Body, absentSkillsNote([]string{"solo-only"}), "its when: fails for the helper only: in the helper prompt")

	// A skill the prompt template names, absent: in the entry agent's note.
	b, err = buildSolo(t, hub, func(r *Request) { r.PromptTemplate = "Charge `a/plugged`." })
	require.NoError(t, err)
	assert.Contains(t, findAgent(b.Spec.Agents, "solo").Body, absentSkillsNote([]string{"plugged"}))
}
