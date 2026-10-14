package money

import "testing"

func TestToCents(t *testing.T) {
	for in, want := range map[float64]int64{0: 0, 1: 100, 12.5: 1250, 3.25: 325} {
		if got := ToCents(in); got != want {
			t.Errorf("ToCents(%v) = %d, want %d", in, got, want)
		}
	}
}
