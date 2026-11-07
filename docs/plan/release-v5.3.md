# Plan — v5.3 : retours de recette du board d'équipe et des fenêtres

> Rédigé le 04/11/2026, après la publication de la **v5.2.1** (`main` = `0adb31bf` en local, `b10e6426` tagué `v5.2.1` ; `github/main` = `b10e6426`, la fusion de la doc `0adb31bf` reste à pousser).
> Ce plan succède à `release-v5.1.md`, qui contient l'historique des versions 5.1.0, 5.2.0 et 5.2.1 (§ J). **Rien n'est encore lancé.**
> Version visée : **5.3.0**. Le retour 4 change le sens de `unassigned_labels` (ET → OU) et le retour 6 change le menu des cartes : ce ne sont pas des correctifs purs. Si l'on ne veut que les correctifs, voir la variante 5.2.2 au § V.

---

## Reprise de session (à lire en premier)

1. `cd /Users/benjamin.datiche/workspace/openhub && git status -sb && git log --oneline -3 && git worktree list`. On attend `main` propre, aucun worktree, et `main` en avance d'un commit sur `github/main` (fusion de la doc, `0adb31bf`) **si l'utilisateur ne l'a pas poussé**.
2. Lire ce plan, puis le § R de `release-v5.1.md`, dont les règles s'appliquent toutes ici, et les dernières lignes de son § J.
3. Faire trancher les décisions du § D par l'utilisateur avec l'outil `question`.
4. Lancer les vagues du § 2 dans l'ordre. Pour chaque lot : un worktree `../openhub-wt-v53-<lot>` et une branche `feat/v53-<lot>` ou `fix/v53-<lot>`, modèle des sessions `amazon-bedrock/eu.anthropic.claude-sonnet-5-5`.

## R. Règles ajoutées à celles de la v5.1

- **Avant chaque tag**, en plus de la vérification de fin de vague, faire ces trois contrôles, qui ont manqué le 04/11 :
  - `go test -count=1 ./...` **sans `-race`**, comme le job de release ;
  - `(cd cli && ../.github/scripts/windows-packages.sh check)` : tout nouveau paquet doit être classé ;
  - `.github/scripts/release-notes.sh vX.Y.Z` doit produire au moins un lien absolu (`TestReleaseNotes_RealChangelog`).
- **Lint** : golangci-lint **2.14**, la version de la CI, avec le binaire local `/private/var/folders/74/n9955l812k972sl5cj314qdm0000gn/T/opencode/golangci-lint-2.14.0-darwin-arm64/golangci-lint` (à retélécharger s'il a disparu). Options : `GOTOOLCHAIN=go1.26.10` et `--build-tags=integration,e2e,container`.
- **Tests asynchrones** : une panic rattrapée par `safego` est journalisée **après** l'écriture du rapport de crash. Un test qui lit le journal doit l'attendre (`Eventually`), et l'écran simulé se lit sur la boucle d'événements (`panicSim.text()`).
- **Tag** : c'est l'orchestrateur qui le pose et le pousse, **seulement si l'utilisateur le demande** explicitement dans la session. On attend toujours la CI verte sur le commit exact.

## D. Décisions à prendre au début de la session

1. **Version** : 5.3.0 (tout) ou 5.2.2 (lots A et B seulement, § V).
2. **Retour 3, ticket GitLab et workflows Beads** :
   - (a) masquer les workflows qui exigent un id Beads sur une carte GitLab et proposer `quick` / `spec` (petit) ;
   - (b) en plus, une action « faire le ticket » qui importe l'issue en Beads (`bd list --external-ref gitlab-…`, sinon `bd create --external-ref`), puis lance `ticket` sur l'id Beads ;
   - (c) un input `issue` dans `ticket` (gros : prompt, validation, réservation).
   
   Recommandé : (a) + (b).
3. **Retour 4, `unassigned_labels`** : passer de ET à **OU**, une requête par label puis dédoublonnage. C'est un changement de sens, annoncé dans le CHANGELOG. On peut aussi garder ET et documenter. Recommandé : OU, car c'est ce que l'assistant laisse croire en proposant plusieurs labels.
4. **Retour 4, valeur `todo` écrite par l'assistant** : corriger l'assistant pour qu'il écrive le statut de claim (`planned`) ou qu'il déclare la colonne `todo` (`role = initial`) ; ajouter un contrôle doctor et une validation à l'enregistrement ; **migrer** les configs existantes à la lecture (`todo` → statut du rôle `initial`, en avertissant). Recommandé : les trois.
5. **Retour 6, actions selon la colonne** : valider la table du § 1.F, notamment :
   - « Faire le ticket » depuis todo passe la carte en in_progress, avec contrôle de la limite de travail en cours ;
   - « Review » depuis la colonne review lance `review` sur la branche de la MR du claim, ou `review-feedback` si `mr_url` est connu ;
   - faut-il un **rôle `review`** de colonne, ou un repli sur l'ID ou le statut `review` ?
6. **Retour 5, versions** : recommander **opencode 2.2.5** (au lieu de 2.0.25) et épingler **bd 1.3.3**. Ce lot fait tourner les tests d'intégration réels (sans modèle). L'e2e et les évals restent à lancer par l'utilisateur.
7. **Taille des fenêtres (retour 1)** : la largeur s'adapte au contenu, entre une borne min et l'écran − 4. Faut-il aussi élargir les formulaires et les saisies, aujourd'hui fixés à 72 colonnes ? Recommandé : oui, une largeur proportionnelle bornée, comme les autres.

### Décisions arrêtées le 04/11

| # | Décision |
|---|---|
| D1 | **5.3.0**, tout le plan (pas de variante 5.2.2) |
| D2 | **(a) + (b)** : filtre selon la source, plus « Faire le ticket » avec import Beads. **(c)** (input `issue` dans `ticket`) **au backlog** (§ 3) |
| D3 | **OU** des labels de `unassigned_labels` (CHANGELOG + MIGRATION) |
| D4 | Les trois : assistant corrigé, validation à l'enregistrement + contrôle doctor, migration à la lecture |
| D5 | Nouveau **rôle `review`** de colonne (repli sur le statut `review` s'il n'est pas déclaré) ; table du § 1.F validée telle quelle |
| D6 | opencode **2.2.5** recommandé, bd **1.3.3** épinglé |
| D7 | Formulaires et saisies : largeur proportionnelle bornée |
| — | Fond « en creux » des champs et des cases (`BgModalField`) : **inchangé** |

## 1. Retours de recette du 04/11, analysés

Repérage en lecture seule du 04/11. Les causes sont vérifiées dans le code et dans `~/.oh`. Les numéros de ligne datent de `0adb31bf`.

### A. Fenêtres : largeur et troncature (retour 1, « a » sur le board, toutes les popups)

- **Chemin** : `teamboard_view.go:400` → `showTeamBoardQuickActions` (l.1034) → `showQuickActionModal` (`views/board_quick_actions.go:96`) → `ShowSelectModal` → `showSelectList` → `showList` (`shell/inline_prompts.go:157`).
- **Causes** :
  - le libellé est coupé à 34 caractères (`board_quick_actions.go:141`) et formaté en colonnes fixes `%s%-16s %-34s %-7s %s` (l.144) ;
  - le libellé est la **description** du workflow quand il n'existe pas de `tui.board.label.<id>` (`cmd/tui_start.go:276`) : seuls `spike` et `incident` en ont un ;
  - la largeur vaut `ProportionalWidth: -3` (≈ 60 %) et ne tient pas compte du contenu (`modal_sizing.go:62`, `computeWidth`) ;
  - une `tview.List` coupe sans « … » ;
  - le titre de la carte est coupé à 40.
- **Même défaut** dans tous les `ShowSelectModal`, `ShowMultiSelectModal` et `showSub*` : une vingtaine de vues et 5 fichiers de `cmd/`.
- **Saisies et formulaires** : largeur fixe de 72 (`ModalMaxWidth`), champs de 40.
- **À faire** :
  - `modalSizeHint.ContentWidth` = plus long libellé (`tview.TaggedStringWidth`) + bordures et marges, borné par [`ModalMinWidth`, écran − 4] ;
  - abandonner la coupe à 34 et les colonnes fixes, au profit d'une mise en page qui suit la largeur ;
  - libellés courts `tui.board.label.<workflow>` pour tous les workflows proposés sur une carte ;
  - formulaires et saisies : largeur proportionnelle bornée (décision D7) ;
  - tests de rendu sur écran simulé, avec un libellé long et une fenêtre large : pas de troncature.

### B. Fenêtres : fond noir sous les textes (retour 2)

- **Cause** (tview v0.42.0) : `tview.NewList()` fige `mainTextStyle` et `secondaryTextStyle` sur `Styles.PrimitiveBackgroundColor`, que le thème met à **BgPanel #1e1e2e** (`theme/tcell.go`). `styleSelectList` (`shell/modal.go:302`) ne colore que la boîte, si bien que chaque libellé est dessiné sur #1e1e2e au-dessus de BgModal #313244.
- **Ne sont pas en cause** : `TextView`, les éléments de `Form` et les cellules de `Table`. Les champs `InputField` et les cases sont plus sombres **volontairement** (`BgModalField`, même valeur que BgPanel, effet « en creux »). À revoir avec l'utilisateur : une teinte intermédiaire ?
- **À faire** :
  - `SetMainTextStyle` / `SetSecondaryTextStyle` sur `BgModal` dans `styleSelectList` et `newOptionList` ;
  - vérifier `widgets/sectioned_list.go` et les listes des vues placées dans une modale ;
  - vérifier qu'un `TagReset` dans un libellé ne ramène pas au fond BgPanel ;
  - test qui lit la couleur de fond d'une cellule de libellé.

### C. « #623 is a GitLab issue, not a Beads id » (retour 3)

- **Message** : `internal/workflow/render_prompt.go:288` (validation `beads-id`).
- **Origine** :
  - le menu de la carte propose **tous** les workflows `ticketWorkflows` (`cmd/tui_start.go:218`, `TicketInput != ""`), même sur une issue GitLab ;
  - `boardTicketLaunch` (`cmd/tui_board.go:61`) passe `Tickets: ["#623"]` ;
  - `ticket.yaml` exige `ticket: beads-id` (obligatoire) et son prompt ne lit pas GitLab. Seuls `spec`, `quick` et `review` ont `.oh.gitlab_mcp`.
- **Ce qui existe** :
  - `beads.BuildExternalRef` et `LinkToTracker` (`beads/beads.go:463`, `source_ref.go:43`) ;
  - `bd create --external-ref` et `bd list --external-ref` (`bdargs/spec_bd.go:63,183`) ;
  - `CreateFromGitLab`, retiré comme code mort en 5.1 (commit `928f83f8`), à reprendre si l'on choisit l'option (b).
- **À faire** (décision D2) : filtrer les workflows selon la source de la carte, puis éventuellement l'import Beads. Le résumé du formulaire de lancement doit dire pourquoi un workflow n'est pas proposé, au lieu de « Impossible de charger ».

### D. Colonne « todo » toujours vide (retour 4)

- **Données réelles** (`t-sru-team`) :
  - 12 claims, tous `claimed_by = benjamin.datiche`, aucun dans le pool ;
  - statuts : 2 `in_progress`, 8 `validation`, 2 `blocked` ;
  - `[board]` est vide ;
  - le dernier commit de synchro indique « 0 created ».
- **Causes** :
  1. **`'todo'` n'est pas un statut de claim.** `label_status_mapping` contient 4 entrées `= 'todo'`, écrites par l'assistant (`tracker/discovery.go:41` `ColumnID: "todo"`, `cmd/tui_team_tracker.go:518`, `team_config.go:416`). Sans colonnes personnalisées, `IsValidStatus` (`teamstate/teamconfig.go:135-159`) les refuse **en silence**, et `MapTrackerStatusWithBoard` (`tracker/sync.go:1052`) retombe sur l'état GitLab `opened`, soit `in_progress`. Les tickets à faire arrivent donc en IN PROGRESS.
  2. **`unassigned_labels` est un ET** (`tracker/gitlab.go:123`, labels joints par une virgule). Aucune issue ne porte les 4 labels, donc le pool est vide.
  3. `autopoolUnassigned` filtre sur `UpdatedAfter = lastSync` (`sync.go:869-871`) sans balayage complet initial (contrairement à `autoplan` et son `AutoPlanScan`) : les vieilles issues ne sont jamais vues.
  4. « Le premier label qui correspond gagne » suit l'ordre des labels de l'issue, pas celui de la config, qui est une map. Le commentaire de `sync.go:634` est faux. Exemple : #491 « Bloqué » + « READY TO DEV » finit en `blocked`.
- **Contournement immédiat pour l'utilisateur** (avant le correctif) : remplacer `'todo'` par `'planned'` dans `label_status_mapping` et ne garder qu'un label dans `unassigned_labels`.
- **À faire** :
  - OU des labels (D3) ;
  - premier balayage complet du pool ;
  - ordre de priorité déterministe des labels, par ordre des colonnes du board puis ordre alphabétique, documenté ;
  - validation des valeurs de `label_status_mapping` et `status_mapping` à l'enregistrement, contrôle doctor et migration à la lecture (D4) ;
  - correction de l'assistant ;
  - tests avec la config réelle anonymisée.

### E. Mises à jour opencode 2.2.5 et bd 1.3.3 (retour 5)

Versions installées : opencode v2.2.5, bd 1.3.3 (Homebrew).

- **opencode** :
  - `compatibility.json` : `opencode_recommended` passe de 2.0.25 à 2.2.5, dans `default` et `5.x` ;
  - `ci.yml:174-176` : `OPENCODE_VERSION` et les SHA-256 darwin ;
  - `pinned_integrity.json` (S-38) : ajouter les cibles de 2.2.5 avec `.github/scripts/opencode-pin.sh` (procédure de `pins.go`). `TestRecommendedVersionIsPinned` et `TestCIVersionIsRecommended` vérifient la cohérence ;
  - agents natifs : les caches de 2.0.20 et 2.2.5 sont identiques (`build`, `explore`, `general`, `plan`) ;
  - revalider l'API avec les tests de contrat et d'intégration (`make test-integration`, sans modèle) : `/api/info`, `/api/agent`, `/api/skill`, `/api/mcp`, `/api/session[/active]`, `/api/vcs`, `/api/pair`, les suffixes de session, `/api/experimental/session/{id}/instructions/entries` et `/wait`, le flux SSE `/api/event`. Vérifier aussi le plugin `plugin/index.ts` et `user_plugins.go` ;
  - docs : `MIGRATION.md`, `SECURITY*`, `migration-v5`, `container`, `tutorial`, `tui`, `release-checklist`.
- **bd** :
  - `ci.yml:96-97` : `BD_VERSION` et son SHA-256 ;
  - `bdargs/spec_bd.go` : `GeneratedFor` vaut 1.3.1, à régénérer (`go run ./gen/main.go`) ;
  - `bdschema/schema_bd.json` : comparer à `bd schema` ;
  - docs `beads-model` et commentaire de `tracking.go:74`.
  
  `oh doctor` exige la **même** version que `GeneratedFor` (`doctor_gateway.go:144`), il avertit donc déjà sur 1.3.3. **Risque** : les nouvelles options de bd sont refusées par la passerelle tant que la table n'est pas régénérée.
- **Vérification réelle** : intégration (`make test-integration`) avec opencode 2.2.5 et bd 1.3.3 installés. L'utilisateur lance l'e2e et les évals.

### F. Actions de la carte selon la colonne (retour 6)

- **Aujourd'hui** :
  - `a` affiche la même liste partout (`board_quick_actions.go:96`) ;
  - `BoardColumnDef` (`board_columns.go:11`) ne contient que `{Name, Status, Color}`, sans **rôle**. Les rôles existants sont `initial`, `active`, `terminal` et `blocked`, il n'y a pas de rôle `review` ;
  - un lancement ne fait passer le claim en `in_progress` que pour des tickets Beads (`start_ticket_claims.go:57`) ;
  - la prise `c` utilise le **projet actif**, pas celui de la carte (`cmd/tui_team_board.go:202-273`) ;
  - `review` n'a pas de `request` et n'est jamais proposé sur une carte.
- **Bug annexe** : `changeStatus` (`teamboard_view.go` ≈ l.1224) met `actionInProgress` à vrai avant d'ouvrir la liste. Après Échap, `c`, `r`, `t` et `s` ne répondent plus, sans message.
- **Table proposée** (à valider, D5) :

| Colonne (rôle) | Actions de `a` |
|---|---|
| todo (`initial`, `planned`) | Cadrer (`spec`, `request=#iid` ou id Beads) ; Faire le ticket (`ticket` si Beads, sinon import Beads ou `quick`), avec passage en in_progress et contrôle de la limite de travail en cours |
| in_progress (`active`) | Faire le ticket ; consigne à la session en cours si elle existe |
| review | Review (`review`, branche de la MR du claim) ; `review-feedback` si `mr_url` est connu |
| validation, blocked, done | Détail, statut, transfert |

- **À faire** :
  - ajouter `Role` à `BoardColumnDef` (depuis `BoardColumnConfig`), avec un rôle `review` ou un repli sur le statut ;
  - filtrer le menu ;
  - prise du claim au lancement pour une issue GitLab ;
  - passer la branche ou la MR à `review` ;
  - utiliser le projet de la carte dans `OnClaim` ;
  - corriger le verrou `actionInProgress` ;
  - mettre à jour la doc (`tui.*`, `team-setup.*`).

## 2. Vagues

| Vague | Lots (en parallèle) | Contenu | Dépend de |
|---|---|---|---|
| 1 | **A+B · fenêtres** | Taille selon le contenu (toutes les popups), fond des listes, libellés courts du board, saisies et formulaires (D7) | — |
| 1 | **D · todo** | `label_status_mapping` validé, migré et contrôlé par le doctor ; assistant corrigé ; OU des labels ; balayage complet du pool ; ordre déterministe | D3, D4 |
| 1 | **E · versions** | opencode 2.2.5 (compat, épinglage S-38, CI, docs), bd 1.3.3 (table `bdargs`, schéma, CI, docs) ; intégration réelle sans modèle | D6 |
| 2 | **C+F · actions du board** | Rôle des colonnes, menu selon la colonne, filtre selon la source (GitLab ou Beads), import Beads si D2 (b), passage en in_progress, review sur la MR, projet de la carte, verrou `actionInProgress` | A+B (même menu), D (colonne todo), D2, D5 |
| 3 | **F1 · release** | CHANGELOG `[5.3.0]`, doc (MIGRATION si changement de sens), vérification complète (§ R), push, CI, tag si l'utilisateur le demande | toutes |

Fichiers à surveiller entre les lots de la vague 1 :
- **A+B** : `shell/*`, `views/board_quick_actions.go`, `cmd/tui_start.go` (libellés) ;
- **D** : `tracker/*`, `teamstate/teamconfig.go`, `cmd/tui_team_tracker.go`, `cmd/team_config.go`, `cmd/doctor*` ;
- **E** : `adapters/opencodev2/*`, `gateway/bdargs`, `beads/bdschema`, `.github/workflows/ci.yml`, docs d'installation.

Les locales se répartissent par préfixe : `v53a.`, `v53d.`, `v53e.`. Le CHANGELOG est écrit sous `[Unreleased]` par chaque lot ; les conflits se résolvent à la fusion en gardant les deux côtés.

## V. Variante 5.2.2 (correctifs seuls)

Lots A+B, plus la partie « sûre » de D : validation et migration de `todo`, assistant corrigé, doctor, sans le passage au OU. On y ajoute le filtre (a) de C et le verrou `actionInProgress`. Les lots E, F et le OU des labels partent en 5.3.0.

## 3. Restes connus, hors de ce plan sauf décision

- **Backlog D2 (c)** : input `issue` dans le workflow `ticket` (prompt, validation, réservation), pour lancer `ticket` sur une issue GitLab sans passer par l'import Beads.

- Évals jamais lancées depuis les changements de consignes : S78 (spec), S79 (triage), S80 (spike), plus les cas existants de spec, triage, spike, implement-spec, audit et debug. Commande d'estimation : `cd cli && OH_EVAL_PLAN=1 OH_EVAL_CASES=S78,S79,S80 make test-eval`.
- Doc : le guide Linear demande `oh mcp setup linear`, que la commande refuse (`cmd/mcp.go`, `mcpSetupServices`) ; `[mcp.github]` n'est pas documenté.
- L'utilisateur peut rattacher ses 5 claims sous `t-sru-b267fbf1` avec `oh doctor --fix` (5.2.1).
- Après la v5.1 (`release-v5.1.md` § H) :
  - CI Windows (20 paquets exclus) ;
  - `brews` → `homebrew_casks` ;
  - écritures atomiques restantes ;
  - remote du projet dans le team-state ;
  - isolation stricte ;
  - `test-gen` / `db-migration` ;
  - fonctions du shell de plus de 80 lignes ;
  - interface fine du sous-domaine workflow de `teamstate`.

## J. Journal

| Date | Lot ou jalon | Résultat |
|---|---|---|
| 04/11 | Plan | Rédigé après la recette de la 5.2.1 (6 retours) et un repérage en lecture seule. Aucun lot lancé, à la demande de l'utilisateur (nouvelle session dédiée). |
| 04/11 | Reprise, décisions D1-D7 | `main` = `b10e6426`, propre, aucun worktree ; `main` = `github/main` (la fusion de la doc `0adb31bf` est déjà poussée) ; `origin` en retard de 19 commits. Décisions arrêtées (§ D) : 5.3.0 complète, D2 (a)+(b) avec (c) au backlog, OU, D4 complet, rôle `review`, table § 1.F validée, opencode 2.2.5 / bd 1.3.3, formulaires élargis, fond des champs inchangé. Vague 1 prête à lancer. |
| 04/11 | **Vague 1 lancée** | Worktrees depuis `b10e6426` : `openhub-wt-v53-windows` (`fix/v53-windows`, session `ses_e59f3b6cfffevdPWUow1TfhwIm`), `openhub-wt-v53-todo` (`fix/v53-todo`, `ses_e59f35c19fferuEf6LIjb1Nah2`), `openhub-wt-v53-versions` (`feat/v53-versions`, `ses_e59f30862ffefcwOHRY8RjlVFp`). Sonnet 5.5. CHANGELOG sous `[Unreleased]` par lot ; MIGRATION laissé à F1 (lot D donne le texte). |
| 04/11 | E et A+B fusionnés | **E** (5 commits) : opencode 2.2.5 (compat `default`, 5.0-5.2 + entrée `5.3`, sha512 épinglés, CI), bd 1.3.3 (table régénérée : seul ajout `rules compact --force` ; schéma inchangé ; `--readonly` plus strict → `SECURITY*`), test `BD_VERSION` = `GeneratedFor`. Intégration complète `-race -tags integration` verte (654 s). Aucun changement d'adaptateur. **A+B** (5 commits) : listes à la largeur du contenu [40, écran − 4] avec « … », saisies/formulaires 70 % [72, 120], menu `a` aligné sans coupe, 11 libellés `tui.board.label.*` (dont `debug`, `review`, `review-feedback` pour la vague 2), fond BgModal des listes. Aucun golden. Conflit CHANGELOG résolu (Added/Changed/Fixed). Vérif `main` `e89a25ce` : build, vet, archtest, i18n, paquets E, TUI `-race`, `-short ./cmd/`. Reste à l'utilisateur : e2e (`TestE2E`, crochets du plugin sous 2.2.5) et évals. Lot D en cours. |
| 04/11 | D fusionné (`4f66f221`) | 8 commits : OU (une requête GitLab par label, dédoublonnage ; Jira déjà OU), premier balayage complet de la réserve (`autopool_scanned` lié aux labels), priorité déterministe (ordre des colonnes puis alphabétique), valeurs de mapping migrées à la lecture / refusées à l'enregistrement, assistant corrigé, `oh doctor` `tracker_mappings` (+ `--fix`) et `tracker_mappings_manual`. Fixture anonymisée `teamstate/testdata/gitlab_team_todo.toml`. Conflit CHANGELOG résolu. Texte MIGRATION proposé dans le récapitulatif du lot (à reprendre en F1). **Décisions utilisateur après relecture** : priorité = **blocked d'abord, puis la colonne la plus avancée** (au lieu de l'ordre des colonnes) ; réserve : **balayage en plusieurs passes** jusqu'à couvrir tout le pool, plafond par passage inchangé → lot **D2**. |
| 04/11 | Fin de vague 1 vérifiée, vague 2 lancée | `main` `4f66f221` : build, vet, archtest, i18n, tracker/teamstate `-race`, `-short ./...`, `./cmd/` sans `-short` (261 s), lint 2.14 = 0, `windows-packages.sh check` OK. Lancés depuis `4f66f221` : **D2** (`openhub-wt-v53-todo2`, `fix/v53-todo2`, session du lot D reprise `ses_e59f35c19fferuEf6LIjb1Nah2`) et **C+F** (`openhub-wt-v53-board`, `feat/v53-board`, `ses_e59d0c77fffesXGu7sNgGV1Ibb`). Partage : rôle `review` et `teamconfig.go` à C+F ; priorité et réserve (`tracker/*`) à D2. |
| 04/11 | D2 fusionné | 4 commits : blocked (rôle `blocked`, ou statut `blocked` sur le board par défaut) l'emporte, puis la colonne la plus avancée, puis l'ordre alphabétique (`IsBlockedStatus`, `StatusRank` = -1 hors board) ; réserve balayée en plusieurs passes sans curseur (les claims commités sont la progression ; `autopool_scanned.covered` posé par un passage qui ne crée plus rien ; claim libéré non recréé ; fenêtre plafonnée à 2000 ; `max_unassigned_issues = 0` → 20). Vérif `main` : build, vet, archtest, i18n, tracker/teamstate `-race`, `-short ./cmd/`. |

### Texte MIGRATION à reprendre en F1 (lots D + D2)

> **Tracker : `unassigned_labels`, `todo`, priorité des labels et balayage de la réserve**
> - `unassigned_labels` est maintenant un **OU** : une issue qui porte l'un des labels entre dans la réserve (c'était un ET). Si vous comptiez sur le ET pour restreindre la réserve, gardez un seul label ou créez un label dédié.
> - La réserve est d'abord balayée **en entier**, vieilles issues comprises, en autant de passages qu'il faut : un passage crée au plus `max_unassigned_issues` claims (20 par défaut). Une réserve de 70 issues demande donc quatre passages de synchro, puis les passages deviennent incrémentaux. Un changement de `unassigned_labels` relance ce balayage. Augmentez `max_unassigned_issues` pour aller plus vite.
> - `'todo'` n'est pas un statut de claim. oh le lit désormais comme `planned` (statut de la colonne initiale) et vous prévient. Lancez `oh doctor`, puis `oh doctor --fix` pour réécrire `config.toml` (un commit dans le dépôt d'équipe). Une valeur qui ne désigne aucune colonne est ignorée : corrigez-la à la main, car l'enregistrement de la config d'équipe est refusé tant qu'elle y est.
> - Quand une issue porte plusieurs labels rattachés à des statuts : une colonne **bloquée** (rôle `blocked`, ou `blocked` sur le board par défaut) l'emporte toujours ; sinon la colonne **la plus avancée** (par défaut DONE > VALIDATION > REVIEW > IN PROGRESS > TODO) ; à égalité, l'ordre alphabétique des labels. Une issue « Bloqué » + « READY TO DEV » est donc bloquée, et un vieux « READY TO DEV » resté à côté de « DEV DOING » ne la fait plus reculer. Pour qu'une colonne passe avant les autres, donnez-lui le rôle `blocked`, ou placez-la plus loin dans le board.
> - Les claims déjà créés suivent leurs labels à la première synchro : un claim dont le seul label de statut est un label « à faire » repasse en TODO.

### Journal (suite)

| Date | Lot ou jalon | Résultat |
|---|---|---|
| 04/11 | C+F fusionné (`9062782c`) | 11 commits, rebasé sur D2 sans conflit : rôle `review` (hors WIP, repli sur la colonne d'ID `review`), `BoardColumnDef.Role`, éditeur de colonnes (`o`) ; menu `a` selon la colonne (`teamboard_card_actions.go`, actions indisponibles grisées avec leur raison, ancien menu si aucune colonne n'a de rôle) ; « Faire le ticket » GitLab → `beads.ImportFromTracker` (`--external-ref gitlab-N`, idempotent) puis `ticket`, sinon `quick` ; `launch.Options.NoClaim` ; WIP contrôlé avant lancement, claim de la carte passé en cours ; `review` avec la branche du claim, `review-feedback` si `mr_url` ; consigne à la session ; `c`/`L`/`t`/`s` sur le projet de la carte (confirmé par l'utilisateur) ; verrou après Échap ; `GitLabRefForBeadsError`. Aucun golden. Vérif complète de `main` lancée. **Décisions utilisateur** → lot **F2** (`openhub-wt-v53-board2`, `fix/v53-board2`, session C+F reprise) : import Beads à la confirmation seulement ; `DecideClaimEnd` appliqué au claim de la carte GitLab en fin de session ; `InReview` suit le rôle `review` ; assistant de découverte propose `review`. `DefaultBoardConfig` garde `active` + repli. |

### Texte MIGRATION C+F (à reprendre en F1)

> **Board d'équipe : le menu `a` suit la colonne.** todo : *Cadrer* et *Faire le ticket* ; en cours : *Faire le ticket* et la consigne à la session ; review : *Review* et *Retours de review* ; les autres colonnes : détail, statut, transfert. Sur une issue GitLab, *Faire le ticket* importe l'issue en ticket Beads (si le projet a Beads) puis lance `ticket` ; sinon `quick`. Pour lancer un autre workflow sur une carte, utilisez la barre de commandes ou l'accueil. Le nouveau rôle de colonne `review` est facultatif : sans lui, la colonne d'identifiant `review` en tient lieu.
| 04/11 | Vérif complète de `main` `9062782c` | build, vet (+ Windows), `-race` (TUI, teamstate, beads, launch, workflow), `-short ./...`, **`go test ./...` sans `-race`** (comme le job de release), lint 2.14 = 0, `windows-packages.sh check` OK. `release-notes.sh v5.3.0` échoue (attendu : pas encore de section `[5.3.0]`). F2 en cours. |
| 04/11 | F2 fusionné (`399b302b`) | 5 commits : import Beads à la confirmation (ticket de substitution `gitlab-N` dans le formulaire, `bd create` à « Lancer », échec = rien lancé) ; claim de carte réglé en fin de session par `DecideClaimEnd` + file durable, renouvelé par le battement du démon ; `Claim.InReviewOn(board)` (bail, expiration, doctor, fin de session, WIP) ; assistant propose le rôle `review`. Vérif `main` : build, vet (+ Windows), `teamstate -race -count=5` (l'échec isolé signalé ne se reproduit pas), `-race` TUI/beads/launch/workflow/tracker/daemon, `go test ./...` sans `-race`, `./cmd/` `-race` sans `-short`, lint 2.14 = 0, `windows-packages.sh check`. |
| 04/11 | **F1 release préparée** | `40e3be62` : CHANGELOG `[5.3.0] — 2026-11-04` (intro, `[Unreleased]` vide) ; `MIGRATION.md` « Vous venez de la 5.2 » ; `migration-v5.{fr,en}` « Depuis la 5.2 vers la 5.3 » (4 points) ; libellés du menu alignés sur l'écran dans `tui.*`, CHANGELOG et guides (« Implémenter le ticket », « Cadrer et écrire la spec », « Review de code », « Traiter les retours de review ») ; exemple `oh 5.3.0` du tutoriel. `release-notes.sh v5.3.0` OK, 4 liens absolus. docstest, i18n, archtest, tests de version OK. `goreleaser check` : seul `brews` déprécié (attendu) ; snapshot OK, formule `git`/`opencode`/`beads`/`terminal-notifier`, `oh version` OK. Simulation du runner (`HOME` vide, `GIT_CONFIG_GLOBAL=/dev/null`) lancée. Non poussé. |
| 04/11 | **`main` poussé, CI verte** | L'utilisateur a poussé `40e3be62` (`github` et `origin`). Simulation du runner verte. CI `41830344112` verte : intégration (opencode V2, sans modèle), ubuntu, macOS, Windows, Mermaid ; « Documentation checks » sauté (PR seulement, normal). **Reste** : e2e, évals S78-S80, recette du board d'équipe (utilisateur), puis tag `v5.3.0` sur `40e3be62` si l'utilisateur le demande (corriger la date du CHANGELOG si le tag n'est pas posé le 04/11). |
| 04/11 | **v5.3.0 publiée** | Tag annoté `v5.3.0` sur `40e3be62` (CI verte), poussé sur `github` à la demande de l'utilisateur. Release `41833585798` verte (5 min 36) ; « latest », 4 archives + `checksums.txt` + `.sigstore.json` ; `cosign verify-blob` OK, SHA-256 OK, `oh version` = 5.3.0 (`40e3be62`, go1.26.10) ; tap `2e113ffa` : formule 5.3.0, `git`/`opencode` obligatoires, `beads`/`terminal-notifier` recommandés ; notes avec liens absolus. CI du tag encore en cours au moment de la vérification. Restent : e2e, évals S78-S80, recette du board d'équipe ; backlog D2 (c) et § 3. |
| 04/11 | **5.3.1 lancée** (retours après publication) | (1) Board : une colonne plus haute que l'écran revient en haut à chaque rafraîchissement — cause : `populateColumns`/`repopulateWithFilters` (5 s, résumé Beads 30 s) et le board Beads font `CardColumn.Clear()`, qui remet `selected` et `offset` à 0. Lot `fix/v531-board-scroll` (`openhub-wt-v531-scroll`, `ses_e594524b2ffeuONTfrq6ngSZtX`). (2) Avertissements `mapping_migrated` bruts au démarrage (4 lignes pour `t-sru-team`) → une ligne par clone avec `oh doctor --fix`, détail au journal. Lot `fix/v531-mapping-warning` (`openhub-wt-v531-warn`, `ses_e5944da26ffe74Q1zbvrX0S6gR`). Depuis `40e3be62`. |
| 04/11 | 5.3.1 fusionnée en local | **Défilement** (`56e1ee64`) : `CardColumn.ReplaceCards` garde sélection et défilement par `Reference` ({Project, ID} sur le board d'équipe, ID sur le board Beads) ; `fillColumns` unique ; un changement de filtre ou d'onglet repart du haut ; 5 tests sur écran simulé rouges avant. **Avertissements** (`7190f2dc`) : notice par clone (`teamstate/mappings_notice.go`) traduite par `cmd/mapping_notices.go` (`v531.mapping.*`) — stderr en commande, toast différé dans la TUI (`holdNoticesForTUI`), slog en JSON ; détail par valeur au niveau Info. CHANGELOG `[5.3.1] — 2026-11-04`, exemple `oh 5.3.1`. Vérif complète lancée (`-race ./...`, runner sans `-race`, lint, Windows). |
| 04/11 | 5.3.1 : vérification d'avant tag | 1er run : `archtest` `TestCommandsWriteToTheirWriters` rouge (`os.Stderr` dans `cmd/mapping_notices.go` et `cmd/tui.go`, lot avertissements) → `f5064ac5` (`a.IO.ErrOut`, exception listée). 2e run : vet (+ Windows) OK, lint 0 ; échecs **sous charge seulement** (load ≈ 35-50) : `beads TestCleanImpactUndoesBdInitCommit` (59 s), `gateway TestCheckHookCallerRealGit` (40 s), `gateway TestShellStartupControlsENV` (1er run), `./cmd/` au délai par défaut de 10 min — tous verts relancés seuls (`-race -count=2`), `./cmd/` runner sans `-race` vert en 554 s avec `-timeout 40m`. À confirmer par la CI. `main` = `f5064ac5`, non poussé. |

## 5.3.2 — retours d'un test en équipe (04/11)

Analyses en lecture seule (3 sessions explore). Causes :
- **Install, phase équipe** : « Créer » n'a aucune étape d'identifiants ; « Rejoindre » les place après le clone ; git ne reçoit que les accès du poste (`GIT_TERMINAL_PROMPT=0`, `teamstate/repo.go:468-477`). Écran d'erreur sans bouton ; Échap → « Passer l'étape » ne saute que l'étape. **`wizardErr` jamais effacée par `skipCurrent`/`skipFromPrompt`** (`inline_wizard_nav.go:59-70`) → `OnComplete(true, err)` → `buildInitOnComplete` sort tôt : pas de `setup_done`, pas de hub.toml d'équipe (palette `oh team init` aussi).
- **Membre non créé** : même cause ; plus collision d'ID qui écrase la fiche d'un collègue (`services/team/init.go:91-97`) ou est ignorée (`team_steps.go:244-247`) ; deux chemins de clone (`~/.oh/team-state` vs `~/.oh/team-states/<hôte>/<dépôt>`), `IsCloned` sans vérif du remote.
- **« 2/5 »** = compteur d'étapes (`inline_wizard_step.go:93-96`) collé à « Membres trouvés dans l'équipe : ». L'assistant palette « Rejoindre » (`cmd/tui_team_rejoin.go`) n'a pas d'ajout de membre. `stale` du pull ignoré.
- **Config > Équipe piégé** : `global_keys.go:35` laisse toutes les touches au focus quand la palette/une fenêtre est active ; une vue qui finit de charger fait `SetFocus(list)` sans condition (settings + ~12 vues) → palette active sans focus, Échap perdu (Ctrl+Q seul). `TeamDetailView.Mount` ne remet pas `dirty*` à zéro → toast rouge 10 s à chaque sortie, qui mange l'Échap suivant ; `handleSelect` ignore l'équipe choisie.

**Décisions utilisateur** : tout en **5.3.2** ; token HTTPS dans le **trousseau d'oh + `GIT_ASKPASS`** pour toutes les opérations git du team-state ; **chacun crée sa propre fiche** en rejoignant, sans validation ; ID déjà pris → **refus, ou « C'est moi »** explicite qui rattache sans écraser ; **handle GitLab demandé partout** (facultatif), pas d'email.

| Lot | Branche | Contenu | Ordre |
|---|---|---|---|
| K | `fix/v532-focus-trap` | garde du shell (palette/fenêtre reprend le focus), `focusIfOwned` dans les vues, `TeamDetailView` (dirty, équipe choisie, état vide), « Échap retour » | parallèle |
| W | `fix/v532-team-wizard` | `wizardErr` effacée au saut ; ordre URL → sonde → accès (token HTTPS trousseau/ASKPASS, SSH) → identité → clone, dans les 4 assistants ; écran d'erreur Réessayer / Configurer l'accès / Modifier l'URL / Passer l'équipe ; « Passer » saute tout le groupe ; fournisseur redemandé si l'équipe est sautée ; un seul chemin de clone + vérif du remote ; libellés et doc | parallèle |
| M | `fix/v532-members` | collision (refus / « C'est moi »), ajout de membre dans « Rejoindre » (palette), formulaire d'identité unique (handle GitLab partout, rôle/DefaultMode alignés), « Étape n/N », `stale` affiché, « Ajouter » de la vue Équipes, dépôt distant vide | après W |

| Date | Lot ou jalon | Résultat |
|---|---|---|
| 04/11 | 5.3.2 vague 1 lancée | depuis `f5064ac5` : **K** (`openhub-wt-v532-focus`, `ses_e58d93768ffeliLa1fAbrrCikN`), **W** (`openhub-wt-v532-wizard`, `ses_e58d8a493ffeAIwwaqqAKSsy4M`). M après W. CI 5.3.1 (`41854246920`) en cours, tag `v5.3.1` à poser si verte. |
| 04/11 | 5.3.1 non publiée → fusionnée dans la 5.3.2 | CI `41854246920` (`f5064ac5`) : ubuntu, Windows, Mermaid verts ; les 2 jobs macOS jamais démarrés, deux fois (« job was not acquired by Runner », capacité macOS arm64 de GitHub). Décision utilisateur : pas de tag `v5.3.1`, son contenu part dans la 5.3.2 (la section `[5.3.1]` du CHANGELOG, déjà poussée, sera fondue dans `[5.3.2]` à la release). |
| 04/11 | K fusionné | 5 commits : `layerKey`/`lostLayerFocus` (le shell rend le focus à la palette ou à la fenêtre avant de livrer la touche), `views/focus_owned.go` (`ownsFocus`/`focusIf`) dans les vues à chargement asynchrone, `hintsOf` ajoute « Échap revenir » au-delà de la racine, `TeamsViewDeps.OnSelectTeam` + `pickTeam`/`makeResolveTeamFunc` (équipe choisie, solo reconnue), `teamConfigWriter` sur le même résolveur, `Mount` remet `dirty*` à zéro, état vide sans équipe. Vérif `main` : build, vet, archtest, i18n, TUI `-race`, `-short ./cmd/`. W en cours. |
| 04/11 | W fusionné (`07d539ed`), M lancé | **W** (7 commits) : moteur (`skipCurrent` efface `wizardErr`, `OnSkip`, `ErrorActions`, `BackSkip`) ; `teamstate/gitauth.go` (`GIT_ASKPASS` à usage unique, 0700/0600, `-c credential.helper=` quand oh fournit le token), `ProbeRemote`, `IsAuthError` exporté, `CheckRemote`/`RemoteMismatchError` ; clés `openhub.git.<hôte>/<chemin>.token`/`.user` ; `cmd/team_access.go` partagé ; `provider_late`, `team_solo_offer` ; chemin unique (`Service.StatePathFor`), anciens `state_path` gardés ; `gittest.ServeHTTP`. **Incident** : un test de W a écrit 5 mots de passe `127.0.0.1` dans le trousseau login via `osxkeychain` ; supprimés par la session, vérifié 0 entrée par l'orchestrateur (avant et après les tests sur `main`). Conflits CHANGELOG et locales résolus. Vérif `main` : build, vet (+ Windows), archtest, i18n, `-race` teamstate/config/services/team/gitutil/views, `-short ./cmd/`. **M** lancé (`openhub-wt-v532-members`, `fix/v532-members`, `ses_e589373daffetUhLKpAIZGhgUK`). |
| 04/11 | M fusionné, release 5.3.2 préparée | **M** (8 commits) : `teamstate/members_join.go` (`JoinMember` `JoinNew`/`JoinOwn`/`JoinClaim`, `CheckJoin`), « C'est moi » (jamais par défaut, complète les champs vides seulement), formulaire unique (`cmd/team_identity.go`, handle GitLab facultatif et unique, `lead` qui crée / `dev` qui rejoint, `semi-auto`), ajout de fiche dans la palette « Rejoindre », « Étape n/N », liste périmée signalée, `n` de la vue Équipes → assistant « Rejoindre », dépôt distant vide (`gittest.EmptyBare`). Trousseau : 0 entrée `127.0.0.1`. **F1** : `6c39f277` CHANGELOG `[5.3.2] — 2026-11-04` (5.3.1 fondue dedans, `[Unreleased]` vide) ; `MIGRATION.md` « Vous venez de la 5.3.0 » ; `migration-v5.{fr,en}` « Depuis la 5.3 vers la 5.3.2 » ; tutoriel `oh 5.3.2` ; `release-notes.sh v5.3.2` : 2 liens absolus. Vérif complète lancée. |
| 04/11 | CI 5.3.2 rouge (`6c39f277`) → corrigée | Windows `TestInitRefusesAFolderThatHoldsAnotherRepository` : `originURL` lisait `.git/config` sans décoder les échappements (chemin Windows `C:\\Users\\…`) → `5655d7b5` (`gitConfigValue`, test). macOS `TestTeamGroupGivenUpInTheGroupedLayout` (30 s, écran à moitié dessiné) : `SimulationScreen.SetSize` ne poste pas de resize, une page dessinée avant restait telle quelle → `a7dbb34e` (resize + `QueueUpdateDraw`) — hypothèse cohérente avec l'écran capturé, non reproduite en local (15/15 vert avant). Vérif locale complète de `6c39f277` verte (`-race ./...`, runner, lint, trousseau 0). Poussé `a7dbb34e`, CI `41884579570` en cours. |

## Audit complet de la fonction équipe (04/11, 5 axes, lecture seule)

Rapports : sessions explore `ses_e58573a1dffeWnF4c0nokb5cm0` (cycle de vie), `ses_e58570da6ffexEAhNh6FV3whuD` (accès UX), `ses_e5856e1f4ffe8DBcJy3nEn3Hf6` (notifications), `ses_e5856b60affeMUQNpP8xdfbgRb` (configuration), `ses_e58568df7ffey09aikEol19nZP` (fonctionnement).

**🔴 Perte ou corruption de données**
- `oh team config status` supprime les collègues de `members.toml` (orphelin = absent de la config locale ; option 0 = supprimer ; Entrée/EOF/sans terminal = 0) — `team_config.go:630-633,845-856,1057-1066`, `team.go:832-852`.
- `state_path` vide : TUI sur le dépôt du dossier courant (pull, et Ctrl+S du détail = commit/push de config.toml sur l'origine du projet) ; 3 replis différents — `tui_helpers.go:99-102`, `repo.go:71-74`.
- `SaveConfig` écrit tout le config.toml depuis une copie lue avant le pull (écrase les modifs d'un collègue, rejoue la copie périmée) — `teamconfig.go:815-833`.
- config.toml illisible ou clone absent → TUI affiche une config vide, Ctrl+S efface board/mappings/MCP/limits/gouvernance — `view_helpers.go:53-66`, `team_detail_view.go:357-360`.
- `oh team config` : Entrée = index 0 (Jira → GitLab), surcharges locales toujours écrites ; `writeLocal` sans verrou, `coerceValue` peut rendre hub.toml illisible.
- Deux dépôts même basename (« team-state ») : même ID, même dossier `<hôte>/<basename>`, même clé → 2e équipe impossible ou remplace la 1re.
- Token git copié vers `openhub.team.<id>.gitlab.token` (écrase) ; scopes conseillés sans `read_user` → `GET /user` 403 → « Rejoindre » bloqué en boucle.
- Synchro tracker écrase à chaque passage les statuts du hub (review/validation/done → in_progress tant que l'issue est ouverte sans label) — `tracker/sync.go:556-565` ; labels hub (`session-ended`, `review`) retirés (`:1265-1273`).

**🟠 Logique / multi-équipes / accès**
- Doublons de claims (tous les projets locaux rattachés au projet tracker sans filtre `TeamID` ; `SRU-42` vs `42`) ; libérations ressuscitées (`sync:` hors `removalPrefixes`) ; synchro « succès » alors que rien n'est poussé (`sync.go:207-212`).
- Mauvaise équipe visée : colonnes, découverte, synchro, `K`, team configure, `oh team *` (pas de `--team`), member_id de l'équipe active ; « équipe par défaut » implicite (ordre de hub.toml, repli sur une archivée).
- `oh team init`/`oh team rejoin` (CLI) clonent sans sonde ni accès ; palette « Initialiser » sur équipe configurée : pull+push sur le fil UI ; impossible de créer une 2e équipe (TUI et CLI).
- `AuthError` perdue/texte FR codé (classification dépendante de la langue) ; sonde = lecture seule ; démon muet sur token git expiré (baux qui expirent), pas de check doctor d'accès au team-state ; période du heartbeat figée au démarrage.
- `auto_sync`/`sync_interval_minutes` documentés mais morts ; `type_enforced` sans effet ; `[parallel]` réécrit ; défauts figés dans le fichier ; clés inconnues effacées ; `BoardConfig.Validate` jamais appelé ; mapping strict qui bloque tout enregistrement.
- Pas de quitter une équipe, retirer un membre, réassigner les claims d'un parti ; release/transfer d'un autre sans contrôle ; board « pris » quand rien n'est pris ; TODO impossible sur le board par défaut ; lancement hors ligne sans claim en silence.

**🟠 Découvrabilité** : section Équipes absente de l'accueil sans équipe, aucun bouton ; « Créer » seulement palette/CLI ; noms multiples (Rejoindre ×4, Créer ×3, `n` « créer » = rejoindre) ; `team.configure` toujours en échec (vue équipe sans projet) ; palette figée au lancement ; MCP/Modèles d'équipe cachés ; `d` de la vue Équipes ne détache pas les projets.

**🔴/🟠 Notifications** : 3 systèmes séparés (système via démon, webhooks d'équipe, toasts TUI) ; décision en attente TUI fermée → 1 notif puis veille au bout de 5 min et décision `gone`, sans rappel ; `notification.enabled` absent de la TUI (+ assistant écrit `mattermost_webhook`, TUI lit `webhook_url`) ; MCP `notify` répond « sent » même désactivé ; `session.complete`/`claim.*` formatés jamais envoyés ; aucun destinataire personnel (`mattermost_username` inutilisé) ; titres « session en erreur » qui cachent la cause ; `-group oh-sessions` unique (une notif en remplace une autre) ; webhook synchrone sans retry ; `session.notify` absent de la TUI et lu au démarrage du démon ; webhook en clair dans le dépôt.

**Décisions utilisateur (04/11, après l'audit)** : **5.3.3** = lot S seul (données), puis **5.4.0** = Y, R, A, U, C, N, L. Synchro : le tracker ne réécrit un statut **que s'il a changé** depuis le passage précédent ; avec `push_labels`, le statut du hub est repoussé en label. **Synchro automatique implémentée** dans le démon (verrou dans le team-state : un seul membre par intervalle) + à l'ouverture du board. Config partagée : tout membre modifie, **événement `config_changed`** (clés, auteur) ; sections `*_enforced` et `limits.enforced` **réservées au rôle lead**. Surcharges du tracker **par équipe** (`[teams.<id>.tracker]`, `[tracker]` = défaut de toutes). Décision en attente TUI fermée : **la veille reste à 5 min, mais notification « mise en veille avec une décision en attente » et la décision est reprise au réveil**. Webhook : **reste dans config.toml, masqué** dans les écrans et récaps, doctor avertit si le dépôt est public.

| 04/11 | CI `a7dbb34e` | ubuntu, macOS, Windows, Mermaid verts ; intégration (macOS) non acquise (capacité GitHub) → relancée. |
| 04/11 | **5.3.3 lot S lancé** (depuis `a7dbb34e`) | **S1** chemins et collisions (`openhub-wt-v533-s1-paths`, `ses_e580d2f16ffewNo54G7UID9UH7`) ; **S2** écriture sûre de config.toml + `oh team config` + `status` qui supprimait les collègues (`openhub-wt-v533-s2-configwrite`, `ses_e580cc2ffffeqCriz0uDCNMD57`) ; **S3** synchro qui n'écrase plus + labels du hub + faux succès + token git ≠ token API (`openhub-wt-v533-s3-sync`, `ses_e580c58bafferl71NCZkJwWO6S`). Locales `v533a.`/`v533b.`/`v533c.`. |
| 04/11 | **v5.3.2 publiée** | CI `41884579570` (`a7dbb34e`) verte après relance de l'intégration (capacité macOS). Tag annoté `v5.3.2` sur `a7dbb34e`, poussé sur `github`. Release `41894762359` verte ; « latest », 4 archives + checksums + Sigstore ; `cosign verify-blob` OK, SHA-256 OK, `oh version` = 5.3.2 (`a7dbb34e`, go1.26.10) ; tap `0e67909f` formule 5.3.2 (git/opencode, beads/terminal-notifier recommandés) ; notes : 2 liens absolus. Lot S (5.3.3) en cours. |
| 04/11 | S3 fusionné | 4 commits : mémoire tracker sur le claim (`tracker_state`, `tracker_labels`, facultatifs, compatibles) ; un claim sans mémoire n'est pas déplacé au 1er passage (sauf fermeture / réouverture) ; statut réécrit seulement si état, nom de statut ou labels rattachés ont changé ; `push_labels` pose le label de la colonne et retire les autres ; labels du hub conservés ; poussée en échec = erreur, `sync-state` non écrit ; token git plus copié vers la clé API, étape token API toujours proposée, `/user` 401/403/réseau → « identité non vérifiée » (avertissement) ; ADR-028 amendée. Vérif `main` : build, vet, archtest, i18n, `-race` tracker/teamstate/services/team, `-short ./cmd/`, trousseau 0. S1, S2 en cours. |
| 04/11 | S2 fusionné (`ec38457c`) | 5 commits : `teamstate/configpatch.go` (`UpdateConfig` patch dans la transaction, `UpdateConfigFrom` diff base→édité, `ErrConfigConflict{Keys}`, rejeu sur rejet), `SaveConfig` réservé aux fixtures + archtest `TestNoCodeSavesAWholeTeamConfig` ; vues détail/MCP/Modèles en état d'erreur si config illisible ou clone absent (Ctrl+S refusé) ; colonnes, découverte, étapes d'assistant migrées ; `oh team config` : Entrée = valeur actuelle, « hériter » = unset, refus sans terminal, `a.IO.In`, `writeLocal` → `config.Update` ; `status` n'écrit plus rien (nettoyage des orphelins retiré). Conflits CHANGELOG/locales résolus. Vérif `main` : build, vet (+ Windows), archtest, i18n, `-race` teamstate/views/tracker, `-short ./cmd/`, trousseau 0. S1 en cours. |
| 04/11 | S1 fusionné (`e530036c`), release 5.3.3 préparée | **S1** (5 commits) : paquet `internal/gitremote` ; `NewRepo(remote, "")` → `ErrNoStatePath` ; `config.ResolveStatePath` à la lecture de hub.toml (vide → clone, ancien clone du bon dépôt réutilisé, `~`, relatif sous `~/.oh`) ; clone `<hôte>/<chemin complet>` ; ID slug unique (`<nom>`, puis `<groupe>-<nom>`, `<hôte>-<groupe>-<nom>`, numéro) ; `UpsertTeam` par dépôt, garde l'ID ; `TeamIDFor` ; formulaire d'ajout de la vue Équipes refuse un ID/dépôt pris. Conflits `team_git_credentials.go` (garde la suppression S3), `team_rejoin_core.go` (`TeamName` S1 + `IdentityUnverified` S3), CHANGELOG, locales résolus. Note : un test de S1 a tourné avec `/etc/gitconfig` (`osxkeychain`) avant correction ; trousseau 0. **F1** : `b1caeb10` CHANGELOG `[5.3.3] — 2026-11-04`, MIGRATION « Vous venez de la 5.3.2 », `migration-v5.{fr,en}` « Depuis la 5.3.2 vers la 5.3.3 », tutoriel `oh 5.3.3`, notes : 1 lien absolu. Vérif complète lancée. |
| 04/11 | Fin de session : plan 5.4.0 rédigé | `~/.opencode/plan/release-v5.4.md` (reprise, règles, décisions, questions à trancher, lots Y/R/A/U/C/N/L avec les constats de l'audit, vagues, restes). La 5.4.0 sera lancée dans une nouvelle session. |
| 04/11 | 5.3.3 : vérif locale + push | Vérif complète de `b1caeb10` : build, vet (+ Windows), docstest, `-race ./...`, runner sans `-race`, lint 0, trousseau 0 ; **`windows-packages.sh check` rouge** : `internal/gitremote` (S1) non classé → `fff1de13` (ajouté aux paquets testés sous Windows, test neutre vis-à-vis de la plateforme). L'utilisateur a poussé `b1caeb10` ; `fff1de13` poussé par l'orchestrateur (github + origin). CI `41906704209` en cours (run de `b1caeb10` annulé). Tag `v5.3.3` à poser sur `fff1de13` si CI verte et si l'utilisateur le demande. |
| 04/11 | CI `fff1de13` verte | Run `41906704209` : 2 jobs macOS non acquis (capacité GitHub) au 1er essai, relancés automatiquement ; ensuite ubuntu, macOS, Windows, intégration, Mermaid verts. Prêt pour le tag `v5.3.3` sur `fff1de13`, en attente de l'accord de l'utilisateur. |
| 04/11 | **v5.3.3 publiée** — fin de session | Tag annoté `v5.3.3` sur `fff1de13`, poussé sur `github`. Release `41912803743` verte ; « latest », 4 archives + checksums + Sigstore ; `cosign verify-blob` OK, SHA-256 OK, `oh version` = 5.3.3 (`fff1de13`, go1.26.10) ; tap `639971f6` formule 5.3.3 ; notes : lien absolu. Suite : `release-v5.4.md`. |
