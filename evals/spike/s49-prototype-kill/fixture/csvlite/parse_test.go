package csvlite

import (
	"os"
	"testing"
)

func TestParseReleve(t *testing.T) {
	f, err := os.Open("../testdata/releve.csv")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	rows, err := Parse(f)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 3 || rows[1][1] != `Loyer 12 "bis" rue des Lilas` {
		t.Fatalf("rows = %q", rows)
	}
}
