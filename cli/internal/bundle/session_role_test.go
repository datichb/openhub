package bundle

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func agentBody(t *testing.T, b *Bundle, id string) string {
	t.Helper()
	for _, a := range b.Spec.Agents {
		if a.ID == id {
			return a.Body
		}
	}
	t.Fatalf("agent %s not in the bundle", id)
	return ""
}

// C0.1: an agent learns its role from the workflow graph, not from a tag its
// caller would have to inject. A subagent never takes the standalone path,
// where its `question` calls would stay invisible to the user.
func TestSessionRoleFromGraph(t *testing.T) {
	feature := shippedBundle(t, "feature")
	assert.Contains(t, agentBody(t, feature, "orchestrator"), "Tu es l'**agent d'entrée** : tu échanges directement")
	for _, id := range []string{"planner", "pathfinder", "designer", "orchestrator-dev", "reviewer"} {
		body := agentBody(t, feature, id)
		assert.Contains(t, body, "Tu es **sous-agent**", id)
		assert.Contains(t, body, "`orchestrator`", "%s: its caller is named", id)
	}

	review := shippedBundle(t, "review")
	assert.Contains(t, agentBody(t, review, "reviewer"), "Tu es l'**agent d'entrée** et tu peux aussi être invoqué via l'outil `task` par `reviewer`")

	quick := shippedBundle(t, "quick")
	assert.Contains(t, agentBody(t, quick, "developer"), "Tu es l'**agent d'entrée** : tu échanges directement")
}

func TestSessionRoleNote(t *testing.T) {
	assert.Contains(t, sessionRoleNote("a", "a", nil), "agent d'entrée** :")
	assert.Contains(t, sessionRoleNote("a", "a", []string{"b"}), "par `b`. Si ton prompt vient")
	assert.Contains(t, sessionRoleNote("a", "x", []string{"b", "c"}), "**sous-agent** : tu es invoqué via l'outil `task` par `b`, `c`.")
	assert.Contains(t, sessionRoleNote("a", "x", nil), "lancé directement par l'utilisateur")
}

// C0.1: no shipped text decides the mode from a `[SKILL:…]` tag any more.
func TestNoModeDetectionFromSkillTag(t *testing.T) {
	hub := repoHub(t)
	for _, dir := range []string{"agents", "skills"} {
		require.NoError(t, filepath.WalkDir(filepath.Join(hub, dir), func(path string, d os.DirEntry, err error) error {
			if err != nil || d.IsDir() || !strings.HasSuffix(path, ".md") {
				return err
			}
			data, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			text := string(data)
			for _, bad := range []string{"déterminé par le tag `[SKILL:", "aucun `[SKILL:...]` injecté", "injecte `[SKILL:"} {
				assert.NotContains(t, text, bad, path)
			}
			return nil
		}))
	}
}
