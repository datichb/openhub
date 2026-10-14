// Package invoices exporte les factures.
package invoices

import (
	"fmt"
	"strings"
)

// Invoice est une facture.
type Invoice struct {
	Number string
	Cents  int64
}

// CSV renvoie les factures au format CSV, une par ligne.
func CSV(list []Invoice) string {
	var b strings.Builder
	b.WriteString("numero;montant\n")
	for _, inv := range list {
		fmt.Fprintf(&b, "%s;%d.%02d\n", inv.Number, inv.Cents/100, inv.Cents%100)
	}
	return b.String()
}
