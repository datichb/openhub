> [Lire en français](workflow-schema.fr.md)

# Reference — `oh/v1` workflow schema

A workflow is a YAML file `apiVersion: oh/v1`, `kind: Workflow`, named `<id>.yaml`. This page describes every field: type, values, effective value when the field is absent, behavior in a patch (`extends`) and locking (`enforce`).

The schema is **frozen** (`cli/internal/workflow/schema.go`, type `Spec`): it only changes through optional additions. Additions made after the freeze: `risk: plan`, `preconditions`, `enforce`, `limits.models`, `limits.max_fix_rounds`, `entry.selectable`, `checkpoints.<id>.verify` and `checkpoints.<id>.when` (R3), `checkpoints.<id>.by` (L3), `checkpoints.<id>.wiki` (LD-D), `checkpoints.<id>.method` and the `must_touch_tests`, `outside_tests`, `since` fields of `verify` (R7), `agents.<id>.writes` and `verify.spec` (R4), `agents.<id>.writes_after` and `checkpoints.<id>.after` (r5-fixes), `fanout.source`, `checkpoints.<id>.tasks`, `agents.<id>.fanout` and `record: tasks` (sweep-fanout), `inputs.<id>.from_equals` and `inputs.<id>.scope` (p6-debts). Decision: [ADR-039](../architecture/adr/039-declarative-workflows-oh-v1.en.md).

See also: [shipped workflows](workflows.en.md) · [team workflows](../guides/team-workflows.en.md) · [workflows CLI](cli-workflows.en.md).

```bash
oh workflow validate ./my-wf.yaml        # a file
oh workflow validate ticket --json       # a workflow of the catalogue
oh workflow show ticket --origin         # resolved values and source document
```

---

## General rules

- **Strict reading**: an unknown field, a duplicate key or a value of the wrong type is an error. Every error of the file is reported, with line and column.
- **One document** per file (no extra `---`). The file name is the id (`ticket.yaml` ↔ `id: ticket`).
- **Translatable texts** (`label`, `description`, `help`): a plain text, or a table per language `{ fr: …, en: … }`. Without the requested language, oh takes the plain text, then `en`, then `fr`.
- **Ordered maps**: `inputs`, `agents`, `checkpoints` and `preconditions` keep the file order (order of the launch form, of the checkpoints and of the checks).
- **Patch** (`extends`): the **presence** of a field in the file replaces it, not its value. Maps are merged by key; lists and texts are replaced as a whole.
- **Security**: `risk`, `isolation`, `runtime.allowed`, `modes.allowed`, `code_mode`, `beads.allow`, mandatory checkpoints (and checkpoints with `unlocks`), `remote`, `limits`, `after` gates, `calls` lists, `circuit_breaker`, the preconditions of a `risk: publish` workflow, and outside the hub layer `mcp` and `plugins`, may only **be hardened**. A loosening is an error (`loosening`) and the parent value is kept.
- **Lock**: a top-level field listed in the `enforce` of a parent document can no longer be written by a document that extends it (`enforced_field`).

In the tables below, the **Patch** column reads:

- **replaces**: the written value replaces the parent one;
- **merge**: merged by key (absent keys are inherited);
- **hardens**: replaces only when the value is at least as strict, otherwise `loosening` error.

Every top-level field can be locked by `enforce`, except `apiVersion`, `kind`, `id`, `version`, `extends` and `enforce`.

---

## Header

| Field | Type | Values | Absent | Patch |
|---|---|---|---|---|
| `apiVersion` | text | `oh/v1` | required | — |
| `kind` | text | `Workflow` | required | — |
| `id` | text | kebab-case (`^[a-z0-9]+(-[a-z0-9]+)*$`), equal to the file name | required | same id as the parent |
| `version` | integer | managed by publication, never by hand | `0` | — |
| `category` | text | `develop`, `frame`, `quality`, `knowledge`, `other` | listed under "other" by the catalogue | replaces |
| `label` | translatable text | displayed name | the id is shown | replaces |
| `description` | translatable text | one sentence | empty | replaces |
| `extends` | text | `hub:<id>`, `team:<id>`, `project:<id>` | full definition | — |
| `enforce` | list | top-level fields, or `"*"` | no lock | locks add up |

### `extends`

- The parent must be in a **less specific or the same layer** (`extends_more_specific`).
- A workflow with the same id as a workflow of a lower layer **must extend it** (`shadow_without_extends`), and extend **the nearest one** (`must_extend_nearest`): a `project:ticket` extends `team:ticket` when it exists, otherwise `hub:ticket`.
- Another id is allowed: `team:ticket-hotfix` may extend `hub:ticket`.
- A parent that became an **alias** (`hub:feature`, `hub:cadrage`) is no longer a workflow: `extends_alias`, with the workflows declaring the alias; `oh doctor` lists these documents (see [`feature`](workflows.en.md#feature-alias-of-spec--implement-spec)).

### `enforce`

```yaml
enforce: [checkpoints, modes]   # or ["*"] for the whole document
```

- Values: `category`, `label`, `description`, `risk`, `isolation`, `code_mode`, `entry`, `inputs`, `prompt`, `agents`, `checkpoints`, `modes`, `circuit_breaker`, `models`, `skills`, `plugins`, `mcp`, `beads`, `runtime`, `outputs`, `limits`, `preconditions`, `headless`, `next`, `aliases`, or `"*"`. Any other value: `enforce_unknown_field`.
- The lock covers the whole top-level field (`checkpoints` locks every checkpoint).
- Locks add up along the `extends` chain; a more specific layer cannot remove any.
- Launch options (mode, environment, inputs) stay allowed, within the limits of the workflow.

---

## Security

| Field | Type | Values | Absent | Patch |
|---|---|---|---|---|
| `risk` | text | `read` < `plan` < `write` < `publish` | **required** (`field_required`) | hardens (equal or lower rank) |
| `isolation` | text | `strict`, `standard` | `standard` | hardens (`strict` does not go back to `standard`) |
| `code_mode` | boolean | `true`, `false` | `false` | hardens (`false` does not go back to `true`) |
| `beads.allow` | list | `bd` subcommands | see below | hardens (subset of the parent) |
| `runtime.default` | text | `local`, `container`, `remote` | `local` | replaces |
| `runtime.allowed` | list | `local`, `container`, `remote` | `[runtime.default]` | hardens (subset of the parent) |
| `limits.budget_usd` | number ≥ 0 | USD per session | no budget at workflow level | hardens (lower or equal value) |
| `limits.models` | list | patterns (`*` wildcard) | no list at workflow level | hardens (patterns covered by the parent) |
| `limits.max_fix_rounds` | integer ≥ 0 | fix rounds per ticket | `0` (no cap) | hardens (lower or equal value, never `0` or absent under a capped parent) |
| `checkpoints.<id>.verify` | object | see [Checkpoints](#verify--verification-run-by-oh) | no verification | hardens (never removed nor weakened) |
| `checkpoints.<id>.when` | object | see [Checkpoints](#when--conditions-evaluated-by-oh) | no condition | hardens (inherited conditions kept) |
| `checkpoints.<id>.wiki` | text | `sync` | ordinary role | replaces |
| `checkpoints.<id>.method` | text | `tdd` | checkpoint of every session | not set on an inherited checkpoint (`loosening`) |

### `risk`

| Value | Files | Beads | Checks |
|---|---|---|---|
| `read` | no edits, no shell | read only | no agent with edit or shell (`read_agent_writes`); `beads` required (`read_beads_unrestricted`), without write commands (`read_beads_write`) |
| `plan` | no edits, no shell | writes limited to `beads.allow` | no agent with edit or shell (`plan_agent_writes`); `beads` required (`plan_beads_unrestricted`), without `delete` (`plan_beads_delete`); warning when there is no write at all (`plan_without_beads_write`: `read` is enough) |
| `write` | edited | no schema restriction | — |
| `publish` | edited, tags created, MRs opened | same | never remote: `remote` not in `runtime.allowed` (`publish_remote_runtime`); every checkpoint that unlocks `push`, `tag` or `mr` says `remote: forbid` (`publish_checkpoint_remote`) and pauses in every allowed mode (`publish_checkpoint_pause`) |

Beads commands counted as writes: `create`, `update`, `close`, `reopen`, `delete`, `edit`, `comment`, `comments`, `label`, `dep`, `duplicate`, `supersede`.

Only a `publish` workflow unlocks `tag` and `mr` (`unlock_requires_publish`). oh also refuses to send it remotely (`oh run --runtime remote`), and the job runner refuses it before the clone: the decision to publish is always taken on the user's machine.

A writing session (`risk` other than `read`) gets a worktree when another writing session is active in the same place (see [workflows CLI](cli-workflows.en.md#oh-run)).

### `isolation`

- `strict`: the closed world must be complete. No permission may be granted "always" from oh. When the adapter does not report full isolation, validation warns (`isolation_unsupported`) and the launch is refused.
- `standard` (or absent): partial isolation is accepted.
- Independent from `[execution] strict_isolation` in `hub.toml` (which hides the user's own tool configuration). See [ADR-041](../architecture/adr/041-closed-world-isolation.en.md).

### `code_mode`

`false` or absent: the opencode `execute` tool is denied to every agent. `true`: it stays available. The field is part of the bundle hash. It is a security field: a layer that extends a workflow may disable it, not enable it (`loosening`); the publication impact summary reports enabling it.

### `beads`

```yaml
beads: { allow: [show, list, update, close] }
```

- Absent: no declared restriction. The Beads gateway then applies a read-only list (`show`, `list`, `ready`, `search`, `query`, `children`, `comments`, `count`, `status`, `graph`, `history`). Every shipped workflow declares its list.
- `allow: []`: no command allowed.
- A list that names only read commands (`show`, `list`, `query`, `comments`, `dep tree`…) is read-only: the write forms of those commands are refused (`comments add`, `ready --claim`). With `risk: read`, every entry must name a read command (`read_beads_write`). See [ADR-052](../architecture/adr/052-beads-identity-provenance.en.md).
- The list is applied by the daemon's Beads gateway in every runtime (locally, oh's fake `bd` comes first on the session `PATH`; a `bd` called by a path is refused) and when the journal is replayed (remote). See [ADR-046](../architecture/adr/046-beads-gateways.en.md).
- Patch: a parent without `beads` stands for the default read-only list (the one the gateway applies: `show`, `list`, `ready`, `search`, `query`, `children`, `comments`, `count`, `status`, `graph`, `history`): the child list must be a subset of it (`create`, `close`… are a loosening). On a parent with a list, the new list must be a subset of it; `beads: null` is a loosening.

### `runtime`

```yaml
runtime: { default: local, allowed: [local, container, remote] }
```

- `runtime.default` must be in `runtime.allowed` (`runtime_default_not_allowed`).
- Choice at launch, within `allowed`: `--runtime` > project default runtime > Settings > `runtime.default`. A preference that is not allowed is skipped (`PickRuntime`); a `--runtime` that is not allowed is an error (`session_runtime_not_allowed`).
- Patch: when only `default` is written, the parent's effective list is kept (a new default does not widen the list).
- `remote` allowed ⇒ no `remote: forbid` checkpoint that waits for the user (`remote_forbidden_checkpoint`).
- See [ADR-045](../architecture/adr/045-execution-environments.en.md).

### `limits`

```yaml
limits:
  budget_usd: 5                       # soft cap per session
  models: ["eu.anthropic.claude-*"]   # allowed models
  max_fix_rounds: 3                   # developer → reviewer fix rounds per ticket
```

- Adds up with the I6 restrictions of the hub, the team and the project (`oh budget show`): the most specific value wins (workflow > project > hub > team recommendation); a value enforced by the team stays a ceiling. See [ADR-044](../architecture/adr/044-credential-proxy-session-limits.en.md).
- `budget_usd`: soft cap, checked between two steps; beyond it, a `$` decision asks to raise it or stop. Negative value: `negative_value`.
- `models`: patterns on the id sent to the provider; the credential proxy refuses the others. An empty pattern: `field_required`.
- `max_fix_rounds`: cap on the fix rounds (developer → reviewer) per ticket. It is an instruction, written in the « Mode de workflow » section of the generated skills (« Au plus N tours de correction… ; au-delà, passe le checkpoint de commit en signalant les points non résolus. »), **and a lock** (R3): oh counts the developer → reviewer pairs of the current ticket; past the cap, it refuses delegations to the `developer*` agents and marks the commit checkpoint "unresolved", until the user gives an instruction at that checkpoint or the ticket is closed ([Checkpoints](checkpoints.en.md#fix-rounds-limit-limitsmax_fix_rounds)). `0` or absent: no cap. A layer may only lower it (`loosening` otherwise). Negative value: `negative_value`. The hub `ticket`, `implement-spec` and `review-feedback` workflows set `3`.

---

## Entry and modes

| Field | Type | Values | Absent | Patch |
|---|---|---|---|---|
| `entry.agent` | text | agent id of the catalogue, primary | `conductor` | replaces the `entry` block |
| `entry.selectable` | boolean | `true`, `false` | `false` | to be written with `entry.agent` |
| `modes.default` | text | a mode of `modes.allowed` | first allowed mode | replaces |
| `modes.allowed` | list | `manuel`, `semi-auto`, `auto`, or custom modes | `[manuel, semi-auto, auto]` | hardens (subset of the parent; an empty list means every mode) |
| `circuit_breaker.max_consecutive_subagents` | integer ≥ 0 | N delegations in a row without interaction | `0` (breaker disabled) | hardens (lower value; `0` does not replace an active breaker) |

### `entry`

- The entry agent must be **primary** (`entry_not_primary`) and, when listed in `agents`, have the `workflow` role (`entry_role`). It cannot be removed by a patch (`entry_disabled`).
- `conductor`: generic agent without write or shell, which follows the generated workflow map and delegates.
- `selectable: true`: the entry agent is chosen at launch (`oh run libre --agent debugger`). Members are then computed: the chosen agent and those it may call (`task` permission of the catalogue, step by step); `agents:` is computed the same way for the default agent. On a workflow without `selectable`, `--agent` is refused (`session_entry_not_selectable`).
- Patch: the `entry` block is replaced when `entry.agent` is written (or `entry: null`); `entry.selectable` written alone changes the agent choice and keeps the parent's entry agent.

### `modes`

- The mode is set at launch (`--mode`, launch form) and is no longer asked by the agent. The prompt always contains `Mode de workflow : <mode>`.
- Meaning of the three modes of the hub workflows: `manuel` stops at every checkpoint; `semi-auto` passes the usual checkpoints on its own and stops on the key decisions; **`auto` = everything except the mandatory decisions**: only the `mandatory` checkpoints and those that unlock an action (`unlocks`: `create`, `commit`, `close`, `apply`, `tag`, `mr`) stop, a `conditional` one stops when it is signalled. A workflow whose every step must be approved is limited to `manuel` (`infra-change`).
- Patch: `modes.allowed` may only be narrowed (`loosening` for a mode missing from the parent); the impact summary reports any added mode as a loosening.
- A duplicate mode: `duplicate_entry`; a default missing from the list: `mode_default_not_allowed`; a `--mode` that is not allowed: `session_mode_not_allowed`.

### `circuit_breaker`

After N consecutive subagent calls without user interaction, the session stops on a ✗ decision (circuit breaker). Negative value: `negative_value`.

### `headless`

`headless: true`: nobody follows the session (`brief-enrich`, launched without interface). The session contract tells every agent: no question, a missing piece of information goes in the output; the `when: headless` condition of the skills holds. A checkpoint that may wait for the user gives the `headless_checkpoint_pause` warning. Patch: replaces.

---

## Inputs (`inputs`)

```yaml
inputs:
  ticket:
    type: beads-id
    required: true
    label: { fr: Ticket, en: Ticket }
    help: Beads ticket delegated to the AI
    picker: { filter: ai-delegated, multi: true }
  branch: { type: string, default: "feat/{{ .ticket }}" }
```

An input id follows `^[a-z][a-z0-9_]*$` (`input_id_invalid`); `oh` is reserved for the session context (`input_id_reserved`).

| Field | Type | Absent | Role |
|---|---|---|---|
| `type` | text | required | see the type table |
| `required` | boolean | `false` | without a value or default: `session_input_missing` |
| `default` | value of the type | empty value of the type | may refer to another input (`{{ .ticket }}`), not to itself |
| `label`, `help` | translatable text | the id | launch form |
| `values` | list | — | choices of an `enum` (required for `enum`, ignored otherwise) |
| `picker` | table | — | ticket picker (`beads-id`, `beads-ids` only) |
| `max_length` | integer ≥ 0 | `0` = 20,000 characters | truncation of the injected value |
| `from` | `source(input)` | — | value computed by oh at launch when it is not given (see below) |
| `from_equals` | table output → value | — | with `from: session.output(…)`: only reads the sessions whose outputs have these values (see below) |
| `scope` | boolean | `false` | the input (`text` or `string`, one per workflow, recorded by a `record: input` output of the same id) is the **file scope** of the session: one entry per line or comma (path, folder, pattern of the repository, or a new file in an existing folder, never absolute nor outside the repository), checked by oh at launch (launch refused with the list of problems); at the checkpoints oh verifies, a change outside the scope refuses the passage. `sweep-task`: `files` (`input_scope_type`, `input_scope_twice`, `input_scope_unrecorded`) |

| Type | Value | In the prompt |
|---|---|---|
| `string` | one line of text | text, to be passed through `data` |
| `text` | free text, several lines | text, to be passed through `data` |
| `bool` | `true` / `false` (`-i x=true`) | boolean (`false` by default) |
| `int` | integer | integer (`0` by default) |
| `enum` | one of the `values` | text |
| `path` | path, a single line | text |
| `branch` | branch name, a single line | text |
| `beads-id` | one Beads id (several with `picker.multi`) | ids joined with `, ` |
| `beads-ids` | list of Beads ids (`a,b` or YAML list) | list (`join`) |

- **`picker`**: `filter` (e.g. `ai-delegated`), `epic` (restrict to an epic), `multi` (several tickets). A `beads-id` input with `multi: true` gives **one session per ticket** (`--tickets a,b`); a `beads-ids` input receives the whole list in a single session.
- **Checks**: an invalid Beads id, or a multi-line `path` / `branch`, is refused at render time. A value that does not match the type: `session_input_invalid`; an unknown input: `session_input_unknown`; an invalid default: `input_default_invalid`.
- **Truncation** (O11): `string`, `text`, `path`, `branch` values are cut to `max_length` characters, with the note `[… tronqué : N caractères sur M]`.
- **Computed inputs** (`from`): see [below](#computed-inputs-from).
- **Patch**: merged by id, field by field (`picker` too). An input cannot be removed.
- **`method`** (R7): the `method` input, an `enum` (`method_input_type`) with the values `standard` and `tdd`, chooses the **method of the session**; see [`method`](#method--method-of-the-session).

### Computed inputs (`from`)

`from: <source>(<input>)` asks oh to compute the input at launch, from the value of another input of the same workflow:

```yaml
inputs:
  mr: { type: string, required: true }
  feedback: { type: text, required: true, from: mr.discussions(mr) }
```

| Source | Computes | Argument |
|---|---|---|
| `mr.discussions(mr)` | the unresolved discussions of the merge request | a merge request: URL, `!iid`, number, branch or ticket |
| `mr.source_branch(mr)` | its branch | same |
| `mr.target_branch(mr)` | its target branch | same |
| `ticket.brief(ticket)` | the takeover brief of the ticket (the enriched version first) | a ticket id |

- **Neutral names**: oh resolves a source with the forge of the project (GitLab today: `oh service setup` token, `tracker_project` project) or its team space; a workflow does not change with the forge. The list is closed, and open to every workflow (shipped, team, project).
- **Priority**: a value given at launch wins, then the computed value, then the default.
- **Failure**: when the computation fails, the launch is refused for an input without default; otherwise the default applies. The source is read once per launch.
- The launch form does not ask for these inputs (they stay editable).
- Checks: `input_from_invalid` (syntax), `input_from_unknown_source`, `input_from_unknown_input` (missing argument, or the input itself).

#### Sources read from a previous session (R5)

```yaml
inputs:
  tickets: { type: beads-ids, required: true, from: session.output(spec.tickets) }
  digest:  { type: text, from: session.trace(ticket) }
```

| Source | Computes | Session read |
|---|---|---|
| `session.output(<workflow>.<output>)` | the value of an [output](#outputs-outputs) (a list of tickets joined by commas) | the session the launch is chained from (`oh run … --parent <id>`) when it is a session of that workflow with that output; otherwise the **last** session of the project of that workflow that has it |
| `session.trace()`, `session.trace(<workflow>)` | a summary of the trace of the session (checkpoints, refused decisions, refused calls, skills, delegations, outputs, totals; 8,000 characters at most), for a retrospective | the parent session, otherwise the last finished session of the project (of that workflow) |

- **`from_equals`** (with `session.output` only): only reads the sessions whose named outputs were recorded with these values, the parent session included. `security-fix`: `from_equals: { type: security }`, the last security audit, never one of another type. No fitting session: the input stays empty and the launch says so (“…: no session fits, the input stays empty”); an output unknown to the workflow read is refused (`input_from_unknown_output`), as is `from_equals` on another source (`input_from_equals`).
- No argument input: these sources are computed without another input. The launch shows the session read (“session.output(spec.tickets): session ses_… ev-1,ev-2”).
- Nothing found: the input stays empty; a required input is then refused at launch (`session_input_missing`).
- Checks: the named workflow exists (`input_from_unknown_workflow`), declares the output (`input_from_unknown_output`), the type of the output fits the input (`input_from_type`: `beads-ids` → `beads-ids`/`beads-id`, `path` → `path`/`string`/`text`, `branch` → `branch`/`string`, `report` → `string`/`text`; `session.trace` → `text`); `session.output(<output>)` without a workflow is refused in `from:` (`input_from_session_workflow`).

---

## Prompt

```yaml
prompt:
  template: prompts/ticket.md.tmpl   # or text: "…"
```

| Field | Type | Role |
|---|---|---|
| `template` | path | Go `text/template`, relative to the `workflows/` directory of the layer that wrote it, without `..` (`prompt_template_path`) |
| `text` | text | inline template |

- Exactly one of the two (`prompt_both`, `prompt_empty`). Absent: no first message (the session waits for the user).
- **Variables**: inputs at the top level (`{{ .ticket }}`); the session context under `.oh`: `.oh.project`, `.oh.location`, `.oh.mode`, `.oh.runtime`, `.oh.lang`, `.oh.workflow`, `.oh.branch`, `.oh.memories` (persistent memories of the project's Beads, `bd remember`, already inside data tags; empty without any). Any other variable: `prompt_unknown_variable`.
- **Functions**:
  - `{{ data "request" .request }}` writes the value, truncated. A computed input (`from:`) goes between `<oh:data name="…">` and `</oh:data>`: the agent treats it as data, never as instructions (a tag inside the value is neutralised). An input typed by the user is written as it is: it is their request. Every `string` or `text` input must go through `data`.
  - `{{ join .tickets ", " }}` joins a list.
- An absent input takes its default, otherwise the empty value of its type: `{{ if .request }}…{{ end }}` works.
- oh ends the prompt with the list of checkpoints to report before each gated agent.
- Patch: the `prompt` block is replaced as a whole.

---

## Agents

```yaml
agents:
  orchestrator-dev: { role: workflow, calls: [developer, reviewer] }
  developer: { role: workflow, after: cp-1 }
  reviewer: { role: workflow, mode: subagent, after: developer }
  documentarian: { role: independent }
```

The key is the id of an agent of the brick catalogue (hub + team), in kebab-case (`agent_id_invalid`, `agent_unknown`). Only the entry agent and these agents are in the bundle (closed world).

| Field | Type | Values | Absent | Role |
|---|---|---|---|---|
| `role` | text | `workflow`, `independent`, `disabled` | required | `workflow`: in the chain, must be reachable from the entry (`agent_unreachable`); `independent`: available on demand, outside the chain; `disabled`: removed (patch) |
| `mode` | text | `primary`, `subagent` | mode of the agent in the catalogue | `primary`: the user may pick it; `subagent`: only called by delegation |
| `after` | text | checkpoint or agent id | no gate | the agent can only be called after this checkpoint or agent (`after_unknown`, `after_cycle`) |
| `calls` | list | agent ids of the workflow | `task` permission of the agent, restricted to the workflow | allowed delegations (`calls_unknown`, `graph_cycle`) |
| `scratch` | table | `{ readers: [<agent>…] }` | no scratch folder | session scratch folder of the agent (see [below](#scratch-session-scratch-folders)) |
| `writes` | list | paths under `docs/`: a file, or a folder ending in `/**` | no write opened | documentation paths the agent may write and commit (see [below](#writes-documentation-paths)) |
| `writes_after` | text | checkpoint id | writes open from the start | the `writes` paths only open once this checkpoint is passed (see [below](#writes-documentation-paths)) |
| `fanout` | text | `only`, `never` | agent of every session | `never`: removed from the fan-out sessions (the `developer` of `sweep`, whose work the children do); `only`: from the other ones. Needs a fan-out (`agent_fanout_without_fanout`), never on the entry agent (`agent_fanout_entry`); only changed in the hub layer (`loosening`) |

- **Graph**: `calls` when written, otherwise the agent's `task` permission, limited to the members. A self-delegation is only kept when written (`reviewer: { calls: [reviewer] }`). The maximum depth of the graph becomes opencode's `experimental.subagent_depth`.
- **Patch**: merged by id, field by field; `role: disabled` removes the agent (except the entry agent) and drops it from the `calls` lists of the others. `after` cannot be removed nor moved earlier (only to a checkpoint declared later); `calls` written by the parent may only shrink (subset); an added agent is accepted.

### `scratch` (session scratch folders)

```yaml
agents:
  debugger: { role: workflow, scratch: { readers: [developer] } }
```

The agent gets a scratch folder for the session: `~/.oh/sessions/<session>/scratch/<agent>/`, outside the repository and the documentation, created 0700 at launch and deleted with the session (`oh gc`, `oh session delete`). Its path is given to the agents by the `oh.scratch` entry of the [session state](../guides/sessions-v5.en.md#session-state), and a line of their prompt points to it.

| Who | Read | Write | Shell |
|---|---|---|---|
| the owner (the agent that declares `scratch`) | its folder, in this session | its folder, in this session | no |
| a reader (`readers`) | the owner's folder, in this session | no | no |
| any other agent, and any agent of another session (parallel sessions included) | no | no | no |

- `readers`: other agents of the workflow, each once (`scratch_reader_unknown`, `scratch_reader_owner`, `duplicate_entry`). `scratch: {}`: the owner only.
- The rules are oh guards, placed after the rules of each agent (see [security](../../SECURITY.md)): an agent rule that allows everything does not loosen them. The shell never reaches the folder, the owner included: a shell rule follows the raw text of the command, an allow by path would allow any command naming it.
- A scratch file is never cited in the documentation or the wiki: a content lint checks it (`TestScratchNeverInTheDocs`).
- Patch: `scratch: null` removes the folder, a shorter `readers` list hardens, a reader the parent does not declare is a loosening (`loosening`); a folder added by the layer is accepted. A disabled agent leaves the readers.

### `writes` (documentation paths)

```yaml
risk: plan
agents:
  planner: { role: workflow, writes: ["docs/wiki/specs/**"] }
```

E8 (R4): an agent that changes no file (the `planner`) may write the declared documentation paths, and only them. The bundle adds, after the agent's own rules and before the guards of oh:

| Action | Rule |
|---|---|
| `edit`, `write` | the declared paths (`docs/wiki/specs/**` becomes `docs/wiki/specs/*`) |
| shell | `git add [--] <path>` and `git commit -m "<message>" -- <path>` (quoted message, a declared path or its folder): only these paths are committed |
| shell, read by oh (p6-debts) | several declared paths (`git add <p1> <p2>…`, `-- <p1> <p2>…`) or several `-m` (commit body): the rule asks, the oh plugin relays the line and oh reads it word by word: every path declared (a declared file, a declared folder or a path inside it, without `..`, hidden part or option), nothing but `-m` messages before `--`, each message one word or one quoted string without expansion (`$`, backquote); otherwise refused with the reason. Without oh's answer the user is asked (and the commit stays held while oh has not confirmed the session rules) |
| shell, refused (p4-fixes) | any other path: `..`, `/.`, quote, `$`, backquote or redirection after the declared folder; an argument between the message and `--` (`-m x src/a.go -- …`, `-p`, `--pathspec-from-file`, `:(glob)…`) is read by oh and refused |
| shell, refused | `git commit --amend`, `-a`/`--all`, `--include`/`-i`, `--only`/`-o`, `-C`, `git add -A`/`--all`/`-u`/`.`/`-f` (they would take in the user's work) |

- The commit stays locked until the checkpoint that unlocks it (`unlocks: [commit]`); in the window of a first commit followed by another commit checkpoint (`unlock_again`), the `writes:` paths (and the folders above them, `git add -A`, `.`, `commit -a`) are refused: they go with the next commit (p6-debts); the guards of oh (`.git/`, `.beads/`, hooks, `--no-verify`, scratch folders, global guard) come after and win. Like the other guards, rules on the raw text of the commands: a guard rail, not a sandbox.
- The session contract of the agent lists its writes, the workflow map recalls them, and the `writes` condition of blocks (`<!-- oh:when writes -->`) keeps a text in those sessions only: the planner of `libre` keeps its “you never change files” prompt.
- Paths: relative, under `docs/`, a file or a folder ending in `/**`, no other wildcard, or one of the root files `ONBOARDING.md`, `CONVENTIONS.md`, `README.md`, `CHANGELOG.md` (files only, p6-debts) (`writes_path_invalid`), each once (`duplicate_entry`); refused in a `risk: read` workflow (`writes_read_only`).
- **`writes_after`** (r5-fixes): until this checkpoint is passed (a checkpoint skipped in the mode counts as passed), writing the `writes` paths is refused to every agent of the session, with a message telling it to wait for that checkpoint: the session rule of these paths is “ask”, the oh plugin relays the request and oh refuses it with its message (without an answer from oh, the user is asked). A checkpoint of the workflow present in every session (`writes_after_unknown`, `order_method_after`), with `writes` (`writes_after_without_writes`). `workflow_status` lists the writes still closed (`locked_writes`). Patch: like `after`, it is added or moved to a checkpoint declared later, never removed nor moved earlier (`loosening`); `writes: []` removes it with the paths.
- Patch, **narrowing only**: a team or project layer keeps, removes or narrows the inherited paths (`docs/wiki/specs/team/**` under `docs/wiki/specs/**`, `writes: []`); a path the parent does not cover, or `writes` on an agent that had none or on an agent added by the layer, is a loosening (`loosening`), and the parent's list is kept.

---

## Checkpoints

```yaml
checkpoints:
  cp-2:
    label: { fr: Commit ou correction, en: Commit or fix }
    description: After the review, ask whether to commit or fix.
    mandatory: true
    mode: { manuel: pause, semi-auto: pause, auto: pause }
    unlocks: [commit, close]
    remote: defer
```

The id is in kebab-case (`checkpoint_id_invalid`) and must not be an agent id (`checkpoint_agent_clash`). Checkpoints are passed in file order. The generated skills give this order on a « Séquence » line, with the operations each checkpoint unlocks (« `cp-1` → `cp-2` [commit, close] → `cp-3` »): the agents rely on it instead of hard-coding checkpoint ids. Each agent sees only **its own** checkpoints there (`by:`, CP-R11).

| Field | Type | Values | Absent | Patch |
|---|---|---|---|---|
| `label`, `description` | translatable text | — | the id | replaces |
| `mode.<mode>` | text | `pause`, `auto`, `skip`, `conditional` | `pause` (warning `checkpoint_mode_missing`) | merged by mode; hardens when `mandatory` |
| `condition` | text | sentence read by the agent | — (required with `conditional`) | replaces |
| `mandatory` | boolean | `true`, `false` | `false` | hardens (`true` does not go back to `false`) |
| `remote` | text | `auto` < `defer` < `forbid` | `defer` | hardens |
| `unlocks` | list | `commit`, `push`, `close`, `create`, `apply`, `tag`, `mr`, `merge` (merge checkpoint of a fan-out only) | — (nothing locked; `apply`, `tag` and `mr` stay refused) | added to the inherited list (never removed) |
| `disabled` | boolean | `true` | — | removes the inherited checkpoint (patch only; refused when it carries `verify`) |
| `verify` | object | `from`, `kinds`, `cmd`, `expect`, `in`, `timeout`, `must_touch`, `must_touch_tests`, `outside_tests`, `since`, `spec` | no verification | hardens (an inherited verification is never removed nor weakened) |
| `when` | object | `entry`, `labels`, `paths`, `outputs` | no condition | hardens (inherited conditions kept) |
| `by` | list | member agents of the workflow | deduced from the graph: the agents that launch an agent gated by this checkpoint (`after:`), else the entry agent | replaces |
| `wiki` | text | `sync` | the card shows the wiki proposals of the session | replaces |
| `method` | text | `tdd` | checkpoint of every session | not set on an inherited checkpoint (`loosening`); an added checkpoint may carry one |
| `after` | text | id of a checkpoint declared before | no order enforced | hardens: added or moved to a checkpoint declared later, never removed nor moved earlier (`loosening`) |
| `fanout` | text | `only` (fan-out sessions only), `never` (the other ones only) | checkpoint of every session | never set nor changed on an inherited checkpoint outside the hub layer (`loosening`) |
| `tasks` | object | `max` (1 to 20, default 12), `domains` | no breakdown read | oh reads, checks and records the breakdown given with the call (see [below](#tasks--breakdown-read-by-oh)); a layer lowers `max` and narrows `domains`, without removing the field (`loosening`) |

- **`after`** (r5-fixes): oh refuses the checkpoint until the one it waits for is passed (a checkpoint skipped in the mode, or absent from the session, counts as passed); the refusal names the checkpoint to pass first, nothing is unlocked and the user is not asked. A checkpoint declared before (`checkpoint_after_order`, `checkpoint_after_unknown`), present in every session (`order_method_after`). The workflow map shows it on the line of the checkpoint.
- **Behaviors**: `pause` always asks the user; `auto` goes on alone; `skip` skips the checkpoint; `conditional` asks depending on `condition`. oh does not evaluate the `condition` sentence: without `when:`, a `conditional` checkpoint always asks the user when it is reported; with `when:`, oh evaluates it (see below).
- **Strictness order**: `skip` < `auto` < `conditional` < `pause`. On a `mandatory` checkpoint, a patch cannot choose a less strict behavior, nor remove it (`mandatory_checkpoint_removed`); `skip` on a mandatory checkpoint counts as a pause.
- **`remote`** (remote session): `auto` approved automatically; `defer` the session stops cleanly and waits for the user on the machine; `forbid` the workflow cannot run remotely.
- **`unlocks`**: until the checkpoint is passed, oh refuses these operations to **every agent** of the session, whatever the model: `commit` (`git commit`, including `env git commit`, `git -c … commit`, `/usr/bin/git commit`, `sh -c '…'`, an alias defined on the command line, and the commands that commit: `revert`, `cherry-pick`, `merge`, `pull`, `am`; the patterns are anchored on the subcommand: `git log --grep commit`, `git diff src/pushX.ts`, `bd list --status closed` stay allowed), `push` (`git push`; warning `unlock_push_without_publish` outside `risk: publish`: no shipped agent pushes, pushing is left to the user), `close` (closing a Beads ticket, also refused by the Beads gateway), `create` (creating Beads tickets: `bd create`, `bd q`, `bd create-form`, `bd duplicate`, `bd supersede`; creation is also refused by the Beads gateway, and the window does not close when a ticket is closed, but at the next mandatory checkpoint or when the checkpoint is asked again), `apply` (changing infrastructure: `terraform apply|destroy`, `kubectl apply|delete|…`, `helm install|upgrade|…`, `aws|gcloud|az` create/delete/update commands, `pulumi up|destroy`), `tag` (creating a tag: `git tag` in every form, listing included — `git describe --tags` and `git for-each-ref refs/tags` read the tags —, `git push --tags|--follow-tags`, `git push origin tag v1`, `git push … refs/tags/…`, `git update-ref refs/tags/…`, writing `.git/refs/tags` or `packed-refs`, `gh|glab release create`), `mr` (opening a merge request: `glab mr create|new`, `gh pr create|new`, their global options and `sh -c`, `glab api …merge_requests`, `gh api …/pulls`, `hub pull-request`, push option `merge_request.create`, MCP tool `create_mr` of `gitlab`). `apply`, `tag` and `mr` are refused in any workflow that declares no checkpoint unlocking them. When the same checkpoint unlocks `commit` and `tag`, the tag stays refused while the commit window is open: it always marks the release commit. The `tag` and `mr` windows close, like `create`, at the next mandatory checkpoint or when the checkpoint is asked again; the `tag` window also after the first tag (oh notes the local tags when the window opens and closes it as soon as they change, as `commit` once the work is committed). Once the checkpoint is passed, closing a ticket is accepted only if a non-empty commit (tracked files outside `.beads/`) was made since and nothing is left to commit; in a workflow where no checkpoint unlocks `close`, closing is not locked. The window closes again when the checkpoint is asked again (next ticket, another fix), for `commit` once the work is committed (HEAD moved and nothing is left to commit) and, for a checkpoint that unlocks `close`, when a ticket is closed (bounded windows, CP-R8). An operation is unlocked by one checkpoint only (`unlock_duplicate`), except the commit with `unlock_again` (below); a checkpoint that unlocks cannot be removed (`unlocking_checkpoint_removed`). A checkpoint with `unlocks` is treated as mandatory: it is never skipped (`skip` means `pause`, warning `unlock_skip`) and a patch cannot relax it (`loosening`). An operation unknown to the running oh (written by a newer version) is a warning (`unlock_unknown`) and locks nothing: update oh on every machine of the team.
- **`unlock_again`** (p5-fixes, boolean): this checkpoint unlocks again the commit that an earlier checkpoint already unlocks (`unlocks: [commit]` on both); `incident` commits the mitigation at `cp-mitigated`, then the postmortem at `cp-postmortem`. The window of the earlier checkpoint then holds one commit: oh does not open it when there is nothing to commit (the answer says so and names the later checkpoint), closes it after the first commit (even with changes left) and, at the latest, when the later checkpoint is asked; the wiki proposals wait for the last commit checkpoint. The answer of `workflow_checkpoint` and the workflow map say "in a single commit", never the closing of a ticket. Only the commit repeats (`unlock_duplicate` for the other operations); the repeated commit adds no irreversible pause to the budget (CP-R2). Diagnostics: `unlock_again_first` (no checkpoint before it unlocks the commit), `unlock_again_without_commit`. Patch: hub layer only (`loosening` elsewhere, on an inherited or an added checkpoint).
- A mode that is neither allowed nor one of the three shipped modes: `checkpoint_mode_unknown`; `conditional` without `condition`: `checkpoint_condition_missing`.
- Checkpoint execution (3 levels: prompt, MCP tool `workflow_checkpoint`, oh plugin): [ADR-042](../architecture/adr/042-checkpoints-headless-decisions.en.md). Modes, planned pauses, verification, evidence and windows: [Checkpoints](checkpoints.en.md).

### `verify` — verification run by oh

```yaml
    verify:
      from: stack                 # commands of the stack detected by oh
      kinds: [build, test]        # default: build, test, typecheck (lint on request)
      cmd: ["make e2e"]           # commands written here, run without a shell
      expect: pass                # pass (default) | fail (test written first)
      in: services/api            # subfolder of the session location
      timeout: 15m                # Go duration, default 10m, at most 30m
      must_touch: ["*_test.go"]   # the changes of the session touch one of these paths
      must_touch_tests: true      # … and a test file of the stack (R7)
      outside_tests: warn         # files outside the tests: warn (noted) | refuse
      since: cp-seams             # changes counted from the last pass of this checkpoint
      spec:                       # specs written by the session (R4)
        paths: ["docs/wiki/specs/**"]
        breakdown: true           # and their breakdown (convergence pass)
```

Before the checkpoint passes, and before its unlocks open, oh runs the commands (cleaned environment, bounded timeout and output); a failure refuses it (`ErrVerifyFailed`, with an output excerpt). At a paused checkpoint, the user sees the result on the card and may **override** it (traced choice). `cmd` only comes from the workflow document: never from the agent nor from an input. In a `risk: read` workflow, a verified checkpoint always asks the user. Details: [Checkpoints](checkpoints.en.md#verification-run-by-oh-verify).

Diagnostics: `verify_empty` (neither `from`, nor `cmd`, nor `spec`), `enum_invalid` (`from`, `kinds`, `expect`), `field_ignored` (`kinds` without `from`), `verify_cmd_template`, `verify_in_invalid`, `verify_timeout_invalid`, `field_required` (empty command or path). Fields of the test written first (R7): `must_touch_tests` requires a test file of the stack to be changed (patterns of `bricks.TestFilePatterns`: `*_test.go`, `*.test.*`, `*.spec.*`, `test_*.py`, `**/tests/**`, `**/src/test/**`…); `outside_tests` says what the other changed files do (`warn`: noted on the result; `refuse`: refused; absent: nothing); `since` counts the changes from the commit recorded at the last pass of a checkpoint declared before (else from the start of the session). With `expect: fail`, oh first runs the commands that build the code (`build`, `typecheck`, `lint`), which must pass, then the tests, which must fail when they run, not when they compile. Diagnostics: `verify_outside_tests_ignored` (warning: `outside_tests` without `must_touch_tests`), `verify_since_unknown`, `verify_since_method` (the `since` checkpoint only exists in a method this checkpoint does not have). Details: [Checkpoints](checkpoints.en.md#tdd-method-cp-seams-and-cp-red-r7).

Written specs (R4): with `spec`, oh reads the `.md` files the session changed under `paths` (a link is not followed) and checks first the wiki format, as `wiki_check` does: a frontmatter present and readable, with only the keys `updated` (a `YYYY-MM-DD` date), `confidence` (`confirmed`, `mixed` or `uncertain`, consistent with the tags of the page: the lowest wins) and `agents` (not empty), and well-formed confidence tag lines; then (`internal/specdoc`) a “Glossaire” section that defines at least one term, “Critères d'acceptation” that each carry a `CA-n` id and, on their line, their test point (`· Test : <test or command>`, a real value: neither empty nor “à définir”) and, with `breakdown: true`, a “Découpage” section whose slices each cite the criteria they cover (`Couvre : CA-1, CA-2`): every criterion covered, no slice without a criterion nor an unknown criterion. Lists and tables are read, headings in French or English (`Glossary`, `Acceptance criteria`, `Breakdown`). No spec changed, or a problem: the verification fails with the list of problems. `spec` alone is enough (no `from` nor `cmd`); diagnostics `field_required` (empty `paths`), `verify_spec_path_invalid`.

In a patch, the inherited verification may only be hardened (same `from`, `expect`, `in` and `since`, kinds, commands and paths kept, `must_touch_tests` kept, `outside_tests` at least as strict — absent < `warn` < `refuse` —, a timeout at least as long, the `spec` paths and `breakdown` kept): otherwise `loosening`; removing the checkpoint: `verified_checkpoint_removed`.

### `when` — conditions evaluated by oh

```yaml
    mode: { manuel: pause, semi-auto: conditional, auto: auto }
    when: { entry: [debugger], labels: [ux], paths: ["docs/specs/**"], outputs: [mr] }
```

One condition is enough. An `auto` checkpoint pauses when a condition holds; a `conditional` one pauses only then; a `pause`, mandatory or unlocking checkpoint always pauses: `when:` can only add a pause. With `when:`, `condition` is no longer required. Diagnostics: `when_empty`, `when_mandatory` (warning: no effect), `when_unknown_agent`, `when_unknown_output` (warnings), `field_required`.

### `by` — who passes the checkpoint

```yaml
  cp-2:
    by: [orchestrator-dev]
```

The agents that pass the checkpoint (CP-R11). Each agent's session contract names only its own checkpoints, and the generated checkpoint table shows an agent only its own. When another agent passes the checkpoint, oh lets it through, logs it and adds a warning to the `workflow_checkpoint` answer. Every checkpoint of the shipped workflows sets it (test `TestCheckpointByMembers`). Diagnostics: `checkpoint_by_unknown` (error: not a member), `checkpoint_by_duplicate` (warning).

### `wiki` — synchronization of the wiki

```yaml
  cp-sync:
    unlocks: [commit]
    wiki: sync
```

`sync` (`doc-sync` workflow): the card of the checkpoint shows **the whole queue** of the wiki proposals of the project, not only those of the session; the user decides on each one. When it is passed, oh applies the accepted proposals and writes in the frontmatter of `docs/wiki/index.md` the commit the session started from (`synced:`), the starting point of the next synchronization. The checkpoint must unlock the commit: otherwise `checkpoint_wiki_commit` (error); another value: `enum_invalid`. See [Living documentation wiki](../architecture/living-wiki.en.md#synchronization-the-doc-sync-workflow-ld-d).

### `method` — method of the session

```yaml
inputs:
  method: { type: enum, values: [standard, tdd], default: standard }
checkpoints:
  cp-seams:
    method: tdd
    mode: { manuel: pause, semi-auto: pause, auto: auto }
```

A checkpoint with `method` only exists in the sessions launched with that method (`method` input, `oh run ticket -i method=tdd`, or the launch form): oh evaluates the method at launch and the session bundle holds only the checkpoints of the chosen method (CP-R7). A `standard` session therefore gets exactly the bundle of a workflow without those checkpoints. Only value today: `tdd` (`enum_invalid` otherwise). Such a checkpoint only adds a step: it can neither be mandatory nor unlock an operation (`checkpoint_method_mandatory`), no agent may wait for it (`after:`, `checkpoint_method_after`), and the `method` input must offer the method (`checkpoint_method_no_input`). In a patch, setting or changing `method` on an inherited checkpoint is a loosening (`loosening`): it would vanish from the other sessions.

### `fanout` — child sessions (R6, experimental)

```yaml
inputs:
  fanout: { type: bool, default: false }
fanout:
  input: fanout            # bool input that makes the session a fan-out
  tickets: tickets         # beads-ids input: one child session per ticket
  child: ticket            # workflow of each child session
  parallel: 3              # child sessions at the same time (1 to 3)
  merge: cp-merge          # merge checkpoint of the parent session
  delegates: [cp-2]        # checkpoints of the children replaced by cp-merge (hub layer only)
  merger: merge-conflicts  # workflow oh launches on a conflict it cannot resolve
  inputs: [method, instructions]  # inputs of the parent given to each child (same name)
  fix: { workflow: quick, input: request }  # fix round when cp-merge is refused
  # source: tasks          # sweep-fanout: the tasks recorded by oh at the tasks: checkpoint (no tickets:)
checkpoints:
  cp-2: { fanout: never, … }
  cp-merge:
    fanout: only
    mandatory: true
    unlocks: [merge]
    verify: { from: stack, expect: pass, on: integration }
```

A session launched with the `fanout` input set to `true` (`oh run implement-spec -i fanout=true`) is a **fan-out**: oh launches one child session per ticket, each on its own branch and worktree, merges it itself into an integration branch, hands a conflict it cannot resolve to the `merger`, and, once the merge checkpoint is validated and verified **on the integration branch**, merges that branch into the branch of the session and closes the merged tickets. The fan-out is experimental and off by default (`[fanout] experimental = true`, see the [guide](../guides/fan-out.en.md)). oh evaluates the input at launch: a fan-out session keeps only the checkpoints without `fanout` or with `fanout: only` (the agents locked by a `fanout: never` checkpoint, the developer after `cp-1`, are left out of the bundle); the other sessions keep exactly the bundle of before (`fanout: only` dropped, `fanout` block ignored).

**Control substitution (CP-R3).** `delegates` lists checkpoints of the child workflow that no longer pause in the children: the merge checkpoint replaces them. The schema requires a **mandatory** merge checkpoint, `fanout: only`, that unlocks `merge` and whose verification run by oh runs on the integration branch (`verify.on: integration`, `expect: pass`); the delegation is declared by the **hub layer** only (a team or project layer, or a draft, that writes it: `fanout_delegates_layer`). A patch may only harden the fan-out: only `parallel` may go down; changing `input`, `tickets`, `child`, `merge`, `merger` or `delegates`, removing the block, or setting `fanout:` on an inherited checkpoint is a `loosening`. The daemon checks everything again at run time (see [Checkpoints](checkpoints.en.md#fan-out-control-substitution-cp-r3)).

**Inputs of the children.** `inputs` lists the inputs of the parent session given to each child under the same name, when the session got them (a default is not given: the child keeps its own); nothing else of the parent reaches the children. Each one is a value input of the parent (`string`, `text`, `bool`, `int`, `enum`, `path`, `branch`; neither the fan-out input nor the tickets input: `fanout_inputs_parent`), declared by the child workflow with the same type (`fanout_inputs_child`, `fanout_inputs_type`) and, for an `enum`, each of its values (`fanout_inputs_value`). A team or project layer may give fewer, never more (`loosening`).

**Fix round.** `fix` names the workflow oh launches in the integration worktree when the merge checkpoint is refused with an instruction, and its text input that gets it, with what failed in the verification (`fanout_fix_input`). That workflow must have a mandatory checkpoint that unlocks the commit (`fanout_fix_commit`): its fix is validated by the user, never delegated. oh takes its commits, then the merge checkpoint is passed again and verified again on the integration branch: the control substitution does not change. At most `limits.max_fix_rounds` rounds (1 without a limit). A patch of another layer than the hub does not change it (`loosening`).

**Children from the breakdown** (`source: tasks`, sweep-fanout). Instead of a `beads-ids` input (`tickets:` is not written: `fanout_tasks_tickets`), the children are the tasks **recorded by oh** at the checkpoint that has [`tasks:`](#tasks--breakdown-read-by-oh): mandatory (`fanout_tasks_mandatory`), present in every session (`fanout_tasks_scope`), declared before the merge checkpoint (`fanout_tasks_after_merge`; without it: `fanout_tasks_checkpoint`). oh writes each task in the inputs of the child workflow: `task` (title and description) and `files` (one scope entry per line), text inputs it must declare (`fanout_task_input`), and `domain` when it declares it (an `enum` holding every value of `tasks.domains`: `fanout_task_domain`); `inputs` does not name them (`fanout_inputs_reserved`). No ticket: oh does not read Beads, closes nothing at the merge, and the verification of the delegated checkpoint of a child fails when a change is outside the scope of its task. Changing `source` is a `loosening` outside the hub layer.

**Checkpoint the children pass, changed by a layer**: a team or project patch on a `fanout: never` checkpoint (the `cp-commit` of `sweep`, the `cp-2` of `implement-spec`) stays valid outside a fan-out, but a fan-out session has no such checkpoint: `fanout_never_patched` warning, and `oh doctor` lists these layers. To harden the fan-out too, change the merge checkpoint.

`merge` operation: `git merge`, `git pull` and the merges of the forges (`glab mr merge`, `gh pr merge`, their APIs, `gitlab` MCP tool `merge_mr`). Only the merge checkpoint of a fan-out unlocks it (`unlock_merge_fanout_only`), and oh merges when it passes: no window opens for the agents.

Diagnostics: `fanout_input`, `fanout_tickets`, `fanout_parallel`, `fanout_unknown_workflow`, `fanout_child_self`, `fanout_child_nested` (one level of children), `fanout_delegate_unknown` (checkpoint missing from the child workflow), `fanout_merge_unknown`, `fanout_merge_scope`, `fanout_merge_mandatory`, `fanout_merge_unlocks`, `fanout_merge_verify`, `fanout_merge_verify_on`, `fanout_merge_verify_expect`, `fanout_delegates_layer`, `checkpoint_fanout_without_fanout`, `unlock_merge_fanout_only`, `verify_on_integration_merge_only`, `fanout_inputs_parent`, `fanout_inputs_child`, `fanout_inputs_type`, `fanout_inputs_value`, `fanout_fix_input`, `fanout_fix_commit`, `fanout_tasks_tickets`, `fanout_tasks_checkpoint`, `fanout_tasks_mandatory`, `fanout_tasks_scope`, `fanout_tasks_after_merge`, `fanout_task_input`, `fanout_task_domain`, `fanout_inputs_reserved`, `agent_fanout_without_fanout`, `agent_fanout_entry`, `fanout_never_patched` (warning).

### `tasks` — breakdown read by oh

```yaml
checkpoints:
  cp-plan:
    mandatory: true
    tasks: { max: 12, domains: [frontend, backend, refactor, tests] }
outputs:
  - { id: tasks, type: text, record: tasks }
```

The agent passes the checkpoint with its breakdown in the `tasks` argument of `workflow_checkpoint`: one entry `{ title, description, files, domain, parallel }` per task. oh does not take it on trust: before any card, it checks 1 to `max` tasks, a one-line title, a domain of `domains` (when the list is written), a `files` scope whose every entry (path, folder, `*`/`**` pattern) names files of the repository (`git ls-files`) or a file to create in an existing folder, never absolute, outside the location, an option or `.git`, and that two tasks that may run at the same time (both `parallel`, or all of them in a fan-out) share no file. A refused breakdown makes no card: the call is refused with the list of problems. Accepted, the card shows it as oh read it (`t1`, `t2`… numbered by oh) and the call that passes the checkpoint must bring the same one (otherwise refused). Once passed, oh records it in the checkpoint state of the session and in the `record: tasks` output; the answer of the checkpoint gives it back to the agent. One checkpoint per workflow (`tasks_duplicate`), bounded `max` (`tasks_max`).

### Pause budget

In semi-auto (or in the default mode when semi-auto is not allowed), a workflow pauses at most **twice for human decisions, plus once per checkpoint that unlocks an irreversible action**; a `conditional` checkpoint or one with `when:` counts as a pause. A method (`method`) adds at most 1 pause, for its own sessions. Over it: warning `pause_budget_exceeded` (blocking for the shipped workflows, `TestCheckpointPauseBudget` test). See [Checkpoints](checkpoints.en.md#pause-budget-cp-r2).

---

## Preconditions

```yaml
preconditions:
  project-context:
    label: { fr: Aucun contexte projet trouvé, en: No project context found }
    check: { path_exists: [docs/wiki/index.md, ONBOARDING.md] }   # one is enough
    on_fail: suggest
    suggest: { workflow: onboarding, resume: true }
```

Checked before launch, in file order, where the session will work: the project directory, the worktree given with `--location <path>`, or, for a worktree created at launch (`--location new`, automatic worktree), what it will hold: its branch, nothing uncommitted, the files of that branch when it exists, otherwise those of the project `HEAD`. Remote session: the project directory. `check` holds at least one test (`precondition_check_invalid`); with several, all must hold.

```yaml
preconditions:
  clean-tree:
    label: { fr: L'arbre de travail doit être propre avant une release, en: The work tree must be clean before a release }
    check: { git_clean: true }
    on_fail: block
  release-branch:
    check: { branch: [main, "release/*"] }
    on_fail: block
  tools:
    check: { tools: [goreleaser] }
```

| Field | Type | Values | Absent |
|---|---|---|---|
| `label` | translatable text | message shown on failure | the id |
| `check.path_exists` | list | relative paths, without `..`; one of them must exist | — (`precondition_path_invalid`) |
| `check.git_clean` | boolean | `true`: git repository without uncommitted change (staged, unstaged, or untracked and not ignored file; `.beads/` excluded) | — |
| `check.branch` | list | branch names or patterns (`release/*`, `path.Match`); the current branch must match one of them, a detached HEAD matches none | — (`precondition_branch_invalid`) |
| `check.tools` | list | command names (no path nor argument); all must be in the `PATH` of oh | — (`precondition_tool_invalid`) |
| `on_fail` | text | `suggest`, `block` | `suggest` |
| `suggest.workflow` | text | id of another workflow of the catalogue | no suggestion (`precondition_self`, `precondition_unknown_workflow`) |
| `suggest.resume` | boolean | `true`, `false` | `false` |
| `disabled` | boolean | `true` | removes an inherited precondition (patch only; never in a `risk: publish` workflow) |

- `suggest`: message + offer to launch `suggest.workflow`; the user may go on. With `resume: true`, oh offers to relaunch the original workflow (same inputs) once the other one has finished.
- `block`: the launch is refused.
- Remote or without interface: a `suggest` failure becomes a warning, a `block` failure stops the session.
- Message: the label, followed by what failed, read by oh — uncommitted changes (the first five, then their number), current and expected branches, detached HEAD, missing tools, directory outside git. The git reads go through the safe git of oh (no hook, no fsmonitor, no repository setting that runs a program); a read that fails fails the precondition.
- Patch: merged by id, field by field. In a `risk: publish` workflow, the inherited preconditions are security fields: a layer may add or harden them (narrower `branch`, added tools, `on_fail: block`), never remove nor loosen them — `disabled`, `git_clean` removed, a branch pattern the parent does not cover, a removed tool, a path added to `path_exists`, `block` changed to `suggest` are loosenings (`loosening`), and the parent's precondition is kept.

---

## Resources

| Field | Type | Absent | Patch |
|---|---|---|---|
| `models.default` | `provider/model[#variant]` | cascade without workflow level | replaces |
| `models.agents.<agent>` | `provider/model[#variant]` | — | merged by agent |
| `skills.extra` | list of skill refs | — | replaces |
| `skills.deny` | list (ref or bare name) | — | replaces |
| `mcp` | list of oh MCP server ids | MCP servers of the project | hardens outside the hub layer (subset of the parent list; parent without a list: any list hardens) |
| `plugins` | list | no plugin besides the oh plugin | hardens outside the hub layer (subset of the parent: a plugin runs code) |
| `beads.allow` | see [Security](#beads) | | |

- **`models`**: workflow level of the [model cascade](model-resolution.en.md) (workflow·agent > workflow > project…). Invalid format: `model_invalid`; agent outside the workflow: `model_agent_unknown`.
- **`skills`**: skills come from the member agents and their dependencies (`requires:`). `extra` adds some (`skill_unknown`), `deny` removes some (a `path/skill` ref or a bare name). A denied skill required by another one: `skill_denied_required`; missing dependency: `skill_closure`; duplicate id: `skill_duplicate`.
- **`mcp`**: ids of oh MCP servers (`gitlab`, `figma`, `jira`, `team`…). Absent: the session keeps the project's MCP servers; present (even empty): only those of the list. The `workflow` server (`workflow_status`, `workflow_checkpoint`, `workflow_outputs`, `workflow_report`, `workflow_report_get`, `workflow_reports`: see [workflow MCP server](cli-mcp.en.md#workflow-mcp-server)) is always added. With `risk: read`, the servers start **without their write tools**, whatever the project setting (`write_enabled`).
- **`plugins`**: an id (npm spec, e.g. `context-mode@latest`) or `{ id, options }`. Refused with `risk: read` for a plugin whose tools run commands (`context-mode`: `read_plugin_exec`). Installed by opencode when the server starts; a plugin that fails to load does not stop the session. The module must export the opencode V2 format (`{ id, setup }`).
- Empty or duplicate list entries: `field_required`, `duplicate_entry`.

---

## Outputs (`outputs`)

```yaml
outputs:
  - { id: branch, type: branch, label: { fr: Branche de travail, en: Working branch } }
  - { id: tickets, type: beads-ids }
```

| Field | Type | Values |
|---|---|---|
| `id` | text | unique in the list |
| `type` | text | `branch`, `merge_request`, `beads-ids`, `path`, `report` (id of an `rp-…` report), `text` (a value, e.g. a recorded input) |
| `label` | translatable text | displayed label |
| `record` | text | absent: declared by the agent; otherwise **recorded by oh** (R5): `created`, `closed`, `spec`, `report`, `input`, `written`, `tasks` |
| `paths` | list of paths | with `record: written` only: the files read (patterns as in `writes:`, relative to the repository) |
| `match` | regular expression | with `record: written` only: the value recorded is the first group of the expression in those files (`text` output) |

Without `record`, the agent declares the value with the MCP tool `workflow_outputs` (the workflow map lists it under “Outputs to declare”, `workflow_status` under `expected_outputs`). At the end of the session, the outputs offer "Chain with…" (key `e` of the Sessions view), the [`next`](#composition-next-and-aliases) steps and `oh session results`. Patch: list replaced as a whole.

### Outputs recorded by oh (`record`)

A `record:` output is filled by oh from what it sees the session do; no agent can declare it (`workflow_outputs` refuses it) and the workflow map does not mention it. A workflow without an output to declare (none, or only `record:` outputs) does not offer the `workflow_outputs` tool to its agents: oh refuses it to them, which removes it from their tools.

| `record` | Type | What oh records |
|---|---|---|
| `created` | `beads-ids` | the tickets the session created through the Beads gateway (`bd create`, `create --file`, `q`, `--json`, `--silent`), added as they are created |
| `closed` | `beads-ids` | the tickets the session closed through the Beads gateway |
| `spec` | `path` | the specs checked by oh at a passed `verify.spec` checkpoint (several: joined by commas); without a `verify.spec` checkpoint, warning `output_record_spec_unchecked` |
| `report` | `report` | the last report deposited with `workflow_report` |
| `input` | `text` | the value of the input with the same id, at launch (else its default), to pass it on through `next:`: `spec` records `method` and passes it to `implement-spec`; without an input of that name, `output_record_input_unknown` |
| `written` | `path`, or `text` with `match` | each time a checkpoint passes, the files the session changed under `paths` (committed since it started, modified or untracked; a deleted file does not count; several: joined by commas); with `match`, the first group of the expression in the first of those files where it is found (nothing found: the previous value stays). `incident` records its postmortem, `spike` its decision and its verdict |
| `tasks` | `text` | the breakdown checked and recorded by oh at the [`tasks:`](#tasks--breakdown-read-by-oh) checkpoint, one line per task (`t1 [domain] title — scope`); without that checkpoint, `output_record_tasks_unchecked` |

- The type must be the one of the table (`output_record_type`).
- `record: written`: `paths` required (`output_written_paths`), clean relative paths (`output_written_path`), `match` with at least one group (`output_match_invalid`); `paths` or `match` without `record: written`: `output_written_only`.
- A `report` output goes on to the next workflow **as its content**: an input `from: session.output(<workflow>.<output>)` receives the text of the report (cleaned), not its id; `next:` does not prefill it, the chained launch (`--parent`) reads it in the session.
- At the end of the workflow, oh no longer fills a recorded output with the closed tickets (as it does for a declared `beads-ids` output).
- A remote session does not record creations (tickets only have a provisional id there).

---

## Composition (`next` and `aliases`)

```yaml
aliases: [cadrage, feature]        # oh run cadrage launches this workflow
next:
  - workflow: implement-spec
    when: { outputs: [tickets] }   # proposed only when the session has this output
    # when: { equals: { verdict: go } }  # … and with this value
    inputs:
      tickets: session.output(tickets)
      spec: session.output(spec)
      method: session.output(method) # recorded at launch (record: input)
```

### `next` — what oh proposes next (E4)

| Field | Type | Role |
|---|---|---|
| `workflow` | id | proposed workflow (or an alias) |
| `label` | translatable text | label of the proposal (default: the workflow's) |
| `when.outputs` | list of ids | the proposal is made only when the session has all these outputs |
| `when.equals` | output → value | … and only when each of these outputs has this value (case ignored): `audit` proposes `security-fix` only after a `security` audit, `spike` proposes `spec` only after a `go` |
| `inputs.<input>` | `session.output(<output>)` | input of the proposed workflow, prefilled with an output of the session that ends |

- **A proposal, never a launch**: when the client of a session launched by `oh run` closes, oh shows the outputs of the session and the command of each step (`oh run implement-spec --parent ses_… --tickets ev-1,ev-2 -i spec=…`); `oh session next <id>` shows them again; the Sessions view of the TUI puts them first in “Chain with…” and in the end-of-session notification.
- The proposed launch carries `--parent`: the `session.output(…)` sources of its inputs read that session first.
- Checks: `next_unknown_workflow`, `next_self`, `duplicate_entry`, `next_unknown_output` (output of `when` or `inputs` not declared), `next_input_invalid` (another form than `session.output(<output>)`), `next_unknown_input` (input missing from the proposed workflow), `next_input_type` (incompatible type), `next_input_report_from` (warning: a report given to an input without `from: session.output(<this workflow>.<output>)` only arrives as its id). Patch: list replaced as a whole.

### `aliases` — other names of the workflow

- `aliases: [cadrage]` or `aliases: [{ id: feature, with: tickets }]`: `oh run <alias>` (and the launch form, `oh eval`) launches this workflow and says so (“cadrage is an alias: launching spec”). The given inputs go to the target workflow.
- **`with`**: the alias goes to this workflow only when this input is given at launch (`--tickets` fills the ticket input); otherwise it goes to the workflow that declares it without `with`. One alias may thus go to two workflows depending on the inputs.
- A document with the id of the alias, in any layer, wins: the alias no longer applies (`alias_shadowed`, warning).
- Checks: `id_invalid`, `alias_self`, `duplicate_entry`, `alias_with_unknown_input`; on the catalogue: `alias_ambiguous` (alias declared without `with` by two workflows), `alias_without_default` (alias with only `with` routes). Patch: list replaced as a whole.

---

## Layers and resolution

| Layer | Location |
|---|---|
| `hub` | `workflows/` of the repository, embedded, extracted to `~/.oh/hub/workflows/` (read-only) |
| `team` | `team-state/workflows/published/<id>.yaml` (or solo space) |
| `project` | `team-state/projects/<project>/workflows/published/<id>.yaml` |
| draft | `workflows/drafts/<member>/<id>.yaml` of the scope, launched with `oh run <id> --draft` |
| session | launch options (mode, environment, inputs, entry agent), never stored |

- **Order**: hub < team < project < draft < session. A draft is not a layer: it replaces, for its author and locally only, the published document of its layer; it is refused when it loosens the published version.
- **Resolution**: oh follows `extends` up to the root definition, applies each patch from the root down to the requested document, then the session options. Each value keeps its origin (`oh workflow show <id> --origin`, TUI editor).
- **Integrity**: only published files whose hash matches `workflows.lock` are loaded; the others are skipped with a warning. See [Team workflows](../guides/team-workflows.en.md#workflowslock-and-integrity).
- Governance, drafts and publication: [ADR-040](../architecture/adr/040-workflows-team-state-governance.en.md).

---

## Main diagnostics

A diagnostic has a severity (error or warning), a code, the field path, the source document and its position. Translated messages: keys `workflow.diag.<code>`.

| Code | Meaning |
|---|---|
| `syntax`, `unknown_field`, `duplicate_key`, `invalid_type` | invalid YAML, unknown field, duplicate key, wrongly typed value |
| `api_version`, `kind`, `field_required` | wrong header or missing required field |
| `multiple_documents`, `empty_document`, `id_filename_mismatch` | one workflow per file, named `<id>.yaml` |
| `id_invalid`, `agent_id_invalid`, `checkpoint_id_invalid`, `precondition_id_invalid`, `input_id_invalid`, `input_id_reserved` | malformed or reserved id |
| `enum_invalid`, `enum_values_missing`, `negative_value`, `duplicate_entry`, `field_ignored` | value out of list, `enum` without `values`, negative number, duplicate, field without effect for this type (warning) |
| `unknown_workflow`, `extends_not_found`, `extends_alias`, `invalid_extends`, `extends_cycle`, `extends_more_specific`, `duplicate_workflow` | `extends` chain cannot be built |
| `shadow_without_extends`, `must_extend_nearest` | same id as a lower layer without extending it, or without extending the nearest one |
| `loosening` | a security field is loosened: the parent value is kept |
| `enforced_field`, `enforce_unknown_field` | field locked by a parent; unknown `enforce` value |
| `entry_not_primary`, `entry_role`, `entry_disabled`, `agent_unknown`, `agent_unreachable` | invalid entry agent or member |
| `after_unknown`, `after_cycle`, `calls_unknown`, `graph_cycle` | `after` gates and delegations |
| `scratch_reader_unknown`, `scratch_reader_owner` | scratch reader not in the workflow, or the owner itself |
| `writes_after_unknown`, `writes_after_without_writes`, `checkpoint_after_unknown`, `checkpoint_after_order`, `order_method_after` | order of the writes and of the checkpoints (r5-fixes) |
| `writes_path_invalid`, `writes_read_only` | write path outside `docs/` or with a wildcard other than `/**`; `writes` in a `risk: read` workflow |
| `verify_spec_path_invalid` | `verify.spec` path absolute or outside the location |
| `checkpoint_agent_clash`, `checkpoint_mode_unknown`, `checkpoint_mode_missing`, `checkpoint_condition_missing`, `mandatory_checkpoint_removed`, `unlocking_checkpoint_removed`, `unlock_duplicate`, `unlock_again_first`, `unlock_again_without_commit`, `unlock_unknown`, `unlock_skip`, `unlock_push_without_publish`, `unlock_requires_publish`, `publish_checkpoint_remote`, `publish_checkpoint_pause`, `patch_unknown_checkpoint`, `checkpoint_by_unknown`, `checkpoint_by_duplicate`, `checkpoint_wiki_commit`, `headless_checkpoint_pause` | checkpoints |
| `verify_empty`, `verify_cmd_template`, `verify_in_invalid`, `verify_timeout_invalid`, `verified_checkpoint_removed`, `when_empty`, `when_mandatory`, `when_unknown_agent`, `when_unknown_output`, `pause_budget_exceeded` | verification run by oh, conditions, pause budget (R3) |
| `fanout_input`, `fanout_tickets`, `fanout_parallel`, `fanout_unknown_workflow`, `fanout_child_self`, `fanout_child_nested`, `fanout_delegate_unknown`, `fanout_merge_unknown`, `fanout_merge_scope`, `fanout_merge_mandatory`, `fanout_merge_unlocks`, `fanout_merge_verify`, `fanout_merge_verify_on`, `fanout_merge_verify_expect`, `fanout_delegates_layer`, `checkpoint_fanout_without_fanout`, `unlock_merge_fanout_only`, `verify_on_integration_merge_only`, `fanout_inputs_parent`, `fanout_inputs_child`, `fanout_inputs_type`, `fanout_inputs_value`, `fanout_fix_input`, `fanout_fix_commit`, `fanout_tasks_tickets`, `fanout_tasks_checkpoint`, `fanout_tasks_mandatory`, `fanout_tasks_scope`, `fanout_tasks_after_merge`, `fanout_task_input`, `fanout_task_domain`, `fanout_inputs_reserved`, `agent_fanout_without_fanout`, `agent_fanout_entry`, `fanout_never_patched` (warning) | fan-out and control substitution (R6, sweep-fanout) |
| `tasks_duplicate`, `tasks_max`, `output_record_tasks_unchecked` | breakdown read by oh (sweep-fanout) |
| `method_input_type`, `checkpoint_method_no_input`, `checkpoint_method_mandatory`, `checkpoint_method_after`, `verify_since_unknown`, `verify_since_method`, `verify_outside_tests_ignored` | method of the session and test written first (R7) |
| `precondition_check_invalid`, `precondition_path_invalid`, `precondition_branch_invalid`, `precondition_tool_invalid`, `precondition_self`, `precondition_unknown_workflow`, `patch_unknown_precondition` | preconditions |
| `prompt_both`, `prompt_empty`, `prompt_template_path`, `prompt_template_missing`, `prompt_parse`, `prompt_unknown_variable` | prompt |
| `next_unknown_workflow`, `next_self`, `next_unknown_output`, `next_input_invalid`, `next_unknown_input`, `next_input_type`, `alias_self`, `alias_shadowed`, `alias_with_unknown_input`, `alias_ambiguous`, `alias_without_default`, `output_record_type`, `output_record_spec_unchecked`, `output_written_paths`, `output_written_path`, `output_match_invalid`, `output_written_only`, `next_input_report_from`, `input_from_type`, `input_from_unknown_workflow`, `input_from_unknown_output`, `input_from_session_workflow` | composition: `next`, `aliases`, outputs recorded by oh, `session.*` sources (R5) |
| `read_agent_writes`, `read_beads_unrestricted`, `read_beads_write`, `read_plugin_exec` | `risk: read` rules |
| `plan_agent_writes`, `plan_beads_unrestricted`, `plan_beads_delete`, `plan_without_beads_write` | `risk: plan` rules |
| `remote_forbidden_checkpoint`, `publish_remote_runtime`, `runtime_default_not_allowed`, `mode_default_not_allowed`, `isolation_unsupported` | environments, modes, isolation |
| `parallel_writers_without_commit_gate` | warning: `risk: write`, at least two agents that edit files and no checkpoint that unlocks `commit` (except a selectable entry, `libre`) |
| `model_invalid`, `model_agent_unknown`, `skill_unknown`, `skill_unknown_deny`, `skill_closure`, `skill_duplicate`, `skill_denied_required` | resources |
| `session_mode_not_allowed`, `session_runtime_not_allowed`, `session_input_unknown`, `session_input_invalid`, `session_input_missing`, `verify_cmd_missing` (an `enum` input set to `custom` requires the `<input>_cmd` input when the workflow declares it), `session_entry_not_selectable`, `session_entry_unknown` | launch options |

---

## Full example

```yaml
apiVersion: oh/v1
kind: Workflow
id: ticket-hotfix                  # = file name ticket-hotfix.yaml
category: develop
label: { fr: Correctif urgent, en: Hotfix }
description:
  fr: Corriger un ticket urgent, review obligatoire
  en: Fix an urgent ticket, mandatory review
extends: hub:ticket                # inherits everything else
enforce: [checkpoints]             # projects do not touch the checkpoints

risk: write                        # ≤ write (parent): allowed
isolation: strict                  # hardens standard → strict
code_mode: false

inputs:                            # merged by id
  ticket:
    picker: { filter: hotfix }     # only the filter changes; multi stays inherited
  severity: { type: enum, values: [p1, p2], default: p1, label: Severity }

prompt:
  text: |                          # replaces the inherited template
    Mode de workflow : {{ .oh.mode }}
    Langue de réponse : {{ .oh.lang }}
    Fix ticket {{ .ticket }} (severity {{ .severity }}) in {{ .oh.project }}.
    {{ if .instructions }}{{ data "instructions" .instructions }}{{ end }}

agents:
  documentarian: { role: disabled }       # removed from the bundle
  reviewer: { mode: subagent }            # only `mode` is rewritten

checkpoints:
  cp-1:
    mode: { semi-auto: pause }       # stricter: allowed
  cp-3: { disabled: true }           # one ticket at a time

modes: { default: manuel, allowed: [manuel, semi-auto] }
circuit_breaker: { max_consecutive_subagents: 8 }

models:
  agents: { reviewer: amazon-bedrock/eu.anthropic.claude-opus-4-6-v1 }
mcp: [gitlab]                      # GitLab only (+ the workflow server)

beads: { allow: [show, list, update, close] }        # subset of the parent
runtime: { default: local, allowed: [local, container] }  # without remote
limits:
  budget_usd: 3
  models: ["eu.anthropic.claude-*"]

preconditions:
  changelog:
    label: No CHANGELOG.md
    check: { path_exists: [CHANGELOG.md] }
    on_fail: block

outputs:
  - { id: branch, type: branch }
  - { id: mr, type: merge_request }
```
