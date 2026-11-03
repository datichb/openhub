// Package ui rend les écrans de l'inventaire.
package ui

import (
	"fmt"
	"strings"

	"example.com/inventaire/operation"
)

// Formulaire est le formulaire d'une opération, partagé par le détail d'une
// opération et la saisie d'une entrée hors inventaire. La catégorie est un
// choix que l'utilisateur modifie avant d'enregistrer.
type Formulaire struct {
	Nom       string
	Categorie string
	Logements int
}

// DepuisDTO remplit le formulaire avec les valeurs enregistrées.
func DepuisDTO(dto operation.DTO) Formulaire {
	return Formulaire{Nom: dto.Nom, Categorie: dto.Categorie, Logements: dto.Logements}
}

// Modifie dit si le formulaire diffère des valeurs enregistrées ; chaque champ
// du formulaire est comparé au DTO.
func (f Formulaire) Modifie(dto operation.DTO) bool {
	return f != DepuisDTO(dto)
}

// Rendre écrit les champs du formulaire, un par ligne.
func Rendre(f Formulaire) string {
	var b strings.Builder
	fmt.Fprintf(&b, "Nom : %s\n", f.Nom)
	fmt.Fprintf(&b, "Catégorie : %s\n", f.Categorie)
	fmt.Fprintf(&b, "Nombre de logements : %d\n", f.Logements)
	return b.String()
}
