package teamstate

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBrickNames(t *testing.T) {
	t.Parallel()
	for _, ok := range []string{"a", "team/hotfix-protocol", "a/b/c/d"} {
		assert.NoError(t, ValidBrickID(BrickKindSkill, ok), ok)
	}
	for _, bad := range []string{"", "../x", "a/../b", "a//b", "/a", "templates/x", "A", "a/b/c/d/e", "a.md", "a\\b"} {
		assert.ErrorIs(t, ValidBrickID(BrickKindSkill, bad), ErrInvalidBrickID, bad)
	}
	assert.NoError(t, ValidBrickID(BrickKindAgent, "hotfixer"))
	assert.ErrorIs(t, ValidBrickID(BrickKindAgent, "team/hotfixer"), ErrInvalidBrickID)
	assert.ErrorIs(t, ValidBrickID("workflow", "x"), ErrInvalidBrickID)

	rel, err := DraftBrickRel("alice", BrickKindAgent, "hotfixer", "incident")
	require.NoError(t, err)
	assert.Equal(t, "catalog/drafts/alice/agents/incident/hotfixer.md", rel)
	_, err = DraftBrickRel("../bob", BrickKindAgent, "hotfixer", "team")
	assert.Error(t, err)
	_, err = PublishedBrickRel(BrickKindAgent, "hotfixer", "..")
	assert.ErrorIs(t, err, ErrInvalidBrickID)

	// A lock entry only points at the published file of its brick.
	assert.NoError(t, BrickLockEntry{Path: "catalog/agents/team/hotfixer.md"}.CheckPath(BrickKindAgent, "hotfixer"))
	assert.NoError(t, BrickLockEntry{Path: "catalog/skills/team/x.md"}.CheckPath(BrickKindSkill, "team/x"))
	for _, p := range []string{"catalog/agents/team/other.md", "catalog/agents/../../etc/passwd", "/etc/passwd", "catalog/agents/team/../x/hotfixer.md", "workflows.lock"} {
		assert.Error(t, BrickLockEntry{Path: p}.CheckPath(BrickKindAgent, "hotfixer"), p)
	}
	assert.Error(t, BrickLockEntry{Path: "catalog/skills/team/y.md"}.CheckPath(BrickKindSkill, "team/x"))
}
