// Package loans gère les prêts et les réservations de la médiathèque.
package loans

import "time"

// Loan est un prêt en cours : le document est sorti.
type Loan struct {
	Item   string
	Member string
	Due    time.Time
}

// Reservation est une place dans la file d'attente d'un document déjà prêté.
type Reservation struct {
	Item   string
	Member string
	Rank   int
}

// Reserve prête le document tout de suite s'il est disponible (le nom vient
// de l'ancien guichet : « réserver » voulait dire « emprunter »).
func Reserve(item, member string, now time.Time) Loan {
	return Loan{Item: item, Member: member, Due: now.AddDate(0, 0, 21)}
}
