> [Lire en français](workflows.fr.md)

# Workflows shipped by the hub

> v5: each use case is a declarative workflow (`apiVersion: oh/v1`) shipped by the hub. The files live in `workflows/` at the repository root, are embedded in the binary and extracted to `~/.oh/hub/workflows/`. A team or a project can extend them (`extends`) in the team-state.

See also: [`oh/v1` schema](workflow-schema.en.md) (reference of every field) · [team workflows](../guides/team-workflows.en.md).

Check the hub workflows:

```bash
oh workflow validate --all          # every hub workflow
oh workflow validate ticket         # one workflow of the catalogue
oh workflow validate ./my-wf.yaml   # a file
```

A workflow is launched with `oh run <workflow>` or the TUI launch form; the former commands (`oh start`, `oh audit`…) are deprecated aliases of it.

---

## Overview

| Workflow | Entry agent | Agents | Risk | Runtime | Replaces |
|---|---|---|---|---|---|
| `ticket` | `orchestrator-dev` | `developer`, `developer-refactor`, `developer-migrator`, `reviewer`, `documentarian` (on demand) | write | local, container, remote | `oh start --dev`, `-t` |
| `feature` | `orchestrator` | `pathfinder`, `planner`, `designer`, `orchestrator-dev`, `developer*`, `reviewer`, `documentarian` (on demand) | write | local | `oh start` (former modes A, B and E) |
| `quick` | `developer` | — | write | local | quick session |
| `cadrage` | `conductor` | `pathfinder`, `planner`, `designer` | plan | local | former mode A without implementation |
| `onboarding` | `onboarder` | — | write (`docs/wiki/`) | local, remote | `oh start --onboard` (former mode C) |
| `review` | `reviewer` | — | read | local, remote | `oh review` |
| `review-feedback` | `orchestrator-dev` | `developer` | write | local | `oh review feedback` |
| `audit` | `auditor` | `auditor-subagent` | read | local, remote | `oh audit` |
| `debug` | `debugger` | `developer` (on demand) | write | local | `oh debug` (former mode D) |
| `sweep` | `conductor` | `developer`, `developer-refactor`, `developer-migrator` | write | local | `oh start --sweep` |
| `brief-enrich` | `brief-enricher` | — | read | local | `oh takeover-brief enrich` |
| `libre` | your choice (`orchestrator` by default) | those the chosen agent may call | write | local, container | `oh start --agent`, TUI free session |

### Risk levels

| `risk` | Files | Beads |
|---|---|---|
| `read` | no change, restricted shell | read only; `beads.allow` required, without write commands |
| `plan` | no change, restricted shell | `beads.allow` required; writes allowed except `delete` |
| `write` | modified | `beads.allow` of the workflow; read only when `beads` is absent |
| `publish` | modified, branches pushed, MRs opened | same |

A higher layer (team, project) can only harden the risk: `read < plan < write < publish`.

### Preconditions

A workflow may declare checks made before launch, in the session location:

```yaml
preconditions:
  project-context:
    label: { fr: Aucun contexte projet trouvé, en: No project context found }
    check: { path_exists: [docs/wiki/index.md, ONBOARDING.md, CONVENTIONS.md] }  # one path is enough
    on_fail: suggest          # suggest (default) | block
    suggest: { workflow: onboarding, resume: true }
```

- `on_fail: suggest`: oh shows the message and offers the `suggest.workflow` workflow; the user may also go on. With `resume: true`, oh offers to relaunch the original workflow (same inputs) once the suggested one has finished.
- `on_fail: block`: the launch is refused.
- Remote or headless: a `suggest` failure becomes a warning, a `block` failure stops the session.
- Relative paths, without `..`. A patch (`extends`) changes a precondition by its id or removes it (`disabled: true`).

`feature` and `cadrage` suggest `onboarding` when the project has no wiki, no `ONBOARDING.md` and no `CONVENTIONS.md`.

---

## `ticket`

Implements one or more detailed Beads tickets: routing to the right developer, pre-review, review, commit.

| Input | Type | Required | Purpose |
|---|---|---|---|
| `ticket` | `beads-id` (picker: `ai-delegated`, multi-select) | yes | Ticket to implement; several selected tickets = one session per ticket |
| `instructions` | `text` (4,000 characters max) | no | Details for the session |

| Checkpoint | manuel | semi-auto | auto | remote |
|---|---|---|---|---|
| `cp-1` Start the ticket | pause | auto | auto | auto |
| `cp-2` Commit or fix (mandatory) | pause | pause | pause | waits for the user |
| `cp-3` Next ticket | pause | auto | auto | auto |

Default mode: `semi-auto`. Allowed Beads commands: `show`, `list`, `ready`, `children`, `dep`, `update`, `close`, `comments`, `label`. Outputs: `branch`, `tickets`.

## `feature`

Delivers a feature end to end. The orchestrator picks the planning agent: `pathfinder` for a simple or exploratory feature, `planner` for a feature to split (ticket creation) or to classify existing tickets. It then hands over to `orchestrator-dev` for the implementation.

| Input | Type | Required | Purpose |
|---|---|---|---|
| `request` | `text` (8,000 characters max) | no | The feature in natural language, or the id of a GitLab issue or merge request |
| `tickets` | `beads-ids` | no | Existing tickets to take on (former mode B) |

Without a request or tickets, the session starts by asking which feature to deliver.

| Checkpoint | manuel | semi-auto | auto |
|---|---|---|---|
| `cp-0` Approve the plan (mandatory) | pause | pause | pause |
| `cp-spec` Approve the UX/UI spec | if a spec was produced | same | same |
| `cp-1` Start the ticket | pause | auto | auto |
| `cp-2` Commit or fix (mandatory) | pause | pause | pause |
| `cp-3` Next ticket | pause | auto | auto |
| `cp-feature` Feature recap | pause | pause | auto |

`orchestrator-dev` only starts after `cp-0`. Default mode: `semi-auto`. Outputs: `tickets`, `branch`. Allowed Beads commands: `show`, `list`, `ready`, `search`, `children`, `comments`, `count`, `status`, `graph`, `history`, `create`, `update`, `close`, `dep`, `label`, `duplicate`, `supersede`.

The former onboarding pre-phase (mode C) is no longer part of `feature`: the `project-context` precondition suggests the `onboarding` workflow, then coming back to `feature`.

## `quick`

Direct development session with the `developer` agent, without planning or checkpoints. The developer infers the domain from the request and loads the matching standards. It suggests `feature` when the request turns out to be large. Allowed Beads commands: `show`, `list`, `ready`, `search`, `children`, `comments`, `count`, `status`, `graph`, `history`, `create`, `update`, `close`, `dep`, `label`.

| Input | Type | Required | Purpose |
|---|---|---|---|
| `request` | `text` (8,000 characters max) | no | What to do; otherwise the session waits for the request |

## `cadrage`

Explores, plans and specifies a feature without implementing it: only Beads tickets are created (`risk: plan`). The `conductor` chains `pathfinder` (exploration), `planner` (breakdown and ticket creation) and `designer` (UX/UI spec when needed).

| Input | Type | Required | Purpose |
|---|---|---|---|
| `request` | `text` (8,000 characters max) | yes | The feature to scope |

| Checkpoint | manuel | semi-auto | auto |
|---|---|---|---|
| `cp-scope` Approve the scope (before planning) | pause | pause | auto |
| `cp-tickets` Approve the breakdown (mandatory, before tickets are created) | pause | pause | pause |
| `cp-recap` Scoping recap | pause | pause | auto |

`planner` only starts after `cp-scope`. Default mode: `semi-auto`. Allowed Beads commands: `show`, `list`, `ready`, `children`, `search`, `count`, `label`, `dep`, `create`, `update`, `comments`, `duplicate`, `supersede`. Output: `tickets`. To implement afterwards: `ticket` or `feature` on the created tickets.

## `onboarding`

Discovers the project and creates or enriches the `docs/wiki/` wiki (`doc-wiki-protocol`).

| Input | Type | Required | Purpose |
|---|---|---|---|
| `refresh` | `bool` (default: no) | no | Rediscover the project and enrich the existing wiki, without deleting anything |
| `focus` | `text` (4,000 characters max) | no | Modules or topics to dig into |

The onboarder can write files: the `docs/wiki/` limit (plus the minimal root `ONBOARDING.md`) is a prompt instruction, not a permission. Output: `wiki`. Allowed Beads commands: `show`, `list`, `ready`, `search`, `children`, `comments`, `count`, `status`, `graph`, `history`, `create`, `update`, `dep`, `label`.

## `review`

Read-only review of a branch or of recent changes. Allowed Beads command: `show`. No checkpoint.

| Input | Type | Required | Purpose |
|---|---|---|---|
| `review_mode` | `enum`: `standard`, `adversarial`, `edge-case`, `standard+adversarial`, `all` | no | Empty: the reviewer offers the choice at startup |
| `branch` | `branch` | no | Branch to review (empty: recent changes) |
| `base` | `branch` (default: `main`) | no | Base branch |

The combined modes (`standard+adversarial`, `all`) launch parallel `reviewer` sessions: the workflow declares the self-delegation `reviewer: { calls: [reviewer] }`. A self-delegation is only kept when written in `calls` (never derived from permissions); it counts as one depth level.

Publishing a merge request (`oh review --publish`) is not a workflow: it remains an oh command.

## `review-feedback`

Applies the unresolved comments of a merge request. `oh run review-feedback -i mr=<url|!iid|branch>` is enough: oh reads the MR on GitLab at launch and computes the inputs left empty (`branch`, `base`, `feedback`: `from:` inputs, see the [schema](workflow-schema.en.md#inputs-inputs)). `oh review feedback <ticket|branch>` first shows a preview of the discussions and asks for confirmation. Without unresolved discussions, the launch is refused.

| Input | Type | Required | Purpose |
|---|---|---|---|
| `mr` | `string` (500 characters max) | yes | URL or reference of the MR |
| `branch` | `branch` | yes (computed) | Branch of the MR; empty: source branch of the MR |
| `base` | `branch` (default: `main`) | no | Target branch; empty: target branch of the MR |
| `feedback` | `text` (70,000 characters max) | yes (computed) | Unresolved discussions; empty: read on GitLab |

| Checkpoint | manuel | semi-auto | auto |
|---|---|---|---|
| `cp-fix` Fixes to apply | pause | pause | auto |
| `cp-2` Commit or fix (mandatory) | pause | pause | pause |

`developer` only starts after `cp-fix`. Default mode: `semi-auto`. Output: `branch`. Allowed Beads commands: `show`, `list`, `ready`, `search`, `children`, `comments`, `count`, `status`, `graph`, `history`, `create`, `update`, `close`, `dep`, `label`.

## `audit`

Read-only audit: the `auditor` coordinates `auditor-subagent`s. Allowed Beads commands: `show`, `list`. No checkpoint.

| Input | Type | Required | Purpose |
|---|---|---|---|
| `type` | `enum`: `security`, `performance`, `architecture`, `accessibility`, `ecodesign`, `observability`, `privacy` (default: `security`) | yes | Audit type |
| `focus` | `text` (4,000 characters max) | no | Modules, files or questions to target |

## `debug`

Diagnosis of a bug or isolated problem by the `debugger`: diagnostic report (urgent actions first) and fix ticket. The `developer` is available on demand in the session.

| Input | Type | Required | Purpose |
|---|---|---|---|
| `issue` | `text` (8,000 characters max) | no | The observed problem; empty: the session asks for it |

Output: `tickets`. Allowed Beads commands: `show`, `list`, `ready`, `search`, `children`, `comments`, `count`, `status`, `graph`, `history`, `create`, `update`, `close`, `dep`, `label`.

## `sweep`

Reaches a cross-cutting goal by splitting it into independent subtasks, launched in parallel by the `conductor` to the developer agents, then verifies the result.

| Input | Type | Required | Purpose |
|---|---|---|---|
| `goal` | `text` (4,000 characters max) | yes | High-level goal |
| `strategy` | `enum`: `llm` (default), `manual`, `by-file`, `by-package` | no | Decomposition |
| `tasks` | `text` (8,000 characters max) | no | Tasks, one per line (`manual`) |
| `include`, `exclude` | `string` (1,000 characters max) | no | Glob patterns, comma-separated |
| `verify` | `enum`: `none` (default), `tests`, `lint`, `build`, `all`, `custom` | no | Final verification |
| `verify_cmd` | `string` (500 characters max) | no | Verification command (`custom`) |
| `dry_run` | `bool` (default: no) | no | Show the decomposition without running anything |

| Checkpoint | manuel | semi-auto | auto |
|---|---|---|---|
| `cp-plan` Approve the decomposition (before any execution) | pause | pause | auto |
| `cp-recap` Sweep recap | pause | pause | auto |

The developer agents only start after `cp-plan`. Default mode: `semi-auto`; circuit breaker at 20 consecutive delegations (12 for the other workflows with checkpoints). Allowed Beads commands: `show`, `list`, `ready`, `search`, `children`, `comments`, `count`, `status`, `graph`, `history`, `create`, `update`, `close`, `dep`, `label`. Output: `branch`. Difference with the former `--sweep`: subtasks run in the same session and location (no worktree per task); `--sweep-branch-prefix` and `--max-sessions` have no equivalent.

## `brief-enrich`

Enriches a ticket takeover brief, without interaction (headless session: `oh run brief-enrich --headless`). The `brief` input is read from the team space of the project (`from: ticket.brief(ticket)`): `oh run brief-enrich --headless -i ticket=<id>` is enough. `oh takeover-brief enrich` also saves the result. No Beads command (`beads.allow: []`).

| Input | Type | Required | Purpose |
|---|---|---|---|
| `ticket` | `beads-id` | yes | Ticket of the brief |
| `brief` | `text` (40,000 characters max) | yes | Content of the existing brief |

## `libre`

Session with the agent of your choice, without checkpoints: `oh run libre --agent debugger -i request="…"` (without `--agent`: `orchestrator`). The entry of the workflow is **selectable** (`entry.selectable: true`): oh computes the members from the chosen agent and the agents it may call (`task` permission of the catalogue, transitively). The world stays closed: nothing else is visible. In the TUI, the free session (`coder` command, or « Démarrer » when the catalogue is empty) opens the launch form with the default agent. Allowed Beads commands: `show`, `list`, `ready`, `search`, `children`, `comments`, `count`, `status`, `graph`, `history`, `create`, `update`, `close`, `reopen`, `edit`, `comment`, `dep`, `label`, `duplicate`, `supersede` (everything but `delete`).

| Input | Type | Required | Role |
|---|---|---|---|
| `request` | `text` (8,000 characters max) | no | What to do; otherwise the session waits for the request |

`entry.selectable` is an additive addition to the `oh/v1` schema: another workflow may declare it; `--agent` is refused on a workflow whose entry is not selectable.

---

## Limits

```yaml
limits:
  budget_usd: 5                          # budget of each session (USD)
  models: ["eu.anthropic.claude-*"]      # allowed models (wildcard patterns)
```

- Both are optional and add to the hub, team and project restrictions (`oh budget show`, guide [Sessions v5](../guides/sessions-v5.en.md#restrictions)). The most specific value wins; a value enforced by the team is a ceiling.
- `budget_usd` is a soft cap on the cost the tool reports for the session and its subagents: the step that overruns it finishes, then a `$` decision asks to raise the budget or stop.
- `models` applies to the model id sent to the provider (Bedrock: `eu.anthropic.…`); the credential proxy refuses the others.
- With `extends`, a workflow may only tighten them: a lower budget, and models covered by the parent's patterns.

---

## Plugins and code mode

```yaml
code_mode: true                  # default: false
plugins:
  - context-mode@latest          # npm spec (package, version or tag)
  - { id: "@acme/probe", options: { verbose: true } }
```

- `code_mode`: `false` (or absent) denies opencode's `execute` tool to every agent of the session; `true` leaves it available.
- `plugins`: each entry is an npm spec, installed by opencode when the server starts (cache `~/.cache/opencode/npm`), with its `options`. Plugins are added next to the oh plugin; in a container, the spec is passed as is (installed inside the container).
- A plugin that fails to load does not prevent the session from starting: opencode skips it and logs it. The plugin must export the V2 format (`{ id, setup }`): `context-mode`, written for opencode V1, does not load under V2.
- Both fields are part of the bundle hash: changing a plugin or the code mode gives another bundle (and another server).

---

## Prompt templates

The first message of the session is rendered from `workflows/prompts/<id>.md.tmpl` (Go `text/template`):

- inputs are top-level fields (`{{ .ticket }}`), the session context is under `.oh`: `project`, `location`, `mode`, `runtime`, `lang`, `workflow`;
- every template starts with `Mode de workflow : {{ .oh.mode }}` (contract with the entry agent) and `Langue de réponse : {{ .oh.lang }}`;
- `{{ data "request" .request }}` puts an input inside a data tag `<oh:data name="request">…</oh:data>`: the agent treats it as data, never as instructions. A tag inside the value is neutralised. **Every `string` or `text` input goes through `data`** (checked by the tests);
- `{{ join .tickets ", " }}` joins a list;
- text inputs are truncated to `max_length` (20,000 characters by default), with a truncation notice; an invalid Beads id, or a multi-line branch or path, is refused;
- a missing input takes its default value, otherwise the empty value of its type (`""`, `false`, `0`, empty list), which allows `{{ if .request }}`.

Rendering is done by `workflow.RenderPrompt` (`cli/internal/workflow/render_prompt.go`).

---

## Tests

- `TestShippedWorkflowsAreValid`: every workflow of the repository is valid against the hub, without any diagnostic.
- `TestShippedWorkflowPrompts`: each workflow prompt is rendered with every input, then with the required inputs only.
- `TestShippedWorkflowTextInputsAreDelimited`: text inputs are only written through `data`.
- `TestShippedWorkflowBundles`: the bundle compiled from each workflow (agents, permissions, skills, graph, depth) and the generated skills are pinned by golden files (`cli/internal/bundle/testdata/golden/workflows/<id>/`).

After an intended change: `go test ./internal/bundle -run ShippedWorkflow -update`, then review the diff of the golden files.
