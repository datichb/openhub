package report

import "testing"

func TestTitle(t *testing.T) {
	if got := Title(3); got != "Semaine 3" {
		t.Fatalf("Title = %q", got)
	}
}
