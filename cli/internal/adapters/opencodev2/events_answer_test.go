package opencodev2

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/datichb/openhub/cli/internal/adapters"
)

func TestPermissionAnswer(t *testing.T) {
	data := func(reply string) map[string]any {
		return map[string]any{"sessionID": "ses_1", "requestID": "per_1", "reply": reply}
	}
	assert.Equal(t, &adapters.DecisionAnswer{ID: "per_1", Approved: true}, permissionAnswer("permission.replied", data("once")))
	assert.Equal(t, &adapters.DecisionAnswer{ID: "per_1", Approved: true}, permissionAnswer("permission.replied", data("always")))
	assert.Equal(t, &adapters.DecisionAnswer{ID: "per_1", Approved: false}, permissionAnswer("permission.replied", data("reject")))
	assert.Nil(t, permissionAnswer("form.replied", data("once")))
	assert.Nil(t, permissionAnswer("permission.replied", map[string]any{"reply": "once"}))
}
