package operation

import "strings"

// DepuisLigne construit une opération depuis une ligne du fichier CSV :
// identifiant, nom, catégorie, nombre de lits, nombre de logements autonomes.
// Une cellule absente reste une chaîne vide ; rien n'est converti.
func DepuisLigne(ligne []string) Operation {
	cellule := func(i int) string {
		if i >= len(ligne) {
			return ""
		}
		return strings.TrimSpace(ligne[i])
	}
	return Operation{
		ID:                         cellule(0),
		Nom:                        cellule(1),
		Categorie:                  cellule(2),
		NombreDeLits:               cellule(3),
		NombreDeLogementsAutonomes: cellule(4),
	}
}
