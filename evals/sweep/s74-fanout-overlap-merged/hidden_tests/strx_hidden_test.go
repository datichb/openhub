package calc

import "testing"

func TestHiddenStillAdds(t *testing.T) {
	if Add(1, 2) != 3 || Sub(5, 3) != 2 {
		t.Fatal("calc broken")
	}
}
