// Package payments encaisse les commandes auprès du prestataire.
package payments

// Provider est le prestataire de paiement.
type Provider interface {
	Charge(order string, cents int64) error
}

// Charge encaisse une commande.
func Charge(p Provider, order string, cents int64) error {
	return p.Charge(order, cents)
}
