package biblio

import (
	"testing"
	"time"
)

func TestReserveThenQueue(t *testing.T) {
	var l Library
	if err := l.Reserve("b1", "m1", time.Now()); err != nil {
		t.Fatal(err)
	}
	if err := l.Reserve("b1", "m2", time.Now()); err != ErrUnavailable {
		t.Fatalf("Reserve d'un livre prêté = %v, attendu ErrUnavailable", err)
	}
	if rank := l.Queue("b1", "m2"); rank != 1 {
		t.Fatalf("Queue = %d, attendu 1", rank)
	}
}
