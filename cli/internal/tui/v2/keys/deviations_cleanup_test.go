package keys

// deviationsCleanup: hints outside the key convention left to the lot
// tui-cleanup (TestKeyConvention). Remove a line once fixed; this file
// belongs to tui-cleanup only.
var deviationsCleanup = []string{
	"fr.json tui.catalog.edit.hints | D diff → compare se fait avec c",
	"fr.json tui.catalog.edit.hints | v valider → accept se fait avec y",
	"fr.json tui.editor.hints.general | x revenir à la valeur héritée → x est réservée (archive)",
	"fr.json tui.editor.hints.general | y YAML → y est réservée (accept)",
	"fr.json tui.editor.hints.graph | x retirer → x est réservée (archive)",
	"fr.json tui.editor.hints.inputs | x retirer → x est réservée (archive)",
	"fr.json tui.editor.hints.resources | x revenir à la valeur héritée → x est réservée (archive)",
	"fr.json tui.history.hints | r restaurer → r est réservée (refresh)",
	"models_view.go ModelsView.StatusHints | a ajouter → create se fait avec n",
	"patterns_view.go PatternsView.StatusHints | a ajouter → create se fait avec n",
	"patterns_view.go PatternsView.StatusHints | v valider → accept se fait avec y",
	"policies_view.go PoliciesView.StatusHints | a ajouter → create se fait avec n",
	"project_models_view.go ProjectModelsView.StatusHints | a ajouter → create se fait avec n",
	"projects_view.go ProjectsView.StatusHints | a ajouter → create se fait avec n",
	"projects_view.go ProjectsView.StatusHints | n renommer → n est réservée (create, refuse, next_match)",
	"provider_view.go ProviderView.StatusHints | d reset → d est réservée (delete)",
	"secrets_view.go SecretsView.StatusHints | a ajouter → create se fait avec n",
	"team_detail_view.go TeamDetailView.StatusHints | a ajouter → create se fait avec n",
	"team_detail_view.go TeamDetailView.StatusHints | d del → d est réservée (delete)",
	"team_detail_view.go TeamDetailView.StatusHints | y discovery → y est réservée (accept)",
	"teamboard_view.go TeamBoardView.StatusHints | f filtrer → filter se fait avec /",
	"teamboard_view.go TeamBoardView.StatusHints | x libérer → x est réservée (archive)",
	"teams_view.go TeamsView.StatusHints | a ajouter → create se fait avec n",
	"worktree_view.go WorktreeView.StatusHints | a ajouter → create se fait avec n",
	"worktree_view.go WorktreeView.StatusHints | x cleanup → x est réservée (archive)",
}
