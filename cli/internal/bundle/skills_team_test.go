package bundle

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCheckTeamSkills(t *testing.T) {
	catalog := fakeHub(t, map[string]string{
		"skills/team/ok.md":            skillMD("ok", "license: MIT\nrequires: [team/other]\n", "x"),
		"skills/team/override.md":      skillMD("override", "extends: hub:a/override\n", "x"),
		"skills/team/unknown.md":       skillMD("unknown", "scope: team\nowner: alice\n", "x"),
		"skills/team/broken.md":        "---\nname: [\n---\nx\n",
		"skills/templates/annex.md":    "---\nscope: annex\n---\n",
		"skills/team/notes.txt":        "scope: not a skill\n",
		"agents/team/agent.md":         "---\nid: agent\nscope: agent\n---\n",
		"skills/team/.hidden.md":       "---\nscope: hidden\n---\n",
		"skills/team/nofrontmatter.md": "# body only\n",
		"skills/team/long.md":          "---\nname: long\ndescription: Utiliser " + strings.Repeat("x", 300) + "\n---\nx\n",
		"skills/team/huge.md":          "---\nname: huge\ndescription: Utiliser " + strings.Repeat("x", 1100) + "\n---\nx\n",
		"skills/team/vague.md":         "---\nname: vague\ndescription: Conventions de l'équipe\n---\nx\n",
	})
	problems, err := CheckTeamSkills(catalog)
	require.NoError(t, err)

	got := map[string]bool{}
	for _, p := range problems {
		if p.Kind == SkillInvalidFrontmatter {
			got[string(p.Kind)+" "+p.Ref] = p.Error
			continue
		}
		got[string(p.Kind)+" "+p.Ref+" "+p.Detail] = p.Error
	}
	assert.Equal(t, map[string]bool{
		"unknown_key team:team/unknown scope":      true,
		"unknown_key team:team/unknown owner":      true,
		"invalid_frontmatter team:team/broken":     true,
		"long_description team:team/long 309":      false,
		"long_description team:team/huge 1109":     true,
		"description_not_trigger team:team/vague ": false,
	}, got)
}

func TestCheckTeamSkills_NoCatalog(t *testing.T) {
	problems, err := CheckTeamSkills(filepath.Join(t.TempDir(), "missing"))
	require.NoError(t, err)
	assert.Empty(t, problems)
}
