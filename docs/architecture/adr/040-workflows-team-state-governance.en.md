> [Lire en français](040-workflows-team-state-governance.fr.md)

# ADR-040 — Workflows in the Team-State, Governance in the Hub, Solo Space

## Status

Accepted

## Date

2026-10-05

## Context

The team-state repository ([ADR-024](./024-team-state-repository.en.md)) already shares a team's claims, events and wiki; [ADR-029](./029-multi-team-support.en.md) allows several teams per hub, and a project without a team has no shared state. The configuration of the single workflow was scattered ([ADR-033](./033-config-cascade-enforcement.en.md)):

- the `[workflow]` section of the team `config.toml`, with `Enforced`;
- the `projects.workflow_config` column in SQLite;
- the `[workflow.overrides]` of `hub.toml`, which were in fact never loaded (bug found during the migration).

With declarative workflows ([ADR-039](./039-declarative-workflows-oh-v1.en.md)), we needed:

- teams and projects to be able to create, edit and publish their own;
- what is loaded to be intact;
- concurrent publications and offline work to be handled;
- projects without a team to work the same way.

Decisions D5, D7 (governance in the hub, without merge requests), O12 (impact summary) and O14 (publication).

## Decision

### 1. Layout

The team-state hosts:

- `workflows/published/<id>.yaml`, `workflows/drafts/<member>/<id>.yaml`, `workflows/prompts/`, `workflows/history/<id>/<version>.{yaml,prompt.md.tmpl,lock.toml}`;
- the same folders under `projects/<p>/workflows/` for the project layer;
- `catalog/agents/<family>/<id>.md` and `catalog/skills/<path>.md`: team bricks, in the hub format;
- `workflows.lock` (TOML: `[team.<id>]`, `[projects.<p>.<id>]` with version, `hash`, `prompt_hash`, author, date, message);
- the `workflow.published`, `workflow.restored` and `workflow.archived` events, under `projects/_team/events/` for the team or under `projects/<p>/events/`.

### 2. Integrity

- Only published files whose YAML **and** prompt template match `workflows.lock` are loaded.
- Otherwise the file is ignored, with a warning: not locked, different hash or template, orphan entry, unreadable lock. The warning is visible in the TUI catalog, in `oh workflow list` and in Doctor.

### 3. Drafts

- A draft belongs to one member. It is committed and pushed (visible to others, never loaded for them) and validated on save together with the member's other drafts; an invalid draft is not written.
- A draft can have its own prompt template, which never touches the published template.
- `oh run <wf> --draft` tests a draft: locally or in a container, never remotely, and without being able to widen anything compared to the published version.

### 4. Publication

`teamstate.Repo.Transact` runs a transaction: pull → build → commit → push.

The build:

1. reads the governance again;
2. computes the impact against the previous version;
3. copies the current version into `history/`;
4. writes version N+1 into the text (comments kept);
5. seals the lock;
6. **revalidates** against the up-to-date state;
7. reports the workflows that extend this one and become invalid;
8. deletes the draft and emits the event.

On concurrency or without network:

- **push rejected**: the commit is dropped and the cycle is redone on top of the concurrent publication, up to 4 times;
- **remote unreachable**: nothing is committed, the operation goes into a local queue (`.git/oh-workflow-queue.json`). The queue is replayed at every TUI sync and by `oh workflow publish --retry`.

Before publication, an **impact summary** (O12) is shown. It covers the risk, agents that write, remote allowed, modes, checkpoints, Beads, budget, MCP, plugins, Code Mode, skills, inputs and prompt, plus the "new" team bricks. A confirmation is asked if the workflow is widened. The message is mandatory.

`History`, `Restore` (republishes an old version as a new version) and `Archive` complete the cycle.

### 5. Governance

- `[governance] publish = "any_member"` in the team `config.toml`: **any member** listed in `members.toml` can publish. It is the only supported value.
- An unknown value refuses publication: we refuse rather than allow.
- The policy is shown in the team detail (TUI).

### 6. Team brick catalog

- The agents and skills of `catalog/` are merged with the hub's into a cached folder (`~/.oh/cache/bricks/<key>`, rebuilt when the hub or the bricks change). This folder is used for validation and for the bundle.
- An ID collision with the hub is **refused**, unless `extends: hub:<id>` is in the frontmatter (the brick replaces the hub one). A refused brick is ignored with a warning.

### 7. Solo space

- `oh team init --solo [--project <p>]` creates `~/.oh/teams/<id>/`: a git repository without a remote, a single member with the `lead` role, and a team declared `solo = true` in `hub.toml`.
- A solo space is **never the active team**: the `team` MCP, claims, board and events stay off. It only carries workflows and bricks.
- `oh team promote --remote <url>` adds the remote and pushes the history. The ID, attached projects and published workflows do not change. On failure, the space stays solo.
- The TUI offers the solo space on first launch and when adding a project without a team, as well as the "Switch to team" action.

### 8. Migration of the former configuration (v38)

The migration runs automatically when a command starts, is idempotent and is retried on failure:

- a team's `[workflow]` and each project's configuration are translated into a patch of `feature`, archived raw, then published as a draft. A solo space is created for a project without a team. `Enforced` becomes `enforce: ["*"]`.
- Migration v38 neutralizes `projects.workflow_config` without loss (copy into `workflow_config_legacy`).
- The `hub.toml` overrides are archived in `~/.oh/migrated/`, but **not loaded**.
- The former fields (`WorkflowTeamConfig`, `Project.WorkflowConfig`, `config.Workflow`) and the Workflow view are removed.

### 9. Commands

- CLI: `oh workflow new|edit|diff|publish|history|restore|archive`.
- TUI: editable catalog (drafts, integrity), 5-section editor (General, Graph, Inputs & prompt, Resources, Bundle preview) with the origin and locks of every value, Publication and History screens.

## Consequences

### Positive

- A team shares its workflows without merge requests or any particular forge, with a full history and restore.
- What is loaded is verified: a file edited by hand is ignored and reported.
- Concurrent publications are revalidated, and offline work is queued.
- A project without a team uses exactly the same model (solo space), promotable without loss.
- Teams can ship their own agents and skills without modifying the hub.

### Negative / Trade-offs

- Any member can publish; finer governance (lead only, approvals) is in the backlog (BL-2).
- No server-side protection of the team-state (BL-5): someone who can push can write a file by hand. The file will be ignored, but nothing prevents it.
- Each operation costs git commands (~2 s each observed on the build machine).
- Drafts are visible to all members.
- The impact does not compare an agent's own permissions from one version to the next (BL-16), and `enforce:` stays at the top level.
- No member invitation from oh (BL-15): `oh team promote` shows the URL to share.
- `oh workflow new --file` refuses a document that references a template yet to be created (issue Q3-7).
- The migrated `hub.toml` overrides are no longer applied.

## Alternatives Considered

| Alternative | Rejected because |
|---|---|
| Governance through merge requests on the team-state | Heavy for a team of 3 to 5 people and dependent on the forge; validation by oh, the lock and the history are enough (D7). |
| Team workflows in `hub.toml` or in SQLite | Neither shared nor versioned; that was the starting point. |
| A repository dedicated to workflows | One more repository to clone and configure for each member. |
| Rebase the publication commit on the concurrent publication | Guaranteed conflicts on `workflows.lock`; redoing the whole cycle revalidates against the real state. |
| Let a team brick silently shadow the hub one | Invisible behavior; `extends: hub:<id>` makes the replacement explicit. |
| Keep workflows of projects without a team in the local database | Two models to maintain; the solo space gives the same model and is promoted without loss. |
