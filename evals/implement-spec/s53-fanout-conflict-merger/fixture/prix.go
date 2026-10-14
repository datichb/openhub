// Package prix met en forme des prix.
package prix

import "fmt"

// Format renvoie le prix p mis en forme pour l'affichage.
func Format(p float64) string {
	return fmt.Sprintf("%v", p)
}
