# 04 — Phase 0 : remettre en marche sous V2 et poser les fondations

## Objectif
Tous les lancements (CLI + TUI) refonctionnent sous opencode V2 (et toujours V1), avec le **nouveau modèle de lancement** (serve → API → `--server`), le **monde fermé vérifié**, le **proxy d'identifiants**, le **registre des serveurs** et l'**ouverture dans iTerm2/Terminal.app**. Les menus TUI actuels restent en place (leur contenu change en phase 1).

Phase 0 introduit déjà le paquet de session **sans encore les workflows YAML** : on compile un paquet à partir de la **sélection actuelle** (agent d'entrée + agents de l'actuel `WorkflowDefinition` de base), afin que les lancements existants (`start`, `--dev`, `audit`, `review`, `debug`, `--onboard`) passent par le nouveau chemin. Le déploiement dans les projets reste en place jusqu'à la phase 3 mais **n'est plus utilisé** par les lancements.

## Prérequis
Lire `01`, `02` (F1–F27, R1–R7), `03` §2, §6–§9, `11`, `13` §3, §6.

## Lots et tâches

### Lot 0.A — Détection d'outil et adaptateurs

| ID | Tâche | Fichiers | Détails |
|---|---|---|---|
| P0-T01 | Package `internal/adapters` + interface `ToolAdapter`, `Capabilities`, `VisibilityReport`, `ServerHandle` | nouveau `internal/adapters/adapter.go` | Voir `03` §2.2 |
| P0-T02 | `Detect()` : binaire, version (`opencode --version`), choix V1/V2, version minimale supportée | `internal/adapters/opencode/detect.go` | Réutiliser `internal/opencode` (FindBinary) et `internal/semver` |
| P0-T03 | Modèle neutre `SessionSpec`, `BundleSpec`, `ProviderSpec`, `PermissionRule` (neutre), `ModelRef` | `internal/sessionspec/` | Pas de dépendance à opencode |
| P0-T04 | **Rendu V2** : `config.json` (`agents` avec `disabled` sur **tous** les natifs recensés, `permissions[]` ordonnées, `skills:[dir]`, `plugins:[dir]`, `experimental.subagent_depth`, `provider.<id>.options{region, baseURL}`, `model`, `default_agent`) | `internal/adapters/opencode/v2/render.go` | Natifs : recensés dynamiquement au premier démarrage (`agent.list` hors paquet, non `hidden`) et mis en cache par version ; refus `skill` sur `opencode`, `report` et `*` sauf paquet ; `execute` deny si `code_mode: false` |
| P0-T05 | **Rendu V1** : schéma V1 (`agent`, `permission{}`, `disable`, `subagent_depth` racine, `instructions`) | `internal/adapters/opencode/v1/render.go` | Réutiliser `convertPermissionForJSON` (`agent_config.go:172-213`) |
| P0-T06 | Tests golden des rendus V1/V2 | `…/testdata/golden/*` | Plusieurs paquets types |

### Lot 0.B — Paquet de session (version transitoire)

| ID | Tâche | Fichiers | Détails |
|---|---|---|---|
| P0-T07 | `BundleService.Build` (transitoire) : à partir d'un agent d'entrée + liste d'agents (dérivée du `WorkflowDefinition` de base et du rôle de l'agent d'entrée), assemble agents (Bucket A inliné), skills (Bucket B + stack), permissions neutres, graphe `subagent`, `MaxDepth` | `internal/services/bundle/` | Déplacer la logique de `skill_assembly.go`, `permission_resolve.go`, `model_resolve.go`, `stack_skills.go` (ne pas dupliquer : extraire en paquets partagés utilisés par deploy (temporaire) et bundle) |
| P0-T08 | Layout disque `~/.oh/bundles/<hash>/` (`03` §6) ; hash canonique ; idempotence | idem | Purge > 30 j |
| P0-T09 | Instructions projet (ONBOARDING.md, CONVENTIONS.md, `.claude/CLAUDE.md`, `[deploy].instruction_files`) **intégrées au corps des agents** dans le paquet (V2 n'ignore plus rien) | idem | Ne jamais écrire `AGENTS.md` |
| P0-T10 | Plugin oh minimal embarqué (`go:embed`) : dossier `plugin/` + `package.json` + `index.ts` exportant `{id:"oh", setup}` et `server()` V1 ; fonctions phase 0 : **prompt ajouté** (O2 : `session.hook("context")` pousse le corps de l'agent courant), **retrait des agents/skills hors paquet** (`agent.transform`/`skill.transform`) | `internal/adapters/opencode/plugin/` | Les agents rendus en V2 **n'ont pas de `system`** (corps injecté par le plugin) ; si le plugin échoue à charger → repli : `system` = corps + skill « posture outillage » (log d'avertissement) |
| P0-T11 | Skill « posture outillage » minimale (tool-agnostique) | `skills/shared/tooling-posture.md` | Paralléliser, outils dédiés vs shell, édition ciblée, conventions du code, commentaires sobres |

### Lot 0.C — Proxy d'identifiants

| ID | Tâche | Fichiers | Détails |
|---|---|---|---|
| P0-T12 | `CredentialProxy` : listener local, table jeton→(session, fournisseur, modèles, budget), remplacement d'en-tête, streaming sans tampon, 401 | `internal/services/credproxy/` | `httputil.ReverseProxy{FlushInterval:-1}` ; tests avec serveur amont factice (SSE/chunké) |
| P0-T13 | Fournisseurs : Bedrock (Bearer API key), Anthropic (`x-api-key`), OpenRouter (Bearer) ; **SigV4** pour Bedrock via profil AWS | idem | `aws-sdk-go-v2/aws/signer/v4` ; région obligatoire (F15) |
| P0-T14 | Lecture des secrets via `domain.SecretStore` (trousseau / filecrypt) — remplace l'injection directe de `opencode.go:159-185` | idem + `internal/provider` | Le processus opencode ne reçoit **que** le jeton de session |
| P0-T15 | Comptage tokens/coût depuis les réponses (`usage`) ; hooks budget (désactivés par défaut, I6) | idem | |

### Lot 0.D — Groupes de serveurs, sessions, lancement

| ID | Tâche | Fichiers | Détails |
|---|---|---|---|
| P0-T16 | Migration **v28** (`servers`) et **v29** (colonnes `sessions`) | `internal/storage/sqlite/store.go` | `03` §7 |
| P0-T17 | `RunService` : clé de groupe (bundle hash, projet, runtime=local) ; démarrage `opencode serve` (port libre, mot de passe aléatoire chiffré, `XDG_DATA_HOME=~/.oh/servers/<gk>/data`, `OPENCODE_DISABLE_PROJECT_CONFIG=1`, config rendue, jeton proxy) ; attente disponibilité (polling `agent.list` avec `location`) | `internal/services/run/` | R1 |
| P0-T18 | **Attest** après démarrage (R2) ; échec ⇒ message précis + session non lancée | `internal/adapters/opencode/v2/attest.go` | Tolérer `compaction`, `title`, `summary` (hidden) |
| P0-T19 | Création de session par l'API (id `ses_` + 26 car. généré par oh, agent, titre, location, permissions de session) + envoi du prompt | `…/v2/session.go` | R3 |
| P0-T20 | **Attachement** : `AttachCommand` = `opencode --server <url> -s <id>` + `OPENCODE_SERVER_PASSWORD` ; ouverture selon préférence I2 via `internal/termlaunch` (étendre : style iTerm `tab/split/window`, Terminal.app `window`, tmux `new-window/split-window` si `$TMUX`, navigateur via `pair`, suspension en dernier recours) | `internal/termlaunch/*`, `internal/services/run/attach.go` | Autorisation macOS « Automation » : détecter l'échec osascript et proposer le mode suivant |
| P0-T21 | Adaptateur V1 : lancement transitoire (`opencode --agent … --prompt …`, `OPENCODE_DATA_HOME`), capacités réduites | `internal/adapters/opencode/v1/launch.go` | |
| P0-T22 | **SSE socle** : abonnement `/api/event` par serveur, resynchronisation (sessions, permissions, formulaires) à la (re)connexion ; mise à jour `sessions.state`, coût/tokens (remplace la lecture de `opencode.db`, `platform_adapter.go:129-169`) | `internal/services/session/events.go` | F18 ; pas de rejeu |
| P0-T23 | **Fermer la fenêtre ≠ arrêter** : la session reste active ; état `detached` | RunService | I1 |
| P0-T24 | **Veille** : serveur inactif sans décision en attente → arrêt après 5 min (réglage) ; reprise = redémarrage du groupe + `-s <id>` | RunService | I7 |
| P0-T25 | **À la fermeture de oh** : sessions inactives/en attente → veille immédiate ; sessions qui travaillent → fenêtre « finir l'étape puis mettre en veille (défaut) / continuer en arrière-plan / arrêter maintenant », par session. « Finir l'étape » = attendre `session.execution.succeeded` ou prochaine décision en attente, puis veille | RunService + TUI + CLI | I7 |
| P0-T26 | Au démarrage de oh : réconciliation du registre (`servers` vivants/morts), récap « ce qui a tourné pendant votre absence » (sessions, coût) | RunService + TUI (toast/notification) | |
| P0-T27 | `experimental.subagent_depth` = profondeur du graphe ; test de contrat (chaîne 3 niveaux, `e2e`) | adaptateur V2 | F9 |

### Lot 0.E — Branchement CLI et TUI (sans changer les menus)

| ID | Tâche | Fichiers | Détails |
|---|---|---|---|
| P0-T28 | `launcher.Launch` délègue au `RunService` (agent, prompt, chemin inchangés) ; `--dev`, `--onboard`, `audit`, `review`, `debug`, `--worktree`, `-t` continuent de marcher | `internal/launcher/launcher.go`, `cmd/start*.go`, `cmd/audit_review_debug.go` | |
| P0-T29 | **Contexte actif unique** `shell.Context{Project, Team, Path}` ; `resolveActiveProject` le lit d'abord ; plus de huh pendant la TUI (B4) ; `makeResolveTeamFunc` lit `shell.ActiveTeam()` (B5) ; config du team board construite à l'ouverture | `internal/tui/v2/shell/shell.go`, `cmd/start_helpers.go`, `cmd/tui_helpers.go`, `cmd/tui_team_board.go` | |
| P0-T30 | **`shell.RunWorkflow(req)`** unique, toujours dans la boucle d'événements (B6) ; indicateur « Préparation… » non bloquant ; toast de fin (coût, branche) | `shell.go`, `cmd/tui_session_actions.go`, `views/board_quick_actions.go` | |
| P0-T31 | Corriger B7 (Quick/coder → même agent par défaut ; Onboard avec prompt ; Audit accueil avec sélecteur ; Debug à l'accueil) | `cmd/tui_session_actions.go`, `views/home.go` | |
| P0-T32 | Réglages : `Ouverture des sessions` (`auto|iterm|terminal|tmux|browser|suspend`, style iTerm), `Veille (min)`, version opencode affichée | `views/settings_view.go`, `internal/config` | |
| P0-T33 | Doctor : version opencode + adaptateur + niveau d'isolation ; autorisation Automation macOS ; proxy OK ; git ≥ 2.48 | `cmd/doctor*.go`, `views/doctor_view.go` | |
| P0-T34 | README : TUI = tview (B14) ; CHANGELOG | `README*.md`, `CHANGELOG.md` | |

## Critères d'acceptation
1. Sous opencode **2.0.20** : `oh start`, `oh start --dev -t <id>`, `oh start --onboard`, `oh audit -t security`, `oh review`, `oh debug -i …` ouvrent une session **dans un onglet iTerm2** (ou une fenêtre Terminal.app) sur le **bon agent**, sans `build/plan/general/explore` visibles, sans skills `opencode`/`report`.
2. `Attest` échoue proprement si un agent hors paquet apparaît (test : injecter un agent dans `~/.config/opencode/agents/`).
3. `env` lancé par un agent ne montre **aucun** secret réel (seulement le jeton de session).
4. Fermer l'onglet opencode laisse la session vivante ; `oh` la montre active ; ré-attacher fonctionne.
5. Quitter `oh` avec une session qui travaille → fenêtre de choix ; défaut = finir l'étape puis veille.
6. Serveur inactif mis en veille après 5 min ; reprise OK.
7. `feature` : orchestrator → orchestrator-dev → developer fonctionne (profondeur 3).
8. Sous opencode **V1** (≥ version minimale) : mêmes commandes fonctionnent (capacités réduites).
9. Tests : unitaires, golden V1/V2, contrat (`integration`, vrai serveur sans LLM), e2e Haiku (`e2e`) pour la profondeur et l'attachement.

## Hors périmètre phase 0
Workflows YAML, fiche de lancement, vue Sessions, boîte À traiter, conteneurs, distant, suppression du déploiement.

## Risques
- API V2 expérimentale (`wait`, `instructions`, `stats`) → replis via SSE.
- Échec de chargement du plugin → repli `system` + posture (P0-T10).
- Autorisation macOS Automation refusée → mode suivant de la chaîne I2.

## Constats complémentaires (relecture du code, préparation du build)
- `internal/parallel/server.go` parle l'**API V1** (`/global/health`, `POST /session`, `/session/{id}/prompt_async` avec `parts`) → **cassé sous V2** ; `--parallel` et `--sweep` sont donc aussi touchés.
- `internal/opencode/compatibility.json` : oh 2.0 → opencode max `1.99.99` ⇒ V2 signalé incompatible ; à mettre à jour (entrée v5, opencode 1.18.29–2.x).
- `platform_adapter.go` : `RequiresDeploy() = true`, `Events = false`, stats lues dans `opencode.db` (`enrichFromDB`) → remplacés par l'API pour V2.
- `provider.ResolveCredentials` renvoie soit un **jeton Bearer** Bedrock, soit un **profil AWS** → le proxy doit gérer les deux (SigV4 pour le profil).
- `internal/launcher/launcher.go` est le point de convergence unique (CLI + TUI) → c'est là qu'on branche le `RunService`.
- `internal/termlaunch` gère déjà iTerm2/Terminal.app (nouvelle fenêtre/onglet via AppleScript) → à étendre (style, tmux, navigateur, détection d'échec).

## Ordre d'exécution (tranche verticale d'abord)

Branche : `feat/v5-phase0`. Un commit par étape, tests verts à chaque étape. Chaque étape est livrable seule.

| Étape | Contenu | Tâches | Vérification |
|---|---|---|---|
| E1 | **Client HTTP V2** (`internal/adapters/opencode/v2/client.go`) : auth Basic, `info`, `agent.list`, `skill.list`, `mcp.list`, `session.create/get/prompt`, `permission list/reply`, `form list/reply`, `wait`, `diff`, SSE `event` + tests de contrat (`-tags integration`) | P0-T01, P0-T03 | Tests contre un vrai `opencode serve` |
| E2 | **Cycle de vie serveur** : port libre, mot de passe aléatoire, `XDG_DATA_HOME`, `OPENCODE_DISABLE_PROJECT_CONFIG`, `OPENCODE_CONFIG_CONTENT`, attente de disponibilité (agents chargés), arrêt propre | P0-T17 (partiel) | Démarrage < 3 s, arrêt sans processus orphelin |
| E3 | **Rendu V2 + Attest** : config (agents, natifs désactivés recensés dynamiquement, permissions, skills, plugins, `experimental.subagent_depth`, provider région/baseURL, `default_agent`), vérification d'isolation | P0-T02, P0-T04, P0-T18, P0-T27 | Golden + contrat (agent parasite dans `~/.config/opencode` ⇒ échec) |
| E4 | **Paquet transitoire** : `BundleService.Build` qui réutilise les fonctions existantes (assemblage Bucket A, permissions, modèles, stack skills) **sans les déplacer** (exports minimaux) ; layout `~/.oh/bundles/<hash>` ; instructions projet intégrées aux agents | P0-T07, P0-T08, P0-T09 | Golden ; `deploy` actuel inchangé |
| E5 | **Plugin oh minimal** (embarqué) : prompt ajouté (`context`), retrait hors paquet (`agent/skill.transform`) ; repli `system` + skill posture | P0-T10, P0-T11 | e2e Haiku : prompt de base conservé + corps de l'agent présent (sonde) |
| E6 | **Proxy d'identifiants** : Bearer Bedrock, Anthropic, OpenRouter ; streaming ; 401 ; comptage | P0-T12, P0-T14, P0-T15 | Tests avec amont factice + e2e Haiku ; `env` sans secret |
| E7 | **SigV4** (profil AWS) dans le proxy | P0-T13 | Test signature (vecteurs AWS) + e2e si profil dispo |
| E8 | **Migrations v28/v29 + RunService** : groupes, registre, création de session par l'API, prompt initial, attachement (`termlaunch` étendu : iTerm2 onglet → Terminal.app fenêtre → tmux → navigateur → suspension) | P0-T16, P0-T17, P0-T19, P0-T20 | Lancement réel dans iTerm2 sur le bon agent |
| E9 | **Branchement du launcher** : V2 → RunService ; V1 → comportement actuel enveloppé dans l'adaptateur V1 ; matrice de compatibilité mise à jour | P0-T21, P0-T28 | Toutes les commandes de lancement OK sous 2.0.20 |
| E10 | **SSE socle** : abonnement par serveur, resynchronisation, état des sessions, coût/tokens par l'API | P0-T22 | Coût/tokens corrects en base après session |
| E11 | **Cycle de vie** : détaché ≠ arrêté, veille 5 min, fermeture de oh (finir l'étape/arrière-plan/arrêter), réconciliation au démarrage | P0-T23, P0-T24, P0-T25, P0-T26 | Scénarios manuels + tests unitaires de la machine d'états |
| E12 | **Parallèle/sweep sous V2** : porter `internal/parallel/server.go` sur le client V2 (ou passer par RunService, N sessions dans un groupe) | (ajout) | `oh start --parallel` OK sous 2.0.20 |
| E13 | **TUI** : contexte unique, `shell.RunWorkflow`, B4–B7, B12, réglages (ouverture, veille), Doctor, README/CHANGELOG | P0-T29 à P0-T34 | Tests vues + manuels |

### Écarts constatés pendant le build
- **E4 — graphe de délégation** : le graphe dérivé du workflow actuel (`workflow.DeriveTaskPermissions`) autorise tout agent « en aval » (ex. developer → orchestrator-dev, reviewer → developer). En phase 0, le paquet utilise donc la permission `task` **résolue de chaque agent** (frontmatter + base), qui exprime les délégations voulues ; le workflow sert seulement à exclure les agents désactivés. Résultat : paquets resserrés (ex. `reviewer` = reviewer + documentarian ; `orchestrator-dev` = 6 agents, profondeur 2 ; `orchestrator` = 12 agents, profondeur 3). En phase 1, le graphe du YAML devient la source de vérité.
- **E4 — B8 corrigé pour les paquets** : l'inlining Bucket A utilise les skills générées par le workflow (`deploy.AssembleAgent`).
- **E5 — plugin** : `ctx.agent.transform` / `ctx.skill.transform` fonctionnent (natifs et skills intégrées retirés ; `/api/skill` reflète alors le retrait). `session.hook("context")` reçoit `event.agent` sous forme d'**identifiant (chaîne)**. Le plugin est installé dans le dossier du groupe (`<data>/oh-plugin/` + `agents/<id>.md`), pas dans le paquet immuable. Les plugins globaux de l'utilisateur sont signalés en **avertissement** par Attest (pas bloquant). La skill « posture outillage » est embarquée dans l'adaptateur (repli uniquement) ; le fichier hub `skills/shared/tooling-posture.md` est reporté à BL-6 (autres adaptateurs).
- **E6 — proxy** : paquet `internal/credproxy` (routes `/<provider>/…`). Le **budget est en tokens** (entrée + sortie) côté proxy ; le budget en USD (I6) sera calculé par le SessionService à partir des coûts renvoyés par opencode. Comptage des tokens par lecture du flux (Bedrock `inputTokens/outputTokens`, Anthropic `input_tokens/output_tokens`, OpenAI/OpenRouter `prompt/completion_tokens`). Validé en e2e : Bedrock via `providers.amazon-bedrock.settings.baseURL`, l'agent ne voit que `ohs_…`, le processus serveur ne contient pas la vraie clé.
- **E7 — SigV4** : `credproxy.SigV4Auth` (`aws-sdk-go-v2` signer + `config.LoadDefaultConfig` avec profil ; cache de credentials ; échappement de chemin par défaut du SDK pour les services non-S3). Vecteur officiel AWS `get-vanilla` vérifié. e2e avec profil réel : `OH_E2E_AWS_PROFILE` (non exécuté ici : aucun profil avec identifiants sur la machine de build).
- **E8 — démon `ohd`** (décision 05/10) : processus unique par utilisateur, socket Unix `~/.oh/run/ohd.sock` (0600), démarré à la demande (`daemon.Ensure`, verrou de spawn), arrêt automatique après **10 min sans serveur vivant**, remplacé seulement sans session active si la version diffère. Héberge le proxy (port stable `ohd.json`), persiste les jetons dans `proxy_grants` (**jeton + source, jamais le secret**), recharge les secrets du trousseau / profil AWS au redémarrage, sinon jetons « en attente » complétés par un client. Supervise les serveurs (PID mort → `stopped` + révocation).
- **E8 — un jeton par serveur (groupe)**, pas par session : le jeton est dans l'environnement du processus `opencode serve`, partagé par les sessions du groupe. Le comptage par session viendra des statistiques opencode (E10).
- **E8 — clés LLM au niveau équipe** (décision 05/10) : cascade `ProjectProviderConfig.TokenKey` → `openhub.provider.<p>.token.<projet>` → `openhub.team.<team>.provider.<p>.token` → `openhub.provider.<p>.token` → (Bedrock) profil AWS SigV4 (`provider.ResolveCredentialSource`). Saisie TUI prévue en E13.
- **E8 — ouverture** : `auto` = terminal courant d'abord (iTerm2 ou Terminal.app selon `TERM_PROGRAM`), puis l'autre, puis tmux ; l'onglet exécute `oh session attach <id> --exec` (aucun secret en ligne de commande). Navigateur / suspension gérés par l'appelant (`--how browser|suspend`).
- **E8 — base** : migrations v28 `servers`, v29 colonnes `sessions` (workflow_id, entry_agent, bundle_hash, group_key, runtime, mode, state), v30 `proxy_grants` ; `PRAGMA busy_timeout=5000` ; `oh.db` en 0600. La numérotation du plan (`03` §7) est décalée : `pending_decisions` → v31, `preferences` → v32, suppression `projects.agents` → v33/v34.
- **E9 — branchement** : `launcher.V5Launch` (posé par `cmd`) prend la main dès qu'opencode V2 est détecté (`OH_V5=0` force l'ancien chemin) ; tous les lancements CLI et TUI passent par lui ; le déploiement automatique est sauté en V5 (`oh start`). Agent par défaut = `orchestrator` (B7). Réglages `[session] attach`, `iterm_style`, `idle_sleep_minutes` dans `hub.toml` + variable `OH_SESSION_ATTACH`.
- **E9 — FAILLE corrigée** : sans restriction, un modèle indisponible faisait **basculer opencode sur ses modèles gratuits hébergés** (`opencode/fledge-alpha-free`) : le prompt partait chez un autre fournisseur. Le rendu V2 ajoute `experimental.policies` : `provider.use` refusé sauf le fournisseur de la session (vérifié : seul `amazon-bedrock` est listé).
- **E9 — Bedrock** : les modèles Anthropic exigent un **profil d'inférence régional** (`eu.anthropic…` en `eu-west-1`) ; l'adaptateur ajoute le préfixe selon la région (`ModelID`). Le modèle de la session est fixé à la création (modèle de l'agent d'entrée).
- **E9 — bug corrigé** : `opencode --version` de V2 renvoie `opencode v2.0.20` → la vérification de compatibilité le jugeait « trop ancien ». Matrice mise à jour (entrées 4.2 et 5.0, opencode ≤ 2.99.99).
- **E9 — `OH_HOME`** relocalise `~/.oh` (tests, environnements isolés) ; propagé à la commande d'attachement.
- **E9 — validé en réel** : `oh start -p … --prompt …` → paquet, démon, serveur, réponse Bedrock EU via le proxy (« OK V5 ») ; 2e lancement = même serveur ; ouverture dans une fenêtre Terminal.app, client `opencode --server … -s …` sans mot de passe en ligne de commande.
- **E10 — suivi** : le **démon** (pas le client) tient un abonnement SSE par serveur prêt (`daemon/watch.go`), avec resynchronisation à chaque (re)connexion via `GET /api/session/active` + permissions/formulaires en attente. États dérivés : `waiting` (décision en attente) > `active` (boucle en cours) > `idle`. Coût/tokens lus via `GET /api/session/{id}` (sans diff) en fin de tour et au plus toutes les 5 s sur `session.usage.updated`. Seules les sessions créées par oh (ligne `sessions` du même groupe) sont suivies. Événements normalisés côté adaptateur (`adapters.EventKind`). `Touch` d'un client réveille immédiatement la supervision.
- **E11 — cycle de vie** : la veille est décidée par le **démon** par groupe de serveur : jamais si un client est attaché (battements `attach` envoyés par `oh session attach`, qui n'utilise plus `exec` mais lance l'interface en enfant) ni si une boucle tourne ; une décision en attente garde le serveur éveillé **tant qu'une TUI oh est ouverte** (battement `presence`), sinon minuterie `[session] idle_sleep_minutes` (5 par défaut). Veille = serveur arrêté, jetons révoqués, sessions `sleeping` (reprenables : même dossier de données). `oh session attach` reprend automatiquement une session en veille (`RunService.ResumeSession`, paquet rechargé par hash). `oh session list [--all]`, `oh session stop`. Fermeture de la TUI : groupes inactifs/en attente → `sleep_when_idle` ; si des sessions travaillent → une **seule** question (finir l'étape puis veille / arrière-plan / arrêter / annuler) appliquée à tous les groupes qui travaillent (choix par session reporté à la vue Sessions, phase 3). Récap « pendant votre absence » au démarrage de la TUI (`~/.oh/run/last_seen`, colonne `sessions.state_changed_at`, migration **v31**). Validé en réel : veille après 1 min, reprise + ouverture dans Terminal.app.
- **E12 — parallèle / sweep / headless** : la plateforme opencode est décorée par `v5Platform` (cmd) quand V2 est présent : `NewParallelRunner` renvoie un runner v5 (une session par worktree, **un seul serveur pour toute l'exécution** grâce au multi-dossiers), `RunHeadless` crée une session sans interface, attend la fin du tour, lit le texte de l'assistant puis arrête la session. `Cleanup` du coordinateur applique « finir l'étape puis veille » au lieu de tuer les serveurs. `autoDeployIfNeeded` et `EnsureWorktreeConfig` (`worktree.SessionBundles`) ne font plus rien sous V2. **Le client V1 de `internal/parallel` est conservé** pour opencode V1 (D3). Validé en réel : `oh start --parallel --tickets t1,t2` → 2 worktrees, 1 groupe, sessions terminées puis en veille. `RunHeadless` non validé de bout en bout (le binaire de test n'accède pas au trousseau) : à vérifier via `oh takeover-brief enrich` en E13.
- **E12 — `hubcontent.HubContentDir`** suit `OH_HOME`. Le contenu embarqué local (`internal/hubcontent/hub`, ignoré par git) peut être obsolète en dev : `make embed-sync`.
- **E13 — TUI** : contexte actif unique (`resolveActiveProject` lit le projet actif du shell, jamais de sélecteur huh pendant la TUI ; vues équipe sur l'équipe active ; `TeamsProvider` pour Ctrl+T) ; **pipeline de lancement unique `runTUILaunch`** hors boucle d'événements, toutes les opérations d'interface remises sur la boucle (toasts, suspension) ; Quick = `orchestrator`, Onboard avec son prompt, Audit/Debug avec sélecteur depuis l'accueil (Debug ajouté) ; réglages **Sessions** (ouverture, style iTerm2, veille) ; **clé LLM d'équipe** (touche `k` dans Détail équipe, stockée dans le trousseau) ; vérifications Doctor v5 (runtime, démon, git ≥ 2.48, chaîne d'ouverture) partagées CLI/TUI ; README (tview, nouveaux paquets, commandes `oh session`/`oh daemon`) et CHANGELOG.
- **Reste à valider manuellement** (pas de TTY dans l'environnement de build) : la TUI interactive (lancement depuis les menus/board, question à la fermeture, récap), `RunHeadless` de bout en bout (`oh takeover-brief enrich`), l'autorisation macOS « Automation » pour iTerm2 (iTerm2 absent de la machine de build), SigV4 avec un vrai profil AWS (`OH_E2E_AWS_PROFILE`).
- **Reporté** : sélecteur de tickets tview (T2, phase 1) ; choix de fermeture session par session (vue Sessions, phase 3) ; détection préalable de l'autorisation Automation dans Doctor (pas de détection fiable sans déclencher la demande).
- **E4 — `context-mode`** n'est pas ajouté aux paquets (compatibilité V2 non vérifiée) ; ce sera un plugin déclaré par workflow (O5).

### Choix d'exécution validés (05/10/2026)
- Branche unique `feat/v5-phase0` depuis `main`, **un commit par étape** (E1…E13), tests verts à chaque commit.
- **SigV4 inclus** en phase 0 (E7) : dépendance `github.com/aws/aws-sdk-go-v2` (signer v4 + chargement de profil).
- **Parallèle et sweep portés via le RunService** (E12) : N sessions dans un même groupe de serveur ; suppression du client V1 de `internal/parallel/server.go` une fois le portage fait.
- Portée de la session de build : **E1 → E13**, une étape à la fois, avec un point de contrôle après chacune.

## Audit avant fusion (05/10/2026) et lot E14 « durcissement »

Audit complet de `feat/v5-phase0` (deux revues indépendantes + vérification) : architecture saine, build/vet/tests/lint OK, **1 bloquant + 9 majeurs** → fusion conditionnée au lot E14 puis à une recette manuelle. Les points couverts par la suite du plan ont été reportés dans les fiches des phases concernées (section « Reprises de l'audit phase 0 »).

### Lot E14 — à faire avant la fusion

| ID | Point | Où | Correction attendue | Test |
|---|---|---|---|---|
| E14-B1 | Un serveur qui redémarre perd son jeton proxy pendant son démarrage (`created_at` non mis à jour à l'upsert → le démon le juge mort, `SetStatus(stopped)` + révocation) | `storage/sqlite/server_store.go:44-48`, `daemon/daemon.go:271-276`, `runsvc/service.go:244-264` | Colonne `started_at` (ou `created_at` mis à jour) ; `SetStatus` conditionnel (`WHERE status=? AND pid=?`) ; relire la ligne avant révocation | Test démon : redémarrage d'un groupe ancien → jeton conservé |
| E14-M1 | PID réutilisé → `SIGTERM/SIGKILL` sur le groupe d'une autre application | `filelock`, `daemon/proc_unix.go`, `opencodev2/proc_unix.go`, `runsvc/service.go:191-201`, `daemon/lifecycle.go:163` | Ne jamais signaler sans appel authentifié réussi (`Healthy` avec mot de passe) ; au démarrage du démon, marquer `stopped` les serveurs plus anciens que le boot | Test : PID vivant mais serveur muet → aucun signal |
| E14-M2 | Veille d'un serveur en cours d'usage (session créée dans l'outil avant la ligne en base → cache négatif ; veille possible sans resynchronisation) | `daemon/watch.go:158,210-223`, `lifecycle.go:139-155`, `runsvc/service.go:135-139` | Persister la session **avant** `CreateSession` ; pas de cache négatif (ou TTL court) ; décision de veille fondée aussi sur `ActiveSessions` de l'outil ; pas de veille sans resync réussie | Test démon : session inconnue puis connue → suivie ; flux jamais connecté → pas de veille |
| E14-M3 | Permissions élargies : fusion `write`/`patch` → `edit`, dernière règle gagne (benchmarker `edit: deny` + `write: allow` → modifie tout) | `bundle/permissions.go:12-31` | Effet le plus restrictif quand plusieurs clés V1 tombent sur la même action (deny > ask > allow) | Test sur le cas benchmarker |
| E14-M4 | SQLite : `busy_timeout`/`foreign_keys` sur une seule connexion du pool ; `oh.db-wal`/`-shm` en 0644, `~/.oh` en 0755 | `storage/sqlite/store.go:31-56` | DSN `?_pragma=busy_timeout(5000)&_pragma=foreign_keys(1)&_pragma=journal_mode(WAL)` ; `chmod 0700` du hub, 0600 des fichiers WAL | Test d'ouverture : droits + pragma sur 2 connexions |
| E14-M5 | Région Bedrock forcée à `us-east-1` sans config oh (données hors UE en silence) | `runsvc/service.go:165-172` | Cascade config → `AWS_REGION`/`AWS_DEFAULT_REGION` → profil AWS → `us-east-1` + avertissement | Test unitaire de la cascade |
| E14-M6 | Clé de groupe sans fournisseur/région/source → un changement de clé ou de région est ignoré tant que le serveur tourne | `runsvc/service.go:114`, `sessionspec/spec.go` | Ajouter fournisseur + région + source d'identifiants dans la clé (hash) ; encodage réversible ou hash de l'ID projet (`a.b` vs `a_b`) | Test : deux fournisseurs → deux groupes |
| E14-M7 | Touche `k` volée dans « Détail équipe » (navigation cassée) | `views/team_detail_view.go:173` | Touche `K` (ou autre) + aide mise à jour | Test de vue |
| E14-M8 | Ctrl+C du moniteur parallèle cassé (`signal.Ignore`/`Reset`) | `cmd/v5_launch.go:209-210` | `signal.Notify(ch, os.Interrupt)` + `signal.Stop(ch)` pendant le client | — |
| E14-M10 | Plus d'événement d'équipe `session.complete` pour les sessions v5 | daemon / runsvc | Émettre l'événement à l'arrêt/fin de session (`StopSession`, `markSessions(stopped)`) avec durée, tokens, coût, membre | Test : arrêt → événement ajouté |
| E14-M11 | Présence TUI jamais déclarée si le démon n'était pas lancé à l'ouverture | `cmd/tui_v5_lifecycle.go:71-75` | Boucle de reprise (Health + KeepAlive toutes les 30 s) tant que la TUI est ouverte | — |
| E14-K | Erreur de trousseau traitée comme « absent » → cascade vers une autre clé (facturation) | `provider/source.go:62` | Ne descendre que sur « non trouvé » ; propager les autres erreurs | Test unitaire |
| E14-W | Windows + opencode V2 : chaque lancement échoue après 10 s (proxy en processus prévu en E8 non implémenté) | `daemon/client.go`, `cmd/v5_launch.go` | **Option B** : message clair « non pris en charge en phase 0 », sans attente ; `Ensure` renvoie `ErrUnsupported` immédiatement. Option A (proxy en processus) reportée (voir phase 4) | — |
| E14-P | Petites corrections | divers | `Accept-Encoding` retiré par le proxy ; plugin : `skills` `[]` au lieu de `null` ; tag `//go:build integration && e2e` sur `opencodev2/e2e_test.go` ; libellé « Amazon Bedrock (clé API) » traduit ; `oh doctor` : checks v5 informatifs si V2 absent (pas d'échec pour V1) ; CHANGELOG : retirer « `OH_V5=0` pour l'ancien comportement » (cassé sous V2) et mentionner Windows | — |
| E14-D | Processus | `.github/workflows/ci.yml`, `SECURITY.{fr,en}.md`, `docs/guides/` | Job CI qui installe opencode 2.0.20 et lance `-tags integration` (sans LLM) ; note de sécurité « mode local : l'agent partage l'utilisateur, accès possible au socket du démon et à `oh.db` ; isolation en phase 4 » ; guide bilingue « sessions v5 » (ouverture, veille, reprise, `OH_V5`, `OH_HOME`, clés d'équipe) | CI verte |

**Recette manuelle après E14** : TUI (lancements menus/board, question de fermeture, récap), iTerm2 si un poste en dispose, `oh takeover-brief enrich` (headless), SigV4 avec `OH_E2E_AWS_PROFILE` si possible. Puis fusion ; annoncer dans les notes de version que le runtime v5 s'active pour tous les utilisateurs d'opencode V2 et que les migrations v28–v31 s'appliquent.

**Estimation** : ~½ journée de code + recette.

### Lot E14 — réalisé (05/10/2026)

Tous les points du lot sont traités, avec ces précisions :
- **B1** : `created_at` mis à jour à l'upsert (date du démarrage courant) ; `SetStatusIf` (changement conditionnel sur pid et statut) avant toute révocation ; un serveur `starting` récent n'est jamais touché.
- **M1** : le démon vérifie un PID vivant par un appel authentifié (`ActiveSessions`) ; le succès est mis en cache par (groupe, pid), et le serveur n'est déclaré étranger qu'après **3 échecs consécutifs** (un serveur chargé peut être lent). Il est alors marqué arrêté **sans signal**. `StopServer` (adaptateur) ne signale qu'après un `Healthy` authentifié. Il n'y a pas de nettoyage spécifique au boot : la vérification suffit.
- **M2** : la ligne de session est écrite **avant** `CreateSession` (marquée `failed` si la création échoue) ; cache négatif de 5 s seulement ; pas de veille tant que le flux n'a pas été resynchronisé (`synced`, remis à faux à chaque coupure) ; avant de mettre en veille, le démon demande à l'outil s'il a des sessions actives (erreur = occupé).
- **M3** : fusion des clés V1 vers une même action par ressource, effet le plus restrictif (deny > ask > allow), évaluation par motif générique.
- **M4** : pragmas dans le DSN (`busy_timeout`, `foreign_keys`, WAL) ; `oh.db` créé en 0600 **avant** la première connexion (SQLite reprend ce mode pour `-wal`/`-shm`) ; dossier du hub en 0700.
- **M5** : cascade de région config oh → SDK AWS (`AWS_REGION`, `AWS_DEFAULT_REGION`, profil) → `us-east-1` + avertissement (`credproxy.AWSRegion`).
- **M6** : `GroupKey.Config`, une empreinte SHA-256 de l'ID projet exact, du fournisseur, de la région, de la source d'identifiants et du hash du secret ; format `<projet>-<paquet12>-<config10>-<runtime>`. Une clé changée démarre donc un nouveau serveur. **À la reprise, la session reste dans son groupe d'origine** (ses données y sont) : un serveur en veille redémarre avec les nouveaux réglages, un serveur vivant garde les siens.
- **M7** : touche `K`. **M8** : `signal.Notify`/`Stop`. **M11** : `KeepAlive` réessaie à chaque période, même sans démon au départ.
- **M10** : crochet `OnSessionEnd` (RunService : synchrone, à la commande `stop` ; démon : asynchrone, à l'arrêt définitif) → événement `session.complete` (`sessionEndHook`).
- **K** : une erreur du trousseau interrompt la cascade (`domain.ErrNotFound` reste traité comme « absent »).
- **W** : `daemon.Ensure` renvoie `ErrUnsupported` immédiatement sous Windows, et `v5Launch` affiche un message clair.
- **P** : `Accept-Encoding` retiré par le proxy ; listes `agents`/`skills` du plugin jamais `null` ; tag `integration && e2e` ; libellé traduit ; `oh doctor` sans vérification v5 pour V1, et vérification git informative ; CHANGELOG corrigé.
- **D** : job CI `go-cli-integration` (macOS, `brew install opencode`, car le paquet npm `opencode-ai` ne publie pas V2) ; section SECURITY (en/fr) ; guide `docs/guides/sessions-v5.{en,fr}.md`.
- **Constat pendant la recette** : quand l'agent d'entrée n'a pas de modèle (`developer`), opencode V2 + Bedrock prenait **son propre modèle par défaut** (`us.openai.gpt-6.1-sol`), refusé par Bedrock en `eu-west-1`. **Correction** : le paquet de session reçoit toujours un modèle par défaut (surcharges, sinon `claude-sonnet-4-6`).
- Validé en réel (environnement `ohsmoke-e9`) : lancement de `developer`, coût suivi en direct, région `eu-west-1` issue du profil AWS, nouveau format de clé de groupe, `oh.db*` en 0600, arrêt propre, `oh doctor`.

### Fusion (05/10/2026)

- Recette manuelle faite par l'utilisateur dans l'environnement `ohsmoke-e9`. Pour ouvrir la TUI sans l'assistant de première configuration, il faut `[cli] setup_done = true` dans `hub.toml`, ou lancer `oh -p smoke`.
- `feat/v5-phase0` est fusionnée dans `main` (`--no-ff`, commit `d404c012`), puis la branche est supprimée en local (elle n'a jamais été poussée). Le build et les tests unitaires passent après la fusion.
- `main` n'est **pas encore poussé**.

