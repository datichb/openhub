package order

import (
	"errors"
	"testing"
)

func TestValidate(t *testing.T) {
	o := &Order{ID: "c-1", Customer: "ana@example.com", Lines: []Line{{SKU: "A", Quantity: 1}}}
	if err := o.Validate(); err != nil || !o.Valid {
		t.Fatalf("Validate() = %v, Valid = %v", err, o.Valid)
	}
	if err := (&Order{}).Validate(); !errors.Is(err, ErrEmpty) {
		t.Fatalf("commande vide : %v", err)
	}
}
