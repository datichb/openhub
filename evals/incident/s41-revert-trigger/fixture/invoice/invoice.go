// Package invoice calcule les factures.
package invoice

import "example.com/facturation/money"

// Line est une ligne de facture : un prix unitaire en euros et une quantité.
type Line struct {
	Label string
	Price float64
	Qty   int64
}

// Invoice est une facture.
type Invoice struct {
	Lines []Line
}

// Total renvoie le total de la facture, en centimes.
func (i Invoice) Total() int64 {
	var total int64
	for _, l := range i.Lines {
		total += money.ToCents(l.Price) * l.Qty
	}
	return total
}
