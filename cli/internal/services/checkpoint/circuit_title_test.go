package checkpoint

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/adapters"
	"github.com/datichb/openhub/cli/internal/domain"
	"github.com/datichb/openhub/cli/internal/i18n"
	"github.com/datichb/openhub/cli/internal/sessionspec"
)

// The circuit breaker decision carries a complete title: the count of
// delegations is filled in (never the raw "%d" format), the text is one line,
// and the message stays empty so that no surface repeats the title.
func TestCircuitBreakerDecisionTitle(t *testing.T) {
	for locale, want := range map[string]string{
		"fr": "Coupe-circuit : 3 délégations d'affilée sans intervention",
		"en": "Circuit breaker: 3 delegations in a row without the user",
	} {
		t.Run(locale, func(t *testing.T) {
			prev := i18n.Locale()
			i18n.SetLocale(locale)
			t.Cleanup(func() { i18n.SetLocale(prev) })

			svc, ctx := newTestService(t)
			writeBundle(t, svc.BundlesDir, "h_cb", &sessionspec.WorkflowRuntime{ID: "cb", MaxConsecutiveSubagents: 3})
			require.NoError(t, svc.Sessions.Create(ctx, &domain.Session{ID: "ses_cb", ProjectID: "p1", Status: domain.SessionStatusRunning, GroupKey: "g1", BundleHash: "h_cb", Mode: "manuel", State: domain.RunActive}))
			call := adapters.ToolCall{Action: sessionspec.ActionSubagent, Status: adapters.CallCalled, Input: map[string]any{"agent": "developer"}}

			var raised *domain.Decision
			for range 3 {
				_, d, err := svc.OnCall(ctx, "ses_cb", call)
				require.NoError(t, err)
				if d != nil {
					raised = d
				}
			}
			require.NotNil(t, raised, "the third delegation trips the breaker")
			assert.Equal(t, domain.DecisionCircuit, raised.Kind)
			assert.Equal(t, want, raised.Payload.Title)
			assert.NotContains(t, raised.Payload.Title, "%", "no raw format")
			assert.NotContains(t, raised.Payload.Title, "\n")
			assert.Empty(t, raised.Payload.Message, "the title is not repeated in the message")
			assert.Equal(t, 3, raised.Payload.Data[DataCount])
		})
	}
}
