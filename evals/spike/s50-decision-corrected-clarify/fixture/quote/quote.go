// Package quote calcule les devis.
package quote

import (
	"context"
	"time"
)

// Deadline est le délai accordé au calcul d'un devis, appels au service de tarifs compris.
const Deadline = 45 * time.Second

// WithDeadline borne le calcul d'un devis.
func WithDeadline(ctx context.Context) (context.Context, context.CancelFunc) {
	return context.WithTimeout(ctx, Deadline)
}
