# 17 — Points critiques des audits v5 (N1 → N27)

> Planifié le 09/10/2026 à partir de l'audit complet du 08/10 (revérifié sur `main` @ `c3b36496`, après fusion de T, L, C, U), complété le 09/10 par l'audit parallèle `docs/dev/audit-v5.md` (branche `feat/v5-fix-legacy-skills`, lot F0 inclus).
> Périmètre : **uniquement les points critiques** (🔴). Les points 🟠/🟡 des deux audits (design TUI global, CLI, docs, MCP, budgets, etc.) feront l'objet d'un fichier ultérieur (`19-…` ; `18` est la recette).
> Ordre global proposé : **F0 + Rel** → **lot 0 (sondes)** → pistes **S, G, W, D (drapeau), R, X** en parallèle → recette de non-régression → revue ergonomie/design → REL.
> **Statut au 09/10 au soir : tout le code de ce fichier est fusionné dans `main` (`890c01da`, non poussé). Prochaine étape : la recette `18-recette-critiques.md` (voir § Avancement › Points ouverts).**

## Principes
1. **Une garantie d'oh plutôt qu'une consigne** (comme `16` §Principes 2) : quand un comportement d'agent est en cause, la correction passe d'abord par oh (règles, passerelles, liaison aux décisions), le contenu n'est qu'un complément.
2. **Honnêteté du modèle de menace** : les règles shell (motifs glob) protègent contre les **erreurs** du modèle et les contournements bon marché ; elles ne sont **pas un bac à sable**. Tout agent qui peut éditer un fichier et lancer un outil de build/test/interpréteur peut exécuter n'importe quoi (`npm run`, `make`, `pytest`+`conftest.py`, `.venv/bin/python`, `go run`…). Le confinement réel vient : de la **liaison des checkpoints aux décisions observées par oh** (N1), du **conteneur** (N5), des **identifiants** (proxy, rôles en lecture seule pour l'infra), des **passerelles** (Beads). SECURITY.md doit le dire explicitement.
3. **Revérifier avant de corriger**, un test de non-régression par point (unitaire, contrat `integration`, e2e Haiku pour les comportements d'agent).
4. **D19** (rien d'opencode hors `internal/adapters/opencodev2`), schéma `oh/v1` : évolutions **additives** uniquement.
5. Les sondes du lot 0 conditionnent plusieurs choix (N2, N3) : **ne pas figer les motifs avant leurs résultats**.

## Apport de l'audit parallèle (`docs/dev/audit-v5.md`, branche `feat/v5-fix-legacy-skills`, 09/10)

Un second audit, fait sur un autre poste, est poussé sur `github/feat/v5-fix-legacy-skills` (base `c3b36496` = `main` actuel, 2 commits). Il apporte :
- **Lot F0 fait** (`19665610`) : registre de skills communautaires déconnecté (ADR-051). `oh skill add|list|remove|search|budget` supprimés, `internal/skillregistry` et `bricks/skill_budget.go` retirés, `~/.oh/skills` plus lu (paquets, validation, `oh skill check`), avertissement Doctor `legacySkillsCheck` (rien supprimé), docs fr/en, migration-v5 §10. **Vérifié ici** (extraction de la branche dans le dossier temporaire) : `go build ./...`, `go vet`, tests `bundle`, `bricks`, `workflow`, `hubcat`, `i18n` verts. Le seul échec de `cmd` (`TestGetCurrentBranch`) vient de l'extraction hors dépôt git, à recontrôler dans un vrai worktree. Restes mineurs : `docs/guides/skill-marketplace.*` garde tout l'ancien contenu sous un bandeau (à réduire à une page « déconnecté » ou archiver), commentaire périmé `cli/cmd/bundle.go:26` (« oh skill budget »). → **à fusionner en premier** (avant les pistes de ce fichier : il touche `bundle/skills.go`, `annexes.go`, i18n).
- **Le document d'audit** (`docs/dev/audit-v5.md`, 256 lignes) : méthode avec preuves (PoC réels sur `bd` 1.3.1, opencode 2.0.20), conformité des ADR 038–050, 42 constats sécurité (S-01→S-42), 16 fonctionnels (F-), 12 ergonomie (U-), 9 architecture (A-), 4 docs (D-), plan P0/P1/P2 et 3 décisions.

**Concordance avec ce plan** :

| audit-v5 | Ce plan | Remarque |
|---|---|---|
| S-26 (routes du socket, 🟡) | **N1** (🔴) | Ce plan va plus loin : `pass()` accepte un checkpoint en pause sans aucune approbation → le socket suffit à débloquer commit/close. Garder 🔴 |
| S-12 (promesses de SECURITY non tenues, lanceurs, `--no-veri`, `-qn`, `CORE.HOOKSPATH`, `git revert`/`pull`, `rm -rf .beads/hooks`) | **N3** + principe 2 | Ajouter à N3 : `git revert*`/`git pull*` créent des commits sans `*git*commit*` ; `rm -rf .beads/hooks` |
| S-19 (`ctx_*` contourne `bash: deny`), S-33 (conteneur), S-35 (fichier env), S-40 (migrations) | N3, N5, N5, N14 | S-33 ajoute : HOME **1777 partagé** par toutes les sessions du projet (`image.go:270-274`) |
| S-28 (environnement filtré par liste de refus) | audit 🟠 (non critique ici) | inchangé |
| F-01 (nettoyage Beads) | **N17** | Ajouter : `update-ref -d HEAD` sur un premier commit d'après des refs distantes possiblement périmées ; suppression des hooks bd de `.claude/settings.json` non suivis ; `.beads/` déplacé vers `.git/info/exclude` casse le partage par git |
| F-05, F-06, F-07, F-10 | N7, N8, N10, N6/N11 | + `quick`, `debug`, `libre`, `onboarding` écrivent sans verrou de commit (→ N11/DC26) |
| F-09 (agents orphelins aux permissions larges) | N4 + décision 3 de l'audit | → DC27 |
| U-08 (Ctrl+B, Ctrl+S sans Récap) | N19 | idem |
| S-01→S-11, S-13, S-14, S-16→S-25, S-31, S-32, S-34, S-41, F-02→F-04, santé du code | **nouveaux** | → N21–N27 ci-dessous ou « après REL » |
| N2 (mot de passe opencode dans le shell), N5 (git d'oh exécuté sur la machine depuis le conteneur en local), N12, N13, N15, N16, N18, N20, N19 (contrat de touches) | absents de l'audit-v5 | apports propres de ce plan |

**Revérifié ici sur `main`** : S-07 (`gateway/tickets.go:51-62` : `-sclosed` a pour nom `-sclosed`, le statut n'est pas lu ; `duplicate`/`supersede` ne sont pas des fermetures) et S-06 (`gateway/fakebd/run.go:46` : `GitHook` vient de la variable `BD_GIT_HOOK` de l'appelant ; `gateway/beads.go:90-93` remplace alors `beads.allow` par la liste des hooks) — **confirmés**.

## Avancement

| Étape | Statut |
|---|---|
| F0 (`feat/v5-fix-legacy-skills`) | ✅ revérifié (`go build`, `go vet`, `go test ./...` verts dans un vrai worktree) et fusionné (`1135dccf`) |
| Rel (N27) | ✅ fusionné (`c471270f`) : `release.yml` sur tag `v*` + tests, vérification Cosign (`selfupdate/signature.go`, `install.sh`), `install.sh` refuse sans somme de contrôle, `x/text` 0.42 et mises à jour Dependabot, golangci-lint 2.14 à 0 (macOS, Linux, Windows), contact sécurité. **Reste pour REL** : essai d'un tag `v5.0.0-rc.1` (nécessite un push), CHANGELOG 5.0.0 (à la fin). Écart : govulncheck signale encore des failles de la **bibliothèque standard** de Go 1.26.4 local (corrigées en 1.26.9) : la CI et la release utilisent `go-version: "1.26"` (dernier correctif) ; mettre à jour Go sur les postes |
| W0 (schéma `create`/`apply`) | ✅ fusionné (`ec8b7136`) : `UnlockCreate`, `UnlockApply`, `ApplyLockPatterns` (première liste, à affiner par S/N4), `unlock_unknown` (avertissement), `LockedOps` ignore une opération inconnue, docs du schéma fr/en |
| Worktrees | ✅ `../openhub-fix-{sec,wf,rob,keys,gw,remote}` sur `ec8b7136` |
| X (N19, N20) | ✅ fusionné (`c7354009`, `52e932b2` ; fusion dans `main`) ; écarts : `17-ecarts-X.md` |
| D (N21 : D1 drapeau, D2 S-01/S-02/S-04/S-05) | ✅ fusionné (`014fa347`, `c4967127` ; fusion `10624be2`, conflit CHANGELOG résolu) ; écarts : `17-ecarts-D.md` (clé du journal transportée par l'enveloppe ; risque résiduel `CI_JOB_TOKEN` → registre ; équivalents locaux de `gitutil.Safe` et de l'analyseur bd à réaligner après S et G ; S-03 attend N23 de S) |
| `main` après X et D | ✅ build, vet (tags), `go test ./...`, `-race -tags integration` (remote, TUI), golangci-lint 2.14 à 0 (macOS, Linux, Windows) |
| G (N22, N25, S-25) | ✅ fusionné (`138d09a1`, `77adcc35`, `6a2027ee`, `9ef8ec30`) ; `main` revérifié (build, vet, tests, `-race -tags integration` gateway/daemon/credproxy/remote/runsvc, lint 0 ×3 OS) ; écarts : `17-ecarts-G.md`. **À trancher** : Podman sans root + pasta (S-25, écoute sur l'adresse de la machine refusée) → réglage explicite ? **Suites** : avertissement Doctor si bd ≠ `bdargs.GeneratedFor` ; contrôle de la région à la saisie (TUI, `oh init`) ; réaligner le rejeu distant (D, `services/remote/replay.go`) et la garde `create` (W) sur `bdargs` |
| W (N24, N6–N11, lint capability) | ✅ fusionné (`e78de156`) ; conflits résolus : `gateway/beads.go` (garde `create` de W branchée sur l'analyseur `bdargs` de G : `createOp` lit `c.Key()`), CHANGELOG, golden ; test `TestWorkflowCreateGuard` adapté à la base Beads exigée par G (`76ea63be`) ; écarts : `17-ecarts-W.md` |
| S (lot 0, N1–N5, N23, N26) | ✅ fusionné (`181f6808`) ; conflits résolus : `bundle/build.go` (ordre own → InfraGuard → GitShellGuard → BeadsShellGuard → HookGuard → BeadsDirGuard ; garde global par le plugin), `checkpoints.go` (motifs ancrés de W + `rebase`, `stash pop` de S ; `ApplyLockPatterns` de S), `locks.go`, `server_store.go`, SECURITY fr/en (fusion des puces hooks/commit/garde global), golden ; tests de W adaptés à N1 (`reachValidated`, approbation observée) et à N23 (`TestE2EQuickCommitGate` : paquet modifié après build enregistré sous un nom de test) ; vérifié : build, vet, `go test ./...`, `-race -tags integration` (bundle, services, daemon, gateway, workflow, runtime, gitutil, adapters, runsvc), lint 0 ×3, e2e Haiku (CheckpointSocketBypassRefused, TicketCommitGate, SubagentPassesItsCheckpoint, ShellEnvHidesServerSecrets, QuickCommitGate, HookBypassRefused, CheckpointGating, LockPatternsWithoutFalsePositives, LibreQuestionIsNotClosed, GitOutputAndUploadPack, ShellMatcherSemantics, WorkflowCheckpointAsked) verts ; écarts : `17-ecarts-S.md` |
| R (IsMerged, N12–N18, rotation des logs, N15-A/B, v43) | ✅ fusionné (`23e64493`) ; conflits résolus : `bundle/build.go` (vérification de S + `touchBundle` de R à la réutilisation), `storage/sqlite/store.go` (version de R : `OpenWith`, `OpenRaw`, garde de schéma), `worktree.go`, `beads/clean.go` (le `git fetch` ajouté par R passe par `gitutil.Command` de S), CHANGELOG ; vérifié : build, vet, `go test ./...`, `-race -tags integration` (storage, teamstate, gc, bundle, beads, worktree, daemon, runsvc, adapters, services, tracker, cmd), lint 0 ×3, e2e Haiku (socket, ticket, quick, gating, hooks) ; écarts et choix à confirmer : `17-ecarts-R.md` |
| **Toutes les pistes** | ✅ fusionnées dans `main` (`23e64493`, non poussé) |
| Finitions (`fin-tui`, `fin-misc`, `fin-remote`) | ✅ fusionnées (`db36f88a`, `5366a401`, `890c01da`) ; écarts : `17-ecarts-fin-{tui,misc,remote}.md`. **fin-tui** : Échap garde son sens dans la fiche et les assistants (toasts qui disparaissent seuls ; vaut aussi pour l'éditeur et la publication) ; bouton « Configurer le fournisseur » (vue Fournisseur, retour à la fiche, nouvelle vérification ; aussi sur refus 422) ; région Bedrock vérifiée à la saisie (`internal/provider/region.go`, TUI, `oh init`, `oh provider setup`, `oh config set`) ; une fenêtre de formulaire refusée garde la saisie. **fin-misc** : Doctor « version de bd » (avertissement, commande de régénération de la table) ; alerte ✗ « configuration git modifiée » (fichier nommé, une alerte par état) ; annulation du commit de `bd init` après confirmation interactive (CLI **et** fenêtre « Appliquer » de la TUI). **fin-remote** : rejeu du journal par `bdargs` (plus strict : alias `done`, `duplicate`, `supersede`, statuts de catégorie done → `close`), git du runner par `gitutil.IsolatedArgs/IsolatedEnv`, chemin « sans hooks » = `/dev/null` (correctif de `gitutil/safe.go`) ; clé du journal chiffrée pour le job (X25519 + HKDF + AES-GCM, clé privée `OH_JOURNAL_KEY` masquée/protégée/limitée à `oh-run`, clé publique `oh-journal-key.pub` dans le dépôt `oh-runner`) ; image du job épinglée par empreinte (dotenv après build, empreinte enregistrée par la machine sinon, `image_replaced`). Ruptures : relancer `oh remote setup` (enveloppe 3, pipeline 4) ; une reconstruction d'image par machine |
| `main` final | ✅ `890c01da` (non poussé) : build, vet (tags), `go test ./...`, `-race -tags integration` (remote, gitutil, gateway, daemon, runsvc, bundle, services, beads, tui, cmd), golangci-lint 2.14 à 0 ×3 OS, e2e Haiku verts (socket, ticket, quick, secrets du shell, sous-agent ; `TestE2EHookBypassRefused` : 1 échec sur 4 exécutions, dépend du modèle, à surveiller) ; worktrees et branches de pistes supprimés ; aucun `opencode serve` orphelin |
| **Points ouverts pour la prochaine session** | (1) **Bug** : vue Fournisseur de la TUI — modifier `aws_region`, `auth_mode` ou `aws_profile` ne change que la mémoire, rien n'est écrit dans `hub.toml` sans une autre action qui sauvegarde (relevé par fin-tui) ; (2) **À confirmer** : annulation du commit de `bd init` depuis la fenêtre « Appliquer » de la TUI (fin-misc l'a autorisée ; R l'interdisait) ; (3) la garde de fermeture des tickets (G) ne remonte pas encore l'alerte « configuration git modifiée » ; (4) distant : l'épinglage par empreinte après build suppose que GitLab Runner développe une variable dotenv dans `image:` (non garanti par la doc) → **à valider sur runner réel**, repli possible : deux pipelines ; (5) option : chiffrer toute l'enveloppe distante avec la même paire de clés ; (6) `[gc] log_*` non transmis au démarrage des serveurs ; (7) recette (`18-recette-critiques.md`), CHANGELOG 5.0.0, push de `main`, tag `v5.0.0-rc.1`, puis le fichier `19-…` des points non critiques |
| Décisions du 09/10 (après les pistes) | Podman sans root + pasta (S-25) : **refus gardé**, message traduit avec les contournements + ligne de dépannage `container.*` (`d1e1f979`), à vérifier pendant la recette Linux ; verrou du team-state pendant la synchronisation du suivi : **gardé** (borne 3 min = délais maximum, durée attendue de quelques secondes), **à mesurer en recette** ; GC du démon : simulation jusqu'au premier `oh gc` ; PgUp/PgDn de la fiche : gardés ; annulation du commit de `bd init` : la confirmation interactive suffit (branche `fin-misc`) ; toasts ignorés par Échap dans la fiche et les assistants (`fin-tui`) ; clé du journal distant et images épinglées par empreinte : **renforcés avant REL** (`fin-remote`) ; suites retenues : réalignement D sur G/S (`fin-remote`), Doctor version de bd, alerte « configuration git modifiée » (`fin-misc`), région à la saisie, bouton « Configurer le fournisseur » (`fin-tui`) |

## Inventaire des points critiques

| Id | Point | Piste | Bloque REL |
|---|---|---|---|
| N1 | Un agent peut passer lui-même un checkpoint (socket du démon, `pass()` sans approbation) | S | oui |
| N2 | Mot de passe du serveur opencode éventuellement visible du shell de l'agent (sonde U3) | S | oui (si U3 positif) |
| N3 | Contournements des permissions shell (couche de refus globale, garde git, `.git` en édition, `ctx_*`) | S | oui |
| N4 | `debugger` shell libre ; `infra` mutations cloud libres ; `benchmarker` `env *` | S | oui |
| N5 | Conteneur → machine via `.git/config`, hooks, fichier `.git` ; git d'oh non protégé sur la machine ; `runArgs` non durcis | S | oui |
| N6 | Protocole des checkpoints contradictoire dans les skills (cp-0/cp-2 par `question`, « terminer la session », en-têtes « CP-n » fermés par oh) | W | oui |
| N7 | `sweep` bloqué, sans review ni commit, parallèle dans la même arborescence | W | oui |
| N8 | `review-feedback` incohérent (commit avant cp-2, pas de push, réponses GitLab impossibles) | W | oui |
| N9 | `feature` : review de fin impossible, « Commit tous » impossible, création de tickets non contrôlée | W | oui |
| N10 | `unlocks: push` sans effet + gabarit qui pousse à `git push` + faux positifs des motifs de verrou | W | oui |
| N11 | Consignes que les permissions des agents refusent (20 cas) + absence de contrôle automatique | W | oui (corrections) / non (lint : 5.0.x) |
| N12 | `oh run` hors projet prend le seul projet ; aucune vérification du fournisseur avant lancement | R | oui |
| N13 | `doctor`/`repair` inutilisables quand config/DB cassée ; processus annexes (MCP, démon) font tout le démarrage ; bugs de `repair` | R | oui |
| N14 | Migrations SQLite : course entre processus, pas de sauvegarde, pas de garde de version | R | oui |
| N15 | team-state : aucun verrou entre (et dans les) processus, `reset --hard`, `git add .` | R | oui (étape A) / non (étape B) |
| N16 | Aucun nettoyage (paquets, données serveurs, logs, sessions, worktrees) ; `auto_cleanup` jamais lu | R | non (5.0.1), mais rotation des logs oui |
| N17 | Nettoyage Beads automatique qui peut annuler un commit sans confirmation | R | oui |
| N18 | Processus `opencode serve` orphelins (tests interrompus, découverte des natifs) | R | non |
| N19 | Raccourcis dangereux de la TUI (Entrée/lettres volées, `d`, Ctrl+S sans Récap, Ctrl+B, assistants sans sortie, worktrees sans confirmation) | X | oui |
| N20 | Petits terminaux : accueil sans liste en 80×24 | X | oui |
| N21 | Mode distant : git final en root avec hooks/fsmonitor de l'agent (S-01, prouvé), build d'image avec les secrets de tous les projets (S-02), paquet retéléchargé non vérifié (S-03), rejeu Beads trop permissif (S-04, prouvé), journal affiché brut (S-05) | D | oui (correction **ou** distant expérimental désactivé, DC22) |
| N22 | Passerelle Beads : mode hook piloté par l'appelant (S-06, prouvé), fermeture contournée (`-sclosed`, `duplicate`, `supersede`, S-07, prouvé), options collées (`-qC/…`, `--DB=`, `-o/…`, `-f/…`, S-08, prouvé), garde « fermé seulement si commité » trop faible (S-09), environnement complet pour `bd` (S-14) | G | oui |
| N23 | Intégrité du paquet de session : empreinte jamais recalculée au chargement (S-03 local), dossiers en 0755 et `external_directory` sur `SkillsDir/*` (S-10) | S | oui |
| N24 | Superposition « ne peut que durcir » contournable : parent sans `beads:` (S-16, prouvé), checkpoint `unlocks:` passable en `skip` (S-17, prouvé), `mcp`/`plugins` remplacés librement (S-18), `risk: read` avec MCP en écriture et `ctx_*` (S-19), `after:`/`calls`/`circuit_breaker`/`skills.extra` (S-20) | W | oui (S-16→S-19) |
| N25 | Proxy d'identifiants : région Bedrock non validée → clé envoyée à un hôte arbitraire (S-21, prouvé), liste des modèles contournable chez OpenRouter (S-22), `allowed_models` illisible = tout autorisé (S-23), second écouteur sur toute IP privée (S-25) | G | oui (S-21→S-23) |
| N26 | Mots de passe des serveurs opencode **en clair dans `oh.db`** : un agent (lecture du fichier via un interpréteur) peut piloter les serveurs d'autres projets (S-24) | S (avec N2) | oui |
| N27 | Hygiène de release : `golang.org/x/text` vulnérable GO-2026-5970 (atteint), golangci-lint de la CI plus ancien (11 constats avec 2.14), `release.yml` jamais déclenché sur un tag (S-41), CHANGELOG à découper en 5.0.0 (D-01), contact sécurité `security@example.com` (S-42), Cosign non vérifié alors que SECURITY dit « indicative » (S-39 doc) | Rel | oui |

## Décisions à valider (avant de lancer les pistes)

> **Statut (09/10/2026)** : DC1–DC27 **validées** par l'utilisateur. Toutes selon la recommandation, sauf **DC21** : tout ce fichier bloque la sortie de la v5 (aucun report en 5.0.1). Conséquence : la colonne « Bloque REL » de l'inventaire vaut « oui » pour N1–N27 ; les mentions « 5.0.1 » / « 5.0.x » des sections N11 (lint), N15 (étape B), N16 (GC), N18, N21 (lot D2) et N25 (S-25) se lisent « avant REL ».

| # | Décision | Recommandation | Alternative |
|---|---|---|---|
| DC1 | Comment empêcher le passage d'un checkpoint par un autre chemin que la décision de l'utilisateur | **Lier `pass()` à une approbation observée par oh** (décision oh, ou réponse positive décodée de l'événement de l'outil), + authentifier `/v1/workflow/*` par le jeton du groupe, + refus shell du socket | Secret partagé avec le serveur MCP seul (insuffisant : même utilisateur, l'agent peut lancer `oh mcp serve workflow` lui-même) |
| DC2 | Couche de refus globale (N3) avec faux positifs assumés (`git log -- fsmonitor.go`, `ls ~/.ssh`, messages de commit contenant ` -xn`…) | **Oui**, en Go, ajoutée en dernier pour tous les agents | Durcir seulement les listes d'autorisation |
| DC3 | `debugger` : passer d'un shell libre à une liste (lecture, diagnostic, `bd` lecture/création), lanceurs de tests en `ask` | **Oui** | Garder `bash: allow` et ne le proposer qu'en conteneur |
| DC4 | `infra` : lecture seule autorisée, mutations **refusées** sauf après un checkpoint `unlocks: [apply]` (nouvelle opération, additive), secrets en `ask` | **Oui** + recommander des identifiants en lecture seule | Mutations en `ask` seulement (pas de lien au workflow) |
| DC5 | Conteneur : `.git/config`, `hooks/`, fichier `.git`, `commondir` en lecture seule dans le conteneur → `git switch -c --track` / `push -u` impossibles dans le conteneur | **Oui**, avec `branch.autoSetupMerge=false` dans `GIT_CONFIG_*` | Ne protéger que côté machine (enveloppe git) |
| DC6 | Skills : supprimer les branches « hors workflow » des checkpoints (un seul protocole `workflow_checkpoint`) ; `libre` garde `question` pour toute confirmation, sans jamais nommer un id de checkpoint | **Oui** (option A de l'analyse W1) | Garder les deux branches derrière un repère corrigé |
| DC7 | `push` retiré de tous les `unlocks` livrés ; le push reste à l'utilisateur ; un workflow `publish` **exécuté par oh** (pas par un agent) est reporté en 5.1 | **Oui** | Agent `publisher` avec droit de push |
| DC8 | Nouvelle opération verrouillable `create` (tickets) : `cadrage` cp-tickets `unlocks: [create]`, `feature` reçoit un `cp-tickets` avant cp-0 | **Oui** | `bd create: ask` pour le planner |
| DC9 | Opération `unlocks` inconnue d'un oh plus ancien : **avertissement** (et non erreur) dans le validateur, pour ne pas bloquer une équipe aux versions mélangées | **Oui** | Version minimale d'oh exigée par l'équipe |
| DC10 | `feature` : un cp-2 **par ticket** (« Commit tous » abandonné) ; fenêtre par ticket reportée | **Oui** | Fenêtre de déverrouillage liée à une liste de tickets |
| DC11 | `sweep` : découpage par `pathfinder` (lecture seule, non verrouillé), `cp-plan` obligatoire en pause dans tous les modes, `cp-commit` `unlocks: [commit]`, `reviewer`, `dry_run` = ne jamais appeler `cp-plan`, périmètres disjoints sinon séquentiel | **Oui** | Autoriser le conductor à lister les fichiers |
| DC12 | `review-feedback` : cp-fix → correction sans commit → nouvelle review → cp-2 (`unlocks: [commit]`) → push et réponses laissés à l'utilisateur (récap) | **Oui** | Publication par oh (avec DC7, en 5.1) |
| DC13 | Projet implicite (`oh run` hors projet) : commandes qui écrivent → confirmation (refus sans terminal, message « utilisez -p ») ; commandes en lecture → avertissement sur stderr | **Oui** | Toujours exiger `-p` hors projet |
| DC14 | Niveaux de démarrage `none / light / degraded / full` par annotation de commande | **Oui** | Liste d'exceptions dans `PersistentPreRunE` |
| DC15 | Nettoyage : seules les sessions `stopped` sont terminées (les `completed`/`failed` restent reprenables) ; rétention 30 j (données), 180 j (historique), 400 j (usage) ; **premier passage du démon en simulation** ; migration **v43** (`sessions.worktree_auto`, `worktree_branch`) | **Oui** | Nettoyage manuel uniquement (`oh gc`) |
| DC16 | Nettoyage Beads (`autoCleanBeads`) : afficher les changements et demander ; sans terminal → rapport seul ; jamais d'annulation de commit sans `--yes` | **Oui** | Supprimer l'annulation de commit |
| DC17 | TUI : la palette ne s'ouvre plus sur une lettre quelconque, seulement `Ctrl+P` et `/` ; Entrée n'ouvre plus la palette quand le widget focalisé l'accepte ; fermeture des toasts par Échap (plus `d`) | **Oui** | Garder la saisie « n'importe où » hors listes |
| DC18 | Fiche de lancement : `Ctrl+S` va au Récap, ne lance que depuis le Récap chargé ; retour = `Alt+←`/`PgUp` en plus de `Ctrl+B` | **Oui** | `Ctrl+S` lance avec un Récap affiché en fenêtre |
| DC19 | Assistants : Échap ouvre « Quitter l'assistant ? [Quitter] [Passer l'étape] (si facultative) [Continuer] » ; suppression du double Échap silencieux | **Oui** | — |
| DC20 | Petits terminaux : mode compact sous 32 lignes (bandeau sur une ligne, palette sur 3 lignes, sans pied de page), message « terminal trop petit » sous 60×16 | **Oui** | — |
| DC21 | **Validé le 09/10 : tout ce fichier bloque REL** (N1–N27, y compris N16 GC complet, N18, l'étape B de N15, le lint de N11, S-25 de N25 et le lot D2 du distant). Le mode distant reste **expérimental** à la sortie (DC22) tant que sa recette réelle sur runner n'est pas faite, même si D2 est corrigé | ~~Répartition 5.0 / 5.0.1~~ (non retenue) | — |
| DC22 | Mode distant (décision 1 de l'audit-v5) : jamais validé sur un vrai runner + 3 failles graves | **Pour 5.0 : distant marqué expérimental, désactivé par défaut** (`[remote] experimental = true` requis, avertissement à `oh remote setup` et `--runtime remote`, retiré des choix par défaut de la TUI) + correction immédiate de S-03 (commune au local, N23) ; lot D complet (S-01, S-02, S-04, S-05) en 5.0.x avant de lever le drapeau, avec la recette réelle de `09` | Corriger tout le lot D avant 5.0 (retarde la sortie, sans recette possible faute de runner) |
| DC23 | Positionnement de la sécurité locale (décision 2 de l'audit-v5) | **Les deux** : SECURITY réécrit en « garde-fous, pas barrières » (principe 2) **et** verrous qui ne dépendent pas des motifs là où c'est possible (N1 liaison aux approbations, N22 passerelle, N5 enveloppe git) ; conteneur conseillé pour les workflows qui écrivent, `strict_isolation` par défaut en local évalué en 5.1 (S-11) | Seulement réécrire la doc |
| DC24 | Mode hook de la passerelle Beads (S-06) | **Jeton à usage unique émis par le démon** et posé par le hook installé par oh (le faux `bd` ne fait plus confiance à `BD_GIT_HOOK`) ; à défaut : en mode hook, seulement les sous-commandes exactes `hooks run <nom>` sans argument supplémentaire | Refuser les hooks bd (casse A19) |
| DC25 | Mots de passe des serveurs (S-24) | **Hors de `oh.db`** : trousseau (comme les secrets), ou fichier 0600 par groupe sous `~/.oh/run/` couvert par le refus shell `*.oh/run*` ; `oh.db` ne garde qu'une empreinte | Chiffrer dans `oh.db` (clé où ?) |
| DC26 | `quick`, `debug`, `libre`, `onboarding` écrivent sans verrou de commit (F-05) | **`quick`/`debug` : checkpoint `cp-commit` obligatoire `unlocks: [commit]`** (pause dans tous les modes) ; `libre` : inchangé (sans checkpoint par nature, consigne « demander avant toute action irréversible ») ; `onboarding` : écrit `docs/wiki/` seulement → édition limitée par chemin, pas de commit | Laisser libre |
| DC27 | Agents orphelins `benchmarker`, `test-generator`, `database`, `infra` (décision 3 de l'audit-v5, F-09) | **Garder, permissions réduites (N4)**, accessibles par `libre` ; workflows dédiés (`perf`, `test-gen`, `db-migration`, `infra-plan`) en 5.1 | Les retirer du hub |

## Lot 0 — Sondes opencode 2.0.20 (avant S2/S3, piste S, une demi-journée)

Test e2e `TestE2EShellMatcherSemantics` (Haiku, sur le modèle de `opencodev2/hooks_e2e_test.go`), résultats notés dans `02-findings-opencode-v2.md` :

| Sonde | Question | Impact |
|---|---|---|
| U1 | Le préfixe d'environnement (`X=y git diff`) fait-il partie du texte comparé ? (`16` §185 et §206 se contredisent) | Forme des motifs d'autorisation et de refus |
| U2 | Espaces et guillemets normalisés ? (`git  checkout`, `git pu''sh`) | Si non : les motifs sont contournables par simple guillemet → l'écrire dans SECURITY |
| U3 | **Le shell de l'agent voit-il `OPENCODE_SERVER_PASSWORD` / `OPENCODE_CONFIG_CONTENT` ?** (`printenv`) | Si oui : l'agent peut répondre à ses propres permissions et remplacer ses règles via l'API → **N2 prioritaire absolu** |
| U4 | `external_directory` vérifié pour les arguments et redirections du shell ? (`echo x > ~/.oh_probe`) | Écritures hors du projet |
| U5 | Une ligne est-elle refusée si une seule de ses commandes l'est ? Les règles de session passent-elles après celles de l'agent ? | Combinaison `InfraGuard` (ask) + verrou `apply` (deny) |

Ajouter un **simulateur du rapprochement d'opencode** (découpage sur `&& || ; | ( ) $( )`, préfixes) dans `internal/bundle` pour les tests unitaires : l'évaluateur actuel (`bundle.evaluate`, `permissions.go:136-145`) compare la ligne entière et ne reflète pas opencode.

---

## Piste S — Sécurité

### N1 — Passage d'un checkpoint sans l'utilisateur (DC1)
**Constat (revérifié)** :
- `POST /v1/workflow/checkpoint|outputs|rules` sans `privileged` (`daemon/handlers.go:34-36`) ; protégé seulement par l'UID du pair (même utilisateur que l'agent).
- `checkpoint.Service.Reached` → `pass()` (`services/checkpoint/state.go:164-200`) accepte tout appel : sans `Approved[id]`, `ap.By = "mode"` (ou `tool` si `Waiting == id`), le checkpoint passe et `Unlocks` (commit, push, close) s'ouvre. `developer-rw` autorise `curl*` → `curl --unix-socket ~/.oh/run/ohd.sock -XPOST http://x/v1/workflow/checkpoint -d '{"tool_session":"ses_…","id":"cp-2"}'`.
- Un secret pour le seul serveur MCP ne suffit pas : MCP et shell sont frères sous le même `opencode serve`, même UID ; l'agent peut lancer `oh mcp serve workflow` et lui envoyer un JSON-RPC forgé.
- L'approbation dans l'interface de l'outil n'est aujourd'hui **pas observée** : `permission.replied` est mappé sans contenu (`opencodev2/events.go:165-166`) ; `Settled` (`state.go`) ne fait que vider `Waiting`.

**Correction** :
1. **Liaison à l'approbation** (cœur) : pour un checkpoint dont le comportement est `pause` dans le mode de la session, `pass()` exige soit `Approved[id]` (décision prise dans oh : `state.go:330`), soit une **approbation outil observée** : nouveau champ `CheckpointState.ToolApproved[id]` posé par le démon quand l'événement de réponse de l'outil porte une réponse positive pour la demande `ToolRef` du checkpoint en attente. Le repli `By = "mode"` ne vaut que pour `auto`/`conditional` passé automatiquement. Refus → erreur `ErrNotApproved` renvoyée à l'outil MCP (message clair à l'agent : « attendez la validation »).
   - Adaptateur (D19) : décoder `permission.replied` (identifiant de la demande + réponse `once|always|reject`) dans une forme neutre `adapters.DecisionReply{ID, Approved bool}` portée par `EventDecisionReplied` ; **vérifier la forme du payload en 2.0.20** (sonde, `02`).
   - Repli si l'outil ne fournit pas la réponse : le démon interroge `GET /api/session/{id}/permission` avant/après (la demande disparue + appel MCP reçu dans la seconde qui suit avec le même `ToolRef` ⇒ approuvée) ; à documenter comme moins fort.
2. **Authentification des routes** `/v1/workflow/{status,checkpoint,outputs}` par le **jeton du groupe** (`ohs_…`, déjà vérifié pour `handleHookPermission`, `daemon/workflow.go:195`) : le serveur MCP `workflow` le lit dans son environnement (hérité du serveur, que le shell de session n'hérite pas en 2.0.20 : constat de 4.C — à reconfirmer par U3) et l'envoie en en-tête ; la session racine doit appartenir au groupe du jeton. `/v1/workflow/rules` → `privileged` (seul oh l'appelle).
3. **Routes voisines** : `/groups/{g}/policy` (`stop_now`) → `privileged` ; `/clients/heartbeat` : TTL borné (≤ 300 s) ; `GET /v1/clients` (U1) → `privileged` (expose TTY/tmux).
4. **Défense en profondeur** (dans N3) : refus shell `*ohd.sock*`, `*--unix-socket*`, `*.oh/run*`, `*oh* mcp serve*`.
5. SECURITY.md : retirer « le shell peut encore joindre le socket du démon » des limites acceptées pour les checkpoints, décrire la liaison à l'approbation.

**Fichiers** : `internal/services/checkpoint/state.go`, `service.go`, `domain/checkpoint*.go` (champ `ToolApproved`), `internal/daemon/workflow.go`, `decisions.go`, `handlers.go`, `lifecycle.go`, `internal/mcp/workflow/server.go`, `internal/daemon/client.go` (en-tête jeton), `internal/adapters/adapter.go` (forme neutre), `opencodev2/events.go`, `SECURITY*.md`.
**Tests** : unitaires `pass()` (pause sans approbation → refus ; approbation oh → passe ; approbation outil observée → passe ; refus outil → refus ; auto → passe) ; démon : route sans jeton → 401, jeton d'un autre groupe → 403 ; e2e Haiku `TestE2ECheckpointSocketBypassRefused` (l'agent fait le `curl` → cp-2 non passé, `git commit` toujours refusé) ; e2e d'approbation dans l'interface de l'outil (réponse API `once`) → passe.

### N2 — Mot de passe du serveur opencode dans le shell (sonde U3)
- Si U3 est positif : `unset OPENCODE_SERVER_PASSWORD OPENCODE_CONFIG_CONTENT` (et toute variable `OPENCODE_*` sensible) dans le démarrage du shell d'oh (`gateway.ShellStartup`, `~/.oh/run/shell/zsh/.zshenv` et équivalent bash), vérifié par session comme le faux `bd` (T2), contrôle Doctor ; refus shell `*OPENCODE_SERVER_PASSWORD*`, `*/api/session/*/permission*`, `*/api/permission*`.
- En conteneur : même traitement dans l'image (couche oh).
- **Tests** : e2e `printenv OPENCODE_SERVER_PASSWORD` vide ; `curl` vers l'API du serveur refusé.
- **Lien avec N26** : même si l'environnement est nettoyé, les mots de passe de **tous** les serveurs sont lisibles en clair dans `oh.db` (`storage/sqlite/server_store.go:22,51`) par n'importe quel interpréteur autorisé (`.venv/bin/python -c "import sqlite3…"`). N2 et N26 se traitent ensemble.

### N26 — Mots de passe des serveurs hors de `oh.db` (DC25)
- **Correction** : le mot de passe d'un groupe est rangé dans le trousseau (clé `oh/server/<groupe>`) ou dans `~/.oh/run/servers/<groupe>.pw` (0600, couvert par le refus shell `*.oh/run*` de N3) ; `oh.db` ne garde qu'une empreinte (pour vérifier un serveur adopté) ; migration au démarrage du démon (relire, déplacer, effacer la colonne) — **pas de nouvelle migration SQL** si la colonne est vidée par le code (sinon prendre v44 et l'inscrire dans `00`) ; `AttachServer`/reprise lisent la nouvelle source ; `oh export` n'inclut plus de mot de passe.
- **Tests** : après démarrage d'un serveur, `oh.db` ne contient aucun mot de passe ; reprise d'un serveur en veille OK ; ancien `oh.db` migré ; `oh export` sans secret.

### N23 — Intégrité du paquet de session
- **Constat** : `bundle.Load` ne recalcule pas l'empreinte (`bundle/build.go:296-307`), `Build` réutilise un dossier existant sans le vérifier (`:276-278`), dossiers en 0755, et `external_directory` est autorisé sur `SkillsDir/*` (`opencodev2/render.go:174-176`) → un agent qui écrit dans le paquet modifie **toutes** les sessions de même empreinte ; un paquet retéléchargé (distant, S-03) n'est jamais comparé.
- **Correction** : manifeste d'empreintes (`bundle.json` + fichiers) vérifié à `Load` et à la réutilisation (coût : un hachage par démarrage de groupe, pas par session) ; dossiers et fichiers en lecture seule (0555/0444) après construction ; `external_directory` limité à la **lecture** sur `SkillsDir/*` (règle d'édition refusée) ; refus shell `*.oh/bundles*` (N3) ; écart détecté → reconstruction (local) ou refus (paquet distant) avec message.
- **Tests** : paquet modifié → détecté et reconstruit ; paquet distant modifié → refusé ; édition d'un fichier du paquet par l'agent refusée (e2e).

### N3 — Couche de refus globale et garde-fous (DC2)
**Constat** : ordre des règles `own → GitShellGuard → BeadsShellGuard → HookGuard` (`bundle/build.go:338`), dernier qui correspond gagne ; correspondance glob sensible à la casse, sur toute la valeur (`permissions.go:159-185`) ; **dans une même carte de frontmatter, l'ordre est par longueur littérale** (`sortPatterns`, `permissions.go:97-105`) : un refus court perd contre une autorisation longue → **les refus importants doivent être en Go**, pas dans le frontmatter. `gitguard.go:50` ne couvre que les préfixes `git ` et `git -C * `.

**Correction** :
1. Nouveau `bundle/globalguard.go` : `GlobalShellGuard()`, `GlobalEditGuard()`, ajoutés **en dernier** à `build.go:338` (couvre aussi les sessions sans workflow, contrairement aux règles de session). Contenu (à ajuster après U1/U2) :
   - plan de contrôle : `*ohd.sock*`, `*--unix-socket*`, `*.oh/run*`, `*.oh/servers*`, `*.oh/bundles*`, `*oh* mcp serve*`, `*OPENCODE_SERVER_PASSWORD*`, `*OPENCODE_CONFIG_CONTENT*`, `*/api/session/*/permission*`, `*/api/permission*` ;
   - git (environnement, transports, écritures) : `*GIT_DIR*`, `*GIT_WORK_TREE*`, `*GIT_EXTERNAL_DIFF*`, `*GIT_SSH*`, `*GIT_EDITOR*`, `*GIT_SEQUENCE_EDITOR*`, `*GIT_ASKPASS*`, `*GIT_EXEC_PATH*`, `*GIT_PAGER*`, `*GIT_INDEX_FILE*`, `*--upload-pack*`, `*--receive-pack*`, `*git*push*--exec*`, `*git*--output*`, `*fsmonitor*`, `*git*--ext-diff*` ;
   - crochets (complète `HookGuard`) : `*git*--no-ve*`, `*git*commit* -?n*`, `*git*commit* -??n*`, `*git*commit* -???n*` ; écriture shell dans `*.husky/*`, `*.githooks/*` ;
   - garde git avec `*` en tête (même liste que `gitGuardDeny`, préfixes `*git* ` et `*git*-C * `, exceptions `gitGuardKeep` réémises après) ;
   - commandes qui en lancent d'autres : `find * -exec*`, `-execdir*`, `-ok *`, `-okdir*`, `-delete*`, `-fprint*`, `-fls*` ;
   - persistance et identifiants hors du projet : `*.zshenv*`, `*.zshrc*`, `*.zprofile*`, `*.bashrc*`, `*.bash_profile*`, `*.profile*`, `*.ssh/*`, `*.gitconfig*`, `*.config/git/*`, `*LaunchAgents*`, `*crontab*`, `*.aws/credentials*`, `*.kube/config*`, `*.docker/config.json*`, `*.netrc*`, `*.npmrc*`, `*.pypirc*` ;
   - installations système quel que soit l'ordre des options : `*npm* -g*`, `*npm*--global*`, `*npm*--location=global*`, `*pnpm* -g*`, `*pnpm*--global*`, `*yarn* global *`, `*bun* -g*`, `*uv*--system*`, `*--break-system-packages*` ;
   - évasion docker : `*docker*--privileged*`, `*docker*--pid*host*`, `*docker*--network*host*`, `*docker*--cap-add*`, `*docker*--security-opt*`, `*docker*--device*`, `*docker*.sock*`, `*docker* -v /*`, `*docker*--volume*/*`, `*docker*--mount*`.
   - Édition : refus `.git`, `*/.git` (fichier pointeur d'un worktree) ; `ask` sur `.husky/*`, `*/.husky/*`, `.githooks/*`, `lefthook*.yml`, `.pre-commit-config.yaml`.
2. `developer-rw.yaml` : retirer `printenv*`, `pkill*`, `kill *` (ou `kill %*` seulement), `docker run*`/`docker exec*` → `ask` ; `find*` → `find * -name*`, `-type*`, `-path*`, `-maxdepth*`… ; ajouter `rg*` / `grep -r*` (remplace `find -exec grep`). `reviewer` : `git fetch*` → `git fetch`, `git fetch origin*`.
3. **`ctx_*` suit `bash`** : `ctx_execute`, `ctx_execute_file`, `ctx_batch_execute` reçoivent l'effet de `bash: "*"` de l'agent (refus si `bash: deny`) — dans `ConvertPermissions` ou `bricks.DropPluginPermissions` ; latent aujourd'hui (aucun workflow ne charge context-mode), à fermer avant toute activation. Le contrôle du risque `read` (`workflow/hubcat/hubcat.go:117-120`) compte aussi ces outils comme du shell.
4. SECURITY.md : section « Ce que les règles shell ne protègent pas » (interpréteurs, scripts du projet, guillemets si U2 négatif).
5. Compléments de l'audit-v5 (S-12) : `git revert*` et `git pull*` créent des commits sans correspondre à `*git*commit*` → les ajouter aux motifs du verrou `commit` (N10) ou les passer en `ask` ; `rm -rf .beads/hooks` → refus shell `*.beads/hooks*` (pas seulement l'écriture) et `*.beads/*` en écriture sauf les fichiers que `bd` écrit (avec N22).

**Fichiers** : `internal/bundle/globalguard.go` (nouveau), `build.go`, `gitguard.go`, `hookguard.go`, `permissions.go`, `internal/bricks/permission_resolve.go` ou `plugin_tools.go`, `workflow/hubcat/hubcat.go`, `permissions/developer-rw.yaml`, `agents/quality/reviewer.md`, golden des paquets, `SECURITY*.md`.
**Tests** : `bundle.TestGlobalGuard` (table de tous les contournements de l'analyse S2.3, via le simulateur du lot 0, pour developer, debugger, benchmarker, reviewer, infra, orchestrator-dev) avec une liste « doit rester autorisé » (les 22 commandes de `developer_permissions_test.go:34-41` + `git log -- x.go`, `git commit --no-edit`, `git switch -c feat/x`, `docker run --rm img`…) ; `TestFrontmatterDenyOrdering` (documente le piège) ; `TestCtxToolsFollowBash` ; e2e `TestE2EGitOutputAndUploadPack` (`git log --output`, `git fetch --upload-pack`, réécriture du fichier `.git` refusés).

### N4 — `debugger`, `infra`, `benchmarker` (DC3, DC4)
- **debugger** (`agents/quality/debugger.md:10`) : `bash: allow` → liste : `git log/show/diff/blame/status*`, `ls*`, `cat *`, `head*`, `tail*`, `wc*`, `rg*`, `grep*`, `bd show/list/search/create*` (passerelle), lanceurs de tests (`npm test*`, `go test*`, `pytest*`, `.venv/bin/python -m pytest*`, `cargo test*`) en `ask`. Prompt mis à jour (reproduction = demander, ou lancer un test).
- **infra** (`agents/developer/infra.md:9-21`) : retirer les autorisations globales (`terraform*`, `tf *`, `kubectl*`, `k9s*`, `helm*`, `aws *`, `gcloud *`, `az *`, `pulumi*`, `vault *`) ; **`InfraGuard()` en Go** (ajouté aux agents qui ont des outils infra) avec les listes de l'analyse S2.6 :
  - autorisé : `terraform fmt/validate/init/plan/show/state list|show/providers/graph/workspace list|show` (idem `tofu`), `kubectl get/describe/logs/explain/version/api-*/top/diff/auth can-i/kustomize/config get-contexts|current-context`, `kubectl * --dry-run=client|server*`, `helm template/lint/show/list/status/history/diff/search/dependency list|build/version/get manifest|notes`, `aws sts get-caller-identity`, `aws * describe-*`, `aws * list-*`, `aws s3 ls*`, `gcloud * list|describe`, `az * list|show`, `az deployment * what-if|validate`, `pulumi preview|stack ls|about|whoami`, `vault status|secrets list|kv list|kv metadata get`, `ansible-lint`, `ansible-playbook * --syntax-check|--list-*` ;
  - `ask` : lectures de secrets (`kubectl get secret*`, `helm get values|all`, `terraform output*`, `vault kv get*`, `aws * get-*`, `pulumi stack output*`), `kubectl port-forward`, `helm repo add|update`, `ansible-playbook * --check*` ;
  - refus, **ouvrables par `unlocks: [apply]`** : toutes les mutations (`*terraform* apply*`, `destroy`, `import`, `state rm|mv|push`…, `*kubectl* apply|create|delete|patch|edit|scale|rollout|exec|cp|drain…`, `*--dry-run=none*`, `*helm* install|upgrade|uninstall|rollback…`, `*aws* create-|delete-|put-|update-|terminate-|run-…`, `*aws* s3 cp|mv|rm|sync|mb|rb*`, `*gcloud* create|delete|update|deploy|ssh…`, `*az* create|delete|update|deploy…`, `*pulumi* up|destroy|refresh|import|state *`, `*vault* write|kv put|delete|token|login|operator…`), et extraction d'identifiants (`*get-secret-value*`, `*--with-decryption*`, `*get-login-password*`, `*assume-role*`, `*print-access-token*`, `*--show-secrets*`, `*VAULT_TOKEN*`, `*kubectl*config view*--raw*`) toujours refusée.
  - Opération `apply` (additive, DC4/DC9) : `workflow/schema.go:256-274` (`UnlockApply`), `validate_spec.go:518` (liste construite depuis `UnlockOps`, opération inconnue = avertissement), `sessionspec/workflow.go:99-101`, `bundle/checkpoints.go:89-93` (`lockPatterns[apply]` = mutations), `templates/partials.md.tmpl:31-32` (texte construit depuis les opérations déclarées — partagé avec N10), `internal/remote/manifest.go` (transporter `unlocks`).
  - Limite assumée (SECURITY) : `terraform plan`, `pulumi preview` exécutent du code du dépôt ; protection réelle = identifiants en lecture seule (rôle `ReadOnlyAccess`, kubeconfig `view`) ou conteneur sans `~/.aws`/`~/.kube`.
- **benchmarker** (`agents/quality/benchmarker.md:46`) : retirer `env *`, `printenv*` ; `hyperfine`, `perf record --`, `wrk -s` : documentés comme exécution arbitraire → `ask`, ou benchmarker proposé en conteneur uniquement.
- **Tests** : `TestInfraReadOnly` (autorisé / ask / refus, y compris options globales : `kubectl --context p delete`, `terraform -chdir=x apply`, `aws --profile p ec2 terminate-instances`) ; `workflow/unlocks_test.go` (`apply` valide, opération inconnue = avertissement, règles de session émises tant que verrouillé) ; e2e debugger : `sed -i` refusé, `go test` demandé.

### N5 — Conteneur et git d'oh sur la machine (DC5)
**Constat (revérifié)** : `runtime/container/spec.go:111-137` monte projet, worktrees et dossier commun git **en écriture** (dont `.git/config`, `hooks/`, fichier `.git` des worktrees, `worktrees/<x>/commondir`) ; données du groupe (`/opt/oh/data`, base opencode avec permissions enregistrées) en écriture ; volume `home` persistant entre sessions ; `runArgs` (`:179-227`) sans `--cap-drop`, `no-new-privileges`, `--pids-limit` ; volumes non validés (`:141-151`, injection d'options de montage par une virgule). Le démon lance sur la machine `git status` (`services/checkpoint/locks.go:60-72`, à chaque passage de checkpoint avec `unlocks` et chaque `bd close`), `git fetch/ls-remote/worktree add/checkout/stash` (`worktree/worktree.go`), `git diff` (`gitutil/diff.go`, `session_diff.go`) : `core.fsmonitor`, hooks `post-checkout`, `diff.external`, `remote.*.uploadpack`, `core.sshCommand` du dépôt s'exécutent alors **sur la machine**.

**Correction (1 + 2 + 3)** :
1. **Superpositions en lecture seule dans le conteneur** (après les montages du projet et du dossier commun) : `<p>/.git/config`, `<p>/.git/hooks`, `<p>/.git/modules/*/{config,hooks}` ; pour un worktree : `<common>/config`, `<common>/hooks`, `<wt>/.git` (fichier), `<common>/worktrees/<x>/{commondir,gitdir}` ; sources absentes ignorées ; `GIT_CONFIG_*` + `branch.autoSetupMerge=false`. Vérifier en intégration que le montage imbriqué en lecture seule sur un montage en écriture fonctionne sous Colima, Docker Desktop (virtiofs) et Podman machine.
2. **Enveloppe git sûre pour oh** (`gitutil/safe.go`), utilisée par tous les appels git d'oh sur un dépôt de projet : `--git-dir`/`--work-tree` tirés des enregistrements d'oh (ou vérification que le fichier `.git` pointe là où oh l'attend), `-c core.fsmonitor=false -c core.hooksPath=/dev/null`, `--no-ext-diff --no-textconv` (diff, log), `GIT_OPTIONAL_LOCKS=0`, `GIT_TERMINAL_PROMPT=0` ; pour fetch/ls-remote : `-c remote.origin.uploadpack=git-upload-pack -c protocol.ext.allow=never`, `GIT_SSH_COMMAND` et `credential.helper` repris de la config **globale** de l'utilisateur. Appels à migrer : `services/checkpoint/locks.go:61-67`, `worktree/worktree.go:55-442`, `gitutil/diff.go:43-148`, `gitutil/session_diff.go:153`, `beads/clean.go:162-424`, `services/remote/machine.go:232-294`, `tui/v2/views/status_view.go:241-250`.
3. **Intégrité** : empreinte de `<common>/config` (+ fichiers `include.path`) et de `hooks/*` au démarrage de la session ; comparaison avant chaque git d'oh et chaque checkpoint ; écart → appel refusé + décision ✗ « configuration git modifiée par la session » (local et conteneur).
4. **`runArgs`** : `--cap-drop=ALL --security-opt=no-new-privileges --pids-limit=<n>` (+ `--memory` depuis `[limits] memory_mb`) ; volumes : refus de `,` `"` retour à la ligne dans source/cible, cible interdite sous `/opt/oh/*`.
5. **Persistance** : volume `home` par session, ou remis à zéro au démarrage (évite un `.zshenv` laissé pour la session suivante) ; l'audit-v5 (S-33) précise que ce HOME est en **1777 et partagé** par toutes les sessions du projet (`runtime/container/image.go:270-274`) → HOME par session, 0700 ; fichier env du conteneur supprimé après le démarrage (S-35, `runtime.go:237-247`) ; option : base opencode hors du montage en écriture, ou contrôle de la table des permissions enregistrées.
**Tests** : `spec_test` (superpositions, sources absentes, volume refusé, options de sécurité) ; `real_test` par moteur (dans le conteneur : `git config x.y z` échoue, écriture de `.git/hooks/post-checkout` et du fichier `.git` échoue ; `git commit`, `fetch`, `switch -c`, `worktree add` réussissent) ; `gitutil.TestSafeGitIgnoresRepoConfig` (dépôt piégé : fsmonitor, post-checkout, diff.external, uploadpack, `.git` redirigé → rien n'est créé) ; `checkpoint.TestGuardRefusesTamperedConfig`.

---

## Piste W — Workflows et contenu

**Faits du moteur sur lesquels s'appuie la piste** : un paquet exige un workflow (`bundle/build.go:93-95`) → **aucune session v5 sans workflow** ; `workflow-map`, `orchestrator-workflow-modes`, `hub-workflow-reference` sont **générés** et remplacent les fichiers statiques (`bundle/workflowgen.go`) ; un verrou `after:` ne s'ouvre qu'après un vrai appel `workflow_checkpoint`, même en `auto` (`checkpoints.go:145-166`) ; la fenêtre `unlocks` se referme au premier `bd close` ou quand le checkpoint est redemandé (`state.go:164-212`, `locks.go:127-134`) ; une `question` qui contient un id (`cp-2`, `CP 2`, `cp-feature`) ou un libellé de checkpoint est fermée par oh (`locks.go:267-325`) ; le graphe du workflow remplace le `task` du frontmatter (`render.go:177-185`) ; workflows sans checkpoint : `libre`, `quick`, `debug`, `audit`, `onboarding`, `review`, `brief-enrich`.

### N6 — Un seul protocole de checkpoint (DC6)
- **Repère** : remplacer partout « premier message avec `Mode de workflow` » (présent aussi dans `libre`) par « **la table des checkpoints de la carte du workflow liste ce checkpoint** » (ou `workflow_status`).
- **Corrections** (détail de l'analyse W1, toutes les occurrences) :
  - `orchestrator-dev-subagent.md` : plus de « TERMINER LA SESSION » pour CP-1/CP-2/CP-3 ; orchestrator-dev appelle `workflow_checkpoint` lui-même en sous-agent (pris en charge : `DataToolSession`) ; `## Question pour l'orchestrator` réservée aux vraies clarifications.
  - `orchestrator-dev-ticket-workflow.md` : en-tête (`:8`) et exemples `question` des checkpoints remplacés par un seul exemple `workflow_checkpoint` ; supprimer `:399-441`, `:37-40`, `:128-131`, `:443-470`, `:638-641` ; ajuster `:553`.
  - `orchestrator-dev-standalone.md` (`:12-20`, `:38`), `orchestrator-dev-feedback-mode.md` (`:38-54` → `workflow_checkpoint cp-fix`).
  - `orchestrator-protocol.md` : `:20` (bash), `:28`, `:40` (ajouter `workflow_checkpoint`), `:179` (cp-0 par `workflow_checkpoint`).
  - `orchestrator.md` : `:96-97`, `:101`, `:112` ; `orchestrator-dev.md` : supprimer `:146-154`.
  - `orchestrator-handoff-format.md` : retirer le relais CP-2 (`:27`, `:49-57`, `:114`, `:141-165`) ; **aucun en-tête ne contient un id de checkpoint** (libellé de phase à la place).
  - `orchestrator-recap-edge.md:57` (en-tête « Review edge-case — fin de feature »), `:75` (« rappelle orchestrator-dev ; cp-2 sera redemandé »).
  - `posture/coordination-only.md:95`.
  - Fichiers statiques remplacés par la génération (`orchestrator-workflow-modes.md`, `hub-workflow-reference.md`) → bouchons « GÉNÉRÉ » ; supprimer les orphelins `orchestrator-ticket-routing.md`, `orchestrator-modes.md` (vérifier `skill_mentions_test.go:126`, `skills.go:89`).
  - Gabarit généré, branche « Aucun checkpoint » (`bundle/templates/*.tmpl`) : « pour toute confirmation, utilise `question`, sans nommer de checkpoint ».
- **Tests** : lint de contenu (nouveau, à côté de `bundle/beads_status_test.go`) : pour chaque workflow livré à checkpoints, aucune `question(` dont l'en-tête/le texte contient un id ou un libellé de checkpoint, aucune « terminer la session » près de `CP-n` ; `TestStaticGeneratedSkillsAreStubs` ; `TestNoOrphanSkills` ; e2e `feature` : orchestrator-dev sous-agent appelle `workflow_checkpoint cp-2` → décision avec `DataToolSession`, fenêtre ouverte, commit autorisé ; e2e négatif : `question` « [OrchestratorDev — CP-2 | …] » fermée ; e2e `libre` : « Commit ? » non fermée.

### N7 — `sweep` (DC11)
- **Constat** : blocage dans le **prompt** (le conductor peut `read/glob/grep`, mais `conductor.md:20,73` le lui interdit et `sweep.md.tmpl:19-23` lui demande un developer, verrouillé `after: cp-plan`) ; pas de reviewer ; pas de verrou de commit (les developers attendent un orchestrator-dev absent) ; consignes developer centrées sur les tickets (`bd ready`, `--claim`) dans un workflow sans ticket ; parallèle dans la même arborescence (`index.lock`, débordements de périmètre) ; `cp-plan` auto en mode auto avec jusqu'à 20 tâches ; `dry_run` seulement dans le prompt et la session n'est jamais « terminée » ; `verify_cmd` refusé par la liste d'autorisation et non validé.
- **Correction** :
  ```yaml
  agents:
    pathfinder: { role: workflow, mode: subagent }          # découpage en lecture seule, non verrouillé
    developer: { role: workflow, after: cp-plan }
    developer-refactor: { role: workflow, after: cp-plan }
    developer-migrator: { role: workflow, after: cp-plan }
    reviewer: { role: workflow, mode: subagent, after: cp-plan }
  checkpoints:
    cp-plan:   { mandatory: true, mode: { manuel: pause, semi-auto: pause, auto: pause } }
    cp-commit: { label: {fr: Commit du sweep, en: Commit the sweep}, mandatory: true, mode: { manuel: pause, semi-auto: pause, auto: pause }, unlocks: [commit] }
    cp-recap:  { mode: { manuel: pause, semi-auto: pause, auto: auto } }
  circuit_breaker: { max_consecutive_subagents: 12 }
  beads: { allow: [show, list, search] }
  ```
  Gabarit : découpage par `pathfinder` (glob par fichier/paquet, ou exploration) → plan (sous-tâches, **périmètres disjoints**, agent, drapeau « parallélisable ») → `{{ if .dry_run }}` afficher et **ne pas appeler `cp-plan`** `{{ else }}` `cp-plan` → exécution (consigne « pas de ticket : sauter les étapes bd, ne pas commiter, ne toucher que ton périmètre » ; parallèle seulement si disjoint) → vérification (`verify_cmd` si `custom`, en signalant qu'il peut être refusé) → `reviewer` sur le diff de la session → `cp-commit` (un commit, ou un par sous-tâche à la suite) → `cp-recap` (sortie `branch`). `pathfinder` : `bd create` à retirer dans ce workflow (`beads.allow`).
  Agents developer (`developer.md:46-51`, `developer-refactor.md:37-45`, `developer-migrator.md:51-59`) : branche « sans ticket » ; « commit seulement sur instruction du coordinateur après le checkpoint de commit » (au lieu de « orchestrator-dev »).
  Validateur : `verify: custom` sans `verify_cmd` → erreur ; nouvelle règle `parallel_writers_without_commit_gate` (≥ 2 agents qui écrivent, `risk: write`, aucun checkpoint `unlocks: [commit]`) → avertissement.
  `finished()` (`locks.go:168-207`) : une session `dry_run` terminée par le récap doit pouvoir être `completed` (sortie déclarée sans `cp-recap` passé, ou `cp-recap` permis en `dry_run`).
- **Tests** : golden `sweep/*` ; `TestShippedWorkflowPrompts` avec `dry_run=true` (le prompt n'appelle pas `cp-plan`) ; `LockedAgents` garde les developers verrouillés sans `cp-plan` en `auto` ; e2e `auto` : `task developer` avant cp-plan refusé, `task pathfinder` autorisé, `git commit` avant cp-commit refusé.

### N8 — `review-feedback` (DC12)
- **Constat** : cp-fix passé par `question` → `task developer` refusé ; « 3. Faire un commit » (`feedback-mode.md:68`) avant cp-2 → refusé ; pré-review demandée à orchestrator-dev sans les commandes (`:84`) ; personne ne pousse ; réponses GitLab sur un commit non poussé, outil interdit à orchestrator-dev et **sans identifiant de discussion** dans les données (`cmd/review_feedback.go:211-236`) ; `Ticket : <ID>` sans entrée ticket (`:62`, `:88`) ; `team_review_verdict … rejected` (`:89-91`) ; pas de nouvelle review.
- **Correction** :
  ```yaml
  agents:
    orchestrator-dev: { role: workflow, calls: [developer, reviewer] }
    developer: { role: workflow, after: cp-fix }
    reviewer:  { role: workflow, mode: subagent, after: cp-fix }
  checkpoints:
    cp-fix: (inchangé)
    cp-2:   { mandatory: true, pause dans tous les modes, unlocks: [commit] }
  ```
  `orchestrator-dev-feedback-mode.md` réécrit : classer → `workflow_checkpoint cp-fix` (classement dans `summary`) → developer : corriger, lancer les tests du projet, **ne pas commiter** (lignes `MR : <ref>`, `Branche : <branch>`, plus de `Ticket :`) → `reviewer` `[MODE:feedback]` (chaque retour traité, sur `git diff <base>...HEAD` + arborescence) → `workflow_checkpoint cp-2` → developer `git commit -m "fix(review): address reviewer feedback"` → récap : « poussez (`git push`) et résolvez les fils » ; **supprimer** réponses GitLab, `team_notify`, `team_review_verdict`. `review-feedback.md.tmpl:16-21` et `orchestrator-dev.md:135-140` alignés. Option (5.1, avec DC7) : identifiant de discussion dans `formatFeedbackDiscussions`.
- **Tests** : golden `review-feedback/*` (graphe avec reviewer) ; lint (N11) : pas de « Faire un commit » avant cp-2, orchestrator-dev sans commande npm ; e2e semi-auto : commit refusé avant cp-2, autorisé après, `git push` toujours refusé.

### N9 — `feature` : review de fin, commits par ticket, création de tickets (DC8, DC10)
1. **Review de fin** : `feature.yaml:30` → `orchestrator: { role: workflow, calls: [pathfinder, planner, designer, orchestrator-dev, reviewer, documentarian] }` + `reviewer: allow` dans le frontmatter de `orchestrator.md` ; prompt de review avec la base tirée de « Branche de travail » / `base` (plus de `main` en dur).
2. **Un cp-2 par ticket** : retirer « Commit tous » (`orchestrator-workflow-modes.md:172`, `orchestrator-handoff-format.md:3,165`, `orchestrator-dev-parallel.md:76-81`) ; chaque ticket = un appel `cp-2`. Documenter : `GuardClose` vérifie l'état du dossier de lancement ; des commits faits dans des worktrees parallèles ne déplacent pas ce HEAD (`locks.go:98-104`) → vérifier le worktree du commit, ou le documenter.
3. **Opération `create`** (additive) : `schema.go:267-274`, `sessionspec/workflow.go`, `validate_spec.go:518`, `lockPatterns` (`*bd*create*`, + `*bd*duplicate*`, `*bd*supersede*`), garde de la passerelle Beads (`daemon/gateway.go:58-70`, `OpCreate`), texte généré ; ne se referme pas sur `bd close` (seulement si le checkpoint est redemandé). `cadrage` : `cp-tickets` `unlocks: [create]` ; `feature` : nouveau `cp-tickets` (mandatory, pause) avant cp-0, planner relancé après lui pour créer (ses phases 5-6 ont déjà « valider puis créer »).
- **Tests** : `TestSessionRulesLockedOperations` (cas `create`) ; `locks_test` (commit refusé après le premier close) ; `validate_spec_test` (`unlocks: [create]` valide) ; golden `feature` (arête `orchestrator → reviewer`) ; e2e deux tickets (close A, commit B refusé, second cp-2 → autorisé) ; e2e `cadrage` (`bd create` refusé avant cp-tickets par la passerelle, autorisé après).

### N10 — `push` et motifs de verrou (DC7)
- Retirer `push` des `unlocks` de `ticket.yaml:52`, `feature.yaml:60`, `review-feedback.yaml:59`.
- `templates/partials.md.tmpl:31-32` : lister **seulement** les opérations déclarées (commit → `git commit`, close → fermeture du ticket, create → création de tickets, apply → modifications d'infrastructure) + « le push revient à l'utilisateur ».
- Validateur : `unlock_push_without_publish` (push dans `unlocks` et `risk != publish`) → avertissement.
- **Faux positifs des motifs** (`checkpoints.go:89-93`) : `*git*push*` bloque `git diff src/pushNotifications.ts`, `*git*commit*` bloque `git log --grep commit`, `*bd*close*` bloque `bd list --status closed`. Option retenue (à confirmer avec U1/U5) : motifs ancrés sur la sous-commande (`git commit*`, `* git commit*`, `git -c * commit*`, `env * git commit*`, `*sh -c *git commit*`…) + le contrôle côté passerelle Beads pour close/create ; table de faux positifs en test.
- **Tests** : `workflowgen_test` (la phrase ne cite que les opérations déclarées ; golden `ticket`, `feature`, `review-feedback`) ; `checkpoints_test` table : verrouillé, `bd list --status closed`, `git diff src/pushX.ts`, `git log --grep=commit` autorisés ; `git commit -m`, `env git commit`, `git -c a=b commit`, `sh -c 'git commit'` refusés (simulateur du lot 0).

### N11 — Consignes refusées par les permissions + contrôle automatique
- **Matrice** (analyse W6) — décision par cas :

| Agent | Consigne | Décision |
|---|---|---|
| documentarian | `bd ready`, `bd show bd-*` (préfixe faux), `--claim`, `bd close`, skill `beads-dev` | autoriser `bd ready*`, `bd show *` ; retirer claim/close (coordinateur) ; retirer `beads-dev` |
| designer | `bd update --claim`, `bd list` sans argument | retirer le claim ; `bd list*` |
| onboarder | `.git/info/exclude`, `shasum`, `git log`, `bd search` avec `bash: deny` | `.gitignore` proposé dans la passation ; pas de hachage (mtime) ; petite liste `git log*`, `bd search *`, `bd list *`, `ls*` |
| benchmarker | écrire `reports/benchmarks/` avec `edit: deny` | `edit` limité à `reports/benchmarks/*` + `mkdir -p reports/benchmarks*` |
| database | `EXPLAIN ANALYZE` sans client | consigne : fournir la requête à exécuter par l'utilisateur, ou `docker compose exec … psql` |
| developer-migrator | lire les changelogs sans web | `webfetch: allow` |
| auditor-subagent | `npm audit`, `pip-audit` avec `bash: deny` | consigne : lire les lockfiles + recherche CVE ; (option : permission dédiée `audit-runner`) |
| orchestrator-dev | pré-review `npm run lint`, `tsc`, `npm test`, `prettier --write .`, `bd comments add`, lecture de `package.json`, `git worktree remove *` | pré-review faite par le **developer** (critère de fin), orchestrator-dev lit les résultats de la passation ; `bd comments` côté developer ; retirer `git worktree remove *` du frontmatter |
| orchestrator | « bash restreint aux lectures » avec `bash: deny` | corriger le texte |
| conductor | « ne lis pas » avec `read/glob/grep: allow` | `read/glob/grep: deny` (le découpage passe par pathfinder, N7) |
| developer × 3 dans `sweep` | `bd ready`, `--claim`, « orchestrator-dev » | branche « sans ticket » (N7) |

- **Contrôle automatique** (« capability lint », peut sortir en 5.0.1) : `cli/internal/bundle/capability_test.go` (ou `oh hub lint --capabilities`) sur chaque workflow livré : extraire des corps d'agents et des skills livrées à l'agent les commandes (spans de code et blocs `bash`/`sh` dont le premier mot est dans un vocabulaire : `bd git npm npx yarn pnpm go cargo pytest python* uv poetry tsc ruff make docker psql mysql sqlite3 shasum mkdir k6 lighthouse hyperfine curl…`), normaliser les gabarits (`<ID>`), découper comme opencode, évaluer contre les règles fusionnées de l'agent (avec les gardes) → refus = **erreur** (sauf marqueur `<!-- capability: user -->` ou liste `testdata/capability_allow.yaml`) ; opération verrouillée = la ligne doit suivre une mention du checkpoint qui l'ouvre ; références `task(subagent_type: X)` ⊂ graphe ; `question(` non refusée ; éditions de `.git/*` ; mention web vs permission. Sortie de référence `testdata/capabilities.golden`. Contrôles associés : `unlocks` sans membre capable de l'opération → avertissement ; agent verrouillé `after: cp-x` sollicité dans le gabarit avant `cp-x` → erreur (aurait détecté N7).

---

## Piste R — Robustesse

### N12 — Projet implicite et vérification du fournisseur (DC13)
- **Projet** : `resolveProject` (`cmd/start_helpers.go:22-75`, **23 appelants**) prend le seul projet enregistré hors de tout projet (`:55-57`), et ouvre sinon un sélecteur huh non traduit qui échoue sans terminal. Nouvelle signature `resolveProject(ctx, a, id, opts{Write bool})` :
  - `Write` (run, alias de run, review feedback, mcp setup projet, project configure, budget set, team solo…) : hors projet → confirmation « Projet X (chemin) ? » ; sans terminal → erreur traduite `cmd.run.no_project` (« lancez depuis le dossier du projet ou utilisez -p ») ; sélecteur traduit ;
  - lecture (bundle show, workflow layers…) : avertissement sur stderr « projet X utilisé (-p pour un autre) » ;
  - `projet %q introuvable` traduit, avec « oh project list ».
  - `oh run` : vérifier `requireV2` **avant** `resolveProject` (`run.go:68` vs `v5_run.go:89`).
- **Fournisseur** : vérification commune CLI/TUI dans la préparation du lancement (avant l'émission du grant par le démon) réutilisant `provider.ResolveCredentialSource` (déjà utilisé par Doctor, `cmd/doctor.go:334`) : aucun identifiant → erreur traduite « aucun identifiant pour le fournisseur X (projet, équipe, hub ; profil AWS pour Bedrock) : lancez `oh provider setup` » ; en TUI, message dans la fiche (étape Récap) avec action « Configurer le fournisseur ». Traduire aussi l'erreur 422 du démon (`issuing proxy grant …`) en message actionnable.
- **Tests** : `run` hors projet avec 1 projet et sans terminal → erreur, aucun lancement ; avec terminal → confirmation ; commande en lecture → avertissement ; `run` sans identifiants → message traduit, aucun appel au démon ; fiche : Récap signale l'absence.

### N13 — Niveaux de démarrage, Doctor/Repair (DC14)
- **Constat** : `PersistentPreRunE` (`cmd/root.go:51-101`) → `initApp` (`:184-247`) pour tout (config, ouverture + migration de la base, trousseau, 3 migrations, `localizeCommands`) ; `doctor` et `repair` utilisent `MustApp()` (`doctor.go:99`, `repair.go:30`) alors que leurs vérifications tolèrent déjà `TryApp()==nil`. `oh mcp serve workflow` n'utilise que le socket du démon mais ouvre et migre la base. `MigrateKeychainKeys` : 8 à 16 appels au trousseau par processus, possible demande de phrase secrète sans terminal. `repair` : relance les migrations (`:43`), sauvegarde sans `-wal`/`-shm` (`:117-123`), conseille `rm oh.db` même pour une migration en échec (`:106-111`).
- **Correction** : annotation `cmd.Annotations["oh.init"]` (héritée des parents) :

| Niveau | Commandes | Étapes |
|---|---|---|
| `none` | version, help, completion, `runner install`, `beads`, `mcp serve workflow` | journalisation seule |
| `light` | autres `mcp serve`, `daemon run`, `runner run` | config en lecture (jamais `Save`), base ouverte et migrée seulement si nécessaire (démon, runner), secrets à la demande, **sans** migrations trousseau/workflows ni `localizeCommands` |
| `degraded` | `doctor`, `repair`, `purge`, `config path/edit`, `export` | chaque étape protégée ; échecs dans un `StartupReport{ConfigErr, DBErr, MigrationErr, SecretsErr, SchemaVersion, BinaryMaxVersion, Pending}` ; App partielle |
| `full` | le reste | comportement actuel ; en cas d'échec, renvoi vers `oh doctor` / `oh repair` |

  Doctor dégradé : vérification config (erreur TOML avec ligne/colonne, sauvegarde `hub.toml.bak-*`, `--fix` = restaurer après confirmation) ; base (ouverture brute sans migration, `integrity_check`, version du schéma vs binaire, migrations en attente, dernière erreur, taille du WAL). Repair : jamais `MustApp`, `sqlite.OpenRaw`, sauvegarde par `VACUUM INTO` (ou copie db+wal+shm), `--migrate` (sauvegarde puis migrations avec erreur détaillée), conseil de suppression seulement si l'intégrité échoue.
  Coûts : `MigrateKeychainKeys` une seule fois (marqueur), jamais sous `full` ; `i18n.KeyOf` avec index inverse construit une fois ; `localizeCommands` seulement pour l'aide et les erreurs.
- **Tests** : `hub.toml` invalide → `oh doctor` code 1, vérification config en échec, autres vérifications exécutées ; `oh repair --check-only` avec config cassée ; migration en échec simulée → Doctor la montre, `repair --migrate` sauvegarde ; base corrompue → repair sans `MustApp` ; `oh mcp serve workflow` démarre avec une base inouvrable (la base n'est jamais ouverte) ; `MigrateKeychainKeys` ≤ 1 fois ; banc `KeyOf` + équivalence ; `TestEveryHelpIsTranslated` vert.

### N14 — Migrations SQLite sûres
- **Constat** : `migrate()` lit les versions appliquées hors transaction (`storage/sqlite/store.go:133-151`), transactions différées (`:190-207`) → deux processus (TUI + démon + plusieurs `oh mcp serve` au premier lancement après mise à jour) exécutent le même `ALTER TABLE` → « duplicate column » → démarrage en échec ; pas de sauvegarde ; aucune garde si le schéma est plus récent que le binaire.
- **Correction** : (1) chemin rapide sans verrou si rien n'est en attente ; sinon `filelock.LockContext(30 s, oh.db.migrate.lock)` ; (2) connexion dédiée, `BEGIN IMMEDIATE`, relecture de `schema_migrations`, une transaction par version ; (3) migrations idempotentes (« duplicate column » / « already exists » tolérés si `pragma_table_info` confirme) ; (4) sauvegarde `VACUUM INTO <hub>/backups/oh.db.v<from>-<ts>.bak` (0600, 3 conservées) avant la première migration d'une base existante ; échec de sauvegarde → refus (contournement `OH_SKIP_DB_BACKUP=1`) ; en cas d'échec de migration : « sauvegarde X ; `oh repair --restore X` » ; (5) garde : une version appliquée > 31 inconnue du binaire → `ErrSchemaNewer` (commandes `full` refusées avec message ; `degraded`/`light` continuent et le signalent ; `OH_ALLOW_NEWER_SCHEMA=1`) ; (6) exposer `MaxKnownVersion`, `PendingMigrations`, `OpenRaw`, `Backup`, `Migrate` pour N13.
- **Tests** : N `sql.DB` distincts ouvrent une base neuve en même temps → tous réussissent, chaque version insérée une fois (+ variante multi-processus) ; base à moitié migrée → ouverture réussie ; sauvegarde créée, ouvrable, contient les lignes du WAL ; `schema_migrations(999)` → `ErrSchemaNewer`, contournement OK ; chemin rapide sans fichier de verrou.

### N15 — Verrou du team-state
- **Constat** : `Repo.mu` par instance (`teamstate/repo.go:37-45`), ~80 créations de `Repo` (liste complète dans l'analyse R3) → rien n'est sérialisé, même dans un processus (ex. `sessionEndHook` : deux instances en parallèle) ; `pull --rebase --autostash` concurrent ; `Transact` fait `reset -q --hard HEAD~1` en cas de push refusé (`transact.go:171-199`) et peut détruire les fichiers en cours d'un autre écrivain ; `CommitAndPush(".")` (`tracker/sync.go:200`, `team_solo.go:176`) commite les fichiers d'autrui ; boucles TUI toutes les 5 s ; serveur MCP `team` par serveur d'outil ; `index.lock` laissé par un git tué ; file hors ligne sans verrou (`transact.go:298-342`).
- **Étape A (bloque REL)** : `.git/oh.lock` (`filelock`) + **registre par chemin dans le processus** (`sync.Map[path]*repoLock` : mutex + compteur + libération du flock ; `filelock` n'est pas réentrant) ; toutes les opérations git sous ce verrou ; écritures interactives `LockContext(30 s)`, démon et synchronisation du suivi 2 min, synchronisations de fond TUI en `TryLock` (on saute si occupé) ; pid/commande/heure écrits dans le fichier de verrou pour le message « occupé par… » ; délais réseau bornés (`GIT_HTTP_LOW_SPEED_*`, `ConnectTimeout`) ; `index.lock` de plus de 2 min supprimé sous notre verrou (avec avertissement) ; `reset --hard HEAD~1` → `reset --soft HEAD~1` + restauration des seuls fichiers de la transaction ; file hors ligne sous le verrou.
- **Étape B (5.0.1)** : `Transact` seul chemin d'écriture (`CommitAndPush` et `withWriteLock` réimplémentés dessus) ; `"."` interdit (erreur) — les appelants listent leurs fichiers ; synchronisation du suivi en une `Transact` par exécution.
- **Tests** : deux `Repo` sur le même chemin, 50 `AppendEvent` concurrents → 50 lignes, 50 commits (+ variante multi-processus) ; fichier non suivi d'un autre écrivain conservé après un push refusé ; `index.lock` ancien supprimé, récent → délai « occupé » ; `CommitAndPush(".")` → erreur (B) ; synchronisation TUI sautée si le verrou est pris.

### N16 — Nettoyage et rotation des logs (DC15)
- **Constat** : rien n'est supprimé (paquets, `servers/<groupe>/data`, `oh-server.log`, `ohd.log`, `sessions/<id>/`, lignes `sessions`, `agent_events`, `usage_*`, `pending_decisions`, worktrees automatiques) ; `worktree.auto_cleanup` jamais lu (le texte i18n promet le contraire) ; `.build-*`/`.fetch-*` laissés si le processus est tué ; reprise possible pour tout sauf `stopped` (`runsvc/service.go:795`) ; la clé de groupe contient l'empreinte du paquet ; `Build` réutilise un paquet sans toucher sa date (`build.go:276-277`) ; `CleanupMerged` (`worktree.go:201-246`) considère une branche neuve comme fusionnée et `IsMerged` ignore le préfixe `+ ` des branches de worktrees.
- **Rotation des logs (bloque REL)** : `internal/logrotate` (`Rotate(path, max, keep)`, `CopyTruncate`) ; `ohd.log` tourné dans `spawnDaemon` sous `SpawnLock` (`daemon/client.go:325`) ; `oh-server.log` tourné dans `StartServer` (`server.go:198-202`) ; copie + troncature pour les fichiers des processus vivants (écriture en `O_APPEND`) depuis la tâche périodique.
- **`oh gc` (5.0.1)** : paquet `internal/gc` (`Plan` lecture seule → `Apply`), section `[gc]` (`enabled`, `every_hours=24`, `session_days=30`, `history_days=180`, `usage_days=400`, `bundle_grace_hours=24`, `log_max_mb=20`, `log_keep=3`) ; invariants :
  1. un paquet est vivant si une session non `stopped` l'utilise, ou un serveur non arrêté, ou une session de moins de `session_days`, ou une référence distante en attente, ou s'il a moins de `bundle_grace_hours` (et `Build` touche sa date à la réutilisation) ;
  2. `servers/<gk>/` seulement si toutes ses sessions sont `stopped` et anciennes, le PID mort, et le verrou de groupe pris pendant la suppression ; puis ligne `servers` supprimée et grant révoqué ;
  3. `sessions/<id>/` seulement pour les `stopped` anciennes ou orphelines (`results/` gardés jusqu'à `history_days`) ;
  4. worktrees : seulement **automatiques** (nouvelle colonne), non utilisés par une session non arrêtée, propres (`status --porcelain --untracked-files=all` vide), non verrouillés, sans commit non poussé ; `worktree remove` sans force ; branche supprimée seulement si fusionnée (`branch -d`) ; `IsMerged` via `merge-base --is-ancestor` ;
  5. `.build-*`/`.fetch-*` de plus d'une heure ;
  6. verrou global `~/.oh/run/gc.lock` ;
  7. suppressions SQL par lots de 500 + `PRAGMA optimize`, pas de VACUUM dans le démon.
  CLI `oh gc [--dry-run] [--yes] [--json] [--only …] [--older-than 30d]` (inventaire avec tailles, confirmation) ; tâche périodique du démon (`daemon.Options.Periodic` devient une liste de tâches ; `gc.last`) qui ne garde pas le démon actif ; **premier passage en simulation** (« N Go récupérables, lancez `oh gc` ») ; `auto_cleanup` = interrupteur des worktrees automatiques, traité par le démon (texte i18n corrigé : « périodiquement »).
  **Migration v43** : `sessions.worktree_auto INTEGER NOT NULL DEFAULT 0`, `sessions.worktree_branch TEXT NOT NULL DEFAULT ''` (renseignés dans `runsvc/run.go:194`) + index `sessions(state, ended_at)`, `agent_events(started_at)`, `pending_decisions(resolved_at)`, `usage_sessions(day)`. Sessions antérieures = « inconnu » : jamais supprimées automatiquement.
- **Tests** : table état × âge × serveur (garder idle/sleeping/completed/failed/active, supprimer seulement les `stopped` anciennes) ; paquet partagé gardé ; délai de grâce ; verrou de groupe occupé → sauté ; worktrees sur vrai dépôt (sale, non suivi, commit non poussé, verrouillé, utilisé, non automatique → gardés ; propre, poussé, automatique → supprimé ; branche supprimée seulement si fusionnée) ; `IsMerged` avec `+ ` ; `CopyTruncate` avec un écrivain en `O_APPEND` ; `--dry-run` ne supprime rien ; tâche périodique une fois par intervalle, sans garder le démon actif.

### N17 — Nettoyage Beads sans confirmation (DC16)
- **Constat** : `autoCleanBeads` (`cmd/beads_clean.go:29`) à l'enregistrement d'un projet et à l'initialisation de Beads : modifie les fichiers du projet et peut annuler le commit de `bd init` (`internal/beads/clean.go:192`, `git reset -q HEAD~1`), sans confirmation ; écart avec la règle 8 de `00-INDEX`.
- **Correction** : afficher les changements prévus (`printCleanActions` en simulation) puis confirmer (comme `oh doctor --fix`, `doctor.go:84`) ; sans terminal → rapport seul + « lancez `oh doctor --fix` » ; annulation de commit seulement avec `--yes` explicite ; TUI : fenêtre de confirmation ; CHANGELOG et guide zéro impact mis à jour.
- **Compléments de l'audit-v5 (F-01)** : l'annulation s'appuie sur des refs distantes possiblement périmées (faire `git fetch` du distant suivi d'abord, ou ne jamais annuler si le distant n'est pas joignable) ; `update-ref -d HEAD` sur un premier commit → interdit ; les hooks bd de `.claude/settings.json` **non suivi** sont supprimés → ne toucher qu'aux fichiers suivis ou demander ; `.beads/` déplacé vers `.git/info/exclude` casse le partage de Beads par git → respecter un `.beads/` suivi (ne rien exclure) ; réglage `[beads] zero_impact = ask|auto|off` (défaut `ask`).
- **Tests** : sans terminal → aucun fichier modifié, aucun reset ; avec confirmation refusée → rien ; acceptée → nettoyage ; `--yes` → annulation du commit (non poussé) ; commit poussé → jamais.

### N18 — Processus `opencode serve` orphelins
- **Constat** : 24 processus vivants (~3,8 Go) lancés le 08/10 par `TestContractServerStartupFailure` (20) et `DiscoverNatives` (4) ; non reproduit isolément → tests/processus **interrompus** avant le `Stop` différé (`setDetached` = `Setsid`, le serveur survit à la mort du parent).
- **Correction** : tests de contrat : `t.Cleanup` qui tue le groupe (au lieu d'un arrêt en fin de fonction) + `TestMain` qui balaie les serveurs dont le dossier courant est sous `$TMPDIR/Test*` ; `DiscoverNatives` : arrêt sur signal (contexte annulé), dossier de travail reconnaissable ; Doctor : vérification « serveurs opencode orphelins » (processus `opencode serve` dont le dossier n'appartient à aucun groupe connu ni à un `OH_HOME`) avec `--fix` ; `oh gc` les inclut.
- **À faire tout de suite** (hors piste) : `for p in $(pgrep -f "opencode serve --hostname"); do lsof -a -p $p -d cwd -Fn | grep -qE "T/(Test|oh-natives-)" && kill $p; done`.

---

## Piste X — TUI (raccourcis et petits terminaux)

### N19 — Contrat des touches (DC17, DC18, DC19)
- **Ordre réel (revérifié)** : `app.SetInputCapture(globalKeyHandler)` (`shell/shell.go:222`) s'exécute **avant** le widget focalisé ; dans le gestionnaire : quitter → sélection → palette active → fenêtres → vue `InputCapturing` → Ctrl+P → Ctrl+T → Échap (dépile) → `HandleKey` de la vue → lettres non consommées : `j/k/g/G` traduites, `?` aide, `d` ferme un toast **sinon tombe**, **toute autre lettre ouvre la palette** (`:1394`) → F1 → **Entrée ouvre la palette** (`:1405`) → reste au widget. Conséquences : Entrée n'atteint jamais une liste dont la vue ne la consomme pas (catalogue : `workflows_view.go:283-286` rend les touches non-lettres) ; la vue reçoit `d` avant le shell (worktrees : suppression immédiate, `worktree_view.go:93-95`).
- **Contrat** :
  1. couches : urgence (Ctrl+C/Ctrl+Q) → fenêtres/palette (reçoivent tout) → vue → widget focalisé → replis du shell ;
  2. le shell ne prend plus Entrée ni les lettres au widget : Entrée rendue à tview (palette seulement si le focus n'accepte pas Entrée : helper `focusAcceptsEnter` pour List, Table, SectionedList, Button, Form, InputField, DropDown, TreeView) ; palette sur `/` et Ctrl+P seulement ;
  3. touches réservées que les vues ne redéfinissent pas : `?`, F1, Ctrl+P, Ctrl+T, Échap, Ctrl+Q/C ; toasts fermés par **Échap** quand un toast est affiché (avant le dépilement) ;
  4. `ShellAccess.Confirm(title, detail, danger, onYes)` pour toute action destructrice : worktrees `d`/`p`/`x` (+ grep de tous les `case 'd':`/`'x'`), opérations git hors de la boucle d'affichage ; `x` (nettoyage des fusionnées) n'agit pas sur un worktree utilisé par une session (hérite du correctif `IsMerged` de N16) ;
  5. fiche de lancement : `Ctrl+S` = aller au Récap ; lancement seulement depuis le Récap chargé (`launchWith` exige `step == Recap && recap != nil`) ; retour `Alt+←`/`PgUp` (suivant `Alt+→`/`PgDn`) en plus de `Ctrl+B`, affichés dans l'aide ;
  6. assistants : Échap → « Quitter l'assistant ? [Quitter] [Passer l'étape] (facultative) [Continuer] » ; Quitter appelle `OnComplete(false, nil)` et dépile ; plus de double Échap silencieux ; mêmes touches de retour que la fiche ;
  7. aide (`?`) et `docs/guides/tui-usage.*` mises à jour (changement de comportement : saisie « n'importe où » retirée, `d` ne ferme plus les toasts) ; notes de version.
- **Fichiers** : `shell/shell.go`, `shell/omnibar.go`, déclaration de `ShellAccess` (`views/view.go`), `views/worktree_view.go` (git injectable), `views/launch_form.go`, `views/inline_wizard.go`, `views/workflows_view.go`, `shell/help.go`, i18n (`tui.keys.*`, `tui.confirm.*`).
- **Tests** (`SimulationScreen`, modèles `views/sessions_view_test.go:186`, `shell/shell_test.go`) : `shell/keys_test.go` (Entrée sur SectionedList focalisée → sélection, pas la palette ; sans focus → palette ; `/` → palette ; `x` non géré → rien ; Échap avec toast → toast fermé sans dépiler ; `d` géré par une vue → vue seule) ; catalogue : Entrée lance (espion `Launch`) ; worktrees : `d` → confirmation, rien supprimé avant ; fiche : Ctrl+S à l'étape 0 → Récap sans lancer, second Ctrl+S → lance ; assistant : Échap sur étape obligatoire → Quitter → `OnComplete(false)` + dépilé.

### N20 — Mode compact et terminal trop petit (DC20)
- **Constat (calculé)** : 80×24 → contenu 24 − 8 (palette fixe, `shell.go:215`) = 16, − marge 1 = 15 ; accueil : en-tête `bh+3 ≈ 9` (`home.go:123`), pied 4 (`:134`), espaceurs (`home_layout.go:168-225`) → **liste de 0 ligne** ; largeur étroite fixe 72 (`home_layout.go:36-45`) déborde sous 76 colonnes ; même schéma dans `project_mode_view.go` et `team_mode_view.go`.
- **Correction** : niveaux selon la hauteur du contenu (seuils centralisés : `minCols=60`, `minRows=16`, `compactRows=32`) — **complet** ≥ 32 ; **compact** 16–31 : bandeau sur une ligne (repli de `renderBanner`), pied supprimé, pas d'espaceurs, **palette sur 3 lignes** (`Omnibar.SetCompact`, `ResizeItem` dynamique au lieu de 8 en dur à `shell.go:1072` et `:268`), marge haute 0 → **≈ 19 lignes de liste en 80×24** ; **trop petit** < 60×16 : message centré « Terminal trop petit (L×H) — 60×16 minimum. Ctrl+Q pour quitter » ; largeur étroite = `min(72, largeur)` ; reconstruction au changement de niveau en gardant le focus (`homeOwnsFocus`) ; option `[cli] compact = auto|always|never`.
- **Tests** : 80×24 → liste ≥ 10 lignes et premier élément visible (`sim.GetContents`) ; 120×40 → bandeau complet ; 50×12 → message et Ctrl+Q ; redimensionnement 120×40 → 80×24 → changement de niveau, focus conservé.

---

### N24 — Superposition « ne peut que durcir » (audit-v5 S-16→S-20)
- **Constat (prouvé par l'audit-v5)** : `workflow/patch.go:398-402` compare `beads.allow` au parent seulement s'il en a un ; un parent **sans** bloc `beads:` (la passerelle applique alors la lecture seule, `gateway/policy.go:11`) laisse l'enfant autoriser `create, close, delete` sans diagnostic. Un checkpoint non obligatoire qui porte `unlocks:` peut passer en `skip` dans tous les modes (`patch.go:314-327`) : `LockedOps` l'ignore (`bundle/checkpoints.go:98-116`), le verrou disparaît. `mcp` et `plugins` remplacés sans contrôle (`patch.go:125-130`, un plugin exécute du code). `risk: read` (`review`, `audit`) garde les MCP du projet en écriture (`validate_spec.go:712-739`, `services/workflow/resolve.go:95-106`). Une couche peut vider `after:`, élargir `calls`, ajouter des agents, relever `circuit_breaker`, ajouter `skills.extra` (`patch.go:107-124,250-266`).
- **Correction** : parent sans `beads:` comparé à `DefaultBeadsAllow` ; un checkpoint qui porte `unlocks:` est traité comme `mandatory` (jamais `skip`, diagnostic si un patch essaie) ; `mcp` et `plugins` : sous-ensemble du parent seulement (ajout = diagnostic `loosening`, sauf couche hub) ; `risk: read` : MCP sans outils d'écriture (`*_WRITE_ENABLED` refusé) et sans `ctx_*` ; `after:` ne peut pas être retiré, `calls` ne peut que se réduire, `circuit_breaker` ne peut que baisser ; `skills.extra` autorisé (contenu, pas permission) mais signalé dans l'impact de publication ; `enforce` respecté.
- **Tests** : les PoC S-16 et S-17 produisent un diagnostic d'erreur ; table de tous les champs (durcir OK / assouplir refusé) dans `workflow/patch_test.go` ; `risk: read` + MCP en écriture → erreur.

### Complément N11 — Verrou de commit des workflows qui écrivent (DC26)
- `quick.yaml`, `debug.yaml` : `checkpoints: { cp-commit: { label: {fr: Commit, en: Commit}, mandatory: true, mode: {manuel: pause, semi-auto: pause, auto: pause}, unlocks: [commit] } }` ; gabarits : « termine par `workflow_checkpoint cp-commit` avec le résumé des changements ; commit après validation » ; consigne developer « commit seulement après le checkpoint de commit » (déjà prévue par N7).
- `onboarding` : édition limitée à `docs/wiki/**` (+ `ONBOARDING.md`, `CONVENTIONS.md` si utilisés), pas de commit par l'agent.
- `libre` : inchangé ; documenter dans la fiche/le récap « aucun verrou de commit dans ce workflow ».

---

## Piste G — Passerelles et proxy d'identifiants

### N22 — Passerelle Beads (audit-v5 S-06→S-09, S-14 ; DC24)
- **S-06 (prouvé, revérifié)** : `fakebd/run.go:46` lit `BD_GIT_HOOK` dans l'environnement de l'appelant ; `gateway/beads.go:90-93` remplace alors `beads.allow` par `gitHookAllow`. Un agent fait `BD_GIT_HOOK=1 bd hooks run post-merge …` (avec arguments) et, combiné à l'écriture de `.beads/*.jsonl`, modifie ou ferme des tickets sans contrôle. **Correction** : le hook installé par oh (crochets bd gérés par oh depuis T2) obtient un **jeton à usage unique** du démon (route privilégiée, durée courte, lié au groupe et au dépôt) et l'envoie ; la passerelle n'accorde le mode hook qu'avec ce jeton ; en mode hook, seulement `hooks run <nom-de-hook-connu>` avec les arguments que git passe (nombre et forme vérifiés) ; refus shell `*BD_GIT_HOOK*` (N3).
- **S-07 (prouvé, revérifié)** : `gateway/tickets.go:51-62` ne lit le statut que pour `-s`/`--status` exacts → `update -sclosed` (forme collée de pflag) passe ; `duplicate`, `supersede` (et `import`) ferment sans être reconnus. **Correction** : analyse des arguments avec le jeu d'options réel de `bd` (formes collées `-sX`, `--x=`, options combinées) **ou** refus des formes collées inconnues ; `duplicate`, `supersede`, `import` traités comme des fermetures (`OpClose`) ; en plus, contrôle du statut **avant/après** (la passerelle relit le ticket après l'exécution : passé à `closed` sans autorisation → annulation et refus) si `bd` le permet.
- **S-08 (prouvé)** : `gateway/policy.go:49-59`, `gateway/paths.go:18-31,102-126` : `-qC/autre` (lit un autre projet), `--DB=/x`, `-o/etc/x`, `-f/…/.ssh/id_rsa` passent sous forme collée ; redirection possible de la base via `.beads/redirect` ou `config.yaml` écrits par l'agent (à confirmer). **Correction** : même analyseur qu'en S-07 ; base fixée par le démon (`--db` imposé, `BEADS_DIR`), `.beads/` protégé en écriture sauf ce que `bd` écrit (refus d'édition `.beads/redirect`, `.beads/config.yaml`), chemins `-o`/`-f` confinés au projet.
- **S-09** : garde « fermé seulement si commité » (`services/checkpoint/locks.go:51-54,101-104`, `daemon/gateway.go:65-67`) : n'importe quel commit suffit (même `--allow-empty`), et une erreur git ou de garde est traitée comme un succès. **Correction** : la garde échoue en fermé (erreur = refus) ; exige un HEAD déplacé depuis le passage du checkpoint **et** un commit non vide qui touche des fichiers suivis ; arbre propre.
- **S-14** : `bd` lancé avec tout l'environnement du démon (`gateway/beads.go:145-149`) → environnement minimal (PATH, HOME, LANG, `BEADS_*`) ; jetons de sous-agents révoqués à la fin de la sous-session ; `gateway.json` vérifié au redémarrage.
- **Tests** : les PoC S-06, S-07, S-08 de l'audit-v5 deviennent des tests de non-régression (`gateway/*_test.go`, + contrat `integration` avec le vrai `bd` 1.3.1) ; garde de fermeture : commit vide → refus, erreur git → refus.

### N25 — Proxy d'identifiants (audit-v5 S-21→S-23, S-25)
- **S-21 (prouvé)** : `credproxy/providers.go:14-20` construit l'hôte Bedrock depuis `AWS_REGION` sans validation → `x.evil.com#` envoie la signature/clé à `bedrock-runtime.x.evil.com`. **Correction** : région validée `^[a-z]{2}(-[a-z]+)+-\d$` (+ liste des régions Bedrock connues en avertissement), à la configuration **et** à chaque requête ; hôte construit puis comparé à `*.amazonaws.com`.
- **S-22** : `credproxy/routes.go:114-149` : chez OpenRouter, `models`, `route`, `preset` contournent la liste des modèles ; données après le premier objet JSON non contrôlées. **Correction** : refuser ces champs (ou les valider contre la liste) ; décodeur strict (`DisallowUnknownFields` sur les champs de routage, refus des données après l'objet).
- **S-23** : `storage/sqlite/server_store.go:161`, `credproxy/proxy.go:63` : `allowed_models` illisible → erreur ignorée → liste vide → tout autorisé. **Correction** : illisible = refus (échec fermé) + log.
- **S-25** : second écouteur du proxy (Linux, conteneur) : toute IP privée acceptée (`daemon/handlers.go:207-209`), adresse fournie par l'image, hôtes restaurés sans contrôle depuis `ohd.json` (`daemon/daemon.go:356-359`). **Correction** : seulement les interfaces virtuelles du moteur de conteneur (bridge docker/podman détecté), revalidées à la restauration. *(peut passer en 5.0.x si DC21 le permet ; laissé dans G)*
- **Tests** : PoC S-21 refusé ; OpenRouter `route`/`models` refusés ; `allowed_models` corrompu → requêtes refusées ; écouteur sur une IP de LAN refusé.

---

## Piste D — Mode distant (DC22)

### N21 — Distant
- **Pour 5.0 (bloquant)** : drapeau expérimental — `[remote] experimental = true` requis par `oh remote setup`, `oh run --runtime remote`, l'option ☁ de la fiche et `runtime.allowed` (sinon message « mode distant expérimental, non validé : `oh config set remote.experimental true` ») ; avertissement permanent dans le récap et `oh remote status` ; docs (`remote-runners.*`, SECURITY) alignées (D-04). **S-03** (empreinte du paquet téléchargé, `services/remote/fetch.go:186-212`, `runner/fetch.go:96-110`) corrigé dans N23 (commun).
- **5.0.x, avant de lever le drapeau** (+ recette réelle de `09`) :
  - **S-01 (prouvé)** : `runner/git.go:22-33,83-113`, `runner/runtime.go:109-123` : `Finish` fait `git status/add/commit/push` **en root** dans un dossier de l'uid de l'agent, avec `safe.directory=*` et le jeton dans `GIT_CONFIG_VALUE_0` ; `--no-verify` ne bloque ni `post-commit`, ni `pre-push`, ni `core.fsmonitor` → un hook déposé par l'agent s'exécute en root et lit les secrets CI (`/proc/1/environ`). **Correction** : tuer tous les processus de l'uid agent avant `Finish` ; clone neuf appartenant à root (ou export du diff appliqué dans un clone propre) ; sinon `-c core.hooksPath=/dev/null -c core.fsmonitor=false`, `GIT_CONFIG_GLOBAL=/dev/null`, `GIT_CONFIG_NOSYSTEM=1` ; jeton par `GIT_ASKPASS` éphémère, jamais dans une variable lisible ni une URL ; réutiliser `gitutil.Safe` (N5).
  - **S-02** : `remote/ciconfig/ciconfig.go:199-228`, `services/remote/image.go:74-81` : build d'image (Kaniko/DinD) qui exécute les `RUN` d'un projet dans un job détenant les secrets de **tous** les projets ; jeton dans l'URL de clonage en DinD ; étiquettes calculées sur le seul Dockerfile. **Correction** : un job de build par projet cible, `CI_JOB_TOKEN` seul, aucun secret oh ; jamais de jeton dans une URL ; étiquette = empreinte du Dockerfile + contexte.
  - **S-04 (prouvé)** : `services/remote/replay.go:136,321-327` : identifiant avec tiret dans le préfixe non reconnu comme ticket (`close my-app-zz9` passe), options `--parent`/`--deps` non contrôlées, journal non authentifié. **Correction** : rejeu via l'analyseur de la passerelle (N22), limité aux tickets de la session, journal signé (HMAC avec une clé de session connue de la machine seule).
  - **S-05** : `cmd/session_resolve.go:47,62` : journal affiché brut → caractères de contrôle (ANSI/OSC) neutralisés à l'affichage.
- **Tests** : drapeau absent → refus traduit partout ; paquet distant modifié → refusé (N23) ; PoC S-01 (hook `post-commit`, fsmonitor) sans effet ; PoC S-04 refusé ; affichage du journal sans séquence de contrôle.

---

## Piste Rel — Hygiène de release (N27)
Petit lot, à faire directement sur `main` ou dans une branche courte, avant le tag :
- `golang.org/x/text` ≥ 0.39 (GO-2026-5970, atteint via `views/team_mcp_view.go:290`) ; `govulncheck` propre ; fusionner/réviser les PR Dependabot en attente (`go-isatty`, `testify`, `x/crypto`, `modernc.org/sqlite`, `bubbletea`).
- golangci-lint de la CI aligné sur 2.14 (version fixée) ; corriger les 11 constats (8 SA1019 `TrackerConfig`, 3 gocritic).
- `.github/workflows/release.yml` : déclenchement `push: tags: ['v*']` (aujourd'hui `branches: [main]` + `head_branch` en `v…` se contredisent, S-41) ; essai sur un tag de test (`v5.0.0-rc.1`).
- SECURITY : contact réel (S-42) ; Cosign : soit vérifié dans `selfupdate` (`selfupdate.go:216-235,376-396`) et `install.sh:143-163` (échec si absent), soit SECURITY dit clairement « non vérifié » (S-39) — **recommandé : vérifier** (5.0.x si trop long, avec la doc corrigée pour 5.0).
- CHANGELOG : section `5.0.0` séparée de `[Unreleased]`, réécrite pour l'utilisateur (déjà au plan de l'audit, lot D/docs).

---

## Hors de ce fichier (P1 de l'audit-v5, à planifier dans `18-…` avec les points 🟠 du premier audit)
- **MCP** : passerelle MCP qui laisse passer sans `_meta`, sessions d'un groupe non séparées, `io.ReadAll` sans limite (S-13) ; URL MCP imposée par l'équipe → jeton Jira redirigeable (S-31) ; chemins d'API Jira/GitHub non échappés, sans tests (S-32).
- **Prompts** : délimiteurs fixes, neutralisation à l'orthographe exacte, branches de MR insérées brutes (S-34).
- **Divers** : plugins de la config utilisateur exécutés en local non strict (S-11) ; budgets sous-comptés, contrôlés seulement en fin d'étape racine (S-27, F-04) ; révocations ignorées (S-29) ; flux non coupés à la révocation (S-28) ; `oh serve` DNS rebinding (S-36) ; `tmux` `#()` dans un nom de branche (S-37, à confirmer) ; intégrité npm d'opencode (S-38).
- **Fonctionnel** : trois modèles de statut « review » (F-02, + BL-25) ; claims libérés même si cp-2 reporté, sans expiration (F-03) ; bugs `oh service remove`, `MCPServer()` nil pour github/linear (F-14) ; `config model agent|family` sans contrôle, agent de `libre` non choisissable dans la TUI (F-15, avec la refonte des briques de l'ADR-051).
- **Ergonomie, i18n, architecture, docs** : U-01→U-12, A-01→A-09, D-02, D-03 (recoupent largement le premier audit).

## Répartition, propriété des fichiers, ordre

| Piste | Worktree / branche | Points | Propriétaire de |
|---|---|---|---|
| **S. Sécurité** | `../openhub-fix-sec` · `feat/v5-fix-sec` | lot 0, N1, N2, N3, N4, N5 | `internal/daemon` (routes, workflow, décisions), `internal/services/checkpoint/state.go` (liaison approbation), `internal/mcp/workflow`, `internal/adapters` (forme neutre de la réponse) + `opencodev2/events.go`, `internal/bundle/{globalguard,gitguard,hookguard,permissions}.go`, `InfraGuard`, `permissions/*.yaml`, frontmatter `permission:` de **debugger, infra, benchmarker, reviewer**, `internal/runtime/container`, `internal/gitutil` (`safe.go`) + migration des appels git, `gateway.ShellStartup`, `SECURITY*.md` |
| **W. Workflows** | `../openhub-fix-wf` · `feat/v5-fix-wf` | N6, N7, N8, N9, N10, N11 | `workflows/`, `agents/` (hors permissions des 4 agents de S), `skills/`, `bundle/templates/`, `bundle/checkpoints.go` (`lockPatterns`, dont `apply` fourni par S en motif), `internal/workflow` (schéma : `create`, `apply`, avertissements), `sessionspec/workflow.go`, `daemon/gateway.go` (garde `OpCreate` : **coordonner avec S**), lint de contenu et `capability_test.go`, golden |
| **R. Robustesse** | `../openhub-fix-rob` · `feat/v5-fix-rob` | N12, N13, N14, N15, N16, N17, N18 | `cmd/root.go`, `cmd/start_helpers.go` + appelants, `cmd/doctor*.go`, `cmd/repair.go`, `internal/storage/sqlite`, `internal/teamstate`, `internal/gc`, `internal/logrotate`, `cmd/gc.go`, `internal/worktree` (`IsMerged`, `IsClean`…), `internal/beads/clean.go`, `cmd/beads_clean.go`, `internal/i18n` (index inverse), tests de contrat (`t.Cleanup`), migration **v43** |
| **X. TUI** | `../openhub-fix-keys` · `feat/v5-fix-keys` | N19, N20 | `internal/tui/v2/shell/**`, `views/{worktree,launch_form,inline_wizard,workflows,home,home_layout,banner,project_mode,team_mode}_view*.go`, aide, `docs/guides/tui-usage.*` |
| **G. Passerelles et proxy** | `../openhub-fix-gw` · `feat/v5-fix-gw` | N22, N25 | `internal/gateway/**` (dont `fakebd`, `tickets.go`, `policy.go`, `paths.go`, `beads.go`, `githooks.go`), installation des crochets bd par oh, route du démon « jeton de hook » (**coordonner avec S** pour `daemon/handlers.go`), `services/checkpoint/locks.go` (garde de fermeture S-09 : **coordonner avec S/W**), `internal/credproxy/**`, écouteurs du proxy (`daemon/handlers.go:handleListen`, `runtime/container/runtime.go:182-211`) |
| **D. Distant** | `../openhub-fix-remote` · `feat/v5-fix-remote` | N21 | drapeau expérimental (config, `oh remote`, `oh run --runtime remote`, option ☁ de la fiche, docs) pour 5.0 ; puis `internal/services/remote/**`, `internal/remote/**`, `cmd/session_resolve.go` (5.0.x) |
| **Rel. Release** | `main` ou `feat/v5-rel-hygiene` | N27 | `go.mod`/`go.sum`, `.golangci.yml`, `.github/workflows/{ci,release}.yml`, `selfupdate`, `install.sh`, SECURITY (contact), CHANGELOG |

Ajouts aux pistes existantes : **S** reçoit N23 (intégrité du paquet : `bundle/build.go` Load/Build, droits, `render.go` `external_directory`) et N26 (mots de passe : `storage/sqlite/server_store.go`, démon, `AttachServer`) ; **W** reçoit N24 (`workflow/patch.go`, `validate_spec.go`, `services/workflow/resolve.go`) et le complément N11/DC26 (`quick`, `debug`, `onboarding`).

**Zones partagées et coordination** :
- `bundle/checkpoints.go` : W propriétaire ; S fournit la liste des motifs `apply` (fonction exportée par `InfraGuard`).
- `internal/workflow/schema.go` : W ajoute `create` **et** `apply` (un seul lot, avec l'avertissement d'opération inconnue DC9) ; S s'appuie dessus → **W lot W0 en premier** (petit).
- `daemon/gateway.go` (garde `OpCreate`) : W ; `daemon/handlers.go`, `workflow.go` : S.
- `internal/worktree` : R (correctifs `IsMerged`) ; S migre les appels vers `gitutil.Safe` → **R livre `IsMerged` d'abord, S rebase**.
- `views/worktree_view.go` : X (confirmation) ; R ne touche pas la vue (le GC est dans le démon).
- Fichiers à conflits : `locales/{fr,en}.json` (préfixes ci-dessous), `CHANGELOG.md` (`[Unreleased]`), golden des paquets (S et W les régénèrent : **fusionner W puis S, ou régénérer après rebase**).

**Préfixes i18n** : S `cmd.security.*`, `cmd.doctor.security.*`, `cmd.daemon.workflow.*` ; W `workflow.diag.unlock_*`, `workflow.diag.parallel_*`, `workflow.diag.verify_*` ; R `cmd.run.project.*`, `cmd.run.provider.*`, `cmd.init.level.*`, `cmd.doctor.db.*`, `cmd.repair.*`, `cmd.gc.*`, `cmd.beads.clean.*`, `cmd.doctor.orphans.*` ; X `tui.keys.*`, `tui.confirm.*`, `tui.compact.*`, `tui.wizard.quit.*`.

**Migrations** : **v43** = R (N16). Toute autre migration : prendre le numéro suivant **et** l'inscrire dans `00-INDEX` avant de coder.

**Ordre de fusion conseillé** : **F0** (`feat/v5-fix-legacy-skills`, déjà fait, revérifier `go test ./cmd` dans un vrai worktree) → **Rel** (petit, indépendant) → W0 (schéma) → S (lot 0, puis N1/N2/N26 en priorité absolue) → G → W → D (drapeau) → R → X. Rebase fréquent. R, X, Rel et D (drapeau) sont indépendants de S/W/G et peuvent fusionner dès qu'ils sont prêts.
**Coordination G ↔ S** : `daemon/handlers.go` (S : authentification des routes N1 ; G : route du jeton de hook, écouteurs) → S d'abord, G rebase. **G ↔ W** : `daemon/gateway.go` (W : garde `OpCreate` ; G : fermetures `duplicate/supersede`, analyseur) → même analyseur d'arguments (G le fournit, W l'utilise pour `create`). **G ↔ S/W** : `services/checkpoint/locks.go` (garde de fermeture S-09) → G propriétaire.
**Préfixes i18n supplémentaires** : G `cmd.gateway.*` (existant), `cmd.proxy.region.*` ; D `cmd.remote.experimental.*` ; S `cmd.bundle.integrity.*`.

## Recette de non-régression (après fusion des quatre pistes)
- Scénario `ticket` manuel (Haiku et Sonnet) sur projet Beads avec hooks : `curl` vers le socket → refusé, cp-2 non passé ; cp-2 validé dans l'interface de l'outil → passe ; commit après, ticket fermé après le commit.
- `feature` complète : cp-tickets, cp-0, review de fin par reviewer, un cp-2 par ticket.
- `sweep` en `auto` et en `dry_run` ; `review-feedback` semi-auto ; `cadrage` (`bd create` avant cp-tickets refusé).
- Conteneur (Colima + Podman) : écriture de `.git/config` impossible, `git commit`/`switch -c` OK ; dépôt piégé (`core.fsmonitor`) → rien ne s'exécute sur la machine.
- `oh doctor` avec `hub.toml` cassé, avec une base au schéma plus récent ; `oh repair --migrate`.
- Premier lancement après mise à jour : TUI + démon + serveurs MCP simultanés, aucune erreur de migration.
- `oh run` hors projet (avec et sans terminal), sans fournisseur.
- TUI en 80×24 et 140×42 : accueil, catalogue (Entrée lance), fiche (Ctrl+S → Récap), assistant (Échap → Quitter), worktrees (`d` → confirmation).
- `oh gc --dry-run` sur un `OH_HOME` réel (copie) ; rotation des logs.
- Passerelle Beads : PoC S-06 (`BD_GIT_HOOK=1 bd hooks run …`), S-07 (`update -sclosed`, `duplicate`, `supersede`), S-08 (`-qC/…`, `--DB=`) refusés ; garde de fermeture avec commit vide refusée.
- Proxy : `AWS_REGION=x.evil.com#` refusé ; OpenRouter `route`/`models` refusés.
- Superposition : enfant d'un parent sans `beads:` qui ajoute `close` → diagnostic ; checkpoint `unlocks:` en `skip` → diagnostic.
- Paquet de session modifié → détecté ; `oh.db` sans mot de passe de serveur.
- Distant : refusé sans `remote.experimental` ; `quick` et `debug` : commit refusé avant `cp-commit`.
- Release : `govulncheck` et lint propres avec la version de la CI ; tag `v5.0.0-rc.1` déclenche `release.yml`.
- Mesurer la durée d'une synchronisation du suivi (GitLab/Jira réel) et l'attente d'une écriture interactive pendant ce temps.
- Linux : conteneur avec Podman sans root (pasta) → message de contournement ; avec slirp4netns / Docker → session OK.

## Prompts

### Piste S
```
Dossier de travail : /Users/benjamin.datiche/workspace/openhub-fix-sec (toutes tes commandes et modifications se font dans ce dossier ; le code Go est dans openhub-fix-sec/cli).
Avant toute chose : `cp /Users/benjamin.datiche/workspace/openhub/opencode.json /Users/benjamin.datiche/workspace/openhub-fix-sec/opencode.json` (ignoré par git, ne pas commiter), puis `make embed-sync` dans cli/.
Plan : ~/.opencode/plan/openhub-v5/. Lis 00-INDEX.md, 01-decisions.md (D13, D19), 02-findings-opencode-v2.md, 03-architecture.md, SECURITY.fr.md, puis 17-critiques.md (§ Principes, § Décisions DC1–DC5, § Lot 0, § Piste S, § Répartition).
Piste S : lot 0 (sondes U1–U5, résultats dans 02 et dans les écarts), puis N1, N2 et N26 (priorité absolue), puis N3, N4, N5, N23 ; un lot à la fois avec un point de contrôle après chacun ; au point de contrôle du lot 0, propose l'ajustement des motifs de N3/N10 selon les sondes. Lis aussi docs/dev/audit-v5.md § 4.2–4.5 (S-10, S-12, S-19, S-24, S-33, S-35) : ses PoC servent de tests.
Revérifie chaque point avant de corriger ; un test de non-régression par point (unitaire avec le simulateur de rapprochement, contrat integration, e2e Haiku pour N1/N2/N3, real_test par moteur pour N5). Environnement de recette : ~/workspace/openhub-test (env.sh).
Propriétaire : voir § Répartition (piste S). Attends la fusion de W0 (schéma create/apply) avant N4 (verrou apply). Règle D19. CHANGELOG : [Unreleased] seulement.
Écarts : 17-critiques.md § « Écarts constatés » → « Piste S ».
Fin de lot : go build, go vet (tags integration e2e container), go test ./..., go test -race -tags integration ./..., make lint à 0, i18n fr/en, doc bilingue (SECURITY) à jour, commit « fix(<zone>): … (v5 critical, Nn) ».
```

### Piste W
```
Dossier de travail : /Users/benjamin.datiche/workspace/openhub-fix-wf (le code Go est dans openhub-fix-wf/cli).
Avant toute chose : copie d'opencode.json (comme les autres pistes), puis `make embed-sync` dans cli/.
Plan : ~/.opencode/plan/openhub-v5/. Lis 00-INDEX.md, 01-decisions.md, 03-architecture.md, docs/reference/workflow-schema.fr.md, 16-corrections.md (§ Piste T, écarts T1–T5), puis 17-critiques.md (§ Principes, § Décisions DC6–DC12, § Piste W, § Répartition).
Piste W : W0 (schéma : opérations create et apply, avertissement d'opération inconnue — petit lot à fusionner en premier), puis N24 (superposition, PoC S-16/S-17 de docs/dev/audit-v5.md § 4.3), N6, N7, N8, N9, N10, N11 (corrections de la matrice + complément DC26 quick/debug/onboarding ; le lint capability peut être un lot final séparé) ; point de contrôle après chaque lot.
Tests : golden des paquets, lints de contenu, e2e Haiku et Sonnet pour feature/ticket/sweep/review-feedback/cadrage.
Propriétaire : voir § Répartition (piste W) ; ne touche pas aux permissions de debugger/infra/benchmarker/reviewer (piste S). Règle D19. CHANGELOG : [Unreleased].
Écarts : 17-critiques.md § « Écarts constatés » → « Piste W ».
Fin de lot : build, vet (tags), go test ./..., -race -tags integration, make lint à 0, i18n, doc bilingue (workflows, schéma), commit « fix(<zone>): … (v5 critical, Nn) ».
```

### Piste R
```
Dossier de travail : /Users/benjamin.datiche/workspace/openhub-fix-rob (le code Go est dans openhub-fix-rob/cli).
Avant toute chose : copie d'opencode.json, puis `make embed-sync` dans cli/.
Plan : ~/.opencode/plan/openhub-v5/. Lis 00-INDEX.md, 01-decisions.md, 03-architecture.md, 06-phase-2.md (team-state, Transact), 16-corrections.md (§ Piste C, écarts C1–C5), puis 17-critiques.md (§ Principes, § Décisions DC13–DC16, § Piste R, § Répartition).
Piste R : N14 (migrations), puis N13 (niveaux de démarrage), N12, N17, N15 étape A, la rotation des logs de N16, N18 ; puis (avant REL, DC21) N16 GC complet (migration v43, à inscrire dans 00) et N15 étape B ; point de contrôle après chaque lot.
Livre en premier le correctif `worktree.IsMerged` (merge-base --is-ancestor) : la piste S en dépend.
Tests : voir chaque point (multi-processus pour N14/N15, vrais dépôts git pour N16).
Propriétaire : voir § Répartition (piste R). Règle D19. CHANGELOG : [Unreleased].
Écarts : 17-critiques.md § « Écarts constatés » → « Piste R ».
Fin de lot : build, vet (tags), go test ./..., -race -tags integration, make lint à 0, i18n, doc bilingue (troubleshooting, config, cli-infra), commit « fix(<zone>): … (v5 critical, Nn) ».
```

### Piste G
```
Dossier de travail : /Users/benjamin.datiche/workspace/openhub-fix-gw (le code Go est dans openhub-fix-gw/cli).
Avant toute chose : copie d'opencode.json, puis `make embed-sync` dans cli/.
Plan : ~/.opencode/plan/openhub-v5/. Lis 00-INDEX.md, 01-decisions.md, 03-architecture.md, 08-phase-4.md (passerelles), 16-corrections.md (§ Piste T, écarts T1–T2 : crochets bd), docs/dev/audit-v5.md (§4.2, §4.4 : S-06→S-09, S-14, S-21→S-25), puis 17-critiques.md (§ Apport de l'audit parallèle, § DC24, § Piste G, § Répartition).
Piste G : N22 (S-07 et S-08 d'abord : analyseur d'arguments de bd ; puis S-06 jeton de hook, S-09 garde de fermeture, S-14), puis N25 (S-21, S-22, S-23, puis S-25) ; point de contrôle après chaque lot ; les PoC de l'audit-v5 deviennent les tests de non-régression (contrat integration avec le vrai bd 1.3.1).
Propriétaire : voir § Répartition (piste G) ; rebase après S pour daemon/handlers.go. Règle D19. CHANGELOG : [Unreleased].
Écarts : 17-critiques.md § « Écarts constatés » → « Piste G ».
Fin de lot : build, vet (tags), go test ./..., -race -tags integration, make lint à 0, i18n, doc bilingue (SECURITY, guide Beads), commit « fix(<zone>): … (v5 critical, Nn) ».
```

### Piste D
```
Dossier de travail : /Users/benjamin.datiche/workspace/openhub-fix-remote (le code Go est dans openhub-fix-remote/cli).
Avant toute chose : copie d'opencode.json, puis `make embed-sync` dans cli/.
Plan : ~/.opencode/plan/openhub-v5/. Lis 00-INDEX.md, 09-phase-5.md, docs/dev/audit-v5.md (§4.1 : S-01→S-05), puis 17-critiques.md (§ DC22, § Piste D).
Piste D : lot D1 (5.0) = drapeau expérimental du mode distant partout (config, CLI, fiche, Doctor, docs) ; lot D2 (avant REL, DC21) = S-01, S-02, S-04, S-05 (S-03 est traité par la piste S, N23), puis recette réelle de 09 dès qu'un runner est disponible (le drapeau reste levé seulement après elle) ; point de contrôle après chaque lot.
Propriétaire : voir § Répartition (piste D). Réutilise gitutil.Safe (piste S) et l'analyseur bd (piste G) : rebase après elles pour D2. Règle D19. CHANGELOG : [Unreleased].
Écarts : 17-critiques.md § « Écarts constatés » → « Piste D ».
Fin de lot : build, vet (tags), go test ./..., -race -tags integration, make lint à 0, i18n, doc bilingue (remote-runners, SECURITY), commit « fix(remote): … (v5 critical, N21) ».
```

### Piste Rel
```
Dossier de travail : /Users/benjamin.datiche/workspace/openhub (branche courte feat/v5-rel-hygiene).
Plan : 17-critiques.md § Piste Rel, docs/dev/audit-v5.md § 2 (santé du code), § 4.5 (S-39, S-41, S-42).
Lot unique : x/text ≥ 0.39 + govulncheck propre, PR Dependabot revues, golangci-lint fixé à 2.14 en CI + 11 constats corrigés, release.yml sur push de tag (essai v5.0.0-rc.1), contact sécurité, décision Cosign (vérifier, ou doc « non vérifié »).
Fin : build, vet, go test ./..., make lint à 0 avec la version de la CI, commit « chore(release): … (v5 critical, N27) ».
```

### Piste X
```
Dossier de travail : /Users/benjamin.datiche/workspace/openhub-fix-keys (le code Go est dans openhub-fix-keys/cli).
Avant toute chose : copie d'opencode.json, puis `make embed-sync` dans cli/.
Plan : ~/.opencode/plan/openhub-v5/. Lis 00-INDEX.md, 10-tui.md, docs/design/tui-shell.md, docs/guides/tui-usage.fr.md, 16-corrections.md (§ Piste U, écarts), puis 17-critiques.md (§ Décisions DC17–DC20, § Piste X, § Répartition).
Piste X : N19 (contrat des touches, confirmations, fiche, assistants), puis N20 (mode compact) ; point de contrôle après chaque lot. Hors périmètre : la refonte visuelle globale (fichier 18 à venir).
Tests : SimulationScreen (keys_test, fiche, assistant, worktrees, 80×24 / 50×12) ; capture réelle avec ~/workspace/openhub-test/bin/oh-tui-drive.
Propriétaire : voir § Répartition (piste X). CHANGELOG : [Unreleased] (changement de comportement des touches à signaler).
Écarts : 17-critiques.md § « Écarts constatés » → « Piste X ».
Fin de lot : build, vet (tags), go test ./..., -race, make lint à 0, i18n, doc bilingue (tui-usage, référence TUI), commit « fix(tui): … (v5 critical, Nn) ».
```

## Mise en place
```bash
cd /Users/benjamin.datiche/workspace/openhub
git fetch github && git merge --no-ff github/feat/v5-fix-legacy-skills   # F0, après revérification (go test ./... dans un vrai worktree)
git push origin main
for p in sec wf rob keys gw remote; do git worktree add ../openhub-fix-$p -b feat/v5-fix-$p main; cp opencode.json ../openhub-fix-$p/; (cd ../openhub-fix-$p/cli && make embed-sync); done
```

## Écarts constatés

### Piste S

### Piste W

### Piste R

### Piste X

### Piste G

### Piste D

### Piste Rel