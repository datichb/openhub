package evalrun

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// r9b (triage cases): a ticket of a case may set its priority.
func TestBeadCreateArgs(t *testing.T) {
	p := 0
	assert.Equal(t, []string{"create", "Crash", "--id", "ev-1", "--force", "-t", "bug", "-p", "0", "-l", "api,bug"},
		beadCreateArgs(BeadTicket{ID: "ev-1", Title: "Crash", Type: "bug", Priority: &p, Labels: []string{"api", "bug"}}))
	assert.Equal(t, []string{"create", "X", "--id", "ev-2", "--force"}, beadCreateArgs(BeadTicket{ID: "ev-2", Title: "X"}))
}
