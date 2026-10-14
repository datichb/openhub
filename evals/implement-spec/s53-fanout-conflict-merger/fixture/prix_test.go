package prix

import "testing"

func TestFormatEntier(t *testing.T) {
	if got := Format(12); got == "" {
		t.Fatal("Format(12) vide")
	}
}
