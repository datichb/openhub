package invoice

import "testing"

// Côté main : la TVA est arrondie au centime le plus proche.
func TestHiddenVATRounded(t *testing.T) {
	lines := []Line{{Label: "a", UnitCents: 333, Qty: 1}}
	// 333 × 0,2 = 66,6 → 67 (et non 66 en tronquant).
	if got := Total(lines, 0.2); got != 400 {
		t.Fatalf("Total = %d, want 400", got)
	}
}

// Côté feature : les lignes de quantité nulle ou négative sont ignorées.
func TestHiddenSkipsNonPositiveQty(t *testing.T) {
	lines := []Line{{Label: "a", UnitCents: 1000, Qty: 1}, {Label: "retour", UnitCents: 500, Qty: -2}, {Label: "vide", UnitCents: 700, Qty: 0}}
	if got := Subtotal(lines); got != 1000 {
		t.Fatalf("Subtotal = %d, want 1000", got)
	}
	if got := Total(lines, 0.2); got != 1200 {
		t.Fatalf("Total = %d, want 1200", got)
	}
}

// Les deux à la fois.
func TestHiddenBothSides(t *testing.T) {
	lines := []Line{{Label: "a", UnitCents: 333, Qty: 1}, {Label: "retour", UnitCents: 999, Qty: -1}}
	if got := Total(lines, 0.2); got != 400 {
		t.Fatalf("Total = %d, want 400", got)
	}
}

// L'arrondi de main reporté dans VAT, extrait par feature.
func TestHiddenVATHelperRounded(t *testing.T) {
	if got := VAT(333, 0.2); got != 67 {
		t.Fatalf("VAT(333, 0.2) = %d, want 67", got)
	}
}

// Côté main : un taux négatif compte pour 0.
func TestHiddenNegativeRate(t *testing.T) {
	lines := []Line{{Label: "a", UnitCents: 1000, Qty: 1}}
	if got := Total(lines, -0.2); got != 1000 {
		t.Fatalf("Total(-0.2) = %d, want 1000", got)
	}
}

// Le HT de TotalDiscounted (main) ignore lui aussi les quantités nulles ou négatives (feature).
func TestHiddenDiscountedSkipsNonPositiveQty(t *testing.T) {
	lines := []Line{{Label: "a", UnitCents: 1000, Qty: 2}, {Label: "retour", UnitCents: 1000, Qty: -1}}
	if got := TotalDiscounted(lines, 0.2, 10); got != 2160 {
		t.Fatalf("TotalDiscounted = %d, want 2160", got)
	}
}
