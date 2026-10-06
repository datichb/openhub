package workflow

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBrickCatalog(t *testing.T) {
	list, err := testService(t).BrickCatalog(context.Background(), Context{})
	require.NoError(t, err)
	byID := map[string]BrickEntry{}
	kinds := map[BrickKind]int{}
	for _, e := range list {
		byID[string(e.Kind)+":"+e.ID] = e
		kinds[e.Kind]++
	}
	assert.Greater(t, kinds[BrickAgent], 10)
	assert.Greater(t, kinds[BrickSkill], 50)
	assert.Equal(t, BrickAgent, list[0].Kind, "agents first")

	dev := byID["agent:developer"]
	assert.Equal(t, OriginHub, dev.Origin)
	assert.Equal(t, "developer", dev.Family)
	assert.Positive(t, dev.Tokens)
	assert.NotEmpty(t, dev.Skills)
	assert.Contains(t, dev.Workflows, "ticket", "member of the test workflow ticket")

	var skill BrickEntry
	for _, ref := range dev.Skills {
		if e, ok := byID["skill:"+ref]; ok {
			skill = e
			break
		}
	}
	require.NotEmpty(t, skill.ID, "a skill of the developer is in the catalogue")
	assert.Contains(t, skill.Agents, "developer")
	assert.Contains(t, skill.Workflows, "ticket", "shipped through the developer")
	_, annex := byID["skill:templates/x"]
	assert.False(t, annex)
	for id := range byID {
		assert.NotContains(t, id, "skill:templates/", "annexes are not skills")
	}
}
