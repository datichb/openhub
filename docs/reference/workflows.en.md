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
| `ticket` | `orchestrator-dev` | `developer`, `reviewer`, `documentarian` (on demand) | write | local, container, remote | `oh start --dev`, `-t` |
| `feature` | `orchestrator` | `pathfinder`, `planner`, `designer`, `orchestrator-dev`, `developer`, `reviewer`, `documentarian` (on demand) | write | local | `oh start` (former modes A, B and E) |
| `quick` | `developer` | — | write | local | quick session |
| `infra-change` | `operator` | — | write (infrastructure, no file) | local | former `infra` agent (application part) |
| `cadrage` | `conductor` | `pathfinder`, `planner`, `designer` | plan | local | former mode A without implementation |
| `onboarding` | `onboarder` | — | write (`docs/wiki/`) | local, remote | `oh start --onboard` (former mode C) |
| `doc-sync` | `documentarian` | — | write (`docs/wiki/`) | local | — |
| `skill-refresh` | `documentarian` | — | write (stack skills of the hub) | local | — |
| `review` | `reviewer` | — | read | local, remote | `oh review` |
| `review-feedback` | `orchestrator-dev` | `developer`, `reviewer` | write | local | `oh review feedback` |
| `audit` | `auditor` | `auditor-subagent` | read | local, remote | `oh audit`, former `infra` and `database` agents (analysis part) |
| `debug` | `debugger` | `developer`, `documentarian` (on demand) | write | local | `oh debug` (former mode D) |
| `benchmark` | `benchmarker` | — | write (`reports/benchmarks/`) | local | `oh run libre --agent benchmarker` |
| `sweep` | `conductor` | `pathfinder`, `developer`, `reviewer` | write | local | `oh start --sweep` |
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

`cp-2` unlocks `commit` and `close`: until it is approved, oh refuses `git commit` (and `revert`, `cherry-pick`, `merge`, `pull`, which commit too) and closing the ticket to every agent. Pushing is left to the user (no shipped workflow unlocks it). Before `cp-2` passes, oh itself runs the build, tests and type check of the detected stack (`verify: { from: stack, expect: pass }`): the decision card shows the result, a failure opens nothing and the user may override it (traced choice). Same verification on `cp-2` of `feature` and `review-feedback`, and on `cp-commit` of `quick`, `debug` and `sweep`; see [Checkpoints](checkpoints.en.md). Once `cp-2` is approved, `orchestrator-dev` hands back to the `developer`, who commits and then closes the ticket; oh refuses closing while the work is not committed. The `reviewer` runs the project's build and tests (and lint, type check) before its review, with the commands of the detected stack, allowed here: a failure is a 🔴 finding (verdict `corriger`); files produced by the build and not ignored are reported, since closing requires a clean tree. The same applies in `feature`, `sweep`, `review-feedback` and `libre`. A question imitating a checkpoint ("CP-2 — commit?" asked with the `question` tool) is closed by oh, which reminds the agent to call `workflow_checkpoint`. When the ticket of the session is closed (or the last checkpoint passed) and the step ends, oh declares the missing outputs (`branch`, `tickets`) and the session becomes **completed**: "Chain with…" is offered.

## `feature`

Delivers a feature end to end. The orchestrator picks the planning agent: `pathfinder` for a simple or exploratory feature, `planner` for a feature to split (ticket creation) or to classify existing tickets. It then hands over to `orchestrator-dev` for the implementation.

| Input | Type | Required | Purpose |
|---|---|---|---|
| `request` | `text` (8,000 characters max) | no | The feature in natural language, or the id of a GitLab issue or merge request |
| `tickets` | `beads-ids` | no | Existing tickets to take on (former mode B) |

Without a request or tickets, the session starts by asking which feature to deliver.

| Checkpoint | manuel | semi-auto | auto |
|---|---|---|---|
| `cp-tickets` Approve the tickets to create (mandatory, unlocks `create`) | pause | pause | pause |
| `cp-0` Approve the plan (mandatory) | pause | pause | pause |
| `cp-spec` Approve the UX/UI spec | if a spec was produced | same | same |
| `cp-1` Start the ticket | pause | auto | auto |
| `cp-2` Commit or fix (mandatory) | pause | pause | pause |
| `cp-3` Next ticket | pause | auto | auto |
| `cp-feature` Feature recap | pause | auto | auto |

No ticket is created before `cp-tickets` (with existing tickets only, `cp-tickets` confirms the list). `orchestrator-dev` only starts after `cp-0`; it passes `cp-1`, `cp-2` and `cp-3` itself, with **one `cp-2` per ticket** (each ticket is committed then closed before the next one; no grouped approval). At the end of the feature, the `orchestrator` has the whole feature reviewed by the `reviewer` (adversarial review on the working branch), before `cp-feature`. Default mode: `semi-auto`. Outputs: `tickets`, `branch`. Allowed Beads commands: `show`, `list`, `ready`, `search`, `query`, `children`, `comments`, `count`, `lint`, `find-duplicates`, `status`, `graph`, `history`, `create`, `update`, `close`, `dep`, `label`, `duplicate`, `supersede`.

The former onboarding pre-phase (mode C) is no longer part of `feature`: the `project-context` precondition suggests the `onboarding` workflow, then coming back to `feature`.

## `quick`

Direct development session with the `developer` agent, without planning. A single checkpoint, `cp-commit` (mandatory, paused in every mode): it unlocks `commit` and `close`; before it, oh refuses every `git commit` and the closing of a ticket. Pushing is left to the user. The developer infers the domain from the request and loads the matching standards. It suggests `feature` when the request turns out to be large. Allowed Beads commands: `show`, `list`, `ready`, `search`, `query`, `children`, `comments`, `count`, `status`, `graph`, `history`, `create`, `update`, `close`, `dep`, `label`.

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
| `cp-tickets` Approve the breakdown (mandatory, unlocks `create`) | pause | pause | pause |
| `cp-recap` Scoping recap | pause | pause | auto |

`planner` only starts after `cp-scope`. Before `cp-tickets`, oh refuses every ticket creation (shell rules and Beads gateway): the planner proposes the breakdown, the `conductor` passes `cp-tickets`, then relaunches the planner, which creates the tickets. Default mode: `semi-auto`. Allowed Beads commands: `show`, `list`, `ready`, `children`, `search`, `query`, `count`, `lint`, `find-duplicates`, `label`, `dep`, `create`, `update`, `comments`, `duplicate`, `supersede`. Output: `tickets`. To implement afterwards: `ticket` or `feature` on the created tickets.

## `onboarding`

Discovers the project and creates or enriches the `docs/wiki/` wiki (`doc-wiki-protocol`).

| Input | Type | Required | Purpose |
|---|---|---|---|
| `refresh` | `bool` (default: no) | no | Rediscover the project and enrich the existing wiki, without deleting anything |
| `focus` | `text` (4,000 characters max) | no | Modules or topics to dig into |

The onboarder only writes `docs/wiki/`, `ONBOARDING.md` and `CONVENTIONS.md`: it is a permission (any other write is refused, `.gitignore` and `.git/` included). It does not commit: committing the wiki is left to the user. Output: `wiki`. Allowed Beads commands (read only): `show`, `list`, `search`, `query`.

## `doc-sync`

Brings the wiki up to date with the commits since the last synchronization, then handles the queued proposals (those of the workflows without a commit: `review`, `audit`…). Precondition: `docs/wiki/index.md` exists (otherwise oh offers `onboarding`).

| Input | Type | Required | Purpose |
|---|---|---|---|
| `focus` | `text` (4,000 characters max) | no | Pages or topics to check first |

| Checkpoint | manuel | semi-auto | auto | remote |
|---|---|---|---|---|
| `cp-sync` Wiki commit (mandatory, unlocks `commit`, `wiki: sync`) | pause | pause | pause | waits for the user |

The `documentarian` calls `wiki_check` with `since_sync` (commits, changed files, pages concerned and drift computed by oh), updates the pages, checks each queued proposal, then presents `cp-sync`. The card shows the whole queue of the project: the user accepts, edits or refuses each proposal. When it is passed, oh applies the accepted ones and records the synchronization (`synced:` in `docs/wiki/index.md`); the `documentarian` commits the wiki alone. One pause in semi-auto (CP-R2 budget: one irreversible action). Allowed Beads commands (read only): `show`, `list`, `ready`, `search`, `query`. Details: [Living documentation wiki](../architecture/living-wiki.en.md#synchronization-the-doc-sync-workflow-ld-d).

## `skill-refresh`

Reviews the stack skills of the hub whose last review is old against the news of their technologies, then updates their content, `versions:` and `reviewed:`. Runs in a checkout of the hub (precondition: `skills/developer/stacks/` exists). Guide: [Stack skill freshness](../guides/skill-refresh.en.md).

| Input | Type | Required | Purpose |
|---|---|---|---|
| `skills` | `text` (2,000 characters max) | no | Skills to review (names or paths, for instance the output of `oh skill check --stale`); empty: the stack skills whose review is older than `max_age_days` (5 at most, the oldest first) |
| `notes` | `text` (8,000 characters max) | no | Release notes or changelogs given by the user, used as a source besides the web search |
| `max_age_days` | `int` (default 90) | no | Maximum age of a review, in days |

| Checkpoint | manuel | semi-auto | auto | remote |
|---|---|---|---|---|
| `cp-refresh` Skill update (mandatory, unlocks `commit`) | pause | pause | pause | — (local only) |

The `documentarian` reads the selected skills, researches each technology of `versions:` (the user's notes, then a web search on the official sources), and presents its proposal at `cp-refresh`: versions before → after, changes with their sources, new `reviewed` date. **Nothing is written before the validation**; a refusal with an instruction applies to the proposal, never to the files. After the validation, it applies the proposal and commits the skills alone. With no skill to review, the session stops without a checkpoint. The added content is in French, inspired by the sources and never copied. One pause in semi-auto (CP-R2 budget: one irreversible action). Allowed Beads commands (read only): `show`, `list`, `ready`, `search`, `query`.

## `review`

Read-only review of a branch or of recent changes. Allowed Beads command: `show`. No checkpoint.

| Input | Type | Required | Purpose |
|---|---|---|---|
| `review_mode` | `enum`: `standard`, `adversarial`, `edge-case`, `standard+adversarial`, `all` | no | Empty: the reviewer offers the choice at startup |
| `branch` | `branch` | no | Branch to review (empty: recent changes) |
| `base` | `branch` (default: `main`) | no | Base branch |

The combined modes (`standard+adversarial`, `all`) launch parallel `reviewer` sessions: the workflow declares the self-delegation `reviewer: { calls: [reviewer] }`. A self-delegation is only kept when written in `calls` (never derived from permissions); it counts as one depth level.

The reviewer runs the build and tests of the branch once, before the parallel sessions: in this `risk: read` workflow (often someone else's merge request, whose tests run its code), each command is asked to the user; refused or impossible to run, it is reported as a 🟡 "not verifiable" finding. Nothing is installed.

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

Flow: classification of the comments → `cp-fix` → fixes by the `developer`, **without committing**, project tests included → new review of the fixes by the `reviewer` → `cp-2` (unlocks `commit`) → commit `fix(review): address reviewer feedback` → recap. Pushing the branch and answering the MR threads **are left to the user** (no agent pushes nor replies on GitLab). `developer` and `reviewer` only start after `cp-fix`. Default mode: `semi-auto`. Output: `branch`. Allowed Beads commands: `show`, `list`, `ready`, `search`, `query`, `children`, `comments`, `count`, `status`, `graph`, `history` (read-only: no ticket in this workflow).

## `audit`

Read-only audit: the `auditor` coordinates `auditor-subagent`s, one per domain (skill `auditor/audit-<type>`). Allowed Beads commands: `show`, `list`. No checkpoint.

| Input | Type | Required | Purpose |
|---|---|---|---|
| `type` | `enum`: `security`, `performance`, `architecture`, `accessibility`, `ecodesign`, `observability`, `privacy`, `infra`, `database` (default: `security`) | yes | Audit type |
| `focus` | `text` (4,000 characters max) | no | Modules, files or questions to target |

- `infra`: IaC review (Terraform/OpenTofu, Kubernetes, Helm, ArgoCD), IaC security, cloud cost, drift. The `auditor-subagent` may run static analysis tools (`tflint`, `tfsec`, `checkov`, `trivy config`, `kube-score`, `kubesec`, `helm lint`, `helm template`, `infracost`, `terraform fmt -check`, `terraform validate`, `terraform plan`), each one asked; any other command, changes included, is refused. Applying a change is the job of `infra-change`.
- `database`: roles and privileges, Row-Level Security, injection surfaces, encryption at rest and in transit, backups, migration safety, exposure of sensitive data. Static reading only: queries against a real database are given to the user.

## `infra-change`

Applies an infrastructure change already written in the repository (Terraform/OpenTofu, Kubernetes, Helm, Pulumi): the `operator` checks the real context against the target, produces the plan (`terraform plan -out`, `kubectl diff`, `helm diff upgrade`, `pulumi preview`), the cost delta (`infracost` when installed), the risks and the rollback, then applies **exactly** the approved plan (`terraform apply <planfile>`) and checks the result. It modifies no file (`readonly-ops` base) and delegates to no agent: writing IaC code belongs to the `developer` (`quick`, `ticket`), read-only analysis to `audit` (`type=infra`). `isolation: strict`, local runtime only. Allowed Beads commands: read only.

| Input | Type | Required | Purpose |
|---|---|---|---|
| `request` | `text` (8,000 characters max) | yes | The change to apply (commit, directory, module, release) and its goal |
| `target` | `string` (200 characters max) | yes | Target environment as the project names it (`staging`, `prod`, Terraform workspace, kube context, Pulumi stack) |

| Checkpoint | manuel | remote |
|---|---|---|
| `cp-plan` Plan | pause | — |
| `cp-apply` Apply (mandatory, unlocks `apply`) | pause | forbidden |

`manuel` mode only (`modes: { default: manuel, allowed: [manuel] }`): `--mode semi-auto` and `--mode auto` are refused, every step of an infrastructure change stays approved by the user.

`target` is free text rather than an enum: projects name their environments in their own way (workspaces, contexts, stacks); the `operator` checks it against the real context before any plan and stops on a mismatch. Infrastructure reads are allowed, secret reads are asked, and every change (`terraform apply`, `kubectl apply`, `helm upgrade`, `pulumi up`, cloud create/delete/update commands…) is refused until `cp-apply`, then asked one by one ([infrastructure guard](../../SECURITY.md)).

## `debug`

Diagnosis of a bug or isolated problem by the `debugger`: diagnostic report (urgent actions first) and fix ticket. The `developer` and the `documentarian` (wiki enrichment asked by the `debugger`) are available on demand in the session.

| Input | Type | Required | Purpose |
|---|---|---|---|
| `issue` | `text` (8,000 characters max) | no | The observed problem; empty: the session asks for it |

The fix ticket is only created after `cp-ticket` (« Create the fix ticket », paused in every mode, unlocks `create`): the `debugger` presents the diagnosis and the ticket there, instead of asking for confirmation with the `question` tool (which it keeps in a `libre` session). A fix made in the session ends with `cp-commit` (mandatory, paused in every mode, unlocks `commit` and `close`). Output: `tickets`. Allowed Beads commands: `show`, `list`, `ready`, `search`, `query`, `lint`, `children`, `comments`, `count`, `status`, `graph`, `history`, `create`, `update`, `close`, `dep`, `label`.

**Session scratch folders** (`debugger: { scratch: { readers: [developer] } }`, see [`scratch`](workflow-schema.en.md#scratch-session-scratch-folders)): in the reproduction phase, the `debugger` writes its reproduction test or script in `~/.oh/sessions/<session>/scratch/debugger/` (path given in the `oh.scratch` entry of the session state), outside the repository and deleted with the session; it describes this test in text in the fix ticket. The `developer`, declared reader, may read this scratch file in the same session, without changing it, and makes it the final test of the project. The `documentarian` has no access to it, nor any agent of another session.

## `benchmark`

Performance measurement of a target by the `benchmarker` (modes `frontend`: Lighthouse, `api`: k6, `go`: go bench and pprof, `python`: py-spy), compared with a baseline. The code is not modified: the `benchmarker` only writes in `reports/benchmarks/` (permission); optimisations become recommendations for the `developer`.

| Input | Type | Required | Purpose |
|---|---|---|---|
| `target` | `string` (1,000 characters max) | yes | What to measure: URL, endpoint, Go package, script or command |
| `mode` | `enum`: `frontend`, `api`, `go`, `python` | no | Empty: inferred from the target and the stack |
| `baseline` | `string` (500 characters max) | no | Git ref or previous report to compare with (before/after measurement); empty: latest report in `reports/benchmarks/` |

| Checkpoint | manuel | semi-auto | auto |
|---|---|---|---|
| `cp-target` Approve the target and the load profile (mandatory, before any measurement) | pause | pause | pause |
| `cp-recap` Benchmark recap | pause | auto | auto |

At `cp-target`, the `benchmarker` presents the target, the environment (production needs an explicit confirmation), the load profile and the duration; in a `libre` session, it confirms with the `question` tool. The load tools (`k6 run`, `wrk`, `ab`, `hey`) and `curl` also ask for confirmation on each call. The `documentarian` is not a member: the wiki enrichments are listed in the report. Output: `report` (report path). Allowed Beads commands: `show`, `list`, `ready`, `search`, `query`.

## `sweep`

Reaches a cross-cutting goal by splitting it into independent subtasks: the `pathfinder` (read-only) splits, the `conductor` has the plan approved, launches the developer agents (in parallel only for disjoint file scopes), has the result verified and reviewed by the `reviewer`, then has it committed after approval. No Beads ticket.

| Input | Type | Required | Purpose |
|---|---|---|---|
| `goal` | `text` (4,000 characters max) | yes | High-level goal |
| `strategy` | `enum`: `llm` (default), `manual`, `by-file`, `by-package` | no | Decomposition |
| `tasks` | `text` (8,000 characters max) | no | Tasks, one per line (`manual`) |
| `include`, `exclude` | `string` (1,000 characters max) | no | Glob patterns, comma-separated |
| `verify` | `enum`: `none` (default), `tests`, `lint`, `build`, `all`, `custom` | no | Final verification |
| `verify_cmd` | `string` (500 characters max) | with `custom` | Verification command; `verify: custom` without it is refused at launch (`verify_cmd_missing`). Run by a `developer`: its permissions may refuse it |
| `dry_run` | `bool` (default: no) | no | Show the decomposition without running anything: `cp-plan` is not called, the session ends with `cp-recap` |

| Checkpoint | manuel | semi-auto | auto |
|---|---|---|---|
| `cp-plan` Approve the decomposition (mandatory, before any execution) | pause | pause | pause |
| `cp-commit` Commit the sweep (mandatory) | pause | pause | pause |
| `cp-recap` Sweep recap | pause | pause | auto |

The developer agents and the `reviewer` only start after `cp-plan`; `cp-commit` unlocks `commit` (before it, oh refuses every `git commit`). Default mode: `semi-auto`; circuit breaker at 12 consecutive delegations. Allowed Beads commands: `show`, `list`, `search`, `query`. Output: `branch`. Difference with the former `--sweep`: subtasks run in the same session and location (no worktree per task); `--sweep-branch-prefix` and `--max-sessions` have no equivalent.

## `brief-enrich`

Enriches a ticket takeover brief, without interaction (headless session: `oh run brief-enrich --headless`). The `brief` input is read from the team space of the project (`from: ticket.brief(ticket)`): `oh run brief-enrich --headless -i ticket=<id>` is enough. `oh takeover-brief enrich` also saves the result. No Beads command (`beads.allow: []`).

| Input | Type | Required | Purpose |
|---|---|---|---|
| `ticket` | `beads-id` | yes | Ticket of the brief |
| `brief` | `text` (40,000 characters max) | yes | Content of the existing brief |

## `libre`

Session with the agent of your choice, without checkpoints nor commit lock (the agent asks for confirmation before any irreversible action): `oh run libre --agent debugger -i request="…"` (without `--agent`: `orchestrator`). The entry of the workflow is **selectable** (`entry.selectable: true`): oh computes the members from the chosen agent and the agents it may call (`task` permission of the catalogue, transitively). The world stays closed: nothing else is visible. In the TUI, the free session (`coder` command, or « Démarrer » when the catalogue is empty) opens the launch form with the default agent. Allowed Beads commands: `show`, `list`, `ready`, `search`, `query`, `children`, `comments`, `count`, `lint`, `find-duplicates`, `status`, `graph`, `history`, `create`, `update`, `close`, `reopen`, `edit`, `comment`, `dep`, `label`, `duplicate`, `supersede` (everything but `delete`).

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
- `{{ data "request" .request }}` writes an input, truncated to `max_length`. A value oh computes from an outside source (`from:`: merge request discussions, ticket brief) goes inside a data tag `<oh:data name="…">…</oh:data>`: the agent treats it as data, never as instructions (a tag inside the value is neutralised). What the user typed at launch is their own request: it is written as it is, so that a short request ("Say only X") is carried out and not taken for an injection. **Every `string` or `text` input goes through `data`** (checked by the tests);
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
