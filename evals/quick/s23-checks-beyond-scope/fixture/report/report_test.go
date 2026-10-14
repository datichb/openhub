package report

import (
	"testing"

	"example.com/compteur/counter"
)

func TestRender(t *testing.T) {
	c := counter.New()
	c.Inc("pomme")
	c.Inc("kiwi")
	c.Inc("pomme")
	want := "kiwi: 1\npomme: 2\n"
	if got := Render(c); got != want {
		t.Fatalf("Render = %q, want %q", got, want)
	}
}
