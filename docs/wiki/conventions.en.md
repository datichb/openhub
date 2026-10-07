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

- **Go**: `gofmt -s` formatting enforced, `golangci-lint` v2.1+ with `bodyclose`, `gocritic`, `misspell`, `nilerr`, with no finding at all (`make lint`: macOS, Linux, Windows, with the test tags)
- **Tool independence** (D19, [ADR-049](../architecture/adr/049-tool-independence-architecture-guard.en.md)): nothing specific to the session tool outside `internal/adapters/<tool>`; go through the interface or a capability of the adapter, the displayed name comes from `ToolInfo.DisplayName`. The `internal/archtest` test checks it
- **Markdown**: bilingual FR/EN for user-facing docs (`README`, `SECURITY`, `docs/guides/`, `docs/reference/`, `docs/architecture/` and its ADRs, `docs/wiki/`), with the same sections in both languages and a language link on line 1 (right after the frontmatter for wiki pages); dev-internal docs (`docs/dev/`) are monolingual
- **TOML**: used for all configuration (`hub.toml`, team-state `config.toml`); workflows are YAML (`apiVersion: oh/v1`)

## Review Process

1. Create feature branch from `main`
2. Implement with tests (`make test`)
3. Lint (`make lint`)
4. Workflow and bundle checks (`oh workflow validate --all`, `oh bundle build <workflow>` for the affected workflows)
5. Open PR with documentation and changelog updates
6. AI review available via `oh run review`
7. Human review required for merge

## File Naming

| Type | Convention | Example |
|------|-----------|---------|
| Agent | `agents/<family>/<id>.md` (`<role>[-<speciality>]`) | `agents/developer/developer-refactor.md` |
| Skill | `skills/<folder>/<domain>-<topic>.md` | `skills/auditor/audit-security.md` |
| Workflow | `workflows/<id>.yaml` + `workflows/prompts/<id>.md.tmpl` | `workflows/ticket.yaml` |
| Guide | `<slug>.{en,fr}.md` | `sessions-v5.en.md` |
| ADR | `<NNN>-<kebab-case>.{en,fr}.md` | `045-execution-environments.en.md` |
