package invoice

import "testing"

// The feature committed after the faulty change stays (only the trigger is reverted).
func TestHiddenCustomerReferenceKept(t *testing.T) {
	inv := Invoice{CustomerRef: "C-42", Lines: []Line{{Label: "Stylo", Price: 19.99, Qty: 1}}}
	if inv.CustomerRef != "C-42" || inv.Total() != 1999 {
		t.Fatalf("CustomerRef = %q, Total = %d", inv.CustomerRef, inv.Total())
	}
}
