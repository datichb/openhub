// Package invoice calcule les totaux d'une facture.
package invoice

// Line est une ligne de facture.
type Line struct {
	Label     string
	UnitCents int64
	Qty       int
}

// Total renvoie le total TTC en centimes, pour un taux de TVA vatRate (0.2 pour 20 %).
func Total(lines []Line, vatRate float64) int64 {
	var ht int64
	for _, l := range lines {
		ht += l.UnitCents * int64(l.Qty)
	}
	vat := int64(float64(ht) * vatRate)
	return ht + vat
}
