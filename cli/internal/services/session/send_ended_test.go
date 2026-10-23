package session

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/domain"
)

func setState(t *testing.T, f *fixture, st domain.RunState) {
	t.Helper()
	sess, err := f.svc.Sessions.Get(f.ctx, "ses_a")
	require.NoError(t, err)
	sess.State = st
	require.NoError(t, f.svc.Sessions.Update(f.ctx, sess))
}

// A6: a message to a session that is over is refused, with a typed error the
// CLI and the TUI turn into « la consigne ne sera pas remise ». Every open
// state takes it (the tool only admits it: it is read at the next step).
func TestSendToAnEndedSessionIsRefused(t *testing.T) {
	for _, st := range []domain.RunState{domain.RunCompleted, domain.RunFailed, domain.RunStopped} {
		t.Run(string(st), func(t *testing.T) {
			f := newFixture(t, false)
			setState(t, f, st)
			err := f.svc.Send(f.ctx, "ses_a", "relis les tests", SendOptions{ReportEnd: true})
			var ended *EndedError
			require.ErrorAs(t, err, &ended)
			assert.ErrorIs(t, err, ErrSessionEnded)
			assert.Equal(t, EndedError{SessionID: "ses_a", State: st, Send: true}, *ended)
			assert.Empty(t, f.tool.controls, "nothing is sent to the tool")
		})
	}
	for _, st := range []domain.RunState{domain.RunActive, domain.RunWaiting, domain.RunIdle} {
		f := newFixture(t, false)
		setState(t, f, st)
		require.NoError(t, f.svc.Send(f.ctx, "ses_a", "relis les tests", SendOptions{ReportEnd: true}), st)
		assert.Len(t, f.tool.controls, 1, st)
	}
}

// Other operations on a session that is over say so too, not with a raw error.
func TestControlOfAnEndedSessionIsTyped(t *testing.T) {
	f := newFixture(t, false)
	setState(t, f, domain.RunStopped)
	err := f.svc.Interrupt(f.ctx, "ses_a")
	var ended *EndedError
	require.ErrorAs(t, err, &ended)
	assert.False(t, ended.Send)
}

// A6: the tool admits a message without reading it. A session that ends as
// the message is sent is reported to a user (ReportEnd), who would otherwise
// be told it was sent; the callers that are not a user keep their result.
func TestSendReportsASessionThatEndedDuringTheSend(t *testing.T) {
	f := newFixture(t, false)
	f.tool.onControl = func() {
		sess, _ := f.svc.Sessions.Get(f.ctx, "ses_a")
		sess.State = domain.RunCompleted
		_ = f.svc.Sessions.Update(f.ctx, sess)
	}
	err := f.svc.Send(f.ctx, "ses_a", "relis les tests", SendOptions{ReportEnd: true})
	var ended *EndedError
	require.ErrorAs(t, err, &ended)
	assert.Equal(t, EndedError{SessionID: "ses_a", State: domain.RunCompleted, Send: true, During: true}, *ended)
	assert.Len(t, f.tool.controls, 1, "the tool was told: the message may be read, or not")

	setState(t, f, domain.RunWaiting)
	require.NoError(t, f.svc.Send(f.ctx, "ses_a", "relis les tests", SendOptions{}), "the checkpoint refusals and the fan-out do not ask")
}

func TestEndedErrorIsNotAnotherError(t *testing.T) {
	assert.False(t, errors.Is(ErrServerNotRunning, ErrSessionEnded))
	assert.False(t, errors.Is(&EndedError{}, ErrServerNotRunning))
}
