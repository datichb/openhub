package calc

import "testing"

func TestHiddenMul(t *testing.T) {
	for _, c := range []struct{ a, b, want int }{{4, 3, 12}, {-2, 3, -6}, {0, 9, 0}} {
		if got := Mul(c.a, c.b); got != c.want {
			t.Errorf("Mul(%d, %d) = %d, want %d", c.a, c.b, got, c.want)
		}
	}
}
