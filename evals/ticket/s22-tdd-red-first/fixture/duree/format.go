// Package duree lit et affiche des durées au format court.
package duree

import (
	"fmt"
	"time"
)

// Format affiche une durée au format court : « 1h30 », « 2h », « 45m ».
func Format(d time.Duration) string {
	h := int(d / time.Hour)
	m := int((d % time.Hour) / time.Minute)
	switch {
	case h == 0:
		return fmt.Sprintf("%dm", m)
	case m == 0:
		return fmt.Sprintf("%dh", h)
	}
	return fmt.Sprintf("%dh%02d", h, m)
}
