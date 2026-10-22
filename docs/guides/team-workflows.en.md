> [Lire en français](team-workflows.fr.md)

# Team workflows (team-state)

> v5, phase 2. The declarative workflows (`apiVersion: oh/v1`) of a team and its projects live in the team-state repository. This guide covers where they are stored, the integrity check and locks.

See also: [`oh/v1` schema](../reference/workflow-schema.en.md) (reference of every field, patch rules and diagnostics).

## Layers

A workflow is resolved from the most general layer to the most specific one:

| Layer | Location |
|---|---|
| `hub` | workflows shipped with oh (`~/.oh/hub/workflows/`) |
| `team` | `team-state/workflows/published/<id>.yaml` |
| `project` | `team-state/projects/<project>/workflows/published/<id>.yaml` |

A workflow with the same id as a workflow of a lower layer **must extend it** (`extends`), otherwise it is refused. Security fields may only be hardened. A team layer that **adds** an agent or skills (`skills.extra`) to a hub workflow widens the workflow: the addition is accepted with a warning (`patch_agent_added`, `patch_skills_extra_added`) and shown in the impact summary of the publication; the project layer, whose content comes from the repository, may add neither an agent nor skills (errors `patch_agent_refused`, `patch_skills_extra_refused`, the addition is ignored); removing skills stays allowed. An added agent must respect the `risk:` of the workflow (for example, an agent that writes is refused in a read-only workflow).

## Layout

```
team-state/
├── workflows/
│   ├── published/<id>.yaml            # published versions (loaded)
│   ├── drafts/<member>/<id>.yaml      # drafts (never loaded for other members)
│   ├── prompts/<id>.md.tmpl           # prompt templates
│   └── history/<id>/<version>.yaml    # previous versions (+ <version>.prompt.md.tmpl)
├── projects/<project>/workflows/…     # same layout for a project
├── catalog/{agents,skills}/           # team bricks (+ drafts/, history/)
├── catalog.lock                       # integrity of the published bricks
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

These warnings show up in `oh workflow list`, `oh workflow validate`, the TUI catalogue and `oh doctor` ("Team workflows"). A published file that cannot be read (syntax error) is listed as invalid **in its own layer** (team or project). To fix: publish the workflow again from oh, or restore the file (`git checkout`).

## Checking team workflows

```bash
oh workflow validate team:ticket-hotfix            # active team
oh workflow validate project:ticket --project web  # project layer + its team
oh workflow validate --all --project web           # hub, team and project
oh workflow validate ./hotfix.yaml --layer team    # file, with the team bricks
```

Validation uses the **merged brick catalogue** (hub + the team `catalog/`), like `oh run` and publication: a workflow using a team agent is valid.

`oh workflow list [-p <project>|--team <id>]` also shows **your drafts** (✎, error count, "new brick"), the files skipped by the integrity check and the publications waiting for the network (⏳). In JSON, drafts are entries of the same array with `"draft": true`.

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

## Approver of a checkpoint (`approver:`)

The team may name the people who validate a sensitive checkpoint (CP-R9, [ADR-055](../architecture/adr/055-checkpoint-approver.en.md)), for instance a lead for `cp-apply` and `cp-publish`:

```yaml
apiVersion: oh/v1
kind: Workflow
id: infra-change
extends: hub:infra-change
checkpoints:
  cp-apply:
    approver: [Grace Hopper, lead@example.com]   # git user.name or user.email
```

- Only the **team layer** declares it; a project layer can neither remove nor change it, nor remove the checkpoint, nor make it pause less (`checkpoint_approver_layer`, `approver_checkpoint_removed`, `loosening`).
- Each approver writes their identity as in their git configuration (`git config user.name`, or `user.email`): that is what oh compares, as for the author of Beads writes.
- **On the machine, oh warns without blocking**: the card shows "Expected approver: …", a validation by someone else is accepted and traced as "out of approver" (`oh session trace`). Who validates and when is always traced.
- In a remote session, prefer `remote: defer` for these checkpoints: with `remote: auto`, the policy passes them without an approver (warning). Real blocking in a remote session is step 2.

## Drafts, publication, history

The life cycle of a team or project workflow goes through the WorkflowService (CLI: `oh workflow new|edit|diff|publish|history|restore|archive`, see the [reference](../reference/cli-workflows.en.md#editing-team-and-project-workflows)):

1. **Draft**: `workflows/drafts/<member>/<id>.yaml` (and its own template `<id>.prompt.md.tmpl` when it has one). It is **validated when saved** (refused with its errors), pushed with the team-state, but never loaded for other members.
2. **Test**: `oh run <id> --draft` runs the draft, locally only, and refuses it when it widens the published version (a draft cannot loosen security).
3. **Publication**: team-state pull → **revalidation** of the draft against the up-to-date state → version + 1 → previous version copied to `history/<id>/` (document, template and lock entry) → published file and `workflows.lock` → commit + push. If another member published in the meantime, everything is **redone** on top of their publication (new revalidation, next version). The draft is consumed; a `workflow.published` event is added.
4. **Offline**: the publication is **queued** (in the clone, not versioned) and replayed later with the same cycle; the draft is kept. The replay is **automatic at each team-state synchronization in the TUI** (the result is notified); from the CLI: `oh workflow publish --retry` (no replay when the CLI starts).

An agent cannot publish for you: the global shell guard refuses `oh workflow publish`, `archive` and `restore` (and `oh brick publish`, `oh purge`, `oh project remove`, `oh teams remove`, `archive` and `restore`), `--yes` included, since a published version can widen the permissions of the whole team. Reads (`list`, `show`, `diff`, `history`) stay allowed. Run these commands yourself, from your terminal.

### Impact summary

Every publication compares the new version with the previous one: risk raised, new agents (and those that write), remote execution or new runtimes allowed, checkpoints removed, made optional or loosened, approver (`approver:`) removed or changed, Beads commands added, budget raised, new MCP servers or plugins, Code Mode, inputs and prompt changed. Changes that **widen** the workflow are flagged. Team catalogue bricks used for the first time get the "new brick" badge.

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
- A symbolic link of the catalogue is skipped: nothing outside the repository reaches a bundle.

### Editing and publishing with `oh brick`

Bricks are managed like workflows ([reference](../reference/cli-bricks.en.md), [ADR-054](../architecture/adr/054-team-brick-catalogue.en.md)):

```bash
oh brick list --skills --origin                  # hub and team, origin and version
oh brick new skill team/hotfix-protocol          # draft from a valid template
oh brick new agent reviewer --extends            # replace a hub agent explicitly
oh brick edit team/hotfix-protocol               # editor, validation when saved
oh brick publish team/hotfix-protocol -m "First protocol"
oh brick history team/hotfix-protocol
```

- **Validation**: hub rules (`oh skill check`); a skill description starts with "Utiliser …" and is 200 characters at most; the frontmatter of an agent is checked (id, description, mode, keys, skills, `permission_base`).
- **Publication**: team governance (below), revalidation on the up-to-date team-state, previous version in `catalog/history/`, hash in `catalog.lock`, `brick.published` event. Replacing a hub brick is reported and asks for confirmation.
- **Integrity**: once `catalog.lock` exists, a brick edited by hand or committed without oh is skipped (`oh brick list`, `oh doctor`). A catalogue committed by hand before `oh brick` keeps loading; it is sealed at once with `oh brick publish --adopt -m "…"`, required before the first publication.
- No import of external bricks; annexes (`skills/templates/`) stay committed by hand.

## Governance

Publishing is configured in the team-state `config.toml`:

```toml
[governance]
publish = "any_member"   # default, the only supported value
```

- `any_member`: any member listed in `members.toml` may publish (workflows and bricks); validation is still required.
- A value this version of oh does not know **blocks publishing** (the rest of the configuration still works): update oh or go back to `any_member`.

## Project without a team: solo space

A project without a team keeps its workflows in a **solo space**: a local team-state, without remote, in `~/.oh/teams/<id>/`.

```bash
oh team init --solo --project web-app     # create the space and attach the project
oh team promote --remote <empty-url>      # later: share it with a team
```

Publications there are local commits. `oh team promote` pushes the whole history to the remote, without loss (see [team CLI](../reference/cli-team.en.md#oh-team-promote)).

In the TUI:

- **first run**: "Solo space (local workflows)" button at the Team step (the "solo" mode of the welcome step selects it); the wizard's project is attached to it;
- **adding a project**: "Create a solo space" / "Solo space <id>" choice at the Team step (the existing space is reused);
- **workflow catalogue**: `n` without a team-state offers to create the solo space (id, member, attachment of the active project);
- **team detail** of a solo space: "Space" line and **"Switch to a team"** action (URL of an empty remote repository, confirmation, then the URL to send to the members, who run `oh team init`). The team detail also shows the workflow **governance**, read only ("Publication: any member").

## Migration of the former workflow overrides (v5)

The overrides of the former single workflow (TUI Workflow tab) are migrated **automatically** at the first start of oh v5 (migration v38), to workflows extending `hub:implement-spec` (`hub:feature` before it became an alias; `cp-1`, `cp-2`, `cp-3` and their agents keep their ids, `cp-0` becomes `cp-plan`):

| Former configuration | Becomes |
|---|---|
| team-state `config.toml`, `[workflow]` section | draft then team workflow `implement-spec` (`extends: hub:implement-spec`; `enforced = true` → `enforce: ["*"]`), published when the team has none yet; section removed from `config.toml` |
| workflow configuration of a project (oh database) | draft then project workflow `implement-spec` (extends `team:implement-spec` when the team has one, else `hub:implement-spec`); a **solo space** is created for a project without team |
| `hub.toml`, `[workflow.overrides]` section (hub development) | files in `~/.oh/migrated/` (not loaded); section removed from `hub.toml` |

- **Nothing is lost**: the original configuration is archived (team-state `workflows/migrated/…`, `~/.oh/migrated/`); what has no equivalent (`cp-routing`, checkpoint agents or agents missing from `implement-spec`, `insert_after`, `task_permissions`) is listed as a comment at the top of the migrated workflow.
- An invalid migrated workflow, or a scope that already has a published `implement-spec`, stays **your draft**: `oh workflow diff implement-spec`, `oh workflow edit implement-spec`, then `oh workflow publish`.
- When the team locked its workflow, the project overrides (ignored before) stay a draft: the lock refuses their publication.
- Offline, the migration of a team is simply retried at the next start.
- The former TUI Workflow view and `oh deploy` were removed in v5: workflows are browsed in the catalogue (`workflows`) and edited in the editor.

## Retired workflows: `feature` and `cadrage`

`feature` and `cadrage` are no longer hub workflows but aliases: `cadrage` and `feature` launch `spec`, `feature --tickets …` launches `implement-spec`. A team or project workflow that **extends** `hub:feature` or `hub:cadrage` no longer resolves: oh refuses it with an explicit error (`extends_alias`), at launch as with `oh workflow validate`, and `oh doctor` reports it ("Retired workflows").

oh does not migrate it on its own: its changes target agents and checkpoints (`orchestrator`, `cp-0`, `cp-spec`, `cp-feature`) that do not exist in the target workflows, and a silent redirection could lose a rule of the team. To migrate it:

```bash
oh doctor                                   # lists the workflows concerned
oh workflow edit feature                    # extends: hub:spec or hub:implement-spec, agents and checkpoints adapted
oh workflow publish team:feature
# or: oh workflow archive team:feature      # the alias applies again
```

A rewritten workflow that keeps the id `feature` shadows the alias (`oh run feature` launches it, warning `alias_shadowed`); under another id (`team-feature`), the alias stays available. Details: [Shipped workflows › `feature`](../reference/workflows.en.md#feature-alias-of-spec--implement-spec).
