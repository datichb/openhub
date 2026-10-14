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
| `spec` (alias `cadrage`) | `conductor` | `pathfinder` (when the request compares designs), `planner` (writes `docs/wiki/specs/`), `designer` | plan (+ `writes:`) | local | `cadrage` (former mode A without implementation), with a written and checked spec |
| `implement-spec` | `orchestrator-dev` | `developer`, `reviewer`, `documentarian` (on demand) | write | local | implementation part of `feature` |
| `onboarding` | `onboarder` | — | write (`docs/wiki/`) | local, remote | `oh start --onboard` (former mode C) |
| `doc-sync` | `documentarian` | — | write (`docs/wiki/`) | local | — |
| `skill-refresh` | `documentarian` | — | write (stack skills of the hub) | local | — |
| `release` | `release-manager` | — | publish (CHANGELOG, release commit and tag; never a push) | local | — |
| `review` | `reviewer` | — | read | local, remote | `oh review` |
| `review-feedback` | `orchestrator-dev` | `developer`, `reviewer` | write | local | `oh review feedback` |
| `audit` | `auditor` | `auditor-subagent` | read | local, remote | `oh audit`, former `infra` and `database` agents (analysis part) |
| `debug` | `debugger` | `developer` (on demand) | write | local | `oh debug` (former mode D) |
| `incident` | `debugger` (writes `docs/wiki/postmortems/`) | `developer` | write | local | — |
| `security-fix` | `conductor` | `developer`, `auditor-subagent` | write | local | — |
| `spike` | `pathfinder` (writes `docs/wiki/decisions/`) | `developer` (throwaway prototype) | write | local | — |
| `benchmark` | `benchmarker` | — | write (`reports/benchmarks/`) | local | `oh run libre --agent benchmarker` |
| `sweep` | `conductor` | `pathfinder`, `developer`, `reviewer` | write | local | `oh start --sweep` |
| `brief-enrich` | `brief-enricher` | — | read | local | `oh takeover-brief enrich` |
| `libre` | your choice (`orchestrator` by default) | those the chosen agent may call | write | local, container | `oh start --agent`, TUI free session |

### Risk levels

| `risk` | Files | Beads |
|---|---|---|
| `read` | no change, restricted shell | read only; `beads.allow` required, without write commands |
| `plan` | no change, restricted shell; only the documentation paths of `writes:` (and their commit) are open to the agents that declare them | `beads.allow` required; writes allowed except `delete` |
| `write` | modified | `beads.allow` of the workflow; read only when `beads` is absent |
| `publish` | modified, tags created, MRs opened; never remote | same |

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
- Other tests: `git_clean: true` (no uncommitted change, `.beads/` excluded), `branch: [main, "release/*"]` (current branch), `tools: [goreleaser]` (commands in the `PATH`); on failure, the message says what was found (changes, branch, missing tools). See the [schema](workflow-schema.en.md#preconditions).
- Relative paths, without `..`. A patch (`extends`) changes a precondition by its id or removes it (`disabled: true`).

`release` refuses to start on a dirty tree or outside `main`, `master` and `release/*`. `feature`, `spec` (and its alias `cadrage`) and `implement-spec` suggest `onboarding` when the project has no wiki, no `ONBOARDING.md` and no `CONVENTIONS.md`.

---

## `ticket`

Implements one or more detailed Beads tickets: routing to the right developer, pre-review, review, commit.

| Input | Type | Required | Purpose |
|---|---|---|---|
| `ticket` | `beads-id` (picker: `ai-delegated`, multi-select) | yes | Ticket to implement; several selected tickets = one session per ticket |
| `method` | `enum` (`standard`, `tdd`) | no (`standard`) | Method: `tdd` adds `cp-seams` and `cp-red` (tests written first) |
| `instructions` | `text` (4,000 characters max) | no | Details for the session |

| Checkpoint | manuel | semi-auto | auto | remote |
|---|---|---|---|---|
| `cp-1` Start the ticket | pause | auto | auto | auto |
| `cp-seams` Seams and planned tests (`tdd` method) | pause | pause | auto | auto |
| `cp-red` Failing tests (`tdd` method, verified by oh) | auto | auto | auto | auto |
| `cp-2` Commit or fix (mandatory) | pause | pause | pause | waits for the user |
| `cp-3` Next ticket | pause | auto | auto | auto |

Default mode: `semi-auto`. Allowed Beads commands: `show`, `list`, `ready`, `children`, `dep`, `update`, `close`, `comments`, `label`. Outputs: `branch`, `tickets`.

With the `tdd` method (`-i method=tdd`), the `developer` passes `cp-seams` itself (it presents the seams, the interfaces and the planned tests), then `cp-red`, where oh checks that the tests written fail for the right reason (the code builds, the tests compile and fail) and that a test file changed since `cp-seams`; the other changed files are noted. Without this method nothing changes: no checkpoint, no text. See [TDD method](checkpoints.en.md#tdd-method-cp-seams-and-cp-red-r7).

`cp-2` unlocks `commit` and `close`: until it is approved, oh refuses `git commit` (and `revert`, `cherry-pick`, `merge`, `pull`, which commit too) and closing the ticket to every agent. Pushing is left to the user (no shipped workflow unlocks it). Before `cp-2` passes, oh itself runs the build, tests and type check of the detected stack (`verify: { from: stack, expect: pass }`): the decision card shows the result, a failure opens nothing and the user may override it (traced choice). Same verification on `cp-2` of `feature` and `review-feedback`, and on `cp-commit` of `quick`, `debug` and `sweep`; see [Checkpoints](checkpoints.en.md). Once `cp-2` is approved, `orchestrator-dev` hands back to the `developer`, who commits and then closes the ticket; oh refuses closing while the work is not committed. The `reviewer` runs the project's build and tests (and lint, type check) before its review, with the commands of the detected stack, allowed here: a failure is a 🔴 finding (verdict `corriger`); files produced by the build and not ignored are reported, since closing requires a clean tree. The same applies in `feature`, `sweep`, `review-feedback` and `libre`. A question imitating a checkpoint ("CP-2 — commit?" asked with the `question` tool) is closed by oh, which reminds the agent to call `workflow_checkpoint`. When the ticket of the session is closed (or the last checkpoint passed) and the step ends, oh declares the missing outputs (`branch`, `tickets`) and the session becomes **completed**: "Chain with…" is offered.

## `feature`

Delivers a feature end to end. The orchestrator picks the planning agent: `pathfinder` for a simple or exploratory feature, `planner` for a feature to split (ticket creation) or to classify existing tickets. It then hands over to `orchestrator-dev` for the implementation.

| Input | Type | Required | Purpose |
|---|---|---|---|
| `request` | `text` (8,000 characters max) | no | The feature in natural language, or the id of a GitLab issue or merge request |
| `tickets` | `beads-ids` | no | Existing tickets to take on (former mode B) |
| `method` | `enum` (`standard`, `tdd`) | no (`standard`) | Method: `tdd` adds `cp-seams` and `cp-red` to each ticket |

Without a request or tickets, the session starts by asking which feature to deliver.

| Checkpoint | manuel | semi-auto | auto |
|---|---|---|---|
| `cp-tickets` Approve the tickets to create (mandatory, unlocks `create`) | pause | pause | pause |
| `cp-0` Approve the plan (mandatory) | pause | pause | pause |
| `cp-spec` Approve the UX/UI spec | if a spec was produced | same | same |
| `cp-1` Start the ticket | pause | auto | auto |
| `cp-seams` Seams and planned tests (`tdd` method) | pause | pause | auto |
| `cp-red` Failing tests (`tdd` method, verified by oh) | auto | auto | auto |
| `cp-2` Commit or fix (mandatory) | pause | pause | pause |
| `cp-3` Next ticket | pause | auto | auto |
| `cp-feature` Feature recap | pause | auto | auto |

No ticket is created before `cp-tickets` (with existing tickets only, `cp-tickets` confirms the list). `orchestrator-dev` only starts after `cp-0`; it passes `cp-1`, `cp-2` and `cp-3` itself (with the `tdd` method, the `developer` passes `cp-seams` and `cp-red`, once per ticket), with **one `cp-2` per ticket** (each ticket is committed then closed before the next one; no grouped approval). At the end of the feature, the `orchestrator` has the whole feature reviewed by the `reviewer` (adversarial review on the working branch), before `cp-feature`. Default mode: `semi-auto`. Outputs: `tickets`, `branch`. Allowed Beads commands: `show`, `list`, `ready`, `search`, `query`, `children`, `comments`, `count`, `lint`, `find-duplicates`, `status`, `graph`, `history`, `create`, `update`, `close`, `dep`, `label`, `duplicate`, `supersede`.

The former onboarding pre-phase (mode C) is no longer part of `feature`: the `project-context` precondition suggests the `onboarding` workflow, then coming back to `feature`.

The same chain exists as two composed workflows (R5): [`spec`](#spec), which proposes [`implement-spec`](#implement-spec) on the created tickets when it ends. `feature` is unchanged in this version; it will become the chain `spec` → `implement-spec` (an alias routed by the inputs: `aliases: [{ id: feature, with: tickets }]` in `implement-spec`, `feature` in `spec`) once the team layers extending it are migrated.

## `quick`

Direct development session with the `developer` agent, without planning. A single checkpoint, `cp-commit` (mandatory, paused in every mode): it unlocks `commit` and `close`; before it, oh refuses every `git commit` and the closing of a ticket. Pushing is left to the user. The developer infers the domain from the request and loads the matching standards. It suggests `feature` when the request turns out to be large. Allowed Beads commands: `show`, `list`, `ready`, `search`, `query`, `children`, `comments`, `count`, `status`, `graph`, `history`, `create`, `update`, `close`, `dep`, `label`.

| Input | Type | Required | Purpose |
|---|---|---|---|
| `request` | `text` (8,000 characters max) | no | What to do; otherwise the session waits for the request |

## `spec`

Understands a request, writes its spec in the wiki and slices it into Beads tickets, without implementing it (`risk: plan`). The `conductor` relaunches the `planner` at each step; the planner stays read-only, except its spec: `agents.planner.writes: ["docs/wiki/specs/**"]` ([`writes`](workflow-schema.en.md#writes-documentation-paths)).

| Input | Type | Required | Purpose |
|---|---|---|---|
| `request` | `text` (8,000 characters max) | yes | The feature to specify |

| Checkpoint | manuel | semi-auto | auto |
|---|---|---|---|
| `cp-understanding` Approve the understanding (restatement, vocabulary, answers to the questions) | pause | pause | auto |
| `cp-converge` Spec and breakdown convergence (checked by oh) | auto | auto | auto |
| `cp-spec` Approve the spec (mandatory, checked by oh, unlocks `commit`) | pause | pause | pause |
| `cp-tickets` Approve the breakdown (mandatory, checked by oh, unlocks `create`) | pause | pause | pause |

Steps:

1. **Interview**: the planner explores the code and loads `grilling`; its blocking questions (one per decision) are asked by the `conductor`. It returns its restatement and the vocabulary (each word of the request against the names of the code), shown at `cp-understanding`.
2. **Spec**: the planner writes `docs/wiki/specs/<slug>.md`: goal, scope, **glossary**, **acceptance criteria** (`CA-n`, each with its test point on its line: `· Test : TestPercent_ZeroTotal`), decisions and a **breakdown** into vertical slices (`Couvre : CA-…`). The format is in the planner's prompt (section “Spec écrite”), not in a skill to load.
3. **Convergence pass** (`cp-converge`, no pause): oh reads the spec and checks the glossary, the test point of each criterion, that every criterion is covered by a slice and that no slice is outside the spec. A gap refuses the checkpoint with the list of problems: the planner fixes it (or asks a question when the gap needs a decision), then the checkpoint is asked again.
4. **`cp-spec`**: the user approves the spec (same check); the planner commits it alone: `git add docs/wiki/specs/<slug>.md`, then `git commit -m "docs(spec): <slug>" -- docs/wiki/specs/<slug>.md`.
5. **`cp-tickets`**: the user approves the breakdown; the planner creates one ticket per slice, whose description cites the spec and its criteria. A spec or breakdown changed at a checkpoint goes again through `cp-converge`, `cp-spec` (new commit) and `cp-tickets`.

Exploration: when the request compares designs or asks for an analysis of the code before specifying, the `conductor` first launches the `pathfinder` (skill `codebase-design`) and hands its report to the planner; otherwise the interview starts right away.

Pause budget (CP-R2) in semi-auto: 3 pauses (1 decision, 2 irreversible actions), for a budget of 4. Default mode: `semi-auto`. Allowed Beads commands: `show`, `list`, `ready`, `children`, `search`, `query`, `count`, `lint`, `find-duplicates`, `label`, `dep`, `create`, `update`, `comments`, `duplicate`, `supersede`.

Outputs **recorded by oh** (no agent declares them): `spec` (the spec checked at `cp-converge` and `cp-spec`) and `tickets` (the tickets created after `cp-tickets`). Proposed next step (`next:`) when tickets were created: **`implement-spec`** on these tickets and this spec; oh shows it when the client closes (`oh run`), in `oh session next <id>` and in “Chain with…” of the TUI, without launching it.

**`cadrage` is an alias of `spec`** (R5): `oh run cadrage` launches `spec` (`aliases: [cadrage]`). The former `cadrage` (systematic exploration, `cp-scope`, `cp-recap`, no file written) no longer exists; a team or project layer defining a `cadrage` workflow wins over the alias.

## `implement-spec`

Implements the tickets of a spec one after the other, in a single session (`risk: write`), then has the whole reviewed. It is the implementation part of `feature`, without planning: the `orchestrator-dev` reads the tickets itself (`bd show`) and sets the order (dependencies, then priority). **Sequential version**: child sessions in worktrees, parallelism and the merge by oh come with R6.

| Input | Type | Required | Role |
|---|---|---|---|
| `tickets` | `beads-ids` | yes | Tickets to implement; by default `session.output(spec.tickets)`: those of the `spec` session the launch is chained from (`--parent`), otherwise of the last `spec` session of the project |
| `spec` | `path` | no | Spec of the tickets; by default `session.output(spec.spec)` |
| `method` | `enum`: `standard`, `tdd` | no (`standard`) | TDD method: `cp-seams` and `cp-red` for each ticket |
| `instructions` | `text` (4,000 characters max) | no | Details for the session |

| Checkpoint | manual | semi-auto | auto |
|---|---|---|---|
| `cp-plan` Approve the plan (mandatory): order of the tickets, planned agent, tickets left out | pause | pause | pause |
| `cp-1` Start the ticket | pause | auto | auto |
| `cp-seams`, `cp-red` (`tdd` method) | as in `ticket` | | |
| `cp-2` Commit or fix (mandatory, checked by oh, unlocks `commit` and `close`) | pause | pause | pause |
| `cp-3` Next ticket | pause | auto | auto |
| `cp-final` Review of the whole and recap | pause | pause | auto |

No delegation before `cp-plan`. Each ticket goes through its `cp-2` (commit, then close, before the next one). Once every ticket is processed, the `reviewer` makes an adversarial review of all the commits of the session; `cp-final` presents its verdict, the tickets and their commits, the open points: the user decides on a fix round (one only, through `cp-2` again) or to stop.

Pause budget (CP-R2) in semi-auto: `cp-plan`, `cp-2`, `cp-final`, i.e. 3 pauses for a budget of 3 (2 decisions, 1 irreversible action); the `tdd` method adds one (`cp-seams`). A feature of N tickets thus costs N + 2 pauses (instead of N + 3 in `feature`). Default mode: `semi-auto`. Runs: locally. Allowed Beads commands: `show`, `list`, `ready`, `children`, `dep`, `update`, `close`, `comments`, `label`. Outputs: `branch` (declared), `tickets` (closed tickets, recorded by oh).

## `onboarding`

Discovers the project and creates or enriches the `docs/wiki/` wiki (`doc-wiki-protocol`).

| Input | Type | Required | Purpose |
|---|---|---|---|
| `refresh` | `bool` (default: no) | no | Rediscover the project and propose the updates of the existing wiki (validated one by one on the card of `cp-wiki`), without deleting anything |
| `focus` | `text` (4,000 characters max) | no | Modules or topics to dig into |

The onboarder only writes `docs/wiki/`, `ONBOARDING.md` and `CONVENTIONS.md`: it is a permission (any other write is refused, `.gitignore` and `.git/` included). Without a wiki, it creates the pages; with a wiki (`docs/wiki/index.md`), it changes none and proposes each update with `wiki_propose` (nothing when the wiki is faithful to the code). It then passes `cp-wiki` (mandatory, unlocks the commit, `remote: defer`): the card shows its proposals, the user accepts, edits or refuses each one, oh applies the accepted ones; then the onboarder commits `docs/wiki/` alone (`writes: ["docs/wiki/**"]`). `ONBOARDING.md`, `CONVENTIONS.md` and the push are left to the user. One pause in semi-auto (CP-R2 budget: one irreversible action). Output: `wiki`. Allowed Beads commands (read only): `show`, `list`, `search`, `query`.

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

## `release`

Prepares a release: SemVer version deduced from the commits since the last tag and from the CHANGELOG (or forced), `[Unreleased]` section dated today, release notes, release commit, annotated tag and, on request, the description of the release merge request. Risk `publish`, local only. Guide: [Preparing a release](../guides/release.en.md).

| Input | Type | Required | Role |
|---|---|---|---|
| `version` | `string` (100 characters max) | no | Version to release; empty: deduced from the changes |
| `notes` | `text` (4,000 characters max) | no | Points to highlight, items to leave out |
| `mr` | `bool` (default: no) | no | Prepare the description of a release MR/PR, and open it when the branch is already on the remote |
| `target` | `string` (default: `main`) | no | Target branch of the MR |

| Precondition | Test | Failure |
|---|---|---|
| `clean-tree` | `git_clean` | launch refused, with the changes found |
| `release-branch` | `branch: [main, master, "release/*"]` | launch refused, with the current branch |
| `changelog` | `path_exists: [CHANGELOG.md]` | warning (the CHANGELOG will be created) |

| Checkpoint | manuel | semi-auto | remote |
|---|---|---|---|
| `cp-release` Release (mandatory, unlocks `commit`, `tag`, `mr`) | pause | pause | forbidden (`remote: forbid`) |

The `release-manager` prepares everything **without writing anything** and presents it at `cp-release`: version and reason, CHANGELOG section, tag message, notes, MR description. After the approval: CHANGELOG (and version files, each one asked), release commit, then annotated tag — oh refuses the tag until the release is committed —, then the MR when it is asked and the branch exists on the remote. **No push**: the report gives the commands (`git push origin <branch>`, then `git push origin <tag>`) and how to undo before the push. One pause in semi-auto. On-demand skills: `release-management`, `pr-description`.

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

Diagnosis of a bug or isolated problem by the `debugger`: diagnostic report (urgent actions first) and fix ticket. The `developer` is available on demand in the session; wiki enrichments go through `wiki_propose`.

| Input | Type | Required | Purpose |
|---|---|---|---|
| `issue` | `text` (8,000 characters max) | no | The observed problem; empty: the session asks for it |

The fix ticket is only created after `cp-ticket` (« Create the fix ticket », paused in every mode, unlocks `create`): the `debugger` presents the diagnosis and the ticket there, instead of asking for confirmation with the `question` tool (which it keeps in a `libre` session). A fix made in the session ends with `cp-commit` (mandatory, paused in every mode, unlocks `commit` and `close`). Output: `tickets`. Allowed Beads commands: `show`, `list`, `ready`, `search`, `query`, `lint`, `children`, `comments`, `count`, `status`, `graph`, `history`, `create`, `update`, `close`, `dep`, `label`.

**Session scratch folders** (`debugger: { scratch: { readers: [developer] } }`, see [`scratch`](workflow-schema.en.md#scratch-session-scratch-folders)): in the reproduction phase, the `debugger` writes its reproduction test or script in `~/.oh/sessions/<session>/scratch/debugger/` (path given in the `oh.scratch` entry of the session state), outside the repository and deleted with the session; it describes this test in text in the fix ticket. The `developer`, declared reader, may read this scratch file in the same session, without changing it, and makes it the final test of the project. No agent of another session has access to it.

## `incident`

Handles an ongoing incident: **mitigate first**, then write the blameless postmortem in the wiki and create the actions as tickets. The `debugger` leads the session; it stays read-only, except for its postmortem (`agents.debugger.writes: ["docs/wiki/postmortems/**"]`); the code of a mitigation is left to the `developer`, which it launches. Guide: [Incident, security fix and spike](../guides/incident-security-spike.en.md).

| Input | Type | Required | Role |
|---|---|---|---|
| `incident` | `text` (16,000 characters max) | yes | What is happening: symptoms, since when, impact, alerts, logs, last deployment |

| Checkpoint | manuel | semi-auto | auto |
|---|---|---|---|
| `cp-mitigated` Mitigation (mandatory, checked by oh: stack tests) | pause | pause | pause |
| `cp-postmortem` Postmortem (mandatory, unlocks `commit`) | pause | pause | pause |
| `cp-actions` Actions (mandatory, unlocks `create`) | pause | pause | pause |

Flow:

1. **Quick diagnosis**: impact, start time, triggering change (`git log` around the start time, then the diff of the suspect commit).
2. **Mitigation**: the safest, preferably reversible measure (undo the triggering change, switch the feature off, configuration). A code measure is applied by the `developer` **in the work tree**, without a commit; a measure outside the repository (deployment, environment variable) is given as is to the user. At `cp-mitigated`, oh runs the stack tests, then the user chooses or confirms the measure.
3. **Postmortem**: `docs/wiki/postmortems/<YYYY-MM-DD>-<slug>.md` (wiki frontmatter; summary, timeline, root cause, mitigation, actions), **blameless**: no person's name, not even the one the report mentions. On-demand skill: `postmortem` (sourced timeline, impact and detection delay in figures, trigger and cause, verifiable actions).
4. **`cp-postmortem`**: once approved, the mitigation fix (if any) is committed by the `developer`, then the postmortem alone by the debugger (`git commit -m "docs(postmortem): <slug>" -- <file>`).
5. **`cp-actions`**: one ticket per approved action (`bd create`, a description that cites the postmortem); a refusal: no ticket.

Why the mitigation fix is only committed at `cp-postmortem`: an operation is unlocked by a single checkpoint (`unlock_duplicate`); `cp-mitigated` stays the decision on the mitigation, checked by oh, and the fix is in place in the work tree from then on (the user may ship it themselves if they cannot wait). Pause budget (CP-R2) in semi-auto: 3 pauses (1 decision, 2 irreversible actions), for a budget of 4. Allowed Beads commands: `show`, `list`, `ready`, `search`, `query`, `lint`, `children`, `comments`, `count`, `status`, `graph`, `history`, `create`, `update`, `dep`, `label`.

## `security-fix`

Fixes the findings of a security audit: triage, fix with a regression test, targeted re-analysis **before** the commit. The `conductor` chains the `developer` and the `auditor-subagent`.

| Input | Type | Required | Role |
|---|---|---|---|
| `findings` | `text` (30,000 characters max) | yes | The findings: audit report or excerpt, one finding per entry with its severity and location |

| Checkpoint | manuel | semi-auto | auto |
|---|---|---|---|
| `cp-triage` Findings to fix | pause | pause | auto |
| `cp-rescan` Re-analysis (checked by oh: stack tests, at least one test file changed) | pause | auto | auto |
| `cp-2` Commit or fix (mandatory, same check, unlocks `commit`) | pause | pause | pause |

Flow:

1. **`cp-triage`**: the findings are presented as received, numbered `F1`, `F2`…; the answer says which ones to fix.
2. **Fixes** (`developer`, domain `security`): only the kept findings, at the cause and for every affected input, each with a regression test; no commit. A false positive is reported, with no change.
3. **Targeted re-analysis** (`auditor-subagent`): for each finding, `corrigé` (fixed), `encore présent` (still present) or `contournable` (bypassable: another path, a neighbouring prefix, a symbolic link, another encoding, another caller).
4. **`cp-rescan`**: no pause in semi-auto; oh runs the tests and refuses a change without a test. A finding still present goes back to the developer (2 rounds at most, `limits.max_fix_rounds`).
5. **`cp-2`**: fixes, tests and verdict of the re-analysis; once approved, the developer commits (`fix(security): …`).

Why the re-analysis comes before the commit: a fix the auditor finds bypassable must not be committed; `cp-2` decides with the tests (oh) and the verdict at hand. Pause budget in semi-auto: 2 pauses (1 decision, 1 irreversible action), for a budget of 3. Allowed Beads commands (read only): `show`, `list`, `ready`, `search`, `query`, `children`, `comments`, `count`, `status`, `graph`, `history`.

**From an audit**: the findings are given as input, for instance the audit report (`oh session report <session> <rp-…>`): `oh run security-fix -i findings="$(oh session report <session> rp-2)"`. Chaining `audit → security-fix` directly (`findings` output of the audit, `next:`) will come with workflow composition (R5).

## `spike`

Settles a technical question within a time box: sourced research, a throwaway prototype when needed, a **go**, **clarify** or **kill** decision written in the wiki. The `pathfinder` leads the session; it stays read-only, except for its decision (`agents.pathfinder.writes: ["docs/wiki/decisions/**"]`); the `developer` only writes the throwaway prototype.

| Input | Type | Required | Role |
|---|---|---|---|
| `question` | `text` (8,000 characters max) | yes | The question, the decision criteria (what would mean go or kill) and the time allowed |

| Checkpoint | manuel | semi-auto | auto |
|---|---|---|---|
| `cp-question` Question and criteria | pause | pause | auto |
| `cp-decision` Spike decision (mandatory, checked by oh: `.oh-spike/` deleted, unlocks `commit`) | pause | pause | pause |

Flow:

1. **`cp-question`**: closed question, verifiable criteria, time box. No research nor prototype before it (the `developer` is locked until `cp-question`, `after:`).
2. **Sourced research**, criterion by criterion: the repository first, then the official documentation (skill `research`); each fact cites `file:line` or its URL.
3. **Throwaway prototype**, only when a criterion can only be settled by running code: the `developer` writes the smallest program that answers it, in `.oh-spike/<slug>/` only, with no dependency added, runs it, records its output, then deletes `.oh-spike/`. Never committed.
4. **Decision**: `docs/wiki/decisions/<slug>.md` (`**Décision : go | clarify | kill**`, findings per criterion with their sources, prototype, next steps). `go`: every criterion holds; `kill`: one does not; `clarify`: one cannot be settled without a decision or information from the user.
5. **`cp-decision`**: oh checks that `.oh-spike/` no longer exists (`verify.cmd: ["test ! -e .oh-spike"]`); once approved, the decision alone is committed.

Pause budget in semi-auto: 2 pauses (1 decision, 1 irreversible action), for a budget of 3. Allowed Beads commands (read only): `show`, `list`, `search`, `query`, `count`.

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
