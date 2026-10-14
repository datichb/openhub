// Package report affiche les comptes d'un compteur.
package report

import (
	"fmt"
	"strings"

	"example.com/compteur/counter"
)

// Render renvoie une ligne « clé: nombre » par clé, dans l'ordre des clés.
func Render(c counter.Counter) string {
	var b strings.Builder
	for _, k := range c.Keys() {
		fmt.Fprintf(&b, "%s: %d\n", k, c.Get(k))
	}
	return b.String()
}
