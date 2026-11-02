package teamstate

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/datichb/openhub/cli/internal/glyph"
	"github.com/datichb/openhub/cli/internal/i18n/i18ntest"
)

// The phrase of an event, after its actor: the same in oh team activity, the
// Activity view and the Team Status view.
func TestEventDescribe(t *testing.T) {
	i18ntest.Set(t, "fr")
	ev := func(typ string, data map[string]interface{}) Event {
		return Event{Type: typ, Project: "api", Ticket: "bd-7", Data: data}
	}
	tests := []struct {
		event Event
		want  string
	}{
		{ev(EventClaimTaken, nil), "a pris api/bd-7"},
		{ev(EventClaimReleased, nil), "a libéré api/bd-7"},
		{ev(EventClaimTransferred, map[string]interface{}{"to": "alice"}), "a transféré api/bd-7 à alice"},
		{ev(EventClaimTransferred, nil), "a transféré api/bd-7"},
		{ev(EventClaimConflict, map[string]interface{}{"current_owner": "alice"}), "conflit de prise sur api/bd-7"},
		{ev(EventSessionComplete, nil), "a terminé une session sur api/bd-7"},
		{Event{Type: EventSessionComplete}, "a terminé une session"},
		{Event{Type: EventSessionComplete, Project: "api"}, "a terminé une session sur api"},
		{ev(EventReviewReady, nil), "revue prête · api/bd-7"},
		{ev(EventReviewApproved, nil), "a approuvé la revue de api/bd-7"},
		{ev(EventReviewRejected, map[string]interface{}{"reason": "tests manquants"}), "a rejeté la revue de api/bd-7 : tests manquants"},
		{ev(EventReviewRejected, nil), "a rejeté la revue de api/bd-7"},
		{ev(EventAuditFinding, nil), "constat d'audit · api/bd-7"},
		{ev(EventWikiProposal, map[string]interface{}{"page": "decisions"}), "proposition wiki decisions"},
		{ev(EventWikiAccepted, map[string]interface{}{"page": "decisions"}), "entrée wiki decisions acceptée"},
		{ev(EventWikiRejected, map[string]interface{}{"page": "decisions", "reason": "déjà couvert"}), "proposition wiki decisions rejetée : déjà couvert"},
		{ev(EventWikiRejected, map[string]interface{}{"page": "decisions"}), "proposition wiki decisions rejetée"},
		{ev(EventWorkflowPublished, map[string]interface{}{"workflow": "ticket", "version": float64(3)}), "a publié le workflow ticket (v3)"},
		{ev(EventWorkflowRestored, map[string]interface{}{"workflow": "ticket", "restored_from": float64(1), "version": float64(4)}), "a restauré le workflow ticket v1 (nouvelle version v4)"},
		{ev(EventWorkflowArchived, map[string]interface{}{"workflow": "ticket"}), "a archivé le workflow ticket"},
		{ev("custom.thing", nil), "custom.thing"},
	}
	for _, tt := range tests {
		assert.Equal(t, tt.want, tt.event.Describe(), tt.event.Type)
	}

	i18ntest.Set(t, "en")
	assert.Equal(t, "claimed api/bd-7", ev(EventClaimTaken, nil).Describe())
}

// Every event type of the package has its symbol in glyph.Events, and the
// table lists no other type.
func TestEveryEventTypeHasASymbol(t *testing.T) {
	types := []string{
		EventSessionComplete, EventReviewReady, EventReviewApproved, EventReviewRejected, EventAuditFinding,
		EventClaimTaken, EventClaimConflict, EventClaimTransferred, EventClaimReleased,
		EventWikiProposal, EventWikiAccepted, EventWikiRejected,
		EventWorkflowPublished, EventWorkflowRestored, EventWorkflowArchived,
	}
	known := map[string]bool{}
	for _, e := range glyph.Events {
		known[e.Type] = true
	}
	for _, typ := range types {
		assert.True(t, known[typ], "%s has no row in glyph.Events", typ)
		assert.NotEqual(t, typ, (&Event{Type: typ}).Describe(), "%s has a phrase", typ)
		assert.Equal(t, typ, Event{Type: typ}.Glyph().Type)
	}
	assert.Len(t, glyph.Events, len(types), "a row of glyph.Events without an event")
}
