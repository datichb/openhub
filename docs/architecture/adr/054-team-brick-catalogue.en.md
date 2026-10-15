> [Lire en français](054-team-brick-catalogue.fr.md)

# ADR-054 — Team Brick Catalogue: Command, Governance and Integrity

## Status

Accepted

## Date

2026-10-15

## Context

The team catalogue (`catalog/{agents,skills}/` of the team-state, [ADR-040](040-workflows-team-state-governance.en.md)) is the only way to add bricks since the community skills registry was disconnected ([ADR-051](051-community-skills-disconnection.en.md)). It was edited by hand, with a commit in the team-state repository:

- **no validation before publication**: a team skill too long or without a trigger only gave an `oh skill check` warning, an agent with an unreadable frontmatter was silently skipped;
- **no integrity**: unlike workflows (`workflows.lock`), any file committed in `catalog/` reached the session bundles, including a hand-edited file or a link to a file outside the repository;
- **no trace**: no version, author or history of the bricks.

ADR-051 listed a brick command as the first redesign track, "on the model of `oh workflow`", with the same governance and the same integrity check.

## Decision

### 1. `oh brick` command

| Command | Role |
|---|---|
| `oh brick list [--agents\|--skills] [--origin]` | Hub and team bricks, with their origin (`hub`, `team`, "replaces hub:…"), version, problems, the member's drafts and the skipped files |
| `oh brick show <id>` | Origin, file, version, uses (agents, workflows), cost, problems; `--raw` prints the file |
| `oh brick new <agent\|skill> <id>` | Draft from a valid template; `--extends` starts from the hub brick and replaces it explicitly |
| `oh brick edit <id>` | Editor, validation when saved, reopened on errors |
| `oh brick publish <id> -m …` | Publishes the draft as the next version; `--adopt` seals a catalogue committed by hand |
| `oh brick history <id>` | Published versions (author, date, message) |

A bare id names an agent or a skill; when it names both, give `agent:<id>` or `skill:<path>`. The context is the one of the `oh workflow` commands: `--project` (the team-state of its team, solo space included) or `--team`.

The brick catalogue is **a team catalogue**: there is no "project" layer for bricks. A project uses the bricks of its team.

### 2. Layout

```
team-state/
├── catalog/
│   ├── agents/<family>/<id>.md                  # published (loaded)
│   ├── skills/<path>.md                         # published (loaded); skills/templates/…: annexes
│   ├── drafts/<member>/{agents,skills}/…        # drafts (never loaded)
│   └── history/{agents,skills}/<id>/<v>.md      # previous versions (+ <v>.lock.toml)
└── catalog.lock                                 # integrity of the published bricks
```

### 3. Validation

A brick written with `oh brick` (draft, publication) is validated against the hub merged with the published team catalogue, with the hub rules:

- **skill**: the `oh skill check` rules (frontmatter, `name:` equal to the file name, known keys, `requires:` and annexes present, no cycle, `when:`, freshness of stack skills); the description starts with "Utiliser …" and is **200 characters at most** (errors, not warnings as for a skill committed by hand);
- **agent**: readable frontmatter, `id:` equal to the file name, `description:` present, `mode:` `primary` or `subagent`, known keys, skills present in the catalogue, existing `permission_base`.

An invalid brick is not saved. The published catalogue is checked by `oh doctor` and `oh brick list` with the `oh skill check` rules for the team (descriptions as warnings).

### 4. Replacing a hub brick

A team brick with the id of a hub agent, or the name of a hub skill, is **refused** unless it says so (`extends: hub:<id>`, ADR-040 rule). `oh brick new … --extends` writes that declaration. The replacement is **reported** everywhere: warning when saved, confirmation when published, origin column of `oh brick list`, `replaces` field of the event.

### 5. Governance

Publication follows `[governance] publish` of the team `config.toml`, as for workflows (`any_member`: a member listed in `members.toml`; an unknown value blocks). It is a team-state transaction (pull → rebuild → commit → push, rebuilt when another member pushed first): the brick is revalidated on the up-to-date state, the previous version is copied to `catalog/history/`, the new one is sealed in `catalog.lock`, the draft is removed, the `brick.published` event is written to `projects/_team/events/`. Offline, nothing is published and the draft stays (no queue in part 1).

### 6. Integrity (`catalog.lock`)

```toml
[agents.hotfixer]
version = 2
path = "catalog/agents/team/hotfixer.md"
hash = "sha256:…"
published_by = "alice"
published_at = 2026-10-15T10:00:00Z
message = "…"

[skills."team/hotfix-protocol"]
…
```

When `catalog.lock` exists, a brick is only loaded when its file is at the recorded path with the same hash; otherwise it is **skipped** with a warning (`brick_unlocked`, `brick_hash_mismatch`), as are the entries without file (`brick_lock_orphan`) and an unreadable lock (`brick_lock_invalid`, every brick skipped). The warnings show in `oh brick list`, `oh workflow list`, the TUI workflow catalogue and `oh doctor`.

**Transition.** Without `catalog.lock` (catalogue committed by hand before this ADR), the bricks keep loading, reported "never sealed" by `oh doctor` and `oh brick list`. The first publication is refused while other bricks are not sealed: `oh brick publish --adopt -m …` seals the whole catalogue as it is, in one transaction (`brick.adopted` event), when no brick is refused or in error.

### 7. Checked paths

Same requirements as ADR-051 set for the disconnected registry:

- an id is made of kebab-case segments (one for an agent and its family, four at most for a skill), without `..`, never absolute, never under `templates/`; the member name is checked;
- every written path (draft, publication, history, lock) is built from these names, under `catalog/`; a `catalog.lock` entry can only point to the published file of its brick;
- a **symbolic link** (or any non-regular file) of the catalogue is skipped (`brick_unsafe_path`): nothing outside the repository reaches a bundle; a draft that is not a regular file is not published.

### 8. Out of scope

**No import of external bricks** (repository, URL, index): the rule stays that no skill is taken as is. Import into the team catalogue (ADR-051 track 2: version pinned by commit, hash, `https`, review before publication) is in the backlog.

## Consequences

### Positive

- Team bricks get the workflow cycle: validated draft, governed publication, version, history, integrity.
- A hand-edited file or a link no longer reaches the bundles of a team that sealed its catalogue.
- Replacing a hub brick no longer goes unnoticed.

### Negative / Trade-offs

- Validating a brick is slower than `oh skill check`: the hub is copied with the brick into a temporary folder (≈ 0.5 s).
- During the transition, a catalogue without `catalog.lock` keeps loading without integrity check; only `oh doctor` reports it.
- Annexes (`catalog/skills/templates/`) are neither edited nor sealed by `oh brick`; they stay committed by hand (links are refused there).
- No archive or restore of a brick, no offline queue, no editing view in the TUI (the TUI brick catalogue stays read-only, with the origin of each brick).
- The TUI does not show the version or the replacement of a hub brick yet.

## Alternatives Considered

| Alternative | Rejected because |
|-------------|------------------|
| Reuse `workflows.lock` | Mixes two entry formats; a brick has a path (family, skill folder) a workflow does not have |
| Refuse every unsealed brick from this ADR on | Silently breaks the catalogues committed by hand; explicit adoption takes them over in one command |
| Seal the existing bricks automatically at the first publication | Seals content without review; `--adopt` is an explicit act of an allowed member |
| A "project" layer for bricks | No expressed need; doubles the integrity surface; a project follows the bricks of its team |
| Warnings only for skill descriptions written with `oh brick` | A long description costs at every session; the hub rule applies to what is written with oh |

## Links

- [ADR-040](040-workflows-team-state-governance.en.md): workflows in the team-state, governance, team bricks
- [ADR-051](051-community-skills-disconnection.en.md): community skills registry disconnected (redesign tracks)
- [CLI reference — Bricks](../../reference/cli-bricks.en.md)
