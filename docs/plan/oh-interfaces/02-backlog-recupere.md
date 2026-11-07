# Backlog repris des autres plans

> **Version qui traite chaque point** : voir le § 0 ci-dessous pour les restes de la 5.4, et [00-plan.md](00-plan.md) § 3 pour la feuille de route. Les phases citées plus bas (P0, L0, N1-x…) correspondent aux versions suivantes : P0 et L0 → 5.5 ; N1-0 et N1-1 → 5.6 ; N1-2, N1-3, N1-6 et N1-7 → 5.7 ; N1-4, N1-5 et N1-8 → 5.8 ; N2 → 5.9 ; N3 → 5.10 ; Web et App → 6.0 ; P4 → 6.1.

## 0. Restes de la 5.4 (`release-v5.4.md` § A, 05/11)

| Point | Version | Lot |
|---|---|---|
| SSH c. `HashKnownHosts` ; b. `GIT_SSH`, `core.sshCommand` | 5.5 | 55-ssh |
| SSH a. bastions (priorité basse) | au fil de l'eau | — |
| Espaces solo à l'accueil | 5.4.1 | 541-b |
| `ticket` sur une issue GitLab | 5.5 | 55-issue |
| Identité du projet dans le team-state | 5.5 | 55-teamstate |
| `brews` → `homebrew_casks` | 5.5 | 55-casks (vérifier Linux) |
| Linear dans `oh mcp setup` | 5.5 | 55-mcp (si D4 = oui) |
| MCP d'équipe à trois états, github et linear | 5.5 | 55-mcp |
| Garde-fou `ActiveTeam()` | 5.4.1 | 541-c (la liste est vidée en 5.6) |
| Le TUI lit les événements d'équipe | 5.6 | 56-0d (premier usage du bus) |
| `audit.finding` (jamais émis aujourd'hui) | 5.4.1 | 541-a |
| Titre « circuit » | 5.4.1 | 541-a |
| `slog.Warn` brut, plus de 5 messages | 5.4.1 | 541-a |
| Store en mémoire du centre de notifications | 5.8 | 58-transverse |
| Rotation concurrente de `notifications.jsonl` | 5.4.1 | 541-a |
| Détection du vrai premier plan du TUI | 5.5 (ADR-062), puis 5.6 (contrat) et 5.10 (serveur) | présence et focus des clients |
| Moins d'allers-retours git au lancement et pour le verdict MCP | 5.5 (API groupée), puis 5.7 (`LaunchDraft`) | 55-teamstate, 57-launch |
| `oh doctor --fix` pour les réservations de membres partis | 5.5 | 55-doctor-claims |
| CI Windows | au fil de l'eau | — |
| Écritures non atomiques | 5.5 (team-state, board, config, si D5) ; le reste au fil de l'eau | 55-atomic |
| Sans date : interface fine du sous-domaine workflow de `teamstate` | 5.5 | 55-teamstate |
| Sans date : fonctions du shell de plus de 80 lignes | 5.8 (en option) | 58-transverse |
| Sans date : isolation stricte, `test-gen`/`db-migration` | au fil de l'eau | — |
| **Sécurité : `GH_TOKEN` absent de `envfilter.hardDenied`** (session F) | **5.4.1, en premier** | 541-e (avec la vérification des autres noms de jetons) |
| `oh mcp status` ignore jira, github et linear, et n'accepte pas de service (session F) | 5.5 | 55-mcp |
| `GITHUB_URL` et `LINEAR_URL` posées mais lues par aucun serveur (session F) | 5.5 | 55-mcp |
| Service `enabled = true` sans `token_key` écarté sans message (session F) | 5.5 | 55-mcp |

Source : relecture de `oh-modernisation-plan.md`, `simplification-tests.md`, `release-v5.0/5.1/5.3/5.4.md`, `oh-tui-*.md`, `mesures/`, `openhub-v5/*`, et dans le dépôt `docs/dev/audit-v5.md`, `tui-*.md`, `docs/design/tui-shell.md`, ainsi que du tracker Beads. Vérifié dans le code sur `main` = `fe0bd908` (05/11/2026).

Légende : ✅ repris dans le plan · 🔀 à décider · ⛔ hors plan, à faire en parallèle (nettoyage indépendant) · ✔ déjà fait, ne pas replanifier.

## 1. Prérequis, à faire avant N1 (phase P0)

| # | Point | Source | Pourquoi c'est un prérequis |
|---|---|---|---|
| P0-1 | ✅ **Terminer la v5.4** : le lot L (cycle de vie) est encore ouvert dans `../openhub-wt-v54-l` | `release-v5.4.md` | Le lot L touche `tui_lifecycle`, le shell et les sessions : démarrer N1 avant sa fusion garantit des conflits |
| P0-2 | ✅ **`teamstate` : interfaces fines et `ctx` partout** : `workflowsvc.TeamState.Repo` et `launch.Deps.TeamRepo` sont des types concrets ; `RepoInfo`, `IsStale`, la file des claims et la lecture des workflows n'ont pas de `ctx` | `release-v5.1.md:483` (B10), `v5.3:206`, `v5.4:207,239` ; `services/workflow/teamstate.go:19`, `services/launch/remote.go:125`, `teamstate/interfaces.go:25` ; `archtest/testdata/teamstate_repo_exceptions.txt` (5 fichiers) | Le contrat impose `ctx` et `Scope` : sans ça, `appsvc` hérite des types concrets |
| P0-3 | ✅ **Le flux de `ohd` (`/v1/stream`) perd des événements** : pas de numéro de séquence ni de reprise, avec un repli par relecture toutes les 2 s | `openhub-v5/07-phase-3.md:111` ; `daemon/stream.go:62`, `services/session/live.go:24` | Base du flux d'événements unique (règle 5, N1-0) et du SSE avec reprise (N3) |
| P0-4 | ✅ **Mesures de référence** : `make test-durations` sur `./cmd` et `./internal/tui/...`, temps de démarrage du TUI, couverture | `simplification-tests.md` | Pour vérifier que le refactoring ne ralentit rien (et accélère `cmd`, aujourd'hui 378 s sous `-race`) |
| P0-5 | ✅ **Réparer Beads** : `bd` échoue (`dolt_database` = `beads_opencode-hub`, le trait d'union est refusé) ; `.beads` est en 0750 | analyse du tracker | Indispensable pour créer les tickets du plan |
| P0-6 | ✅ **Nettoyer le tracker** : les 4 issues ouvertes datent de l'époque bash (mai 2026) | `opencode-hub-mep`, `mep.2.1`, `mep.2.3` (dashboard bash), `xnj` | `mep.2.3` (dashboard) est caduc et recoupe N3/Web ; à fermer avec une note |

## 2. L0 : cadrage et garde-fous

| Point | Source | Décision |
|---|---|---|
| ✅ Élargir le détecteur de `team_resolution_test.go` à tous les appels `.ActiveTeam()` (aujourd'hui seul `<x>.Config.ActiveTeam()` est vu ; non vus : `tui_views_project.go` ×2, `tui_team_board.go` ×2, `tui_catalog.go`, `views/*`, `shell/modes.go`) | `release-v5.4.md:241` (§A.7) | L0, liste gelée ; vidée en N1-0 et N1-8 |
| ✅ `messages_literal_test` ne parcourt pas `shell/` ni `widgets/` (environ 100 libellés d'un mot, `AddInputField`/`AddDropDown` non surveillés) | `release-v5.1.md:470` (F7) | L0 : l'étendre, avec une liste gelée |
| ✅ La faille des tags de build dans `archtest` (A-03) | `simplification-tests.md`, `audit-v5` | L0 |
| ✅ Exceptions de couches existantes : `views/secrets_view.go → storage/keychain`, `launcher/ui_cli.go → tui/theme` | `archtest/layers_test.go:75,90` | Gelées ; vidées en N1-7 (secrets) et N2 (`ui_cli`) |
| ✔ Listes `ui_style` et `tui_keys` vides | `release-v5.1.md:482` (A12) | Modèle à suivre pour les nouvelles règles |

## 3. N1 : par domaine

| Domaine | Point | Source |
|---|---|---|
| N1-0 socle | ✅ **`ResolveTeam` (métier) lit `tuiShell.Mode()` et `ActiveTeam()`** : `team_resolve.go:89-109,260`, utilisé dans 16 fichiers | inventaire |
| N1-0 socle | ✅ `tuiContext()` est un contexte implicite (13 appels dans 6 fichiers) | `release-v5.1.md:483` |
| N1-0 socle | ✅ BL-6 « `Capabilities()` partielles gérées par l'UI » | `openhub-v5/12-backlog-adr.md:12` |
| N1-1 Sessions | ✅ Repli « branche courante » de l'enchaînement à déplacer dans les services (`tui_chain.go`) | `openhub-v5/05-phase-1.md:174` |
| N1-1 Sessions | 🔀 `--headless` : fragments de texte dans le désordre (`AssistantText`) — ambigu, à revérifier | `openhub-v5/06-phase-2.md:197` |
| N1-1 Sessions | ✅ BL-9 : suivi en direct d'une session distante — **seulement prévu dans le contrat** (champ `Remote`, événement), l'implémentation reste hors plan | `12-backlog-adr.md:15` |
| N1-2 Lancement | ✅ Regroupement par catégorie avec un libellé traduit dans `cmd` (`tui_start.go:149`) à déplacer dans les services | `05-phase-1.md:174` |
| N1-2 Lancement | ✅ Restent dans `cmd` : affichage du récapitulatif, `afterStart`, `startRun`, crochets `Deps` ; `launch.Deps.TeamRepo` concret | `release-v5.1.md:424` (C6) |
| N1-2 Lancement | ✅ `tuiRunCache` ne garde que les succès (état caché) | `17-ecarts-fin-tui.md:19` |
| N1-2 Lancement | ✅ DC26 : la mention « aucun verrou de commit » de `libre` absente de la fiche et du récapitulatif (seulement dans le prompt) : en faire une donnée du contrat | `17-ecarts-W.md:146` |
| N1-3 Projets | ✅ `projectRemover`, `Renamer` et `Mover` appellent le store directement, en double avec `project_remove.go` | inventaire |
| N1-3 Projets | 🔀 Sélecteur de projet `huh` avant le TUI (`start_helpers.go`) | `17-ecarts-X.md:54` |
| N1-4 Équipe | ✅ `actionTrackerDiscovery` (453 l.), `buildInitWizardRejoinSteps` (309 l.) | `audit-v5` A-01 |
| N1-4 Équipe | ✅ `teamstate` importé par 82 fichiers de `cmd` ; `teamstate_repo_exceptions.txt` (team.go ×3, launch_deps, workflow_layers, workflow_doctor, brick_doctor) ; `activeTeamAllowed` (init_wizard, team_identity, history, root) | `audit-v5`, `archtest` |
| N1-4 Équipe | ✅ **Regroupement du board écrit deux fois** (`serve_api.go:51-121` et `views/board_data.go:67-171`) ; séquence de prise de ticket répétée 3 fois | inventaire |
| N1-4 Équipe | ✅ Détection de conflit absente sur les claims et le board (elle existe pour `config.toml` depuis la v5.3.3) | `00-plan` §7, `17-critiques.md:346` (N15) |
| N1-4 Équipe | ⛔ `mcp.<svc>` : booléen à trois états sans retour à « hérité » ; github et linear absents du MCP d'équipe (fonctionnel) | `release-v5.4.md:240` |
| N1-5 Wiki | ✔ ou 🔀 Wiki accepté ou refusé : notifié en CLI mais pas dans le TUI — le lot N de la v5.4 (fusionné) l'a probablement traité, **à revérifier** | `release-v5.4.md:149` |
| N1-6 Workflows | ✅ BL-1 : éditeur de prompt intégré (aujourd'hui `$EDITOR`, `workflow_editor_fields.go:34`) — capacité `Editor` du client ; un éditeur dans le web plus tard | `12-backlog-adr.md:7` |
| N1-6 Workflows | ✅ `longHints` : `tui.editor.hints.graph`, `tui.catalog.edit.hints` | `keys/convention_test.go:334` |
| N1-6 Workflows | ⛔ `TestBrickCLI` environ 1 s par vérification (`mergeBricks` copie tout le hub, `services/workflow/bricks.go:171`) | `simplification-tests.md:43` |
| N1-7 Administration | ✅ `services/doctor` : seuls les contrôles machine sont migrés, il reste 32 fichiers `cmd/doctor_*.go` ; globales `views.DoctorChecks` et `DoctorOffline` | `release-v5.1.md:429` (C7) |
| N1-7 Administration | ✅ Secrets : règles de repli écrites deux fois ; écritures dans le trousseau sur la boucle d'affichage ; exception `archtest` | inventaire |
| N1-7 Administration | ✅ Modèles : `config.Update` et `Projects.Update` sur la boucle d'affichage (`models_view.go:455-566`) | inventaire |
| N1-7 Administration | ✅ Politiques : contexte git écrit deux fois (`policies.go:153-216` et `policies_view.go:810-867`) | inventaire |
| N1-7 Administration | ✅ Fournisseur : la vue ne règle que le hub ; un fournisseur surchargé au niveau projet ou équipe se règle ailleurs (choix à rendre explicite par `Scope`) | `17-ecarts-fin-tui.md:23-24` |
| N1-7 Administration | ✅ Métriques : pas de tableau par projet, pas de `--team` ; `platform.StatsProvider` à reprendre dans `appsvc/metrics` | `release-v5.1.md:477`, `03-architecture.md:107` |
| N1-8 Transverse | ✅ `SetLayoutLevel` global (une seule TUI par processus) | `17-ecarts-X.md:44` |
| N1-8 Transverse | ✅ Notifications : trois systèmes (système via le démon, webhooks, messages du TUI) ; `NotificationsProvider` et le store en mémoire (200 entrées) peut-être devenus du code mort — **à revérifier après le lot N de la v5.4** | `release-v5.4.md:144,157` |
| N1-8 Transverse | ✅ `formatEvent` : textes en français en dur (en partie remplacés par `glyph.Events`) | `release-v5.1.md:450` |
| N1-8 Transverse | 🔀 Fonctions du shell de plus de 80 lignes : `showHelpOverlay` (126), `buildModalFrame` (103), `updateSuggestions` (95), `showToast` (91) — facultatif | `release-v5.1.md:363`, `v5.4:207,239` |
| N1-8 Transverse | ⛔ Dernier projet de Ctrl+T non conservé ; pages du hub sans `AutoSaver` ni indicateur | `release-v5.1.md:475` (B7) |
| Tous | ✅ Familles de tests non fusionnées (`launch_form*`, `sessions_*`, `beads_picker`, `shell/keys_test`, `cmd/tui_*`, équipes) : à regrouper au moment où les tests déménagent vers `appsvc` | `simplification-tests.md:41` |

## 4. N2 : formulaires

| Point | Source | Décision |
|---|---|---|
| 🔀 **`huh` encore utilisé par la CLI** (`cmd/{root, run_existing, provider, start_helpers, confirm, run, project_configure, mcp, session_resolve, worktree}.go`, `launcher/ui_cli.go`, `tui/theme/huh.go`, `tui/components/floating`) : deux bibliothèques de formulaires, « dette assumée » | `audit-v5` A-05, `10-tui.md:330` | **À décider** : passer aussi les confirmations et sélecteurs de la CLI par `ui/forms`, ce qui supprime `huh` (recommandé, environ 1 à 2 semaines de plus) |
| ✅ Anciens paquets hors `v2` : `tui/progress` (2 importeurs), `tui/common` (4), `tui/components/summary` (2), `tui/components/floating` (1, `huh`) | inventaire | À supprimer ou à migrer en N2 |
| ✅ « Nouveau workflow » : mise en page du formulaire, champ Identifiant à faire défiler | `16-corrections.md:253` (A39), `15-tests-manuels.md:336` (A34) | Corrigé de fait par le schéma N2 |
| ✅ `runProjectAddInteractive` (299 l.) | `audit-v5` A-01 | N2 (assistant d'ajout de projet) |
| ✅ Environ 100 libellés d'un mot (`InfoField`, en-têtes) | `release-v5.1.md:470` (F7) | Deviennent des clés du schéma |
| ✅ `LoadConfig` appelé dans `WizardStep.Form` (I/O au montage, `tui_team_actions.go:85`) | inventaire | Effet côté serveur dans la machine à états |
| 🔀 `ShowScrollableModal` : Entrée ne valide pas tant que le focus est sur le texte | `06-phase-2.md:195` | Convention de touches du moteur tview |
| ✔ `RunWizard` et `layout.Build` supprimés ; `renderStep` 337 → 26 l. ; `ShowInlineForm` 437 → 12 l. | `release-v5.1.md:299-302` | Ne pas replanifier |

## 5. N3 : serveur d'interface

| Point | Source | Décision |
|---|---|---|
| ✅ BL-3 / D17 : « `oh serve` expose les services : décisions, suivi, propositions » | `12-backlog-adr.md:9`, `01-decisions.md:25` | C'est exactement N3 ; la décision D17 est reprise |
| ✅ `oh dashboard` et `oh metrics` dépréciés, retrait prévu en v6 ; `status`, `serve`, `history` et `metrics` en texte jusqu'en v6 | `audit-v5` F-11, `release-v5.1.md:51` | Retrait en même temps que le lancement du web (voir le jalonnement en v6) |
| ✅ `oh board --watch` caché et sans effet (`cmd/board.go:36`) ; premier Échap de `oh dashboard` absorbé par le message | `release-v5.1.md:477` | À supprimer en N3 |
| ✔ S-36 (DNS rebinding sur `oh serve`) corrigé (`serve_auth.go:35-65`) | `17-critiques.md:458` | À reprendre dans `uiserver` |

## 6. App et P4

| Point | Source | Phase |
|---|---|---|
| ✅ BL-11 : boutons valider/refuser dans la notification ; BL-22 : notificateur macOS signé | `12-backlog-adr.md:17,28` | App (notifications natives) |
| ✅ `brews` → `homebrew_casks` (goreleaser l'annonce comme déprécié) | `release-v5.1.md:390`, `v5.3:200`, `v5.4:237`, `audit-v5:356` | P4, **prérequis du cask de l'app** ; peut être fait avant |
| ⛔ 20 paquets exclus de la CI Windows, dont `cmd`, `daemon`, `teamstate`, `services/session`, `services/workflow` ; recette Windows réelle jamais faite | `audit-v5` N-2, `00-INDEX.md:204` | Indépendant ; à rappeler en N3 (mode direct sous Windows) |

## 7. Revue d'ergonomie et de design du TUI : 🔀 à décider

Les points A32 à A42 (`15-tests-manuels.md:350`, `16-corrections.md:4,11`, `17-critiques.md:5`) n'ont jamais été planifiés :
- descriptions coupées sans « … », curseur parasite ;
- aide `?` trop étroite en 64 colonnes ;
- en-tête du projet qui coupe le chemin en mode compact ;
- « Patterns », « Policies » et « Sync Tracker » non traduits en français ;
- titre du catalogue recouvert (`SectionedList`).

**Proposition** : ne pas la faire avant N1, puisque N1 sort justement l'affichage du branchement, et que les vues seront plus simples à retoucher après. On la fait **après N1, domaine par domaine**, et elle **alimente les maquettes du web** (P0 de la phase Web). Les petits correctifs de traduction (« Patterns »…) peuvent partir tout de suite comme nettoyage indépendant.

## 8. Nettoyage indépendant (⛔ hors plan, à faire en parallèle si besoin)

- **108 `os.WriteFile` non atomiques** (`release-v5.1.md:391`, « au fil de l'eau »). Le web écrira en même temps que le TUI : prioriser ceux du team-state, du board et de la config **avant N3**.
- **Retraits de la v6** : clés anciennes (`config/legacy_keys.go`), commandes retirées (`removed_commands.go`), environ 45 tests du code de migration, cascades de configuration (A-08).
- **C9** : environ 32 icônes et un `tabwriter` encore dans 3 fichiers (`team_rejoin_cli` 20, `project_add` 8, `team_wiki` 5…).
- `TestOhGC` laisse un dossier dans `$TMPDIR` (à revérifier) ; segfaults de `git` en fan-out sous forte charge.
- Petits points d'interface : traductions manquantes, `SectionedList`, aide en 64 colonnes (voir § 7).
- Docs : fermer ou archiver `openhub-v5/18-recette-critiques.md` (§9 toujours ☐, alors que 9.12 est corrigé) ; dérouler `oh-tui-recette.md`, qui servira de **recette de non-régression** à la fin de N1 et de N2.

## 9. Déjà fait, à ne pas replanifier

- `services/{team, launch, policy, doctor}` et `services/session/history.go` existent.
- `daemon/api` et `daemon/client` existent, et `compat.go` a été supprimé.
- `app.App` a été élargi ; 21 `config.Save` sont passés en `config.Update`.
- La synchronisation du tracker n'est plus écrite deux fois (`teamsvc.SyncTracker`).
- `RunWizard`, `layout.Build`, `widgets.ColorTag` et `FilterableList` ont été supprimés.
- Il n'y a plus de `go` nu (`safego` partout).
- Le modèle de statut est unique (BL-25, F-02) ; Ctrl+T ouvre le mode projet (BL-24) ; les sessions des autres membres sont visibles (`team_sessions.go`).
- Garde-fou U-11 (chaînes non traduites) ; S-36 corrigé.
