package invoice

import "testing"

func TestTotal(t *testing.T) {
	lines := []Line{{Label: "café", UnitCents: 250, Qty: 2}, {Label: "thé", UnitCents: 300, Qty: 1}}
	if got := Total(lines, 0.2); got != 960 {
		t.Fatalf("Total = %d, want 960", got)
	}
}
