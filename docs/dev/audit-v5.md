# Audit de la v5 et plan de correction

> **Date** : 2026-10-08 (vérifications dynamiques le 2026-10-09)
> **Périmètre** : évolution v5 (`e1455c36..c3b36496`, ~213 commits, 1 352 fichiers, +132k/−42k lignes) et état global du projet après la v5.
> **Axes** : sécurité, design, ergonomie, fonctionnel, architecture et qualité, opportunités.
> **Statut** : plan proposé. Le lot **F0** (registre de skills communautaires) est fait (ADR-051, branche `feat/v5-fix-legacy-skills`).

---

## 1. Méthode

1. **Lecture statique** en 8 analyses parallèles : proxy d'identifiants, démon et stockage ; monde fermé et permissions ; passerelles Beads/MCP ; conteneur, distant, MCP natifs, `oh serve`, `termlaunch`, mise à jour, CI ; complétude V5 (ADR 038 à 050 comparés au code) ; fonctionnel (workflows, agents, skills) ; design et ergonomie (CLI, TUI) ; architecture et qualité du code.
2. **Vérification manuelle** des constats les plus graves dans le code.
3. **Vérifications dynamiques** (Go 1.27.2, golangci-lint 2.14, govulncheck, Beads `bd` 1.3.1, opencode 2.0.20) :
   - build, `go vet`, `go test ./... -race`, `go test -race -tags integration` ;
   - lint (configuration du dépôt) et gosec ;
   - tests de preuve (PoC) écrits dans un worktree jetable, puis supprimés ;
   - scénarios réels avec le binaire `oh` dans un `OH_HOME` isolé, et avec le vrai `bd`.
4. **Non exécuté** : tests `e2e` (appel d'un vrai modèle Bedrock, coût) et `container` (Docker non démarré).

Niveau de confiance des constats :

| Marque | Sens |
|--------|------|
| **Prouvé** | Reproduit par un test ou une commande réelle |
| **Code** | Lu et confirmé dans le code, non exécuté |
| **À confirmer** | Dépend du comportement d'un outil externe (GitHub, tmux, bd…) |

---

## 2. Verdict

- **Le plan v5 est bien réalisé** : les 13 ADR (038 à 050) sont implémentés (modèle de session neutre et adaptateur, workflows `oh/v1` superposés par couches, paquet de session immuable, monde fermé avec `Attest`, démon `ohd` et proxy d'identifiants, passerelles, checkpoints, conteneur et distant). Pas d'oubli majeur, docs fr/en à parité, migrations sans trou, aucun TODO réel.
- **Mais la v5 a ouvert une grande surface de sécurité**, et plusieurs promesses de `SECURITY.md` sont plus fortes que ce que fait le code :
  - le **mode distant** a 3 failles graves et n'a jamais été validé sur un vrai runner ;
  - les **verrous locaux** (commit, fermeture de ticket, hooks, `beads.allow`) sont présentés comme absolus mais se contournent.
- **Le reste est de la dette** : surface CLI trop large, doublons, contenu V1 dans les skills, couche `cmd/` trop grosse.
- **Pas encore de release v5** : pas de tag, CHANGELOG entièrement sous `[Unreleased]`. C'est le moment de traiter le lot P0.

### Santé du code (2026-10-09)

| Contrôle | Résultat |
|----------|----------|
| `go build`, `go vet` | ✅ (après `make embed-sync`) |
| `go test ./... -race` | ✅ 66 packages |
| `go test -race -tags integration` | ✅ (opencode 2.0.20) |
| golangci-lint 2.14 | ⚠️ 11 constats (8 SA1019 sur `TrackerConfig`, 3 gocritic), alors que la v5 annonce « zéro constat » : la CI utilise une version plus ancienne |
| govulncheck | ⚠️ GO-2026-5970 (`golang.org/x/text` 0.38, atteint via `team_mcp_view.go:290`), corrigé en 0.39 |

---

## 3. Conformité aux ADR v5

| ADR | Statut | Écart |
|-----|--------|-------|
| 038 SessionSpec et adaptateurs | ✅ | — |
| 039 Workflows oh/v1 | ✅ | Skills qui renvoient aux « modes A–E » (`planner-execution-modes.md:612`, `planner-phase-2.md:484`) |
| 040 Gouvernance team-state | ✅ | Colonnes `workflow_config_legacy` conservées sans date de fin |
| 041 Monde fermé | ✅ / partiel | Plugins globaux seulement signalés (voir S-11) |
| 042 Checkpoints | ✅ | — |
| 043 Paquet de session | ✅ | Empreinte jamais revérifiée (voir S-08) |
| 044 Proxy et restrictions | ✅ | Test e2e des budgets non fait (reconnu par l'ADR) |
| 045 Environnements d'exécution | ⚠️ | Distant livré sans validation réelle, sans avertissement dans la CLI |
| 046 Passerelles Beads | ⚠️ | Contournements (S-04 à S-07) |
| 047 Démon | ✅ | — |
| 048 Abandon d'opencode V1 | ✅ code / ❌ contenu | Agents et skills parlent encore d'`opencode.json`, `.opencode/context.json`, `scripts/lib/*.sh` |
| 049 Indépendance de l'outil | ✅ / partiel | `archtest` ne vérifie pas le contenu embarqué (agents, skills) |
| 050 Contexte de session | ✅ | — |

---

## 4. Constats

Sévérités : 🔴 élevée · 🟠 moyenne · 🟡 faible · ⚪ info.

### 4.1 Sécurité — mode distant (GitLab CI)

| ID | Sév. | Constat | Preuve | Où |
|----|------|---------|--------|----|
| S-01 | 🔴 | `Finish` lance `git status/add/commit/push` **en root** dans un dossier appartenant au compte de l'agent (uid 10001), avec `safe.directory=*` et le jeton dans `GIT_CONFIG_VALUE_0`. `--no-verify` ne bloque ni `post-commit`, ni `pre-push`, ni `core.fsmonitor` : un hook déposé par l'agent (par exemple via un test) s'exécute en root et peut lire tous les secrets CI d'`oh-runner` (`/proc/1/environ`) | **Prouvé** (hooks et fsmonitor exécutés, jeton visible) | `services/remote/runner/git.go:22-33,83-113`, `runner/runtime.go:109-123` |
| S-02 | 🔴 | La construction de l'image (Kaniko ou DinD) exécute les `RUN` du Dockerfile d'un projet cible dans un job qui détient les secrets de **tous** les projets cibles ; en DinD, le jeton est dans l'URL de clonage (contexte de build, image poussée) ; étiquettes d'image calculées sur le seul Dockerfile | Code | `remote/ciconfig/ciconfig.go:199-228`, `services/remote/image.go:74-81` |
| S-03 | 🔴 | Paquet de session retéléchargé (`ensureBundle`) jamais comparé à son empreinte ; `bundle.Load` ne la recalcule pas. Un paquet remplacé dans le registre de packages s'exécute sur la machine à la reprise | Code | `services/remote/fetch.go:186-212`, `bundle/build.go:296-307`, `runner/fetch.go:96-110` |
| S-04 | 🟠 | Rejeu du journal Beads : un identifiant dont le préfixe contient un tiret (`my-app-x9`) n'est pas reconnu comme ticket et passe ; valeurs d'options (`--parent`, `--deps`) non contrôlées ; journal non authentifié | **Prouvé** (`close my-app-zz9`, `update … --status closed`, `--parent` acceptés) | `services/remote/replay.go:136,321-327` |
| S-05 | 🟠 | `oh session resolve` affiche le journal brut avant confirmation : des séquences ANSI/OSC peuvent masquer des lignes | Code | `cmd/session_resolve.go:47,62` |

### 4.2 Sécurité — passerelles et verrous locaux

| ID | Sév. | Constat | Preuve | Où |
|----|------|---------|--------|----|
| S-06 | 🔴 | **Chemin « hook git » piloté par l'appelant** : la passerelle fait confiance à `BD_GIT_HOOK=1` (champ `git_hook` de la requête), fixé par le client. Avec `allow: []`, `hooks run post-merge` passe (avec des arguments en plus). Combiné à l'écriture de `.beads/*.jsonl`, permet de modifier ou fermer des tickets sans contrôle | **Prouvé** | `gateway/fakebd/run.go:46`, `gateway/beads.go:90-93,147-149`, `gateway/githooks.go` |
| S-07 | 🔴 | **Verrou de fermeture contourné** : `update -sclosed` (forme collée de pflag), `duplicate` et `supersede` ferment un ticket sans être reconnus | **Prouvé** (passerelle 200 ; le vrai `bd` 1.3.1 ferme bien avec `-sclosed`) | `gateway/tickets.go:51-62` |
| S-08 | 🔴 | **Options refusées sous forme collée** : `-qC/autre` (le vrai `bd` liste les tickets d'un autre projet), `--DB=/x`, `-o/etc/x`, `-f/…/.ssh/id_rsa` passent alors que les formes séparées sont refusées. Redirection possible de la base via `.beads/redirect` ou `config.yaml` (écrits par l'agent) | **Prouvé** (sauf `.beads/redirect` : à confirmer) | `gateway/policy.go:49-59`, `gateway/paths.go:18-31,102-126` |
| S-09 | 🟠 | « Fermé seulement si commité » : n'importe quel commit suffit (même `--allow-empty`) ; erreurs git ou de garde traitées comme un succès | Code | `services/checkpoint/locks.go:51-54,101-104`, `daemon/gateway.go:65-67` |
| S-10 | 🟠 | **Paquet de session modifiable** : dossiers en 0755, `external_directory` autorisé sur `SkillsDir/*` ; une modification vaut pour toutes les sessions de même empreinte | Code | `adapters/opencodev2/render.go:174-176`, `bundle/build.go:276-278` |
| S-11 | 🟠 | En local non strict, les plugins de la configuration opencode de l'utilisateur s'exécutent dans le serveur ; `Attest` ne fait qu'avertir (et rien si `/api/plugin` échoue) | Code | `adapters/opencodev2/attest.go:84-98` |
| S-12 | 🟠 | `SECURITY.md:108-109` affirme que les hooks ne sont « jamais contournés » et que commit/push sont « refusés à tous les agents » : faux. `git revert` et `git pull` (autorisés) créent des commits ; tout lanceur autorisé (`make`, `npm run`, `.venv/bin/*`, `env *`, `ctx_execute`) exécute un script hors règles ; `rm -rf .beads/hooks`, `CORE.HOOKSPATH`, `--no-veri`, `commit -qn` passent | Code | `bundle/hookguard.go:14-18`, `bundle/checkpoints.go:89-93`, `permissions/developer-rw.yaml` |
| S-13 | 🟠 | Passerelle MCP : laisse passer quand `_meta` est absent, inconnu ou en erreur ; sessions d'un même groupe non séparées ; corps lu sans limite (`io.ReadAll`) | Code | `daemon/gateway.go:202-220`, `gateway/mcp.go:70` |
| S-14 | 🟡 | `bd` réel lancé avec tout l'environnement du démon ; jetons de sous-agents valides jusqu'à la fin de la session racine ; `gateway.json` relu sans contrôle au redémarrage | Code | `gateway/beads.go:145-149`, `gateway/store.go` |
| S-15 | 🟡 | Démarrage du shell : `BASH_ENV` ignoré par bash interactif et `sh` ; fish non couvert ; contrôle unique par démon, depuis `$HOME` | Code | `gateway/shellenv.go`, `daemon/gateway.go:241-267` |

### 4.3 Sécurité — superposition des workflows (« ne peut que durcir »)

| ID | Sév. | Constat | Preuve | Où |
|----|------|---------|--------|----|
| S-16 | 🟠 | Parent sans bloc `beads:` (la passerelle applique la lecture seule) : l'enfant peut autoriser `create, close, delete` sans diagnostic | **Prouvé** | `workflow/patch.go:398-402` vs `gateway/policy.go:11` |
| S-17 | 🟠 | Checkpoint non obligatoire portant `unlocks:` passable en `skip` dans tous les modes, sans diagnostic : `LockedOps` l'ignore, le verrou disparaît | **Prouvé** | `workflow/patch.go:314-327`, `bundle/checkpoints.go:98-116` |
| S-18 | 🟠 | `mcp` et `plugins` remplacés sans contrôle de durcissement (un plugin exécute du code) | Code | `workflow/patch.go:125-130` |
| S-19 | 🟠 | Workflows `risk: read` (`review`, `audit`) qui gardent les serveurs MCP du projet en écriture (`gitlab_create_mr`…) ; outils `ctx_*` qui contournent `bash: deny` si le plugin context-mode est ajouté | Code | `workflow/validate_spec.go:712-739`, `services/workflow/resolve.go:95-106`, `permissions/*.yaml` |
| S-20 | 🟡 | Une couche peut vider `after:`, élargir `calls`, ajouter des agents, relever `circuit_breaker`, ajouter `skills.extra` | Code | `workflow/patch.go:107-124,250-266` |

### 4.4 Sécurité — proxy d'identifiants, démon, stockage

| ID | Sév. | Constat | Preuve | Où |
|----|------|---------|--------|----|
| S-21 | 🟠 | Région Bedrock non validée : `AWS_REGION=x.evil.com#` envoie la clé vers `bedrock-runtime.x.evil.com` | **Prouvé** | `credproxy/providers.go:14-20` |
| S-22 | 🟠 | Liste des modèles contournable chez OpenRouter (`models`, `route`, `preset`) ; données après le premier objet JSON non contrôlées | Code | `credproxy/routes.go:114-149` |
| S-23 | 🟠 | `allowed_models` corrompu en base : erreur ignorée, liste vide = tout modèle autorisé | Code | `storage/sqlite/server_store.go:161`, `credproxy/proxy.go:63` |
| S-24 | 🟠 | Mots de passe des serveurs opencode en clair dans `oh.db` : le shell d'un agent peut piloter les serveurs d'autres projets ; contredit `SECURITY.md:63` | Code | `storage/sqlite/server_store.go:22,51` |
| S-25 | 🟠 | Second écouteur du proxy sous Linux : toute IP privée acceptée (réseau local, VPN), adresse fournie par l'image du projet ; hôtes restaurés sans contrôle depuis `ohd.json` | Code | `runtime/container/runtime.go:182-211`, `daemon/handlers.go:207-209`, `daemon/daemon.go:356-359` |
| S-26 | 🟡 | Routes du socket sans capacité : `/groups/{g}/policy` (arrêt), battements de cœur qui maintiennent un groupe éveillé, `/workflow/checkpoint` et `/outputs` | Code | `daemon/handlers.go:24-37` |
| S-27 | 🟡 | Budgets sous-comptés (jetons de cache, flux interrompus, requêtes concurrentes) | Code | `credproxy/usage.go:22-23`, `credproxy/proxy.go:500-508` |
| S-28 | 🟡 | Client qui fait confiance au socket (secret envoyé à un squatteur si le démon est arrêté) ; flux en cours non coupés à la révocation ; en-têtes et query relayés ; socket créé avant `chmod` ; liste d'environnement filtrée en refus (manque `GEMINI_API_KEY`, `GH_TOKEN`…) | Code | `daemon/client.go`, `credproxy/proxy.go`, `adapters/opencodev2/server.go:62-92` |
| S-29 | 🟡 | Erreurs de révocation ignorées (`_ = …RevokeOwner`) : un jeton révoqué en mémoire peut rester actif en base | Code | `daemon/daemon.go:403,617`, `runsvc/service.go` |

### 4.5 Sécurité — conteneur, MCP natifs, divers

| ID | Sév. | Constat | Preuve | Où |
|----|------|---------|--------|----|
| S-30 | 🔴 | **Registre de skills communautaires** : écriture hors de `~/.oh` (« zip-slip »), `oh skill remove ..` efface tout `~/.oh`, `skill_file` non contrôlé, ni empreinte ni signature, http accepté | **Prouvé** | ✅ **Traité** par le lot F0 (ADR-051) |
| S-31 | 🟠 | URL MCP imposée par l'équipe : un membre du team-state peut rediriger le jeton Jira de tous les membres (http et hôtes privés acceptés) | Code | `mcpresolve/resolve.go:146-159`, `mcp/jira/server.go:184-206` |
| S-32 | 🟠 | Chemins d'API MCP non échappés (Jira, GitHub), y compris pour les outils d'écriture ; aucun test sur ces deux serveurs | Code | `mcp/jira/server.go:145,159,177,322`, `mcp/github/server.go:161-324` |
| S-33 | 🟠 | Conteneur : pas de `--cap-drop` ni `no-new-privileges` ; HOME partagé en 1777 par toutes les sessions du projet | Code | `runtime/container/spec.go:140,180-227`, `image.go:270-274` |
| S-34 | 🟠 | Délimiteurs de prompt fixes, neutralisation limitée à l'orthographe exacte ; branches de la MR insérées brutes | Code | `workflow/render_prompt.go:20-25,83-111`, `workflows/prompts/review-feedback.md.tmpl:1,8` |
| S-35 | 🟡 | Fichier env du conteneur laissé sur disque avec les jetons ; repli `os.TempDir()/oh-container` | Code | `runtime/container/runtime.go:49,237-247` |
| S-36 | 🟡 | `oh serve` sans authentification ni contrôle de l'en-tête Host (lisible par DNS rebinding, lecture seule) | Code | `cmd/serve.go:41,47-134` |
| S-37 | 🟡 | `tmux -c` / `-n` avec `#()` dans un nom de branche | À confirmer | `termlaunch/launch.go:169-177`, `worktree.go:31-39` |
| S-38 | 🟡 | Intégrité npm d'opencode tirée de la même réponse que le paquet ; binaire en cache jamais revérifié | Code | `adapters/opencodev2/linux_binary.go:91-126` |
| S-39 | 🟡 | Mise à jour : signature Cosign **pas du tout** vérifiée (`SECURITY.md` dit « indicative ») ; redirections non contrôlées ; rétrogradation possible ; `install.sh` saute la vérification sans échouer | Code | `selfupdate/selfupdate.go:216-235,376-396`, `install.sh:143-163` |
| S-40 | 🟡 | Migrations SQLite non verrouillées entre processus (CLI, TUI, démon) | Code | `storage/sqlite/store.go` |
| S-41 | ⚪ | `release.yml` : `branches: [main]` + `head_branch` commençant par `v` se contredisent, la publication ne se déclencherait jamais sur un tag | À confirmer | `.github/workflows/release.yml:3-29` |
| S-42 | ⚪ | `SECURITY.md` : adresse `security@example.com` | — | `SECURITY*.md:23` |

**Bonnes pratiques à garder** : empreintes SHA-256 seules pour les jetons ; identifiants appliqués dans le transport (refus si impossible) ; chemins du proxy décodés une fois ; URL des fournisseurs figées ; `crypto/rand` partout ; capacité comparée à temps constant ; UID du pair vérifié ; `oh.db` en 0600 ; durcissement des couches correct pour la plupart des champs, avec tests ; `Attest` qui arrête le serveur ; `ScrubEnv` et politique de réponse sans `--auto` à distance.

### 4.6 Fonctionnel

| ID | Sév. | Constat | Où |
|----|------|---------|----|
| F-01 | 🟠 | Nettoyage « zéro impact » de Beads sans confirmation ni désactivation : défait le commit `bd init` d'après des refs distantes possiblement périmées (`update-ref -d HEAD` sur un premier commit), supprime les hooks `bd` de `.claude/settings.json` non suivis, déplace `.beads/` vers `.git/info/exclude` (casse le partage par git) | `beads/clean.go:169-198,292-301,398-460`, `cmd/beads_clean.go` |
| F-02 | 🟠 | Trois modèles de statut incohérents : label `ready-for-review` (Beads), statut `review` (claims), `--status review` rejoué à distance (refusé par bd 1.3) ; colonne REVIEW de `oh board` toujours vide | `teamstate/claims.go:29`, `services/remote/return_test.go:91`, `cmd/board.go:113-131` |
| F-03 | 🟠 | Claims libérés en fin de session même si `cp-2` est reporté ; pas d'expiration après crash ; libération dans une goroutine qui meurt avec la CLI | `cmd/v5_claims.go:57-62,118`, `teamstate/claims.go:724` |
| F-04 | 🟠 | Budgets contrôlés seulement en fin d'étape racine (une étape peut lancer 12 à 20 délégations) ; liste des modèles jamais vérifiée au lancement | `daemon/watch.go:273,540-542`, `limits` |
| F-05 | 🟠 | Workflows qui écrivent sans verrou de commit : `quick`, `sweep`, `debug`, `libre`, `onboarding` ; `sweep` lance des développeurs en parallèle dans le même arbre | `workflows/*.yaml`, `prompts/sweep.md.tmpl` |
| F-06 | 🟡 | `unlocks: [push]` dans `feature`, `ticket`, `review-feedback` alors qu'aucun agent n'a le droit de pousser ; niveau de risque `publish` jamais utilisé | `workflows/*.yaml`, `permissions/developer-rw.yaml:17` |
| F-07 | 🟡 | `review-feedback` ne relit pas ses corrections ; entrées `mr.*` GitLab seulement | `workflows/review-feedback.yaml`, `cmd/v5_inputs_gitlab.go` |
| F-08 | 🟡 | Doublons : `conductor` / `orchestrator` / `orchestrator-dev` ; `cadrage` reprend la moitié de `feature` ; checkpoints copiés-collés ; même liste `beads.allow` dans 4 workflows | `agents/planning/*`, `workflows/*.yaml` |
| F-09 | 🟡 | 4 agents utilisés par aucun workflow (`benchmarker`, `test-generator`, `database`, `infra`), avec les permissions les plus larges (`env *`, `kubectl*`, `aws *`) | `agents/quality/*`, `agents/developer/*` |
| F-10 | 🟡 | Contenu d'avant la v5 livré : `opencode.json`, `.opencode/context.json`, `scripts/lib/*.sh`, `orchestrator-modes.md` (Cas E), mode parallèle ; copies statiques de skills générées ; `orchestrator-dev` garde `git worktree add/remove` | `skills/orchestrator/*`, `skills/shared/websearch-usage.md`, `agents/planning/pathfinder.md:1268`, `agents/planning/orchestrator-dev.md` |
| F-11 | 🟡 | Vues de suivi qui se recoupent : `status`, `metrics`, `dashboard` (en partie factice), `serve`, `board`, `team board`, `history` | `cmd/dashboard_cmd.go:41-73` |
| F-12 | 🟡 | Commandes CLI qui figent les valeurs déjà déclarées dans le YAML (`oh audit`, `oh review`) ; quatre implémentations de jokers | `cmd/audit_review_debug.go:51-84` |
| F-13 | 🟡 | Manques : nettoyage des paquets/sessions/worktrees (seul `purge`), `--dry-run`, annulation d'une session, coût par projet, GitHub à moitié branché, `oh import --merge` documenté mais inactif | `cmd/purge.go`, `cmd/worktree.go:228`, `cmd/export.go:50` |
| F-14 | 🟡 | Bugs probables : `oh service remove` supprime l'ancien nom de clé du trousseau ; `MCPServer()` renvoie `nil` pour github et linear ; clés de trousseau propres à un projet jamais migrées | `cmd/misc.go:383`, `config/config.go:130-142`, `cmd/mcp.go` |
| F-15 | 🟡 | `oh config model agent|family` accepte n'importe quel identifiant (CLI et vue Modèles) ; la TUI ne permet pas de choisir l'agent de `libre` (`selectable: true`) | `cmd/config_model.go`, `views/models_view.go:277`, `cmd/tui_session_actions.go` |
| F-16 | ✅ | Registre de skills communautaires non fonctionnel (index 404, installation GitHub sans effet), skills communautaires absentes de la TUI, `oh skill budget` en doublon | **Traité** par le lot F0 (ADR-051) |

### 4.7 Design et ergonomie

| ID | Constat | Où |
|----|---------|----|
| U-01 | 41 commandes de premier niveau, 18 sous-commandes `session` ; `oh --help` fait 477 lignes | `cmd/help.go:54-66` |
| U-02 | Alias dépréciés (`start`, `audit`, `review`, `debug`) sans champ `Deprecated` de cobra : affichés comme des commandes normales ; `review --publish` et `review feedback` vivent sous une commande dépréciée | `cmd/help.go:132-134`, `cmd/audit_review_debug.go` |
| U-03 | Options courtes incohérentes : `-p` projet/port, `-m` prompt/mode/message, `-i` input/issue, `-t` type/ticket ; deux options pour ouvrir une fenêtre (`--attach`, `--how`, `here` non documenté) | `cmd/serve.go:35`, `run.go:44`, `session.go:62,264` |
| U-04 | Noms mi-français mi-anglais : modes `manuel`/`semi-auto`/`auto`, workflows `libre`, `cadrage` ; « session libre » a trois noms (`coder`, `libre`, Free session) | `workflow/schema.go:110`, `cmd/tui_commands.go:27` |
| U-05 | Codes de sortie : en mode sans interface « en attente » et « échec » valent tous deux 1 ; `oh deploy`/`sync` renvoient 0 ; `oh doctor` renvoie 1 si Docker (optionnel) est arrêté | `cmd/v5_headless.go:73`, `cmd/deploy_removed.go`, `cmd/doctor.go` |
| U-06 | Deux modèles de clavier dans la TUI (ancien `layout.Build` pour `board`/`dashboard`/board d'équipe, shell avec omnibar) ; lettres surchargées (`x` a 5 sens) ; `g`/`d` « globales » redéfinies par les vues ; `i` interrompt sans confirmation | `layout/layout.go`, `views/sessions_view.go:541,592`, `shell/shell.go:1363-1391` |
| U-07 | `?` n'affiche pas les touches de Sessions, Démarrer, fiche de lancement et catalogue ; ligne d'aide de Sessions coupée (19 entrées) et fausse (« n reject ») ; `$` absent de la doc | `shell/help.go:42,49-67`, `en.json:3143` |
| U-08 | `Ctrl+B` (retour) pris par tmux ; `Ctrl+S` depuis Entrées saute le récapitulatif (choix du dossier modifié jamais proposé) ; `[b]` proche de `Ctrl+B` | `views/launch_form.go:164-183,425-427` |
| U-09 | Symboles ambigus (`✔` terminé et arrêté, `⏸` attente et checkpoint, `✗` trois sens) ; pas de `NO_COLOR` ; descriptions de `oh init` masquées sous 30 lignes | `services/session/format.go:16-50`, `cmd/init_steps.go:241` |
| U-10 | `oh init` : ~8 écrans pour un solo, dont la moitié d'introduction, MCP montré alors qu'optionnel ; index de groupes d'étapes codés en dur | `cmd/init_wizard.go:118-191` |
| U-11 | i18n : le garde-fou A12 ne couvre que 4 fichiers ; chaînes en dur dans `export.go`, `policies.go`, `mcp.go`, `team_detail_view.go`, `picker.go`, `dashboard_cmd.go`, `serve.go`, `metrics.go` ; confirmation de `oh init` qui n'accepte que `y` ; aucun test des clés orphelines | `cmd/messages_literal_test.go:19` |
| U-12 | Mot « mode » à 4 sens (agent, workflow, navigation, configuration) ; un workflow en `auto` s'arrête quand même à certains checkpoints | `docs/reference/glossary.*` |

### 4.8 Architecture et qualité

| ID | Constat | Où |
|----|---------|----|
| A-01 | `cmd/` ~220 fichiers ; `tui_team_actions.go` 2 419 lignes, fonctions de 500 à 600 lignes ; 38 fichiers importent `teamstate` directement (pas de service équipe/tracker) ; synchronisation du tracker dupliquée CLI/TUI, déjà divergente | `cmd/tui_team_actions.go`, `cmd/team_sync_tracker.go` |
| A-02 | `safego` absent des packages critiques (démon, runsvc, proxy, passerelle) : sous Windows le démon tourne dans `oh`, une panique tue l'interface et laisse les serveurs | `internal/safego`, `cmd/v5_inprocess.go:41` |
| A-03 | `archtest` ne vérifie ni le contenu embarqué, ni les couches (`cmd` → `storage/sqlite`, `services` → `tui`) ; faille `!tag` | `internal/archtest` |
| A-04 | Watchers non attendus à l'arrêt ; un seul verrou pour 11 champs du démon ; pas de `ResponseHeaderTimeout` dans le proxy | `daemon/watch.go:134`, `daemon/daemon.go:116`, `credproxy/proxy.go:119-125` |
| A-05 | Deux bibliothèques TUI (tview et huh/bubbletea), `viper` pour un seul TOML, dépendances non maintenues (`go-figure`, `go-difflib`) | `cli/go.mod` |
| A-06 | CI : tests e2e et conteneur jamais lancés ; version d'opencode non fixée dans le job d'intégration ; 16 `time.Sleep` dans les tests du démon ; `cmd/integration_test.go` sans tag | `.github/workflows/ci.yml` |
| A-07 | Migrations : pas de rollback pour v24–v34 malgré `irreversible: false` ; `MigrateDown` jamais appelé hors tests | `storage/sqlite/store.go` |
| A-08 | Au moins 9 sources de configuration ; deux cascades (modèles, restrictions) aux priorités différentes ; clés héritées sans date de fin | `config/legacy_keys.go`, `bundle/models.go:27-28`, `limits/limits.go:98-101` |
| A-09 | Paquets sans tests : `mcp/jira`, `mcp/github`, `mcp/figma`, `mcp/gslides`, `storage/keychain`, `gateway/beadswire`, `adapters` (registre), `sessionresults` ; dans `cmd/` : `export`/`import`, `purge`, `serve`, `remote_setup` | — |

### 4.9 Documentation et release

| ID | Constat | Où |
|----|---------|----|
| D-01 | Pas de tag v5, CHANGELOG entièrement `[Unreleased]` et contradictoire par endroits | `CHANGELOG.md:10-277` |
| D-02 | Chemin du package `deploycleanup/` faux dans le README, l'overview, le wiki et l'ADR-043 | `README*.md:214`, `docs/architecture/overview.*:263` |
| D-03 | Nombre d'ADR faux (48) dans plusieurs docs de `docs/dev/` | `docs/dev/*.md:3` |
| D-04 | Mode distant marqué « non validé » dans la doc, mais rien dans la CLI | `SECURITY*.md:156`, `remote-runners.*:7` |

---

## 5. Plan de correction

Chaque lot se fait sur une branche `feat/v5-fix-<lot>`, avec un test de non-régression par correction (les PoC de la section 4 en sont le point de départ), puis une fusion au format des corrections v5.

### Lot F0 — Registre de skills communautaires ✅ fait

- Déconnexion de `oh skill add|list|remove|search|budget`, retrait de `internal/skillregistry` et de l'ancien calcul de budget, `~/.oh/skills` plus lu, avertissement dans `oh doctor`, docs et [ADR-051](../architecture/adr/051-community-skills-disconnection.fr.md) (avec les pistes de refonte).
- Couvre : S-30, F-16.

### P0 — Avant la release v5

| Lot | Contenu | Couvre | Critère d'acceptation |
|-----|---------|--------|-----------------------|
| **R — Distant** | Étapes git finales sans rien de contrôlé par l'agent (clone neuf appartenant à root, ou `core.hooksPath=/dev/null`, `core.fsmonitor=false`, `GIT_CONFIG_GLOBAL=/dev/null`, `GIT_CONFIG_NOSYSTEM=1`) ; tuer les processus uid 10001 avant `Finish` ; construction d'image isolée des secrets (un job par cible, `CI_JOB_TOKEN` seul) ; jamais de jeton dans une URL ; empreinte du paquet vérifiée au téléchargement (machine et job) ; rejeu limité aux tickets de la session via l'analyseur de la passerelle ; caractères de contrôle neutralisés à l'affichage. **À défaut** : distant marqué expérimental et désactivé par défaut dans la CLI | S-01 à S-05, D-04 | Les PoC S-01 et S-04 échouent ; un paquet modifié est refusé |
| **G — Passerelle Beads** | Analyse des arguments avec le jeu d'options de `bd` (formes collées, options combinées, `--x=`), ou refus des formes collées inconnues ; `duplicate`, `supersede`, `import` traités comme des fermetures (ou comparaison du statut avant/après) ; mode hook signé par un jeton à usage unique émis par le démon (plus de confiance dans `BD_GIT_HOOK`) ; tout `.beads/` protégé en écriture sauf ce que bd écrit ; base fixée par le démon ; garde de fermeture qui échoue en cas d'erreur et exige un HEAD déplacé et un arbre propre ; environnement minimal pour `bd` | S-06 à S-09, S-14 | Les PoC S-06, S-07, S-08 échouent |
| **B — Paquet de session** | Empreinte vérifiée au chargement et à la réutilisation ; dossiers en 0555 ; `external_directory` limité à la lecture | S-10, S-03 (partie locale) | Une modification du paquet est détectée |
| **W — Couches** | Parent sans `beads:` comparé à `DefaultBeadsAllow` ; `unlocks:` traité comme `mandatory` ; `mcp` et `plugins` en sous-ensemble seulement ; `risk: read` limité aux MCP sans écriture et sans `ctx_*` | S-16 à S-19 | Les PoC S-16 et S-17 produisent un diagnostic |
| **P — Proxy** | Région Bedrock validée (`^[a-z]{2}(-[a-z]+)+-\d$`) ; `models`/`route`/`preset` refusés chez OpenRouter ; données après l'objet JSON refusées ; `allowed_models` illisible = refus | S-21 à S-23 | Le PoC S-21 est refusé |
| **D — Doc sécurité** | Réécriture de `SECURITY.md` (lignes 63, 99, 108-109, 165 : garde-fous et non barrières, mot de passe des serveurs, Cosign) ; adresse de contact | S-12, S-24 (doc), S-39 (doc), S-42 | Plus aucune promesse absolue non tenue |
| **Rel — Release** | `golang.org/x/text` ≥ 0.39 ; golangci-lint à jour en CI (11 constats corrigés) ; `release.yml` déclenché sur `push: tags` ; CHANGELOG séparé en v5.0.0 ; tag | S-41, D-01, santé du code | `govulncheck` et lint propres ; release publiée |

### P1 — Juste après

| Lot | Contenu | Couvre |
|-----|---------|--------|
| **M — MCP** | Échappement et validation des chemins Jira/GitHub ; URL imposée par l'équipe soumise à confirmation (https, hôte épinglé) ; `_meta` qui échoue en fermé, sessions d'un groupe séparées, `MaxBytesReader` ; tests Jira/GitHub | S-13, S-31, S-32, A-09 |
| **C — Conteneur et proxy** | `--cap-drop=ALL`, `no-new-privileges`, HOME par session ; fichier env supprimé après démarrage ; second écouteur limité aux interfaces virtuelles ; capacité exigée sur `policy` ; flux coupés à la révocation ; liste d'environnement en autorisation | S-25 à S-28, S-33, S-35 |
| **H — Robustesse** | `safego` dans démon, runsvc, proxy, passerelle (et interdiction des `go` nus) ; verrou des migrations entre processus ; erreurs de révocation traitées ; watchers attendus à l'arrêt | A-02, A-04, S-29, S-40 |
| **K — Beads et claims** | Nettoyage « zéro impact » avec confirmation et désactivable, respect d'un `.beads/` partagé ; statuts unifiés ; claims avec expiration et non libérés si `cp-2` est reporté | F-01 à F-03 |
| **L — Budgets** | Contrôle après chaque sous-agent, réservation par requête, comptage du cache ; modèles autorisés vérifiés au lancement | F-04, S-27 |
| **E — Ergonomie rapide** | `?` complet ; alias marqués `Deprecated` ; options courtes harmonisées ; codes de sortie distincts (attente = 3, commandes retirées ≠ 0, contrôles optionnels de `doctor` en avertissement) ; `Ctrl+B` remplacé ; confirmation de `i` ; `NO_COLOR` | U-02, U-03, U-05, U-07 à U-09 |
| **T — Contenu** | Nettoyage du contenu V1 des agents et skills ; `archtest` étendu au contenu embarqué ; skills orphelines retirées ; workflows ou suppression pour les 4 agents orphelins (permissions réduites) | F-09, F-10, A-03 |
| **I — i18n** | Garde-fou étendu à tout `cmd/` et à la TUI ; test des clés orphelines ; confirmations en `o`/`oui` | U-11 |

### P2 — Feuille de route

- **Architecture** : service équipe/tracker et découpage de `cmd/` ; une seule bibliothèque TUI ; retrait de `viper`, `go-figure`, `go-difflib` ; règles de couches dans `archtest` ; e2e et conteneur planifiés en CI (A-01, A-05, A-06).
- **Simplification** : un seul coordinateur (`conductor`), `feature` composé de `cadrage` et `ticket`, fragments de checkpoints partagés ; identifiants anglais avec alias français ; une seule vue de suivi ; accueil en 3 étapes (U-01, U-04, U-10, U-12, F-08, F-11).
- **Fonctions** : nettoyage des paquets, sessions et worktrees ; `oh run --dry-run` ; annulation d'une session ; coût par projet et par workflow ; relecture dans `review-feedback` ; worktree par sous-tâche dans `sweep` ; verrous de commit pour les workflows qui écrivent ; GitHub au niveau de GitLab ou retrait (F-05 à F-07, F-13).
- **Briques** : refonte décrite dans l'ADR-051 (commande `oh brick`, import vérifié vers le catalogue d'équipe, éditeur et choix d'agent dans la TUI, identifiants vérifiés dans `config model`) (F-15).
- **Isolation** : conteneur conseillé pour les workflows qui écrivent, `strict_isolation` par défaut en local ; verrous appliqués par le démon (hooks pre-commit/pre-push gérés par oh) plutôt que par motifs (S-11, S-12).

---

## 6. Décisions à prendre

1. **Mode distant** : corriger pour la v5 (lot R), ou le sortir derrière un drapeau expérimental et publier sans lui ?
2. **Sécurité en local** : garder le positionnement « garde-fous » (lot D), ou investir dans des verrous appliqués par le démon et le conteneur par défaut ?
3. **Agents orphelins** (`benchmarker`, `test-generator`, `database`, `infra`) : leur donner des workflows avec des permissions réduites, ou les retirer ?
