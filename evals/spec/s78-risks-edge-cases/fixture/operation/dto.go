package operation

// DTO est ce que l'API renvoie pour une opération ; la liste et le détail le
// partagent.
type DTO struct {
	ID        string
	Nom       string
	Categorie string
	Logements int
}

// VersDTO convertit une opération en DTO.
func VersDTO(op Operation) DTO {
	return DTO{ID: op.ID, Nom: op.Nom, Categorie: op.Categorie, Logements: op.Logements}
}
