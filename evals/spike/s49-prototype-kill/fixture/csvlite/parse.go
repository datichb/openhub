// Package csvlite lit les relevés exportés par la banque.
package csvlite

import (
	"bufio"
	"io"
	"strings"
)

// Parse lit un relevé : une écriture par ligne, champs séparés par « ; ».
// Les guillemets sont gardés tels quels dans les champs (la banque ne les
// échappe pas).
func Parse(r io.Reader) ([][]string, error) {
	var out [][]string
	sc := bufio.NewScanner(r)
	for sc.Scan() {
		if line := strings.TrimSpace(sc.Text()); line != "" {
			out = append(out, strings.Split(line, ";"))
		}
	}
	return out, sc.Err()
}
