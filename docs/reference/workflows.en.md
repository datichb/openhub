# Workflows shipped by the hub

> v5: each use case is a declarative workflow (`apiVersion: oh/v1`) shipped by the hub. The files live in `workflows/` at the repository root, are embedded in the binary and extracted to `~/.oh/hub/workflows/`. A team or a project can extend them (`extends`) from phase 2.

Check the hub workflows:

```bash
oh workflow validate --all          # every hub workflow
oh workflow validate ticket         # one workflow of the catalogue
oh workflow validate ./my-wf.yaml   # a file
```

Launching with `oh run <workflow>` and the TUI launch form come with the rest of phase 1; until then, `oh start` and the existing commands remain the entry points.

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

### Risk levels

| `risk` | Files | Beads |
|---|---|---|
| `read` | no change, restricted shell | read only; `beads.allow` required, without write commands |
| `plan` | no change, restricted shell | `beads.allow` required; writes allowed except `delete` |
| `write` | modified | unrestricted when `beads` is absent |
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

`orchestrator-dev` only starts after `cp-0`. Default mode: `semi-auto`. Outputs: `tickets`, `branch`.

The former onboarding pre-phase (mode C) is no longer part of `feature`: the `project-context` precondition suggests the `onboarding` workflow, then coming back to `feature`.

## `quick`

Direct development session with the `developer` agent, without planning or checkpoints. The developer infers the domain from the request and loads the matching standards. It suggests `feature` when the request turns out to be large.

| Input | Type | Required | Purpose |
|---|---|---|---|
| `request` | `text` (8,000 characters max) | no | What to do; otherwise the session waits for the request |

## `cadrage`

Explores, plans and specifies a feature without implementing it: only Beads tickets are created (`risk: plan`). The `conductor` chains `pathfinder` (exploration), `planner` (breakdown and ticket creation) and `designer` (UX/UI spec when needed).

| Input | Type | Required | Purpose |
|---|---|---|---|
| `request` | `text` (8,000 characters max) | yes | The feature to scope |

Checkpoints: `cp-scope` (scope, before planning), `cp-tickets` (mandatory: breakdown approved before tickets are created), `cp-recap`. Output: `tickets`. To implement afterwards: `ticket` or `feature` on the created tickets.

## `onboarding`

Discovers the project and creates or enriches the `docs/wiki/` wiki (`doc-wiki-protocol`).

| Input | Type | Required | Purpose |
|---|---|---|---|
| `refresh` | `bool` (default: no) | no | Rediscover the project and enrich the existing wiki, without deleting anything |
| `focus` | `text` (4,000 characters max) | no | Modules or topics to dig into |

The onboarder can write files: the `docs/wiki/` limit (plus the minimal root `ONBOARDING.md`) is a prompt instruction, not a permission. Output: `wiki`.

## `review`

Read-only review of a branch or of recent changes.

| Input | Type | Required | Purpose |
|---|---|---|---|
| `review_mode` | `enum`: `standard`, `adversarial`, `edge-case`, `standard+adversarial`, `all` | no | Empty: the reviewer offers the choice at startup |
| `branch` | `branch` | no | Branch to review (empty: recent changes) |
| `base` | `branch` (default: `main`) | no | Base branch |

The combined modes (`standard+adversarial`, `all`) launch parallel `reviewer` sessions: the workflow declares the self-delegation `reviewer: { calls: [reviewer] }`. A self-delegation is only kept when written in `calls` (never derived from permissions); it counts as one depth level.

Publishing a merge request (`oh review --publish`) is not a workflow: it remains an oh command.

## `review-feedback`

Applies the unresolved comments of a merge request. oh fetches the discussions from GitLab at launch and passes them in the `feedback` input.

| Input | Type | Required | Purpose |
|---|---|---|---|
| `mr` | `string` | yes | URL or reference of the MR |
| `branch` | `branch` | yes | Branch of the MR |
| `base` | `branch` (default: `main`) | no | Target branch |
| `feedback` | `text` (70,000 characters max) | yes | Unresolved discussions |

Checkpoints: `cp-fix` (fixes to apply), `cp-2` (mandatory: commit or fix). Output: `branch`.

## `audit`

Read-only audit: the `auditor` coordinates `auditor-subagent`s.

| Input | Type | Required | Purpose |
|---|---|---|---|
| `type` | `enum`: `security`, `performance`, `architecture`, `accessibility`, `ecodesign`, `observability`, `privacy` (default: `security`) | yes | Audit type |
| `focus` | `text` (4,000 characters max) | no | Modules, files or questions to target |

## `debug`

Diagnosis of a bug or isolated problem by the `debugger`: diagnostic report (urgent actions first) and fix ticket. The `developer` is available on demand in the session.

| Input | Type | Required | Purpose |
|---|---|---|---|
| `issue` | `text` (8,000 characters max) | no | The observed problem; empty: the session asks for it |

Output: `tickets`.

## `sweep`

Reaches a cross-cutting goal by splitting it into independent subtasks, launched in parallel by the `conductor` to the developer agents, then verifies the result.

| Input | Type | Required | Purpose |
|---|---|---|---|
| `goal` | `text` | yes | High-level goal |
| `strategy` | `enum`: `llm` (default), `manual`, `by-file`, `by-package` | no | Decomposition |
| `tasks` | `text` | no | Tasks, one per line (`manual`) |
| `include`, `exclude` | `string` | no | Glob patterns, comma-separated |
| `verify` | `enum`: `none` (default), `tests`, `lint`, `build`, `all`, `custom` | no | Final verification |
| `verify_cmd` | `string` | no | Verification command (`custom`) |
| `dry_run` | `bool` | no | Show the decomposition without running anything |

Checkpoints: `cp-plan` (decomposition, before any execution), `cp-recap`. Difference with the former `--sweep`: subtasks run in the same session and location (no worktree per task); `--sweep-branch-prefix` and `--max-sessions` have no equivalent.

## `brief-enrich`

Enriches a ticket takeover brief, without interaction (headless session). oh provides the brief and saves the result.

| Input | Type | Required | Purpose |
|---|---|---|---|
| `ticket` | `beads-id` | yes | Ticket of the brief |
| `brief` | `text` (40,000 characters max) | yes | Content of the existing brief |

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
