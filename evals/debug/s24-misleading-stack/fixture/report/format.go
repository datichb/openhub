// Package report met en forme les écritures d'un relevé.
package report

import (
	"fmt"

	"example.com/releve/ledger"
)

// Line renvoie « date | libellé | montant » pour une écriture.
func Line(e ledger.Entry) string {
	return fmt.Sprintf("%s | %s | %s", e.Date, e.Label, money(e.Cents))
}

func money(cents int64) string {
	return fmt.Sprintf("%d,%02d €", cents/100, cents%100)
}
