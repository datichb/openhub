# Conventions de la TUI : touches et liste d'actions

Guide des vues de la TUI (lots `tui-keys` à `tui-docs`). La convention de touches est décidée (esquisses du 15/10) ; la référence utilisateur est le tableau « Convention des touches » de [`docs/reference/tui.fr.md`](../reference/tui.fr.md#convention-des-touches).

## La convention en bref

| Contexte | Touches |
|----------|---------|
| Partout | `↑↓`/`jk` déplacer, `g`/`G` premier/dernier, `/` filtrer, `r` rafraîchir, `?` aide, `Entrée` ouvrir, `Échap` revenir |
| Liste | `n` créer, `e` éditer, `d` supprimer (toujours confirmé), `x` archiver/classer (jamais destructif), `R` reprendre/restaurer, `c` comparer (diff) |
| Décision | `y` valider/accepter, `n` refuser (consigne, confirmé), `e` modifier, `s` laisser en attente, `A` tout accepter |
| Lecteur | `/` rechercher, `n`/`N` résultat suivant/précédent |

Règles :

- une vue peut donner une lettre libre à une action à elle (`a` attacher, `t` transférer, `p` relancer le pilote, `B` lot…) ;
- elle ne donne **jamais** un autre sens à `g G / r ? d x R y n` (touches strictes), ni une autre touche à une action de la convention (`a ajouter` → `n créer`, `r restaurer` → `R`, `D diff` → `c`) ;
- `e`, `s`, `A` et `c` peuvent porter une action propre à la vue quand le contexte n'a pas l'action de la convention (`e enchaîner` dans Sessions) ;
- `d` passe toujours par une confirmation (`ShellAccess.Confirm`, `danger`) ; `x` ne détruit rien (ce qui est classé se restaure avec `R`) ;
- `n` refuser demande une consigne : la consigne validée vaut confirmation.

## Le paquet `keys`

`cli/internal/tui/v2/keys` : une valeur `keys.Action` par action, avec ses touches et son libellé traduit (`tui.keys.action.*`, description `tui.keys.describe.*`).

```go
import "github.com/datichb/openhub/cli/internal/tui/v2/keys"

keys.Delete.Matches(ev)         // la touche (d) sans Ctrl ni Alt
keys.Delete.Hint()              // "d supprimer"
keys.Resume.HintAs(i18n.T(...)) // "R récupérer" : libellé précis, même sens
keys.HintsOf(keys.Open, keys.Back)             // "Entrée ouvrir · Échap revenir"
keys.Hints(keys.Accept.Hint(), "a attacher")   // assemble avec " · ", ignore les vides
keys.HelpRows(keys.Decision)    // lignes de l'aide : touches + description
keys.ByContext(keys.List), keys.All()
keys.Changes(), keys.ChangesByView() // touches changées, par vue (message « Nouvelles touches »)
keys.Pairs(keys.Create, keys.Delete) // "n", "créer", "d", "supprimer" : bloc « Touches » d'un détail
keys.SpaceName()                // « Espace » / « Space »
```

Une touche propre à une vue se déclare avec `keys.Own`, en variable du paquet de la vue :

```go
var relaunchKey = keys.Own("sessions.relaunch", 'p', "tui.sessions.fanout.relaunch")

relaunchKey.Matches(ev) // sans Ctrl ni Alt
relaunchKey.Hint()      // "p relancer le pilote"
```

`keys.Own` panique sur une touche stricte de la convention (`g G / r ? d x R y n`) ou un déplacement (`j k`) ; `TestOwnActions` (`keys/own_test.go`) vérifie que chaque action déclarée est traduite, a un identifiant unique et que son libellé ne reprend pas une action de la convention sur une autre touche (« a ajouter », « f filtrer »). Actions propres déclarées : `a` attacher, `m` consigne et `i` interrompre (Sessions), `p` relancer le pilote et `B` lot (fan-out), `C` confirmer le lot, `T` découvrir le tracker, `L` libérer un ticket, `M` nettoyer les worktrees fusionnés, `Y` YAML brut, `f` corriger et `o` hors ligne (Doctor).

Un changement de touche visible s'ajoute à `keys.Changes()` (vue, avant, après, libellé `tui.keys.change.<vue>.<action>`) : le message « Nouvelles touches » et la référence le montrent.

Actions : `Up`, `Down`, `First`, `Last`, `Navigate` (↑↓/jk), `Ends` (g/G), `Filter`, `Refresh`, `Help`, `Open`, `Back` ; `Create`, `Edit`, `Delete`, `Archive`, `Resume`, `Compare` ; `Accept`, `Refuse`, `Modify`, `Leave`, `AcceptAll` ; `Search`, `NextMatch`, `PrevMatch`. `Action.Confirm` (d, n) et `Action.Reason` (n) disent ce que l'action exige.

Dans une vue :

```go
func (v *MaVue) StatusHints() string {
	return keys.Hints(keys.Navigate.Hint(), keys.Open.Hint(),
		keys.Create.Hint(), keys.Delete.Hint(), keys.Refresh.Hint(), keys.Back.Hint())
}

func (v *MaVue) HandleKey(ev *tcell.EventKey) *tcell.EventKey {
	switch {
	case keys.Delete.Matches(ev):
		v.shell.Confirm(i18n.Tf("tui.mavue.delete_title", id), detail, true, func() { v.delete(id) })
		return nil
	case keys.Refresh.Matches(ev):
		v.reload()
		return nil
	}
	return ev
}
```

Le shell garde `Échap`, `?`/`F1`, `Ctrl+P`, `Ctrl+T`, `Ctrl+Q` ; il transforme `j`/`k`/`g`/`G` en flèches/`Début`/`Fin` et ouvre la palette sur `/` quand la vue ne prend pas `/` (une vue qui filtre consomme `/` dans `HandleKey`).

Une vue qui garde toutes les touches (`InputCapturing`, pour garder `Échap`) les reçoit d'abord ; sans champ de texte, elle implémente aussi `ShellKeysPassing` (`PassesShellKeys() bool`) : le shell prend alors `?` et `Ctrl+P` qu'elle n'a pas utilisés, et rend le focus au widget de la vue en fermant la palette ou l'aide (onglets et lot de Sessions, sections du Nettoyage). Quand ses touches changent (onglet, lot, section), elle appelle `RefreshHints()` du shell (`HintsRefresher`) pour que la ligne d'aide suive.

La ligne d'aide d'une vue reste courte : au plus 10 entrées ou 120 colonnes (`keys/convention_test.go`, `TestHintLinesAreShort` ; les lignes plus longues d'avant sont figées dans `longHints`, qui ne fait que diminuer). Une vue qui a plus de touches garde les essentielles dans la ligne, avec `? aide`, et implémente `HelpKeysProvider` (`HelpKeys() string`) : l'aide (`?` / `F1`) affiche alors cette liste complète à la place de la ligne d'aide (vue Sessions : `tui.sessions.help_keys`).

## Le composant `ActionList`

`views.ActionList` (`cli/internal/tui/v2/views/action_list.go`) : liste d'éléments sur une ligne (colonnes alignées, sections facultatives, état), détail sous la liste (`DetailBelow`, défaut) ou à côté (`DetailRight`), actions sur l'élément sélectionné et actions globales, sans limite d'éléments (la liste défile, le pied indique « 2/9 · 2 de plus ↓ »). Il sert aux propositions wiki (fiche, file du projet), à la validation par lot, aux enfants du fan-out, aux briques.

```go
l := views.NewActionList(views.ActionListConfig{
	Title:       i18n.Tf("tui.cpwiki.title", cp, len(list)),
	Layout:      views.DetailBelow,
	StateLabels: map[views.ItemState]string{views.ItemAccepted: "✓ " + i18n.T("tui.wiki.accepted_f")},
	Shell:       v.shell, // confirmation, consigne, toasts
	Actions: []views.ItemAction{
		{Key: keys.Accept, Label: i18n.T("tui.wiki.accept"), Applies: pending, Run: v.accept},
		{Key: keys.Refuse, Applies: pending, Run: v.refuse}, // demande la consigne
		{Key: keys.Modify, Run: v.edit},
		{Key: keys.Leave, Run: v.leave},
	},
	Global: []views.GlobalAction{{Key: keys.AcceptAll, Run: v.acceptAll}},
	OnOpen: v.validateCheckpoint, OpenLabel: i18n.T("tui.cpwiki.validate"),
})
l.SetItems(items) // []views.ActionItem{ID, Section, Cells, State, Detail, Ref}
l.SetEmpty(i18n.T("tui.mavue.loading")) // texte sans élément (chargement, erreur, aucun)
content.AddItem(l.Primitive(), 0, 1, true)
```

- `Run(it, reason)` est appelé sur la boucle d'événements : appeler le service hors de la boucle, puis `l.SetState(id, views.ItemAccepted)` ou `l.SetItems(...)` sur la boucle (`QueueUpdateDraw`). La sélection est gardée par `ID`.
- `ActionItem.Cells` sont du texte brut (échappé par la liste) ; `Detail` accepte les balises tview : **échapper** ce qui vient des agents ou des fichiers (`tview.Escape`).
- Confirmation : toujours pour `keys.Delete` (destructive), et pour toute action qui a `Confirm` ; consigne : toujours pour `keys.Refuse` (`ReasonOptional` l'autorise vide), ou avec `AskReason`.
- Les touches arrivent par la table (`InputCapture`) ; une vue qui garde les touches de son écran peut aussi appeler `l.HandleKey(ev)`. `StatusHints` peut renvoyer `l.Hints()`.
- Dans une fenêtre (fiche de checkpoint) : `Shell.Confirm` et `ShowInputModal` remplacent la fenêtre ; passer `Confirm` / `AskReason` qui rouvrent la fiche ensuite (comme `openCheckpoint` aujourd'hui).
- `cfg.Validate()` dans un test de la vue : refuse deux actions sur une touche, une touche réservée (`r`, `/`, `g`…), une action sans `Run`.

## Tests

- **Instantané du rendu** : `screenText(t, l.Primitive(), 80, 18)` puis `golden.Assert(t, "<nom>", ...)` (`internal/testutil/golden` ; fichiers `views/testdata/<nom>.golden`, le nom peut avoir des sous-dossiers). `go test ./internal/tui/v2/views -run <Test> -update` ne s'utilise que sur un test nouveau, le diff relu ; un golden existant ne se régénère pas pour faire passer un test. Le flag `-update` est déclaré par `golden` : un paquet de test ne déclare pas le sien. Exemples : `action_list_test.go`, `sessions_checkpoint_*_test.go` (`checkpoint_card/`). Voir [`tui-testing.md`](tui-testing.md).
- **`TestKeyConvention`** (`keys/convention_test.go`) lit les lignes d'aide (messages français dont la clé contient « hint », formats `fmt.Sprintf` du code TUI) et signale les touches hors convention. Il est strict : tout écart fait échouer le test (plus de liste d'écarts tolérés), la ligne d'aide est à corriger. Une ligne d'aide construite avec `keys.Hint` est conforme par construction.
- Parité i18n fr/en : `go test ./internal/i18n/...`.

## Fichiers partagés

`keys/keys.go`, `views/action_list.go`, `views/keys_help.go`, `shell/help.go` appartiennent à la fondation : un besoin nouveau (une action, une option du composant) se signale plutôt que de se coder en parallèle dans plusieurs lots. Les locales `fr.json` / `en.json` sont partagées : ajouter ses clés groupées sous le préfixe de sa vue.
