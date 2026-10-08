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
	cases := map[string]string{
		`{"actor":"b","event":"session.complete","project":"openhub-projet-test-eb3f0f23"}`:                            "completed a session on openhub-projet-test-eb3f0f23",
		`{"actor":"b","event":"session.complete","project":"p","ticket":"pt-1"}`:                                       "completed a session on p/pt-1",
		`{"actor":"b","event":"session.complete"}`:                                                                     "completed a session",
		`{"actor":"b","event":"workflow.published","project":"_team","data":{"workflow":"ticket-hotfix","version":2}}`: "published workflow ticket-hotfix (v2)",
		`{"actor":"b","event":"workflow.restored","data":{"workflow":"ticket-hotfix","restored_from":1,"version":3}}`:  "restored workflow ticket-hotfix v1 (new version v3)",
		`{"actor":"b","event":"workflow.archived","project":"_team","data":{"workflow":"old"}}`:                        "archived workflow old",
		`{"actor":"b","event":"claim.transferred","ticket":"pt-2","data":{"to":"alice"}}`:                              "transferred pt-2 to alice",
	}
	for in, want := range cases {
		assert.Equal(t, want, formatEventDescription(parse(in)), in)
	}
	assert.NotEqual(t, "[white]·[-]", eventIcon(teamstate.EventWorkflowPublished))
}
