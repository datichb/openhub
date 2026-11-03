package ui

import "example.com/inventaire/operation"

// RendreDetail est l'écran « Détails d'une opération » : le formulaire des
// valeurs enregistrées, que le gestionnaire peut modifier puis enregistrer.
func RendreDetail(dto operation.DTO) string {
	return "== Détails d'une opération ==\n" + Rendre(DepuisDTO(dto))
}
