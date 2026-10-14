// Package biblio gère les prêts d'une médiathèque.
package biblio

import (
	"errors"
	"time"
)

// Member est un adhérent de la médiathèque.
type Member struct {
	ID   string
	Name string
}

// Loan est un livre prêté ; BorrowerID est l'identifiant de l'emprunteur.
type Loan struct {
	BookID     string
	BorrowerID string
	Due        time.Time
}

// Reservation place un adhérent dans la file d'attente d'un livre déjà
// prêté ; Rank est sa place dans la file.
type Reservation struct {
	BookID   string
	MemberID string
	Rank     int
}

// Library garde l'état de la médiathèque en mémoire.
type Library struct {
	Loans        []Loan
	Reservations []Reservation
}

// ErrUnavailable : le livre est déjà prêté.
var ErrUnavailable = errors.New("livre déjà prêté")

// Reserve prête un livre disponible à l'adhérent pour trois semaines.
func (l *Library) Reserve(bookID, memberID string, now time.Time) error {
	for _, ln := range l.Loans {
		if ln.BookID == bookID {
			return ErrUnavailable
		}
	}
	l.Loans = append(l.Loans, Loan{BookID: bookID, BorrowerID: memberID, Due: now.Add(21 * 24 * time.Hour)})
	return nil
}

// Queue inscrit l'adhérent dans la file d'attente d'un livre prêté.
func (l *Library) Queue(bookID, memberID string) int {
	rank := 1
	for _, r := range l.Reservations {
		if r.BookID == bookID {
			rank++
		}
	}
	l.Reservations = append(l.Reservations, Reservation{BookID: bookID, MemberID: memberID, Rank: rank})
	return rank
}
