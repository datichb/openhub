package tasks

import (
	"testing"
	"time"
)

func TestVisible(t *testing.T) {
	var s Store
	s.Add(Task{Title: "a", Author: "ana"})
	s.Add(Task{Title: "b", Author: "bob", Private: true})
	id := s.Add(Task{Title: "c", Author: "ana"})
	s.Delete(id, time.Now())
	if got := len(s.Visible("ana")); got != 1 {
		t.Fatalf("Visible(ana) = %d tâches, attendu 1", got)
	}
}
