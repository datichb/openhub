// Package ledger lit un relevé de compte.
package ledger

import (
	"bufio"
	"fmt"
	"io"
	"strconv"
	"strings"
)

// Entry est une écriture du relevé.
type Entry struct {
	Date  string
	Label string
	Cents int64
	// Fields sont les champs bruts de la ligne : toujours 4 (date, libellé,
	// montant, note), un champ vide restant une chaîne vide.
	Fields []string
}

// Parse lit un relevé, une écriture par ligne non vide.
func Parse(r io.Reader) ([]Entry, error) {
	var out []Entry
	sc := bufio.NewScanner(r)
	n := 0
	for sc.Scan() {
		n++
		line := strings.TrimSpace(sc.Text())
		if line == "" {
			continue
		}
		f := splitRecord(line)
		if len(f) < 3 {
			return nil, fmt.Errorf("ligne %d : %d champs, au moins 3 attendus", n, len(f))
		}
		cents, err := parseCents(f[2])
		if err != nil {
			return nil, fmt.Errorf("ligne %d : %w", n, err)
		}
		out = append(out, Entry{Date: f[0], Label: f[1], Cents: cents, Fields: f})
	}
	return out, sc.Err()
}

// splitRecord découpe une ligne « date;libellé;montant;note ».
func splitRecord(line string) []string {
	return strings.FieldsFunc(line, func(r rune) bool { return r == ';' })
}

// parseCents lit un montant « 12.50 » en centimes.
func parseCents(s string) (int64, error) {
	whole, frac, _ := strings.Cut(strings.TrimSpace(s), ".")
	w, err := strconv.ParseInt(whole, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("montant invalide %q", s)
	}
	frac = (frac + "00")[:2]
	c, err := strconv.ParseInt(frac, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("montant invalide %q", s)
	}
	return w*100 + c, nil
}
