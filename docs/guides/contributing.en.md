> 🇫🇷 [Lire en français](contributing.fr.md)

# Contribution Guide

This guide explains how to add an agent, a skill, or an adapter to the hub,
and how to contribute via a PR.

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
| Quality agents (qa-engineer, debugger, reviewer) | `ctx_search`, `ctx_execute`, `ctx_execute_file`, `ctx_batch_execute` |
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

### 5. Deploy and test

```bash
oh deploy opencode
# Verify the agent appears
oh agent list
oh agent info <id>
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
description: <Short description — visible in oh agent edit and oh skills list>
---
```

> The `name` key is documentary. Scripts only read `description`.
> The file path is the reference used in agent frontmatter.

### 3. Skill content

A good skill contains:

- **Role**: reminder of the identity of the agent using this skill
- **Absolute rules**: ❌/✅ — non-negotiable constraints
- **Protocol / workflow**: detailed steps
- **Output formats**: exact report structures, with examples
- **Checklists**: systematic verifications
- **What you do NOT do**: explicit anti-patterns

See `skills/reviewer/review-protocol.md` or `skills/qa/qa-protocol.md` as examples.

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

## Adding an adapter

An adapter translates agents from the hub format to the format of a target tool.

The full contract (8 mandatory functions, parameters, available utility functions
and minimal example) is documented in
[docs/architecture/adapters.en.md](../architecture/adapters.en.md).

### Quick steps

1. Create `scripts/adapters/<target>.adapter.sh` with the 8 contract functions
2. Add the target to `config/hub.json`
3. Test with `oh deploy <target>` then `oh agent list`

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
| Shell script | `cmd-<command>.sh` | `cmd-deploy.sh` |
| Adapter | `<target>.adapter.sh` | `opencode.adapter.sh` |

### Shell scripts

Mandatory rules for all shell scripts:

```bash
#!/bin/bash
set -euo pipefail

# ✅ Local variables are declared inside functions
my_function() {
  local my_var="value"
}

# ❌ Never use 'local' outside a function — undefined behavior with set -euo pipefail
# ❌ Never use "$var" && command — always use if [ "$var" = "true" ]
```

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
# Verify agents deploy correctly
oh deploy opencode
oh deploy --check opencode

# List agents to verify consistency
oh agent list
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
- [ ] `oh deploy opencode` and `oh deploy --check opencode` pass without errors
- [ ] `oh deploy --diff opencode` shows no unexpected divergence

---

## Creating a release

> Reserved for maintainers with write access to `main`.

### Prerequisites

- Be on the `main` branch with a clean working tree
- `jq` installed (`brew install jq` or `apt install jq`)
- Push access to the remote repository

### Preparing the CHANGELOG

Before running the script, write the release content under `## [Unreleased]` in `CHANGELOG.md`:

```markdown
## [Unreleased]

### Added
- ...

### Fixed
- ...
```

The script will automatically insert the `## [X.Y.Z] — YYYY-MM-DD` header after `[Unreleased]`.

### Running the release

```bash
# Preview without modifying anything (dry-run)
bash scripts/release.sh 1.2.0 --dry-run

# Create the release
bash scripts/release.sh 1.2.0
```

The script performs in order:
1. Validates the `X.Y.Z` format
2. Checks `main` branch + clean working tree + non-existing tag
3. Updates `config/hub.json` (local, not tracked by git) → `.version`
4. Updates `config/hub.json.example` (tracked) → `.version`
5. Inserts `## [X.Y.Z] — date` into `CHANGELOG.md`
6. Creates commit `chore(release): vX.Y.Z`
7. Creates annotated tag `vX.Y.Z`
8. Offers to push (`git push && git push --tags`)

### Version file conventions

| File | Git tracked | Role |
|------|------------|------|
| `config/hub.json` | No (local) | Active instance config — never committed |
| `config/hub.json.example` | Yes | Version source of truth — committed at each release |

A new user who clones the repository runs `install.sh` which copies `hub.json.example` → `hub.json`.

### Tag convention

Tags follow the `vX.Y.Z` format (annotated):

```bash
git tag -a v1.2.0 -m "Release v1.2.0"
```

### Post-release install one-liner

After pushing, the one-liner to install this version is:

```bash
curl -fsSL https://raw.githubusercontent.com/datichb/openhub/main/install.sh | VERSION=v1.2.0 bash
```
