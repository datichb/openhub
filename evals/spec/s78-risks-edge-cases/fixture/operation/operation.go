// Package operation porte le modèle d'une opération de logement, son import
// et son export.
package operation

// Valeurs de la catégorie de logement, telles que la base les stocke.
const (
	CategorieFoyer     = "LOGEMENT FOYER"
	CategorieOrdinaire = "LOGEMENT ORDINAIRE"
)

// Operation est une opération de logement de l'inventaire.
type Operation struct {
	ID        string
	Nom       string
	Categorie string
	// Logements est le nombre de logements actifs : calculé, jamais importé.
	Logements int
	// NombreDeLits et NombreDeLogementsAutonomes viennent de l'import (colonnes
	// du fichier des directions) et sont gardés tels qu'écrits.
	NombreDeLits               string
	NombreDeLogementsAutonomes string
}
