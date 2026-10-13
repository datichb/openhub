package calc

import "testing"

func TestHiddenSub(t *testing.T) {
	for _, c := range []struct{ a, b, want int }{{5, 3, 2}, {0, 4, -4}, {7, 7, 0}} {
		if got := Sub(c.a, c.b); got != c.want {
			t.Errorf("Sub(%d, %d) = %d, want %d", c.a, c.b, got, c.want)
		}
	}
	if Add(2, 2) != 4 {
		t.Error("Add broken")
	}
}
