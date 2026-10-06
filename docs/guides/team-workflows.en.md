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

## Governance

Publishing is configured in the team-state `config.toml`:

```toml
[governance]
publish = "any_member"   # default, the only supported value
```

- `any_member`: any member listed in `members.toml` may publish; validating the workflow is still required.
- A value this version of oh does not know **blocks publishing** (the rest of the configuration still works): update oh or go back to `any_member`.
