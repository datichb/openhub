package beads

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/datichb/openhub/cli/internal/beads/bdschema"
)

// Every field oh decodes from bd is declared in the bd output contract.
func TestDecodersFollowBdSchema(t *testing.T) {
	assert.Empty(t, bdschema.Uncovered(bdIssue{}, "issue"))
	assert.Empty(t, bdschema.Uncovered(TicketDetail{}, "issue"))
}
