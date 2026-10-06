# Team workflows (team-state)

> v5, phase 2. The declarative workflows (`apiVersion: oh/v1`) of a team and its projects live in the team-state repository. This guide covers where they are stored, the integrity check and locks.

## Layers

A workflow is resolved from the most general layer to the most specific one:

| Layer | Location |
|---|---|
| `hub` | workflows shipped with oh (`~/.oh/hub/workflows/`) |
| `team` | `team-state/workflows/published/<id>.yaml` |
| `project` | `team-state/projects/<project>/workflows/published/<id>.yaml` |

A workflow with the same id as a workflow of a lower layer **must extend it** (`extends`), otherwise it is refused. Security fields may only be hardened.

## Layout

```
team-state/
├── workflows/
│   ├── published/<id>.yaml            # published versions (loaded)
│   ├── drafts/<member>/<id>.yaml      # drafts (never loaded for other members)
│   ├── prompts/<id>.md.tmpl           # prompt templates
│   └── history/<id>/<version>.yaml    # previous versions (+ <version>.prompt.md.tmpl)
├── projects/<project>/workflows/…     # same layout for a project
├── catalog/{agents,skills}/           # team bricks
└── workflows.lock                     # integrity of the published files
```

A template path (`prompt.template: prompts/<id>.md.tmpl`) is relative to the `workflows/` folder of its scope, whatever the subfolder of the document (published, draft, history). It cannot leave it.

## `workflows.lock` and integrity

Every publication records the version and hash of the published file (and of its prompt template):

```toml
[team.ticket-hotfix]
version = 2
hash = "sha256:…"
prompt_hash = "sha256:…"
published_by = "alice"
published_at = 2026-10-06T10:00:00Z
message = "Mandatory review checkpoint"

[projects.web.ticket]
…
```

When loading, oh **skips with a warning**:

- a published file missing from `workflows.lock` (never published from oh);
- a file or template changed by hand (different hash);
- a lock entry whose file is gone;
- every team workflow when `workflows.lock` cannot be read.

These warnings show up in `oh workflow validate` and in `oh doctor` ("Team workflows"). To fix: publish the workflow again from oh, or restore the file (`git checkout`).

## Checking team workflows

```bash
oh workflow validate team:ticket-hotfix            # active team
oh workflow validate project:ticket --project web  # project layer + its team
oh workflow validate --all --project web           # hub, team and project
```

## Locks (`enforce`)

A workflow can lock fields for the more specific layers:

```yaml
apiVersion: oh/v1
kind: Workflow
id: ticket
extends: hub:ticket
enforce: [checkpoints, modes]   # or ["*"] for the whole document
```

- Allowed values: the top-level fields of the document (`checkpoints`, `modes`, `agents`, `models`, `runtime`…) or `"*"`.
- A document that extends a locked workflow and writes a locked field is **refused** (`enforced_field`). With `"*"`, only `id`, `version`, `extends` and `enforce` remain allowed.
- Locks add up along the `extends` chain; a more specific layer cannot remove them.
- Launch options (mode, runtime, inputs) are still chosen within the limits of the workflow.

## Drafts, publication, history

The life cycle of a team or project workflow goes through the WorkflowService (the `oh workflow new|edit|publish…` commands come with the phase 2 CLI):

1. **Draft**: `workflows/drafts/<member>/<id>.yaml` (and its own template `<id>.prompt.md.tmpl` when it has one). It is **validated when saved** (refused with its errors), pushed with the team-state, but never loaded for other members.
2. **Test**: `oh run <id> --draft` runs the draft, locally only, and refuses it when it widens the published version (a draft cannot loosen security).
3. **Publication**: team-state pull → **revalidation** of the draft against the up-to-date state → version + 1 → previous version copied to `history/<id>/` (document, template and lock entry) → published file and `workflows.lock` → commit + push. If another member published in the meantime, everything is **redone** on top of their publication (new revalidation, next version). The draft is consumed; a `workflow.published` event is added.
4. **Offline**: the publication is **queued** (in the clone, not versioned) and replayed later with the same cycle; the draft is kept.

### Impact summary

Every publication compares the new version with the previous one: risk raised, new agents (and those that write), remote execution or new runtimes allowed, checkpoints removed, made optional or loosened, Beads commands added, budget raised, new MCP servers or plugins, Code Mode, inputs and prompt changed. Changes that **widen** the workflow are flagged. Team catalogue bricks used for the first time get the "new brick" badge.

### History, restore, archive

- The history lists the published version, then the previous ones (author, date, message).
- **Restoring** a version publishes it again as a **new** version (`workflow.restored` event).
- **Archiving** withdraws the published workflow (moved to history, lock entry removed, `workflow.archived` event); it can be restored.
- Published workflows extending this one that would become invalid are reported when publishing.

Team workflow events go to `projects/_team/events/`, project ones to `projects/<project>/events/`.

## Team brick catalogue

A team can provide its own agents and skills, in the same format as the hub:

```
team-state/catalog/
├── agents/<family>/<id>.md
└── skills/<path>.md            # + skills/templates/… (annexes)
```

- Team and project workflows use them like hub bricks (validation and session bundle).
- An identifier already used by the hub (agent id, skill path or name) is **refused**, unless the brick explicitly declares `extends: hub:<id>` (agent) or `extends: hub:<path>` (skill) in its frontmatter: it then replaces the hub brick. A refused brick is skipped with a warning (`oh doctor`).
- An annex cannot replace a hub file.

## Governance

Publishing is configured in the team-state `config.toml`:

```toml
[governance]
publish = "any_member"   # default, the only supported value
```

- `any_member`: any member listed in `members.toml` may publish; validating the workflow is still required.
- A value this version of oh does not know **blocks publishing** (the rest of the configuration still works): update oh or go back to `any_member`.

## Project without a team: solo space

A project without a team keeps its workflows in a **solo space**: a local team-state, without remote, in `~/.oh/teams/<id>/`.

```bash
oh team init --solo --project web-app     # create the space and attach the project
oh team promote --remote <empty-url>      # later: share it with a team
```

Publications there are local commits. `oh team promote` pushes the whole history to the remote, without loss (see [team CLI](../reference/cli-team.en.md#oh-team-promote)).
