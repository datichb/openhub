# Plan détaillé du refactoring : P0, L0, N1, N2, N3

> **Document de référence.** La forme à jour, à utiliser en session, se trouve dans les plans de version ([versions/](versions/)). Correspondance : P0 et L0 → 5.5 ; N1-0 et N1-1 → 5.6 ; vagues B et C → 5.7 ; vagues D et E → 5.8 ; N2 → 5.9 ; N3 → 5.10. **La grille du § 6 ci-dessous est remplacée par [00-plan.md](00-plan.md) § 3** (tout est décalé d'une version pour faire place à la 5.4.1 et aux restes de la 5.4).

S'appuie sur [01-etat-des-lieux.md](01-etat-des-lieux.md) (inventaire avec fichier:ligne) et [02-backlog-recupere.md](02-backlog-recupere.md) (points repris des autres plans). Les ADR de référence sont 061, 062 et 063.

---

## 0. Règles d'exécution (reprises de `release-v5.4.md` § R, toutes valables)

- **Un lot = un worktree** `../openhub-wt-ui-<lot>` et une branche `refactor/ui-<lot>`. Au plus 3 lots en parallèle, et **jamais deux lots qui touchent les mêmes fichiers** (voir la matrice du § 5).
- **Commits** en anglais, `refactor:` / `test:` / `feat:` / `docs:` avec le suffixe de version (par exemple ` (v5.5)`), un sujet par commit, chacun compile et passe le lint.
- **Interdits pour les sessions** : aucun `git push`, tag ou MR ; aucune éval, aucun e2e, aucun appel de modèle ; aucun appel réseau réel ; ne pas toucher `~/.oh`, `~/.config/opencode` ni la config git globale.
- **Sécurité du trousseau** : dans tout test qui lance git, `GIT_CONFIG_NOSYSTEM=1`, `GIT_CONFIG_GLOBAL` temporaire, `HOME` temporaire. Contrôle `security dump-keychain … | grep -c 127.0.0.1` = 0 avant chaque récapitulatif et après chaque fusion.
- **Tests d'un lot** : ses paquets en `-race`, plus **toujours** `./internal/archtest/...` et `./internal/i18n/...`, plus `make sync-content && go test -short ./cmd/`, plus `GOOS=windows go vet`.
- **Goldens jamais régénérés pour faire passer un test.** Toute régénération est listée et justifiée dans le récapitulatif du lot.
- **Écran simulé** : lu sur la boucle d'événements ; après `SetSize`, poster `NewEventResize` puis `QueueUpdateDraw`.
- **Fusion** : `git merge --no-ff`, une branche à la fois, puis build, vet (y compris Windows), archtest, i18n, `-race` des paquets du lot, `-short ./cmd/`, trousseau. **En fin de vague** : `go test -race -timeout 40m ./...`, lint du module, `windows-packages.sh check`.
- **Pas de changement visible pour l'utilisateur en N1** : le TUI et la CLI doivent se comporter à l'identique. Une différence voulue est une décision, notée dans le récapitulatif.

---

## 1. P0 : prérequis (2 à 3 semaines)

| # | Tâche | Détail | Fin |
|---|---|---|---|
| P0-1 | Terminer la v5.4 | Fusionner le lot L, publier ; **aucun lot N1 avant** | `feat/v54-l` fusionné, worktree supprimé |
| P0-2 | `teamstate` : interfaces fines et `ctx` | `ctx` ajouté à `RepoInfo`, `IsStale`, la file des claims et la lecture des workflows ; interfaces fines à la place de `*teamstate.Repo` dans `workflowsvc.TeamState`, `launch.Deps.TeamRepo` et `launch.TeamClaims` | `teamstate_repo_exceptions.txt` raccourci ; plus aucun `*teamstate.Repo` dans `services/` |
| P0-3 | Flux de `ohd` avec reprise | Numéro de séquence croissant sur `/v1/stream`, reprise `?since=<seq>` sur une fenêtre bornée, événement `resync` au-delà ; suppression du repli par relecture toutes les 2 s dans `services/session/live.go` | Test : un client lent ne perd plus d'événement dans la fenêtre ; `resync` au-delà |
| P0-4 | Mesures de référence | `make test-durations` sur `./cmd` et `./internal/tui/...`, temps de démarrage du TUI (jusqu'au premier affichage), couverture des paquets du TUI ; résultats dans `mesures/ui-baseline.md` | Fichier de mesures |
| P0-5 | Réparer Beads | `dolt_database` sans trait d'union (ou `bd doctor`), droits de `.beads` en 0700 | `bd list` fonctionne |
| P0-6 | Nettoyer le tracker | Fermer `opencode-hub-mep.2.3` (dashboard bash, remplacé par N3/Web), trancher `mep.2.1`, puis l'epic `mep` ; `xnj` relève de la doc | 0 issue de l'époque bash ouverte |
| P0-7 | (En option, en parallèle) Écritures atomiques | Les `os.WriteFile` du team-state, du board et de la config passent en écriture atomique (fichier temporaire puis `rename`) | Liste des `os.WriteFile` restants avec leur raison |

---

## 2. L0 : cadrage et garde-fous (1 à 1,5 semaine)

### 2.1 Décisions à prendre avant N1-1

- [ ] **Langue (ADR-061 §7)** : à trancher **avant N1-1**, car l'étape 4 de chaque lot en dépend. Recommandation : le contrat renvoie des codes et des paramètres, et chaque client traduit ; seuls les contenus rédigés (rapports, descriptions de MR) sont traduits côté serveur.
- [ ] **Emplacement des fonctions d'affichage du TUI** : un nouveau paquet `internal/tui/v2/present` (libellés d'état, icônes, timeline, badge), qui reprend `stateLabel`, `StateIcon`, `KindIcon`, `TimelineText`, `fanoutLine`, `SessionsBadge`.
- [ ] **`huh`** : on le remplace par `ui/forms` en N2 (recommandé) ou on le garde.
- [ ] **Jalonnement des versions** (voir le § 6).
- [ ] **Nommage** : `internal/ui/contract`, `internal/appsvc/<domaine>`, `internal/ui/forms`, `internal/uiserver`, `internal/ui/client`, `internal/tui/v2/present`.

### 2.2 Tâches

- [ ] Finaliser les ADR 061, 062 et 063, en français et en anglais, dans `docs/architecture/adr/`, plus leur entrée dans l'index des ADR.
- [ ] `docs/dev/ui-contract.fr.md` et `.en.md` : règles du contrat, conventions JSON (le § 3.2 ci-dessous), méthode d'un lot (§ 4.1), exemple Sessions.
- [ ] **Règles `archtest`** (`internal/archtest/ui_layers_test.go`), sur le modèle des listes gelées qui ne peuvent que raccourcir :
  - [ ] `ui/contract`, `appsvc` et `ui/forms` n'importent ni `internal/tui`, ni `cmd`, ni tview ou tcell, ni `net/http`, ni `storage/sqlite` ;
  - [ ] `ui/contract` n'importe aucun paquet métier, à l'exception de `time` et des types de base ;
  - [ ] pas de champ de type `func` ni `chan` dans les structures de `ui/contract` (analyse syntaxique) ;
  - [ ] `uiserver` et `ui/client` n'importent pas `internal/tui` ;
  - [ ] **gel du branchement** : nombre de lignes et de fonctions de `cmd/tui_*.go` enregistré dans `testdata/tui_wiring_budget.txt`, qui ne peut que baisser ;
  - [ ] **gel des imports métier des vues** : liste `testdata/views_business_imports.txt` (teamstate 21, config 12…), qui ne peut que raccourcir ;
  - [ ] **gel de `tuiShell` hors des fichiers `tui_*`** (9 fichiers, dont `team_resolve.go`) ;
  - [ ] **gel des `WizardStep.Form`** (pour N2) ;
  - [ ] élargir le détecteur de `team_resolution_test.go` à tous les appels `.ActiveTeam()` (v5.4 §A.7) ;
  - [ ] étendre `messages_literal_test` à `shell/` et `widgets/` (F7), avec une liste gelée ;
  - [ ] corriger la faille des tags de build (A-03).
- [ ] **Makefile** : `test-tui` couvre aussi `./cmd -run 'TUI|Tui|tui'`, ou une cible `test-ui` qui regroupe `./internal/tui/...`, `./internal/appsvc/...`, `./internal/ui/...` et les tests `tui_*` de `cmd`.
- [ ] **CI** : ajouter `internal/appsvc/...` et `internal/ui/...` à `.github/windows-test-packages.txt` dès leur création.
- [ ] Créer les tickets Beads (une epic par phase, un ticket par lot, avec ses dépendances).

---

## 3. N1-0 : socle du contrat (environ 2 semaines)

Il faut le faire **avant** N1-1 : tous les domaines en dépendent.

### 3.1 Contenu

| Élément | Paquet | Détail |
|---|---|---|
| `Scope` | `ui/contract` | `Scope{Kind hub\|project\|team; ProjectID; TeamID}` ; validation ; sérialisation `scope=project:<id>` |
| Résolution de la portée | `appsvc/scope` | **Remplace la lecture de `tuiShell.Mode()` et `ActiveTeam()` dans `cmd/team_resolve.go:89-109,260`.** `ResolveTeam(ctx, Scope)` devient une fonction pure. Le TUI construit le `Scope` à partir de son mode, la CLI à partir des options et du dossier courant. Supprime aussi `tuiContext()` (13 appels) |
| Erreurs | `ui/contract` | `Error{Code, Params}`, registre des codes ; correspondance avec les erreurs sentinelles (`views.ErrOverrideRequired`, `ErrProviderCredentials`, `ErrNoTracker`, `NotOfferedError`, `TransferFailedError`, `config.ErrConfigConflict`, `config.ErrExternalModification`) |
| Messages | `ui/contract` | `Message{Code, Params, Level}` pour les retours d'action, à la place des chaînes déjà traduites (`DeleteSession`, `syncSummary`, `TestMCP`…) |
| Intentions | `ui/contract` | `Intent{Kind, Params}` : `open_view`, `open_session`, `open_launch`, `enter_project`, `enter_team`, `open_editor`, `open_browser` |
| Capacités | `ui/contract` | `ClientCaps{Terminal, Suspend, Browser, Editor, Notifications}` |
| Événements | `ui/contract` et `appsvc/events` | `Event{Seq, Type, Scope, Time, Data}` ; **bus en mémoire** avec numéros de séquence et fenêtre de reprise ; `Subscribe(ctx, filter)` ; sources : flux `ohd` (P0-3), `config.Update`, synchronisation du team-state (remplace `views.teamSyncHook`), notifications (lot N de la v5.4), écritures `appsvc` |
| Types communs | `ui/contract` | `Option` (remplace `views.SelectOption`, 50 usages), `InfoField` (170 usages, pour N2), `Page`/`Cursor` si besoin |
| Racine | `appsvc` | `appsvc.Deps` (stores, services, `daemon/client`, `ToolInfo`… construits depuis `app.App`) ; `appsvc.Services`, qui regroupe les implémentations ; construit **uniquement** dans `cmd/service_wiring.go` |
| Affichage du TUI | `tui/v2/present` | Squelette : `StateLabel(state)`, `StateIcon(state)`, `DecisionIcon(kind)`, `MessageText(Message)`, `ErrorText(Error)` |
| Harnais de contrat | `ui/contract/contracttest` | `Run<Domain>Contract(t, newImpl func(t) (impl, seed))` ; faux exportés (reprise de `fakeSessionsBackend`, aujourd'hui privé à `shell/sessions_mouse_test.go:27`) ; outils `sqlitetest.Seed` et `testenv.Isolate` |
| Tests de caractérisation | `cmd/*_characterization_test.go` | Outil `characterize(t, name, v any)` : JSON trié et indenté, puis `golden.Assert` ; base `TestTUISessionsBackendIsQuiet` |

### 3.2 Conventions JSON du contrat

- Champs en `snake_case` ; `omitempty` sur les champs facultatifs ; dates en `time.Time` (RFC 3339, UTC).
- Les énumérations sont des types `string` nommés avec leurs constantes (`type SessionState string`), **jamais** un texte traduit.
- Les montants portent leur unité dans le nom (`cost_usd`), et les durées aussi (`*_ms`, `*_s`).
- Les identifiants sont toujours des `string`.
- Pas de `map[string]any` dans les types publics, sauf pour les `Params` des codes.
- Chaque type public est couvert par un test d'aller-retour JSON.

### 3.3 Fin de N1-0

- [ ] `team_resolve.go` ne lit plus `tuiShell` ; la liste `activeTeamAllowed` est raccourcie.
- [ ] Le bus d'événements est branché sur `ohd` et sur `config.Update` ; un test vérifie la séquence et la reprise.
- [ ] Le harnais de contrat, les faux et l'outil de caractérisation sont prêts et documentés.
- [ ] Les règles `archtest` de L0 passent, sans nouvelle exception.

---

## 4. N1 : un domaine à la fois

### 4.1 Méthode d'un lot (liste de contrôle)

**Avant de modifier quoi que ce soit**
1. [ ] Rebaser sur `main` ; vérifier qu'aucun lot en cours ne touche les fichiers du domaine (matrice du § 5).
2. [ ] **Tests de caractérisation**, commités en premier (`test: characterize <domaine> wiring`) :
   - **sortie** de chaque méthode de backend et de chaque fonction de configuration du domaine, en JSON golden, sur une base `sqlitetest.Seed` déterministe ;
   - **écrans** : goldens de rendu des vues du domaine, alimentées par un faux construit à partir des mêmes données (en ajouter là où il en manque). **Ces goldens d'écran ne doivent pas changer pendant tout le lot.**
3. [ ] Inventaire du domaine, à reprendre de `01-etat-des-lieux.md` et à revérifier : globales, crochets, `tuiShell`, `SuspendAndExec`, imports métier des vues, code en double avec la CLI.

**Pendant**
4. [ ] **Types** → `ui/contract/<domaine>.go`, avec tags JSON et types nommés ; alias provisoires dans `views` (`type SessionRow = contract.Session`, ou une fonction de correspondance si la forme change).
5. [ ] **Interfaces** → `ui/contract`, avec `ctx` et `Scope` partout, des erreurs avec un code, et des `Message` à la place des chaînes.
6. [ ] **Implémentation** → `appsvc/<domaine>`, en partant de `cmd/tui_<domaine>*.go`. Pas de `tuiShell`, pas de navigation, pas de `theme`, pas de `i18n.T` (hors contenus rédigés).
7. [ ] **La CLI passe aussi par `appsvc`** pour le domaine : les commandes `cmd/<domaine>*.go` appellent le même service, et seule leur mise en forme (tableau, JSON de sortie) reste dans `cmd`. **C'est ce qui supprime le code en double** (01 § 6).
8. [ ] **Affichage** → `tui/v2/present` : libellés, icônes, timeline, textes des messages. Le contrat ne transporte que des données brutes.
9. [ ] **Logique métier des vues** → `appsvc` (01 § 4). Les vues n'importent plus `teamstate`, `config`, `workflow`… pour ce domaine.
10. [ ] **Écritures sur la boucle d'affichage** → appels à `appsvc` en dehors de la boucle (comme `SessionsBackend` aujourd'hui : la vue appelle depuis une goroutine `safego`, puis `QueueUpdateDraw`).
11. [ ] **Rafraîchissement** : `Changes()`, `RemountIf`, les crochets de synchronisation et les relectures périodiques sont remplacés par un abonnement au bus d'événements.
12. [ ] **Navigation** : les actions qui ouvraient un écran depuis `cmd` renvoient une `Intent`, que le TUI applique (`NavigateTo`, `PushView`…).
13. [ ] **Suite de contrat** `Run<Domain>Contract` exécutée sur `appsvc/<domaine>` (et plus tard sur le client HTTP, en N3).
14. [ ] **Tests** : les tests de `cmd/tui_<domaine>*_test.go` déménagent vers `appsvc/<domaine>` (ce qui permet d'ajouter `t.Parallel()`) ; on en profite pour regrouper les familles de tests (`simplification-tests.md:41`).

**Fin de lot**
15. [ ] Plus aucun type de données `views` de ce domaine dans `cmd` ; `cmd/tui_<domaine>*.go` se réduit à la composition (construire la vue avec `appsvc.Services` et `present`).
16. [ ] Les vues du domaine n'importent plus de paquet métier ; listes gelées raccourcies (`views_business_imports`, `tuiShell`, budget du branchement).
17. [ ] Goldens d'écran **identiques** ; goldens de caractérisation régénérés **seulement** pour les champs d'affichage retirés (liste justifiée).
18. [ ] Alias supprimés ; `appsvc/<domaine>` inscrit dans `windows-test-packages.txt`.
19. [ ] Durées des tests pas plus mauvaises (`make test-durations -compare`).
20. [ ] Recette manuelle de la section du domaine dans `oh-tui-recette.md`.

### 4.2 Les lots

#### N1-1 Sessions (lot pilote, 2 à 3 semaines, **seul**)

Il valide la méthode : on corrige la liste de contrôle à partir de ce lot avant de lancer les autres.

- **Fichiers** : `tui_sessions` (610), `tui_sessions_tabs` (296), `tui_batch` (102), `tui_chain` (178), `tui_checkpoints*` (142), `tui_remote` (179), `tui_session_actions` (61) ; partie sessions de `tui_lifecycle`.
- **Contrat** : `Sessions` (List, Get, Follow, Decide, Send, Interrupt, SwitchModel, Stop, Resume, Attach, Results, NextSteps, Trace, Reports, PlanDelete, Delete), `Checkpoints` (Card, Report, Verify, WikiDecide, WikiEdit), `Batch` (Items, Apply), `Fanout` (Status, Relaunch, Clean), `Remote` (Fetch, PlanReplay, ApplyReplay), `Chain` (Options → `Intent open_launch`).
  - On regroupe les **14 interfaces** implémentées par `*tuiSessions` en 5 à 6 interfaces cohérentes ; les capacités facultatives deviennent des méthodes qui renvoient un code `not_supported`.
- **Globales à supprimer** : `tuiSess` (les caches deviennent un service avec une durée de vie explicite), `tuiChainSuggestions` et `tuiChainMu`, `chainNextByID`, `remoteTrack`.
- **Logique à déplacer** :
  - repli « branche courante » de l'enchaînement (`05-phase-1.md:174`) ;
  - sections, rétention et tri (`sessions_tree.go:54-82`) : le contrat renvoie `section` et `sort_key` ;
  - `BatchDecision.Failed` et `visible` ;
  - `statePriority`, `summary`.
- **Affichage → `present`** : `stateLabel`, `StateIcon`, `KindIcon`, `DecisionSummary`, `fanoutLabel`, `fanoutLine`, `TimelineText`, lignes de session distante (`tui_remote.go:69,100-124`), `SessionsBadge`, messages « toast_one/many ».
- **Code en double avec la CLI** : `session.go`, `session_delete.go`, `session_next.go`, `session_trace.go`, `session_batch.go`, `session_control.go`.
- **Capacités** : `Attach` selon `ClientCaps` (terminal, navigateur `pair`, suspension) ; `SuspendAndExec` reste dans le TUI.
- **Événements** : `session.changed`, `decision.created`, `decision.resolved`, `session.feed` ; supprimer `Changes()` et la relecture.
- **Contrat seulement** : champ `remote` et événement pour BL-9 (suivi distant en direct), implémentation hors plan.
- **À revérifier** : `AssistantText` en `--headless` (`06-phase-2.md:197`).

#### N1-2 Lancement (2 semaines)

- **Fichiers** : `tui_launch` (573), `tui_launch_inputs` (141), `_runtime` (76), `_remote` (42), `tui_start` (325), `tui_card_import` (91) ; côté CLI, `run_start.go`, `run_resume.go`, `run_existing.go`, `launch_config.go`.
- **Contrat** : `Launch.Prepare(ctx, Scope, LaunchRequest) → LaunchDraft{ID, spec résumée, inputs avec leurs défauts, runtimes, emplacements, exécutions existantes, récapitulatif structuré, avertissements}` ; `Launch.Update(draftID, changes)` ; `Launch.Start(draftID) → Session + Intent` ; `Start.Entries(ctx, Scope)` ; `Start.TogglePin`.
- **État caché à supprimer** : `tuiRunCache` devient le `LaunchDraft` côté service, en gardant aussi les échecs (`17-ecarts-fin-tui.md:19`) ; `tuiStartWiring` devient un service avec invalidation par événement ; `boardRequestInputs` devient une table du service.
- **Logique à déplacer** :
  - `newLaunchModel`, `validate`, `typeError`, `LaunchExisting.Allows` (`launch_form_model.go:117-345`) ;
  - regroupement par catégorie (`tui_start.go:149`) ;
  - ce qui reste dans `cmd` après C6 (`afterStart`, `startRun`, crochets `Deps`) ;
  - `launch.Deps.TeamRepo` est déjà traité par P0-2.
- **Données à ajouter** : mention « aucun verrou de commit » pour `libre` (DC26).
- **Code en double** : `run_start.go` / `tui_launch.go:178-282`.
- **`runOnLoop`** (`launch_config.go:20`) : il disparaît, car le service n'a pas besoin de la boucle d'affichage.

#### N1-3 Projets (2 semaines)

- **Fichiers** : `tui_views_project` (301), `tui_actions` (279, l'assistant d'ajout reste en tview jusqu'à N2), `tui_project_team` (186), partie projet de `tui_solo`.
- **Contrat** : `Projects` (List, Get, Add, Remove, Rename, Move, AttachTeam, InitBeads), `ProjectConfig` (lecture et écriture par sections, MCP, modèles, exécution), `ProjectWiki.Metrics`.
- **Code en double** : `projectRemover`, `Renamer` et `Mover` (appels directs au store) d'un côté, `project_remove.go:41,67` et `project_add.go` de l'autre.
- **Vues à libérer** : `project_config_view`, `project_mcp_view`, `project_models_view`, `worktree_view` (lecture de `Sessions`/`Projects`) → `domain` et `app` ne sont plus importés.
- 🔀 **Sélecteur `huh`** de `start_helpers.go` : décision N2.

#### N1-4 Équipe (3 à 4 semaines, découpé en 3 sous-lots consécutifs)

- **N1-4a Board** (`tui_team_board` 390, `tui_board` 129, `tui_board_card` 259, `tui_team_autosync` 70 ; vues `teamboard_view`, `teamboard_card_actions`, `board_data`, `board_columns`, `board_quick_actions`)
  - **Contrat** : `Board.Get(ctx, Scope, filters) → colonnes, cartes, actions disponibles par carte` ; `Claim`, `Release`, `Transfer`, `SetStatus`, `Launch(card) → Intent` ; `Sync`.
  - **Une seule implémentation du regroupement** : `serve_api.go:51-121` et `board_data.go:67-171` en ont chacune une, on n'en garde qu'une. Même chose pour la **séquence de prise de ticket**, répétée 3 fois.
  - **Logique à déplacer** : filtres, tri, `ticketContext`, `statusMoves`, `cardEntries`, `reviewEntries`, rythme de synchronisation.
  - **Détection de conflit** sur les claims et le board (code `conflict`), qui prépare N3.
- **N1-4b Équipes et configuration d'équipe** (`tui_views_team` 248, `tui_team_new` 114, `tui_team_actions` 214, `tui_solo` 118, `tui_team_mcp_check` 116 ; vues `team_detail_*` (23 fonctions), `teams_view` (11), `team_mode_view` (8), `team_mcp_view` (7), `team_models_view`, `teamstatus_view`, `activity_view`, `takeover_view`)
  - **Contrat** : `Teams`, `TeamConfig` (par sections, avec les champs réservés au rôle `lead`), `TeamMembers`, `TeamActivity`, `Takeover`.
  - **Globales à supprimer** : `teamDetailView`, `pickedTeam`.
  - **Code en double** : `takeover_brief.go` / `tui_team_actions.go:163-214`.
- **N1-4c Tracker et retour dans l'équipe** (`tui_team_tracker` 675, `tui_tracker` 49, `tui_team_rejoin` 585, `tui_team_columns*` 1 091)
  - **Contrat** : `Tracker` (Sync, Discover, Apply), `Columns` (Get, Save).
  - **Traduction des erreurs** de synchronisation : une seule, à la place des deux versions (`team_sync_tracker.go:107` / `tui_team_tracker.go:90`).
  - **Texte anglais en dur** (`tui_team_tracker.go:80`) remplacé par des données.
  - L'**éditeur de colonnes** (tview) reste un écran propre au TUI, mais il lit et écrit par le contrat.
  - Les assistants de retour dans l'équipe et de découverte du tracker restent en tview jusqu'à N2, mais leurs effets passent déjà par `appsvc`.
- **Exceptions gelées à raccourcir** : `teamstate_repo_exceptions.txt`, `activeTeamAllowed` ; les 82 fichiers de `cmd` qui importent `teamstate` diminuent.

#### N1-5 Wiki (1 semaine)

- **Fichiers** : `tui_wiki` (143), `tui_checkpoints_wiki` (39) ; vues `project_wiki_view`, `wiki_view`, `wiki_proposals`, `patterns_view`.
- **Contrat** : `Wiki` (Index, Page, Pending, Decide, Edit → `Intent open_editor`, Metrics), `Patterns`.
- **À revérifier** : les notifications d'acceptation et de refus dans le TUI (lot N de la v5.4) → événement `wiki.decided`.

#### N1-6 Workflows (2 semaines)

- **Fichiers** : `tui_catalog` (278), `tui_editor` (208), `tui_publish` (129), `tui_bricks` (67), `tui_workflow_sync` (84).
- **Contrat** : `Workflows` (Catalog, Get, New, Archive, DiscardDraft, Check, BundlePreview, Publish, PublishPreview, History, Restore, Diff), `Bricks`.
- Le **modèle de l'éditeur** (`workflow_editor_model.go`, qui importe `workflow`) passe par le contrat : la vue édite un brouillon, les validations sont côté service.
- **Globale à supprimer** : `tuiQueueFlushing` (garde de rejeu dans le service).
- `$EDITOR` passe par la capacité `Editor` et une `Intent open_editor` ; BL-1 (éditeur intégré) reste pour le web.
- `longHints` : 2 entrées à réduire.

#### N1-7 Administration (3 semaines, en 2 sous-lots parallélisables)

- **N1-7a Configuration** : réglages (`settings_view`, schéma de `buildFields` :385-654), fournisseurs, MCP du hub, modèles, secrets.
  - **Écritures sur la boucle d'affichage à supprimer** : `models_view.go:455-566`, `secrets_view.go:476-568`.
  - **Code en double** : repli des secrets (`secrets.go` / `secrets_view.go`).
  - **Exception `archtest`** `secrets_view → storage/keychain` supprimée.
  - **Fournisseur** : `Scope` explicite (hub, projet, équipe).
  - Le schéma des réglages est écrit **directement en `ui/forms`**, ce qui fait gagner du temps sur N2.
- **N1-7b Outils** : doctor (ce qui reste des 32 `cmd/doctor_*.go`, globales `DoctorChecks` et `DoctorOffline` avec leur `os.Setenv` dans un `init`), gc, nettoyage (`tui_cleanup`, `*shell.Shell` en paramètre), historique, métriques (par projet, `--team`, `platform.StatsProvider`), notifications, worktrees (`worktree.Remove`/`Prune` hors de la vue), politiques (contexte git écrit deux fois, `policy.New(store)` dans la vue), statut.
  - **Les 7 vues construites avec `*app.App`** n'en dépendent plus.

#### N1-8 Transverse (1 à 2 semaines, **en dernier**)

- **Fichiers** : `tui` (231), `tui_commands` (698), `tui_helpers` (279), `tui_views` (305), `tui_views_hub` (partie home, board, teams), `tui_keys_notice` (56), `tui_lifecycle` (199).
- **Contrat** :
  - `Home` (sections par `Scope`) ;
  - `Commands` : catalogue des 76 commandes de l'omnibar sous forme de données (identifiant, code du libellé, catégorie, modes, action vers `appsvc` ou `Intent`), réutilisable par la palette du web ;
  - `Keys` : catalogue des actions et de leurs raccourcis, avec des identifiants et non des `tcell.EventKey`.
- **Suppression de `tuiShell`** :
  - il est remplacé par une interface étroite, passée explicitement aux seuls constructeurs de vues qui en ont besoin ;
  - il disparaît aussi des 9 fichiers hors `tui_*` (`beads_clean`, `config_notices`, `hub_config`, `init_wizard`, `launch_config`, `mapping_notices`, `team_helpers`, `project_exec`) ;
  - `heldNotices` passe par le bus d'événements.
- **Globales de `views` à remplacer par de l'injection** : `ToolName`, `teamSyncHook`, `currentLevel` (`SetLayoutLevel`), `wizardTermHeight`.
- **Notifications** : une seule source (bus) pour les messages du TUI, les notifications système et le centre de notifications ; supprimer `NotificationsProvider` et le store en mémoire s'ils sont morts après le lot N de la v5.4.
- `formatEvent` : sans aucun texte en dur.
- 🔀 **En option** : découper `showHelpOverlay`, `buildModalFrame`, `updateSuggestions` et `showToast`.

### 4.3 Fin de N1

- [ ] `cmd/tui_*.go` ne fait plus que de la composition (budget d'environ 2 000 lignes à confirmer en L0) ; `tuiShell` n'est plus une globale.
- [ ] Les vues n'importent plus aucun paquet métier ; liste `views_business_imports` vide.
- [ ] Toutes les interfaces et tous les types de données viennent de `ui/contract` ; plus d'alias.
- [ ] La CLI et le TUI partagent `appsvc` sur les 8 domaines ; le code en double recensé (01 § 6) a disparu.
- [ ] Les suites de contrat passent sur toutes les implémentations `appsvc`.
- [ ] Le temps des tests de `cmd` a baissé (objectif : −30 %, à mesurer par rapport à P0-4).
- [ ] La recette complète `oh-tui-recette.md` est passée.

---

## 5. Recoupement des lots et vagues

| | N1-1 | N1-2 | N1-3 | N1-4 | N1-5 | N1-6 | N1-7 | N1-8 |
|---|---|---|---|---|---|---|---|---|
| **N1-1 Sessions** | — | ✗ (`tui_launch` suppression, chain) | · | · | ✗ (`tui_checkpoints_wiki`) | · | · | ✗ (`tui_lifecycle`, badge) |
| **N1-2 Lancement** | | — | · | ✗ (`tui_card_import`, lancement depuis le board) | · | · | · | ✗ |
| **N1-3 Projets** | | | — | ✗ (`tui_project_team`, `tui_solo`) | ✗ (wiki du projet) | · | ✗ (modèles et MCP du projet) | ✗ |
| **N1-4 Équipe** | | | | — | · | · | ✗ (MCP et modèles d'équipe) | ✗ |
| **N1-6 Workflows** | | | | | | — | · | ✗ |
| **N1-7 Administration** | | | | | | | — | ✗ |

✗ = fichiers partagés, à ne pas lancer en parallèle ; · = parallélisable.

**Vagues proposées :**

| Vague | Lots | Durée (calendaire) |
|---|---|---|
| A | N1-0 socle, puis N1-1 Sessions (pilote) | 4 à 5 sem |
| B | N1-2 Lancement ∥ N1-6 Workflows ∥ N1-7a Configuration | 2 à 3 sem |
| C | N1-3 Projets ∥ N1-7b Outils | 2 à 3 sem |
| D | N1-4a → N1-4b → N1-4c Équipe ∥ N1-5 Wiki | 3 à 4 sem |
| E | N1-8 Transverse | 1 à 2 sem |

Durée calendaire de N1 avec parallélisme : **environ 12 à 17 semaines**, contre 16 à 20 en séquentiel.

---

## 6. Jalonnement des versions (🔀 proposition à valider)

| Version | Contenu | Visible par l'utilisateur ? |
|---|---|---|
| 5.4.x | P0-1, P0-5, P0-6 | non |
| 5.5 | P0-2, P0-3, L0, N1-0, N1-1 | non (TUI identique) |
| 5.6 | vagues B et C | non |
| 5.7 | vagues D et E (fin de N1) | non |
| 5.8 | N2 (formulaires, fin de `huh`) | un peu (assistants de la CLI harmonisés) |
| 5.9 | N3 (`uiserver`, TUI en mode client, `oh web` expérimental) | oui (`oh serve` remplacé) |
| **6.0** | Web, app macOS, retraits de la v6 (`dashboard`, `metrics`, clés anciennes, commandes retirées, code de migration) | oui |

Le retrait de `oh dashboard` et `oh metrics`, déjà annoncé pour la v6, tombe en même temps que l'arrivée du web, qui les remplace.

---

## 7. N2 : formulaires décrits par des données (8 à 10 semaines)

| Lot | Contenu | Durée |
|---|---|---|
| N2-0 | `ui/forms` (schéma, types de champs, options dynamiques, validation par champ et par étape, `SkipIf`, `BackSkip`, message pendant le traitement, `InfoFields`), machine à états `WizardSession`, moteur tview à la place de l'intérieur d'`InlineWizardView` ; **pilote de test** `answers[stepID][fieldKey]` qui accepte les deux formes (pour garder les 79 appels `typeInto`/`choose`/`tick` et les 16 `scriptWizard`) | 3 sem |
| N2-1 | Assistants de la CLI : fournisseur, MCP, politiques, patterns, ajout, configuration et suppression de projet (8 commandes `runCLIWizard`) ; `runProjectAddInteractive` (299 l.) | 2 sem |
| N2-2 | `oh init` et équipe : `init_steps`, `team_steps`, retour dans l'équipe, découverte du tracker (`actionTrackerDiscovery`, 453 l.), `buildInitWizardRejoinSteps` (309 l.) ; supprimer le `LoadConfig` appelé dans `Form` | 2 à 3 sem |
| N2-3 | Paramètres de lancement, assistants intégrés au TUI (ajout de projet, création d'équipe) ; l'éditeur de colonnes devient un écran dédié ; « Nouveau workflow » (A39) | 1 à 2 sem |
| N2-4 | 🔀 Remplacement de `huh` (confirmations et sélecteurs de la CLI, `launcher/ui_cli.go`, `theme/huh.go`) ; suppression de `tui/common`, `tui/progress`, `tui/components/*` ; exception `launcher/ui_cli → tui/theme` supprimée | 1 à 2 sem |

**Fin de N2** : plus aucun `WizardStep.Form` ni `CustomView` (sauf les écrans dédiés listés) ; `huh` supprimé si décidé ; les 121 tests d'assistants passent par le nouveau pilote ; les 5 tests `onKeys` sont inchangés.

## 8. N3 : client/serveur (4 à 6 semaines)

| Lot | Contenu |
|---|---|
| N3-0 | `uiserver` : socket `ui.sock` (UID du pair) et TCP 127.0.0.1 (jeton, cookie, Host, Origin, CSRF, CSP) ; démarrage à la demande avec verrou ; arrêt sans client ; vérification de version ; mode direct sous Windows ; `oh doctor` |
| N3-1 | Registre des routes dérivé du contrat, erreurs `{code, params}`, **OpenAPI 3.1 généré** et contrôlé en CI |
| N3-2 | SSE : `/api/v2/events` (le bus de N1-0, avec reprise par `Last-Event-ID`), suivi en direct des sessions |
| N3-3 | Routes des assistants (`WizardSession`), sessions d'assistant rattachées au client, expiration |
| N3-4 | `ui/client` (implémentation HTTP du contrat) ; **les suites de contrat tournent deux fois** (direct et HTTP) ; faux serveur `httptest` pour les goldens d'écran de bout en bout |
| N3-5 | TUI en mode client ; mesure de la latence et du temps de démarrage par rapport à P0-4 ; bascule par défaut si c'est acceptable ; `--direct` |
| N3-6 | `oh web` (lien à usage unique, `--at-login`) ; `oh serve` remplacé (message de renvoi pendant une version) ; `dashboardHTML`, `/api/v1` et `oh board --watch` supprimés ; détection de conflit vérifiée avec deux clients simultanés |

## 9. Ce qui pourrait faire déraper le planning

- **Le lot pilote N1-1 dure plus de 4 semaines** : il faut revoir la méthode (taille des lots, part des alias) avant de lancer la vague B.
- **Les goldens d'écran changent** pendant un lot N1 : c'est un signe que la logique d'affichage a bougé ; on s'arrête et on corrige.
- **Le TUI évolue pendant N1** : chaque nouvelle fonctionnalité doit être écrite directement dans `appsvc` (règle `archtest` du budget de branchement) ; sinon, on la reporte après le lot du domaine.
- **La latence en mode client (N3-5) est trop forte** : le mode direct reste le mode par défaut du TUI, et seuls le web et l'app passent par `uiserver`. Le plan reste valable.
