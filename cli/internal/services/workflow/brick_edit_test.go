package workflow

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/gitutil/gittest"
	"github.com/datichb/openhub/cli/internal/teamstate"
	wf "github.com/datichb/openhub/cli/internal/workflow"
)

func brickService(t *testing.T, env *teamEnv, member string) *Service {
	t.Helper()
	svc := env.service(t, member)
	svc.BricksCacheDir = t.TempDir()
	return svc
}

func codesOf(ds wf.Diagnostics) []string {
	var out []string
	for _, d := range ds {
		out = append(out, d.Code)
	}
	return out
}

const brickSkill = `---
name: hotfix-protocol
description: "Utiliser quand un incident de production demande un correctif immédiat."
---

# Hotfix
`

// Acceptance (ADR-054): a team brick goes from a validated draft to a
// sealed publication, its previous version kept in history.
func TestBrickLifecycle(t *testing.T) {
	env := newTeamEnv(t, "alice")
	svc := brickService(t, env, "alice")
	ctx := context.Background()
	root := env.clones["alice"].Path()
	ref := BrickRef{Kind: BrickSkill, ID: "team/hotfix-protocol"}

	tmpl, err := svc.NewBrickText(ctx, Context{}, NewBrick{Ref: ref})
	require.NoError(t, err)
	diags, err := svc.CheckBrick(ctx, Context{}, BrickInput{Ref: ref, Data: tmpl.Data})
	require.NoError(t, err)
	assert.False(t, diags.HasErrors(), "the skill template is valid: %v", diags)
	agentTmpl, err := svc.NewBrickText(ctx, Context{}, NewBrick{Ref: BrickRef{Kind: BrickAgent, ID: "hotfixer"}})
	require.NoError(t, err)
	diags, err = svc.CheckBrick(ctx, Context{}, BrickInput{Ref: BrickRef{Kind: BrickAgent, ID: "hotfixer"}, Data: agentTmpl.Data})
	require.NoError(t, err)
	assert.False(t, diags.HasErrors(), "the agent template is valid: %v", diags)

	d, err := svc.SaveBrickDraft(ctx, Context{}, BrickInput{Ref: ref, Data: []byte(brickSkill)})
	require.NoError(t, err)
	assert.True(t, d.Pushed)
	assert.FileExists(t, filepath.Join(root, "catalog", "drafts", "alice", "skills", "team", "hotfix-protocol.md"))

	b, err := svc.Bricks(ctx, Context{})
	require.NoError(t, err)
	assert.Empty(t, b.Team, "a draft is never loaded")

	p, err := svc.PreviewBrick(ctx, Context{}, "team/hotfix-protocol")
	require.NoError(t, err)
	assert.Equal(t, 1, p.Next)
	assert.Empty(t, p.Unsealed)

	pub, err := svc.PublishBrick(ctx, Context{}, "skill:team/hotfix-protocol", "Premier protocole")
	require.NoError(t, err)
	assert.Equal(t, 1, pub.Version)
	assert.NoFileExists(t, filepath.Join(root, "catalog", "drafts", "alice", "skills", "team", "hotfix-protocol.md"))
	lock, sealed, err := teamstate.ReadBricksLock(root)
	require.NoError(t, err)
	assert.True(t, sealed)
	e, ok := lock.Get("skill", "team/hotfix-protocol")
	require.True(t, ok)
	assert.Equal(t, "catalog/skills/team/hotfix-protocol.md", e.Path)
	assert.Equal(t, "alice", e.PublishedBy)

	b, err = svc.Bricks(ctx, Context{})
	require.NoError(t, err)
	assert.Equal(t, []string{"skill:team/hotfix-protocol"}, b.Team)
	assert.True(t, b.Sealed)
	assert.Empty(t, b.Diagnostics)

	// Edit: the published text, a new draft, v2; v1 kept in history.
	txt, err := svc.EditableBrickText(ctx, Context{}, "team/hotfix-protocol")
	require.NoError(t, err)
	assert.Equal(t, "team", txt.Origin)
	_, err = svc.SaveBrickDraft(ctx, Context{}, BrickInput{Ref: ref, Data: append(txt.Data, []byte("\nÉtape 2.\n")...)})
	require.NoError(t, err)
	pub, err = svc.PublishBrick(ctx, Context{}, "team/hotfix-protocol", "Étape 2")
	require.NoError(t, err)
	assert.Equal(t, 2, pub.Version)
	versions, err := svc.BrickHistory(ctx, Context{}, "team/hotfix-protocol")
	require.NoError(t, err)
	require.Len(t, versions, 2)
	assert.True(t, versions[0].Current)
	assert.Equal(t, 1, versions[1].Version)
	assert.Equal(t, "Premier protocole", versions[1].Entry.Message)
	old, err := os.ReadFile(versions[1].Path)
	require.NoError(t, err)
	assert.Equal(t, brickSkill, string(old))

	ws, err := svc.BrickWorkspace(ctx, Context{})
	require.NoError(t, err)
	var found bool
	for _, e := range ws.Bricks {
		if e.ID == "team/hotfix-protocol" {
			found = true
			assert.Equal(t, OriginTeam, e.Origin)
			assert.Equal(t, 2, e.Version)
			assert.Equal(t, "catalog/skills/team/hotfix-protocol.md", e.Source)
		}
	}
	assert.True(t, found)

	// The hub is read-only.
	_, err = svc.EditableBrickText(ctx, Context{}, "agent:reviewer")
	assert.ErrorIs(t, err, ErrHubReadOnly)
}

// The integrity of a sealed catalogue: a brick edited by hand or committed
// without oh is not loaded.
func TestBrickIntegrity(t *testing.T) {
	env := newTeamEnv(t, "alice")
	svc := brickService(t, env, "alice")
	ctx := context.Background()
	root := env.clones["alice"].Path()
	_, err := svc.SaveBrickDraft(ctx, Context{}, BrickInput{Ref: BrickRef{Kind: BrickSkill, ID: "team/hotfix-protocol"}, Data: []byte(brickSkill)})
	require.NoError(t, err)
	_, err = svc.PublishBrick(ctx, Context{}, "team/hotfix-protocol", "")
	require.NoError(t, err)

	p := filepath.Join(root, "catalog", "skills", "team", "hotfix-protocol.md")
	require.NoError(t, os.WriteFile(p, []byte(brickSkill+"\nAjout à la main.\n"), 0o644))
	writeCatalog(t, root, map[string]string{"skills/team/by-hand.md": strings.ReplaceAll(brickSkill, "hotfix-protocol", "by-hand")})

	b, err := svc.Bricks(ctx, Context{})
	require.NoError(t, err)
	assert.Empty(t, b.Team)
	assert.ElementsMatch(t, []string{DiagBrickHashMismatch, DiagBrickUnlocked}, codesOf(b.Diagnostics))
	integrity, err := svc.Integrity(ctx, Context{})
	require.NoError(t, err)
	assert.Len(t, integrity, 2)

	require.NoError(t, os.Remove(p))
	b, err = svc.Bricks(ctx, Context{})
	require.NoError(t, err)
	assert.ElementsMatch(t, []string{DiagBrickLockOrphan, DiagBrickUnlocked}, codesOf(b.Diagnostics))

	// A link never reaches a bundle.
	outside := filepath.Join(t.TempDir(), "secret.md")
	require.NoError(t, os.WriteFile(outside, []byte(brickSkill), 0o644))
	require.NoError(t, os.Symlink(outside, filepath.Join(root, "catalog", "skills", "team", "link.md")))
	b, err = svc.Bricks(ctx, Context{})
	require.NoError(t, err)
	assert.Contains(t, codesOf(b.Diagnostics), DiagBrickUnsafePath)
}

// A catalogue committed by hand before ADR-054 keeps loading; it is sealed
// as a whole (adopt) before the first publication.
func TestBrickAdoption(t *testing.T) {
	env := newTeamEnv(t, "alice")
	svc := brickService(t, env, "alice")
	ctx := context.Background()
	root := env.clones["alice"].Path()
	writeCatalog(t, root, map[string]string{"agents/team/hotfixer.md": teamAgent, "skills/team/hotfix-protocol.md": teamSkill,
		"skills/templates/hotfix-report.md": "# Rapport"})
	gittest.Run(t, root, "add", ".")
	gittest.Run(t, root, "commit", "-m", "bricks by hand")

	b, err := svc.Bricks(ctx, Context{})
	require.NoError(t, err)
	assert.False(t, b.Sealed)
	assert.Equal(t, []string{"agent:hotfixer", "skill:team/hotfix-protocol"}, b.Unsealed)
	assert.Len(t, b.Team, 2, "loaded while the catalogue was never sealed")

	other := BrickRef{Kind: BrickSkill, ID: "team/other-protocol"}
	_, err = svc.SaveBrickDraft(ctx, Context{}, BrickInput{Ref: other, Data: []byte(strings.ReplaceAll(brickSkill, "hotfix-protocol", "other-protocol"))})
	require.NoError(t, err)
	_, err = svc.PublishBrick(ctx, Context{}, "team/other-protocol", "")
	assert.ErrorIs(t, err, ErrBricksUnsealed)

	pub, err := svc.AdoptBricks(ctx, Context{}, "Reprise du catalogue")
	require.NoError(t, err)
	assert.Equal(t, []string{"agent:hotfixer", "skill:team/hotfix-protocol"}, pub.Adopted)
	_, err = svc.AdoptBricks(ctx, Context{}, "")
	assert.ErrorIs(t, err, ErrBricksSealed)

	pub, err = svc.PublishBrick(ctx, Context{}, "team/other-protocol", "")
	require.NoError(t, err)
	assert.Equal(t, 1, pub.Version)
	b, err = svc.Bricks(ctx, Context{})
	require.NoError(t, err)
	assert.Len(t, b.Team, 3)
	assert.Empty(t, b.Diagnostics)
}

// Strict rules of a brick written with oh brick, and the explicit
// replacement of a hub brick.
func TestBrickChecks(t *testing.T) {
	env := newTeamEnv(t, "alice")
	svc := brickService(t, env, "alice")
	ctx := context.Background()
	check := func(kind BrickKind, id, text string) wf.Diagnostics {
		t.Helper()
		ds, err := svc.CheckBrick(ctx, Context{}, BrickInput{Ref: BrickRef{Kind: kind, ID: id}, Data: []byte(text)})
		require.NoError(t, err)
		return ds.Errors()
	}
	assert.Contains(t, codesOf(check(BrickSkill, "team/x", "---\nname: x\ndescription: Protocole x\n---\nx\n")), "description_not_trigger")
	assert.Contains(t, codesOf(check(BrickSkill, "team/x", "---\nname: x\ndescription: \"Utiliser "+strings.Repeat("a", 200)+"\"\n---\nx\n")), "long_description")
	assert.Contains(t, codesOf(check(BrickSkill, "team/x", "---\nname: y\ndescription: Utiliser quand x\n---\nx\n")), "name_mismatch")
	assert.Contains(t, codesOf(check(BrickSkill, "team/x", "---\nname: x\ndescription: Utiliser quand x\nrequires: [team/nope]\n---\nx\n")), "missing_require")
	assert.Contains(t, codesOf(check(BrickSkill, "team/x", "pas de frontmatter\n")), DiagBrickNoFrontmatter)
	assert.Contains(t, codesOf(check(BrickAgent, "a", "---\nid: b\nmode: subagent\n---\nx\n")), DiagBrickIDMismatch)
	assert.Contains(t, codesOf(check(BrickAgent, "a", "---\nid: a\nmode: subagent\n---\nx\n")), DiagBrickNoDescription)
	assert.Contains(t, codesOf(check(BrickAgent, "a", "---\nid: a\ndescription: x\nmode: subagent\ncolor: red\n---\nx\n")), DiagBrickUnknownKey)
	assert.Contains(t, codesOf(check(BrickAgent, "a", "---\nid: a\ndescription: x\nmode: subagent\nskills: [team/nope]\n---\nx\n")), "missing_agent_skill")
	assert.Contains(t, codesOf(check(BrickAgent, "a", "---\nid: a\ndescription: x\nmode: subagent\npermission_base: nope\n---\nx\n")), DiagBrickPermBase)

	// Same id as a hub agent: refused, unless the brick says it replaces it.
	_, err := svc.NewBrickText(ctx, Context{}, NewBrick{Ref: BrickRef{Kind: BrickAgent, ID: "reviewer"}})
	assert.ErrorIs(t, err, ErrBrickInHub)
	assert.Contains(t, codesOf(check(BrickAgent, "reviewer", "---\nid: reviewer\ndescription: x\nmode: primary\n---\nx\n")), DiagBrickCollision)
	_, err = svc.NewBrickText(ctx, Context{}, NewBrick{Ref: BrickRef{Kind: BrickAgent, ID: "ghost"}, Extends: true})
	assert.ErrorIs(t, err, ErrBrickNotInHub)
	txt, err := svc.NewBrickText(ctx, Context{}, NewBrick{Ref: BrickRef{Kind: BrickAgent, ID: "reviewer"}, Extends: true})
	require.NoError(t, err)
	assert.True(t, strings.HasPrefix(string(txt.Data), "---\nextends: hub:reviewer\n"))
	_, err = svc.SaveBrickDraft(ctx, Context{}, BrickInput{Ref: BrickRef{Kind: BrickAgent, ID: "reviewer"}, Data: txt.Data})
	require.NoError(t, err)
	p, err := svc.PreviewBrick(ctx, Context{}, "reviewer")
	require.NoError(t, err)
	assert.Equal(t, "hub:reviewer", p.Shadows, "the replacement of a hub brick is reported")
	pub, err := svc.PublishBrick(ctx, Context{}, "agent:reviewer", "Revue d'équipe")
	require.NoError(t, err)
	assert.Equal(t, "hub:reviewer", pub.Shadows)

	// Names never leave the catalogue.
	for _, id := range []string{"../x", "a/../../b", "templates/x", "A", "/abs", "a//b"} {
		_, err := svc.NewBrickText(ctx, Context{}, NewBrick{Ref: BrickRef{Kind: BrickSkill, ID: id}})
		assert.ErrorIs(t, err, teamstate.ErrInvalidBrickID, id)
	}
	_, err = svc.NewBrickText(ctx, Context{}, NewBrick{Ref: BrickRef{Kind: BrickAgent, ID: "x"}, Family: "../.."})
	assert.ErrorIs(t, err, teamstate.ErrInvalidBrickID)
	_, err = svc.SaveBrickDraft(ctx, Context{}, BrickInput{Ref: BrickRef{Kind: BrickAgent, ID: "x/y"}, Data: []byte(teamAgent)})
	assert.ErrorIs(t, err, teamstate.ErrInvalidBrickID)
}

// Publication follows the governance of the workflows.
func TestBrickGovernance(t *testing.T) {
	env := newTeamEnv(t, "alice")
	svc := brickService(t, env, "alice")
	ctx := context.Background()
	_, err := svc.SaveBrickDraft(ctx, Context{}, BrickInput{Ref: BrickRef{Kind: BrickSkill, ID: "team/hotfix-protocol"}, Data: []byte(brickSkill)})
	require.NoError(t, err)
	mallory := brickService(t, env, "alice")
	mallory.TeamState = func(context.Context, Context) (*TeamState, error) {
		return &TeamState{Repo: env.clones["alice"], Member: "mallory"}, nil
	}
	_, err = mallory.SaveBrickDraft(ctx, Context{}, BrickInput{Ref: BrickRef{Kind: BrickSkill, ID: "team/hotfix-protocol"}, Data: []byte(brickSkill)})
	require.NoError(t, err)
	_, err = mallory.PublishBrick(ctx, Context{}, "team/hotfix-protocol", "")
	assert.ErrorIs(t, err, teamstate.ErrNotMember)
	_, err = svc.PublishBrick(ctx, Context{}, "team/nope", "")
	assert.ErrorIs(t, err, ErrUnknownBrick)
}
