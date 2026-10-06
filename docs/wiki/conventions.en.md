---
page: conventions
title: Conventions
confidence: CONFIRMED
sources:
  - docs/guides/contributing.en.md
  - skills/shared/team-policies-enforcement.md
last_updated: 2026-10-06
---

> [Lire en francais](conventions.fr.md)

# Conventions

## Commit Format

Conventional Commits format is required:

```
<type>(<scope>): <description>
```

| Type | Usage |
|------|-------|
| `feat` | New feature |
| `fix` | Bug fix |
| `docs` | Documentation only |
| `refactor` | Code restructuring (no behavior change) |
| `perf` | Performance improvement |
| `test` | Adding or fixing tests |
| `chore` | Maintenance (deps, CI, tooling) |
| `ci` | CI/CD changes |
| `security` | Security fix |

Scope is optional but recommended: `feat(workflow)`, `fix(tui)`, `docs(adr)`.

## Branch Naming

| Pattern | Usage | Example |
|---------|-------|---------|
| `feat/<ticket-or-slug>` | Feature branches | `feat/BD-42-auth-flow` |
| `fix/<ticket-or-slug>` | Bug fix branches | `fix/BD-55-nil-pointer` |
| `docs/<slug>` | Documentation branches | `docs/migration-v5-guide` |

The former `sweep/<slug>` prefix (one branch per subtask of the sweep mode) is no longer used: the `sweep` workflow works in the same session and location.

## Code Style

- **Go**: `gofmt -s` formatting enforced, `golangci-lint` v2.1+ with `bodyclose`, `gocritic`, `misspell`, `nilerr`
- **Markdown**: bilingual FR/EN for user-facing docs (`docs/guides/`, `docs/reference/`, `docs/architecture/adr/`); dev-internal docs (`docs/dev/`) are monolingual
- **TOML**: used for all configuration (`hub.toml`, team-state `config.toml`); workflows are YAML (`apiVersion: oh/v1`)

## Review Process

1. Create feature branch from `main`
2. Implement with tests (`make test`)
3. Lint (`make lint`)
4. Session bundle check (`oh bundle build <workflow>` for the affected workflows)
5. Open PR with documentation and changelog updates
6. AI review available via `oh run review`
7. Human review required for merge

## File Naming

| Type | Convention | Example |
|------|-----------|---------|
| Agent | `<domain>[-<speciality>].md` | `developer-frontend.md` |
| Skill | `<domain>-<topic>.md` | `audit-security.md` |
| Guide | `<slug>.{en,fr}.md` | `sessions-v5.en.md` |
| ADR | `<NNN>-<kebab-case>.{en,fr}.md` | `037-documentation-policy.en.md` |
