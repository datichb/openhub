# ADR-036 — Couche d'abstraction plateforme

## Statut

Accepté (Phases 0-4 implémentées, corrections Lot 1 appliquées, préparation structurelle Lot 2 faite)

## Date

2026-09-25

## Contexte

Le CLI du hub (`oh`) est fortement couplé à OpenCode en tant que seul runtime
de sessions de coding IA. Un audit exhaustif a identifié **~120 points de
couplage direct** répartis sur 23 fichiers :

- **`internal/opencode/`** (adaptateur attendu) — ~30 références à la gestion
  du binaire, aux arguments CLI, au schéma SQLite, aux chemins filesystem.
- **`cmd/`** (16 fichiers) — ~50 imports directs de `internal/opencode` pour
  les stats, les vérifications de version, le téléchargement et l'exécution.
- **`internal/`** hors adaptateur (7 packages) — ~40 fuites de références dans
  `launcher`, `llm`, `parallel`, `deploy`, `worktree`, `plugin`, `tui`, `config`.

Ce couplage crée trois problèmes :

1. **Pas de flexibilité backend** — ajouter un autre assistant de coding (Aider,
   Cursor CLI) ou un mode API LLM directe (Anthropic, Bedrock) nécessite de
   modifier des dizaines de fichiers qui devraient être agnostiques du backend.

2. **Fragmentation du modèle de données** — deux bases SQLite indépendantes
   (`oh.db` et `opencode.db`) tracent les mêmes sessions avec des IDs différents
   et sans cross-référence. Les tokens sont toujours à zéro dans `oh.db` car
   l'enrichissement post-exécution n'existe pas. Le dashboard `oh serve` expose
   deux endpoints sessions incompatibles.

3. **Architecture basée sur le polling** — le coordinateur parallèle interroge
   l'API HTTP d'OpenCode toutes les 5 secondes (180 requêtes/minute pour 5
   sessions) sans consommer les flux d'événements. Le broadcaster SSE du
   dashboard ajoute une couche de polling supplémentaire.

La seule abstraction existante est `llm.Completer` (`internal/llm/llm.go`),
explicitement conçue pour plusieurs backends mais couvrant uniquement la
complétion headless/batch. Les sessions interactives, le mode serve, les stats
et les session guards n'ont pas d'interface.

## Décision

Nous introduisons un **package `platform`** (`internal/platform/`) définissant
trois interfaces qui abstraient le backend de session IA.

### 1. `SessionPlatform` — Abstraction d'exécution

Couvre le cycle de vie complet des sessions : exécutions interactives, batch
headless, remplacement de processus (resume), session guard et factory du
mode serve.

```go
type SessionPlatform interface {
    Name() Name
    Available() bool
    Version() (string, error)

    RunInteractive(ctx context.Context, opts RunOpts) (*RunResult, error)
    ExecReplace(opts RunOpts) error
    RunHeadless(ctx context.Context, opts HeadlessOpts) (*HeadlessResult, error)

    FindActiveSessions(ctx context.Context, path string) ([]ActiveSession, error)
    IsGhostSession(s ActiveSession) bool

    SupportsServeMode() bool
    NewServer(port int, dir string, id string) (SessionServer, error)
    RequiresDeploy() bool
}
```

`RunResult` retourne des données structurées post-session (tokens, coût, modèle,
ID de session externe) — élimine le problème des « tokens toujours à zéro ».

### 2. `SessionServer` — Abstraction du mode serve

Couvre la surface API HTTP parallèle/serve : CRUD de sessions, envoi de prompts,
polling de statut, suivi des fichiers modifiés et gestion du cycle de vie.

```go
type SessionServer interface {
    Start(ctx context.Context) error
    WaitReady(ctx context.Context, timeout time.Duration) error
    IsAlive() bool
    Dispose() error
    Kill()

    CreateSession(title string) (string, error)
    SendPrompt(sessionID, prompt, agent string) error
    GetStatus() (map[string]string, error)
    GetModifiedFiles() ([]FileChange, error)
    AbortSession(sessionID string) error

    TicketID() string
    Port() int
    Dir() string
}
```

`FileChange` inclut un champ `Operation` (`"created"`, `"modified"`,
`"deleted"`) qui corrige la limitation actuelle où tous les fichiers sont
traités comme modifiés.

### 3. `StatsProvider` — Abstraction des métriques

Couvre les stats agrégées, par projet, l'historique de sessions et les
graphiques de coûts journaliers. Actuellement alimenté par lecture du SQLite
d'OpenCode ; les futurs backends pourront utiliser des enregistrements internes
ou des APIs distantes.

```go
type StatsProvider interface {
    Available() bool
    AggregateStats(period string) (*AggregateStats, error)
    ProjectStats(projectPath, period string) (*AggregateStats, error)
    RecentSessions(limit int) ([]SessionStat, error)
    ProjectSessions(projectPath string, limit int) ([]SessionStat, error)
    DailyCosts(period string) ([]DayCost, error)
}
```

### 4. Injection via le conteneur `App`

Deux nouveaux champs dans `app.App` :

- `Platform platform.SessionPlatform`
- `Stats platform.StatsProvider`

Câblés dans `cmd/root.go` lors de `initApp()`. Tous les consommateurs utilisent
`app.Platform.*` et `app.Stats.*` au lieu d'importer `internal/opencode`.

### 5. Évolution du schéma de base de données

La migration v24 ajoute des colonnes d'enrichissement agnostiques dans la table
`sessions` :

| Colonne | Type | Rôle |
|---------|------|------|
| `cost` | REAL | Coût de la session remonté par le backend |
| `tokens_reasoning` | INTEGER | Tokens de raisonnement |
| `tokens_cache_read` | INTEGER | Tokens lus depuis le cache |
| `platform` | TEXT | Nom du backend (`"opencode"`, `"directllm"`, ...) |
| `external_session_id` | TEXT | ID de session côté backend (cross-référence) |
| `slug` | TEXT | Identifiant humain de la session |

### 6. Migration incrémentale (5 phases)

| Phase | Périmètre | Effort | Risque |
|-------|-----------|--------|--------|
| 0 | Définir interfaces + types dans `internal/platform/` ; ajouter champs à `App` | 0.5j | Aucun |
| 1 | Adaptateur `StatsProvider` pour OpenCode ; migrer `cmd/metrics`, `cmd/serve_api`, dashboard, TUI | 1.5j | Faible |
| 2 | Adaptateur `SessionPlatform` pour OpenCode ; migrer `launcher`, `llm/`, `cmd/start` ; migration DB v24 ; enrichissement post-run | 3j | Moyen |
| 3 | Adaptateur `SessionServer` pour OpenCode ; migrer `parallel/coordinator` | 2.5j | Moyen |
| 4 | Nettoyage des fuites restantes : doctor, status, vues TUI, restructuration config, deploy | 3j | Faible |

Chaque phase laisse le système fonctionnel. La Phase 0 ne change aucun comportement.

### 7. Futurs backends

Trois catégories de backends sont anticipées :

| Type de backend | Exemple | SessionPlatform | SessionServer | StatsProvider | Deploy |
|---|---|---|---|---|---|
| Assistant de coding externe | OpenCode, Aider | Complet | Si mode serve disponible | Lecture depuis la DB de l'assistant | Oui (config assistant) |
| API LLM directe | Anthropic, Bedrock | Headless uniquement | Non | Enregistrements internes | Non |
| Moteur interne | Runtime hub custom | Complet | Optionnel | Enregistrements internes | Optionnel |

## Conséquences

### Positives

- **Hub agnostique du backend** : les nouvelles plateformes s'ajoutent en
  implémentant `SessionPlatform` dans un nouveau package. Aucune modification
  de `launcher`, `parallel`, `cmd/` ou `tui/` nécessaire.
- **Tokens/coût toujours renseignés** : `RunResult` retourne des données
  structurées, éliminant le problème des tokens à zéro dans `oh.db` et les
  événements team-state.
- **Vision unifiée des sessions** : le dashboard sert un seul endpoint sessions
  enrichi au lieu de deux incompatibles (`/api/v1/sessions` vs
  `/api/v1/opencode/sessions`).
- **Testabilité** : `SessionPlatform` et `StatsProvider` sont des interfaces —
  tous les consommateurs peuvent être testés avec des mocks. Actuellement,
  tester le launcher nécessite un vrai binaire OpenCode.
- **Suit les patterns existants** : `domain.SessionStore`, `domain.ProjectStore`,
  `tracker.Tracker` utilisent déjà le même pattern interface+adaptateur.
- **Livraison incrémentale** : chaque phase est déployable indépendamment avec
  zéro risque de régression en Phase 0.

### Négatives / Compromis

- **Coût initial** : ~10.5 jours développeur sur 5 phases pour un refactoring
  qui n'ajoute pas de fonctionnalités visibles (la valeur est architecturale).
- **Abstraction avant le second backend** : les interfaces sont conçues depuis
  la forme de l'adaptateur OpenCode. Un second backend peut révéler des
  incompatibilités nécessitant des ajustements d'interface.
- **Le deploy reste fortement couplé** : le layout filesystem `.opencode/` et
  le format `opencode.json` sont profondément ancrés dans `deploy/` et
  `worktree/`. L'abstraction complète du deploy est différée —
  `RequiresDeploy()` le conditionne pour l'instant.
- **Migration de configuration** : restructurer `[opencode]` en
  `[platforms.opencode]` dans `hub.toml` nécessite un chemin de migration
  rétrocompatible.

## Alternatives rejetées

| Alternative | Raison du rejet |
|-------------|----------------|
| Étendre uniquement `llm.Completer` (interface existante) | `Completer` ne couvre que les runs headless/batch. Les sessions interactives, le mode serve, les stats et les session guards sont des préoccupations distinctes nécessitant leurs propres abstractions. |
| Interface unique `CodingPlatform` regroupant toutes les méthodes | Trop large (20+ méthodes), viole le principe de ségrégation des interfaces. Le découpage en `SessionPlatform`, `SessionServer` et `StatsProvider` permet aux backends de n'implémenter que ce qu'ils supportent. |
| Bus d'événements au lieu d'interfaces | Sur-ingénierie pour la situation actuelle à 1 backend. Les interfaces sont plus simples, testables et suffisantes. Un bus peut être ajouté par-dessus plus tard. |
| Ne rien faire — refactorer quand un second backend sera réellement nécessaire | La fragmentation du modèle de données (tokens à zéro, dual-DB, ghost sessions) doit être corrigée quoi qu'il arrive. Corriger ces problèmes sans la couche d'abstraction créerait du code jetable. |

## Addendum : ParallelRunner et parallèle multi-backend (Lot 2)

Après l'implémentation des Phases 0-4, une analyse concurrentielle des coding
agents (OpenCode, Claude Code, Cline, Aider, Mammouth Code) a révélé que
l'interface `SessionServer` était trop couplée au mode serve HTTP d'OpenCode.
Aucun autre backend n'offre d'API HTTP équivalente. Cependant, l'exécution
parallèle est réalisable via différents mécanismes selon le backend :

| Backend | Mécanisme parallèle |
|---------|-------------------|
| OpenCode | API HTTP `opencode serve` (actuel) |
| Claude Code | Daemon `claude --bg` + CLI `claude agents --json` |
| Cline | Coordination multi-agent `cline --team-name` |
| Aider / API directe | N process headless concurrents |

### Nouvelles interfaces (Lot 2, implémenté)

**`ParallelRunner`** (`platform/parallel.go`) — orchestration haut-niveau de
tâches concurrentes. Agnostique du backend : fonctionne avec des serveurs HTTP,
des daemons CLI ou des process headless. Méthodes : `LaunchTask`,
`GetAllStatuses`, `GetModifiedFiles`, `SendMessage`, `AbortTask`, `AttachTask`,
`Cleanup`.

**`EventSource`** (`platform/parallel.go`) — flux d'événements temps réel
optionnel. Quand un `ParallelRunner` implémente aussi `EventSource`, le
coordinateur utilise les événements au lieu du polling. L'endpoint SSE
d'OpenCode (`GET /event`) est le candidat principal.

**`Capabilities`** — remplace `SupportsServeMode() bool` par une struct
indiquant les fonctionnalités optionnelles supportées (`Parallel`, `Events`).

**`SessionPlatform.NewParallelRunner()`** — remplace `NewServer()`. Retourne un
`ParallelRunner` configuré pour la plateforme.

### Changements structurels

- `SessionServer` déplacé de `platform/` vers `parallel/` — c'est désormais un
  détail d'implémentation du chemin parallèle OpenCode, pas une interface
  publique de la plateforme.
- `FileChange` reste dans `platform/` comme type partagé.
- Le coordinateur reçoit une `ServerFactory` dans ses opts pour créer des
  serveurs sans importer `opencode/`. Pattern transitoire en attendant que le
  Lot 3 remplace les internes du coordinateur par `ParallelRunner`.

### Lot 3 (différé)

Refactorer le coordinateur pour dépendre de `ParallelRunner` au lieu de
`SessionServer`. Créer `OpenCodeParallelRunner` qui encapsule la logique HTTP
serve en interne. Sera déclenché quand un second backend nécessitera le
support parallèle.
