package cmd

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/beads"
)

func TestBeadsTrackingResult(t *testing.T) {
	assert.Empty(t, beadsTrackingResult("api", nil, nil, nil, nil), "nothing to follow up")

	var epic beads.EpicProgress
	epic.Epic.ID = "pt-1"
	out := beadsTrackingResult("api", []beads.Orphan{{ID: "pt-2", Commit: "d0927d7"}}, []beads.Ticket{{ID: "pt-3"}}, []beads.EpicProgress{epic}, errors.New("boom"))
	require.Len(t, out, 4)
	for _, c := range out {
		assert.True(t, c.OK, "warnings only")
		assert.True(t, c.Warn)
		assert.Contains(t, c.Name, "api")
	}
	assert.Contains(t, out[0].Detail, "boom")
	assert.Contains(t, out[1].Detail, "pt-2 (d0927d7)")
	assert.Contains(t, out[2].Detail, "pt-3")
	assert.Contains(t, out[3].Detail, "pt-1")
	assert.Contains(t, shortList([]string{"a", "b", "c", "d", "e", "f", "g"}), "a, b, c, d, e (+2")
}
