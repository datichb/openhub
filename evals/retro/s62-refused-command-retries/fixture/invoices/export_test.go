package invoices

import "testing"

func TestCSV(t *testing.T) {
	if got := CSV([]Invoice{{"F-1", 1234}}); got != "numero;montant\nF-1;12.34\n" {
		t.Fatalf("CSV = %q", got)
	}
}
