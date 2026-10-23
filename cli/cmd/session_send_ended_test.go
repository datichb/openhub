package cmd

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/datichb/openhub/cli/internal/domain"
	"github.com/datichb/openhub/cli/internal/i18n"
	sessionsvc "github.com/datichb/openhub/cli/internal/services/session"
)

// A6: oh session send and the Sessions view say, in the language of the user,
// that the instruction is not delivered (session over) or may not be (session
// ended during the send), instead of « session ses_a is completed ».
func TestSessionEndedMessages(t *testing.T) {
	defer i18n.SetLocale("en")
	for lang, want := range map[string]map[string]string{
		"fr": {
			"completed": "La session ses_a est terminée : la consigne ne sera pas remise. Pour continuer, ouvre-la dans l'interface (oh session attach).",
			"failed":    "La session ses_a a échoué : la consigne ne sera pas remise. Pour continuer, ouvre-la dans l'interface (oh session attach).",
			"stopped":   "La session ses_a a été arrêtée : la consigne ne sera pas remise (une session arrêtée ne se reprend pas).",
			"during":    "La session ses_a s'est terminée pendant l'envoi : l'outil a reçu la consigne mais elle n'a peut-être pas été prise en compte (oh session trace).",
			"other":     "La session ses_a est terminée (stopped) : l'opération n'a pas eu lieu.",
		},
		"en": {
			"completed": "Session ses_a is over: the instruction will not be delivered. To go on, open it in the client (oh session attach).",
			"failed":    "Session ses_a failed: the instruction will not be delivered. To go on, open it in the client (oh session attach).",
			"stopped":   "Session ses_a was stopped: the instruction will not be delivered (a stopped session cannot be resumed).",
			"during":    "Session ses_a ended while the instruction was sent: the tool received it but it may not have been taken into account (oh session trace).",
			"other":     "Session ses_a is over (stopped): the operation did not take place.",
		},
	} {
		i18n.SetLocale(lang)
		for _, st := range []domain.RunState{domain.RunCompleted, domain.RunFailed, domain.RunStopped} {
			err := decisionError(fmt.Errorf("wrapped: %w", &sessionsvc.EndedError{SessionID: "ses_a", State: st, Send: true}))
			assert.EqualError(t, err, want[string(st)], lang+" "+string(st))
		}
		assert.EqualError(t, decisionError(&sessionsvc.EndedError{SessionID: "ses_a", State: domain.RunCompleted, Send: true, During: true}), want["during"], lang)
		assert.EqualError(t, decisionError(&sessionsvc.EndedError{SessionID: "ses_a", State: domain.RunStopped}), want["other"], lang)
	}
}
