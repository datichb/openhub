package operation

import "strings"

// LigneExport est la ligne CSV d'une opération, relue par le service des statistiques.
func LigneExport(op Operation) string {
	return strings.Join([]string{op.ID, op.Nom, op.Categorie, op.NombreDeLits, op.NombreDeLogementsAutonomes}, ";")
}
