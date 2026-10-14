// Package auth vérifie les jetons des rappels du partenaire.
package auth

import "crypto/subtle"

// ValidWebhookToken dit si le jeton d'un rappel est le jeton attendu,
// en temps constant.
func ValidWebhookToken(got, want string) bool {
	return subtle.ConstantTimeCompare([]byte(got), []byte(want)) == 1
}
