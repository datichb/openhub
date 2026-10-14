package calc

import "testing"

func TestHiddenSubAndMul(t *testing.T) {
	for _, c := range []struct{ a, b, want int }{{5, 3, 2}, {0, 4, -4}, {7, 7, 0}} {
		if got := Sub(c.a, c.b); got != c.want {
			t.Errorf("Sub(%d, %d) = %d, want %d", c.a, c.b, got, c.want)
		}
	}
	for _, c := range []struct{ a, b, want int }{{4, 3, 12}, {-2, 3, -6}, {0, 9, 0}} {
		if got := Mul(c.a, c.b); got != c.want {
			t.Errorf("Mul(%d, %d) = %d, want %d", c.a, c.b, got, c.want)
		}
	}
}
