package operation

import "testing"

func TestDepuisLigne(t *testing.T) {
	op := DepuisLigne([]string{"op-1", "Résidence des Lilas", "FOYER LOGEMENT", "12", ""})
	if op.NombreDeLits != "12" || op.NombreDeLogementsAutonomes != "" {
		t.Fatalf("op = %+v", op)
	}
}

func TestVersDTO(t *testing.T) {
	dto := VersDTO(Operation{ID: "op-1", Categorie: "FOYER LOGEMENT", Logements: 4})
	if dto.Logements != 4 {
		t.Fatalf("dto = %+v", dto)
	}
}
