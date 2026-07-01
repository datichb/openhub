# Architecture du CLI oh — Guide pour contributeurs

> Ce document décrit l'architecture logicielle du CLI `oh`, ses principes
> directeurs, et les conventions à respecter lors de l'ajout de fonctionnalités.
>
> Mis à jour le 1er juillet 2026 — reflète l'état post-Phase 7.

---

## Principes directeurs

### 1. Dependency Rule (Clean Architecture)

Les dépendances vont **toujours vers l'intérieur** :

```
cmd/ → internal/app/ → internal/domain/
              ↓
       internal/storage/
       internal/opencode/
       internal/prompt/
       internal/deploy/
       internal/mcp/
       internal/tui/
```

- `domain/` n'importe **aucun** autre package interne (zéro dépendance infra)
- `storage/` implémente les interfaces définies dans `domain/`
- `cmd/` orchestre, ne contient pas de logique métier
- `tui/` dépend de `domain/` pour les types, jamais l'inverse
- `mcp/` est autonome (protocole + handlers API)

### 2. Interface-Driven Design

Les contrats sont définis comme interfaces dans `domain/`. Les implémentations concrètes vivent dans `storage/`. Cela permet :
- De tester avec des mocks sans toucher à SQLite ou au keychain
- De remplacer une implémentation sans modifier le code appelant
- De vérifier la conformité au compile-time avec `var _ Interface = (*Impl)(nil)`

### 3. Dependency Injection via App Factory

Toutes les dépendances sont injectées via `*app.App` (pattern Factory, similaire à `gh`). Les commandes accèdent aux stores via `GetApp()` dans le package `cmd`.

### 4. Commandes Cobra minces

Les fonctions `RunE` dans `cmd/` ne font que :
1. Parser les flags
2. Appeler un module interne (store, service, deploy)
3. Formater l'output (ou lancer un TUI)

La logique métier vit dans les packages `internal/` dédiés.

---

## Structure des packages

```
cli/
├── main.go                              # Point d'entrée unique → cmd.Execute()
│
├── cmd/                                 # Commandes Cobra (30 commandes)
│   ├── root.go                          # Root command, PersistentPreRunE, GetApp(), initApp()
│   ├── version.go                       # oh version
│   ├── init.go                          # oh init (wizard Huh)
│   ├── status.go                        # oh status
│   ├── doctor.go                        # oh doctor
│   ├── start.go                         # oh start (résolution projet, exec opencode)
│   ├── quick.go                         # oh quick (sélection rapide)
│   ├── project.go                       # oh project (parent)
│   ├── project_list.go                  # oh project list
│   ├── project_add.go                   # oh project add (interactif ou flags)
│   ├── project_remove.go               # oh project remove
│   ├── config.go                        # oh config [get|set|list|path]
│   ├── deploy.go                        # oh deploy (transactionnel)
│   ├── sync.go                          # oh sync
│   ├── agent.go                         # oh agent [list]
│   ├── skills.go                        # oh skills [list]
│   ├── plugin.go                        # oh plugin [list]
│   ├── mcp.go                           # oh mcp [serve|list]
│   ├── board.go                         # oh board (lance TUI kanban)
│   ├── dashboard_cmd.go                 # oh dashboard (lance TUI dashboard)
│   ├── metrics.go                       # oh metrics
│   ├── optimize_yield.go               # oh optimize + oh yield
│   ├── worktree.go                      # oh worktree [list|add|remove]
│   ├── audit_review_debug.go           # oh audit, oh review, oh debug
│   ├── misc.go                          # oh conventions, beads, service, upgrade, update
│   └── completion.go                    # oh completion [bash|zsh|fish|powershell]
│
├── internal/
│   ├── app/                             # Factory / DI Container
│   │   └── app.go                       # struct App { Config, Projects, Sessions, Secrets, IO }
│   │
│   ├── domain/                          # Coeur métier (ZERO DEPS INFRA)
│   │   ├── errors.go                    # ErrNotFound, ErrAlreadyExists, ErrInvalidInput
│   │   ├── project.go                   # Entité Project + interface ProjectStore
│   │   ├── session.go                   # Entité Session + interface SessionStore
│   │   └── secret.go                    # Interface SecretStore
│   │
│   ├── storage/                         # Implémentations des interfaces domain
│   │   ├── sqlite/
│   │   │   ├── store.go                 # Connexion SQLite, WAL, FK, migrations
│   │   │   ├── project_store.go         # Implémente domain.ProjectStore (+ tests)
│   │   │   └── session_store.go         # Implémente domain.SessionStore (+ tests)
│   │   └── keychain/
│   │       └── store.go                 # Implémente domain.SecretStore (OS keyring)
│   │
│   ├── config/                          # Configuration TOML (Viper)
│   │   ├── config.go                    # Load(), Reset(), HubDir(), ConfigPath()
│   │   └── config_test.go
│   │
│   ├── i18n/                            # Internationalisation
│   │   ├── i18n.go                      # T(), Tf(), SetLocale() — embedded JSON
│   │   ├── i18n_test.go
│   │   └── locales/                     # go:embed
│   │       ├── en.json
│   │       └── fr.json
│   │
│   ├── opencode/                        # Gestion du binaire opencode
│   │   ├── opencode.go                  # FindBinary(), Exec(), Run(), Version()
│   │   └── opencode_test.go             # Tests args/env building, path expansion
│   │
│   ├── prompt/                          # Détection stack & construction contexte
│   │   ├── prompt.go                    # DetectStack(), BuildContext()
│   │   └── prompt_test.go              # Tests Go/TS/Python/Rust/Docker/CI/NextJS
│   │
│   ├── deploy/                          # Déploiement transactionnel
│   │   ├── deploy.go                    # Execute(), Snapshot, Rollback, Phases
│   │   └── deploy_test.go              # Tests full deploy, rollback, merge config
│   │
│   ├── mcp/                             # Serveurs MCP intégrés
│   │   ├── protocol/
│   │   │   └── server.go               # JSON-RPC stdio server (initialize, tools/list, tools/call)
│   │   ├── figma/
│   │   │   └── server.go               # figma_get_file, figma_get_node, figma_get_styles
│   │   ├── gitlab/
│   │   │   └── server.go               # gitlab_get_project, gitlab_list_issues, gitlab_list_mrs
│   │   └── gslides/
│   │       └── server.go               # gslides_get_presentation, gslides_get_slide
│   │
│   └── tui/                             # Couche présentation TUI (Bubbletea)
│       ├── common/
│       │   └── styles.go               # Palette couleurs, styles Lipgloss, icônes
│       ├── components/
│       │   └── picker/
│       │       ├── picker.go           # Picker full-screen (single/multi, filter, scroll)
│       │       └── picker_test.go      # 13 tests (navigation, sélection, filtre, scroll)
│       └── views/
│           ├── board/
│           │   └── board.go            # Kanban 4 colonnes, live refresh, cards
│           └── dashboard/
│               └── dashboard.go        # Multi-panneaux (projets, sessions, tokens)
│
├── Makefile                             # build, test, lint, clean, install, deps
└── .goreleaser.yml                      # Multi-arch + Homebrew tap auto
```

---

## Flux de données typique

### Commande simple (oh project list)

```
[User] → oh project list --status active
         │
         ▼
    cmd/root.go        PersistentPreRunE → initApp()
         │                ↓
         │             app.New() → config.Load() → i18n.SetLocale()
         │             sqlite.OpenDefault() → NewProjectStore(), NewSessionStore()
         │             keychain.New()
         │             → *app.App{Projects, Sessions, Secrets, IO}
         │
         ▼
    cmd/project_list.go  RunE: GetApp().Projects.List("active") → tabwriter → stdout
         │
         ▼
    storage/sqlite/      ProjectStore.List() → SQL → []domain.Project
         │
         ▼
    [stdout]             Table formatée
```

### Commande TUI (oh board)

```
[User] → oh board --watch
         │
         ▼
    cmd/board.go        fetchTickets() → bd list --json → []board.Ticket
         │
         ▼
    tui/views/board/    board.Run(Config{Tickets, RefreshFunc})
         │
         ▼
    Bubbletea           Init → EnterAltScreen + tickCmd
                        Update(tickMsg) → RefreshFunc() → re-render
                        Update(KeyMsg "q") → tea.Quit
         │
         ▼
    [terminal]           Kanban full-screen, live
```

### Commande exec (oh start)

```
[User] → oh start --agent coder
         │
         ▼
    cmd/start.go        resolveProject() → détection cwd ou picker
                        prompt.DetectStack() → StackInfo
                        secrets.Get("bedrock-token-*") → bearer token
         │
         ▼
    opencode/           opencode.Exec(StartOpts{Path, Agent, Token})
                        → os.Chdir(project.Path)
                        → syscall.Exec(opencode_binary, args, env)
         │
         ▼
    [opencode process]   Remplace le process oh (exec)
```

---

## Conventions de code

### Nommage

| Élément | Convention | Exemple |
|---------|-----------|---------|
| Package | court, singulier, lowercase | `config`, `domain`, `sqlite`, `figma` |
| Interface | nom abstrait | `ProjectStore`, `SecretStore` |
| Impl struct | nom concret | `sqlite.ProjectStore`, `keychain.Store` |
| Constructeur | `New` + type | `NewProjectStore(s *Store)` |
| Sentinel errors | `Err` + condition | `ErrNotFound`, `ErrAlreadyExists` |
| Fichiers cmd | `<domaine>.go` ou `<domaine>_<action>.go` | `project.go`, `project_add.go` |
| Test files | `*_test.go` dans le même package | `project_store_test.go` |

### Erreurs

```go
// Wrapping avec contexte
return fmt.Errorf("creating project %s: %w", p.ID, err)

// Sentinel errors pour les conditions attendues
if err == sql.ErrNoRows {
    return nil, domain.ErrNotFound
}

// Vérification côté appelant
if errors.Is(err, domain.ErrNotFound) {
    // handle gracefully
}
```

### Tests

- **Table-driven tests** pour les cas multiples
- **Fichiers `*_test.go`** dans le même package (test boîte blanche)
- **`testify`** pour assertions et require
- **`t.TempDir()`** pour les fichiers temporaires (auto-cleanup)
- **Helpers** avec `t.Helper()` pour les setup communs

### Ajouter une nouvelle commande

1. Créer un fichier dans `cmd/` (un fichier par commande ou groupe logique)
2. Définir la commande avec `&cobra.Command{}`
3. Enregistrer via `init()` : `rootCmd.AddCommand(...)` ou comme sous-commande
4. Accéder aux stores via `GetApp()` (nil-safe après PersistentPreRunE)
5. Formater l'output vers `GetApp().IO.Out`
6. Retourner les erreurs (jamais `os.Exit` dans un handler)

### Ajouter une nouvelle entité

1. Définir le type + interface dans `internal/domain/`
2. Créer l'implémentation dans `internal/storage/sqlite/` (ou autre)
3. Ajouter la migration SQL dans `store.go:migrate()`
4. Ajouter le champ dans `*app.App`
5. Wire dans `cmd/root.go:initApp()`
6. Compile-time check : `var _ domain.XxxStore = (*XxxStore)(nil)`

### Ajouter un composant TUI

1. Créer un package dans `internal/tui/components/<name>/`
2. Implémenter `tea.Model` (Init, Update, View)
3. Les composants reçoivent les données domain en entrée (pas les stores)
4. Les composants émettent des `tea.Msg` pour communiquer vers le parent
5. Exposer une fonction `Run(config) error` pour usage standalone

### Ajouter une vue TUI (plein écran)

1. Créer un package dans `internal/tui/views/<name>/`
2. Implémenter `tea.Model` avec `tea.EnterAltScreen` dans `Init()`
3. Gérer `tea.WindowSizeMsg` pour la responsivité
4. Exposer `Run(config) error` comme API publique
5. La commande `cmd/<name>.go` appelle `views.<name>.Run(cfg)`

### Ajouter un serveur MCP

1. Créer un package dans `internal/mcp/<name>/`
2. Implémenter une fonction `Serve() error`
3. Créer un `protocol.NewServer(name, version)`
4. Enregistrer les tools via `server.RegisterTool(tool, handler)`
5. Chaque handler reçoit `json.RawMessage` et retourne `*protocol.ToolResult`
6. Ajouter le case dans `cmd/mcp.go:mcpServeCmd()`

---

## Diagramme de dépendances

```
                         ┌───────────┐
                         │  main.go  │
                         └─────┬─────┘
                               │
                         ┌─────▼─────┐
                         │   cmd/    │  ← 30 commandes Cobra
                         └─────┬─────┘
                               │
                         ┌─────▼─────┐
                         │   app/    │  ← Factory, DI wiring
                         └──┬──┬──┬──┘
                            │  │  │
          ┌─────────────────┼──┼──┼─────────────────┐
          │                 │  │  │                  │
          ▼                 ▼  │  ▼                  ▼
    ┌───────────┐   ┌─────────┐│┌──────────┐  ┌──────────┐
    │ storage/  │   │ config/ │││  tui/    │  │opencode/ │
    │  sqlite/  │   └─────────┘│└──────────┘  └──────────┘
    │ keychain/ │              │
    └─────┬─────┘              │
          │                    ▼
          │            ┌──────────────┐
          │            │   deploy/    │
          │            │   prompt/    │
          │            │   mcp/       │
          │            └──────────────┘
          │
          │  implémente
          ▼
    ┌───────────┐
    │  domain/  │  ← Entités + Interfaces (ZERO DEPS)
    └───────────┘
```

**Règle : les flèches pointent toujours vers `domain/`. Aucun package n'importe vers le haut.**

---

## Stack technique

| Besoin | Package | Rôle |
|--------|---------|------|
| CLI | `github.com/spf13/cobra` | Commandes, flags, shell completion |
| Config | `github.com/spf13/viper` | TOML, env vars, defaults |
| TUI engine | `github.com/charmbracelet/bubbletea` | Architecture Elm, event loop |
| TUI styling | `github.com/charmbracelet/lipgloss` | CSS-like, tables, layouts |
| TUI forms | `github.com/charmbracelet/huh` | Wizards interactifs |
| DB | `modernc.org/sqlite` | Pur Go, pas de CGO, cross-compile trivial |
| Keychain | `github.com/zalando/go-keyring` | macOS/Windows/Linux |
| i18n | Embedded JSON + `go:embed` | Custom, léger |
| UUID | `github.com/google/uuid` | Génération IDs projets |
| Tests | `testing` + `github.com/stretchr/testify` | Stdlib + assertions |
| Release | `goreleaser` | Multi-arch + Homebrew tap |

---

*Document mis à jour le 1er juillet 2026 — Post-Phase 7*
