package ui

// RendreEntree est l'écran de saisie d'une entrée hors inventaire : il
// réutilise le formulaire de l'opération, vide.
func RendreEntree() string {
	return "== Nouvelle entrée hors inventaire ==\n" + Rendre(Formulaire{})
}
