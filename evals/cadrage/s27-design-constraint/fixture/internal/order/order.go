// Package order porte les commandes et leurs règles.
package order

import "errors"

// ErrEmpty signale une commande sans ligne.
var ErrEmpty = errors.New("order: commande vide")

// Line est une ligne de commande.
type Line struct {
	SKU      string
	Quantity int
}

// Order est une commande d'un client.
type Order struct {
	ID       string
	Customer string // adresse e-mail du client
	Lines    []Line
	Valid    bool
}

// Validate vérifie la commande et la marque comme validée.
func (o *Order) Validate() error {
	if len(o.Lines) == 0 {
		return ErrEmpty
	}
	o.Valid = true
	return nil
}
