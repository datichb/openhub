package cmd

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/teamstate"
)

// Acceptance (ADR-054, CLI): a team skill created from the template,
// edited, published, seen by another member; a hub agent replaced
// explicitly; the doctor reports a hand-edited brick.
func TestBrickCLI(t *testing.T) {
	env := setupWorkflowCLI(t, "alice", "bob")

	_, err := runWorkflowSub(t, brickNewCmd, "workflow", "x")
	require.Error(t, err)
	_, err = runWorkflowSub(t, brickNewCmd, "skill", "../x", "--no-edit")
	require.Error(t, err)
	out, err := runWorkflowSub(t, brickNewCmd, "skill", "team/hotfix-protocol", "--no-edit")
	require.NoError(t, err, out)
	assert.Contains(t, out, "skill:team/hotfix-protocol")
	_, err = runWorkflowSub(t, brickNewCmd, "skill", "team/hotfix-protocol", "--no-edit")
	require.Error(t, err, "already exists")
	_, err = runWorkflowSub(t, brickNewCmd, "agent", "reviewer", "--no-edit")
	require.Error(t, err, "hub brick without --extends")

	// An edit that breaks the description rule is refused (not interactive).
	script := filepath.Join(t.TempDir(), "editor.sh")
	require.NoError(t, os.WriteFile(script, []byte("#!/bin/sh\nsed -i.bak 's/^description:.*/description: Protocole de correctif/' \"$1\"\n"), 0o755))
	t.Setenv("VISUAL", script)
	_, err = runWorkflowSub(t, brickEditCmd, "team/hotfix-protocol")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "skill:team/hotfix-protocol")

	require.NoError(t, os.WriteFile(script, []byte("#!/bin/sh\nsed -i.bak 's/^description:.*/description: \"Utiliser quand un incident demande un correctif immédiat.\"/' \"$1\"\n"), 0o755))
	out, err = runWorkflowSub(t, brickEditCmd, "team/hotfix-protocol")
	require.NoError(t, err, out)

	_, err = runWorkflowSub(t, brickPublishCmd, "team/hotfix-protocol")
	require.Error(t, err, "message required")
	out, err = runWorkflowSub(t, brickPublishCmd, "team/hotfix-protocol", "-m", "Premier protocole")
	require.NoError(t, err, out)
	assert.Contains(t, out, "v1")

	out, err = runWorkflowSub(t, brickNewCmd, "agent", "reviewer", "--extends", "--no-edit")
	require.NoError(t, err, out)
	out, err = runWorkflowSub(t, brickPublishCmd, "agent:reviewer", "-m", "Revue d'équipe", "-y")
	require.NoError(t, err, out)
	assert.Contains(t, out, "hub:reviewer", "the replacement of a hub brick is reported")

	env.as(t, "bob")
	pull := exec.Command("git", "pull")
	pull.Dir = env.clones["bob"]
	require.NoError(t, pull.Run())
	out, err = runWorkflowSub(t, brickListCmd, "--origin")
	require.NoError(t, err, out)
	assert.Contains(t, out, "team/hotfix-protocol")
	assert.Contains(t, out, "catalog/skills/team/hotfix-protocol.md")
	assert.Contains(t, out, "hub:reviewer")
	out, err = runWorkflowSub(t, brickListCmd, "--agents")
	require.NoError(t, err, out)
	assert.NotContains(t, out, "team/hotfix-protocol")
	_, err = runWorkflowSub(t, brickListCmd, "--agents", "--skills")
	require.Error(t, err)

	out, err = runWorkflowSub(t, brickShowCmd, "team/hotfix-protocol")
	require.NoError(t, err, out)
	assert.Contains(t, out, "v1")
	out, err = runWorkflowSub(t, brickShowCmd, "skill:team/hotfix-protocol", "--raw")
	require.NoError(t, err, out)
	assert.Contains(t, out, "Utiliser quand un incident")
	out, err = runWorkflowSub(t, brickHistoryCmd, "team/hotfix-protocol")
	require.NoError(t, err, out)
	assert.Contains(t, out, "Premier protocole")
	_, err = runWorkflowSub(t, brickEditCmd, "agent:planner")
	require.Error(t, err, "hub bricks are read-only")

	// Hand-edited after publication: skipped, reported by the doctor.
	clone := env.clones["bob"]
	p := filepath.Join(clone, "catalog", "skills", "team", "hotfix-protocol.md")
	require.NoError(t, os.WriteFile(p, []byte("---\nname: hotfix-protocol\ndescription: x\n---\n"), 0o644))
	check, ok := brickCatalogCheck("acme", teamstate.NewRepo(env.bare, clone), findHubDir())
	require.True(t, ok)
	assert.False(t, check.OK)
	assert.Contains(t, check.Detail, "hotfix-protocol.md")
}
