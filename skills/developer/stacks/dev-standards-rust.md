---
name: dev-standards-rust
description: Standards Rust — ownership, borrowing, gestion des erreurs, async/tokio, clippy, tests, sécurité mémoire. S'applique à tout projet avec un Cargo.toml détecté.
---

# Skill — Standards Rust

## Rôle

Ce skill définit les conventions Rust à respecter sur les projets Rust.
Il complète `dev-standards-universal.md` et s'active automatiquement
dès que `Cargo.toml` est détecté dans le projet.

---

## Ownership et borrowing

### Règles fondamentales

- Comprendre le modèle : chaque valeur a un unique propriétaire — la propriété se transfère ou s'emprunte
- Préférer les références (`&T`, `&mut T`) à la copie quand possible
- `Clone` explicitement si la copie est intentionnelle — pas de Clone implicite caché
- Lifetime annotations uniquement quand le compilateur ne peut pas les inférer

### Patterns recommandés

```rust
// Préférer l'emprunt
fn process(data: &[u8]) -> Result<usize, Error> { ... }

// Ownership pour les constructeurs et transformations
fn parse(input: String) -> Result<Config, Error> { ... }

// Arc<Mutex<T>> pour le partage concurrent
let shared = Arc::new(Mutex::new(state));
```

---

## Gestion des erreurs

### Approche recommandée

- `Result<T, E>` pour les erreurs récupérables — jamais `unwrap()` en production
- `panic!` uniquement pour les invariants impossibles (erreurs de programmation)
- Bibliothèques recommandées :
  - `thiserror` pour les types d'erreur personnalisés (bibliothèques)
  - `anyhow` pour la propagation d'erreurs dans les applications
- Opérateur `?` pour propager — éviter le chaînage `.unwrap_or_else`

```rust
// Avec thiserror
#[derive(Debug, thiserror::Error)]
pub enum AppError {
    #[error("resource not found: {0}")]
    NotFound(String),
    #[error("database error: {0}")]
    Database(#[from] sqlx::Error),
}

// Propagation propre
fn fetch_user(id: &str) -> Result<User, AppError> {
    let user = db.query(id)?;
    Ok(user)
}
```

---

## Organisation du code

### Structure Cargo workspace

```
my-project/
├── Cargo.toml          # workspace root
├── crates/
│   ├── core/           # logique métier (pas de dépendances externes)
│   ├── api/            # couche HTTP/gRPC
│   └── cli/            # point d'entrée CLI
└── tests/              # tests d'intégration
```

### Modules

- Fichier `mod.rs` ou `nom_module.rs` — choisir une convention et s'y tenir
- `pub` minimal — exposer uniquement ce qui est nécessaire
- `pub(crate)` pour la visibilité interne au crate
- Pas de code dans `lib.rs` au-delà des déclarations de modules et re-exports

---

## Async avec Tokio

### Conventions

- `async fn` uniquement si la fonction attend réellement une I/O
- Runtime Tokio configuré une seule fois dans `main` :
```rust
#[tokio::main]
async fn main() -> anyhow::Result<()> {
    // ...
    Ok(())
}
```
- `tokio::spawn` pour les tâches concurrentes — retourner le `JoinHandle`
- `tokio::select!` pour attendre plusieurs futures avec annulation
- `CancellationToken` (depuis `tokio-util`) pour la propagation d'annulation

### Pièges

- Pas d'opérations bloquantes dans un contexte async — utiliser `spawn_blocking`
- Pas de `Mutex` std dans du code async — utiliser `tokio::sync::Mutex`
- Attention aux tailles de futures sur la stack — `Box::pin` si nécessaire

---

## Tests

### Structure

```rust
// Tests unitaires — dans le même fichier
#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn test_nom_explicite() {
        // Arrange
        let input = ...;
        // Act
        let result = ma_fonction(input);
        // Assert
        assert_eq!(result, expected);
    }

    #[tokio::test]
    async fn test_async() { ... }
}

// Tests d'intégration — dans tests/
// tests/integration_test.rs
```

### Bonnes pratiques

- Nommer les tests de façon descriptive : `should_return_error_when_input_is_empty`
- `assert_eq!` / `assert!` pour les assertions simples
- `pretty_assertions` crate pour les diffs lisibles sur les assertions complexes
- Fixtures dans `tests/fixtures/` — pas de données hardcodées dans les tests

---

## Sécurité

### Unsafe

- `unsafe` uniquement si strictement nécessaire et documenté avec un commentaire `// SAFETY:`
- Encapsuler le code `unsafe` dans des abstractions sûres
- Review systématique de tout bloc `unsafe` — jamais de merge sans approbation explicite

### Mémoire et concurrence

- Rust garantit la sécurité mémoire à la compilation — ne pas contourner avec des patterns UB
- `Arc<Mutex<T>>` ou `Arc<RwLock<T>>` pour le partage entre threads
- Pas de `static mut` — utiliser `OnceLock` ou `LazyLock` (Rust 1.80+)
- Secrets : utiliser `secrecy` crate pour les données sensibles (zeroize à la libération)

---

## Outils obligatoires

| Outil | Usage | Commande |
|-------|-------|----------|
| `clippy` | Linter | `cargo clippy --all-targets -- -D warnings` |
| `rustfmt` | Formatage | `cargo fmt` |
| `cargo test` | Tests + doc-tests | `cargo test --all` |
| `cargo audit` | CVE scanner | `cargo audit` |
| `cargo doc` | Documentation | `cargo doc --no-deps` |

### Configuration clippy recommandée (`.clippy.toml`)

```toml
# Activer les lints restrictifs
cognitive-complexity-threshold = 10
```

### Configuration rustfmt (`rustfmt.toml`)

```toml
edition = "2021"
max_width = 100
use_small_heuristics = "Default"
```

---

## Conventions spécifiques

### Documentation

- Tout élément public doit avoir un doc comment `///`
- Les exemples dans les doc comments doivent compiler (`cargo test` les exécute)
- Sections `# Examples`, `# Errors`, `# Panics` dans les docs publiques

```rust
/// Charge la configuration depuis un fichier TOML.
///
/// # Errors
/// Retourne `ConfigError::NotFound` si le fichier est absent.
///
/// # Examples
/// ```
/// let cfg = Config::load("config.toml")?;
/// ```
pub fn load(path: &str) -> Result<Config, ConfigError> { ... }
```

### Logging

- `tracing` crate recommandée (structured logging, spans async-aware)
- `log` crate acceptable pour les bibliothèques simples
- Niveaux : `error!` (recup impossible), `warn!` (dégradé), `info!` (événements), `debug!` (développement)

---

## Ce que tu NE fais PAS

- Pas de `unwrap()` / `expect()` en dehors des tests et du prototypage
- Pas de `clone()` réflexe — analyser si l'ownership peut être transféré
- Pas de `#[allow(unused)]` permanents — corriger le code sous-jacent
- Pas de dépendances non-nécessaires dans `Cargo.toml` — auditer régulièrement
- Pas de `std::mem::transmute` sans SAFETY comment exhaustif
- Pas de `#![feature(...)]` en dehors des crates explicitement nightly
