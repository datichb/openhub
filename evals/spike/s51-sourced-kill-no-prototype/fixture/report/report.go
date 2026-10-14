// Package report produit le rapport hebdomadaire.
package report

import "fmt"

// Title renvoie le titre du rapport de la semaine n.
func Title(n int) string { return fmt.Sprintf("Semaine %d", n) }
