# Conventions de la TUI : touches et liste d'actions

Guide des vues de la TUI (lots `tui-keys` à `tui-docs`). La convention de touches est décidée (esquisses du 15/10) ; la référence utilisateur est le tableau « Convention des touches » de [`docs/reference/tui.fr.md`](../reference/tui.fr.md#convention-des-touches).

## La convention en bref

| Contexte | Touches |
|----------|---------|
| Partout | `↑↓`/`jk` déplacer, `g`/`G` premier/dernier, `/` filtrer, `r` rafraîchir, `?` aide, `Entrée` ouvrir, `Échap` revenir, `Ctrl+S` enregistrer (`keys.Save`) |
| Liste | `n` créer, `e` éditer, `d` supprimer (toujours confirmé), `x` archiver/classer (jamais destructif), `R` reprendre/restaurer, `c` comparer (diff), `Espace` cocher/basculer (`keys.Toggle`) |
| Décision | `y` valider/accepter, `n` refuser (consigne, confirmé), `e` modifier, `s` laisser en attente, `A` tout accepter |
| Lecteur | `/` rechercher, `n`/`N` résultat suivant/précédent |

Règles :

- une vue peut donner une lettre libre à une action à elle (`a` attacher, `t` transférer, `p` relancer le pilote, `B` lot…) ;
- elle ne donne **jamais** un autre sens à `g G / r ? d x R y n` (touches strictes), ni une autre touche à une action de la convention (`a ajouter` → `n créer`, `r restaurer` → `R`, `D diff` → `c`) ;
- `e`, `s`, `A` et `c` peuvent porter une action propre à la vue quand le contexte n'a pas l'action de la convention (`e enchaîner` dans Sessions) ;
- `d` passe toujours par une confirmation (`ShellAccess.Confirm`, `danger`) ; `x` ne détruit rien (ce qui est classé se restaure avec `R`) ;
- `n` refuser demande une consigne : la consigne validée vaut confirmation ;
- `w` n'a qu'un sens (ouvrir dans le navigateur, Sessions) : enregistrer est `Ctrl+S` (`keys.Save`). Les pages Hub et Projet enregistrent seules après `views.SaveDelay` (500 ms) et le disent dans le titre (`saveBadge`) ; les pages d'équipe publient dans le team-state sur `Ctrl+S` et disent « non publié — Ctrl+S » jusque-là ;
- `Ctrl+T` fait le tour Hub → Projet → Équipe → Hub (`Shell.cycleMode`), en sautant ce qui n'est pas configuré ; il est annoncé dans `keys.Changes()`.

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

`keys.Own` panique sur une touche stricte de la convention (`g G / r ? d x R y n`) ou un déplacement (`j k`) ; `TestOwnActions` (`keys/own_test.go`) vérifie que chaque action déclarée est traduite, a un identifiant unique et que son libellé ne reprend pas une action de la convention sur une autre touche (« a ajouter », « f filtrer »). Actions propres déclarées : `a` attacher, `m` consigne et `i` interrompre (Sessions), `p` relancer le pilote et `B` lot (fan-out), `C` confirmer le lot, `T` découvrir le tracker, `L` libérer un ticket, `M` nettoyer les worktrees fusionnés, `Y` YAML brut, `f` corriger et `o` hors ligne (Doctor), `m` déplacer, `p` vue Projet et `b` initialiser Beads (Projets), `o` ouvrir et `p` nettoyer les références (Worktrees), `{` `}` sections d'une `SectionedList` et `h` `l` à gauche/à droite dans le graphe d'un workflow (widgets).

Un changement de touche visible s'ajoute à `keys.Changes()` (vue, avant, après, libellé `tui.keys.change.<vue>.<action>`) : le message « Nouvelles touches » et la référence le montrent. `keys.ChangesRevision()` est l'empreinte de la table : elle change quand une ligne change, et le message est alors montré de nouveau (la préférence `tui.keys_notice` garde la dernière empreinte vue). Une touche qui change sans ligne dans `Changes()` est un oubli : l'utilisateur ne l'apprend nulle part.

### Une touche passe toujours par `keys`

Ni une vue ni un widget ne lit la lettre d'un événement : pas de `switch ev.Rune() { case 'j': … }`, pas de `ev.Rune() == ' '`. Il demande une action de la convention (`keys.Down.Matches(ev)` couvre `↓` et `j`, `keys.Toggle.Matches(ev)` l'espace, `keys.Open` Entrée) ou, pour une touche qui n'est qu'à lui, la déclare avec `keys.Own` en variable du paquet (`graphLeftKey = keys.Own("graph.left", 'h', "a12.keys.graph_left")`) et appelle son `Matches`. Le libellé d'une action propre est une clé i18n, même quand rien ne l'affiche : `TestOwnActions` la vérifie. Une ligne d'aide se construit avec `.Hint()` de la même action, jamais `"p "+i18n.T(…)`, pour que la touche affichée et la touche lue ne puissent pas diverger.

`TestTUIKeysGoThroughKeys` (`internal/archtest/tui_keys_test.go`) analyse le code de production de `internal/tui/**` (hors `keys/`, où les lettres sont liées) et de `cmd/tui_*.go` : un `switch` sur le rune d'un événement qui a un `case '<lettre>'`, ou une comparaison `ev.Rune() == '<lettre>'` (même quand le rune a été lu dans une variable), fait échouer le test. Sa liste d'exceptions, `internal/archtest/testdata/tui_keys_exceptions.txt`, est **vide** : elle ne fait que diminuer, avec le nombre exact par fichier (un compte qui baisse sans que la ligne soit mise à jour échoue aussi). Une touche qui ne rentre pas dans la convention se déclare avec `keys.Own` ; une lettre stricte (`g G / r ? d x R y n`, `j k`) fait paniquer `keys.Own` : c'est la convention qui doit répondre (une action existante, ou une action de plus dans `keys.All()` avec son libellé et sa description traduits).

Les modificateurs : `Matches` refuse une lettre avec `Ctrl` ou `Alt` (`Alt+j` n'est pas `j`).

Actions : `Up`, `Down`, `First`, `Last`, `Navigate` (↑↓/jk), `Ends` (g/G), `Filter`, `Refresh`, `Help`, `Open`, `Back`, `Save` ; `Create`, `Edit`, `Delete`, `Archive`, `Resume`, `Compare`, `Toggle` (Espace) ; `Accept`, `Refuse`, `Modify`, `Leave`, `AcceptAll` ; `Search`, `NextMatch`, `PrevMatch`. `Action.Confirm` (d, n) et `Action.Reason` (n) disent ce que l'action exige.

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

## Couleurs, symboles et textes

- Une couleur, une balise tview ou un style vient de `theme/helpers.go` : `theme.Paint(theme.SuccessHex, texte)` colore un texte et rend la couleur ; `theme.StyleTag(theme.AttrBold)` / `theme.TagAttrs` pour le gras ; `theme.BgTag(couleur)` / `theme.TagBg` pour un fond ; `theme.ColorTag(hex)` et `theme.ColorTagOf(couleur)` quand le texte est assemblé en plusieurs morceaux. Tous suivent `NO_COLOR` (sans couleur : pas de balise, `Paint` rend le texte seul). Jamais `"[#64a0ff]"`, `"[::b]"` ni `tcell.ColorRed` dans une vue.
- Un symbole de la table des sens vient de `theme.Sym*` (`SymSuccess` ✔, `SymFailure` ✗, `SymWarning` ⚠, `SymInactive` ○, `SymAbsent` –, `SymLive` ●, `SymCurrent` ◆, `SymModified` ✎, `SymDefault` ★, `SymCursor` ▸, `SymSelected` ◉, `SymSelectedMulti` ☑, `SymUnselectedMulti` ☐), jamais d'un littéral : `✓` est interdit (c'est `✔`). Un symbole veut dire une seule chose ; la table est dans [`aurum.md`](../design/aurum.md#table-des-sens-themeiconsgo-glyphsymbolsgo) et l'aide (`?`) en montre la légende, produite par `theme.Meanings`. Ajouter un symbole : une ligne dans `glyph.Symbols`, une dans `theme.Meanings` (`TestMeaningsAreTheGlyphTable` les rapproche), un libellé `a12.legend.<nom>` en français et en anglais.
- Une date, une durée, un coût, une taille ou une coupure de texte vient de `textfmt`. Une colonne se mesure avec `textfmt.ClipWidth`, pas avec `len`.
- Un texte qui ne vient pas du code est échappé (`tview.Escape`) ; le texte donné à `tview.Escape` est hors de la garde (`tview.Escape("[" + nom + "]")` n'est pas une balise).
- `TestUIStyleGuard` (`internal/archtest/ui_style_test.go`) fait respecter tout cela dans `internal/tui/**` (le thème excepté, `theme/icons.go` pour les symboles) et `cmd/tui_*.go`. Quatre genres d'écart : `tag-color`, `tag-style`, `color-literal`, `symbol`. Sa liste, `testdata/ui_style_exceptions.txt`, est **vide** et ne peut que diminuer ; on n'y ajoute pas de ligne, on corrige le code.

## Lire et écrire hors de la boucle

La boucle d'événements de tview ne fait ni E/S ni attente. Une vue lit avec `load` (`views/async.go`) :

```go
load(app, v.shell.Context(), &v.slot, func(ctx context.Context) ([]Row, error) {
	return v.cfg.ReadRows(ctx) // hors boucle : ne touche pas à la vue
}, func(rows []Row, err error) {
	// sur la boucle, et seulement si c'est la dernière lecture de la vue
	if err != nil { setState(v.text, stateError, err.Error()); return }
	v.render(rows)
})
```

- Ce que `fetch` doit savoir de la vue (mode, période, sélection) est copié **avant** l'appel, sur la boucle.
- `loadSlot` numérote les lectures : la plus récente gagne ; `Unmount` appelle `slot.Invalidate()`. Une panique dans `fetch` est récupérée et donnée à `apply` comme erreur. `syncLoad` tire d'abord le team-state (toast si le pull dépasse 1 s) puis lit comme `load` ; `everyOnLoop` ne fait que lancer une lecture à intervalle.
- Une écriture passe par `persist` : elle n'est ni abandonnée par une écriture plus récente ni par un `Unmount`, et son contexte n'est pas celui du shell. Les pages Hub/Projet l'utilisent avec `views.SaveDelay`.
- Ce qu'une vue dit sans données (chargement, vide, erreur) s'écrit avec `stateText(kind, msg)` / `stateView` / `setState` (`stateLoading`, `stateEmpty`, `stateError`, clés `tui.state.*`) : mêmes mots et mêmes couleurs partout ; l'erreur est en ✗ rouge. `ActionList.SetEmpty(kind, msg)` fait de même pour une liste.
- Toute goroutine et tout minuteur passent par `safego` (`safego.Go`, `safego.AfterFunc`) ; la garde `TestNoBareGoInCriticalPackages` couvre tout `cmd/` et `internal/` (sauf `safego`), y compris `errgroup.Go` / `sync.WaitGroup.Go` (`g.Go(safego.WrapErr(fn))`, `wg.Go(safego.Wrap(fn))`) et `time.AfterFunc`. Une panique rattrapée écrit `~/.oh/run/crash-<date>.log` (`internal/crashlog`) et le toast en donne le chemin ; un `recover` écrit à la main appelle `safego.LogRecovered`.

## Le composant `ActionList`

`views.ActionList` (`cli/internal/tui/v2/views/action_list.go`) : liste d'éléments sur une ligne (colonnes alignées, sections facultatives, état), détail sous la liste (`DetailBelow`, défaut) ou à côté (`DetailRight`), actions sur l'élément sélectionné et actions globales, sans limite d'éléments (la liste défile, le pied indique « 2/9 · 2 de plus ↓ »). Il sert aux propositions wiki (fiche, file du projet), à la validation par lot, aux enfants du fan-out, aux briques.

```go
l := views.NewActionList(views.ActionListConfig{
	Title:       i18n.Tf("tui.cpwiki.title", cp, len(list)),
	Layout:      views.DetailBelow,
	StateLabels: map[views.ItemState]string{views.ItemAccepted: theme.SymSuccess + " " + i18n.T("tui.wiki.accepted_f")},
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
- **`TestUIStyleGuard`** et **`TestTUIKeysGoThroughKeys`** (`internal/archtest`) : couleurs, balises, symboles et touches (voir plus haut). Leurs listes d'exceptions sont vides ; elles ne font que diminuer.
- Parité i18n fr/en : `go test ./internal/i18n/...`. Les clés d'un lot se groupent sous un préfixe (celui de la vue, ou celui du lot dans les fichiers partagés).

## Fichiers partagés

`keys/keys.go`, `views/action_list.go`, `views/keys_help.go`, `shell/help.go` appartiennent à la fondation : un besoin nouveau (une action, une option du composant) se signale plutôt que de se coder en parallèle dans plusieurs lots. Les locales `fr.json` / `en.json` sont partagées : ajouter ses clés groupées sous le préfixe de sa vue.
