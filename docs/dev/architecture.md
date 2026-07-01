# Architecture du CLI oh — Guide pour contributeurs

> Ce document décrit l'architecture logicielle du CLI `oh`, ses principes
> directeurs, et les conventions à respecter lors de l'ajout de fonctionnalités.

---

## Principes directeurs

### 1. Dependency Rule (Clean Architecture)

Les dépendances vont **toujours vers l'intérieur** :

```
cmd/ → internal/app/ → internal/domain/
              ↓
       internal/storage/
       internal/tui/
```

- `domain/` n'importe **aucun** autre package interne (zéro dépendance infra)
- `storage/` implémente les interfaces définies dans `domain/`
- `cmd/` orchestre, ne contient pas de logique métier
- `tui/` dépend de `domain/` pour les types, jamais l'inverse

### 2. Interface-Driven Design

Les contrats sont définis comme interfaces dans `domain/`. Les implémentations concrètes vivent dans `storage/`. Cela permet :
- De tester avec des mocks sans toucher à SQLite ou au keychain
- De remplacer une implémentation sans modifier le code appelant
- De vérifier la conformité au compile-time avec `var _ Interface = (*Impl)(nil)`

### 3. Dependency Injection via App Factory

Toutes les dépendances sont injectées via `*app.App` (pattern Factory, similaire à `gh`). Les commandes reçoivent `App` et accèdent aux stores via ses champs.

### 4. Commandes Cobra minces

Les fonctions `RunE` dans `cmd/` ne font que :
1. Parser les flags
2. Appeler un service ou un store
3. Formater l'output (ou lancer un TUI)

La logique métier vit dans `internal/service/` (à venir) ou directement dans les stores pour les cas simples.

---

## Structure des packages

```
cli/
├── main.go                         # Point d'entrée unique, appelle cmd.Execute()
│
├── cmd/                            # Commandes Cobra (wiring + UI)
│   ├── root.go                     # Root command, PersistentPreRunE (init App)
│   ├── version.go                  # oh version
│   └── <group>/                    # Futures commandes groupées par domaine
│       ├── <group>.go              # Commande parent
│       └── <subcommand>.go         # Sous-commande
│
├── internal/
│   ├── app/                        # Factory / DI Container
│   │   └── app.go                  # struct App + constructeur New()
│   │
│   ├── domain/                     # Coeur métier (ZERO DEPS)
│   │   ├── errors.go               # Sentinel errors (ErrNotFound, etc.)
│   │   ├── project.go              # Entité Project + interface ProjectStore
│   │   ├── session.go              # Entité Session + interface SessionStore
│   │   └── secret.go               # Interface SecretStore
│   │
│   ├── service/                    # Cas d'usage / orchestration (à venir)
│   │   └── (project_service.go)    # Logique multi-store, validations complexes
│   │
│   ├── storage/                    # Implémentations des interfaces domain
│   │   ├── sqlite/
│   │   │   ├── store.go            # Connexion, migrations, WAL, FK
│   │   │   ├── project_store.go    # Implémente domain.ProjectStore
│   │   │   ├── session_store.go    # Implémente domain.SessionStore
│   │   │   └── *_test.go
│   │   └── keychain/
│   │       └── store.go            # Implémente domain.SecretStore (OS keyring)
│   │
│   ├── config/                     # Configuration TOML (Viper)
│   │   ├── config.go
│   │   └── config_test.go
│   │
│   ├── i18n/                       # Internationalisation
│   │   ├── i18n.go                 # T() / Tf() avec fallback
│   │   ├── i18n_test.go
│   │   └── locales/                # Fichiers JSON embedded (go:embed)
│   │       ├── en.json
│   │       └── fr.json
│   │
│   └── tui/                        # Couche présentation TUI (Bubbletea)
│       ├── common/
│       │   ├── styles.go           # Palette, styles Lipgloss partagés
│       │   └── keys.go             # Keybindings partagés (à venir)
│       ├── components/             # Composants réutilisables (à venir)
│       │   ├── spinner/
│       │   ├── picker/
│       │   ├── statusbar/
│       │   └── dialog/
│       └── views/                  # Vues plein-écran (à venir)
│           ├── board/
│           ├── dashboard/
│           └── projects/
│
├── Makefile                        # build, test, lint, install
└── .goreleaser.yml                 # Release multi-arch + Homebrew
```

---

## Flux de données typique

```
[User] → oh project list
         │
         ▼
    cmd/root.go          PersistentPreRunE → initApp()
         │                  ↓
         │               app.New() → config.Load() → i18n.SetLocale()
         │               sqlite.OpenDefault() → NewProjectStore()
         │               keychain.New()
         │               → *app.App{Projects, Sessions, Secrets, IO}
         │
         ▼
    cmd/project/list.go  RunE: app.Projects.List(status) → format output
         │
         ▼
    storage/sqlite/      ProjectStore.List() → SQL query → []domain.Project
         │
         ▼
    [stdout]             Rendered table / JSON
```

---

## Conventions de code

### Nommage

| Élément | Convention | Exemple |
|---------|-----------|---------|
| Package | court, singulier, lowercase | `config`, `domain`, `sqlite` |
| Interface | verbe-er ou nom abstrait | `ProjectStore`, `SecretStore` |
| Impl struct | nom concret | `sqlite.ProjectStore`, `keychain.Store` |
| Constructeur | `New` + type | `NewProjectStore(s *Store)` |
| Sentinel errors | `Err` + condition | `ErrNotFound`, `ErrAlreadyExists` |
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

1. Créer un package dans `cmd/<group>/`
2. Définir la commande avec `&cobra.Command{}`
3. Accéder aux stores via `App()` (fonction globale dans `cmd/root.go`)
4. Formater l'output vers `App().IO.Out`
5. Retourner les erreurs (jamais `os.Exit` dans un handler)

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

---

## Diagramme de dépendances

```
                    ┌───────────┐
                    │  main.go  │
                    └─────┬─────┘
                          │
                    ┌─────▼─────┐
                    │   cmd/    │  ← Cobra commands, mince
                    └─────┬─────┘
                          │
                    ┌─────▼─────┐
                    │   app/    │  ← Factory, DI wiring
                    └──┬──┬──┬──┘
                       │  │  │
          ┌────────────┘  │  └────────────┐
          ▼               ▼               ▼
    ┌───────────┐   ┌──────────┐   ┌───────────┐
    │ storage/  │   │  config/ │   │   tui/    │
    │  sqlite/  │   │          │   │           │
    │ keychain/ │   └──────────┘   └───────────┘
    └─────┬─────┘
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
| CLI | `cobra` | Commandes, flags, completion |
| Config | `viper` | TOML, env vars, defaults |
| TUI | `bubbletea` + `lipgloss` + `bubbles` + `huh` | Architecture Elm, composants |
| DB | `modernc.org/sqlite` | Pur Go, pas de CGO |
| Keychain | `go-keyring` | macOS/Windows/Linux |
| i18n | Embedded JSON + `go:embed` | Custom (léger, adapté) |
| Tests | `testing` + `testify` | Stdlib + assertions |
| Release | `goreleaser` | Multi-arch + Homebrew |

---

*Document mis à jour le 1er juillet 2026*
