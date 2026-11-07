# Plan — Simplification de la suite de tests et corrections rapides

> Issu de l'audit du 21/10/2026 (code, architecture, affichage, tests).
> Périmètre : `cli/` (module `github.com/datichb/openhub/cli`).
> Exécution : **4 vagues**, avec **une session de tests entre chaque vague** (voir § « Vagues »).
> Le contenu technique reste découpé en étapes : 1 vitesse, 2 fixtures communes, 3 suppressions, 4 bugs, 5 fusions, 6 parallélisme et sleeps.
> **Pour reprendre demain : lire d'abord le § « É. État au 21/10/2026 au soir et reste à faire » ci-dessous.**
> **22/10/2026 : plan clos (tests validés, CHANGELOG et audit mis à jour). La suite est dans `release-v5.0.md` puis `release-v5.1.md`.**
> **22/10/2026 au soir : v5.0.0 publiée.** Suivis É.3 réglés depuis : isolation `USERPROFILE` sous Windows, tests instables de la CI (daemon, procinfo, shell, fanout sous Windows), dossier `TestOhGC` (non revérifié). Les autres suivis É.3 et É.4 sont repris dans `release-v5.1.md`.

---

## É. État au 21/10/2026 au soir et reste à faire

### É.1 État actuel

- **Les 4 vagues de développement sont faites et fusionnées en local sur `main`** (dernier merge : `1de669e5`, vague 4). Tout est en local : **122 commits d'avance sur `github/main` au 22/10 (dont 14 merges ; + CHANGELOG et audit), RIEN n'est poussé** (dernier push : 20/10 à 20 h 28).
- Fusions du jour : A `3cb1e6f7` (script de mesure, CI en un passage, job integration limité), C `274ee598` (bd hors -short, `Runner.Prepare`), B `bd1e1611` (cache de bundle), H `bd2e9a54` (étiquettes des boards), G `48f5eb28` (outils communs + `UpsertTeam`), I `bd775af6` (écritures du fan-out), J `9c5930c7` (tests instables, couverture), F `ef52f1a4` (TUI), D1 `6bed577a`, E `49d61e7c`, D2 `34a16efa`, L `d1362a16` (copie profonde, classement Windows), K `1de669e5` (sleeps, t.Parallel).
- **Validations** : vagues 1, 2 (après reprise J) et 3 (après vérification ciblée au calme) validées ; **session de tests 4 (finale, `1de669e5`) : 0 échec dans les deux suites, `-short` 107 s (−59 %), `-race` 384 s (−61 %), goldens intacts ; **validée** (réserve levée le 22/10, voir É.7.8) : seule baisse = `internal/beads` en `-short` (65,7 → 56,9 %, tests du vrai `bd` sortis de `-short` en vague 1 ; 65,8 % en suite complète) → décision É.7.8. Détail dans le § J et `openhub-wt/mesures/final/RESUME.md`.
- **Chiffres** (référence du matin → maintenant) : suite `-short` 261 s → ~170 s (avant vague 4) ; suite `-race` 974 s → ~350 s au calme (avant vague 4 ; la vague 4 a encore accéléré fanout 154→41 s, teamstate 109→35 s, daemon 86→68 s, cmd 211→176 s en isolation `-race`) ; tests de premier niveau 2 963 → 2 504 ; −4 150 lignes de tests (vague 3) ; couverture : aucune baisse ; goldens : aucun contenu perdu.
- **Bugs corrigés aujourd'hui** (tous avec test fonctionnel) : étiquettes/badges/assignés invisibles sur les boards d'équipe ; `oh init` qui écrasait `Teams[0]` ; fan-out en échec à tort après un arrêt ; écritures du fan-out avalées (`fixDone`) et session enfant orpheline ; copie profonde des réglages qui modifiait la config avant l'enregistrement ; langue des messages qui fuyait d'un test à l'autre ; 3 tests instables (`TestParallelScheduling`, `TestWatchdog`, ordre de démarrage des enfants du fan-out).
- **Worktrees** : aucun (nettoyage du 22/10 : `openhub-wt/tests` retiré, dossier `openhub-wt/` supprimé). Toutes les branches `feat/tests-*` sont fusionnées et supprimées ; seule reste `main`.
- **Mesures** : le dossier `openhub-wt/mesures/` a été supprimé le 22/10 (nettoyage, option A) ; conservés dans `~/.opencode/plan/mesures/` : `audit-v5-recheck.md` (relecture de l'audit v5) et `RESUME-final.md` (résumé de la session finale). Les chemins `openhub-wt/mesures/...` cités dans le Journal ne sont plus disponibles.
- **Modèle des sous-sessions** : Sonnet 5.5 EU (`amazon-bedrock/eu.anthropic.claude-sonnet-5-5`), en **eu-west-1** (écart accepté : `AWS_REGION` non défini dans le service OpenCode, voir R4).

### É.2 Clôture du plan des tests (à faire en premier)

1. ~~Trancher la réserve de la session finale~~ : **fait le 22/10**, baisse acceptée, « plan des tests validé » noté dans le Journal.
2. ~~**CHANGELOG**~~ : **fait le 22/10** (`27f3e6b9`). Texte d'origine : entrée du lot « modernisation tests » (commit `docs(changelog): … (modernisation)`, comme les lots précédents) : CI en un passage et job integration limité, outils de test communs (`sqlitetest`, `gittest`, `configtest`, `golden`, `i18ntest`, `bdtest`), suite réduite et accélérée (chiffres de la session finale), bugs corrigés (liste É.1).
3. ~~**Mettre à jour `docs/dev/audit-v5.md`**~~ : **fait le 22/10** (`0b5c4836`), S-38 laissé PARTIAL (décision utilisateur, à corriger ensuite) → bilan **52 / 19 / 12** ; section 4.11 ajoutée pour N-1 à N-5. Texte d'origine : avec la relecture (voir É.6 et `mesures/audit-v5-recheck.md` § 7) : 3 statuts, bilan recalculé, fragment orphelin l.219-227 supprimé, nouveaux constats N-1 à N-5.
4. **Push (par l'utilisateur)**, puis vérifier le CI sur les 3 OS : un seul passage tests + couverture, job integration sur 4 paquets, temps réels gagnés. Vérifier aussi que la liste Windows des paquets (`.github/windows-test-packages*.txt`) passe.
5. ~~**Fichiers temporaires**~~ : **fait le 22/10** (404 `Test*` + 61 `go-build*`, ~3 Go supprimés ; worktree de mesure retiré). Texte d'origine : `$TMPDIR` contient ~337 dossiers `Test*` (~0,1 Go, du 12/10 au 21/10) et ~62 dossiers `go-build*` (~2,9 Go) laissés par des exécutions interrompues ; les supprimer s'ils ont plus d'une heure. Vérifier via la session finale que la suite complète, terminée normalement, n'en laisse pas.

### É.3 Suivis notés pendant les vagues (petits, sans priorité de release)

| Sujet | Détail |
|---|---|
| **Isolation sous Windows** | `configtest.Home` ne fixe que `HOME`, or Windows lit `USERPROFILE` (`os.UserHomeDir`) : les tests qui s'appuient dessus ne sont pas isolés du vrai profil sous Windows ; vérifier aussi `testenv.Isolate`. Fixer `USERPROFILE` ; puis remettre `configtest` dans la liste Windows testée. |
| `golden` sous Windows | `filepath.ToSlash` dans le message d'échec → paquet testable sous Windows. |
| Extensions des outils communs | `gittest.CommitAll(t, dir, msg)` ; `gittest.Run` avec variables d'environnement (dates, `BD_GIT_HOOK`) → migrer `session/decide_test`, `daemon/hooktoken_test` ; `sqlitetest.Seed` avec un nom de projet ≠ id ; `configtest` qui crée `<home>/.oh`. |
| Familles de tests non fusionnées | TUI : `launch_form*`, `beads_picker`, `sessions_*`, `shell/keys_test` ; cmd : teams, `tui_*` ; teamstate : policies, workflows ; `gateway/shellenv`, `credproxy`, `evalrun`, `peercheck`, `termlaunch` ; micro-tests `TestFanout_*`, `TestValidate_*`, `TestPatch_*` restants. |
| Couleurs en dur | `metrics_view.go`, `project_mode_view.go`, `team_mode_view.go`. |
| Lenteur de `cmd` | `TestBrickCLI` ~1 s par vérification : le code de production copie tout le hub (210 fichiers) dans `mergeBricks` à chaque vérification de brique → optimisation de production ; `cmd` reste limité en parallélisme par ses globales (`application`, `store`, `tuiShell`, cobra). |
| Segfault de `git` sous charge | Sous forte charge, des `git` fils plantent parfois (« signal: segmentation fault ») dans fanout, aussi sur l'ancien binaire ; fanout limité à 4 fixtures vivantes. À surveiller dans le CI. |
| Tests de migration | ~45 tests du code de migration et d'ancien code : à supprimer avec ce code à la fin de la fenêtre de migration. |
| Sleeps restants justifiés | teamstate `TestBusyClone`, credproxy `TestProxyHoldWaitsForRestoredGrants`, checkpoint `service_test` (rien n'indique que la goroutine attend déjà). |
| Dossier temporaire laissé | `cmd/gc_test.go` `TestOhGC` laisse un dossier `$TMPDIR/TestOhGC<n>` à chaque exécution (~245 accumulés depuis le 12/10) : trouver ce qui échappe au nettoyage de `t.TempDir` (processus encore actif, fichier rendu non supprimable…). |
| `cmd`, chemin critique | 378 s sous `-race` dans la suite finale (357 s à la référence) : le paquet le plus long ; voir « Lenteur de `cmd` ». |

### É.4 Reste à faire en dehors des tests (chiffres du 21/10 sur `main`)

**A. Bugs et robustesse**
| # | Sujet | État |
|---|---|---|
| A1 | Copie profonde des réglages | ✅ corrigé (L) |
| A2 | Fan-out : `fixDone` et session orpheline | ✅ corrigé (K) |
| A3 | Classement des 5 nouveaux paquets pour Windows | ✅ fait (L) ; reste l'isolation `USERPROFILE` (É.3) |
| A4 | Statuts de l'audit v5 à revérifier | ✅ relu (M), mise à jour du fichier à faire (É.2.3) |

**B. Cohérence et réutilisation de l'affichage** (demande initiale de l'audit)
| # | Sujet | État actuel | Ce qu'on ferait | Effort |
|---|---|---|---|---|
| B1 | Formateur commun | 5 fonctions « il y a… » (seuils et textes différents, « 02 Jan » non traduit), 23 fonctions de troncature, 53 formats de date en dur (6 styles, `.Local()` parfois absent), coût `$%.2f`/`$%.3f`, 6 fonctions booléennes, 2 de tailles | Un paquet partagé CLI + TUI, un test fonctionnel, remplacement partout | 1 j |
| B2 | Couleurs et icônes | 92 balises de couleur construites à la main (contournent `theme.ColorTag` et `NO_COLOR`), 6 couleurs nommées, deux `ColorTag`, ~150 icônes en dur, symboles à plusieurs sens (● actif/réclamé/modifié ; ✗ échoué/invalide) | Un sens par symbole (`theme`/`glyph`), un seul `ColorTag`, test garde-fou | 1 j |
| B3 | Chargement asynchrone | Bloc recopié dans ~22 vues, 4 sans garde de génération (`history`, `metrics`, `doctor`, `gc`), 4 formes d'état « chargement », 17 clés | Helper `asyncMount` + état chargement / vide / erreur unique | 1 j |
| B4 | Deux moteurs d'écran en trop | `views.RunWizard` (~450 lignes, recopie le rendu de l'assistant intégré, 9 appelants dans `cmd/`), `layout.Build` (5 usages : board, dashboard, board d'équipe, wizard ; `Ctrl+C`/`Ctrl+N`, textes non traduits), kanbans en double | Tout sur le shell et `InlineWizardView`, retrait de `RunWizard` et `layout.Build` | 2-3 j |
| B5 | Asymétries Hub / Projet / Équipe | Auto-enregistrement (500 ms ou 200 ms) vs touche `w` en Équipe ; `w` a 3 sens ; refus wiki d'équipe sans consigne ; widgets de liste différents (Projets vs Équipes) ; bannière en majuscules seulement en Équipe ; icônes d'événements CLI ≠ TUI | **Décision utilisateur (ADR)**, puis alignement | 1 j après décision |
| B6 | Petites duplications | `FilterableList` utilisé une seule fois, deux filtres recopiés (`wiki_proposals`, `bricks`), ~94 touches lues sans `keys`, pied de page de l'accueil copié 3 fois | Composant de filtre, `keys.Own`, pied de page commun | 1 j |
| B7 | Fonctions géantes de la TUI | `ShowInlineForm` (~440 lignes), `renderStep` (~340), `buildColumnEditorStep` (complexité cognitive 437), `buildViews` (~275-739 lignes), `globalKeyHandler` | Découpage (B4 en retire une partie) | 1-2 j |

**C. Architecture et maintenabilité**
| # | Sujet | État actuel | Ce qu'on ferait | Effort |
|---|---|---|---|---|
| C1 | Couches non vérifiées | 23 fichiers de `cmd/` importent `storage/sqlite` ; `app.App` n'expose que 6 stores sur 14 ; SQL brut dans `cmd/team_rejoin_core.go`, `doctor_db.go` | `app.App` complet + règles de couches dans `archtest` (corriger aussi la faille `!tag`, A-03) | 1 j |
| C2 | Services manquants | team (moteur tracker ×3, token GitLab ×4 avec priorités différentes, init d'équipe ×3), launch (`prepareWorkflowRun`, ~140 lignes, 7 appelants), history (export/import ×2 divergents), policy (`policies_view` écrit du TOML et fait `CommitAndPush` sur la boucle d'affichage), doctor (fausse commande cobra) | `services/team`, `launch`, `history`, `policy` appelés par la CLI et la TUI | 3-4 j |
| C3 | `teamstate` = 2e couche de persistance | Importé par 47 fichiers de `cmd/` et 14 vues ; interfaces de 24 et 27 méthodes sans `ctx` | Interfaces par sous-domaine avec `ctx`, accès via les services (après C2) | 2 j |
| C4 | Helpers dupliqués | Écriture atomique ×18 (sans `fsync`), correspondance par motif `*` ×5 (risque de dérive sur les permissions, F-12), paquet `retry` utilisé une fois pour 7 boucles maison, 4 clients GitLab, 2 clients Jira | Paquet utilitaire unique, un client GitLab | 2 j |
| C5 | Sortie de la CLI | 27 `tabwriter` (2 réglages), 51 `os.Stdout` en dur, ~20 encodeurs JSON maison, >100 lignes de succès copiées, 4 façons de confirmer (dont 12 `huh.NewConfirm`), `--force` (13) vs `--yes` | Paquet de sortie (JSON, tableau, succès vers `a.IO`), confirmation unique, `--yes` partout (`--force` en alias déprécié) | 1-2 j |
| C6 | Démon | Client et serveur dans le même paquet (`runsvc`, `services/session` récupèrent tout le serveur) | `daemon/api` + `daemon/client` | 1 j |
| C7 | Lisibilité pour un nouvel arrivant | ~99 codes de plan dans les commentaires (N13, P2-T16…), fichiers `c2_test.go`, `config_fields_rob.go`, `v5_*` (version d'oh) mélangé à `V2` (version de l'outil) | Passe de renommage et de nettoyage | 1 j |
| C8 | Dépendances (A-05) | `viper` pour un seul TOML, `go-figure`, deux bibliothèques d'interface (tview + huh/bubbletea) ; `go-difflib` sert désormais à `golden` | Retirer `viper` et `go-figure` | 1 j |

**D. Pour publier la v5**
| # | Sujet | Détail |
|---|---|---|
| D1 | Tag et CHANGELOG (D-01) | Dernier tag `v4.2.0` ; CHANGELOG ~376 Ko, tout sous `[Unreleased]` (~1 145 lignes) ; `release.yml` ne part que sur un tag `v*` |
| D2 | Validation réelle | Aucun e2e ni conteneur en CI ; mode distant jamais validé sur un vrai runner (expérimental, désactivé par défaut) ; dernière évaluation enregistrée (14/10, Haiku) : 80/105 (76 %), 4 cas à 0/3, 35 cas sur 71 seulement — **à vérifier** : c'est peut-être la copie de test (`cli/internal/evalrun/testdata/evals/baseline.json`) et non la dernière mesure. **Demande l'accord de l'utilisateur pour des appels de modèle.** |
| D3 | Mode équipe (F-02, F-03) | 3 modèles de statut, colonne REVIEW de `oh board` toujours vide, `internal/prompt/dev.go` impose `ready-for-review` ; claims libérés même si `cp-2` est reporté, pas d'expiration après plantage, libération dans une goroutine nue (`cmd/v5_claims.go:118`) |
| D4 | Ergonomie ouverte | U-01 (`oh --help` ~568 lignes, 43 commandes visibles, 52 `AddCommand`), U-04 (noms mi-français mi-anglais), U-10 (`oh init` ~8 écrans en solo), U-12 (« mode » a 4 sens), F-11 (7 vues de suivi qui se recoupent) |
| D5 | Sécurité partielle documentée | S-11 (isolation stricte non par défaut), S-12 (`make`/`npm run` hors règles), S-13 (sessions d'un même groupe non distinguées), S-15 (bash interactif), S-20 (ajouts d'agents seulement signalés), S-39 (Cosign facultatif) |
| D6 | Fonctionnel au backlog | F-07 (entrées MR GitLab seulement), F-08 (coordinateurs et checkpoints en double), F-13 (`oh run --dry-run`, annulation, coût par projet), A-07 (pas de retour arrière des migrations v24-v34), A-08 (9 sources de configuration), A-09 (paquets sans tests : `mcp/figma`, `mcp/gslides`, `gateway/beadswire`, `adapters`, `sessionresults`, `cmd/purge`) |
| D7 | Poids du contexte | 475 000 tokens de prompts sur les 14 workflows (cible du rapport de modernisation : −62 %) |
| D8 | Windows le moins vérifié | 27 paquets exclus du CI Windows (dont `daemon`, `gateway`, `bundle`, `runsvc`, `config`, `storage/sqlite`) alors que le démon tourne dans `oh` sous Windows ; isolation `HOME` inopérante sous Windows (É.3) |

### É.5 Ce qui est bloquant pour une release v5

**Bloquant**
1. **Rien n'a été vérifié par le vrai CI** : 120 commits locaux (restructuration du CI, milliers de lignes de tests, correctifs de production) jamais exécutés sous Linux ni Windows → push puis CI vert sur les 3 OS.
2. **Jamais validée avec de vrais agents** (D2) : e2e manuel + passe d'évaluation sur la version actuelle ; décider si les cas à 0/3 bloquent.
3. **Pas de version** (D1) : CHANGELOG découpé en v5.0.0 + entrée « modernisation tests » + tag.
4. ~~Bug de copie profonde~~ : corrigé le 21/10 (L).

**Bloquant sous condition**
5. **Mode équipe F-02 / F-03** : bloquant si la v5 annonce le mode équipe comme stable ; sinon l'annoncer en **bêta** et corriger en v5.1.
6. **S-38** : épingler l'empreinte npm de la version d'opencode recommandée au moment de la release (aujourd'hui seule 2.0.20 est épinglée, le CI installe 2.0.25).

**Pas bloquant** : restes de tests (É.3), cohérence de l'affichage (B), architecture (C), segments de sécurité partiels documentés comme garde-fous (D5, à rappeler dans l'annonce), ergonomie et poids du contexte (D4, D7).

**Chemin le plus court vers la v5** : (1) résultat de la session finale ; (2) CHANGELOG du lot + mise à jour de l'audit ; (3) push + CI vert 3 OS ; (4) décisions D2 (budget d'évaluation) et D3 (corriger ou bêta) ; (5) évaluation + e2e, analyse des échecs ; (6) épingle S-38, CHANGELOG v5.0.0, tag.

### É.6 État de l'audit v5 (relecture du 21/10, session M, lecture seule)

- Détail : `~/.opencode/plan/mesures/audit-v5-recheck.md` (tableaux par axe, preuves, texte des corrections à apporter à `docs/dev/audit-v5.md`, lignes 6, 121, 122, 137, 152, 170, 179, 188).
- **3 statuts passent à FIXED** : **S-28** (`b33c7c36`, liste blanche des en-têtes et de la query, `credproxy/relay.go`), **S-38 avec réserve** (`8fdab730`, `pinned_integrity.json` ; seule 2.0.20 est épinglée, le CI installe 2.0.25 : variante prudente = garder PARTIAL), **A-02** (`f05f74b3`, démon du job distant sous `safego`, `archtest` étendu à `cmd/runner.go` et `cmd/daemon*.go`).
- **Bilan** : l'audit annonçait 50 FIXED / 20 PARTIAL / 13 OPEN, en réalité 50 / 21 / 12 (erreur de comptage sur l'axe A) ; **maintenant 53 / 18 / 12** — S 36/6/0, F 9/4/3, U 5/3/4, A 1/5/3, D 2/0/2.
- **Aucune régression** sur une vingtaine de FIXED revérifiés (S-01, S-03, S-04, S-06, S-07, S-08, S-09, S-21, S-24, S-30, S-33, S-36, F-01, F-04, F-05…), aucun revert depuis le 20/10.
- **Nouveaux constats** (aucune faille exploitable) : N-1 couverture réelle de l'épingle npm (S-38) ; N-2 CI Windows partielle (27 paquets exclus) ; N-3 libellés d'options en dur (`cmd/gc.go:50`, `cmd/secrets.go:333`, `cmd/start.go:43`) à vérifier contre `TestEveryHelpIsTranslated` ; N-4 `internal/prompt/dev.go:47,56` impose `ready-for-review` (preuve de F-02) ; N-5 fragment orphelin `audit-v5.md` l.219-227.
- **Chiffres mis à jour** : U-01 43 commandes visibles / 52 `AddCommand` (lignes de `--help` non remesurées) ; A-01 47 fichiers de `cmd/` hors tests importent `teamstate` ; A-05 `go.mod` inchangé ; D-01 pas de tag v5 ; D-03 12 pages disent « ADR 038 à 048 » alors que les ADR vont jusqu'à 055 ; A-06 : 16 `time.Sleep` dans les tests du démon au 20/10 (la vague 4 les a remplacés le 21/10).
- **OPEN les plus importants pour la v5** : D-01, F-02, F-03 ; à mentionner dans l'annonce : S-11, S-12, S-13, S-39 ; à la release : S-38.

### É.7 Décisions en attente (utilisateur)

1. **B5** : quel comportement commun pour Hub / Projet / Équipe (auto-enregistrement ou `w`, sens de `w`, refus wiki avec consigne, widget de liste) ?
2. **D2** : autoriser une passe d'évaluation et un e2e lancés à la main, avec quel budget ? (interdit par R1 tant qu'il n'y a pas d'accord explicite)
3. **D3** : corriger F-02 / F-03 avant la v5, ou publier le mode équipe en bêta ?
4. ~~**S-38**~~ — **décidé le 22/10 : reste PARTIAL**, à corriger dans un lot suivant (épingler la version recommandée). Question d'origine : FIXED avec réserve, ou PARTIAL tant que la version installée par le CI n'est pas épinglée ?
5. **Priorité après la clôture** : release v5 d'abord (D1-D3, ~2-3 j + évaluation), ou affichage et architecture d'abord (B et C, ~3 semaines) ? Recommandation : clôture (É.2), puis D1-D3 jusqu'au tag, avec B1-B3 en parallèle (fichiers séparés) ; B4, C2, C3 pour une v5.1.
6. ~~**Fichiers temporaires**~~ (accordé et fait le 22/10) : accord pour supprimer les `Test*` et `go-build*` de plus d'une heure dans `$TMPDIR` (~3 Go).
7. **Région** : régler `AWS_REGION=eu-west-3` dans le service OpenCode (puis `opencode service restart`) si les prochaines sous-sessions doivent tourner à Paris (aujourd'hui eu-west-1, voir R4).
8. ~~**Réserve de la session finale**~~ — **acceptée le 22/10 par l'utilisateur** (à documenter dans le CHANGELOG du lot) : accepter la baisse de couverture `-short` de `internal/beads` (65,7 → 56,9 %, conséquence voulue de la vague 1 : les tests qui lancent le vrai `bd` ne tournent plus en `-short`, et `beads` reste à 65,8 % dans la suite complète, celle du CI), en la documentant dans le CHANGELOG du lot — **recommandé** ; ou remettre ces 5 tests en `-short` (~120 s de plus en local). Si accepté : « plan des tests validé ».

---

## R. Règles impératives (à lire avant de commencer, valables pour toutes les sessions)

### R1. Interdictions strictes
- **Interdit de pousser** : aucun `git push` (ni `--force`, ni vers une autre remote), aucune création de MR/PR, aucune commande qui publie (`gh`, `glab`, `oh remote …`, `oh team … push`, `CommitAndPush` déclenché à la main, `goreleaser`, `make release`…).
- **Interdit de lancer des évaluations ou des tests e2e avec de vrais agents** : pas de `make test-e2e`, pas de `-tags e2e`, pas de `make test-eval`, pas de `oh eval …`, pas de `oh run` / `oh start` / session réelle, aucun appel à un modèle (aucune variable `OH_E2E_*`, aucun jeton Bedrock/Anthropic/OpenRouter exporté pour les tests).
- Tolérés seulement si une étape l'exige et **sans aucun identifiant de modèle** : `make test-integration` (contrat contre `opencode serve`, sans appel de modèle) et `make test-container`. Par défaut, ne pas les lancer.
- Pas de modification de `~/.oh`, `~/.config/opencode` ni de la configuration globale git.

### R2. Un worktree dédié par session de développement
- Chaque session de développement travaille **dans son propre worktree**, jamais dans `/Users/benjamin.datiche/workspace/openhub` :
  ```bash
  cd /Users/benjamin.datiche/workspace/openhub
  git worktree add ../openhub-wt-<lot> -b feat/tests-<lot> main
  ```
  (la session est **créée directement dans ce dossier**, voir R4 : pas créée dans `openhub/` puis déplacée).
- Un commit par paquet ou par sujet, messages au format du dépôt (`test: …`, `ci: …`, `fix: …`, `feat: …` + `(modernisation tests)`).
- Pendant le développement, la session ne lance **que les tests de ses paquets** (`go test ./internal/<paquet>/... -count=1`), jamais la suite complète (la machine est partagée avec les autres sessions de la vague).
- Pas de changement de comportement du code de production, sauf : étape 1.d (`Runner.Prepare`) et étape 4 (bugs).

### R3. Fusion sur `main` une fois terminé (en local uniquement)
- Une session est terminée quand : ses tests de paquet passent, `go vet` et `golangci-lint` sont propres sur ses paquets, et son récapitulatif est écrit (fichiers touchés, tests supprimés/fusionnés, risques).
- Fusion **locale** sur `main`, une branche à la fois, dans l'ordre indiqué par la vague :
  ```bash
  cd /Users/benjamin.datiche/workspace/openhub
  git checkout main
  git merge --no-ff feat/tests-<lot>
  git worktree remove ../openhub-wt-<lot> && git branch -d feat/tests-<lot>
  ```
- Si `main` a bougé entre-temps : `git rebase main` dans le worktree, relancer les tests de ses paquets, puis fusionner. En cas de conflit sur un fichier d'un autre lot : s'arrêter et le signaler, ne pas résoudre au hasard.
- **Jamais de push** après la fusion (R1).

### R4. Modèle des sous-sessions
- Toutes les sous-sessions (développement et sessions de tests) sont lancées avec **Claude Sonnet 5.5 EU** : `amazon-bedrock/eu.anthropic.claude-sonnet-5-5`.
- Région : **eu-west-3** (Paris) demandée, par **la variable d'environnement `AWS_REGION=eu-west-3`** (option retenue le 21/10/2026). Aucune sous-session ne modifie ce réglage elle-même.
- **Écart accepté le 21/10/2026 (vague 1)** : `AWS_REGION` n'était pas défini dans l'environnement du service OpenCode, et le redémarrer aurait coupé la session d'orchestration. L'utilisateur a accepté que les sous-sessions tournent en **eu-west-1** (profil EU `eu.anthropic.claude-sonnet-5-5`, données dans l'UE). À revoir avant les vagues suivantes si `AWS_REGION` est réglé entre-temps.
- Ce qu'il faut savoir (doc OpenCode V2, page Providers, section Bedrock) : *« Without an explicit region, OpenCode uses `AWS_REGION`, then `AWS_DEFAULT_REGION`, then `us-east-1` »*. Une région écrite dans une configuration **l'emporte** sur `AWS_REGION`.
  - `openhub/opencode.json` (local, ignoré par git via `.gitignore:23`) fixe `amazon-bedrock.options.region = "eu-west-1"` : une session **située dans `openhub/`** part donc en eu-west-1 malgré `AWS_REGION`.
  - Les worktrees `../openhub-wt-<lot>` ne contiennent pas ce fichier (ignoré) et n'ont pas `openhub/` parmi leurs dossiers parents ; la config globale `~/.config/opencode/opencode.jsonc` ne fixe pas de région. Une session **située dans un worktree** utilise donc bien `AWS_REGION`.
  - Conséquence : les sous-sessions doivent être **créées directement dans leur worktree** (et non créées dans `openhub/` puis déplacées) ; la session de tests est elle aussi lancée depuis un dossier sans région configurée (par exemple un worktree de test en lecture seule sur `main`, `../openhub-wt-tests`).
  - Les réglages locaux de `openhub/.opencode/` (ignoré via `.git/info/exclude`) et de `openhub/opencode.json` (agents, permissions, plugin `context-mode`) ne s'appliquent pas non plus dans les worktrees : c'est voulu, les règles R1-R3 sont données dans le prompt de chaque sous-session.
- `AWS_REGION` doit être présent dans l'environnement **du service OpenCode** (le serveur en arrière-plan qui exécute les sessions), pas seulement dans le terminal client : exporter `AWS_REGION=eu-west-3` puis redémarrer le service depuis ce terminal (`opencode service restart`), ou lancer avec un serveur privé (`AWS_REGION=eu-west-3 opencode --standalone`).
- **Vérification avant la vague 1** : dans le journal du serveur (`~/.local/share/opencode/log/opencode.log`, lignes `role=server`), contrôler que les requêtes Bedrock de la première sous-session partent vers `bedrock-runtime.eu-west-3.amazonaws.com`. Si ce n'est pas le cas : s'arrêter et régler avant de continuer.

### R5. Responsables uniques des fichiers partagés
- `.github/workflows/ci.yml` : session A (vague 1) uniquement.
- `docs/guides/contributing.fr.md` / `.en.md` : session G (vague 2) ; les autres lui signalent ce qu'il faut ajouter (ou l'ajoutent après la fusion de G, en vague 3).
- `docs/dev/tui-architecture.md`, `docs/dev/tui-conventions.md`, `docs/dev/tui-testing.md` : session F (vague 3).
- Plan (ce fichier) : seule la session de tests ajoute ses résultats dans le § « Journal ».

### R6. Maximum 3 sessions de développement simultanées
Au-delà, la relecture et les fusions deviennent le goulot. Une session en attente démarre quand une autre est fusionnée.

---

## V. Vagues

Chaque vague = sessions de développement en parallèle (worktrees, R2) → fusions locales successives (R3) → **session de tests** (seule sur la machine).

### Session de tests (entre chaque vague)
Lancée seule (aucune autre session active), dans un worktree de test à jour sur `main` (`git worktree add --detach ../openhub-wt-tests main` — `main` étant déjà extrait dans `openhub/`, le worktree de test est en HEAD détaché ; après chaque fusion : `git -C ../openhub-wt-tests checkout --detach main`), modèle et région R4. Elle ne modifie pas le code : si quelque chose casse, elle le note et la vague est rouverte (correctif dans un nouveau worktree).
1. `cd cli && go vet ./... && golangci-lint run`
2. `make test` (suite complète `-race`, **sans** `integration`, `e2e`, `container`, `eval`)
3. Durées : script de mesure (étape 1.f ; avant la fusion de 1.f, la commande `go test -json` du § 0) en `-short` puis complet, comparés à la mesure précédente.
4. Couverture par paquet (`-coverprofile`), comparée paquet par paquet à la mesure précédente : **aucune baisse** tolérée.
5. Vérification que les goldens n'ont pas changé (`git diff --stat <tag-précédent> -- '**/testdata/**'`), sauf goldens volontairement ajoutés en vague 3.
6. Résultats consignés dans le § « Journal » de ce plan (date, durées, couverture, écarts, décision : vague validée / à reprendre).

Avant la vague 1 : une **session de tests de référence** (mêmes points, sur `main` actuel), pour avoir le point de départ sous les mêmes conditions.

### Vague 1 — Vitesse (étape 1) : 3 sessions en parallèle
| Session | Contenu | Fichiers | Ordre de fusion |
|---|---|---|---|
| A | 1.f script de mesure, 1.a CI un seul passage, 1.e job d'intégration limité | `ci.yml`, script, `docs` sur le CI | 1ʳᵉ |
| B | 1.b cache de `bundle` puis `t.Parallel()` | tests de `internal/bundle` | 2ᵉ |
| C | 1.c `RequireRealBd`, 1.d `Runner.Prepare` | tests de `beads`, `bdschema`, `cmd` (bd), `evalrun` + champ `Runner.Prepare` | 3ᵉ |
→ **Session de tests 1.**

### Vague 2 — Fixtures communes et bugs : 3 sessions en parallèle
| Session | Contenu | Ordre de fusion |
|---|---|---|
| G | 2.a-2.d : création de `sqlitetest`, `gittest` (Run, Repo, Commit, BareClone), `configtest`, `golden` + leurs tests minimaux + doc contributing (y ajouter aussi le paragraphe proposé par la session C de la vague 1 : `bdtest.RequireRealBd` / `bdtest.InitedRepo`, ignorés en `-short` ; `Runner.Prepare` d'`evalrun` pour éviter git) ; **puis** 4.2 `config.UpsertTeam` (son test utilise `configtest`) | 1ʳᵉ |
| H | 4.1 étiquettes du board d'équipe + son test | 2ᵉ |
| I | 4.3 écritures du fan-out (`WithoutCancel`, `record`, erreurs critiques) + son test | 3ᵉ |
Les migrations vers les nouveaux outils **ne sont pas** faites ici (vague 3).
→ **Session de tests 2.**

### Vague 3 — Nettoyage par groupe de paquets (étapes 2-migration, 3, 5, et 4.4)
Chaque session fait, **dans ses paquets** et dans cet ordre : migration vers les outils communs → suppressions (3) → fusions (5) → goldens. Un fichier n'est ouvert qu'une fois.
| Session | Paquets | En plus |
|---|---|---|
| D1 | `daemon`, `runsvc`, `services/*`, `storage/sqlite` | réécrire `newFixture` (fanout) sur les outils communs |
| E | `cmd`, `config`, `app`, `teamstate`, `tracker`, `mcp/*`, `i18n`, `selfupdate`, `provider` | 3.d (`session_launch_test.go`, après vérification de `provider`) ; 4.4 : suppression de `internal/task`, paramètre `notifStore` ; fuite de locale signalée en vague 2 : `TestTeamRejoinCore_MemberNotFound` échoue avec `-run 'Team|Init'` (un test précédent laisse la locale en `fr`) → chaque test qui change la locale la restaure (`t.Cleanup`) ; un test des chemins `teamInitCore` / `writeTeamConfig` sur `UpsertTeam` si la migration de `cmd` le permet simplement |
| F | `tui/v2/views`, `shell`, `widgets`, `keys`, `router`, `tui/common` | 3.f (`deviations_*`) ; 4.4 : `padlines.go`, troncatures, `tui-architecture.md` ; restes signalés par la session H (vague 2) : `[yellow]` en dur dans `activity_view.go` (~l.185, 205), `status_view.go` (~l.192, 218, 231), `policies_view.go` (~l.776) et dans les clés i18n `tui.teamboard.empty_*` → `theme.ColorTag` ; `launch_form.go` ~l.307 : `v.cfg.Origin` affiché sans `tview.Escape` ; un test unitaire en table de l'analyseur de balises des cartes (`widgets/cardcolumn.go`, `validColorTag` / `[x[]`) si la couverture du paquet `widgets` a baissé |
| D2 (démarre quand une place se libère, R6) | `bundle`, `beads`, `gateway`, `evalrun`, `adapters/opencodev2`, `gitutil`, `gc`, `worktree`, `runtime/*`, `remote/*`, `workflow`, `bricks`, reste | golden unique pour `bundle`, `opencodev2`, `ciconfig`, `widgets/graph` |
Ordre de fusion : D1 → E → F → D2.

**Suivi vague 3 (21/10/2026)** :
- Ordre réel : F fusionnée en premier (`ef52f1a4`), car terminée avant D1 et E ; périmètres disjoints, sans conflit. D2 lancée à la libération de la place de F.
- Bug de production trouvé par F (nouveau test de copie profonde, `views/config_deepcopy_test.go`) : `deepCopyConfig` / `deepCopyTeamConfig` partagent `Execution.EnvAllow`, `Limits.Models`, `Board.Columns` et le pointeur `Limits` entre la copie et l'original (une modification dans une vue de réglages touche la config d'origine avant l'enregistrement). Champs exclus du test pour l'instant (`configSharedFields`, `teamConfigSharedFields`) → **correctif à faire après la vague 3** (session dédiée : copie profonde de ces champs, puis retrait des exclusions du test).
- Restes de F : couleurs nommées en dur dans `metrics_view.go`, `project_mode_view.go`, `team_mode_view.go` ; familles de tests non fusionnées : `launch_form*`, `beads_picker`, `sessions_*`, `shell/keys_test`.
- D1 fusionnée (`6bed577a`) : 62 copies de fixtures base supprimées, helpers git locaux remplacés, `newFixture` réécrit ; couverture identique partout (sqlite `-short` 75,9→75,6 : fluctuation mesurée aussi sur `main`). Extensions de `gittest` proposées par D1 : `CommitAll(t, dir, msg)` (add -A + commit ; wiki, runner, brick_edit) et une variante de `Run` avec variables d'environnement (dates, `BD_GIT_HOOK` ; `session/decide_test`, `daemon/hooktoken_test`, non migrés pour cette raison). Si un test a besoin d'un nom de projet différent de l'id : `Seeder.ProjectNamed`.
- E fusionnée (`49d61e7c`) : cmd 436→371 tests, teamstate 196→138, config 83→54, tracker 53→32, mcp 100→71, selfupdate 27→17, app 9→2 ; ~2 450 lignes de test en moins ; aucune couverture en baisse (app 56,5→95,7, i18n 91,2→96,5). Nouveau paquet `internal/i18n/i18ntest` (`Set`/`Restore`) : fuite de locale corrigée. `internal/task` supprimé (et retiré de `.github/windows-test-packages.txt`). Restes : familles `cmd` (teams, tui_*) et `teamstate` (policies, workflows) non fusionnées ; `exec.Command("git")` marginaux dans cmd ; propositions : `configtest` qui crée `<home>/.oh`, `sqlitetest.Seed` avec un nom de projet. À surveiller en session de tests 3 : durée de `teamstate` en complet (108 s avant, 117-166 s mesurés sous charge).
- **CI Windows** : `windows-packages.sh check` avertit que 5 nouveaux paquets ne sont classés dans aucune liste : `internal/beads/bdtest`, `internal/config/configtest`, `internal/i18n/i18ntest`, `internal/storage/sqlite/sqlitetest`, `internal/testutil/golden` → à classer (testés ou exclus avec leur cause) ; avertissement seulement, pas un échec. **Fait** par la session L (`d1362a16`) : `i18ntest` et `sqlitetest` testés ; `bdtest`, `configtest`, `golden` exclus avec leur cause.
- **Suivis de la session L** : (1) `configtest.Home` ne fixe que `HOME`, or Windows lit `USERPROFILE` (`os.UserHomeDir`) : sous Windows, les tests qui s'appuient sur `configtest.Home` ne sont pas isolés du vrai profil → fixer aussi `USERPROFILE` (et vérifier `testenv.Isolate`, même cause) ; (2) `golden` : `filepath.ToSlash` dans le message d'échec rendrait le paquet testable sous Windows. Bug de copie profonde **corrigé** (`d1362a16`, `Enforced.Models` aussi partagé, trouvé au passage), exclusions retirées du test.

Notes de la session G (vague 2) pour la migration :
- `gittest.Run(t, dir, args...)` prend les arguments de git **sans** le mot `git` (le `run` local de `fanout/driver_test.go` prend la commande en premier) ;
- `golden` déclare le flag `-update` : un paquet qui l'importe doit d'abord supprimer son propre `flag.Bool("update", …)` (sinon panique au démarrage du binaire de test) ; paquets concernés : `views/action_list`, `widgets/graph`, `bundle/workflowgen`, `opencodev2/render` (compare en `JSONEq` : prévoir un `golden.AssertJSON` si besoin), `remote/ciconfig` ;
- migrer les goldens existants vers `golden.Assert` peut demander de les renommer en `*.golden` : contenu identique au caractère près, renommage seulement (`git mv`).
→ **Session de tests 3.**

### Vague 4 — Parallélisme et sleeps (étape 6) : 1 session
Une seule session (elle change les temps qu'on mesure) : 6.a sleeps, 6.b `t.Parallel()`, 6.c `fanout` / `gitutil`, revue des tests lents de `daemon`, `checkpoint`, `cmd`, `services/workflow`, `storage/sqlite`. **Priorité après la vague 1 : `cmd` (341 s sous `-race`, devenu le chemin critique de la suite complète), puis `daemon` (84 s même en isolation).**

**Suivi vague 4 (21/10/2026)** : session K fusionnée (`main` après `d1362a16`), 13 commits. Isolé `-p 1` : fanout 120→33 s (race 154→41), teamstate 109→35 s, daemon 53→34 s (race 86→68), cmd 164→135 s (race 211→176) ; couverture identique ou en hausse ; sleeps à risque remplacés (daemon, checkpoint/locks, shell, views) ; `t.Parallel` sur fanout 43/44, teamstate 133/138, workflow 132/133, bricks 30/32, config 37/52, views 74/225, cmd 11 tests ; correctif fan-out (fixDone journalisé, session enfant arrêtée si le lancement n'est pas enregistré). Points d'injection en production (valeurs par défaut inchangées) : `daemon.commitProbeTimeout` (10 s), `daemon.unknownTTL` (5 s) — tests de `daemon` non parallèles, donc pas de course. Restes : 3 sleeps justifiés (teamstate `TestBusyClone`, credproxy `TestProxyHoldWaitsForRestoredGrants`, checkpoint `service_test`) ; `cmd` limité par ses globales et par la copie complète du hub (210 fichiers) dans `mergeBricks` à chaque vérification de brique (`TestBrickCLI` ~1 s par vérification) → optimisation de production à envisager ; `gateway` non parallélisable (`slog.SetDefault` global). **Risque à surveiller en session de tests 4** : sous forte charge, des `git` fils plantent parfois (« signal: segmentation fault ») dans fanout ; vu aussi sur l'ancien binaire ; limité à 4 fixtures vivantes à la fois.
→ **Session de tests 4 (finale)** : comparaison avec la session de référence ; mise à jour de la doc sur le CI et des durées.

---

## J. Journal (rempli par les sessions de tests)
| Date | Session | Durée `-short` | Durée complète | Couverture totale | Paquets en baisse | Décision |
|---|---|---|---|---|---|---|
| 21/10/2026 | Référence (7ac4a25c) | 261 s (274 s mur) — 4099 pass, 0 fail, 136 skip | 974 s `-race` (1004 s mur) — 4242 pass, 0 fail, 7 skip ; bundle 964 s, cmd 357, fanout 225, services/workflow 188, evalrun 184, teamstate 176, beads 164 | 61,2 % (moyenne simple sur 101 paquets, fichier `coverage-by-package.tsv`) | — | Référence. Goldens : 342 fichiers, sha256 `94922074…5dba79`. Mesures : `openhub-wt/mesures/reference/`. Lint : 0 issue avec caches vierges (le cache partagé entre worktrees donne des faux positifs → utiliser `GOLANGCI_LINT_CACHE` propre à chaque worktree). |
| 21/10/2026 | Après vague 1 (bd1e1611) | 151 s (−42 %) — 4096 pass, 0 fail, 143 skip (+4 tests nouveaux, 6 tests bd réels passés en skip en -short) | 347 s `-race` (−64 %) — 4246 pass, 0 fail, 7 skip ; bundle 965→161, evalrun 184→28, cmd 341 (devenu le chemin critique), fanout 177, beads 174, daemon 159 | 2 baisses : beads 65,7→56,8 en -short (tests bd réels ignorés ; 65,7 % hors -short), tui/v2/shell 58,6→58,5 (instable, non touché) | aucune inexpliquée | **Validée.** Goldens identiques (342, `94922074…dba79`). Paquets apparemment plus lents (gateway, ciconfig, mcp/team, container, gitutil, daemon, docstest, gc, mcpregistry) : effet de concurrence, code inchangé ; en isolation `-race -p 1` gitutil 9 s, daemon 84 s. Mesures : `openhub-wt/mesures/vague1/`. |
| 21/10/2026 | Après vague 2 (bd775af6) | 167,7 s (+11 % vs v1, machine chargée : load 14-21, tous les paquets +10-20 % y compris non touchés) — 1 fail : `testenv TestWatchdog` (instable, préexistant) | 397,6 s `-race` (+15 % vs v1, même dérive) — 1 fail : `evalrun TestParallelScheduling` (instable depuis la vague 1 : ordre des lignes de journal) ; en isolation `-p 1`, paquets modifiés identiques à v1 (fanout 122,8 vs 121,4 s) | 3 baisses expliquées : fanout 82,6→81,1 (branches d'erreur ajoutées), widgets 38,8→38,0 (analyseur réécrit couvert seulement via views), bundle 93,8→93,7 (non déterministe) ; nouveaux : configtest 100, sqlitetest 100, golden 77,8, gittest 58,8→81,2 | — | **À reprendre** (2 tests instables). Goldens identiques. +10 tests, aucun disparu. 3 nouveaux tests stables 5/5. Reprise : session J (`feat/tests-j`) : TestParallelScheduling, TestWatchdog, couverture fanout et widgets. Mesures : `openhub-wt/mesures/vague2/`. |
| 21/10/2026 | Après vague 2 + reprise J (9c5930c7) | 166,9 s (−36 % vs réf) — 4127 pass, 0 fail, 143 skip | 362,3 s `-race` (−63 % vs réf) — 4277 pass, 0 fail, 7 skip (machine chargée, load 9-23) | fanout 83,4 % (v1 82,6), widgets 44,8 % (v1 38,8) ; bundle et daemon −0,1 non déterministes (relances : valeurs v1) | aucune inexpliquée | **Vague 2 validée.** Goldens identiques. Stabilité : TestParallelScheduling `-race -count=200 -cpu 1,4,8`, TestWatchdog `-count=100`, tests fanout `-race -count=5`, widgets `-count=20` : tous verts. Mesures : `openhub-wt/mesures/vague2bis/`. |
| 21/10/2026 | Après vague 3 (34a16efa) | 173,8 s — 4141 pass, 0 fail, 136 skip | 3 passages `-race` sous charge 150-170 (machine saturée : Chrome, Mattermost, VM, agents de sécurité) : passage 2 = 0 fail (4283 pass, 1125 s) ; passages 1 et 3 = échecs liés à la charge (`daemon TestDaemonBeadsActorAndProvenance` : `git rev-parse` > 10 s ; `fanout` ×6 : délai de 30 s de `runDriverErr` dépassé) ; durées inexploitables. **Vérification ciblée au calme (charge 9,4 → 7,4)** : `go test ./internal/services/fanout ./internal/daemon -race -count=3 -p 1` → exit 0, fanout 231 pass (418,6 s pour 3 passages), daemon 450 pass / 3 skip (248,1 s), 695 s au total | aucune baisse en `-short` ; `internal/task` disparu, `i18ntest` nouveau | — | **Vague 3 validée.** Goldens : 0 contenu de référence perdu (256 R100, 29 A, aucun M/D). Tests de premier niveau 2963 → 2504 (−459), total avec sous-tests 4270 → 4277 ; −4150 lignes de `*_test.go`. Non faits : couverture complète, `teamstate` isolé (→ session de tests 4). Instabilités sous forte charge (délai fixe de 30 s du fan-out, lecture du journal bd sans attente, `git rev-parse` borné à 10 s) → vague 4. Mesures : `openhub-wt/mesures/vague3/` (`check-fanout-daemon.json`). |
| 21/10/2026 | Après vague 4 — session finale (1de669e5) | **107,0 s** (réf 260,7 s : −59 %) — 4144 pass, 0 fail, 136 skip ; charge 18,6 | **383,8 s** `-race` (réf 974,3 s : −61 %) — 4286 pass, 0 fail, 8 skip, aucun data race, aucun segfault git ; charge 7,2 au lancement ; bundle 964→166, evalrun 184→49, fanout 225→182, teamstate 176→136, cmd 357→378 (chemin critique), beads 164→234 (durées de paquet sous concurrence) | vs réf : seule baisse persistante **`internal/beads` 65,7→56,9 en `-short`** (5 tests du vrai `bd` sortis de `-short` en vague 1, attendu ; 65,8 % en suite complète) ; daemon 84,8→84,7 fluctuation ; hausses : app +39,2, tui/v2/layout +33,3, gittest +22,4 ; couverture complète : moyenne simple 64,4 % sur 105 paquets | beads (`-short`, attendu) | **Validé sous réserve** que l'utilisateur accepte la baisse `-short` de `beads` (documentée ; déjà expliquée et acceptée aux vagues 1 et 2) — sinon remettre les 5 tests en `-short` (~120 s). Goldens : 0 contenu perdu (256 R100, 29 A, 0 M, 0 D ; 341 fichiers en réf). Tests de premier niveau exécutés 2949 → 2505 ; `func Test` 3062 → 2749 ; total avec sous-tests 4235 → 4280 ; `*_test.go` 709 → 816 fichiers, −2 056 lignes nettes depuis la réf. `$TMPDIR` : aucun `go-build*` laissé ; `cmd/gc_test.go` `TestOhGC` laisse un dossier `TestOhGC<n>` à chaque exécution (~245 accumulés depuis le 12/10). Mesures : `openhub-wt/mesures/final/`. |
| 22/10/2026 | Clôture (utilisateur) | — | — | — | beads (`-short`, accepté) | **Plan des tests validé.** Baisse `-short` de `internal/beads` (65,7→56,9 %, 65,8 % en suite complète) acceptée par l'utilisateur, à documenter dans le CHANGELOG du lot. Nettoyage : worktree `openhub-wt/tests` et dossier `openhub-wt/` supprimés (mesures utiles copiées dans `~/.opencode/plan/mesures/`), 404 `Test*` + 61 `go-build*` supprimés de `$TMPDIR` (~3 Go). |

---

## 0. Règles techniques communes (s'appliquent à chaque étape)

### Style de test retenu
1. **Un test par scénario, pas par fonction** : on vérifie le résultat visible (état en base, état git, écran rendu), pas les étapes internes.
2. **Plusieurs assertions par test** : `require` pour les préconditions, `assert` pour les vérifications (un passage remonte toutes les régressions).
3. **Un message par assertion** (`"l'espace solo est conservé"`).
4. **Table de cas** quand c'est la même fonction avec des entrées différentes, avec `t.Run(tc.name, …)` et des noms explicites.
5. **Golden** pour le rendu d'écran, **assertions explicites** pour le comportement (touches, callbacks).
6. Réutiliser les fixtures communes (étape 2) ; pas de nouveau helper local qui duplique un helper commun.

### Mesures obligatoires avant / après chaque étape
```bash
cd cli
# Durée par paquet et par test (référence)
go test ./... -short -count=1 -json -timeout 30m > /tmp/oh-tests-<etape>-<avant|apres>.json
# Couverture par paquet
go test ./... -short -count=1 -coverprofile=/tmp/oh-cover-<etape>-<avant|apres>.out
go tool cover -func=/tmp/oh-cover-<etape>-<avant|apres>.out | tail -1
```
Critères d'acceptation de chaque étape :
- `go vet ./...` et `golangci-lint run` propres (config du dépôt) ;
- `make test` (suite complète `-race`) verte ;
- **couverture de chaque paquet ≥ avant** (comparer paquet par paquet, pas seulement le total) ;
- durée ≤ avant (sauf étape 4 qui ajoute 3 tests).

### Référence mesurée le 21/10/2026 (local macOS, `-short`, sans `-race`)
| Indicateur | Valeur |
|---|---|
| Durée réelle | **4 min 37** |
| Durée cumulée des paquets | 1 446 s |
| Fonctions `Test` | 3 019 (709 fichiers, ~103 k lignes) |
| Tests < 0,1 s | 2 139 tests, **23 s** au total |
| Tests > 1 s | 231 tests, **1 189 s** (82 %) |
| Paquets les plus lents | bundle 274 s, services/fanout 161 s, beads 160 s, evalrun 149 s, cmd 147 s, daemon 87 s, runsvc 61 s |

Note : sur ce poste, un appel `git` coûte ~150 ms (20 × `git --version` = 3,1 s). Les tests riches en `git`/`bd` sont donc plus lents en local qu'en CI.

### Référence CI (dernier run Ubuntu, `-race`, relevé par une autre session)
| Paquet | Durée |
|---|---|
| internal/bundle | **1 004 s (≈ 17 min)** — plus que tous les autres paquets réunis |
| internal/daemon | 105 s |
| internal/services/checkpoint | 105 s |
| cmd | 92 s |
| internal/services/workflow | 62 s |
| internal/services/fanout | 59 s |
| internal/storage/sqlite | 53 s |
| internal/runsvc | 50 s |
| autres | < 25 s chacun (dont beads et evalrun) |

Tests les plus lents de bundle en CI : `TestBudgetBaseline` 159 s, `TestCapabilities` 92 s, `TestPendingGuardShipped` 71 s, `TestNoSpuriousCatalogue` 71 s, `TestAgentSkillReachability` 49 s, `TestRuleLedger` 43 s, 15 autres de 10 à 35 s.

Lecture croisée local / CI :
- **bundle est la priorité absolue** dans les deux mesures ;
- beads et evalrun sont lents surtout en local (git ~150 ms par appel ici, `bd` absent des runners CI) : les pistes 1.c et 1.d sont des gains **locaux** ;
- la CI fait tourner la suite complète **3 fois** : `Test` (-race) et `Coverage` sur chacun des 3 OS, plus le job d'intégration (macOS) qui relance `./...` en entier avec `-tags integration`.

---

## Étape 1 — Vitesse de la suite

Objectif : bundle en CI de ~17 min à quelques minutes ; un seul passage de la suite par OS ; job d'intégration limité aux paquets concernés ; local `-short` de 4 min 37 à ~2 min.

Règles propres à l'étape 1 :
- **aucun test supprimé ni affaibli** : la même vérification, faite plus vite (les suppressions sont l'étape 3) ;
- **les goldens restent identiques au caractère près** (aucun `-update` pendant l'étape) ;
- mesures sur une machine au calme (pas d'autre suite de tests en parallèle pendant la mesure).

### 1.a CI : un seul passage (le plus gros gain)
**Constat** : `.github/workflows/ci.yml` lance la suite deux fois par OS (3 OS) : étape `Test` (l. 78, `-race`) puis étape `Coverage` (l. 82, sans `-race`).

**Changement** : fusionner les deux étapes :
```yaml
- name: Test and coverage
  run: |
    go test ./... -race -count=1 -coverprofile=coverage.out -covermode=atomic -timeout "$GO_TEST_TIMEOUT"
    COVERAGE=$(go tool cover -func=coverage.out | grep "^total:" | awk '{print $3}' | tr -d '%')
    echo "Total coverage: ${COVERAGE}%"
    THRESHOLD=60
    if awk -v c="$COVERAGE" -v t="$THRESHOLD" 'BEGIN { exit !(c < t) }'; then
      echo "Coverage ${COVERAGE}% is below threshold ${THRESHOLD}%"
      exit 1
    fi
```
- Retirer `-v` (journaux illisibles) ou le garder seulement à la relance d'un échec (`go test -json` + résumé, optionnel).
- Mettre à jour le commentaire de tête (l. 14-16) et `docs/` qui décrit ce que lance le CI (commit `67a9ce28` l'a documenté).

**Gain attendu** : −40 à −50 % de la durée du job Go sur chaque OS. **Risque** : nul.

### 1.b `internal/bundle` : construire une seule fois les workflows livrés
**Constat** : 29 fichiers appellent `shippedWorkflows(t)` (52 appels, `shipped_workflows_test.go:24`) qui recharge le hub (`hubcat.LoadWorkflows`, `hubcat.New`, `NewSkillCatalog`) ; puis chaque test refait `Build(...)` pour chaque workflow, parfois × 3 projets (`none`, `capProject`, `instructProject`).
Tests lents : `TestBudgetBaseline` 39 s, `TestNoSpuriousCatalogue` 20 s, `TestCapabilities` 19 s, `TestAgentSkillReachability` 15 s, `TestShippedWorkflowBundles` 9,5 s, `TestCheckpointByMembers` 9,6 s, `TestShippedSkillMentionsResolveTransitively` 9 s, `TestShippedCheckpointProtocol` 8,7 s, `TestQuestionRefusedToPureSubagents` 8,5 s, `TestShippedWorkflowContracts` 8,2 s, `TestReturnTitlesFromContract` 8,1 s, `TestRuleLedger` 7,8 s, `TestPhaseSkillsHaveTrigger` 7,2 s, `TestPendingGuardShipped` 6,9 s, `TestShippedWorkflowSkillMentionsResolve` 6,7 s, `TestNoDeadRoleBlocksInAgents` 6,2 s…

**Changement** (fichiers de test uniquement) :
1. `shippedWorkflows` mémorise hub + catalogue + env au niveau du paquet (`sync.Once`) : données en lecture seule.
2. Nouveau `shippedBuild(t, project, ref) *Bundle` : cache `map[clé]*Bundle` protégé par mutex, clé = (projet, workflow) ; `OutDir` dans un dossier créé par `TestMain` (`internal/bundle/main_test.go`, qui appelle déjà `testenv.Isolate`) et supprimé à la fin.
3. `capProject` / `instructProject` créés une fois (dans ce même dossier) et mémorisés.
4. **Règle** : seuls les tests qui **lisent** le bundle passent par le cache ; un test qui modifie `Bundle`/`Spec` ou les fichiers produits garde son `Build` propre. Revue test par test (liste ci-dessus en priorité).
5. Garde-fou : dans `TestMain`, après `m.Run()`, vérifier qu'aucun fichier du cache n'a changé (empreinte des fichiers prise à la création) ; échec explicite sinon. Pour les structures en mémoire (`*Bundle`, `Spec`), le cache rend une **copie défensive** si un test en a besoin en écriture, sinon la valeur partagée en lecture seule.
6. Cache par workflow **et par couche/projet** (`sync.Once` par clé) : un test qui ne demande qu'un workflow ne construit que celui-là.
7. `t.Parallel()` sur les tests de bundle indépendants (ceux qui lisent le cache et n'utilisent ni `t.Setenv` ni `os.Chdir`) : `TestMain` isole déjà l'environnement au niveau du processus (`testenv.Isolate`). Vérifier par `go test ./internal/bundle -race -count=3`.
8. Priorité de migration, dans l'ordre des durées CI : `TestBudgetBaseline`, `TestCapabilities`, `TestPendingGuardShipped`, `TestNoSpuriousCatalogue`, `TestAgentSkillReachability`, `TestRuleLedger`, puis les 15 tests de 10 à 35 s.

**Gain attendu** : bundle en CI de ~1 004 s à quelques minutes ; en local ~270 s → ~60-80 s cumulées.
**Risque** : mutation cachée du cache → couvert par le garde-fou du point 5 et par `-race`.

### 1.c Tests avec le vrai `bd` : hors du mode `-short`
**Constat** : les tests qui lancent le vrai `bd` (Dolt) ne sont ignorés que si `bd` est absent, pas en `-short`.
Occurrences de `exec.LookPath("bd")` **qui tournent sans tag** (les seules concernées par `-short`) :
- `internal/beads/query_test.go:45,97,133`
- `internal/beads/bdschema/bdschema_test.go:40,59`
- `internal/beads/diag_bd13_test.go:57,167`
- à vérifier aussi : `cmd TestPrintBeadsCleanCommit` (30 s), `beads TestUndoCommit*`, `TestFirstCommitNeverUndone`, `TestKeepUntrackedSettings` (3-7 s chacun).

Déjà derrière le tag `integration` (non concernés par `-short`, mais à faire passer par le même helper pour l'homogénéité) : `internal/services/remote/bd_integration_test.go:20,64`, `internal/gateway/beads_real_test.go:26,152,193`, `internal/runsvc/service_gateway_hooks_test.go:115`, `internal/runsvc/service_gateway_local_test.go:83`.

Tests lents concernés : `TestCleanImpactAfterRealBdInit` 35 s, `TestPrintBeadsCleanCommit` 30 s, `TestInstalledBdViewFields` 28 s, `TestCleanImpactUndoesBdInitCommit` 27 s, `TestTrackingRealBd` 22 s, `TestLintRealBd` 18 s, `TestQueryRealBd` 13 s.

**Changement** :
1. Un helper unique `RequireRealBd(t) string` (ex. dans `internal/beads/bdtest` ou `internal/testenv`) : `t.Skip` si `testing.Short()` **ou** si `bd` est absent, message unique ; retourne le chemin de `bd`. Remplace les ~14 `LookPath("bd")` + skips ad hoc.
2. Suite complète (`make test`) : pour les tests qui n'ont pas besoin d'un `bd init` neuf, `bd init` une fois par paquet dans un modèle puis copie du dossier par test. Les tests qui vérifient les effets de `bd init` (`TestCleanImpactAfterRealBdInit`, `TestCleanImpactUndoesBdInitCommit`) le gardent.
3. Vérifier dans les journaux CI si `bd` est présent sur les runners (probablement non : seul `oh-bd` est compilé pour les conteneurs). Documenter le résultat dans `docs/` (ce que le CI lance / ne lance pas).

**Gain attendu** : ~−150 s en local `-short`. Effet CI probablement faible (à confirmer).

### 1.d `internal/evalrun` : pas de git pour les tests d'ordonnancement
**Constat** : chaque passe appelle `PrepareClone` (`runner.go:309` : copie de la fixture + ~8 commandes git ; `runner.go:200`). `TestParallelReportsMatchSequential` (46 s) fait 24 passes ≈ 200 appels git pour vérifier l'ordre et l'égalité des rapports.
Tests concernés : `TestParallelReportsMatchSequential` 46 s, `TestEscalation` 27 s, `TestBudgetTotal` 15 s, `TestThrottlingRetry` 13 s, `TestParallelScheduling` 9 s, `TestRunnerWithVariants` 8 s.

**Changement** :
1. Code de production : champ `Runner.Prepare func(ctx context.Context, c *Case, dest string) (string, error)` ; `nil` = `PrepareClone` (comportement inchangé). Même principe que `Exec` déjà injectable.
2. Tests d'ordonnancement / budget / escalade / retry : préparation factice (création du dossier, commit fictif constant).
3. `PrepareClone` reste couvert par `TestPrepareCloneAndChanges` et `TestRunnerAndReport` (git réel), et `TestPrepareVariantOnTheShippedHub`.

**Gain attendu** : evalrun ~150 s → ~25 s en local (gain faible en CI, où evalrun prend < 25 s). **Risque** : faible.

### 1.e CI : job d'intégration limité aux paquets qui ont des tests `integration`
**Constat** (vérifié) : le job « Go CLI integration » (macOS, `ci.yml` l. ~163) lance `go test ./... -race -count=1 -tags integration`, donc **toute la suite** une troisième fois (bundle compris, ~17 min), alors que seuls **4 paquets** ont des fichiers `//go:build integration` (hors `e2e`, non lancé en CI) :
- `internal/adapters/opencodev2` (7 fichiers + `main_test.go` en `!integration || windows`)
- `internal/runsvc` (4 fichiers)
- `internal/gateway` (`beads_real_test.go`)
- `internal/services/remote` (`bd_integration_test.go`)

**Changement** : calculer la liste automatiquement pour qu'un nouveau paquet soit pris en compte sans retoucher le CI :
```bash
pkgs=$(grep -rlE --include='*_test.go' '^//go:build .*\bintegration\b' . \
  | xargs -n1 dirname | sort -u | sed 's|^\.|github.com/datichb/openhub/cli|')
go test $pkgs -race -count=1 -tags integration -timeout "$GO_TEST_TIMEOUT"
```
- Exclure les fichiers dont la contrainte exige `e2e` si l'on veut éviter de compiler pour rien (optionnel : ils sont déjà ignorés sans le tag `e2e`).
- Garder un garde-fou : échouer si la liste est vide.
- Mettre à jour la doc « ce que le CI lance / ne lance pas ».

**Gain attendu** : job d'intégration de ~24 min à quelques minutes. **Risque** : faible (liste calculée).

### 1.f Script de mesure (durée par paquet et par test)
**Changement** : un petit script versionné, par exemple `cli/scripts/test-durations.sh` (ou une cible `make test-durations`), qui :
1. lance `go test <args> -json` (par défaut `./... -short -count=1`) ;
2. affiche la durée réelle, la durée par paquet (triée) et les N tests les plus lents ;
3. peut comparer deux fichiers JSON (avant / après).

Implémentation sans dépendance (Go dans `cli/internal/...` en `//go:build ignore`, ou `jq` s'il est déjà requis ailleurs : à vérifier). Documenter son usage dans `docs/guides/contributing.fr.md` et `.en.md` (section Tests), et l'utiliser pour toutes les mesures « avant / après » de ce plan.

### 1.g Hors périmètre de l'étape 1
- `internal/services/fanout` (157 s, 25 tests de 4 à 12 s : git réel + attentes du driver) → étape 6.
- `internal/gitutil` (`TestSafeGitIgnoresRepoConfig` 14 s, `TestCheckDetectsTamperedConfig` 9 s) → à regarder à l'étape 6 (nombre d'appels git).

### Ordre des commits de l'étape 1
Par ordre de gain en CI :
1. `test: test durations script` (1.f) → mesure de référence locale
2. `ci: one test run with race and coverage per system` (1.a)
3. `ci: integration job on the packages with integration tests only` (1.e)
4. `test(bundle): build the shipped workflows once per package` (1.b, sans `t.Parallel`) → mesure
5. `test(bundle): parallel read-only tests` (1.b point 7) → mesure, `-race -count=3`
6. `test: real bd tests out of -short (RequireRealBd)` (1.c, gain local)
7. `feat(evalrun): injectable Prepare` + `test(evalrun): scheduling tests without git` (1.d, gain local)
→ mesure après chaque commit ; vérifier la durée CI réelle après 2, 3 et 5.

### Hors étape 1, à garder pour l'étape 6
Revue des tests les plus lents de `daemon` (105 s CI), `services/checkpoint` (105 s), `cmd` (92 s), `services/workflow` (62 s), `services/fanout` (59 s), `storage/sqlite` (53 s) : `time.Sleep` restants, délais fixes, absence de `t.Parallel()`.

---

## Étape 2 — Fixtures communes

Objectif : supprimer ~1 200 lignes de fixtures dupliquées et préparer les étapes 3 à 6 (notamment `t.Parallel`).

### 2.a `internal/storage/sqlite/sqlitetest`
**Constat** : 81 `sqlite.Open(` et 72 `INSERT INTO projects` dans les tests :
| Paquet | Copies | Fichiers |
|---|---|---|
| internal/daemon | 31 | 28 (bloc open/cleanup/insert identique, ex. `workflow_test.go:26-31`) |
| internal/runsvc | 10 | 9 |
| internal/storage/sqlite | 10 | 6 |
| cmd | 9 | 6 |
| internal/services/session | 6 | 5 |
| services/remote, remote/runner, fanout, checkpoint, adapters/opencodev2 | 6 | 6 |

**API proposée** :
```go
st := sqlitetest.New(t)                          // base temporaire, Close au Cleanup
sqlitetest.Seed(t, st).Project("p1", "/p1")      // projet
sqlitetest.Seed(t, st).Project("p1", dir).Session(&domain.Session{...}) // chaînable
```
- Le paquet `sqlite` lui-même ne peut pas importer `sqlitetest` (cycle) : ses tests internes gardent **un seul** helper (fusionner `openTemp` `server_store_test.go:16` et `openTestStore` `project_store_test.go:15`).
- Migrer daemon, runsvc, cmd, services/*, adapters.

### 2.b `internal/gitutil/gittest` : helpers git
**Constat** : 76 `git init` et ~25 helpers locaux :
`cmd/team_cmd_test_helpers_test.go:169 testGitCmd`, `cmd/init_test.go:17 setupGitRepo`, `cmd/workflow_migrate_test.go:39 gitIn`, `internal/gitutil/safe_test.go:17 gitIn`, `internal/gitutil/session_diff_test.go:14 gitT`, `internal/runtime/container/real_test.go:129 gitRun`, `internal/worktree/worktree_test.go:211 runGit`, `internal/teamstate/claims_test.go:259 setupGitTestRepo` et `:379 gitCmd`, `internal/teamstate/repo_test.go:168 gitExec`, `internal/tracker/sync_tx_test.go:50 git`, `internal/mcp/team/handlers_test.go:140 initGitRepo` et `:151 gitCmd`, `internal/daemon/workflow_locks_test.go:35 gitRun`, `internal/beads/beads_test.go:639 setupTempGitDir`, `internal/gc/gc_test.go:265 git`, `internal/services/fanout/driver_test.go:26 run` et `:36 commit`, `internal/services/wiki/ld_test.go:103 gitRepo`, `internal/services/workflow/publish_test.go:25 git`, `internal/services/remote/runner/git_test.go:26 newGitFixture`, `internal/runsvc/finalisation_test.go:15 git`, `internal/runsvc/existing_test.go:151 gitRepo` et `:159 gitCommit`.
Motif « bare + clone » répété : `teamstate/repo_test.go` ×5, `claims_test.go:267`, `tracker/sync_tx_test.go:72`, `cmd/team_cmd_test_helpers_test.go:87`, `mcp/team/handlers_test.go:321`, `teamstate/solo_test.go:98`.

**API proposée** (dans `gittest`, qui fournit déjà `Main` / `Defaults`) :
```go
out := gittest.Run(t, dir, "rev-parse", "HEAD")   // échoue le test avec la sortie
dir := gittest.Repo(t)                            // init -b main + commit initial
gittest.Commit(t, dir, "file.go", "contenu", "msg")
bare, clone := gittest.BareClone(t)               // dépôt nu + clone prêt à pousser
```
- Les helpers respectent `gittest.Defaults()` (identité, branche `main`, maintenance coupée).
- Ne pas migrer `gateway/shellenv_modes_test.go:61 run` (ce n'est pas git).

### 2.c `internal/config/configtest`
**Constat** : 43 `Setenv("HOME")` (cmd/helpers_test.go ×8, app/app_test.go ×7, cmd/team_rejoin_core_test.go ×6, config/config_test.go ×4, …) ; `os.Setenv` avec restauration manuelle (fragile, empêche `t.Parallel`) dans `cmd/team_cmd_test_helpers_test.go`, `cmd/team_rejoin_core_test.go`, `cmd/integration_test.go`, `internal/config/migrate_test.go`.

**API proposée** :
```go
home := configtest.Home(t)  // t.Setenv("HOME", t.TempDir()) + config.Reset() + Cleanup
```
- Placé dans `internal/config/configtest` (pas dans `testenv`, pour que `testenv` n'importe pas `config`).
- Remplacer tous les `os.Setenv` des tests par `t.Setenv` / `configtest.Home`.

### 2.d `internal/testutil/golden`
**Constat** : 5 helpers golden et 5 flags `-update` :
`views/action_list_test.go:30 assertGolden` (+ flag l. 19), `bundle/shipped_workflows_test.go:350 assertGolden`, `bundle/capability_test.go:617 assertGoldenFile`, `adapters/opencodev2/render_test.go:62 goldenCompare` (+ flag l. 17), `remote/ciconfig/ciconfig_test.go:19 golden` (+ flag l. 17), flags aussi dans `widgets/graph_test.go:17`, `bundle/workflowgen_test.go:16`.

**API proposée** :
```go
golden.Assert(t, "nom", got)   // testdata/nom.golden, -update pour régénérer, diff lisible
```
- Un seul flag `-update` déclaré dans le paquet `golden`.
- Mettre à jour `docs/dev/tui-testing.md` et `tui-conventions.md` (section Tests).

### 2.e Modèle de fixture de paquet
`newFixture` (`internal/services/fanout/driver_test.go:193`) est le modèle : store + projet + dépôt + faux en un appel. Le réécrire sur `sqlitetest` + `gittest` pour servir d'exemple, et documenter le modèle dans `docs/guides/contributing.fr.md` / `.en.md` (section Tests).

### Ordre des commits de l'étape 2
1. `test: add sqlitetest, gittest helpers, configtest, golden` (avec leurs propres tests minimaux)
2. Migration paquet par paquet : daemon → runsvc → services/* → cmd → teamstate/tracker/mcp → reste
3. Doc contributing (fr/en) : section « Fixtures de test »

---

## Étape 3 — Suppressions (~75 tests)

### 3.a Vérifications de compilation écrites comme des tests (14)
- Vides : `views/views_test.go` `TestBoardView_ImplementsView`, `TestTeamBoardView_ImplementsView`, `TestHomeView_ImplementsView`, `TestProjectsView_ImplementsView` ; `views/inline_wizard_test.go` `TestInlineWizardView_ImplementsView`, `TestInlineWizardView_ImplementsInputCapturing` ; `views/standalone_test.go` `TestStandaloneShellStub_ImplementsShellAccess`.
- → remplacer par `var _ View = (*X)(nil)` dans le fichier de production (s'il n'existe pas déjà).
- Les 7 qui montent aussi la vue (Status, Metrics, Doctor, MCP, Help, TeamStatus, Worktree) rejoignent la table MountUnmount de l'étape 5.

### 3.b « Ne plante pas avec nil » (~18)
`inline_wizard_test.go` `TestIsDropDownFocused_NilForm`, `TestFixFormLabelFocus_NilForm`, `TestFixFormDropDownStyles_Export_NilSafety`, `TestFixFormLabelFocus_Export_NilSafety` ; `shell/clipboard_test.go` `TestNativeClipboardCmd_NeverPanics`, `TestCopyToClipboard_DoesNotPanicOnAnyInput` ; `TestLaunchProgressNil`, `TestRouter_CurrentNil`, `TestCloneStringMap_Nil`, `TestIsAuthError_NilError`, `TestWriteResponseNil`, `TestSyncState_KnownTickets_NilMapInit`, `TestFindBranchForTicket_NoProject`, `app_test` `TestWithSecretStoreNil` / `TestNilSecretStoreAccess`.
→ devenir une ligne de la table voisine (étape 5) ou disparaître si le cas est sans intérêt.

### 3.c Accesseurs triviaux et constantes (~55)
`internal/app/app_test.go` `TestWith*` ×4, `TestDefaultIOStreams` ; `widgets/inlineselect_test.go` (9 `Get*`) ; `widgets/stepbar_test.go` (8) ; `teamstate` `TestMembersFilePath`, `TestClaimFilePath`, `TestBrickLockPath`, `TestNewRepo` ; `TestBuild_ReturnsNonNilResult`, `TestHubContentDir_ReturnsNonEmpty`, `TestNewUndoStack_DefaultCapacity` ; `TestExitCodesAreDistinct`, `TestDoctorChecksRegistered`, `TestServiceRemoveHasAProjectFlag`, `TestStepInputs`, `TestRuntimePrefs`, `TestToolSession`, `TestHookTokenRoute`, `TestGatewayURL`, `TestSplitStatements`, `TestNotificationStore_ZeroMaxDefaultsTo50`, `TestExecutionImagesDefault`, `TestFormField_Hint*` ×4, `TestInlineFormConfig_BackwardCompatibility`, `TestAuthErrorMessage_*` ×2, `TestColorTag_*` ×2.
→ supprimer, ou intégrer comme assertion dans le test fonctionnel du composant (étape 5) quand la valeur a un sens métier.

### 3.d Test d'une copie de la fonction (vérifié)
`cmd/session_launch_test.go` : 9 tests `TestResolveCredentials_*` (l. 62-169) testent `resolveCredentials`, **définie dans le fichier de test** (l. 216), enveloppe de `provider.ResolveCredentials`.
→ **Avant de supprimer** : vérifier que `internal/provider/credentials_test.go` couvre Bedrock (token projet, repli défaut, profil AWS, surcharge projet), Anthropic (clé projet, repli), OpenRouter, fournisseur inconnu, pas de secrets ; compléter par une table si un cas manque. Garder `TestCredentialKeysUseCanonicalFormat` (l. 181) s'il teste du code de production.

### 3.e Doublons entre couches (~4)
`TestRunInlineWizardStandalone_EmptySteps` / `AllSkipped` (doublons de `TestInlineWizardView_EmptySteps` / `AllSkippedSteps`) ; `cmd/integration_test.go:TestExitCodesOfTheBinary` vs `exit_test.go` ; `TestDeprecatedCommandsOfTheBinary` vs `deprecated_commands_test.go`.
→ garder une seule couche (celle qui teste le binaire si elle est la seule à couvrir le câblage réel).

### 3.f Mécanisme des écarts de touches (vérifié)
`internal/tui/v2/keys/deviations_{cleanup,launch,sessions,wiki_bricks}_test.go` : les 4 listes sont vides (`[]string{}`).
→ supprimer les 4 fichiers et la logique « known / fixed » de `keys/convention_test.go:39-57` ; `TestKeyConvention` devient strict (tout écart = échec). Mettre à jour `docs/dev/tui-conventions.md` (section Tests, paragraphe `deviations_<lot>_test.go`).

### 3.g Hors périmètre : tests de migration et d'ancien code (~45)
`config/migrate_test.go`, `config/legacy_keys_test.go`, `config/mcpkeys_migrate_test.go`, `config/team_resolve_test.go:TestResolveTeamForProject_Legacy*`, `workflow/legacy_test.go`, `cmd/workflow_migrate_test.go`, `cmd/deprecated_options_test.go`, `cmd/deprecated_commands_test.go`, migrations sqlite V36-V49, `*BackwardCompat`.
→ **ne pas toucher** : ils partiront avec le code de migration à la fin de la fenêtre de migration.

---

## Étape 4 — Bugs (lot 1 de l'audit)

Trois tests fonctionnels au total, écrits avec les fixtures de l'étape 2.

### 4.1 Étiquettes invisibles dans le board d'équipe (confirmé par exécution)
**Cause** : tview lit `[mot]` (lettres/chiffres) comme une balise de couleur (`tview@v0.42.0/strings.go:333`) ; nom inconnu → couleur par défaut, texte avalé.
Mesuré : `[green][AI][-]` → rien ; `[gray][bug][-]` → rien ; `[gray][v2][-]` → rien ; `[yellow][!][-]` → `[!]` ; `[gray][needs-review][-]` → `[needs-review]`.
**Où** : `internal/tui/v2/views/teamboard_view.go:402` (badge IA, vue intégrée) et `:517` (`formatTicketLabels`, board plein écran `oh team board` via `teamboard.go:89`) ; `"@"+t.Assignee` non échappé (l. 388).
**Correction** :
- écrire les crochets littéraux avec l'échappement tview (`"[AI[]"`, `"[" + l + "[]"`) ou remplacer le badge par une icône `theme` ;
- `[green]/[yellow]/[gray]` → `theme.ColorTag(...)` (respect de `NO_COLOR` et du thème) ;
- **un seul rendu des étiquettes** pour les deux boards (aujourd'hui l'un filtre les étiquettes de workflow et compare aux constantes `teamstate.Label*`, l'autre non et compare à des textes en dur) ;
- échapper l'assigné (`tview.Escape`).
**Test** (`views/views_test.go`, à côté des tests TeamBoardView) : un ticket avec `agent-reviewed`, `needs-human-review`, `bug`, `needs-review`, une étiquette de workflow, assigné `[dev]` ; rendu par les deux chemins ; lecture via `GetText(true)`. Assertions : badge IA visible, `bug`, `needs-review`, `[!]`, `@[dev]` visibles ; étiquette de workflow masquée **dans les deux rendus** ; aucune balise brute (`[-]`, `[green]`) dans le texte visible.

### 4.2 `oh init` peut écraser une équipe dans `hub.toml` (confirmé)
**Cause** : `cmd/team_init_core.go:77-82` remplace toujours `Teams[0]`.
**Scénarios** : (1) espace solo en `Teams[0]` ignoré par `ActiveTeam()` (`config.go:187`) → l'assistant propose « créer » → l'espace solo disparaît ; (2) plusieurs équipes, « Reconfigurer » l'équipe par défaut qui n'est pas la première → c'est `Teams[0]` qui est écrasée ; (3) `Name` et `Solo` perdus ; projets dont le `TeamID` pointait l'équipe écrasée orphelins.
**Correction** :
- `config.UpsertTeam(cfg *Config, t TeamConfig)` dans `internal/config` : remplace sur `ID` ou `StateRepo`, sinon ajoute ; **conserve `Name`** (et ne touche pas `Solo` d'une autre entrée) ;
- l'utiliser dans `teamInitCore` et `writeTeamConfig` (`cmd/team.go:743`, qui a déjà la bonne logique sauf `Name`) ;
- corriger le commentaire de `teamInitCore` (appelants `runTeamInitFromTUI` / `runTeamCustomSetup` inexistants ; seul appelant : `cmd/team_helpers.go:469`).
**Test** (`internal/config/team_solo_test.go`) : départ `[solo, acme (Name "ACME", désactivée), beta (par défaut)]` ; (1) upsert `gamma` ; (2) upsert `beta` avec un autre membre ; (3) upsert d'`acme` retrouvée par son dépôt avec un ID différent ; puis Save + Load (modèle `save_roundtrip_test.go`, `configtest.Home`). Assertions : 4 équipes, `solo` intacte ; `beta` remplacée à sa place, `Name` conservé, autres inchangées ; pas de doublon, `acme` à jour avec son `Name` ; `ActiveTeam()` / `DefaultTeam()` corrects ; tout est encore vrai après relecture.

### 4.3 Une écriture perdue fait échouer à tort un fan-out (confirmé)
**Cause** : 20 `_, _ = d.update(...)` dans `internal/services/fanout/driver.go` (l. 219, 262, 459, 503, 585, 649, 663, 673, 909, 941, 1074, 1083, 1123, 1137, 1160, 1173, 1207) et `fix.go` (l. 89, 95, 138).
**Enchaînement** : `merge()` fusionne dans git (l. 1064) → `merged()` enregistre `Integration.Head` + `ChildMerged` (l. 1160) ; si l'écriture échoue (ctx annulé à l'arrêt de `ohd`, `SQLITE_BUSY` au-delà de `busy_timeout(5000)`), git est fusionné mais la base non → à la fusion suivante ou à la reprise, `HEAD != Integration.Head` (l. 1059) → `tampered()` → fan-out en échec avec un message trompeur.
**Gravité des écritures ignorées** :
| Gravité | Lignes | Effet si perdue |
|---|---|---|
| Critique | 1160, 1083, 1123, 262 | état incohérent avec git ou fan-out bloqué |
| Moyenne | 219, 503, 585, 909, 1173, fix.go 89, 95 | enfant « en cours » alors qu'il a échoué |
| Faible | 649, 663, 673, 941, 1074, 1137, 1207, fix.go 138 | budget / journal incomplet |

**Correction** :
1. `update` écrit avec `context.WithoutCancel(ctx)` + délai court (~5 s) : un état déjà acquis dans git est enregistré même pendant l'arrêt ;
2. `d.record(ctx, parent, fn)` : `update` + journalisation de l'échec via `d.logf` ; remplace les 20 `_, _ =` ;
3. transitions critiques : remonter l'erreur pour arrêter proprement le driver plutôt que de continuer sur un état faux.
**Test** (`internal/services/fanout/driver_test.go`, fixture `newFixture`) : fan-out à 2 tickets ; enveloppe du store (~10 lignes) qui **annule le contexte du driver** au moment d'enregistrer la fusion de `ev-1`, et fait échouer une écriture secondaire (budget) ; relance du driver avec un contexte neuf. Assertions : fusion de `ev-1` enregistrée (`ChildMerged`, `Integration.Head == HEAD` git) ; après reprise `FanoutReady`, 2 enfants `ChildMerged` ; aucun événement `tampered`, aucun enfant en échec ; 2 commits de fusion ; parent prévenu une fois ; l'échec secondaire apparaît dans `d.Log` ; dossier projet intact.

### 4.4 Petits correctifs sans test dédié
- `views/teams_view.go:401 truncateString` : coupe en octets → couper en runes avec `…` (ou attendre le formateur unique du lot « affichage ») ; `views/sessions_view.go:1068 clipText` : protéger `n <= 0`.
- Code mort : supprimer `internal/task` (aucun importeur) et `internal/tui/common/padlines.go` (`PadLines`/`PadLinesAuto` jamais appelées, commentaire citant `common.Surface` inexistant) ; retirer le paramètre inutilisé `notifStore` de `buildViews` (`cmd/tui_views.go:96`). Ne **pas** supprimer `internal/platform` (7 importeurs).
- `docs/dev/tui-architecture.md` : `buildCommands`/`buildViews` sont dans `cmd/tui_commands.go:17` / `cmd/tui_views.go:96` ; signature réelle de `syncAsync` (`teamstate.TeamStateWriter`, `onDone func(error)`) ; 15 appels dans 12 vues ; vues `TeamMembers/Claims/Wiki/Events` supprimées ; `ShellAccess` complet (19 méthodes de plus) ; arborescence (`layout/`, `keys/`, `action_list.go`, `list_page.go`, `standalone.go`, `internal/glyph`) ; exemple de hints conforme à `keys` ; règle « pas de `[mot]` sans échappement ».

---

## Étape 5 — Fusions (~255 tests → ~50)

Paquet par paquet, mesure de couverture à chaque commit.

| # | Famille | Avant | Cible |
|---|---|---|---|
| 1 | `tracker/tracker_test.go` `TestGitLab_*` (13) + `TestJira_*` (4) | 17 | 1 table FetchIssue/AddLabels sur le `gitlabServer` existant (l. ~260) + 1 scénario découverte/pagination par tracker |
| 2 | même fichier : `TestMapTrackerStatus_*` (4) + `TestTruncateDescription_*` (4) | 8 | 2 tables |
| 3 | `config/team_resolve_test.go` | 22 | 3 tables : ResolveTeamConfig, ResolveTeamForProject, TeamStatePath |
| 4 | `config/config_test.go` `TestTeamConfig_Validate_*`, `TestValidateTeams_*` ; `resolution_test.go` `TestResolve{Bool,String}_*` | 17 | 2 tables |
| 5 | `cmd/helpers_test.go` `TestGenerateProjectID_*` (9) + `TestBuildMCPServersForProject_*` (9) | 18 | 2 tables, un seul `configtest.Home` |
| 6 | `cmd/init_steps_test.go` `*_Structure` / `*_FormRendering` / `*_SkipWhenFlagged` (5 étapes) + `TestExpressMode_*` | ~19 | 1 table sur les étapes ; scénarios `OnDone_*` gardés |
| 7 | `cmd/integration_test.go` (binaire : version, help, completion, mcp…) | 19 | 1 table smoke `{args, exit, contains}` |
| 8 | teamstate `*_PathTraversal` (`claims_test.go` 8, `safepath_test.go` 5, wiki/takeover 4) + `mcp/team/handlers_test.go` (4) | 21 | 1 table « toute API publique refuse `../` » |
| 9 | `teamstate/credentials_test.go` (`IsAuthError` ×6, `IsHTTPS` ×5) ; `wiki_test.go` `TestValidateProposal*` (8) | 21 | 3 tables |
| 10 | `teamstate/repo_test.go` (`Has*`, `IsCloned*`, `Pull/PushNotCloned`) ; `members_test.go` (14) | 25 | 1 scénario cycle de vie du dépôt, 1 scénario CRUD membres, 1 table d'erreurs |
| 11 | `views/views_test.go` (`MountUnmount`, `MountNo*`, + 7 de l'étape 3.a) | ~20 | 1 table MountUnmount sur toutes les vues + 2 scénarios « vide puis rempli » |
| 12 | `views/config_deepcopy_test.go` | 14 | 1 test : modifier chaque champ de la copie, l'original reste intact |
| 13 | `views/inline_wizard_test.go` : `ErrorState_*` (3), `HandleKey_CustomViewEsc*` (2), `EscQuit`/`BackKeys`, utilitaires d'export (6) | ~13 | 3 scénarios + 1 table |
| 14 | `shell` : `notifications_test.go` (16), `selection_test.go` (17), `command_test.go` `Search*` (8), `clipboard_test.go` (6), `tui_log_handler_test.go` (7) | 54 | ~10 (store, fichier, souris, recherche) |
| 15 | `widgets` (`inlineselect`, `stepbar`, `undo_stack`, `filterlist`) + `router_test.go` | ~39 | 1 test par composant |
| 16 | `workflow/unlocks_test.go` (10 à 1 assertion), `resolution_test.go` `TestResolve_*` à 1 assertion (~6), `patch_test.go` micro (4) | ~20 | 2 tables `{spec, wantCode}` |
| 17 | `mcp/gitlab/server_test.go` `ValidateGitLabURL`/`IsPrivateHost` (12) ; `mcp/team/handlers_test.go` (23) | 35 | 2 tables + 1 table de handlers |
| 18 | `beads/beads_test.go` (`EnsureInitFlags` 5, `RemoveBeadsSection` 4, `SanitizeGitignore` 5, `SanitizeHooks` 3) ; `runtime/container/engine_test.go` `TestDetect*` (19) | 36 | 4 tables + 1 table sur un runner simulé |
| 19 | `i18n_test.go` `TestT_*` (6), `selfupdate` `ValidateDownloadURL`/`VerifyChecksum` (7), `sqlite/project_store_test.go` erreurs (4) | 17 | 4 tables |

**Vues : passer en golden** les tests qui empilent des `Contains` sur le rendu : `TestCheckpointCard` (13 assertions), `TestCheckpointApproverCard` (16), `TestCheckpointCardVerify` (14), `TestCheckpointCardReports` (11), `TestSettingsExecSectionOrderAndTexts`, `TestTeamsViewTranslated`, `TestTeamEventPhrases`, partie rendu de `cleanup_views_test.go:*ViewKeys`, `list_page_test.go` (5 `*Detail`). Garder des assertions explicites pour les touches et callbacks.

**À garder absolument (ne pas fusionner au point de perdre leur lisibilité)** : `cmd/messages_literal_test.go` (`TestNoLiteralUserMessages`, `TestLiteralExceptionsAreUsed`, `TestLiteralGuardDetects` ; déplacer `TestProjectLoadErrorTranslated` ailleurs), `cmd/help_test.go`, i18n `parity_test.go` / `usage_test.go`, `archtest`, `docstest`, gardes de contenu livré de `bundle` (`shipped_*`, `globalguard`, `contract`, `capability`), sécurité (path traversal, `envfilter/nosecret_test.go`, `*HoldsNoCredential`, `credproxy`, `gateway`), `keys/convention_test.go`, gros scénarios `daemon` / `runsvc` / `checkpoint` / `fanout`.

---

## Étape 6 — Parallélisme et sleeps

### 6.a Remplacer les `time.Sleep` qui attendent « que rien ne se passe »
Risque fort (lenteur + instabilité) :
- `daemon/lifecycle_test.go:113,126,140,152,162,188` (100-700 ms, ~3 s) ; `daemon/budget_test.go:170,193,288,312` ; `daemon/proxyport_test.go:78,122` ; `daemon/hardening_test.go:52` ; `daemon/watch_test.go:180` ; `daemon/shell_alert_test.go:85` ; `daemon/fanout_driver_test.go:65`
- `services/checkpoint/locks_test.go:260` (1,1 s, granularité mtime) ; `services/checkpoint/service_test.go:272`
- `runsvc/service_e2e_test.go:174` (2 s) ; `teamstate/lock_test.go:136` ; `credproxy/proxy_test.go:371` ; `adapters/opencodev2/e2e_test.go:278,302`
→ horloge ou `Tick` injecté, ou `assert.Never` avec une fenêtre courte.

TUI :
- `views/sessions_decision_overlay_test.go:102,120,131`, `views/launch_form_existing_test.go:171,202,212`, `views/launch_form_test.go:137`, `shell/layout_test.go:95,112,134`, `keys_test.go:99,302`
→ un helper de test qui vide de façon synchrone la file `QueueUpdateDraw`.
- `shell/notifications_test.go:32,34,86` (1 ms pour ordonner des horodatages) → horloge injectée.

Acceptables (garder) : boucles de polling `runsvc/service_gateway_*` (passer en `require.Eventually`), délais simulés dans les faux (`fanout/driver_test.go:112,436`, `clean_test.go:166`, `evalrun/schedule_test.go`, `gateway/mcp_test.go:40`), `opencodev2/processes_test.go:50`.

### 6.b `t.Parallel()`
Prérequis : étape 2.c terminée (plus aucun `os.Setenv`, `HOME` via `t.Setenv`, `config.Reset` maîtrisé). Note : `t.Setenv` est incompatible avec `t.Parallel` dans le même test → les tests qui changent l'environnement restent séquentiels.
Candidats : `internal/tui/v2/views`, `internal/workflow`, `internal/teamstate`, `internal/config` (tests sans env), `internal/services/fanout` (fixtures indépendantes, 25 tests de 4 à 12 s).
→ ajouter par paquet, lancer `-race -count=3` pour détecter les interférences.

### 6.c `services/fanout` et `gitutil`
- fanout : après `t.Parallel`, regarder `Poll` / `IdleTimeout` de `newFixture` (20 ms / 400 ms) et les attentes du faux launcher.
- gitutil : compter les appels git de `TestSafeGitIgnoresRepoConfig` (14 s) et `TestCheckDetectsTamperedConfig` (9 s) ; regrouper les cas sur un même dépôt si possible.

---

## Résultats attendus (cumulés)

| | Avant | Après (estimation) |
|---|---|---|
| Fonctions de test | 3 019 | ~2 690 (−11 %) ; ~45 de plus à la fin de la fenêtre de migration |
| Lignes de test | ~103 k | ~98 k (−5 %) |
| Local `-short` | 4 min 37 | ~2 min après l'étape 1, moins après l'étape 6 |
| CI, job Go par OS | 2 passages complets, bundle ~17 min sous `-race` | 1 passage, bundle à quelques minutes |
| CI, job d'intégration | suite complète (~24 min) | 4 paquets (quelques minutes) |
| Bugs corrigés | — | étiquettes du board, écrasement d'équipe, reprise du fan-out |

Les comptes de tests et de lignes de l'étape 5 sont des estimations issues de l'analyse ; les durées sont mesurées.

---

## Suite (hors de ce plan, issue de l'audit)
Le détail à jour (état, chiffres du 21/10, efforts, décisions) est au § « É. État au 21/10/2026 au soir et reste à faire » en tête de ce fichier (É.4 : affichage B1-B7, architecture C1-C8, release D1-D8).
