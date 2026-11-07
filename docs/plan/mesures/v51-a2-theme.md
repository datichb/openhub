# A2 — `theme`, table des symboles, garde de style (entrée de A4-A7 et A12)

> Récapitulatif du lot A2 (23/10/2026), fusionné sur `main`.

## API
`theme.ColorTagOf(tcell.Color)`, `Paint(c, text)` (hex ou `tcell.Color`), `BgTag(c)`, `StyleTag(attrs)` (`AttrBold`, `AttrItalic`, `AttrDim`, `AttrBlink`, `AttrStrike`, `AttrReverse`), `TagBg`, `TagAttrs` ; tout suit `NoColor()` (`StyleTag` vide aussi sous NO_COLOR). `widgets.ColorTag` délègue (53 appels non migrés).
Table : `theme.Sym*` = `glyph.*` (`Success` ✔, `Failure` ✗, `Inactive` ○, `Absent` –, `Warning` ⚠, `Active`/`SymLive` ●, `Current` ◆, `Modified` ✎, `Default` ★, `Cursor` ▸, `Selected` ◉) ; `glyph.Symbols` liste la table.

## Garde `TestUIStyleGuard`
`internal/archtest/ui_style_test.go`, liste `internal/archtest/testdata/ui_style_exceptions.txt` (`catégorie fichier compte`, ne fait que diminuer, le test affiche la ligne à écrire). Au départ : 108 lignes, 393 écarts, 67 fichiers — `tag-color` 126, `symbol` 172 (dont ~moitié `✓`), `tag-style` 37, `widgets-tag` 53, `color-literal` 5 (`board.go`).
Top : `workflow_editor` 37, `inline_wizard` 32, `publish_view` 22, `cmd/tui_team_columns` 20, `launch_form` 20, `workflows_view` 16, `beads_picker` 15, `mcp_view` 14, `gc_view` 12, `provider_view` 11, `teamboard_view` 11, `activity_view` 10, `policies_view` 9, `wizard` 9, `cmd/tui_remote` 8.
Risque : A4/A5/A6 éditent tous ce fichier (lignes triées par fichier, conflits d'adjacence possibles → fusions l'une après l'autre, rebase).

## Écarts `theme` / `glyph` à trancher (avant A4)
1. `IconSuccess` = `✓` (166 usages, préfixe « sélectionné » de huh) vs table `✔`.
2. `IconWarning` = `!` (112 usages) vs table `⚠`.
3. `IconDone` = `●` (étape terminée) alors que `●` = actif seulement ; terminé = `✔`.
4. `IconSkipped` = `◌`, déjà `RunSleeping` dans glyph (deux sens).
5. `RunFailed` = `⊗` alors que échec = `✗` (`✗` pris par `DecisionError`, `TestNoSymbolStandsForTwoThings`).
6. `○` = au repos (`RunIdle`) et désactivé / à venir ; nommé `Inactive`, `–` = absent.
7. « Sélectionné » divergent : `◉`/`○` (`inlineselect`), `☑`/`☐` (`shell.go` ×2), `●` (`beads_picker`), `[x]` (`sessions_fanout`), `✓` (huh). A2 a retenu `◉`.
8. Nommage : `theme.IconActive` = `◆` (courant) vs `glyph.Active` = `●`.

## Décisions du 23/10 (utilisateur) — appliquées en tête du lot A3
1. Réussite : **✔ partout** (`IconSuccess` ✓ → ✔, préfixe « sélectionné » de huh compris ; ✓ reste un écart de la garde jusqu'à disparition).
2. Avertissement : **⚠ partout** (`IconWarning` ! → ⚠) ; vérifier l'alignement des colonnes (largeur ambiguë : mesurer avec `runewidth`, pas `len`).
3. Étape terminée : **✔** (`IconDone` ● → ✔) ; ● = actif seulement.
4. Étape sautée : **–** (`IconSkipped` ◌ → –) ; ◌ réservé à « en veille ».
5. Session en échec : **⊗ gardé** ; la table documente ✗ (erreur ponctuelle, `DecisionError`) et ⊗ (arrêt en échec).
6. Sélection : **◉ / ○ choix unique, ☑ / ☐ choix multiple** ; `●` (`beads_picker`), `[x]` (`sessions_fanout`), `✓` (huh) migrés.
Goldens touchés : attendus (symboles), à lister et justifier un par un dans le récapitulatif.
