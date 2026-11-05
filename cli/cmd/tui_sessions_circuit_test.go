package cmd

import (
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/domain"
	"github.com/datichb/openhub/cli/internal/i18n"
	"github.com/datichb/openhub/cli/internal/i18n/i18ntest"
	sessionsvc "github.com/datichb/openhub/cli/internal/services/session"
)

// A circuit breaker decision reaches the Sessions view and its toast with
// the complete title (the count filled in), once, short enough for the toast.
func TestCircuitDecisionTitleOnTheTUISurfaces(t *testing.T) {
	for locale, want := range map[string]string{
		"fr": "Coupe-circuit : 12 délégations d'affilée sans intervention",
		"en": "Circuit breaker: 12 delegations in a row without the user",
	} {
		t.Run(locale, func(t *testing.T) {
			i18ntest.Set(t, locale)
			d := domain.Decision{ID: "circuit:ses_a:1", SessionID: "ses_a", Kind: domain.DecisionCircuit, CreatedAt: time.Now(),
				Payload: domain.DecisionPayload{Title: i18n.Tf("tui.checkpoint.breaker_title", 12)}}
			require.Equal(t, want, d.Payload.Title)

			vd := toViewDecision(d)
			assert.Equal(t, want, vd.Summary, "list and detail")
			assert.Empty(t, vd.Message, "the card does not repeat the title")
			assert.Equal(t, "circuit", vd.Kind)

			toast := i18n.Tf("tui.notify.toast_one", sessionsvc.KindIcon(d.Kind), sessionsvc.DecisionSummary(d))
			assert.Equal(t, 1, strings.Count(toast, want), "the whole title, once")
			assert.NotContains(t, toast, "%")
			assert.LessOrEqual(t, len([]rune(toast)), 80, "fits the toast without being clipped")
		})
	}
}
