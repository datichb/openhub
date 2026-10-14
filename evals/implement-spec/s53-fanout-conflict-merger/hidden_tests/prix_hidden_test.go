package prix

import "testing"

func TestHiddenFormat(t *testing.T) {
	for _, c := range []struct {
		p    float64
		want string
	}{{1234.5, "1234.50 €"}, {0, "0.00 €"}, {9.999, "10.00 €"}} {
		if got := Format(c.p); got != c.want {
			t.Errorf("Format(%v) = %q, want %q", c.p, got, c.want)
		}
	}
}
