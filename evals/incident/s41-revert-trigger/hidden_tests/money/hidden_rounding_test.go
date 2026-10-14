package money

import "testing"

func TestHiddenToCentsRounds(t *testing.T) {
	for in, want := range map[float64]int64{19.99: 1999, 0.29: 29, 4.35: 435, 1.15: 115} {
		if got := ToCents(in); got != want {
			t.Errorf("ToCents(%v) = %d, want %d", in, got, want)
		}
	}
}
