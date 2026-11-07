# 03 — Architecture cible

## 1. Vue d'ensemble

```
 CLI (cobra, cli/cmd)      TUI (tview, cli/internal/tui/v2 + cli/cmd/tui_*.go)      oh serve (BL-3)
          │                              │                                              │
          └──────────────┬───────────────┴──────────────────────────────────────────────┘
                         ▼
 ┌────────────────────────────── Services (cli/internal/services/…) ─────────────────────────────┐
 │ WorkflowService    catalogue, résolution par couches, validation, brouillons, publication     │
 │ BundleService      compilation du paquet (agents, skills+requires, permissions, plugin)       │
 │ RunService         groupes de serveurs, sessions, lancement, attachement, veille, file        │
 │ SessionService     registre, suivi SSE, boîte « À traiter », décisions, consignes             │
 │ CheckpointService  machine à états par session, MCP workflow_*, politique (manuel/CI)          │
 │ CredentialProxy    proxy LLM (jeton par session, SigV4, budget, liste blanche)                │
 │ GatewayService     faux bd → passerelle Beads ; passerelle MCP HTTP (phase 4)                 │
 │ PreferenceService  épinglés, récents, réglages UI                                             │
 └───────────────────────────────────────────────┬───────────────────────────────────────────────┘
                                                 ▼
 ┌──────────────── Adaptateurs d'outil (cli/internal/adapters/<tool>) ───────────────┐
 │ opencode/v2   Render · Launch(serve/API/--server) · Attest · Capabilities · Events │
 │ opencode/v1   Render (V1 schema) · Launch (--agent, OPENCODE_DATA_HOME) · Attest   │
 │ (futur)       claudecode, maison…                                                   │
 └────────────────────────────────────────────────────────────────────────────────────┘
                                                 ▼
 ┌──────────────── Environnements d'exécution (cli/internal/runtime/…) ───────────────┐
 │ local          processus opencode serve sur la machine                              │
 │ container      image projet + conteneur par groupe (Colima/Podman/Docker CLI)       │
 │ remote         projet GitLab oh-runner (phase 5)                                    │
 └─────────────────────────────────────────────────────────────────────────────────────┘
```

Règle : `cmd/*` et `cmd/tui_*` n'appellent **que** les services. Les services n'appellent **que** des interfaces (adaptateur, runtime, stores). Les noms d'agents natifs d'un outil n'existent que dans son adaptateur.

## 2. Interfaces Go (cible)

> Noms indicatifs ; respecter les conventions du dépôt (contexte en premier argument, erreurs enveloppées `%w`).

### 2.1 Modèle neutre de session

```go
// package sessionspec
type SessionSpec struct {
    SessionID    string            // "ses_" + 26 car. (choisi par oh)
    GroupKey     GroupKey          // (WorkflowRef@hash, ProjectID, Runtime)
    Workflow     WorkflowRef       // id, couche, version, hash
    Bundle       BundleRef         // hash + chemin
    Location     string            // dossier de travail (base ou worktree)
    EntryAgent   string
    Mode         string            // manuel | semi-auto | auto
    Prompt       string            // prompt rendu (entrées délimitées, O11)
    Inputs       map[string]any
    Model        ModelRef          // résolu via cascade O9
    Provider     ProviderSpec      // id, région, baseURL proxy, type d'auth
    SessionEnv   map[string]string // S7 : variables propres à la session
    SessionRules []PermissionRule  // F26 : règles de session (checkpoints, skills)
    Instructions map[string]string // S8 (si supporté)
    Runtime      RuntimeKind       // local | container | remote
    Attach       AttachPref        // auto | iterm | terminal | tmux | browser | suspend | none
    Budget       *BudgetLimit      // I6 (optionnel)
}

type BundleSpec struct { // contenu compilé (indépendant de l'outil)
    Agents        []AgentDef        // id, description, mode, body, model, permissions neutres
    Skills        []SkillDef        // id, chemin du dossier (SKILL.md + annexes)
    MCP           []MCPServerDef
    Plugins       []PluginDef       // plugin oh + plugins déclarés (O5)
    SubagentGraph map[string][]string // appels autorisés (dérivés du graphe)
    MaxDepth      int               // O1
    CodeMode      bool              // CM
    Isolation     IsolationLevel    // strict | standard
}
```

### 2.2 Adaptateur d'outil

```go
// package adapters
type ToolAdapter interface {
    Name() string                          // "opencode-v2", "opencode-v1"
    Detect(ctx context.Context) (ToolInfo, error) // binaire, version, compat
    Capabilities() Capabilities            // Isolation(full|partial|none), Events, HeadlessDecisions, MultiLocation, PluginHooks
    Render(b BundleSpec, p ProviderSpec) (RenderedConfig, error) // env + fichiers à écrire dans le paquet
    StartServer(ctx context.Context, g ServerGroup) (ServerHandle, error) // serve + attente disponibilité
    Attest(ctx context.Context, h ServerHandle, b BundleSpec, loc string) (VisibilityReport, error) // D13
    CreateSession(ctx context.Context, h ServerHandle, s SessionSpec) error
    SendPrompt(ctx context.Context, h ServerHandle, sessionID, text string) error
    AttachCommand(h ServerHandle, sessionID string) (argv []string, env []string) // pour termlaunch / suspension
    Events(ctx context.Context, h ServerHandle) (<-chan ToolEvent, error) // S2 (avec resync)
    Pending(ctx context.Context, h ServerHandle, sessionID string) ([]PendingDecision, error)
    Reply(ctx context.Context, h ServerHandle, d DecisionReply) error // permission / formulaire
    Control(ctx context.Context, h ServerHandle, sessionID string, op ControlOp) error // interrupt, synthetic, switchModel, compact, fork
    Results(ctx context.Context, h ServerHandle, sessionID string) (SessionResult, error) // diff, coût, tokens
    Export(ctx context.Context, h ServerHandle, sessionID string) ([]byte, error) // phase 5
    Import(ctx context.Context, h ServerHandle, data []byte) (string, error)
    StopServer(ctx context.Context, h ServerHandle) error
}

type VisibilityReport struct {
    Agents, Skills, MCP []string
    Unexpected []string   // éléments hors paquet → échec si non vide
    Level IsolationLevel
}
```
- `opencode-v2` : implémentation complète (recettes `02` R1–R4).
- `opencode-v1` : `Render` au schéma V1, lancement `--agent`, `OPENCODE_DATA_HOME`, pas de décisions headless (Capabilities réduites → l'UI masque les actions non supportées).
- `platform.SessionPlatform` (ADR-036) est conservé comme façade de compatibilité le temps de la migration puis remplacé par `ToolAdapter` + `RunService`.

### 2.3 Environnement d'exécution

```go
type Runtime interface {
    Kind() RuntimeKind
    Available(ctx context.Context) (Availability, error) // ex : colima démarré, image en cache, runner configuré
    Prepare(ctx context.Context, g ServerGroup) (PreparedGroup, error) // image, montages, réseau
    Exec(ctx context.Context, pg PreparedGroup, argv []string, env []string) (Process, error)
    HostAddress() string // adresse de la machine vue de l'intérieur (proxy, passerelles)
    Teardown(ctx context.Context, pg PreparedGroup) error
}
```

### 2.4 Services (signatures principales)

```go
type WorkflowService interface {
    Catalog(ctx context.Context, c Context) ([]WorkflowSummary, error)          // toutes couches, statut, intégrité
    Resolve(ctx context.Context, c Context, id string, opt ResolveOpts) (*ResolvedWorkflow, error) // avec origine de chaque valeur
    Validate(ctx context.Context, wf *ResolvedWorkflow) []Diagnostic
    SaveDraft(ctx context.Context, c Context, layer Layer, raw []byte) (*Draft, error)
    Publish(ctx context.Context, c Context, draftID, message string) (*Publication, error) // O14
    History(ctx context.Context, c Context, id string) ([]Version, error)
    Restore(ctx context.Context, c Context, id string, version int) (*Publication, error)
    Impact(ctx context.Context, from, to *ResolvedWorkflow) ImpactReport // O12
}

type BundleService interface {
    Build(ctx context.Context, wf *ResolvedWorkflow, c Context) (*Bundle, error) // idempotent par hash
    Show(ctx context.Context, b *Bundle) BundleReport // agents, skills, budget tokens, permissions
}

type RunService interface {
    Plan(ctx context.Context, req RunRequest) (*RunPlan, error)   // fiche de lancement : récap, avertissements
    Start(ctx context.Context, plan *RunPlan) ([]SessionHandle, error) // multi-sélection → N sessions
    Attach(ctx context.Context, sessionID string, pref AttachPref) error
    Stop(ctx context.Context, sessionID string) error
    Resume(ctx context.Context, sessionID string) error
    Sleep(ctx context.Context, groupKey GroupKey) error       // I7
    OnQuit(ctx context.Context, policy QuitPolicy) error      // I7
}

type SessionService interface {
    List(ctx context.Context, f SessionFilter) ([]SessionView, error)
    Inbox(ctx context.Context) ([]PendingDecision, error)     // I3
    Decide(ctx context.Context, r DecisionReply) error
    Send(ctx context.Context, sessionID, text string) error   // S6
    Follow(ctx context.Context, sessionID string) (<-chan FeedItem, error) // flux en direct
    Subscribe(ctx context.Context) (<-chan SessionChange, error) // badge, notifications
}

type CheckpointService interface {
    State(sessionID string) CheckpointState
    OnEvent(ev ToolEvent)                 // permission.asked sur workflow_checkpoint → ⏸
    Policy(sessionID string) CheckpointPolicy // manuel / auto / defer / forbid (CI)
}
```

`RunRequest` = `{WorkflowID, Layer?, Inputs, Mode, Runtime, Location (base|worktree existant|nouveau), Tickets[] (multi), Attach, Draft bool}`.

## 3. Schéma YAML d'un workflow (`apiVersion: oh/v1`)

```yaml
apiVersion: oh/v1
kind: Workflow
id: ticket                       # kebab-case, unique par couche
version: 3                       # géré par la publication (ne pas éditer à la main)
category: develop                # develop | frame | quality | knowledge | other
description: Implémenter un ticket Beads déjà détaillé
extends: null                    # "hub:ticket" | "team:ticket" | null ; patch via champs ci-dessous
risk: write                      # read | write | publish
isolation: strict                # strict (exige Isolation=full) | standard
code_mode: false                 # CM
entry:
  agent: orchestrator-dev        # omis → conductor
inputs:
  ticket:  { type: beads-id, required: true, picker: { filter: ai-delegated, multi: true } }
  branch:  { type: string, default: "feat/{{ .ticket }}" }
  publish: { type: bool, default: false }
  type:    { type: enum, values: [security, performance, architecture], default: security }
prompt:
  template: prompts/ticket.md.tmpl   # Go text/template ; entrées injectées délimitées (O11)
agents:
  orchestrator-dev: { role: workflow }
  developer:        { role: workflow, after: cp-1 }
  reviewer:         { role: workflow, after: developer }
  documentarian:    { role: independent }
checkpoints:
  cp-1:
    label: Démarrer
    mode: { manuel: pause, semi-auto: auto, auto: auto }
    remote: auto                 # auto | defer | forbid
  cp-2:
    label: Commit ou correction
    mandatory: true
    mode: { manuel: pause, semi-auto: pause, auto: pause }
    remote: defer
modes: { default: semi-auto, allowed: [manuel, semi-auto] }
circuit_breaker: { max_consecutive_subagents: 12 }
models:                          # O9 (optionnel)
  default: amazon-bedrock/eu.anthropic.claude-sonnet-4-6
  agents: { reviewer: amazon-bedrock/eu.anthropic.claude-opus-4-6-v1 }
skills: { extra: [shared/beads-dev], deny: [] }
plugins: []                      # O5 (ex : context-mode)
mcp: [gitlab]
beads: { allow: [show, update, close, list, children] }
runtime: { default: local, allowed: [local, container, remote] }
outputs:                         # O7 (typés)
  - { id: branch, type: branch }
  - { id: mr, type: merge_request }
  - { id: tickets, type: beads-ids }
limits: { budget_usd: null }     # I6 (optionnel)
```

> **Schéma figé (P1-T01)** dans `cli/internal/workflow/schema.go` (type `Spec`, exemple de référence `cli/internal/workflow/testdata/spec_v1_full.yaml`). Écarts par rapport à l'exemple ci-dessus :
> - `label` (optionnel) et `description`, `label`/`help` des entrées, `label` des checkpoints et sorties : **texte simple ou carte par langue** (`{ fr: …, en: … }`, type `LocalizedText`).
> - `agents.<id>.calls: [..]` : délégations explicites (source du `SubagentGraph`) ; omis ⇒ dérivé des permissions `task` de l'agent, restreintes au workflow. `after` = verrou (checkpoint ou agent préalable), utilisé par le gating (P3-T04).
> - `agents.<id>.mode` (primary/subagent) optionnel ; `checkpoints.<id>.condition` (comportement `conditional`) et `disabled: true` (retrait en patch) ; comportements possibles : `pause|auto|skip|conditional`.
> - `prompt.text` (gabarit en ligne) en alternative à `prompt.template` (exactement un des deux).
> - `inputs.<id>.max_length` (troncature, O11) ; `picker.epic`.
> - `plugins` : chaîne ou `{ id, options }`. Sorties : `branch | merge_request | beads-ids | path`.
> - `inputs`, `agents`, `checkpoints` sont des **cartes ordonnées** (ordre de déclaration conservé : fiche de lancement, ordre des checkpoints).
> - Patch (`extends`) : scalaires/pointeurs remplacent si présents ; listes remplacées en bloc ; cartes fusionnées par clé (agent retiré par `role: disabled`, checkpoint par `disabled: true`, entrées non retirables) ; champs de sécurité : durcissement seulement.
> - **Ajouts additifs (piste E, 06/10)** : `risk: plan` (entre `read` et `write` : aucun fichier modifié, shell restreint, `beads.allow` obligatoire, écritures Beads permises sauf `delete`) ; `preconditions:` (carte ordonnée `id → { label, check: { path_exists: [..] }, on_fail: suggest|block, suggest: { workflow, resume }, disabled }`, fusionnée par clé en patch, non durcie). Évaluateur : `workflow.EvaluatePreconditions(spec, fs.FS)`.
> - **Ajout additif (piste P2, 06/10, fusionné `53955aa9`)** : `enforce: [<champ de premier niveau>…]` ou `["*"]` — verrous pour les couches plus spécifiques : un document qui étend et **écrit** un champ verrouillé ⇒ erreur `enforced_field` ; verrous cumulés le long de la chaîne, non retirables (`workflow.EnforceableFields`, `06-phase-2.md` § Écarts).
> - Ajouts futurs : **uniquement additifs** (nouveau champ optionnel, nouvelle valeur d'énumération). Tout changement incompatible passe par une discussion explicite (toutes les pistes en dépendent).

### 3.1 Types d'entrées
`string`, `text`, `bool`, `int`, `enum`, `path`, `branch`, `beads-id` (simple/multi, filtres `ai-delegated`, `epic`), `beads-ids`, plus tard `tracker-issue`.

### 3.2 Règles de validation (WorkflowService.Validate)
- Schéma (types, champs inconnus refusés).
- Agents référencés existent dans le catalogue (hub / équipe / communauté).
- Graphe sans cycle ; `entry.agent` primaire ; tous les agents `workflow` atteignables.
- `risk: read` ⇒ aucun agent avec `edit`/`shell` en écriture ; `beads.allow` sans `create/update/close`.
- `remote` autorisé ⇒ aucun checkpoint obligatoire `remote: forbid`, et tous les `pause` ont `remote: auto|defer`.
- Fermeture des skills (`requires:`) résoluble, **identifiants uniques** (en V2 : nom de dossier).
- Variables du gabarit de prompt ⊆ entrées déclarées.
- `isolation: strict` ⇒ l'adaptateur courant doit annoncer `Isolation=full` (sinon erreur au lancement, avertissement à la validation).

### 3.3 Workflows livrés par le hub

| id | entrée | agents | risk | runtime | remplace |
|---|---|---|---|---|---|
| `feature` | orchestrator (dédié) | pathfinder, planner, designer, orchestrator-dev, developer*, reviewer, documentarian | write | local | `oh start` |
| `ticket` | orchestrator-dev | developer*, reviewer, documentarian | write | local, container, remote | `--dev`, `-t` |
| `cadrage` | conductor | pathfinder, planner, designer | plan (création Beads) | local | mode A sans dev |
| `onboarding` | onboarder | brief-enricher | write (`docs/wiki/` seulement) | local, remote | `--onboard` |
| `review` | reviewer | — | read | local, remote | `oh review` |
| `review-feedback` | orchestrator-dev | developer | write | local | `review feedback` |
| `audit` | auditor | auditor-subagent | read | local, remote | `oh audit` |
| `debug` | debugger | developer (opt.) | write | local | `oh debug` |
| `quick` | conductor | — | write | local | `--quick` |
| `sweep` | conductor | developer* | write | local | `--sweep` |
| `brief-enrich` | brief-enricher (headless) | — | read | local | enrichissement des briefs |

## 4. Couches et résolution

```
hub (embarqué : cli/internal/hubcontent → ~/.oh/hub/workflows/*.yaml)
  ↓ extends / patch
team-state/workflows/published/<id>.yaml            (équipe ou solo)
  ↓
team-state/projects/<p>/workflows/published/<id>.yaml
  ↓ (optionnel, local, explicite)
team-state/…/drafts/<member>/<id>.yaml
  ↓
options de session (CLI/TUI), jamais persistées
```
- Enchaînement : la couche la plus spécifique gagne, sauf les champs verrouillés par une couche supérieure (`enforce:`, piste P2).
- Sécurité (`risk`, `isolation`, `runtime.allowed`, checkpoints `mandatory`, `beads.allow`, permissions) : **ne peut que se durcir** (réutiliser la logique de `teamstate/policies.go`, `enforcementStrictness`).
- Chaque valeur résolue porte son **origine** (affichée dans la TUI et `oh workflow show`).
- Seuls les fichiers `published` dont le hash ∈ `workflows.lock` sont chargés ; sinon avertissement + ignoré.

## 5. Arborescence team-state (ajouts)

```
team-state/
├── config.toml                         # existant ; [workflow] (unique) → migré vers workflows/
├── workflows/
│   ├── published/<id>.yaml
│   ├── drafts/<member>/<id>.yaml       # + <id>.prompt.md.tmpl (gabarit propre au brouillon)
│   ├── prompts/<id>.md.tmpl            # gabarits référencés
│   └── history/<id>/<version>.yaml     # + <version>.prompt.md.tmpl, <version>.lock.toml (restauration)
├── catalog/                            # briques d'équipe (optionnel, format du hub)
│   ├── agents/<family>/<id>.md
│   └── skills/<path>.md (+ skills/templates/… annexes)
├── projects/<p>/workflows/{published,drafts,prompts,history}/…
├── workflows.lock                      # TOML : [team.<id>] / [projects.<p>.<id>] version, hash, prompt_hash, published_by, published_at, message
└── projects/_team/events/…             # événements des workflows d'équipe (workflow.published / restored / archived)
```
Solo : `~/.oh/teams/<solo-id>/` (git init, sans remote) ; `oh team promote --remote <url>`.

## 6. Paquet de session (layout)

```
~/.oh/bundles/<bundle-hash>/            # immuable, partagé par les sessions d'un même groupe
├── bundle.json                         # BundleSpec sérialisé + provenance (workflow, couches, hash)
├── rendered/<adapter>/                 # sortie de ToolAdapter.Render
│   ├── config.json                     # → OPENCODE_CONFIG_CONTENT (ou fichier OPENCODE_CONFIG)
│   └── env.json                        # variables non secrètes
├── agents/<id>.md                      # corps assemblé (Bucket A inliné) — sans frontmatter hub
├── skills/<id>/SKILL.md (+ annexes, templates)
└── plugin/                             # plugin oh (O6) : package.json + index.ts
~/.oh/sessions/<session-id>/
├── session.json                        # SessionSpec (sans secrets), état, liens
├── journal.jsonl                       # événements utiles (checkpoints, décisions)
└── results/                            # diff, récap, export (distant)
~/.oh/servers/<group-key>/
├── data/                               # XDG_DATA_HOME du groupe (opencode.db…)
└── server.json                         # pid, port, mot de passe chiffré, adaptateur, version, runtime
```
Hash du paquet = SHA-256 de `bundle.json` canonique + contenu des fichiers. Purge : paquets non référencés depuis 30 j.

## 7. Base de données `oh.db` (migrations)

> ⚠ Table **historique** (numérotation indicative de l'étude, dépassée) : les numéros réels et leur état sont dans `00-INDEX.md` § « Migrations SQLite réservées » (ex. `servers` = v28, `pending_decisions` = v33, `preferences` = v32, colonnes de workflow des sessions = v29/v34, `checkpoint_state` = v35, `projects.workflow_config` = v38).

Dernière migration existante à l'étude : **v27** (`storage/sqlite/store.go:391`). Nouvelles migrations (numérotation indicative) :

| Version | Contenu |
|---|---|
| v28 | Table `servers` : `group_key PK, adapter, adapter_version, runtime, pid, port, password_enc, data_dir, bundle_hash, status (starting/ready/sleeping/stopped), last_activity_at, created_at` |
| v29 | Colonnes `sessions` : `workflow_id, workflow_layer, workflow_version, bundle_hash, group_key, runtime, location, mode, state (preparing/active/waiting/idle/sleeping/completed/failed/stopped/archived), attached_clients, checkpoint_state JSON, outputs JSON, parent_session_id (O7), remote_ref` |
| v30 | Table `pending_decisions` : `id, session_id, kind (checkpoint/question/permission/budget/error), payload JSON, created_at, resolved_at, resolved_by (oh/tool/web/policy), resolution JSON` |
| v31 | Table `preferences` : `scope (global/project:<id>), key, value JSON` (épinglés, récents dérivés, réglages UI) |
| v32 | Neutraliser `projects.agents` (ignorée), puis suppression en v33 après une version |
| v33 | Supprimer `projects.agents` ; migrer `projects.workflow_config` → team-state (fichiers) puis neutraliser la colonne |

## 8. Proxy d'identifiants (CredentialProxy)
- Un listener `127.0.0.1:<port>` par machine (ou par groupe) ; routes par fournisseur.
- Jeton de session aléatoire (32 octets) transmis à opencode comme clé du fournisseur ; correspondance jeton → (session, fournisseur, modèles autorisés, budget).
- Remplacement d'en-tête (Bearer Bedrock, `x-api-key` Anthropic, Bearer OpenRouter) ou **signature SigV4** si la source est un profil AWS.
- Streaming sans tampon (`FlushInterval: -1`) ; comptage des tokens à partir des réponses (`usage`) pour budget/coût.
- Révocation à la fin de la session ; 401 sinon.
- Conteneur : écoute sur l'adresse joignable depuis la VM (Colima `host.docker.internal` / `host.lima.internal`, Podman `host.containers.internal`) ; filtrage par jeton.
- **Réalisé** (phase 0 et pistes D, G, H) : un seul listener par machine, hébergé par le démon `ohd` (port stable dans `ohd.json`, 2e écoute Linux), un jeton **par groupe de serveur**. Les mêmes écoutes servent aussi :
  - les **hooks** du plugin oh (`/oh/v1/hooks/*`, piste G), authentifiés par le jeton du proxy ;
  - la **passerelle MCP** (`/oh/v1/hooks/mcp/<nom>`, §10 et §11 bis) ;
  - la **passerelle Beads** (`/oh-gateway/beads/v1/exec`, §11), qui a ses propres jetons de session (`credproxy.Proxy.Mount`).
- **Durci** (piste R) : liste blanche des chemins d'inférence par fournisseur (`credproxy/routes.go`, un seul décodage, `.`/`..` refusés), 413 au-delà de 64 Mio, requête refusée si l'identifiant ne s'applique pas (signature dans le transport) ; clé JSON `model` exacte ; jetons connus par leur **empreinte** seulement (`credproxy.TokenHash`, base et registre des serveurs) ; restauration après l'ouverture du socket (cache SigV4 par profil/région, jetons orphelins révoqués) ; port changé ⇒ groupes concernés mis en veille (`servers/<groupe>/proxy_url`) ; usage compté aussi pour les flux OpenAI (`include_usage` ajouté) et Bedrock `invoke-with-response-stream` (base64), cumulé par groupe dans `usage_proxy` (v40).
- **Socket du démon** : identité du pair vérifiée (UID) ; routes qui donnent un accès ou arrêtent (`/grants*`, `/owners/*/grants`, `/shutdown`, `/proxy/listeners`, `/gateway/grants`) réservées à la CLI par une **capacité** (`X-Oh-Capability`, trousseau ou `~/.oh/run/capability` 0600). Windows : démon **dans le processus oh** (`daemon.InProcessMode`).
- **Restrictions I6** (`internal/limits`) : cascade hub → équipe (recommandé/imposé) → projet (`preferences`) → workflow (`limits:`) ; file des premiers prompts, budgets en USD sur le coût rapporté (registre v40, décision `$` en fin d'étape), plafond mémoire, liste de modèles sur le jeton du groupe.

## 9. Plugin oh (TypeScript embarqué)
- Source : `cli/internal/adapters/opencode/plugin/` (`index.ts`, `package.json`), `go:embed`, écrit dans `bundle/plugin/`.
- Export dual : `{ ...{ id: "oh", setup }, server() }` (V2 `setup`, V1 `server`).
- Fonctions :
  1. **Prompt ajouté** (O2) : `session.hook("context")` → `event.system.push({type:"text", text: <corps de l'agent courant>})` (corps lu depuis `bundle/agents/<id>.md`).
  2. **Monde fermé** (D13) : `agent.transform` → retirer tout agent hors paquet (sauf système) ; `skill.transform` → retirer toute skill hors paquet.
  3. **Checkpoints niveau 3** (D8) : `permission.hook("evaluate")` → interroge le `CheckpointService` d'oh (HTTP local, jeton de session) pour décider allow/ask/deny + message.
  4. Télémétrie minimale si le flux SSE ne suffit pas.
- Aucune logique métier : toute décision vient d'oh.

## 10. Outil MCP `workflow` (servi par oh)
- Lancé comme serveur MCP local (stdio `{{oh.bin}} mcp serve workflow`, la session appelante lue dans le `_meta` de l'appel) ; en conteneur, servi en HTTP par la passerelle MCP (§11 bis), sur la machine.
- Outils : `workflow_status` (étape courante, checkpoints, mode) ; `workflow_checkpoint {id, summary}` (permission `ask` générée → décision humaine via oh ou l'outil) ; `workflow_outputs {type, value}` (déclarer une sortie typée, O7).

## 11. Passerelle Beads (phase 4) — réalisée (piste H)
- **Faux `bd`** (`cmd/oh-bd`, Go, `linux/{amd64,arm64}`, embarqué dans l'image) :
  - envoie `argv`, le dossier courant et stdin (uniquement pour `--stdin` ou `-`) en `POST $OH_GATEWAY_URL/beads/v1/exec`, avec `Authorization: Bearer $OH_GATEWAY_TOKEN` ;
  - relaie stdout, stderr et le code retour.
- **GatewayService** (`internal/gateway`, dans le démon, sur les écoutes du proxy, préfixe `/oh-gateway`) :
  - liste blanche = `beads.allow` du workflow (`StartRequest.BeadsAllow`). Sans bloc `beads:`, lecture seule : `show, list, ready, search, children, comments, count, status, graph, history`. Une liste vide refuse tout ;
  - options globales qui changent de base ou de dossier refusées partout dans la commande ;
  - traduction conteneur → machine (`PathMap.ToHost`) du dossier courant et des chemins ; fichiers limités aux locations de la session ;
  - vrai `bd` lancé sans shell, dans la location de la session (ou le sous-dossier traduit).
- **Jetons** `ohg_…` :
  - un par session et par sous-session, transmis par l'**environnement de session** (`runsvc.Service.SessionEnv`, S7) ;
  - le démon n'écrit que leur empreinte (`~/.oh/run/gateway.json`) ;
  - valides tant que la session est ouverte et éveillée ; révoqués avec le groupe.
  - opencode 2.0.20 ne transmet ni l'environnement du serveur au shell de session, ni l'environnement de session aux sous-agents : c'est le démon qui l'applique à chaque sous-session.
- **Distant** : mode **journal** (lecture depuis l'instantané, écritures dans `journal.jsonl`), prévu dans le faux `bd` (`OH_BD_MODE=journal`, `beadswire.JournalEntry`), à implémenter en P5-T12.

## 11 bis. Passerelle MCP (phase 4) — réalisée (piste H)
- **Serveurs concernés** : ceux du paquet de forme `oh mcp serve <nom>` (`sessionspec.OhMCPName`). Pour un runtime hors machine, l'adaptateur les déclare `type: remote` : URL `…/oh/v1/hooks/mcp/<nom>`, en-tête `Authorization: Bearer {env:<variable du jeton du proxy>}`, développé par opencode. Le paquet et son hash ne changent pas.
- **Côté démon** : il lance sur la machine la commande du serveur, relue dans le paquet immuable avec l'exécutable oh courant. Le serveur lit lui-même ses jetons dans le trousseau.
- **Relais** : JSON-RPC en « streamable HTTP » avec réponses JSON, identifiants réécrits (plusieurs clients par processus). Un processus par (groupe, serveur), arrêté avec le groupe.
- **Contrôle** : refus des appels dont le `_meta` désigne une session d'un autre groupe.
