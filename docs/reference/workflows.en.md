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

The other workflows (`cadrage`, `onboarding`, `review`, `review-feedback`, `audit`, `debug`, `sweep`, `brief-enrich`) follow.

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

The former onboarding pre-phase (mode C) is no longer part of `feature`: the `onboarding` workflow replaces it.

## `quick`

Direct development session with the `developer` agent, without planning or checkpoints. The developer infers the domain from the request and loads the matching standards. It suggests `feature` when the request turns out to be large.

| Input | Type | Required | Purpose |
|---|---|---|---|
| `request` | `text` (8,000 characters max) | no | What to do; otherwise the session waits for the request |

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
