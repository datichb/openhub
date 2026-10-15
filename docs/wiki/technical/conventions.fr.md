---
updated: 2026-10-15
confidence: confirmed
agents: [onboarder]
---

> [Read in English](conventions.en.md)

# Conventions

Sources de la page — `CONFIRMÉ` · onboarder · 2026-10-06 · docs/guides/contributing.fr.md, skills/shared/team-policies-enforcement.md

## Format des commits

Format Conventional Commits obligatoire :

```
<type>(<scope>): <description>
```

| Type | Usage |
|------|-------|
| `feat` | Nouvelle fonctionnalité |
| `fix` | Correction de bug |
| `docs` | Documentation uniquement |
| `refactor` | Restructuration de code (pas de changement de comportement) |
| `perf` | Amélioration de performance |
| `test` | Ajout ou correction de tests |
| `chore` | Maintenance (deps, CI, outillage) |
| `ci` | Changements CI/CD |
| `security` | Correctif de sécurité |

Le scope est optionnel mais recommandé : `feat(workflow)`, `fix(tui)`, `docs(adr)`.

## Nommage des branches

| Pattern | Usage | Exemple |
|---------|-------|---------|
| `feat/<ticket-ou-slug>` | Branches feature | `feat/BD-42-auth-flow` |
| `fix/<ticket-ou-slug>` | Branches de correction | `fix/BD-55-nil-pointer` |
| `docs/<slug>` | Branches documentation | `docs/migration-v5-guide` |

L'ancien préfixe `sweep/<slug>` (une branche par sous-tache du mode sweep) n'est plus utilisé : le workflow `sweep` travaille dans la même session et le même emplacement.

## Style de code

- **Go** : formatage `gofmt -s` obligatoire, `golangci-lint` v2.1+ avec `bodyclose`, `gocritic`, `misspell`, `nilerr`, sans aucun signalement (`make lint` : macOS, Linux, Windows, avec les tags de test)
- **Indépendance vis-à-vis de l'outil** (D19, [ADR-049](../../architecture/adr/049-tool-independence-architecture-guard.fr.md)) : rien de propre à l'outil des sessions hors de `internal/adapters/<outil>` ; passer par l'interface ou une capacité de l'adaptateur, le nom affiché vient de `ToolInfo.DisplayName`. Le test `internal/archtest` le vérifie
- **Markdown** : bilingue FR/EN pour les docs utilisateur (`README`, `SECURITY`, `docs/guides/`, `docs/reference/`, `docs/architecture/` et ses ADR, `docs/wiki/`), avec les mêmes sections dans les deux langues et un lien de langue en ligne 1 (juste après le frontmatter pour les pages du wiki) ; les docs dev-internes (`docs/dev/`) sont monolingues
- **TOML** : utilisé pour toute configuration (`hub.toml`, team-state `config.toml`) ; les workflows sont en YAML (`apiVersion: oh/v1`)

## Processus de review

1. Créer une branche feature depuis `main`
2. Implémenter avec des tests (`make test`)
3. Lint (`make lint`)
4. Vérification des workflows et du paquet de session (`oh workflow validate --all`, `oh bundle build <workflow>` pour les workflows concernés)
5. Ouvrir une PR avec documentation et mise à jour du changelog
6. Review IA disponible via `oh run review`
7. Review humaine requise pour le merge

## Nommage des fichiers

| Type | Convention | Exemple |
|------|-----------|---------|
| Agent | `agents/<famille>/<id>.md` (`<role>[-<specialite>]`) | `agents/auditor/auditor-subagent.md` |
| Skill | `skills/<dossier>/<domaine>-<sujet>.md` | `skills/auditor/audit-security.md` |
| Workflow | `workflows/<id>.yaml` + `workflows/prompts/<id>.md.tmpl` | `workflows/ticket.yaml` |
| Guide | `<slug>.{en,fr}.md` | `sessions-v5.en.md` |
| ADR | `<NNN>-<kebab-case>.{en,fr}.md` | `045-execution-environments.en.md` |
