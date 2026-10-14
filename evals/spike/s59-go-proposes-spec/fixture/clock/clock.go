// Package clock donne l'heure locale de l'agenda.
package clock

import "time"

// Now renvoie l'heure locale (fuseau de TZ).
func Now() time.Time { return time.Now().Local() }
