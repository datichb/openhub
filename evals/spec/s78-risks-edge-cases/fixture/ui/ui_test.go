package ui

import (
	"strings"
	"testing"

	"example.com/inventaire/operation"
)

func TestRendreDetail(t *testing.T) {
	out := RendreDetail(operation.DTO{ID: "op-1", Nom: "Résidence des Lilas", Categorie: "FOYER LOGEMENT", Logements: 4})
	if !strings.Contains(out, "Nombre de logements : 4") {
		t.Fatalf("out = %q", out)
	}
}

func TestModifie(t *testing.T) {
	dto := operation.DTO{Nom: "A", Categorie: "FOYER LOGEMENT"}
	f := DepuisDTO(dto)
	if f.Modifie(dto) {
		t.Fatal("un formulaire intact ne doit pas être modifié")
	}
	f.Categorie = operation.CategorieOrdinaire
	if !f.Modifie(dto) {
		t.Fatal("un changement de catégorie doit être détecté")
	}
}

func TestRendreEntree(t *testing.T) {
	if !strings.Contains(RendreEntree(), "Nouvelle entrée") {
		t.Fatal("titre manquant")
	}
}
