//go:build integration

package opencodev2

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/adapters"
	"github.com/datichb/openhub/cli/internal/sessionspec"
)

// A session without prompt is idle: WaitIdle returns at once, the assistant
// wrote nothing, the results carry the session model.
func TestContractHeadlessWait(t *testing.T) {
	cfg := `{"agents":{"pinger":{"mode":"primary","description":"p"}}}`
	s := startLiveServer(t, cfg, "pinger")
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	ad := &Adapter{}
	h := adapters.ServerHandle{URL: s.URL, Password: s.Password, PID: s.PID}
	sid := sessionspec.NewSessionID()
	model := sessionspec.ParseModelRef("amazon-bedrock/eu.anthropic.claude-haiku-4-5-20251001-v1:0")
	require.NoError(t, ad.CreateSession(ctx, h, sessionspec.SessionSpec{SessionID: sid, EntryAgent: "pinger", Location: s.project, Model: &model}))

	require.NoError(t, ad.WaitIdle(ctx, h, sid))
	text, err := ad.AssistantText(ctx, h, sid)
	require.NoError(t, err)
	assert.Empty(t, text)
	res, err := ad.Results(ctx, h, sid)
	require.NoError(t, err)
	assert.Contains(t, res.Model, "claude-haiku")
}
