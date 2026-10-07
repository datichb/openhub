> [Lire en français](workflow-schema.fr.md)

# Reference — `oh/v1` workflow schema

A workflow is a YAML file `apiVersion: oh/v1`, `kind: Workflow`, named `<id>.yaml`. This page describes every field: type, values, effective value when the field is absent, behavior in a patch (`extends`) and locking (`enforce`).

The schema is **frozen** (`cli/internal/workflow/schema.go`, type `Spec`): it only changes through optional additions. Additions made after the freeze: `risk: plan`, `preconditions`, `enforce`, `limits.models`, `entry.selectable`. Decision: [ADR-039](../architecture/adr/039-declarative-workflows-oh-v1.en.md).

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
- **Security**: `risk`, `isolation`, `runtime.allowed`, `beads.allow`, mandatory checkpoints, `remote` and `limits` may only **be hardened**. A loosening is an error (`loosening`) and the parent value is kept.
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

### `enforce`

```yaml
enforce: [checkpoints, modes]   # or ["*"] for the whole document
```

- Values: `category`, `label`, `description`, `risk`, `isolation`, `code_mode`, `entry`, `inputs`, `prompt`, `agents`, `checkpoints`, `modes`, `circuit_breaker`, `models`, `skills`, `plugins`, `mcp`, `beads`, `runtime`, `outputs`, `limits`, `preconditions`, or `"*"`. Any other value: `enforce_unknown_field`.
- The lock covers the whole top-level field (`checkpoints` locks every checkpoint).
- Locks add up along the `extends` chain; a more specific layer cannot remove any.
- Launch options (mode, environment, inputs) stay allowed, within the limits of the workflow.

---

## Security

| Field | Type | Values | Absent | Patch |
|---|---|---|---|---|
| `risk` | text | `read` < `plan` < `write` < `publish` | **required** (`field_required`) | hardens (equal or lower rank) |
| `isolation` | text | `strict`, `standard` | `standard` | hardens (`strict` does not go back to `standard`) |
| `code_mode` | boolean | `true`, `false` | `false` | replaces |
| `beads.allow` | list | `bd` subcommands | see below | hardens (subset of the parent) |
| `runtime.default` | text | `local`, `container`, `remote` | `local` | replaces |
| `runtime.allowed` | list | `local`, `container`, `remote` | `[runtime.default]` | hardens (subset of the parent) |
| `limits.budget_usd` | number ≥ 0 | USD per session | no budget at workflow level | hardens (lower or equal value) |
| `limits.models` | list | patterns (`*` wildcard) | no list at workflow level | hardens (patterns covered by the parent) |

### `risk`

| Value | Files | Beads | Checks |
|---|---|---|---|
| `read` | no edits, no shell | read only | no agent with edit or shell (`read_agent_writes`); `beads` required (`read_beads_unrestricted`), without write commands (`read_beads_write`) |
| `plan` | no edits, no shell | writes limited to `beads.allow` | no agent with edit or shell (`plan_agent_writes`); `beads` required (`plan_beads_unrestricted`), without `delete` (`plan_beads_delete`); warning when there is no write at all (`plan_without_beads_write`: `read` is enough) |
| `write` | edited | no schema restriction | — |
| `publish` | edited, branches pushed, MRs opened | same | — |

Beads commands counted as writes: `create`, `update`, `close`, `reopen`, `delete`, `edit`, `comment`, `comments`, `label`, `dep`, `duplicate`, `supersede`.

A writing session (`risk` other than `read`) gets a worktree when another writing session is active in the same place (see [workflows CLI](cli-workflows.en.md#oh-run)).

### `isolation`

- `strict`: the closed world must be complete. No permission may be granted "always" from oh. When the adapter does not report full isolation, validation warns (`isolation_unsupported`) and the launch is refused.
- `standard` (or absent): partial isolation is accepted.
- Independent from `[execution] strict_isolation` in `hub.toml` (which hides the user's own tool configuration). See [ADR-041](../architecture/adr/041-closed-world-isolation.en.md).

### `code_mode`

`false` or absent: the opencode `execute` tool is denied to every agent. `true`: it stays available. The field is part of the bundle hash. It is not a security field for patches: a layer may enable it (the publication impact summary reports it).

### `beads`

```yaml
beads: { allow: [show, list, update, close] }
```

- Absent: no declared restriction. In a container, the Beads gateway then applies a read-only list (`show`, `list`, `ready`, `search`, `children`, `comments`, `count`, `status`, `graph`, `history`).
- `allow: []`: no command allowed.
- The list is applied by the daemon's Beads gateway (container) and when the journal is replayed (remote). See [ADR-046](../architecture/adr/046-beads-gateways.en.md).
- Patch: on a parent without `beads`, any list hardens. On a parent with a list, the new list must be a subset of it; `beads: null` is a loosening.

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
```

- Adds up with the I6 restrictions of the hub, the team and the project (`oh budget show`): the most specific value wins (workflow > project > hub > team recommendation); a value enforced by the team stays a ceiling. See [ADR-044](../architecture/adr/044-credential-proxy-session-limits.en.md).
- `budget_usd`: soft cap, checked between two steps; beyond it, a `$` decision asks to raise it or stop. Negative value: `negative_value`.
- `models`: patterns on the id sent to the provider; the credential proxy refuses the others. An empty pattern: `field_required`.

---

## Entry and modes

| Field | Type | Values | Absent | Patch |
|---|---|---|---|---|
| `entry.agent` | text | agent id of the catalogue, primary | `conductor` | replaces the `entry` block |
| `entry.selectable` | boolean | `true`, `false` | `false` | to be written with `entry.agent` |
| `modes.default` | text | a mode of `modes.allowed` | first allowed mode | replaces |
| `modes.allowed` | list | `manuel`, `semi-auto`, `auto`, or custom modes | `[manuel, semi-auto, auto]` | replaces |
| `circuit_breaker.max_consecutive_subagents` | integer ≥ 0 | N delegations in a row without interaction | `0` (breaker disabled) | replaces |

### `entry`

- The entry agent must be **primary** (`entry_not_primary`) and, when listed in `agents`, have the `workflow` role (`entry_role`). It cannot be removed by a patch (`entry_disabled`).
- `conductor`: generic agent without write or shell, which follows the generated workflow map and delegates.
- `selectable: true`: the entry agent is chosen at launch (`oh run libre --agent debugger`). Members are then computed: the chosen agent and those it may call (`task` permission of the catalogue, step by step); `agents:` is computed the same way for the default agent. On a workflow without `selectable`, `--agent` is refused (`session_entry_not_selectable`).
- Patch: the `entry` block is replaced when `entry.agent` is written (or `entry: null`); writing `entry.selectable` alone has no effect.

### `modes`

- The mode is set at launch (`--mode`, launch form) and is no longer asked by the agent. The prompt always contains `Mode de workflow : <mode>`.
- A duplicate mode: `duplicate_entry`; a default missing from the list: `mode_default_not_allowed`; a `--mode` that is not allowed: `session_mode_not_allowed`.

### `circuit_breaker`

After N consecutive subagent calls without user interaction, the session stops on a ✗ decision (circuit breaker). Negative value: `negative_value`.

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
- **Patch**: merged by id, field by field (`picker` too). An input cannot be removed.

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
- **Variables**: inputs at the top level (`{{ .ticket }}`); the session context under `.oh`: `.oh.project`, `.oh.location`, `.oh.mode`, `.oh.runtime`, `.oh.lang`, `.oh.workflow`. Any other variable: `prompt_unknown_variable`.
- **Functions**:
  - `{{ data "request" .request }}` puts the value between `<oh:data name="request">` and `</oh:data>`: the agent treats it as data, never as instructions. A tag inside the value is neutralised. Every `string` or `text` input must go through `data`.
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

- **Graph**: `calls` when written, otherwise the agent's `task` permission, limited to the members. A self-delegation is only kept when written (`reviewer: { calls: [reviewer] }`). The maximum depth of the graph becomes opencode's `experimental.subagent_depth`.
- **Patch**: merged by id, field by field; `role: disabled` removes the agent (except the entry agent); `calls` is replaced as a whole.

---

## Checkpoints

```yaml
checkpoints:
  cp-2:
    label: { fr: Commit ou correction, en: Commit or fix }
    description: After the review, ask whether to commit or fix.
    mandatory: true
    mode: { manuel: pause, semi-auto: pause, auto: pause }
    remote: defer
```

The id is in kebab-case (`checkpoint_id_invalid`) and must not be an agent id (`checkpoint_agent_clash`). Checkpoints are passed in file order.

| Field | Type | Values | Absent | Patch |
|---|---|---|---|---|
| `label`, `description` | translatable text | — | the id | replaces |
| `mode.<mode>` | text | `pause`, `auto`, `skip`, `conditional` | `pause` (warning `checkpoint_mode_missing`) | merged by mode; hardens when `mandatory` |
| `condition` | text | sentence read by the agent | — (required with `conditional`) | replaces |
| `mandatory` | boolean | `true`, `false` | `false` | hardens (`true` does not go back to `false`) |
| `remote` | text | `auto` < `defer` < `forbid` | `defer` | hardens |
| `disabled` | boolean | `true` | — | removes the inherited checkpoint (patch only) |

- **Behaviors**: `pause` always asks the user; `auto` goes on alone; `skip` skips the checkpoint; `conditional` asks depending on `condition`. oh does not evaluate the condition: a `conditional` checkpoint always asks the user when it is reported.
- **Strictness order**: `skip` < `auto` < `conditional` < `pause`. On a `mandatory` checkpoint, a patch cannot choose a less strict behavior, nor remove it (`mandatory_checkpoint_removed`); `skip` on a mandatory checkpoint counts as a pause.
- **`remote`** (remote session): `auto` approved automatically; `defer` the session stops cleanly and waits for the user on the machine; `forbid` the workflow cannot run remotely.
- A mode that is neither allowed nor one of the three shipped modes: `checkpoint_mode_unknown`; `conditional` without `condition`: `checkpoint_condition_missing`.
- Checkpoint execution (3 levels: prompt, MCP tool `workflow_checkpoint`, oh plugin): [ADR-042](../architecture/adr/042-checkpoints-headless-decisions.en.md).

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

Checked before launch, in file order, in the session location.

| Field | Type | Values | Absent |
|---|---|---|---|
| `label` | translatable text | message shown on failure | the id |
| `check.path_exists` | list | relative paths, without `..`; one of them must exist | required (`precondition_check_invalid`, `precondition_path_invalid`) |
| `on_fail` | text | `suggest`, `block` | `suggest` |
| `suggest.workflow` | text | id of another workflow of the catalogue | no suggestion (`precondition_self`, `precondition_unknown_workflow`) |
| `suggest.resume` | boolean | `true`, `false` | `false` |
| `disabled` | boolean | `true` | removes an inherited precondition (patch only) |

- `suggest`: message + offer to launch `suggest.workflow`; the user may go on. With `resume: true`, oh offers to relaunch the original workflow (same inputs) once the other one has finished.
- `block`: the launch is refused.
- Remote or without interface: a `suggest` failure becomes a warning, a `block` failure stops the session.
- Patch: merged by id, field by field.

---

## Resources

| Field | Type | Absent | Patch |
|---|---|---|---|
| `models.default` | `provider/model[#variant]` | cascade without workflow level | replaces |
| `models.agents.<agent>` | `provider/model[#variant]` | — | merged by agent |
| `skills.extra` | list of skill refs | — | replaces |
| `skills.deny` | list (ref or bare name) | — | replaces |
| `mcp` | list of oh MCP server ids | MCP servers of the project | replaces |
| `plugins` | list | no plugin besides the oh plugin | replaces |
| `beads.allow` | see [Security](#beads) | | |

- **`models`**: workflow level of the [model cascade](model-resolution.en.md) (workflow·agent > workflow > project…). Invalid format: `model_invalid`; agent outside the workflow: `model_agent_unknown`.
- **`skills`**: skills come from the member agents and their dependencies (`requires:`). `extra` adds some (`skill_unknown`), `deny` removes some (a `path/skill` ref or a bare name). A denied skill required by another one: `skill_denied_required`; missing dependency: `skill_closure`; duplicate id: `skill_duplicate`.
- **`mcp`**: ids of oh MCP servers (`gitlab`, `figma`, `jira`, `team`…). Absent: the session keeps the project's MCP servers; present (even empty): only those of the list. The `workflow` server (`workflow_status`, `workflow_checkpoint`, `workflow_outputs`) is always added.
- **`plugins`**: an id (npm spec, e.g. `context-mode@latest`) or `{ id, options }`. Installed by opencode when the server starts; a plugin that fails to load does not stop the session. The module must export the opencode V2 format (`{ id, setup }`).
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
| `type` | text | `branch`, `merge_request`, `beads-ids`, `path` |
| `label` | translatable text | displayed label |

The agent declares the values with the MCP tool `workflow_outputs`. At the end of the session, they offer "Chain with…" (key `e` of the Sessions view) and `oh session results`. Patch: list replaced as a whole.

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
| `unknown_workflow`, `extends_not_found`, `invalid_extends`, `extends_cycle`, `extends_more_specific`, `duplicate_workflow` | `extends` chain cannot be built |
| `shadow_without_extends`, `must_extend_nearest` | same id as a lower layer without extending it, or without extending the nearest one |
| `loosening` | a security field is loosened: the parent value is kept |
| `enforced_field`, `enforce_unknown_field` | field locked by a parent; unknown `enforce` value |
| `entry_not_primary`, `entry_role`, `entry_disabled`, `agent_unknown`, `agent_unreachable` | invalid entry agent or member |
| `after_unknown`, `after_cycle`, `calls_unknown`, `graph_cycle` | `after` gates and delegations |
| `checkpoint_agent_clash`, `checkpoint_mode_unknown`, `checkpoint_mode_missing`, `checkpoint_condition_missing`, `mandatory_checkpoint_removed`, `patch_unknown_checkpoint` | checkpoints |
| `precondition_check_invalid`, `precondition_path_invalid`, `precondition_self`, `precondition_unknown_workflow`, `patch_unknown_precondition` | preconditions |
| `prompt_both`, `prompt_empty`, `prompt_template_path`, `prompt_template_missing`, `prompt_parse`, `prompt_unknown_variable` | prompt |
| `read_agent_writes`, `read_beads_unrestricted`, `read_beads_write` | `risk: read` rules |
| `plan_agent_writes`, `plan_beads_unrestricted`, `plan_beads_delete`, `plan_without_beads_write` | `risk: plan` rules |
| `remote_forbidden_checkpoint`, `runtime_default_not_allowed`, `mode_default_not_allowed`, `isolation_unsupported` | environments, modes, isolation |
| `model_invalid`, `model_agent_unknown`, `skill_unknown`, `skill_unknown_deny`, `skill_closure`, `skill_duplicate`, `skill_denied_required` | resources |
| `session_mode_not_allowed`, `session_runtime_not_allowed`, `session_input_unknown`, `session_input_invalid`, `session_input_missing`, `session_entry_not_selectable`, `session_entry_unknown` | launch options |

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
  developer-migrator: { role: disabled }
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
