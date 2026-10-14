package report

import (
	"strings"
	"testing"

	"example.com/releve/ledger"
)

func entries(t *testing.T, in string) []ledger.Entry {
	t.Helper()
	es, err := ledger.Parse(strings.NewReader(in))
	if err != nil {
		t.Fatal(err)
	}
	return es
}

func TestLine(t *testing.T) {
	e := entries(t, "2026-03-01;Boulangerie;4.20;pain\n")[0]
	if got, want := Line(e), "2026-03-01 | Boulangerie | 4,20 €"; got != want {
		t.Fatalf("Line = %q, want %q", got, want)
	}
}
