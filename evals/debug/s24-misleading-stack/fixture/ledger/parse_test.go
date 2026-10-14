package ledger

import (
	"strings"
	"testing"
)

func TestParse(t *testing.T) {
	in := "2026-03-01;Boulangerie;4.20;pain\n\n2026-03-02;Loyer;850.00;mars\n"
	got, err := Parse(strings.NewReader(in))
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 {
		t.Fatalf("%d écritures, 2 attendues", len(got))
	}
	if got[1].Label != "Loyer" || got[1].Cents != 85000 {
		t.Fatalf("écriture 2 = %+v", got[1])
	}
}

func TestParseInvalidAmount(t *testing.T) {
	if _, err := Parse(strings.NewReader("2026-03-01;Café;abc;x\n")); err == nil {
		t.Fatal("erreur attendue")
	}
}
