package invoice

import "testing"

func TestTotal(t *testing.T) {
	inv := Invoice{Lines: []Line{{Label: "Stylo", Price: 1.5, Qty: 2}, {Label: "Cahier", Price: 4, Qty: 1}}}
	if got := inv.Total(); got != 700 {
		t.Fatalf("Total = %d, want 700", got)
	}
}
