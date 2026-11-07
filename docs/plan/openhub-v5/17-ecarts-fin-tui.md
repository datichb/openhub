# Écarts — Finitions TUI (après fusion de toutes les pistes)

Branche `feat/v5-fin-tui` (worktree `../openhub-fin-tui`), base `main` `23e64493`. Trois commits, non fusionnés, non poussés.

## 1. Échap et toasts — `738ba65a`

**Fait** : dans `shell.globalKeyHandler`, la fermeture des toasts par Échap passe **après** les vues `InputCapturing` : dans la fiche de lancement et les assistants, Échap va directement à la vue (fermer la fiche, « Quitter l'assistant ? ») et les toasts disparaissent seuls ; ailleurs, Échap ferme d'abord les toasts, puis dépile (inchangé). Aide `?` (`tui.help.esc`, `tui.keys.help.esc_wizard`), `tui-usage`, référence TUI, `design/tui-shell.md`, CHANGELOG (entrée N19 complétée).

**Écarts** :
- La règle suit `InputCapturing`, donc s'applique aussi à l'**éditeur de workflow** et à la **vue de publication** (elles aussi capturent la saisie) ; c'est écrit dans la référence TUI. Le sélecteur Beads n'est pas une vue du routeur (il est dans la fiche).
- Si la vue capturante ne consomme pas Échap (liste déroulante ouverte), Échap va au widget, sans fermer les toasts.

**Tests** (SimulationScreen, vrai shell) : `TestKeysEscInLaunchFormKeepsTheToasts` (Échap ferme la fiche, toast gardé), `TestKeysEscInWizardKeepsTheToasts` (question affichée, toast gardé ; après Quitter, Échap referme les toasts) ; les deux échouent sans le correctif. `TestKeysEscClosesTheToastsFirst` inchangé et vert.

## 2. Bouton « Configurer le fournisseur » — `e0144134`

**Fait** :
- `views.ErrProviderCredentials` + `views.ProviderCredentialsError(msg)` ; `cmd.tuiLaunchError` marque ainsi `ProviderCredentialError` (vérification de R avant lancement) **et** le refus 422 du démon sur `/grants` (message traduit conservé).
- `LaunchFormConfig.ConfigureProvider` (branché sur `tuiShell.NavigateTo("provider")`) : quand le Récap ou le lancement échoue pour ces raisons, le Récap affiche le bouton **Configurer le fournisseur** (sélectionné, entre Retour et Lancer) et une ligne d'aide ; au retour sur la fiche (Échap depuis la vue Fournisseur → `Mount`), le Récap est recalculé (la vérification est relancée ; le cache `tuiRunCache` ne garde que les succès).
- i18n `tui.launch.configure_provider`, `tui.launch.configure_provider_hint` (fr/en) ; docs `providers`, `tui-usage`, référence TUI ; CHANGELOG (entrée N12).

**Écarts** :
- Ouvre la **vue Fournisseur** (pas l'assistant `oh provider setup`) : on y trouve `s` (assistant de configuration) et les réglages. La vue modifie la configuration du **hub** ; un fournisseur surchargé au niveau projet/équipe se règle ailleurs (Config Projet, équipe `K`).
- `NavigateTo("provider")` : si la vue Fournisseur était déjà plus bas dans la pile, le routeur y revient en dépilant la fiche (cas improbable : la fiche s'ouvre depuis l'accueil, le catalogue ou le board).
- Correctif de test en passant : `TestDecisionSettledElsewhereClosesItsWindow` était instable sous charge (le test lisait les toasts entre `CloseOverlay` et le toast) → attente des deux.

**Tests** : `views.TestLaunchFormConfigureProvider` (bouton affiché et sélectionné, appel, démontage/remontage → Récap recalculé, bouton disparu), `TestLaunchFormConfigureProviderOnlyForCredentials` (autre erreur ou pas de rappel → pas de bouton), `shell.TestKeysLaunchFormConfigureProvider` (vrai shell : Ctrl+S → bouton sélectionné, Entrée → vue `provider`, Échap → fiche, Récap revérifié), `cmd.TestTUILaunchErrorIsTranslated` (marquage 422 et `ProviderCredentialError`).

## 3. Région Bedrock vérifiée dès la saisie — `27cc17a5`

**Fait** :
- `provider.RegionProblem(region) (msg, invalid)` (`internal/provider/region.go`) : **même règle que G** (`credproxy.CheckRegion`, `credproxy.KnownBedrockRegion`), messages `cmd.proxy.region.invalid|unknown` existants ; `cmd.checkBedrockRegion` (G) s'appuie désormais dessus (une seule source).
- Formulaires de la TUI : `FormField.Validate` (champs texte) — message sous le champ mis à jour à chaque frappe (rouge ✗ refusé, jaune ⚠ avertissement, `views.ValidationText`), envoi refusé si invalide (toast, fenêtre gardée).
- Vue Fournisseur : la ligne `aws_region` s'édite dans un formulaire avec ce contrôle (au lieu de `ShowInputModal`) ; les deux saisies « Région personnalisée » (bearer, profil) aussi.
- `oh init` (étape fournisseur) et `oh provider setup` (étapes bearer/profil) : `addBedrockRegionDropDown` ajoute sous le champ « région personnalisée » une ligne de message mise à jour à la frappe ; la validation de l'étape (`bedrockRegionStepProblem`) refuse une région vide ou invalide.
- Docs `providers` (paragraphe région), `tui-usage`, référence TUI ; CHANGELOG (entrée N25 de G et ligne formulaires).

**Écarts** :
- **Correctif de comportement en passant** (`shell.ShowInlineForm`) : le bouton Confirmer fermait toujours la fenêtre **avant** la validation (le contrôle des champs obligatoires affichait un toast mais la saisie était perdue). Les boutons ne ferment plus d'office : `submitForm`/`cancelForm` ferment eux-mêmes ; un envoi refusé garde la fenêtre. Touche toutes les fenêtres de formulaire (amélioration, signalée au CHANGELOG).
- Une région connue choisie dans la liste déroulante n'a pas de message (toujours valide). Le contrôle ne retire pas les espaces (` eu-west-1` est refusé, comme au lancement).
- **Constaté, non corrigé** (hors périmètre) : dans la vue Fournisseur, les lignes `select`/`string` (dont `aws_region`, `auth_mode`, `aws_profile`) modifient la configuration en mémoire sans appeler `SaveConfig` ; la valeur sert pendant la session TUI mais n'est écrite dans `hub.toml` que par une autre action qui sauvegarde (assistant `s`, défaut). À trancher.

**Tests** : `provider.TestRegionProblem`, `views.TestProviderViewRegionCheckedAsTyped` (formulaire avec contrôle, invalide/inconnue/valide, valeur appliquée), `views.TestValidationText`, `shell.TestInlineFormValidateAsTyped` (SimulationScreen : message sous le champ à la frappe, Confirmer refusé → fenêtre gardée, correction → message effacé et envoi), `cmd.TestBedrockRegionCheckedAsTyped` (`bedrockRegionStepProblem`, champ personnalisé de `addBedrockRegionDropDown`), `cmd.TestCheckBedrockRegion` (G) vert.

## Vérifications (chaque commit)

`go build ./...`, `go vet -tags "integration e2e container" ./...`, `go test ./...`, `go test -race -count=1 ./internal/tui/...`, golangci-lint 2.14 (`--allow-parallel-runners`, tags integration/e2e/container) : 0 problème sur darwin, linux, windows. Pas de capture réelle (pty) faite dans cette passe.
