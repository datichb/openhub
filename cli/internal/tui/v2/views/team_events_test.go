package views

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/teamstate"
)

// A41: complete phrases (the project of a finished session) and translated
// workflow events, read back from the team-state JSON.
func TestTeamEventPhrases(t *testing.T) {
	parse := func(s string) teamstate.Event {
		var e teamstate.Event
		require.NoError(t, json.Unmarshal([]byte(s), &e))
		return e
	}
	tests := []struct{ name, event, want string }{
		{"session on a project", `{"actor":"b","event":"session.complete","project":"openhub-projet-test-eb3f0f23"}`, "completed a session on openhub-projet-test-eb3f0f23"},
		{"session on a ticket", `{"actor":"b","event":"session.complete","project":"p","ticket":"pt-1"}`, "completed a session on p/pt-1"},
		{"session without project", `{"actor":"b","event":"session.complete"}`, "completed a session"},
		{"workflow published", `{"actor":"b","event":"workflow.published","project":"_team","data":{"workflow":"ticket-hotfix","version":2}}`, "published workflow ticket-hotfix (v2)"},
		{"workflow restored", `{"actor":"b","event":"workflow.restored","data":{"workflow":"ticket-hotfix","restored_from":1,"version":3}}`, "restored workflow ticket-hotfix v1 (new version v3)"},
		{"workflow archived", `{"actor":"b","event":"workflow.archived","project":"_team","data":{"workflow":"old"}}`, "archived workflow old"},
		{"claim transferred", `{"actor":"b","event":"claim.transferred","ticket":"pt-2","data":{"to":"alice"}}`, "transferred pt-2 to alice"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, formatEventDescription(parse(tc.event)), "event phrase")
		})
	}
	assert.NotEqual(t, "[white]·[-]", eventIcon(teamstate.EventWorkflowPublished), "workflow events have their own icon")
}
