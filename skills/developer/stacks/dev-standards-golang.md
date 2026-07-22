---
name: dev-standards-golang
description: Standards Go — idiomes, gestion des erreurs, interfaces, concurrence, tests, performance, sécurité. S'applique à tout projet avec un go.mod détecté.
---

# Skill — Standards Go

## Rôle

Ce skill définit les conventions Go à respecter sur les projets Go.
Il complète `dev-standards-universal.md` et s'active automatiquement
dès que `go.mod` est détecté dans le projet.

---

## Idiomes fondamentaux

### Gestion des erreurs

- Toujours vérifier les erreurs — ne jamais ignorer silencieusement
- Wrapper avec contexte : `fmt.Errorf("operation: %w", err)` pour préserver la chaîne
- Sentinel errors pour les conditions domaine : `var ErrNotFound = errors.New("not found")`
- Vérifier avec `errors.Is()` et `errors.As()` — jamais comparer directement
- Pas de panic dans les bibliothèques — réserver aux erreurs de programmation irrecupérables
- Recover uniquement en point d'entrée (main, handler HTTP) avec log structuré

### Interfaces

- Définir les interfaces côté consommateur, pas côté producteur
- Interfaces petites et ciblées (1-3 méthodes) — composition plutôt qu'héritage
- `interface{}` / `any` uniquement si le type est réellement indéterminé
- Vérification compile-time : `var _ MonInterface = (*MonType)(nil)`
- Préférer accepter des interfaces et retourner des types concrets

### Nommage

- Packages : noms courts, lowercase, sans underscore ni camelCase (`httputil` pas `httpUtil`)
- Exportés : PascalCase — non-exportés : camelCase
- Acronymes en majuscules si exportés : `URLParser`, `HTTPClient`, `ID`
- Receivers : 1-2 lettres représentant le type — consistant dans tout le type
- Éviter les noms redondants avec le package : `http.Server` pas `http.HTTPServer`

### Structures et méthodes

- Zero value utilisable quand possible — documenter si ce n'est pas le cas
- Constructeurs `New*` qui retournent une valeur prête à l'emploi
- Méthodes sur valeur si pas de mutation — sur pointeur si mutation ou grosse struct
- Embedding pour la composition — éviter l'embedding d'interfaces dans des structs concrètes

---

## Concurrence

### Goroutines

- Toujours documenter la durée de vie d'une goroutine
- Toujours propaguer un `context.Context` pour permettre l'annulation
- Signaler la fin via `sync.WaitGroup`, channel fermé, ou context cancel
- Éviter les goroutines de durée infinie sans mécanisme d'arrêt

### Synchronisation

- `sync.Mutex` / `sync.RWMutex` pour les états partagés
- `sync.Once` pour l'initialisation singleton (thread-safe)
- Channels pour la communication — mutexes pour la protection d'état
- Éviter `sync.Map` sauf si mesure de performance le justifie
- Race detector obligatoire en CI : `go test -race`

### Patterns

```go
// Worker avec context et WaitGroup
func worker(ctx context.Context, jobs <-chan Job) {
    for {
        select {
        case <-ctx.Done():
            return
        case j, ok := <-jobs:
            if !ok {
                return
            }
            process(j)
        }
    }
}
```

---

## Organisation du code

### Structure de projet

```
cmd/          → points d'entrée (main packages)
internal/     → code privé non importable de l'extérieur
  domain/     → entités et interfaces (zéro import infra)
  service/    → logique métier
  store/      → implémentations persistence
pkg/          → code public réutilisable (si bibliothèque)
```

- `internal/` pour tout ce qui n'est pas une API publique intentionnelle
- Pas de package `utils` ou `helpers` — nommer par responsabilité
- Dépendances circulaires interdites — architecture en couches

### Dependency Rule

```
cmd → internal/service → internal/domain (interfaces)
                      ↑
         internal/store (implementations)
```

Le domaine n'importe jamais les packages infrastructure.

---

## Tests

### Conventions

- Fichiers `_test.go` dans le même package (boîte blanche) ou `_test` suffix (boîte noire)
- Table-driven tests par défaut :
```go
tests := []struct {
    name  string
    input string
    want  string
}{
    {"cas nominal", "input", "output"},
}
for _, tt := range tests {
    t.Run(tt.name, func(t *testing.T) {
        got := maFonction(tt.input)
        assert.Equal(t, tt.want, got)
    })
}
```
- `t.Helper()` dans les helpers de test
- `t.TempDir()` pour les fichiers temporaires — nettoyage automatique
- Interfaces pour mocker les dépendances — pas de framework de mock lourd
- `-short` flag pour les tests rapides en CI

### Couverture

- `go test ./... -coverprofile=coverage.out`
- `go tool cover -func=coverage.out` pour le résumé
- Priorité aux chemins critiques — pas de couverture cosmétique

---

## Performance

### Règles de base

- Mesurer avant d'optimiser — `pprof`, `go test -bench`
- Escape analysis : `go build -gcflags="-m"` pour voir les allocations heap
- Réutiliser les buffers avec `sync.Pool` pour les allocations fréquentes
- Préférer `strings.Builder` à la concaténation de strings
- `append` avec capacité pré-allouée si la taille est connue : `make([]T, 0, n)`

### Benchmarks

```go
func BenchmarkMaFonction(b *testing.B) {
    b.ReportAllocs()
    for i := 0; i < b.N; i++ {
        maFonction(input)
    }
}
```

---

## Sécurité

- Pas de `exec.Command` avec input utilisateur non validé
- SQL : uniquement des requêtes paramétrées — jamais de concaténation
- Secrets via variables d'environnement ou keychain — jamais dans le code
- `crypto/rand` pour la génération aléatoire cryptographique — jamais `math/rand`
- Timeouts sur tous les clients HTTP : `&http.Client{Timeout: 30 * time.Second}`
- `io.LimitReader` sur toutes les lectures réseau non bornées

---

## Outils obligatoires

| Outil | Usage | Commande |
|-------|-------|----------|
| `go vet` | Analyse statique | `go vet ./...` |
| `golangci-lint` | Linter multi-règles | `golangci-lint run ./...` |
| `go test -race` | Race detector | `go test ./... -race` |
| `govulncheck` | CVE scanner | `govulncheck ./...` |
| `gofmt` / `goimports` | Formatage | `gofmt -s -w .` |

### Configuration golangci-lint recommandée

```yaml
linters:
  enable:
    - bodyclose
    - gocritic
    - nilerr
    - misspell
    - errcheck
    - gosec
    - ineffassign
    - unused
```

---

## Ce que tu NE fais PAS

- Pas d'`init()` avec effets de bord complexes
- Pas de variables globales mutables (sauf registres thread-safe)
- Pas de `log.Fatal` / `log.Panic` dans les packages — uniquement dans `main`
- Pas de `time.Sleep` dans les tests — utiliser channels et contexts
- Pas de copie de `sync.Mutex` — passer par pointeur
- Pas d'assertion de type sans vérification : `v := x.(T)` → toujours `v, ok := x.(T)`
