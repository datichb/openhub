// Package money convertit les prix du catalogue.
package money

import "math"

// ToCents convertit un prix en euros en centimes, arrondi au centime le plus proche.
func ToCents(eur float64) int64 {
	return int64(math.Round(eur * 100))
}
