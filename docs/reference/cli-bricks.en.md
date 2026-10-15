> [Lire en français](cli-bricks.fr.md)

# CLI Reference — Bricks

`oh brick` manages the **team brick catalogue**: the agents and skills of `catalog/` in the team-state (solo space included), used by the workflows of the team and its projects like the hub bricks. Same cycle as team workflows: draft validated when saved, governed publication, version sealed in `catalog.lock`, history. Decision: [ADR-054](../architecture/adr/054-team-brick-catalogue.en.md). No import of external bricks.

Every command accepts `-p, --project <project>` (team-state of the project's team; default: project of the current directory, else the active team or the only solo space) and `--team <id>` (a team or a solo space, without project). They are not repeated in the tables.

**Naming a brick.** An agent by its id (`hotfixer`), a skill by its path under `skills/`, without `.md` (`team/hotfix-protocol`). An id naming both an agent and a skill is refused: prefix it (`agent:<id>`, `skill:<path>`). Names are checked: kebab-case segments (four at most for a skill), no `..`, nothing under `templates/`.

## oh brick list

```
oh brick list [--agents|--skills] [--origin] [--json]
```

Lists the bricks of the context: those of the hub and those of the team catalogue, with their kind, their **origin** (`hub`, `team`, or `team (replaces hub:<id>)` for a brick replacing a hub brick), their published version and their state (`✓`, `!` warnings, `✗` errors). Then come your **drafts** (`✎`), the bricks of a never-sealed catalogue and the files skipped by the integrity check.

| Flag | Type | Description |
|------|------|-------------|
| `--agents` | bool | Agents only |
| `--skills` | bool | Skills only (excludes `--agents`) |
| `--origin` | bool | Adds the source file column (`catalog/…` of the team-state, or `hub`) |
| `--json` | bool | JSON output: `bricks` (with `origin`, `team_id`, `source`, `shadows`, `version`, `errors`, `warnings`), `drafts`, `sealed`, `unsealed`, `integrity`, `problems` |

## oh brick show

```
oh brick show <id> [--raw] [--json]
```

Shows a brick (your draft first, else the published version, else the hub one): origin, file, version, family, mode and skills of an agent, requirements and agents of a skill, workflows shipping it, estimated cost in tokens, problems found.

| Flag | Type | Description |
|------|------|-------------|
| `--raw` | bool | Prints the file as is |
| `--json` | bool | JSON output (`ref`, `origin`, `path`, `draft`, `entry`, `diagnostics`) |

## oh brick new

```
oh brick new <agent|skill> <id> [--family <family>] [--extends] [--file <file>|-] [--no-edit]
```

Creates your draft from a **valid template** (read-only `subagent` with `shared/universal-guardrails`; skill with a « Utiliser quand … » description), opens `$VISUAL`/`$EDITOR` (else `vi`), then validates. On errors the editor can be reopened; otherwise the edited file is kept and nothing is saved.

A hub agent id, or a hub skill name, is refused: to **replace** a hub brick, say so with `--extends`. The draft then starts from the hub brick and begins with `extends: hub:<id>`.

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--family` | string | `team` | Folder of the agent in `catalog/agents/` |
| `--extends` | bool | `false` | Start from the hub brick with the same id (agent) or name (skill) and replace it |
| `--file` | string | | Read the brick from this file (`-`: standard input), without editor |
| `--no-edit` | bool | `false` | Save the template without opening the editor |

## oh brick edit

```
oh brick edit <id> [--file <file>|-] [--family <family>]
```

Opens your draft, else the published version of the team brick, and saves a validated draft. Hub bricks are read-only (`oh brick new … --extends`). `--family` moves an agent to another folder.

## Validation

A draft is validated against the hub merged with the published team catalogue, with the hub rules:

| Brick | Rules (errors) |
|-------|----------------|
| skill | those of [`oh skill check`](cli-tools.en.md#oh-skill-check): frontmatter, `name:` equal to the file name, known keys, `requires:` and annexes present, no cycle, `when:`, freshness of stack skills; **« Utiliser … » description of 200 characters at most** |
| agent | readable frontmatter (`project_instructions` included), `id:` equal to the file name, `description:`, `mode:` `primary` or `subagent`, known keys (`id`, `label`, `description`, `mode`, `model`, `permission_base`, `permission`, `skills`, `native_skills`, `verify`, `stack_skills`, `skills_from_callers`, `project_instructions`, `extends`), skills present, existing `permission_base` |
| both | hub id without `extends: hub:<id>` (`brick_collision`), `extends` naming no hub brick (`brick_bad_extends`), skill name already taken in the team (`brick_duplicate`) |

Replacing a hub brick gives the `brick_shadows` warning.

## oh brick publish

```
oh brick publish <id> -m <message> [-y]
oh brick publish --adopt -m <message> [-y]
```

Publishes your draft as the next version. The command prints the version and the findings, then asks for confirmation when the brick **replaces a hub brick** (without terminal or with `-y`: no question). The publication:

1. checks the team governance (`[governance] publish`, as [`oh workflow publish`](cli-workflows.en.md#oh-workflow-publish));
2. revalidates the brick on the up-to-date team-state (pull), rebuilt when another member pushed first;
3. copies the previous version to `catalog/history/<agents|skills>/<id>/<v>.md` (and `<v>.lock.toml`);
4. writes the brick, seals it in `catalog.lock`, removes your draft, writes the `brick.published` event;
5. commits and pushes.

Offline, nothing is published and the draft stays: run it again once online.

**Catalogue committed by hand.** Without `catalog.lock`, the existing bricks keep loading but are not sealed; publishing another brick is refused until they are. `--adopt` (without id) seals them all as they are, in one transaction (`brick.adopted` event), when none is refused or in error.

| Flag | Type | Description |
|------|------|-------------|
| `-m, --message` | string | Publication message (required) |
| `--adopt` | bool | Seal as is a catalogue committed by hand |
| `-y, --yes` | bool | No confirmation |

## oh brick history

```
oh brick history <id> [--json]
```

Published versions of a team brick, most recent first: version (`(published)` for the version in use), date, author, message.

## Integrity

When `catalog.lock` exists, a brick is only loaded (workflow validation, session bundles) when its file is at the recorded path with the same hash. Otherwise it is skipped with a warning:

| Code | Case |
|------|------|
| `brick_unlocked` | file not in `catalog.lock` (committed without oh) or moved |
| `brick_hash_mismatch` | file edited by hand since its publication |
| `brick_lock_orphan` | `catalog.lock` entry without file |
| `brick_lock_invalid` | unreadable `catalog.lock`: every team brick is skipped |
| `brick_unsafe_path` | symbolic link or non-regular file in `catalog/` (always, lock or not) |

These warnings show in `oh brick list`, `oh workflow list`, the TUI workflow catalogue and `oh doctor`. To fix: publish the brick again with `oh brick edit` then `oh brick publish`, or restore the file (`git checkout`). Annexes (`catalog/skills/templates/`) are not sealed.

## oh doctor

The "Team bricks (<team>)" check shows for every team-state with bricks: number of bricks in use, **invalid** bricks (errors of the validation rules on the published catalogue, skill descriptions as warnings as in `oh skill check`), files skipped by the integrity check, never-sealed catalogue (warning).
