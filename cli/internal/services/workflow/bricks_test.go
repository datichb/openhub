package workflow

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/bundle"
	wf "github.com/datichb/openhub/cli/internal/workflow"
)

const teamAgent = `---
id: hotfixer
description: Corrige un incident de production
mode: subagent
skills: [team/hotfix-protocol]
permission:
  edit: allow
  bash: deny
---
Hotfixer
`

const teamSkill = `---
name: hotfix-protocol
description: Protocole de correctif
annexes: [templates/hotfix-report.md]
---
Protocole
`

func writeCatalog(t *testing.T, root string, files map[string]string) {
	t.Helper()
	for rel, content := range files {
		p := filepath.Join(root, "catalog", filepath.FromSlash(rel))
		require.NoError(t, os.MkdirAll(filepath.Dir(p), 0o755))
		require.NoError(t, os.WriteFile(p, []byte(content), 0o644))
	}
}

// Acceptance (P2-T07): team bricks are used by the workflows; an id already
// used by the hub is refused unless the brick extends it explicitly.
func TestTeamBricks(t *testing.T) {
	env := newTeamEnv(t, "alice")
	svc := env.service(t, "alice")
	svc.BricksCacheDir = t.TempDir()
	repo := env.clones["alice"]
	ctx := context.Background()

	writeCatalog(t, repo.Path(), map[string]string{
		"agents/team/hotfixer.md":           teamAgent,
		"skills/team/hotfix-protocol.md":    teamSkill,
		"skills/templates/hotfix-report.md": "# Rapport",
		// Same id as a hub agent, without extends: refused.
		"agents/team/reviewer.md": "---\nid: reviewer\nmode: primary\npermission:\n  edit: allow\n---\nTeam reviewer\n",
		// Explicit replacement of a hub agent.
		"agents/team/documentarian.md": "---\nid: documentarian\nextends: hub:documentarian\nmode: subagent\npermission:\n  edit: allow\n---\nTeam documentarian\n",
		// Same skill name as a hub skill, wrong extends: refused.
		"skills/team/context-mode-usage.md": "---\nname: context-mode-usage\nextends: hub:nope\ndescription: x\n---\nx\n",
		// extends on a new brick: refused.
		"agents/team/ghost.md": "---\nid: ghost\nextends: hub:ghost\nmode: subagent\n---\nGhost\n",
	})

	b, err := svc.Bricks(ctx, Context{})
	require.NoError(t, err)
	assert.Equal(t, []string{"agent:documentarian", "agent:hotfixer", "skill:team/hotfix-protocol"}, b.Team)
	codes := map[string]string{}
	for _, d := range b.Diagnostics {
		codes[filepath.Base(d.Source)] = d.Code
		assert.Equal(t, wf.SeverityWarning, d.Severity)
		assert.NotEmpty(t, d.Message)
	}
	assert.Equal(t, map[string]string{
		"reviewer.md": DiagBrickCollision, "context-mode-usage.md": DiagBrickCollision, "ghost.md": DiagBrickBadExtends,
	}, codes)

	data, err := os.ReadFile(filepath.Join(b.Dir, "agents", "documentation", "documentarian.md"))
	require.NoError(t, err)
	assert.Contains(t, string(data), "Team documentarian", "explicit extends replaces the hub agent")
	assert.NotContains(t, string(data), "extends:")
	data, err = os.ReadFile(filepath.Join(b.Dir, "agents", "quality", "reviewer.md"))
	require.NoError(t, err)
	assert.NotContains(t, string(data), "Team reviewer", "a colliding brick never replaces the hub one")
	assert.FileExists(t, filepath.Join(b.Dir, "skills", "templates", "hotfix-report.md"))

	// A team workflow uses the team agent; validated against the merged bricks.
	saveDraft(t, svc, wf.LayerTeam, `apiVersion: oh/v1
kind: Workflow
id: hotfix
risk: write
entry: { agent: orchestrator-dev }
agents:
  orchestrator-dev: { role: workflow, calls: [hotfixer] }
  hotfixer: { role: workflow }
`)
	pub, err := svc.Publish(ctx, Context{}, "hotfix", "")
	require.NoError(t, err)
	assert.Equal(t, []string{"agent:hotfixer", "skill:team/hotfix-protocol"}, pub.Impact.NewBricks, "« nouvelle brique »")

	res, err := svc.Resolve(ctx, Context{}, "hotfix", ResolveOpts{})
	require.NoError(t, err)
	assert.Equal(t, b.Dir, res.BricksDir(), "bundles are built from the merged bricks")

	integrity, err := svc.Integrity(ctx, Context{})
	require.NoError(t, err)
	assert.Len(t, integrity, 3, "refused bricks are reported with the integrity warnings")

	// Same content: the merged directory is reused.
	b2, err := svc.Bricks(ctx, Context{})
	require.NoError(t, err)
	assert.Equal(t, b.Dir, b2.Dir)

	// Without a team catalogue, the hub is used as is.
	plain := testService(t)
	res, err = plain.Resolve(ctx, Context{}, "ticket", ResolveOpts{})
	require.NoError(t, err)
	assert.Empty(t, res.BricksDir())
}

func TestStripExtends(t *testing.T) {
	got := string(stripExtends([]byte("---\nid: a\nextends: hub:a\nmode: subagent\n---\nBody\n")))
	assert.Equal(t, "---\nid: a\nmode: subagent\n---\nBody\n", got)
	assert.Equal(t, "hub:a", frontmatterExtends([]byte("---\nextends: hub:a\n---\n")))
	assert.Empty(t, frontmatterExtends([]byte("no frontmatter")))
}

func TestTeamBricksInBundle(t *testing.T) {
	env := newTeamEnv(t, "alice")
	svc := env.service(t, "alice")
	svc.BricksCacheDir = t.TempDir()
	ctx := context.Background()
	writeCatalog(t, env.clones["alice"].Path(), map[string]string{
		"agents/team/hotfixer.md":           teamAgent,
		"skills/team/hotfix-protocol.md":    teamSkill,
		"skills/templates/hotfix-report.md": "# Rapport",
	})
	saveDraft(t, svc, wf.LayerTeam, "apiVersion: oh/v1\nkind: Workflow\nid: hotfix\nrisk: write\nentry: { agent: orchestrator-dev }\nagents:\n  orchestrator-dev: { role: workflow, calls: [hotfixer] }\n  hotfixer: { role: workflow }\n")
	_, err := svc.Publish(ctx, Context{}, "hotfix", "")
	require.NoError(t, err)
	res, err := svc.Resolve(ctx, Context{}, "hotfix", ResolveOpts{})
	require.NoError(t, err)

	b, err := bundle.Build(bundle.Request{HubDir: res.BricksDir(), OutDir: t.TempDir(), Spec: res.Spec, EntryAgent: res.Spec.EntryAgent()})
	require.NoError(t, err)
	var ids []string
	for _, a := range b.Spec.Agents {
		ids = append(ids, a.ID)
	}
	assert.Contains(t, ids, "hotfixer")
	// The skill is inlined in the agent (frontmatter skills:), its annex ships next to it.
	assert.FileExists(t, filepath.Join(b.Dir, "skills", "hotfix-protocol", "templates", "hotfix-report.md"))
}
