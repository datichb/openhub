# Revérification de docs/dev/audit-v5.md — 2026-10-21

> Lecture seule, sans exécution (ni test, ni build, ni binaire `oh`). Dépôt : `main` au commit `34a16efa`. Base de comparaison : l'audit à jour au merge `541e7c06` (2026-10-20 16:02) ; depuis, 123 commits (dont 11 merges : `tech-leftovers`, `ci-fix`, `ci-fix-2`, 11 lots « modernisation tests »).
> Méthode : `git diff --stat 541e7c06 HEAD` hors tests/goldens/docs (65 fichiers touchés) pour savoir quels fichiers cités par l'audit ont bougé, puis lecture du code actuel et grep.
> Fichiers non-test modifiés depuis l'audit et utiles ici : `credproxy/{relay,routes,proxy,usage}.go`, `adapters/opencodev2/{pins,linux_binary,adapter,server,neutral}.go` + `pinned_integrity.json`, `cmd/{runner,v5_adapters,team,team_board,team_init_core}.go`, `archtest/goroutines_test.go`, `daemon/{budgetgate,cutestimate,wiki,handlers,daemon}.go`, `services/fanout/*`, `config/config.go` (UpsertTeam), TUI (teamboard, cardcolumn, settings_exec…), `.github/workflows/ci.yml`. **Aucun** des fichiers cités par les autres constats OPEN/PARTIAL n'a changé.

## 1. Résultat

**3 statuts changent** (tous vers FIXED) : S-28, S-38 (avec réserve), A-02. Les 30 autres constats non FIXED restent inchangés.

**Anomalie de comptage dans l'audit** : les lignes du tableau 4.10 ne correspondent pas aux lignes de la section 4 : l'axe A est annoncé 0/5/4 alors que les lignes donnent 0 FIXED, 6 PARTIAL (A-01, 02, 03, 04, 06, 09), 3 OPEN (A-05, 07, 08). Le total annoncé « 50 / 20 / 13 » est donc en réalité **50 / 21 / 12** (compté par script sur les 83 lignes).

## 2. Constats non FIXED (33)

### Sécurité (S)

| ID | Audit | Actuel | Preuve | Reste à faire |
|----|-------|--------|--------|---------------|
| S-11 | 🟡 PARTIAL | 🟡 PARTIAL | `adapters/opencodev2/attest.go` inchangé ; `strict_isolation` toujours `false` par défaut (`config/config.go:83-85`) | Isolation stricte par défaut, ou échec (pas avertissement) hors strict |
| S-12 | 🟡 PARTIAL | 🟡 PARTIAL | `bundle/hookguard.go`, `permissions/developer-rw.yaml` inchangés | Verrous appliqués par le démon (P2) ; lanceurs autorisés = risque documenté |
| S-13 | 🟡 PARTIAL | 🟡 PARTIAL | `daemon/gateway.go`, `gateway/mcp.go` inchangés (jeton par groupe) | Jeton par session, ou séparation des sessions d'un groupe |
| S-15 | 🟡 PARTIAL | 🟡 PARTIAL | `gateway/shellenv.go` inchangé | Bash interactif (`~/.bashrc`) seulement détecté (non redirigeable) |
| S-20 | 🟡 PARTIAL | 🟡 PARTIAL | `workflow/patch.go` inchangé | Ajout d'agent / `skills.extra` hors couche hub : avertissement seulement |
| **S-28** | 🟡 PARTIAL | **✅ FIXED** | `b33c7c36` : `credproxy/relay.go` (liste blanche d'en-têtes par fournisseur `providerRelayHeaders`, `commonRelayHeaders`, `filterRelay` l.97-111, `filterQuery` : seuls les paramètres de la route, doublons retirés), appelée dans `proxy.go:686` ; journal des noms retirés (jamais les valeurs) ; `relay_test.go` ; `SECURITY.md` « Header and query allow-lists (S-28) ». Les autres volets étaient déjà faits par v5-fix-h2 | Résidu documenté (non constat) : un en-tête autorisé est relayé avec la valeur de l'agent (`anthropic-beta`) |
| **S-38** | 🟡 PARTIAL | **✅ FIXED (réserve)** | `8fdab730` : `adapters/opencodev2/pinned_integrity.json` + `pins.go` ; `linux_binary.go:147` refuse un paquet dont l'intégrité annoncée par npm diffère de l'épingle ; binaire en cache sans épingle ou autre valeur retéléchargé (`binaryPinSuffix`) ; version non épinglée : avertissement, refusée sous `execution.strict_isolation` (`linux_binary.go:~200`) ; `linux_binary_pin_test.go`, `v5_adapters_pins_test.go` | **Réserve** : seule `2.0.20` est épinglée (4 cibles Linux) alors que la plage supportée est `2.0.0–2.99.99` (`compatibility.json`) et que la CI installe `2.0.25` ; toute autre version installée = simple avertissement hors strict. À faire à chaque release : épingler la version recommandée (procédure dans `pins.go`). Si l'on veut être strict, garder PARTIAL jusqu'à ce que l'épingle couvre la version réellement installée |
| S-39 | 🟡 PARTIAL | 🟡 PARTIAL | `selfupdate/selfupdate.go`, `install.sh` inchangés (seuls des tests, `69c480a7`) | Cosign obligatoire par défaut (ou décision documentée de rester facultatif) |

### Fonctionnel (F)

| ID | Audit | Actuel | Preuve | Reste à faire |
|----|-------|--------|--------|---------------|
| F-02 | ❌ OPEN | ❌ OPEN | `cmd/board.go:113-131` inchangé ; `teamstate/claims.go:29` (statut `review`) ; `internal/prompt/dev.go:47,56` demande toujours `bd update <ID> --add-label ready-for-review` (utilisé par `cmd/start_dev_mode.go:185`) | Un seul modèle de statut (label vs statut vs claims) ; colonne REVIEW alimentée |
| F-03 | ❌ OPEN | ❌ OPEN | `cmd/v5_claims.go:57-62` : claim libéré si ticket non fermé ; goroutine nue `v5_claims.go:118` ; aucune expiration dans `teamstate/claims.go` | Ne pas libérer si `cp-2` reporté ; expiration après crash ; libération hors goroutine fragile |
| F-07 | 🟡 PARTIAL | 🟡 PARTIAL | `cmd/v5_inputs_gitlab.go` inchangé (seul `gitlabapi`) | Entrées `mr.*` pour GitHub |
| F-08 | 🟡 PARTIAL | 🟡 PARTIAL | `conductor.md`, `orchestrator.md`, `orchestrator-dev.md` toujours dans `hubcontent/hub/agents/planning/` | Un seul coordinateur ; fragments de checkpoints partagés ; `beads.allow` factorisée |
| F-11 | ❌ OPEN | ❌ OPEN | `cmd/help.go:54-66` : `status`, `metrics`, `dashboard`, `serve`, `history` toujours dans la section « analytics » ; `board`/`team board` présents | Une vue de suivi |
| F-12 | 🟡 PARTIAL | 🟡 PARTIAL | Quatre implémentations de jokers : `limits/limits.go:190` (`Match`), `credproxy/proxy.go:828` (`wildcard`), `opencodev2/render.go:366`, `bundle/permissions.go:189` | Un seul `wildcard` partagé |
| F-13 | 🟡 PARTIAL | 🟡 PARTIAL | `--dry-run` existe pour `gc`, `purge`, `fanout clean`, `secrets cleanup`, pas pour `oh run` ; `cmd/budget.go` = coût du jour seulement ; pas de commande d'annulation | `oh run --dry-run`, coût par projet/workflow, annulation de session |

### Design et ergonomie (U)

| ID | Audit | Actuel | Preuve | Reste à faire |
|----|-------|--------|--------|---------------|
| U-01 | ❌ OPEN | ❌ OPEN | 568 lignes : **non mesurable sans exécuter** `oh --help`. Mesurable : 52 appels `rootCmd.AddCommand` (46 à `c3b36496`) ; 43 commandes listées dans les sections de `helpSectionOrder` (`cmd/help.go:54-66` : 6+4+5+1+3+5+6+13) | Réduire la surface (P2) |
| U-04 | ❌ OPEN | ❌ OPEN | Modes `manuel`/`semi-auto`/`auto`, workflows `libre`, `cadrage` inchangés | Identifiants anglais + alias |
| U-06 | 🟡 PARTIAL | 🟡 PARTIAL | `layout.Build/New` toujours utilisé par `views/board.go`, `teamboard.go`, `dashboard.go`, `wizard.go` | Migrer ces 4 vues vers le shell |
| U-07 | 🟡 PARTIAL | 🟡 PARTIAL | `sessions_view.go` : seul `clipText` rendu rune-safe (`n <= 0`) ; ligne d'aide à 19 entrées inchangée | Revérifier le rendu en terminal étroit |
| U-09 | 🟡 PARTIAL | 🟡 PARTIAL | `cmd/init_steps.go` inchangé (descriptions masquées sous 30 lignes) | Afficher les descriptions en terminal court |
| U-10 | ❌ OPEN | ❌ OPEN | `cmd/init_wizard.go` inchangé | Accueil en 3 étapes |
| U-12 | ❌ OPEN | ❌ OPEN | `docs/reference/glossary.*` inchangés ; « mode » = agent / workflow / navigation / configuration | Renommer ou lever l'ambiguïté |

### Architecture et qualité (A)

| ID | Audit | Actuel | Preuve | Reste à faire |
|----|-------|--------|--------|---------------|
| A-01 | 🟡 PARTIAL | 🟡 PARTIAL | **47 fichiers non-test** de `cmd/` importent `teamstate` (37 à `c3b36496`, 47 à `541e7c06`, 47 maintenant ; 59 tests compris). `cmd/` : 215 fichiers non-test (326 avec tests). `tui_team_actions.go` = 434 lignes ; fonctions de 500 à 740 lignes : `buildViews` (739), `buildColumnEditorStep` (590), `actionTeamRejoin` (560), `buildCommands` (521), `actionTrackerDiscovery` (498) | Service équipe/tracker ; synchro tracker CLI/TUI en un seul endroit |
| **A-02** | 🟡 PARTIAL | **✅ FIXED** | `f05f74b3` : `cmd/runner.go:246` passe par `safego.Go` (plus de `go` nu) ; `archtest/goroutines_test.go` garde maintenant `cmd/runner.go` et `cmd/daemon*.go` (`guardedDirs`) ; `cmd/v5_inprocess.go:44` déjà sous `safego`. grep : plus aucun `go` nu dans `daemon`, `credproxy`, `runsvc`, `gateway` | Aucun. Hors périmètre du constat : ~40 `go` nus de `cmd/` (redessin tview, `serve.go:176`, `v5_claims.go:118`, `eval_fanout.go:52`), documentés comme volontairement hors garde |
| A-03 | 🟡 PARTIAL | 🟡 PARTIAL | `internal/archtest` = `goroutines_test.go` + `tool_independence_test.go` ; pas de règles de couches ; faille `!tag` toujours là (`tool_independence_test.go:55-56` : `strings.Contains(expr.String(), tag)` accepte `!integration`) | Règles de couches ; corriger l'analyse des tags |
| A-04 | 🟡 PARTIAL | 🟡 PARTIAL | `daemon/daemon.go:193-203` : `mu` protège encore 8 champs (`clients`, `verified`, `verifyFails`, `policies`, `pending`, `lastBusy`, `queuedGroups`, `periodicBusy`) | Scinder `mu` |
| A-05 | ❌ OPEN | ❌ OPEN | `go.mod` : `viper v1.21.0`, `go-figure`, `go-difflib v1.0.0`, `tview v0.42.0` + `bubbletea v1.3.10`/`huh v1.0.0` ; `go.mod` non modifié depuis l'audit | Retrait de viper/go-figure/go-difflib ; une seule bibliothèque TUI |
| A-06 | 🟡 PARTIAL | 🟡 PARTIAL | `ci.yml` : opencode épinglé (version + SHA-256, `2.0.25`) ; job d'intégration limité aux paquets ayant des tests `integration` (`1453595a`) ; e2e/conteneur hors CI par décision ; **toujours** 16 `time.Sleep` dans `daemon/*_test.go` ; `cmd/integration_test.go` toujours sans tag de build. Nouveau : la CI Windows ne teste que 82 paquets listés (`.github/windows-test-packages.txt`), **27 exclus** (`windows-test-packages-excluded.txt`) | Résorber les `Sleep`, tagger `cmd/integration_test.go`, planifier e2e/conteneur, réduire les exclusions Windows |
| A-07 | ❌ OPEN | ❌ OPEN | `storage/sqlite/store.go` et `migrate.go` inchangés ; `MigrateDown` (`store.go:186`) jamais appelé hors tests | Rollbacks v24–v34 ou suppression de `MigrateDown` / de `irreversible: false` |
| A-08 | ❌ OPEN | ❌ OPEN | `config/legacy_keys.go`, `bundle/models.go`, `limits/limits.go` inchangés | Unifier les cascades, dater les clés héritées |
| A-09 | 🟡 PARTIAL | 🟡 PARTIAL | Toujours 0 test : `mcp/figma`, `mcp/gslides`, `gateway/beadswire`, `adapters` (registre), `sessionresults` ; pas de `cmd/purge*_test.go` | Écrire ces tests |

### Documentation et release (D)

| ID | Audit | Actuel | Preuve | Reste à faire |
|----|-------|--------|--------|---------------|
| D-01 | ❌ OPEN | ❌ OPEN | Dernier tag `v4.2.0` ; `CHANGELOG.md:10` `## [Unreleased]`, puis `## [4.2.0]` l.1155 (≈1 145 lignes sous Unreleased). `release.yml` sur tag `v*` en place. Prérequis de santé vérifiés : `golang.org/x/text v0.42.0` (≥ 0.39), golangci-lint v2.14 et govulncheck dans `ci.yml` | Couper `[5.0.0]`, tagguer |
| D-03 | ❌ OPEN | ❌ OPEN | 12 pages de `docs/dev/` (ligne 3) portent « ADR 038 à 048 » / « ADRs 038 to 048 » (`progress-bar{,.fr}`, `performance-optimizations{,.fr}`, `cli-migration-plan-v2`, `cli-remaining-work`, `git-hooks`, `cli-migration-analysis`, `file-locking`, `shell-gotchas`, `roadmap-audit`, `fiabilisation-agents-en-cours`). Le dossier `docs/architecture/adr/` va désormais jusqu'à **055** (051 à 055 ajoutés) | Remplacer par « ADR 038 et suivants » (ou 038 à 055) |

## 3. Échantillon de constats ✅ FIXED revérifiés (12) : aucune régression

| ID | Vérification dans le code actuel | Résultat |
|----|----------------------------------|----------|
| S-01 | `services/remote/runner/git.go` : doc « Finish never runs git in the agent's repository », copie de la base d'objets dans un dépôt neuf, `gitutil.IsolatedEnv/IsolatedArgs`, `GIT_ASKPASS` ; fichier non modifié depuis l'audit | OK |
| S-03 | `services/remote/fetch.go:262` `bundle.Verify(tmp, hash)` avant adoption du paquet téléchargé ; `bundle/integrity.go:31` | OK |
| S-04 | `services/remote/replay.go:139-140` `bdargs.Parse`, option inconnue refusée | OK |
| S-06 | `gateway/beads.go:106-124` : mode hook seulement avec `consumeHookToken` (403 sinon) ; `BD_GIT_HOOK` posé par la passerelle seulement (`beadsEnv`, l.320), supprimé de l'environnement hérité (l.340) ; `gateway/hooktoken.go` + test | OK |
| S-07/S-08 | `gateway/bdargs/` (analyseur `bd`, `TestParseCombinedShorts`), `gateway/beads.go:176` fermeture via `bdargs.IsRead` / verrou ; `gateway/base.go`, `paths.go` | OK |
| S-09 | `services/checkpoint/locks.go` : commit non vide, arbre propre (« an empty commit does not count », l.134) | OK |
| S-21 | `credproxy/region.go:17` `^[a-z]{2}(-[a-z]+)+-\d{1,2}$` + `region_test.go` | OK |
| S-24 | `storage/sqlite/server_store.go:22-44` : mots de passe en fichiers 0600 par groupe, hash `sha256:` en base | OK |
| S-30 / F-16 | `internal/skillregistry` supprimé ; `cmd/skill.go:9` « disconnected in v5 (ADR-051) » | OK |
| S-33 | `runtime/container/spec.go:214` `--cap-drop=ALL`, `--security-opt=no-new-privileges` | OK |
| S-36 | `cmd/serve_auth.go` : contrôle de l'en-tête Host (`serveAllowedHost`), cookie `HttpOnly`/`SameSite=Strict` | OK |
| F-01 | `cmd/beads_clean.go:30-44` `[beads] zero_impact` (ask/…), confirmation | OK |
| (aussi) S-23, S-29, S-31, S-35, F-04, F-05, U-02, U-05, D-04 | `server_store.go:309` (erreur de désérialisation = refus), `daemon/revoke.go`, `mcpresolve/trust.go`, `workflows/quick.yaml:33` et `debug.yaml:44` (`unlocks: [commit, close]`), `cmd/start.go:18` et `audit_review_debug.go:105,124` (`Deprecated`), `cmd/exitcodes.go:15-16`, `config/remote.go:12` (`Experimental`) | OK |

Aucun commit de type revert depuis le 2026-10-20 (`git log --grep=revert` vide). Les modifications de `credproxy` et `daemon/budgetgate.go` (tech-leftovers) ajoutent le comptage des flux coupés en dollars sans retirer la réservation (S-27, F-04 toujours en place).

## 4. Affirmations chiffrées

| Affirmation | Valeur actuelle |
|-------------|-----------------|
| U-01 : `oh --help` 568 lignes | **Non mesurable sans exécuter.** Indicateurs statiques : 43 commandes de premier niveau visibles dans `helpSectionOrder` (41 à l'audit), 52 `rootCmd.AddCommand` (46 à `c3b36496`, 52 déjà à `541e7c06`) |
| A-01 : 47 fichiers de `cmd/` importent `teamstate` | **47 (hors tests)**, toujours vrai ; 59 tests compris. Précision à ajouter : « hors tests » |
| A-05 : dépendances | inchangées (viper, go-figure, go-difflib, tview + bubbletea/huh) ; `go.mod` : `go 1.26.4` |
| D-01 : tag v5 / CHANGELOG | pas de tag v5 (dernier `v4.2.0`) ; `[Unreleased]` l.10, `[4.2.0]` l.1155 |
| D-03 : « ADR 038 à 048 » | 12 pages concernées ; ADR 051 à 055 existent maintenant |
| A-06 : 16 `time.Sleep` du démon | toujours 16 (`internal/daemon/*_test.go`) |
| Bilan 4.10 | 50/20/13 annoncé = en réalité 50/21/12 ; nouveau : 53/18/12 |

## 5. Nouveau bilan chiffré (même format que la section 4.10)

### Bilan des statuts (2026-10-21)

| Axe | ✅ FIXED | 🟡 PARTIAL | ❌ OPEN | ❔ non vérifié | Total |
|-----|---------|-----------|--------|---------------|-------|
| Sécurité (S) | 36 | 6 | 0 | 0 | 42 |
| Fonctionnel (F) | 9 | 4 | 3 | 0 | 16 |
| Design et ergonomie (U) | 5 | 3 | 4 | 0 | 12 |
| Architecture et qualité (A) | 1 | 5 | 3 | 0 | 9 |
| Documentation et release (D) | 2 | 0 | 2 | 0 | 4 |
| **Total** | **53** | **18** | **12** | **0** | **83** |

Avant (tel qu'annoncé) : 50 / 20 / 13. Avant (recompté sur les lignes) : 50 / 21 / 12. Passent à FIXED : S-28, S-38, A-02.

## 6. Nouveaux constats (relecture, non exhaustif)

Aucun nouveau constat de sécurité exploitable. Points à noter :

1. **N-1 (info, 🟡) — Couverture réelle de l'épingle npm (S-38).** `pinned_integrity.json` ne contient que `2.0.20`, la plage supportée est `2.0.0–2.99.99` et la CI utilise `2.0.25` : en pratique, la plupart des installations n'ont qu'un avertissement (`oh runner install` n'a pas de configuration de hub, donc avertit seulement). À traiter dans la checklist de release (épingler la version recommandée).
2. **N-2 (🟡, qualité) — CI Windows partielle.** 27 paquets exclus, dont `daemon`, `gateway`, `runsvc`, `bundle`, `config`, `storage/sqlite`, `selfupdate`, `termlaunch`. Or le démon tourne dans `oh` sous Windows (A-02). Documenté (`windows-test-packages-excluded.txt`), mais à lier à A-06 et à annoncer avant une release v5 « multiplateforme ».
3. **N-3 (⚪, i18n) — Libellés d'options en dur** malgré le garde-fou U-11 : `cmd/gc.go:50` (français : « Afficher ce qui serait supprimé… »), `cmd/secrets.go:333` (anglais), `cmd/start.go:43` (français). À vérifier si `TestEveryHelpIsTranslated` les couvre ; sinon U-11 n'est pas strictement complet.
4. **N-4 (⚪, doc) — Prompt `prompt.BuildDevPrompt`** (`internal/prompt/dev.go:47,56`) impose encore le label `ready-for-review` : preuve vivante de F-02 (à ajouter au constat).
5. **N-5 (doc d'audit)** — fragment orphelin dans `audit-v5.md` l.219-227 (ancien bilan du 12/10 sans en-tête de tableau ni titre) à supprimer ou à retitrer.

## 7. Corrections à apporter à `docs/dev/audit-v5.md` (sans modifier le fichier)

Numéros de lignes de la version actuelle du fichier.

1. **l.6 (en-tête « Statut »)** : « mis à jour le 2026-10-20 » → « 2026-10-21 » ; ajouter `tech-leftovers` à la liste des lots (après `keychain-mcp`) ; remplacer « Bilan des 83 constats : **50 FIXED, 20 PARTIAL, 13 OPEN, 0 non vérifié** (12/10 : 34, 11, 37, 1 ; détail en section 4.10) » par « Bilan des 83 constats : **53 FIXED, 18 PARTIAL, 12 OPEN, 0 non vérifié** (20/10 : 50, 21, 12 — l'annonce « 50, 20, 13 » était une erreur de comptage ; 12/10 : 34, 11, 37, 1 ; détail en section 4.10) ».
2. **l.122 (S-28)**, colonne Statut : « ✅ FIXED — v5-fix-h2 (client qui vérifie le dossier, le socket et le processus avant d'envoyer, dossier fermé 0700 avant le socket, flux coupés à la révocation, environnement du serveur en liste blanche `execution.env_allow`) ; tech-leftovers (en-têtes et paramètres de la query relayés selon une liste blanche par fournisseur et par route, noms retirés journalisés au niveau debug, `credproxy/relay.go`) ».
3. **l.137 (S-38)**, colonne Statut : « ✅ FIXED — v5-fix-sec (binaire en cache revérifié par son SHA-256) ; tech-leftovers (intégrité sha512 des quatre paquets Linux épinglée dans `pinned_integrity.json`, téléchargement refusé si npm annonce une autre valeur, version non épinglée : avertissement, refusée sous `execution.strict_isolation` ; `adapters/opencodev2/pins.go`). Réserve : seule la version 2.0.20 est épinglée » (variante prudente : conserver 🟡 PARTIAL avec ce texte).
4. **l.188 (A-02)**, colonne Statut : « ✅ FIXED — v5-fix-h2 (goroutines de `daemon`, `credproxy`, `runsvc` et `gateway` sous `safego`, `TestNoBareGoInCriticalPackages`) ; tech-leftovers (démon du job distant lancé par `safego.Go` dans `cmd/runner.go`, garde étendue à `cmd/runner.go` et `cmd/daemon*.go`) ».
5. **l.121 (S-27)** et **l.152 (F-04)** (facultatif) : ajouter « ; tech-leftovers : réponse coupée comptée en dollars (`daemon/cutestimate.go`) » (S-27) et « ; tech-leftovers : le merger reçoit une part du budget restant comme les enfants » (F-04).
6. **l.170 (U-01)** : « … `oh --help` fait désormais 568 lignes (477 à l'audit) ; au 21/10, 43 commandes visibles dans l'aide et 52 appels `rootCmd.AddCommand` (non remesuré en lignes, binaire non lancé) ».
7. **l.179 (F-02)**, colonne Statut (facultatif) : ajouter « ; `internal/prompt/dev.go:47,56` demande toujours le label `ready-for-review` ».
8. **l.187 (A-01)** : préciser « 47 fichiers de `cmd/` (hors tests) importent `teamstate` » ; ajouter `actionTrackerDiscovery` (498 lignes) à la liste des grosses fonctions (facultatif).
9. **l.192 (A-06)** : « 🟡 PARTIAL — ci-fix : opencode épinglé (version et SHA-256), e2e volontairement hors CI (décision), conteneur non lancé en CI ; modernisation tests : un seul passage de tests avec `-race` et couverture par système, job d'intégration limité aux paquets qui ont des tests `integration` ; restent 16 `time.Sleep` dans les tests du démon, `cmd/integration_test.go` sans tag, et 27 paquets exclus de la CI Windows (`.github/windows-test-packages-excluded.txt`) ».
10. **l.203 (D-03)** : « ❌ OPEN — 12 pages historiques de `docs/dev/` portent encore « ADR 038 à 048 » ; les ADR vont désormais jusqu'à 055 : écrire « ADR 038 et suivants » ».
11. **l.206 (titre 4.10)** : « (2026-10-21) ».
12. **l.210 et l.213-215 (tableau 4.10)** : Sécurité `| 36 | 6 | 0 | 0 | 42 |` ; Architecture `| 1 | 5 | 3 | 0 | 9 |` ; Total `| **53** | **18** | **12** | **0** | **83** |` (Fonctionnel, Ergonomie, Documentation inchangés).
13. **l.217 (paragraphe des lots)** : remplacer « H (N14, v5-fix-h2 : S-29, S-40 ; A-02 et A-04 partiels) » par « H (N14, v5-fix-h2, tech-leftovers : S-29, S-40, A-02 ; A-04 partiel) » ; « C (… ; S-28 partiel) » → « S-28 » traité ; remplacer « une partie de S-11, S-20, S-38 et S-39 » par « une partie de S-11, S-20 et S-39 » et ajouter « Le lot tech-leftovers (20/10, soir) clôt S-28, S-38 (avec réserve) et A-02 ; les lots ci-fix, ci-fix-2 et « modernisation tests » (CI, outils de test communs, correctifs : étiquettes des boards d'équipe, `Config.UpsertTeam`, écritures du fan-out) ne changent aucun statut hors A-06 (précisé) ».
14. **l.219-227** : supprimer ce fragment orphelin (ancien bilan du 12/10 sans en-tête), ou le reprendre sous un titre « Bilan du 2026-10-12 » avec la ligne d'en-tête du tableau.
15. **Section « Lots du 20/10 » (l.273-289)** : ajouter une puce « tech-leftovers : S-28, S-38, A-02 ; S-27 et F-04 complétés » ; dans la liste « Reste en P2 ou au backlog » (l.289) retirer « S-28 (en-têtes et query relayés) », « S-38 (empreinte épinglée) » et « A-02 (`cmd/runner.go`) » ; dans la ligne « architecture » (l.288) ne rien changer (A-01, A-03, A-05 à A-08 ; ajouter A-06 et A-09 si l'on veut être exhaustif).
16. **l.267 (P2, Architecture)** (facultatif) : ajouter « réduire les exclusions de la CI Windows » à côté de « e2e et conteneur planifiés en CI (A-06) ».

## 8. Ce qui reste OPEN de plus important avant une release v5

1. **D-01** : couper `[5.0.0]` dans le CHANGELOG et tagguer (prérequis techniques en place : `release.yml` sur tag, `x/text` 0.42, lint et govulncheck en CI).
2. **S-11 / S-12** (🟡) : l'isolation stricte est désactivée par défaut et les verrous restent des garde-fous ; acceptable s'il est annoncé (`SECURITY.md` le dit), mais à garder en tête dans la communication de release.
3. **F-02 / F-03** : modèle de statut incohérent (label vs statut vs claims, colonne REVIEW vide) et claims libérés à tort en mode équipe : seuls vrais défauts fonctionnels ouverts de gravité 🟠.
4. **S-13** : jeton MCP par groupe (sessions d'un groupe non distinguées) ; **S-39** : Cosign facultatif ; **S-38** : épingle limitée à 2.0.20 (épingler la version recommandée à la release).
5. **A-06 / N-2** : CI Windows partielle (27 paquets exclus dont daemon et gateway) alors que le démon tourne dans `oh` sous Windows.
6. Le reste (U-01, U-04, U-10, U-12, F-08, F-11, A-01, A-05, A-07, A-08, D-03) est de la dette P2 sans effet bloquant pour une v5.0.
