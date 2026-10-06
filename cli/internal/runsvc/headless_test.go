package runsvc

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/adapters"
	"github.com/datichb/openhub/cli/internal/domain"
)

// waitAdapter adds headless waits to the fake adapter.
type waitAdapter struct {
	*rtAdapter
	pending bool
	block   chan struct{} // WaitIdle blocks until closed (or ctx done)
}

func (a *waitAdapter) WaitIdle(ctx context.Context, _ adapters.ServerHandle, _ string) error {
	select {
	case <-a.block:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}
func (a *waitAdapter) AssistantText(context.Context, adapters.ServerHandle, string) (string, error) {
	return "# Brief enrichi", nil
}
func (a *waitAdapter) Pending(context.Context, adapters.ServerHandle, string) ([]adapters.PendingDecision, error) {
	if a.pending {
		return []adapters.PendingDecision{{ID: "per_1", Kind: adapters.DecisionPermission}}, nil
	}
	return nil, nil
}
func (a *waitAdapter) Results(context.Context, adapters.ServerHandle, string) (adapters.SessionResult, error) {
	return adapters.SessionResult{Cost: 0.02, Model: "amazon-bedrock/claude"}, nil
}

func headlessFixture(t *testing.T, pending bool) (*rtFixture, *waitAdapter, string) {
	t.Helper()
	f, _ := runFixture(t)
	wa := &waitAdapter{rtAdapter: f.ad, pending: pending, block: make(chan struct{})}
	f.svc.Adapter = wa
	plan, err := f.svc.Plan(context.Background(), f.run("read", LocationChoice{Kind: LocationBase}))
	require.NoError(t, err)
	res, err := f.svc.Start(context.Background(), plan)
	require.NoError(t, err)
	return f, wa, res[0].SessionID
}

func TestAwaitTurn(t *testing.T) {
	f, wa, sid := headlessFixture(t, false)
	close(wa.block)
	out, err := f.svc.AwaitTurn(context.Background(), sid)
	require.NoError(t, err)
	assert.Equal(t, "# Brief enrichi", out.Text)
	assert.Equal(t, "amazon-bedrock/claude", out.Result.Model)
	assert.Equal(t, sid, out.SessionID)
}

func TestAwaitTurnStopsOnPendingDecision(t *testing.T) {
	old := decisionPoll
	decisionPoll = 10 * time.Millisecond
	t.Cleanup(func() { decisionPoll = old })
	f, _, sid := headlessFixture(t, true)
	_, err := f.svc.AwaitTurn(context.Background(), sid)
	assert.ErrorIs(t, err, ErrDecisionPending)
}

func TestAwaitTurnUnsupported(t *testing.T) {
	f, _ := runFixture(t)
	_, err := f.svc.AwaitTurn(context.Background(), "ses_x")
	assert.ErrorIs(t, err, ErrHeadlessUnsupported)
}

func TestHeadlessSessionType(t *testing.T) {
	f, _ := runFixture(t)
	req := f.run("read", LocationChoice{Kind: LocationBase})
	req.Base.Headless = true
	plan, err := f.svc.Plan(context.Background(), req)
	require.NoError(t, err)
	res, err := f.svc.Start(context.Background(), plan)
	require.NoError(t, err)
	sess, err := f.svc.Sessions.Get(context.Background(), res[0].SessionID)
	require.NoError(t, err)
	assert.Equal(t, domain.SessionTypeHeadless, sess.Type)
}
