---
page: conventions
title: Conventions
confidence: CONFIRMED
sources:
  - docs/guides/contributing.fr.md
  - skills/shared/team-policies-enforcement.md
last_updated: 2026-10-02
---

> [Read in English](conventions.en.md)

# Conventions

## Format des commits

Format Conventional Commits obligatoire :

```
<type>(<scope>): <description>
```

| Type | Usage |
|------|-------|
| `feat` | Nouvelle fonctionnalite |
| `fix` | Correction de bug |
| `docs` | Documentation uniquement |
| `refactor` | Restructuration de code (pas de changement de comportement) |
| `perf` | Amelioration de performance |
| `test` | Ajout ou correction de tests |
| `chore` | Maintenance (deps, CI, outillage) |
| `ci` | Changements CI/CD |
| `security` | Correctif de securite |

Le scope est optionnel mais recommande : `feat(parallel)`, `fix(tui)`, `docs(adr)`.

## Nommage des branches

| Pattern | Usage | Exemple |
|---------|-------|---------|
| `feat/<ticket-ou-slug>` | Branches feature | `feat/BD-42-auth-flow` |
| `fix/<ticket-ou-slug>` | Branches de correction | `fix/BD-55-nil-pointer` |
| `docs/<slug>` | Branches documentation | `docs/parallel-mode-guide` |
| `sweep/<slug>` | Branches mode sweep | `sweep/migrate-slog` |

## Style de code

- **Go** : formatage `gofmt -s` obligatoire, `golangci-lint` v2.1+ avec `bodyclose`, `gocritic`, `misspell`, `nilerr`
- **Markdown** : bilingue FR/EN pour les docs utilisateur (`docs/guides/`, `docs/reference/`, `docs/architecture/adr/`) ; les docs dev-internes (`docs/dev/`) sont monolingues
- **TOML** : utilise pour toute configuration (`hub.toml`, team-state `config.toml`)

## Processus de review

1. Creer une branche feature depuis `main`
2. Implementer avec des tests (`make test`)
3. Lint (`make lint`)
4. Verification du paquet de session (`oh bundle build <workflow>` pour les workflows concernes)
5. Ouvrir une PR avec documentation et mise a jour du changelog
6. Review IA disponible via `oh review`
7. Review humaine requise pour le merge

## Nommage des fichiers

| Type | Convention | Exemple |
|------|-----------|---------|
| Agent | `<domaine>[-<specialite>].md` | `developer-frontend.md` |
| Skill | `<domaine>-<sujet>.md` | `audit-security.md` |
| Guide | `<slug>.{en,fr}.md` | `parallel-mode.en.md` |
| ADR | `<NNN>-<kebab-case>.{en,fr}.md` | `037-documentation-policy.en.md` |
