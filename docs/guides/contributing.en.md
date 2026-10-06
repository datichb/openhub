> [Lire en francais](contributing.fr.md)

# Contribution Guide

This guide explains how to contribute to the hub: Go CLI development,
adding agents and skills, and the PR process.

---

## CLI Development (code contributions)

### Prerequisites

| Tool | Version | Installation |
|------|---------|-------------|
| Go | 1.26.4+ | [go.dev/dl](https://go.dev/dl/) |
| golangci-lint | 2.1+ | `brew install golangci-lint` or [docs](https://golangci-lint.run/welcome/install/) |
| git | 2.5+ | System-provided |

### Local setup

```bash
git clone https://github.com/datichb/openhub.git
cd openhub/cli

# Download dependencies
make deps

# Sync agents/skills/permissions into the Go embed directory
# (MANDATORY step before first build — GoReleaser does this automatically in release)
make embed-sync

# Build the binary
make build          # -> bin/oh
```

> **Important:** `make embed-sync` copies `agents/`, `skills/`, and `permissions/` into
> `internal/hubcontent/hub/`. If you modify an agent or skill, re-run
> `make embed-sync` before rebuilding.

### Running tests

```bash
make test           # All tests with -race detector
make test-unit      # Unit tests only (-short, faster)
make test-tui       # TUI tests only (./internal/tui/...)
make test-cover     # Generates coverage.out + coverage.html
```

CI enforces a minimum coverage threshold of **60%** with the race detector enabled.

### Linting and formatting

```bash
make lint           # golangci-lint run ./...
make vet            # go vet ./...
make fmt            # gofmt -s -w .
```

Active linters include: `bodyclose`, `gocritic` (diagnostic + style), `misspell`, `nilerr`.

### Code overview

The CLI is a single Go binary (`cli/main.go` -> `cli/cmd/root.go`). Internal architecture:

| Package | Responsibility |
|---------|---------------|
| `cmd/` | Cobra commands (CLI entry point) |
| `internal/deploy/` | Transactional deployment engine (agents -> opencode.json) |
| `internal/config/` | Hub configuration (`hub.toml`, TOML + Viper) |
| `internal/mcp/` | 7 built-in MCP servers (Figma, GitLab, GitHub, Jira, Linear, GSlides, Team) |
| `internal/teamstate/` | Team state management (claims, wiki, policies, patterns) |
| `internal/opencode/` | OpenCode integration (sessions, platform, parallel runner) |
| `internal/tui/` | TUI interface (BubbleTea + tview, shell, views, widgets) |
| `internal/workflow/` | Workflow definitions, permissions, validation |
| `internal/storage/` | SQLite (projects, sessions), keychain, file encryption |
| `internal/parallel/` | Parallel session coordination |
| `internal/sweep/` | Sweep mode (split, collect, verify) |
| `internal/tracker/` | External tracker sync (GitLab, Jira) |
| `internal/i18n/` | Internationalization (FR + EN, JSON) |

The binary embeds agents, skills, and permissions via `go:embed` (`internal/hubcontent/`).
`CGO_ENABLED=0` — no C dependencies, static binary.

---

## Adding an agent

### 1. Create the agent file

```bash
touch agents/<family>/<id>.md
```

Follow the naming convention:
- `<domain>-<speciality>.md` for sub-agents (e.g. `auditor-subagent.md`)
- `<role>.md` for primary agents (e.g. `orchestrator.md`)

### 2. Minimal frontmatter structure

```markdown
---
id: <unique-identifier>
label: <DisplayName>
description: <Short description in one sentence — visible in agent lists>
permission:
  skill: allow        # allow for agents using native skills; deny for coordinators
  ctx_search: allow   # allow for agents that analyse code or data
skills: [path/to/skill, ...]          # Bucket A — always-on inline skills
native_skills: [path/to/skill, ...]   # Bucket B — on-demand native skills (optional)
---
```

**Rules:**
- `id`: unique slug, lowercase, hyphens allowed, no spaces
- `label`: PascalCase, displayed in the AI tool
- `description`: one sentence, starts with a verb or a role name
- `skills`: Bucket A paths relative to `skills/` — workflow protocols, handoff formats, universal principles (always active)
- `native_skills`: Bucket B paths relative to `skills/` — domain standards, checklists, contextual skills (loaded on-demand via the `skill` tool)
- `permission.skill`: `allow` if the agent uses native skills; `deny` for coordinators/orchestrators
- `permission.ctx_*`: ctx tools must be explicitly allowed per agent — they are **not** inherited. See the table below for the recommended set per agent type.

**ctx permissions by agent type:**

| Agent type | Recommended ctx permissions |
|---|---|
| Orchestrators (planning) | `ctx_search`, `ctx_stats`, `ctx_batch_execute` |
| Developers (developer, refactor, migrator) | `ctx_search`, `ctx_execute`, `ctx_execute_file`, `ctx_batch_execute`, `ctx_fetch_and_index`, `ctx_index` |
| Quality agents (debugger, reviewer) | `ctx_search`, `ctx_execute`, `ctx_execute_file`, `ctx_batch_execute` |
| Planning agents (planner, pathfinder, onboarder) | `ctx_search`, `ctx_stats`, `ctx_batch_execute` |
| Design agents (designer) | `ctx_search`, `ctx_batch_execute` |
| Documentation (documentarian) | `ctx_search`, `ctx_batch_execute`, `ctx_index` |
| Audit agents (auditor, auditor-subagent) | `ctx_search`, `ctx_batch_execute` |

See [ADR-010](../architecture/adr/010-hybrid-skills-architecture.en.md) for the Bucket A / B rationale.

### 3. Agent body

Recommended structure (see `agents/auditor/auditor.md` as a reference for coordinators,
`agents/developer/developer-frontend.md` for implementer agents):

```markdown
# <DisplayName>

<Identity sentence: who you are and what you do in 2-3 lines>

## What you do

- <Action 1>
- <Action 2>

## What you do NOT do

- <Constraint 1>
- <Constraint 2>

## Workflow

<Condensed workflow in 4-6 steps>

## Invocation examples (optional)

| Request | Action |
|---------|--------|
| "..." | ... |
```

### 4. Create or reference skills

If the agent requires a dedicated protocol, create the corresponding skill
(see the "Adding a skill" section below) before referencing it in the frontmatter.

### 5. Build and test

```bash
# Build the session bundle of a workflow that uses the agent
oh bundle build <workflow>
# Verify the agent appears in the bundle
oh bundle show <workflow>
```

---

## Adding a skill

### 1. Choose the right folder

Skills are organised by domain in `skills/`:

| Folder | Usage |
|--------|-------|
| `skills/developer/` | Development standards (shared between developers and reviewer) |
| `skills/auditor/` | Audit protocols |
| `skills/orchestrator/` | Coordination protocols |
| `skills/planning/` | Planning protocols |
| `skills/posture/` | Posture and cross-cutting behavior (`expert-posture`, `tool-question`) |
| `skills/qa/` | Quality protocols |
| `skills/debugger/` | Diagnostic protocols |
| `skills/reviewer/` | Review protocols |
| `skills/documentarian/` | Documentation protocols |
| `skills/designer/` | Design protocols (designer) |
| `skills/design/` | Design handoff contracts |
| `skills/quality/` | Quality handoff contracts (debugger and agents not in qa/ or reviewer/) |

For a new domain, create a new sub-folder.

### 2. Minimal frontmatter structure

```markdown
---
name: <skill-name>
description: <Short description — visible in the agent's skill list>
---
```

> The `name` key is documentary. The bundle builder reads `description`.
> The file path is the reference used in agent frontmatter.

### 3. Skill content

A good skill contains:

- **Role**: reminder of the identity of the agent using this skill
- **Absolute rules**: ❌/✅ — non-negotiable constraints
- **Protocol / workflow**: detailed steps
- **Output formats**: exact report structures, with examples
- **Checklists**: systematic verifications
- **What you do NOT do**: explicit anti-patterns

See `skills/reviewer/review-protocol.md` as an example.
For Bucket B native skills that handle mode selection and parallel orchestration, see
`skills/reviewer/reviewer-adversarial.md`, `skills/reviewer/reviewer-edge-case.md`, and
`skills/reviewer/review-merge.md` (report fusion from parallel sessions).

### 4. Reference the skill in an agent

Decide whether the skill is Bucket A or Bucket B (see [ADR-010](../architecture/adr/010-hybrid-skills-architecture.en.md)):

**Bucket A** — add to `skills:` in the agent frontmatter:
```markdown
---
skills: [path/to/my-skill]
---
```

**Bucket B** — add to `native_skills:` in the agent frontmatter, and add `permission: skill: allow`:
```markdown
---
permission:
  skill: allow
native_skills: [path/to/my-skill]
---
```
Also add a row to the "## Available skills" guide section in the agent body with the loading trigger.

**Handoff skills:** if your skill defines a structured return format between two agents (a `## Return to ...` block), it is always **Bucket A** — inject it in **both** the producing agent and the consuming agent. This guarantees the two agents share the same contract. See `skills/reviewer/reviewer-handoff-format.md` or `skills/auditor/audit-handoff-format.md` as examples.

---

## Session bundle

Since v5, nothing is deployed into the project (`oh deploy` / `oh sync` removed in v5). Each session starts from a session bundle built at launch outside the project (`~/.oh/bundles/<hash>/`) from its workflow: agents translated from the hub format to the opencode format, skills, permissions, MCP servers.

The bundle logic is implemented in Go in `cli/internal/bundle/`.

### Quick steps

1. Add or modify agents/skills in the `agents/` and `skills/` directories
2. Run `oh bundle build <workflow>` to build the bundle of a workflow that uses them
3. Run `oh bundle show <workflow> --budget` to inspect its content and context budget

---

## Contribution conventions

### Commits

**Conventional Commits** format required:

```
feat: add agent <name>
fix: fix <issue> in <file>
docs: update <section>
chore: <maintenance>
refactor: <restructuring>
```

### File naming

| Type | Convention | Example |
|------|-----------|---------|
| Agent | `<domain>[-<speciality>].md` | `developer-frontend.md` |
| Skill (in a sub-folder) | `<domain>-<topic>.md` | `audit-security.md` |

### Internal documentation

The `docs/` folder is organized as follows (for reference when adding documentation):

- `docs/architecture/` — architectural decisions (ADR), diagrams, agents and skills reference
- `docs/guides/` — practical guides (contributing, workflows, authoring)
- `docs/reference/` — CLI and configuration reference
- `docs/dev/` — internal technical notes (gotchas, shell patterns)

### ADR

Any significant architectural decision must be documented in an ADR:

```bash
touch docs/architecture/adr/<NNN>-<kebab-case-title>.md
```

Format: see [ADR-001](../architecture/adr/001-agent-skill-separation.en.md) as a template.

### PR

Before submitting a PR:

```bash
# Verify the session bundle builds correctly
oh bundle build <workflow>

# Review the bundle content
oh bundle show <workflow> --budget
```

---

## Pre-PR checklist

- [ ] The agent file follows the minimal structure (frontmatter + body)
- [ ] The skill has a frontmatter with `name` and `description`
- [ ] Bucket A skills are in `skills:`, Bucket B skills are in `native_skills:` — rationale documented in [ADR-010](../architecture/adr/010-hybrid-skills-architecture.en.md)
- [ ] If the agent uses `native_skills:`, `permission: skill: allow` is set; if it's a coordinator, `skill: deny` is set
- [ ] ctx permissions (`ctx_search`, `ctx_batch_execute`, etc.) are declared according to the agent type — they are NOT inherited and must be explicit (see frontmatter rules above)
- [ ] If the agent has native skills, the agent body has a "## Available skills" guide section listing them with loading triggers
- [ ] The agent is referenced in `README.md` and `docs/architecture/agents.en.md`
- [ ] The skill is referenced in `docs/architecture/skills.en.md` with its bucket marker (A) or (B)
- [ ] If the skill defines a structured return format: injected in both the producing agent AND the consuming agent (always Bucket A)
- [ ] If architectural decision: an ADR is created in `docs/architecture/adr/`
- [ ] The commit follows Conventional Commits
- [ ] `make test` passes without errors (or `make test-unit` at minimum)
- [ ] `make lint` produces no warnings
- [ ] `oh bundle build <workflow>` passes without errors for the affected workflows
- [ ] `oh bundle show <workflow>` shows no unexpected divergence

---

## Creating a release

> Reserved for maintainers with write access to `main`.

### Prerequisites

- Be on the `main` branch with a clean working tree
- [GoReleaser](https://goreleaser.com/) installed (`brew install goreleaser`)
- Push access to the remote repository

### Preparing the CHANGELOG

Before releasing, write the release content under `## [Unreleased]` in `CHANGELOG.md`.

Items ready for the next release include:
- Team kanban board (5 columns, wired c/x/t/s actions, visual labels `[AI]`)
- Claim lifecycle (5 statuses: `planned`, `in_progress`, `review`, `blocked`, `done`)
- `oh claim --planned` and automatic `planned → in_progress` transition via `oh start --dev`
- External tracker sync (`oh team sync-tracker`, `cli/internal/tracker/` package, GitLab + Jira)
- `agent-reviewed` label auto-applied on `review.ready`
- Async pull on all team views

```markdown
## [Unreleased]

### Added
- ...

### Changed
- ...

### Fixed
- ...
```

### Running the release

```bash
# From the cli/ directory
cd cli

# Preview without publishing (snapshot)
goreleaser release --snapshot --clean

# Create the release (triggered by a tag push)
git tag -a v1.2.0 -m "Release v1.2.0"
git push && git push --tags
```

GoReleaser is configured via `.goreleaser.yml` in the `cli/` directory and handles:
1. Cross-compilation of the `oh` binary
2. Changelog generation from commits
3. GitHub release creation with artifacts

### Tag convention

Tags follow the `vX.Y.Z` format (annotated):

```bash
git tag -a v1.2.0 -m "Release v1.2.0"
```
