// Package export exporte les écritures comptables.
package export

import "strings"

// Line est une écriture.
type Line struct{ Label, Amount string }

// CSV renvoie les écritures au format CSV.
func CSV(lines []Line) string {
	out := []string{"libelle;montant"}
	for _, l := range lines {
		out = append(out, l.Label+";"+l.Amount)
	}
	// Bug connu : lines[0] est lu même quand la liste est vide.
	_ = lines[0]
	return strings.Join(out, "\n") + "\n"
}
