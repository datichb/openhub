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

A workflow may have other names, its `aliases:` (see [`aliases`](workflow-schema.en.md#aliases--other-names-of-the-workflow)), recognised by `oh run` and by the TUI palette alike: `cadrage` and `feature` (`spec`), `feature` with `--tickets` (`implement-spec`), `secu` (`security-fix`), `perf` (`benchmark`), `dbg` (`debug`), `feedback` and `retours` (`review-feedback`). The other terms of the former palette table (`dev`, `start`, `fast`, `rev`…) are no longer recognised.

---

## Overview

| Workflow | Entry agent | Agents | Risk | Runtime | Replaces |
|---|---|---|---|---|---|
| `ticket` | `orchestrator-dev` | `developer`, `reviewer`, `documentarian` (on demand) | write | local, container, remote | `oh start --dev`, `-t` |
| `quick` | `developer` | — | write | local, container | quick session |
| `infra-change` | `operator` | — | write (infrastructure, no file) | local | former `infra` agent (application part) |
| `spec` (aliases `cadrage`, `feature`) | `conductor` | `pathfinder` (when the request compares designs), `planner` (writes `docs/wiki/specs/`), `designer` | plan (+ `writes:`) | local | `cadrage` (former mode A without implementation), with a written and checked spec |
| `implement-spec` (alias `feature` with `--tickets`) | `orchestrator-dev` | `developer`, `reviewer`, `documentarian` (on demand); as a fan-out: `reviewer` and one `ticket` child session per ticket | write | local, container | implementation part of the former `feature` (mode B: existing tickets) |
| `merge-conflicts` | `merger` | — | write (concludes a merge in progress; never a push) | local, container | — (launched by oh in a fan-out) |
| `onboarding` | `onboarder` | — | write (`docs/wiki/`) | local, remote | `oh start --onboard` (former mode C) |
| `doc-sync` | `documentarian` | — | write (`docs/wiki/`) | local | — |
| `skill-refresh` | `documentarian` | — | write (stack skills of the hub) | local | — |
| `release` | `release-manager` | — | publish (CHANGELOG, release commit and tag; never a push) | local | — |
| `review` | `reviewer` | — | read | local, remote | `oh review` |
| `review-feedback` (aliases `feedback`, `retours`) | `orchestrator-dev` | `developer`, `reviewer` | write | local | `oh review feedback` |
| `audit` | `auditor` | `auditor-subagent` | read | local, remote | `oh audit`, former `infra` and `database` agents (analysis part) |
| `debug` (alias `dbg`) | `debugger` | `developer` (on demand) | write | local | `oh debug` (former mode D) |
| `incident` | `debugger` (writes `docs/wiki/postmortems/`) | `developer` | write | local | — |
| `security-fix` (alias `secu`) | `conductor` | `developer`, `auditor-subagent` | write | local | — |
| `spike` | `pathfinder` (writes `docs/wiki/decisions/`) | `developer` (throwaway prototype) | write | local | — |
| `retro` | `debugger` (writes `docs/wiki/retros/`) | — | write | local | — |
| `triage` | `planner` | — | plan | local | — |
| `benchmark` (alias `perf`) | `benchmarker` | — | write (`reports/benchmarks/`) | local | `oh run libre --agent benchmarker` |
| `sweep` | `conductor` | `pathfinder`, `developer`, `reviewer`; in a fan-out: `pathfinder`, `reviewer` and one `sweep-task` child session per subtask | write | local, container | `oh start --sweep` |
| `sweep-task` | `developer` | — | write | local, container | — (launched by oh in a fan-out sweep) |
| `brief-enrich` | `brief-enricher` | — | read | local | `oh takeover-brief enrich` |
| `libre` | your choice (`orchestrator` by default, its only workflow) | those the chosen agent may call | write | local, container | `oh start --agent`, TUI free session |

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

`release` refuses to start on a dirty tree or outside `main`, `master` and `release/*`. `spec` (and its aliases `cadrage` and `feature`) and `implement-spec` suggest `onboarding` when the project has no wiki, no `ONBOARDING.md` and no `CONVENTIONS.md`.

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

`cp-2` unlocks `commit` and `close`: until it is approved, oh refuses `git commit` (and `revert`, `cherry-pick`, `merge`, `pull`, which commit too) and closing the ticket to every agent. Pushing is left to the user (no shipped workflow unlocks it). Before `cp-2` passes, oh itself runs the build, tests and type check of the detected stack (`verify: { from: stack, expect: pass }`): the decision card shows the result, a failure opens nothing and the user may override it (traced choice). Same verification on `cp-2` of `implement-spec` and `review-feedback`, and on `cp-commit` of `quick`, `debug`, `sweep` and `sweep-task`; see [Checkpoints](checkpoints.en.md). Once `cp-2` is approved, `orchestrator-dev` hands back to the `developer`, who commits and then closes the ticket; oh refuses closing while the work is not committed. The `reviewer` runs the project's build and tests (and lint, type check) before its review, with the commands of the detected stack, allowed here: a failure is a 🔴 finding (verdict `corriger`); files produced by the build and not ignored are reported, since closing requires a clean tree. The same applies in `implement-spec`, `sweep`, `review-feedback` and `libre`. A question imitating a checkpoint ("CP-2 — commit?" asked with the `question` tool) is closed by oh, which reminds the agent to call `workflow_checkpoint`. When the ticket of the session is closed (or the last checkpoint passed) and the step ends, oh declares the missing outputs (`branch`, `tickets`) and the session becomes **completed**: "Chain with…" is offered.

## `feature` (alias of `spec` → `implement-spec`)

`feature` is no longer a workflow (`workflows/feature.yaml` is removed): it is the chain [`spec`](#spec) then [`implement-spec`](#implement-spec), routed by the inputs (see [`aliases`](workflow-schema.en.md#aliases--other-names-of-the-workflow)):

| Launch | Workflow launched | Former mode |
|---|---|---|
| `oh run feature -m "…"` (and `oh start -m "…"`); without a request, the session asks for it | `spec`: `aliases: [cadrage, feature]` | A (feature in natural language) |
| `oh run feature --tickets bd-1,bd-2` | `implement-spec`: `aliases: [{ id: feature, with: tickets }]`; guidance: `-i instructions=…` (`-m` is refused there: no `request` input) | B (existing tickets) |

oh shows the redirection (`feature → spec`). When `spec` ends, oh proposes `implement-spec` on the created tickets and the written spec, without launching it (`oh session next <id>`, when the client closes, "Chain with…" in the TUI).

**TDD method passed on.** `spec` takes the `method` input (`standard` or `tdd`) without using it itself: oh records it at launch (output `method`, `record: input`) and passes it to `implement-spec` in the proposed next step (`-i method=tdd`). `oh run feature -m "…" -i method=tdd` thus gives a spec, then a TDD implementation.

What changes compared with the former `feature` (⚠):

- without a request (`oh run feature`, `oh start`), the session asks for it first, as the former `feature` did: the `conductor` asks with `question`;
- the spec is written and checked in the wiki (`docs/wiki/specs/`), the tickets created at `cp-tickets`; the implementation is a **second session**, proposed by oh;
- checkpoints: those of `spec` (`cp-understanding`, `cp-converge`, `cp-spec`, `cp-tickets`), then those of `implement-spec` (`cp-plan` replaces `cp-0`, `cp-final` replaces `cp-feature` and its review of the whole); `cp-spec` (conditional UX/UI spec) goes away: the designer is called by the planner of `spec`;
- the `orchestrator` is now the entry agent of [`libre`](#libre) only;
- the Beads commands, the verification of `cp-2` and the commit lock are those of `implement-spec`.

**Team and project layers.** A document extending `hub:feature` (or `hub:cadrage`) no longer resolves: oh refuses it with an explicit error, `extends_alias` ("hub:feature is no longer a workflow: it is an alias of implement-spec, spec…"), at launch as with `oh workflow validate`, and `oh doctor` lists the published workflows concerned ("Retired workflows"). There is no automatic migration: the patches of `feature` target agents and checkpoints (`orchestrator`, `cp-0`, `cp-spec`, `cp-feature`) that exist neither in `spec` nor in `implement-spec`, and redirecting them silently could lose a rule of the team (`enforce`, a checkpoint made mandatory). To migrate, either:

- **delete** the document: `oh run feature` goes back to the alias;
- **rewrite** it as a layer of `spec` or `implement-spec` (`extends: hub:spec`, `extends: hub:implement-spec`), carrying the changes over to their agents and checkpoints; a document that keeps the id `feature` shadows the alias (warning `alias_shadowed`).

A complete `feature` document, without `extends`, remains a team workflow in its own right (it shadows the alias too). The migration of the former configurations (v38) now writes a patch of `implement-spec` (`cp-0` becomes `cp-plan`; the agents `implement-spec` does not have are listed in a comment, not translated).

## `quick`

Direct development session with the `developer` agent, without planning. A single checkpoint, `cp-commit` (mandatory, paused in every mode): it unlocks `commit` and `close`; before it, oh refuses every `git commit` and the closing of a ticket. Pushing is left to the user. The developer infers the domain from the request and loads the matching standards. It suggests `feature` when the request turns out to be large. Allowed Beads commands: `show`, `list`, `ready`, `search`, `query`, `children`, `comments`, `count`, `status`, `graph`, `history`, `create`, `update`, `close`, `dep`, `label`.

| Input | Type | Required | Purpose |
|---|---|---|---|
| `request` | `text` (8,000 characters max) | no | What to do; otherwise the session waits for the request |

## `spec`

Understands a request, writes its spec in the wiki and slices it into Beads tickets, without implementing it (`risk: plan`). The `conductor` relaunches the `planner` at each step; the planner stays read-only, except its spec and its line in the wiki index: `agents.planner.writes: ["docs/wiki/specs/**", "docs/wiki/index.md"]` ([`writes`](workflow-schema.en.md#writes-documentation-paths)).

| Input | Type | Required | Purpose |
|---|---|---|---|
| `request` | `text` (8,000 characters max) | no | The feature to specify, or a Beads ticket, a GitLab issue (`#12`, URL) or merge request (`!7`, URL) that the `planner` reads; without it (nor `source`), the `conductor` asks the user with `question` before any step |
| `source` | `path` | no | Wiki page the request comes from (the decision of a `spike`, the postmortem of an `incident`), given by their `next:`; the planner reads it at the interview |
| `method` | `enum` (`standard`, `tdd`) | no (`standard`) | Method of the implementation proposed next: `spec` does not use it, oh records it and passes it to `implement-spec` |

| Checkpoint | manuel | semi-auto | auto |
|---|---|---|---|
| `cp-understanding` Approve the understanding (restatement, vocabulary, answers to the questions) | pause | pause | auto |
| `cp-converge` Spec and breakdown convergence (checked by oh) | auto | auto | auto |
| `cp-spec` Approve the spec (mandatory, checked by oh, unlocks `commit`) | pause | pause | pause |
| `cp-tickets` Approve the breakdown (mandatory, checked by oh, unlocks `create`) | pause | pause | pause |

Steps:

1. **Interview**: the planner explores the code and loads `grilling`; its blocking questions (one per decision) are asked by the `conductor`. It returns its restatement and the vocabulary (each word of the request against the names of the code), shown at `cp-understanding`.
2. **Spec**: once `cp-understanding` is passed (before, oh refuses the write of the spec and tells the planner to wait for that checkpoint), the planner writes `docs/wiki/specs/<slug>.md`: goal, scope, **glossary**, **acceptance criteria** (`CA-n`, each with its test point on its line: `· Test : TestPercent_ZeroTotal`), decisions and a **breakdown** into vertical slices (`Couvre : CA-…`). The format is in the planner's prompt (section “Spec écrite”), not in a skill to load.
3. **Convergence pass** (`cp-converge`, no pause): oh reads the spec and checks the glossary, the test point of each criterion, that every criterion is covered by a slice and that no slice is outside the spec. A gap refuses the checkpoint with the list of problems: the planner fixes it (or asks a question when the gap needs a decision), then the checkpoint is asked again.
4. **`cp-spec`**: the user approves the spec (same check); the planner commits it with its line in `docs/wiki/index.md` (when it exists): `git add docs/wiki/specs/<slug>.md docs/wiki/index.md`, then `git commit -m "docs(spec): <slug>" -- docs/wiki/specs/<slug>.md docs/wiki/index.md`.
5. **`cp-tickets`**: the user approves the breakdown; the planner creates one ticket per slice, whose description cites the spec and its criteria. A spec or breakdown changed at a checkpoint goes again through `cp-converge`, `cp-spec` (new commit) and `cp-tickets`.

Order enforced by oh: `cp-converge` is refused before `cp-understanding`, `cp-spec` before `cp-converge`, `cp-tickets` before `cp-spec` (`after:`); the refusal names the checkpoint to pass first.

Exploration: when the request compares designs or asks for an analysis of the code before specifying, the `conductor` first launches the `pathfinder` (skill `codebase-design`) and hands its report to the planner; otherwise the interview starts right away.

Pause budget (CP-R2) in semi-auto: 3 pauses (1 decision, 2 irreversible actions), for a budget of 4. Default mode: `semi-auto`. Allowed Beads commands: `show`, `list`, `ready`, `children`, `search`, `query`, `count`, `lint`, `find-duplicates`, `label`, `dep`, `create`, `update`, `comments`, `duplicate`, `supersede`.

Outputs **recorded by oh** (no agent declares them): `spec` (the spec checked at `cp-converge` and `cp-spec`), `tickets` (the tickets created after `cp-tickets`) and `method` (the launch input, `record: input`). Proposed next step (`next:`) when tickets were created: **`implement-spec`** on these tickets, this spec and this method; oh shows it when the client closes (`oh run`), in `oh session next <id>` and in “Chain with…” of the TUI, without launching it.

**`cadrage` and `feature` are aliases of `spec`** (R5, feature-migration): `oh run cadrage` and `oh run feature` launch `spec` (`aliases: [cadrage, feature]`; `feature` with `--tickets` launches `implement-spec`, see [`feature`](#feature-alias-of-spec--implement-spec)). The former `cadrage` (systematic exploration, `cp-scope`, `cp-recap`, no file written) no longer exists; a team or project layer that defines a `cadrage` workflow wins over the alias, a layer extending `hub:cadrage` is refused (`extends_alias`).

## `implement-spec`

Implements the tickets of a spec one after the other, in a single session (`risk: write`), then has the whole reviewed. It is the implementation part of the former `feature` (alias `feature` with `--tickets`), without planning: the `orchestrator-dev` reads the tickets itself (`bd show`) and sets the order (dependencies, then priority). By default, **sequential** version in the session; with `fanout=true` (experimental), **child sessions** (see below).

| Input | Type | Required | Role |
|---|---|---|---|
| `tickets` | `beads-ids` | yes | Tickets to implement; by default `session.output(spec.tickets)`: those of the `spec` session the launch is chained from (`--parent`), otherwise of the last `spec` session of the project |
| `spec` | `path` | no | Spec of the tickets; by default `session.output(spec.spec)` |
| `method` | `enum`: `standard`, `tdd` | no (`standard`) | TDD method: `cp-seams` and `cp-red` for each ticket |
| `fanout` | `bool` (default: no) | no | Experimental (`[fanout] experimental = true`): one child session per ticket, each in its own worktree, merged by oh |
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

Pause budget (CP-R2) in semi-auto: `cp-plan`, `cp-2`, `cp-final`, i.e. 3 pauses for a budget of 3 (2 decisions, 1 irreversible action); the `tdd` method adds one (`cp-seams`). A feature of N tickets thus costs N + 2 pauses (instead of N + 3 in the former `feature`). Default mode: `semi-auto`. Runs: locally or in a container. Allowed Beads commands: `show`, `list`, `ready`, `children`, `dep`, `update`, `close`, `comments`, `label`. Outputs: `branch` (declared), `tickets` (closed tickets, recorded by oh).

### Fan-out (`fanout=true`, experimental)

`oh run implement-spec -i fanout=true` (with `[fanout] experimental = true`): the parent session keeps `cp-plan`, `cp-merge` and `cp-final`; `cp-1`, `cp-2`, `cp-3`, `cp-seams` and `cp-red` move to the children, and the developer is not in the bundle of the parent.

1. Once `cp-plan` is validated, oh creates the integration branch `oh/fanout/<session>/integration` (sibling worktree) from the branch of the session, then launches one `ticket` child session per ticket (its branch `oh/fanout/<session>/<ticket>` from the integration branch, its worktree), 1 to 3 at a time (`fanout.parallel` of the workflow and `[fanout] parallel`), a ticket after those it depends on, none for a ticket already closed; the children get the mode of the parent and the inputs declared by `fanout.inputs` (`method`, `instructions`).
2. Each child implements, has its work reviewed and passes its `cp-2` **without pause** (delegated to `cp-merge`, CP-R3), verified by oh in its worktree, then commits; it does not close its ticket, merge nor push (oh refuses it).
3. Once a child is done (session ended, or end of work seen by oh, its turn over with no decision waiting: commit checkpoint passed, a commit on its branch, nothing uncommitted; guard: 10 minutes idle without this signal), oh merges it into the integration branch (`--no-ff`). On a conflict, oh launches [`merge-conflicts`](#merge-conflicts) in the integration worktree (merger guard: 2 minutes idle without the merge concluded), then checks the result (merge committed, both sides in the history, no marker); otherwise it puts the integration branch back where it was and the child stays unmerged. Without a commit, with uncommitted changes, or stopped by the global budget: unmerged.
4. Once every child is done, oh writes to the parent session (table of the children); the `reviewer` reviews the whole integration branch, then `cp-merge` (mandatory, pauses in every mode): oh runs the verification **on the integration branch** there, never overridden; once validated, oh merges the integration branch into the branch of the session and closes the merged tickets itself (output `tickets`).
5. `cp-merge` refused with an instruction: **fix round** (`fanout.fix`); oh launches a [`quick`](#quick) session in the integration worktree (role `fixer`: no push, merge, closing nor branch move) with the instruction and what failed, its `cp-commit` validated by the user (never delegated); oh takes its commits when they are on top of the integration branch, without merge and with nothing uncommitted, else it puts the branch back where it was; the parent has the fix reviewed and passes `cp-merge` again, verified again on the integration branch. At most `limits.max_fix_rounds` rounds (3).
6. `cp-final`: recap; a new `implement-spec` for the unmerged tickets. oh then removes the `oh/fanout/…` worktrees and branches (after `cp-merge`, or when the parent is stopped; never a branch whose commits are on no other branch: `oh fanout clean --discard`).

Pause budget in semi-auto: `cp-plan`, `cp-merge`, `cp-final`, i.e. **3 pauses whatever the number of tickets** (2 more per fix round). Global budget (F-04): the parent and its children stay within `[fanout] budget_usd` (10 USD by default); beyond it, oh launches no more child and stops the running one (state `budget`), and each child gets what remains as its session budget. State: `oh fanout status <session>`; in `oh session list` and the TUI, a child is marked `↳` (with its ticket), the parent `⑂`. Guide: [Fan-out](../guides/fan-out.en.md).

**In a container** (parent session launched with `--runtime container`): the children, the merger and the fix round run in the runtime of the parent, each in a container of its workflow with its worktree mounted; the driver, the merges and the closing of the tickets stay on the machine; the verifications (children and `cp-merge` on the integration branch) run in a container of the project image, never on the machine instead. Remote session: fan-out refused (to come). Details: [Fan-out › In a container](../guides/fan-out.en.md#in-a-container).

## `merge-conflicts`

Finishes a merge stopped on conflicts (`risk: write`, `auto` mode only, no checkpoint): the `merger` loads `resolving-merge-conflicts`, keeps the intent of both sides, looks for conflicts without markers, verifies (build, tests), `git add` then `git commit --no-edit`. It does not start, abort nor push any merge, does not change branch and closes no ticket; when it cannot establish the intent of a side, it leaves the merge in progress and explains why. Inputs: `branch` (required, the merged branch), `into`, `files` (conflicted files). Launched by oh in a fan-out; usable by hand on a merge in progress (`oh run merge-conflicts -i branch=<branch>`).

## `onboarding`

Discovers the project and creates or enriches the `docs/wiki/` wiki (`doc-wiki-protocol`).

| Input | Type | Required | Purpose |
|---|---|---|---|
| `refresh` | `bool` (default: no) | no | Rediscover the project and propose the updates of the existing wiki (validated one by one on the card of `cp-wiki`), without deleting anything |
| `focus` | `text` (4,000 characters max) | no | Modules or topics to dig into |

The onboarder only writes `docs/wiki/`, `ONBOARDING.md` and `CONVENTIONS.md`: it is a permission (any other write is refused, `.gitignore` and `.git/` included). Without a wiki, it creates the pages; with a wiki (`docs/wiki/index.md`), it changes none and proposes each update with `wiki_propose` (nothing when the wiki is faithful to the code). It then passes `cp-wiki` (mandatory, unlocks the commit, `remote: defer`): the card shows its proposals, the user accepts, edits or refuses each one, oh applies the accepted ones; then the onboarder commits `docs/wiki/`, `ONBOARDING.md` and `CONVENTIONS.md` in one commit (`writes: ["docs/wiki/**", ONBOARDING.md, CONVENTIONS.md]`), nothing else. The push is left to the user. One pause in semi-auto (CP-R2 budget: one irreversible action). Output: `wiki`. Allowed Beads commands (read only): `show`, `list`, `search`, `query`.

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
| `branch` | `branch` | no | Branch to review (empty: recent changes of the current branch) |
| `base` | `branch` (default: `main`) | no | Base branch |

**Reviewing a branch other than the current one.** The diff (`git diff base...branch`) and the files read by `read`, `grep` and `glob` must come from the same branch. When `branch` names a branch that is not the one checked out in the project, oh creates a **temporary worktree** of that branch (detached HEAD: the branch is neither moved nor checked out, and the session cannot delete it) in `~/.oh/worktrees/<project>/<branch>-<id>`, and the session works there. The launch says so ("Review in a temporary worktree of <branch>", path included, in the recap and the warnings) and the agent is told in its prompt. Read-only comes from the workflow (`risk: read`): the agent can neither edit nor write, and its git commands only read. The worktree is removed when the session is **stopped** (`oh session stop`, `oh session delete`, or the final stop by the daemon); `oh gc` removes those whose session is gone (killed) as soon as no resumable session uses them. A sleeping or completed session keeps it: it resumes there. When the branch exists only on a remote (`origin/<branch>` already fetched, nothing is downloaded), that reference is read; a branch found neither locally nor on the remotes refuses the launch (`git fetch origin <branch>` first). Reviewed branch = current branch, or empty `branch`: unchanged (the session works in the project directory). With `--location <worktree>`, that worktree is read (oh warns when it is not on the reviewed branch). **Remote**: the job clones the current branch of the project, so reviewing another branch is refused (launch the review locally). The container uses the worktree like any other location.

The combined modes (`standard+adversarial`, `all`) launch parallel `reviewer` sessions: the workflow declares the self-delegation `reviewer: { calls: [reviewer] }`. A self-delegation is only kept when written in `calls` (never derived from permissions); it counts as one depth level. The tool refuses to launch a `primary` agent through `task`: the bundle therefore ships the `reviewer` with `mode: all` (see [Primary / Subagent modes](../architecture/agents.en.md#primary--subagent-modes)).

The reviewer runs the build and tests of the branch once, before the parallel sessions: in this `risk: read` workflow (often someone else's merge request, whose tests run its code), each command is asked to the user; refused or impossible to run, it is reported as a 🟡 "not verifiable" finding. Nothing is installed.

Publishing a merge request (`oh mr create`) is not a workflow: it remains an oh command.

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

Outputs **recorded by oh**: `findings` (the last report deposited, `record: report`) and `type` (the launch input, `record: input`). Proposed next step (`next:`) after a `security` audit that deposited its report: **`security-fix`**, which reads the findings in this session (`oh run security-fix --parent <session>`), without launching it.

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
| `cp-mitigated` Mitigation (mandatory, checked by oh: stack tests, unlocks `commit`: one commit) | pause | pause | pause |
| `cp-postmortem` Postmortem (mandatory, unlocks `commit` again, `unlock_again`) | pause | pause | pause |
| `cp-actions` Actions (mandatory, unlocks `create`) | pause | pause | pause |

Flow:

1. **Quick diagnosis**: impact, start time, triggering change (`git log` around the start time, then the diff of the suspect commit).
2. **Mitigation**: the safest, preferably reversible measure (undo the triggering change, switch the feature off, configuration). A code measure is applied by the `developer` **in the work tree**, without a commit; a measure outside the repository (deployment, environment variable) is given as is to the user. At `cp-mitigated`, oh runs the stack tests, then the user chooses or confirms the measure; once approved, the code fix (if any) is committed at once by the `developer`, on its own (`fix(incident): …`).
3. **Postmortem**: `docs/wiki/postmortems/<YYYY-MM-DD>-<slug>.md` (wiki frontmatter; summary, timeline, root cause, mitigation, actions), **blameless**: no person's name, not even the one the report mentions. On-demand skill: `postmortem` (sourced timeline, impact and detection delay in figures, trigger and cause, verifiable actions).
4. **`cp-postmortem`**: once approved, the postmortem is committed alone by the debugger (`git commit -m "docs(postmortem): <slug>" -- <file>`).
5. **`cp-actions`**: one ticket per approved action (`bd create`, a description that cites the postmortem); a refusal: no ticket.

Outputs **recorded by oh**: `postmortem` (the file written under `docs/wiki/postmortems/`, recorded when a checkpoint passes, `record: written`) and `actions` (the tickets created). Proposed next steps (`next:`), never launched: **`ticket`** on the action tickets, if any; **`spec`** with the postmortem as source page (`-i source=…`), for actions that need a spec first.

Two commit checkpoints (`unlock_again`, see the [schema](workflow-schema.en.md#checkpoints)): the window of `cp-mitigated` holds one commit, the fix's (oh does not open it when there is nothing to commit, closes it after that commit and at the latest when `cp-postmortem` is asked); the postmortem waits for `cp-postmortem`. The fix can thus be shipped as soon as the mitigation is approved. Pause budget (CP-R2) in semi-auto: 3 pauses (1 decision, 2 irreversible actions: the repeated commit does not count), for a budget of 4. Allowed Beads commands: `show`, `list`, `ready`, `search`, `query`, `lint`, `children`, `comments`, `count`, `status`, `graph`, `history`, `create`, `update`, `dep`, `label`.

## `security-fix`

Fixes the findings of a security audit: triage, fix with a regression test, targeted re-analysis **before** the commit. The `conductor` chains the `developer` and the `auditor-subagent`.

| Input | Type | Required | Role |
|---|---|---|---|
| `findings` | `text` (65,536 characters max) | no | The findings: audit report or excerpt, one finding per entry with its severity and location; by default, the report of the security audit the launch is chained from (`--parent`), else of the last security audit of the project (`from: session.output(audit.findings)`, `from_equals: { type: security }`); none: the session asks for them before any step (`question` tool) |

| Checkpoint | manuel | semi-auto | auto |
|---|---|---|---|
| `cp-triage` Findings to fix (unlocks `create`) | pause | pause | auto |
| `cp-rescan` Re-analysis (checked by oh: stack tests, at least one test file changed) | pause | auto | auto |
| `cp-2` Commit or fix (mandatory, same check, unlocks `commit`) | pause | pause | pause |

Flow:

1. **`cp-triage`**: the findings are presented as received, numbered `F1`, `F2`…; the answer says which ones to fix, the others are set aside with their reason (later, accepted risk, false positive…).
2. **Tickets of the findings set aside** (`developer`): one ticket per finding set aside (`bd create … -t bug -l security`), with the finding as received and the reason of the triage; before `cp-triage`, oh refuses the creation. oh records these tickets (output `deferred`, `record: created`).
3. **Fixes** (`developer`, domain `security`): only the kept findings, at the cause and for every affected input, each with a regression test; no commit. A false positive is reported, with no change. The developer returns a precise contract, in the `### Critères d'acceptance couverts` section of its report: one line per finding (`F<n>`, fixed or false positive, regression test, changed files).
4. **Targeted re-analysis** (`auditor-subagent`): it receives the developer's report (`rp-…`) and that contract; for each finding, `corrigé` (fixed), `encore présent` (still present) or `contournable` (bypassable: another path, a neighbouring prefix, a symbolic link, another encoding, another caller).
5. **`cp-rescan`**: no pause in semi-auto; oh runs the tests and refuses a change without a test. A finding still present goes back to the developer (2 rounds at most, `limits.max_fix_rounds`).
6. **`cp-2`**: fixes, tests and verdict of the re-analysis; once approved, the developer commits (`fix(security): …`).

Why the re-analysis comes before the commit: a fix the auditor finds bypassable must not be committed; `cp-2` decides with the tests (oh) and the verdict at hand. Pause budget in semi-auto: 2 pauses (2 irreversible actions: the ticket creation at `cp-triage`, the commit at `cp-2`), for a budget of 4. Allowed Beads commands: `show`, `list`, `ready`, `search`, `query`, `children`, `comments`, `count`, `status`, `graph`, `history`, `create`.

**From an audit**: a `security` audit proposes `security-fix` (`next:`); `oh run security-fix --parent <audit session>` reads the report of that audit (its content, not its id). Without `--parent` nor `-i findings=…`, oh takes the report of the last `security` audit of the project (never one of another type) and says so at launch; without a security audit, it says so and the session asks for the findings. An excerpt can always be given as input: `oh run security-fix -i findings="…"`.

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

Outputs **recorded by oh** when a checkpoint passes (`record: written`): `decision` (the file written under `docs/wiki/decisions/`) and `verdict` (`go`, `clarify` or `kill`, read on the `**Décision : …**` line, `match:`). Proposed next step (`next:`) after a `go` only: **`spec`**, with the decision as source page (`-i source=…`), without launching it.

Pause budget in semi-auto: 2 pauses (1 decision, 1 irreversible action), for a budget of 3. Allowed Beads commands (read only): `show`, `list`, `search`, `query`, `count`.

## `retro`

Retrospective of past sessions, from their trace: what slowed, cost or blocked the work, and verifiable actions. The `debugger` leads the session; it stays read-only, except for the page of the retrospective (`agents.debugger.writes: ["docs/wiki/retros/**"]`); it does not read the traces itself, oh gives them. Guide: [Retrospective and triage](../guides/retro-triage.en.md).

| Input | Type | Required | Role |
|---|---|---|---|
| `sessions` | `string` (400 characters max) | no | The sessions to look at: ids (`ses_…`), a period (`7d`, `24h`, `2w`, or a date `YYYY-MM-DD`: since that day) and workflows (`ticket`), separated by spaces or commas; empty: the session the launch is chained from (`--parent`), else the last finished session of the project |
| `trace` | `text` (20,000 characters max) | no | The trace summaries, **computed by oh** (`from: session.traces(sessions)`); given at launch only to replay a retrospective |
| `focus` | `text` (2,000 characters max) | no | What the user wants to understand |

Sessions picked with a period or workflows: the most recent first, finished, 6 at most, without the `retro` sessions themselves (unless named). Each summary (the one of [`oh session trace`](cli-sessions.en.md), shortened) gives the title, the state, the duration, the totals and the cost per agent, the checkpoints and those asked several times (retries), the refused decisions with the user's instruction, the refused calls, the skills, the delegations and the outputs; 16,000 characters at most for all of them.

| Checkpoint | manuel | semi-auto | auto |
|---|---|---|---|
| `cp-retro` Retrospective (mandatory, unlocks `commit`) | pause | pause | pause |
| `cp-actions` Actions (mandatory, after `cp-retro`, unlocks `create`) | pause | pause | pause |

Flow:

1. **Facts**: session by session, the facts of the summary, each with its source (`<session> · <section> · <line>`); nothing else: a fact missing from the summary does not exist.
2. **Findings**: the facts grouped by cause (a command refused in two sessions, the same instruction at two checkpoints), with their count, their cost (retries, duration, dollars) and their cause in the system (prompt, permission, workflow, convention, missing check), **blameless**; an unproven cause is a hypothesis. The repository is read only to shed light on a cause.
3. **Actions**: one per finding at most, each with the sign that will tell, in a later trace, that it is done.
4. **Page**: `docs/wiki/retros/<YYYY-MM-DD>-<slug>.md` (wiki frontmatter; sessions looked at, what went well, findings `C<n>`, actions `- [ ] A<n> … — vérification : …`).
5. **`cp-retro`**: once approved, the page alone is committed (`git commit -m "docs(retro): <slug>" -- <file>`).
6. **`cp-actions`**: one ticket per approved action; a refusal: no ticket.

With no session to look at, the debugger says so, gives the command to run again (`oh run retro -i sessions="…"`) and stops: no checkpoint, no write.

Why `writes:` and not `wiki_propose`: a retrospective is a new, dated page written in one go (like the postmortem of `incident` and the decision of `spike`); `wiki_propose` adds or replaces lines in an existing page, a proposal of 60 lines at most approved one by one on the commit card. With `writes:`, oh opens only `docs/wiki/retros/**` to the debugger, lets it commit that path only after `cp-retro`, and records the page (`record: written`).

Outputs **recorded by oh**: `retro` (the page, `record: written`) and `actions` (the created tickets). Proposed next step (`next:`), never launched: **`triage`** on the action tickets. Pause budget in semi-auto: 2 pauses (2 irreversible actions), for a budget of 4. Allowed Beads commands: `show`, `list`, `search`, `query`, `count`, `create`.

## `triage`

Triage of a batch of Beads tickets or findings: priorities, duplicates, labels, ticket too big to be done at once. The `planner` leads the session; it changes no file and launches no agent. **Nothing is written to Beads before `cp-triage`**, which shows every proposed change. Guide: [Retrospective and triage](../guides/retro-triage.en.md).

| Input | Type | Required | Role |
|---|---|---|---|
| `tickets` | `beads-ids` | no | The tickets to triage |
| `query` | `string` (400 characters max) | no | A `bd query` filter of the tickets to triage; without tickets or filter: the open tickets of the project, 30 at most |
| `findings` | `text` (16,000 characters max) | no | Findings (review, audit, retrospective…): each is attached to the ticket that already covers it (a comment) or becomes a ticket |

| Checkpoint | manuel | semi-auto | auto |
|---|---|---|---|
| `cp-triage` Triage changes (mandatory, unlocks `create` and `update`) | pause | pause | pause |

Flow:

1. **Reading**: `bd show`, `bd label list-all`, `bd find-duplicates --status open` (candidates), `bd search` for each finding.
2. **Triage**: a priority changed only when the ticket justifies it; a duplicate only when both tickets describe the same problem at the same place (the most complete one is kept), else a `related` link; the project's labels only; a ticket that mixes independent deliverables gets one child ticket per deliverable; neither status, assignee nor description change.
3. **`cp-triage`**: the table of changes (ticket, field, before → after, reason), the tickets to create, the unchanged ones. Before it passes, oh refuses **every write to Beads**: creating (`create`) and changing (`update`: `bd update`, `label add|remove`, `dep add|remove`, `comments add`, `duplicate`, `supersede`, `reopen`…), in the shell and in the Beads gateway. A refusal: nothing is written.
4. **Writes**: exactly the approved lines, one command per line.

Outputs **recorded by oh** from what the Beads gateway saw: `updated` (the changed tickets, `record: updated`), `created` (the created tickets) and `duplicates` (the duplicates closed, `record: closed`). Proposed next step: **`ticket`** on the created tickets. Pause budget in semi-auto: 1 pause (1 irreversible action), for a budget of 3. Allowed Beads commands: `show`, `list`, `ready`, `search`, `query`, `count`, `children`, `lint`, `find-duplicates`, `label`, `dep`, `comments`, `create`, `update`, `duplicate`, `supersede`.

Prerequisites from audit v5 (S-13, S-31, S-32, still open): they concern the MCP gateway and the Jira and GitHub MCP servers, which `triage` does not use: it only triages Beads tickets (Beads gateway, `create` and `update` locks) and findings given as input.

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

Reaches a cross-cutting goal by splitting it into independent subtasks: the `pathfinder` (read-only) splits, the `conductor` has the breakdown approved, **read and checked by oh**, then the subtasks are carried out, verified and reviewed. No Beads ticket. By default, **sequential** version in the session; with `fanout=true` (experimental), **one child session per subtask** (see below).

| Input | Type | Required | Purpose |
|---|---|---|---|
| `goal` | `text` (4,000 characters max) | yes | High-level goal |
| `strategy` | `enum`: `llm` (default), `manual`, `by-file`, `by-package` | no | Decomposition |
| `tasks` | `text` (8,000 characters max) | no | Tasks, one per line (`manual`) |
| `include`, `exclude` | `string` (1,000 characters max) | no | Glob patterns, comma-separated |
| `verify` | `enum`: `none` (default), `tests`, `lint`, `build`, `all`, `custom` | no | Verification asked on top of oh's |
| `verify_cmd` | `string` (500 characters max) | with `custom` | Verification command; `verify: custom` without it is refused at launch (`verify_cmd_missing`). Run by a `developer`: its permissions may refuse it |
| `dry_run` | `bool` (default: no) | no | Show the decomposition without running anything: `cp-plan` is not called, the session ends with `cp-recap` (no child session in a fan-out) |
| `fanout` | `bool` (default: no) | no | Experimental (`[fanout] experimental = true`): one `sweep-task` child session per approved subtask, each in its own worktree, merged by oh |

| Checkpoint | manuel | semi-auto | auto |
|---|---|---|---|
| `cp-plan` Approve the decomposition (mandatory, read and checked by oh, before any execution) | pause | pause | pause |
| `cp-commit` Commit the sweep (mandatory, verified by oh; outside a fan-out) | pause | pause | pause |
| `cp-merge` Approve the merges (mandatory, verified by oh on the integration branch; fan-out) | pause | pause | pause |
| `cp-recap` Sweep recap | pause | pause | auto |

**Breakdown read by oh** (`checkpoints.cp-plan.tasks`, [schema](workflow-schema.en.md#tasks--breakdown-read-by-oh)). The `conductor` gives the breakdown in the `tasks` argument of `workflow_checkpoint`: one entry `{ title, description, files, domain, parallel }` per subtask. oh checks it **before any card**: 1 to 12 subtasks; a one-line title; a domain among `frontend`, `backend`, `fullstack`, `api`, `data`, `mobile`, `devops`, `platform`, `security`, `refactor`, `migration`, `tests`; a scope whose every entry (path, folder or `*`, `**` pattern) names files of the repository, or a file to create in an existing folder (never absolute, outside the repository, an option or `.git`); two subtasks that may run at the same time (the parallel ones, or all of them in a fan-out) share no file. A refused breakdown makes no card: the call is refused with the list of problems, and the `conductor` fixes it. Accepted, the card shows the breakdown **as oh read it** (`t1`, `t2`… numbered by oh); the call that passes the checkpoint must bring the same one, otherwise it is refused. oh records it once `cp-plan` is passed: `tasks` output (`record: tasks`, one line per subtask) and, in a fan-out, the child sessions.

Sequential version: the developer agents and the `reviewer` only start after `cp-plan` (in parallel only for parallel subtasks with disjoint scopes); `cp-commit` unlocks `commit` (before it, oh refuses every `git commit`) and oh runs the stack verification there. Default mode: `semi-auto`; circuit breaker at 12 consecutive delegations. Allowed Beads commands: `show`, `list`, `search`, `query`. Outputs: `branch`, `tasks` (recorded by oh). Difference with the former `--sweep`: without a fan-out, subtasks run in the same session and location; `--sweep-branch-prefix` and `--max-sessions` have no equivalent.

### Fan-out sweep (`fanout=true`, experimental)

`oh run sweep -i goal="…" -i fanout=true` (with `[fanout] experimental = true`): `sweep` uses the fan-out mechanism of [`implement-spec`](#fan-out-fanouttrue-experimental), its children being the subtasks recorded at `cp-plan` (`fanout.source: tasks`) instead of tickets. The parent session keeps `cp-plan`, `cp-merge` and `cp-recap`; `cp-commit` moves to the children and the `developer` is not in the bundle of the parent (`agents.developer.fanout: never`).

1. Once `cp-plan` is approved, oh creates the integration branch `oh/fanout/<session>/integration`, then launches one [`sweep-task`](#sweep-task) session per subtask **recorded by oh** (never a list declared by an agent), on `oh/fanout/<session>/t<n>` in its own worktree, 1 to 3 at a time. oh gives it its subtask (`task` input: title and description), its scope (`files`) and its domain (`domain`), plus `goal`, `verify` and `verify_cmd` when the session got them (`fanout.inputs`), nothing else.
2. Each child does its subtask, runs the verification asked, passes its `cp-commit` **without a pause** (delegated to `cp-merge`, CP-R3), verified by oh in its worktree, then commits. oh refuses this commit when a change is outside the **scope of the subtask** (what oh recorded): the scopes being disjoint, a child never touches the work of another. It does not merge nor push (oh refuses it); no ticket to close.
3. Merge by oh into the integration branch, `merger` on a conflict, table of the subtasks to the parent, review of the whole by the `reviewer`, then **`cp-merge`** (pause in every mode): oh runs the stack verification there **on the integration branch**, never overridden; once approved, oh merges the integration branch into the branch of the session. Nothing is closed: there is no `tickets` output.
4. `cp-merge` refused with an instruction: fix round (`fanout.fix`, a `quick` session on the integration branch, its `cp-commit` approved by the user), one round (no `limits.max_fix_rounds`), then `cp-merge` verified again.
5. `cp-recap`: subtasks merged or not, and why. oh then cleans up the `oh/fanout/…` worktrees and branches as for `implement-spec`.

Pause budget in semi-auto: `cp-plan`, `cp-merge`, `cp-recap`, i.e. 3 pauses whatever the number of subtasks (as the sequential version; two more per fix round). `dry_run`: `cp-plan` is not called, no child session starts.

**Fan-out disabled** (`[fanout] experimental` set to `false`, the default): `sweep` keeps its sequential version, unchanged but for the breakdown read by oh; `-i fanout=true` is refused at launch, as for `implement-spec`. The fan-out is never chosen by default.

**Team layers that extend `hub:sweep`**: nothing to migrate, the ids (`cp-plan`, `cp-commit`, `cp-recap`, agents, inputs) do not change. A layer may lower `tasks.max`, narrow `tasks.domains` or `fanout.parallel`, never the reverse, nor remove `tasks:`, change `fanout.source` or keep the `developer` in a fan-out (`loosening`). A layer that changes `cp-commit` (stricter verification, pause) keeps it outside a fan-out, but a fan-out session has no such checkpoint: `fanout_never_patched` warning at validation, and `oh doctor` lists these layers ("Layers and fan-out"); to harden the fan-out too, change `cp-merge`.

## `sweep-task`

One subtask of a sweep, without a ticket (`risk: write`): the `developer` (entry agent) does it within its file scope, runs the verification asked, passes `cp-commit` (mandatory, verified by oh, unlocks `commit`) and commits. Launched by oh for each subtask of a fan-out `sweep`, where its `cp-commit` is delegated to the `cp-merge` of the parent and oh refuses a change outside the recorded scope; usable by hand (`oh run sweep-task -i task="…" -i files="…"`) with the same check: oh checks the scope at launch (each entry a path, a folder or a pattern of the repository, or a new file in an existing folder; otherwise the launch is refused with the list of problems), records it (output `files`) and refuses `cp-commit` on a change outside the scope (`inputs.files.scope`). Inputs: `task` and `files` (required, one path, folder or pattern per line), `domain` (same list as `sweep`), `goal`, `verify`, `verify_cmd`. Allowed Beads commands: `show`, `list`, `search`, `query`. Default mode: `semi-auto` (in a fan-out, the parent's).

## `brief-enrich`

Enriches a ticket takeover brief, without interaction (headless session: `oh run brief-enrich --headless`). The `brief` input is read from the team space of the project (`from: ticket.brief(ticket)`): `oh run brief-enrich --headless -i ticket=<id>` is enough. `oh takeover-brief enrich` also saves the result. No Beads command (`beads.allow: []`).

| Input | Type | Required | Purpose |
|---|---|---|---|
| `ticket` | `beads-id` | yes | Ticket of the brief |
| `brief` | `text` (40,000 characters max) | yes | Content of the existing brief |

## `libre`

Session with the agent of your choice, without checkpoints nor commit lock — on purpose: a free session is driven by the user, so `git commit` is allowed there and the agent asks for confirmation before any irreversible action: `oh run libre --agent debugger -m "…"` (without `--agent`: `orchestrator`, the entry agent of this workflow only since `feature` became an alias: it coordinates without reading or changing any file, routes to the `pathfinder` or the `planner` then the `orchestrator-dev`, and has the plan approved with `question`, for lack of a checkpoint). The entry of the workflow is **selectable** (`entry.selectable: true`): oh computes the members from the chosen agent and the agents it may call (`task` permission of the catalogue, transitively). The world stays closed: nothing else is visible. An agent never calls itself there (no parallel `reviewer` sessions: `oh run review` for the combined modes). In the TUI, the free session (`coder` command, or « Démarrer » when the catalogue is empty) opens the launch form with the default agent. Allowed Beads commands: `show`, `list`, `ready`, `search`, `query`, `children`, `comments`, `count`, `lint`, `find-duplicates`, `status`, `graph`, `history`, `create`, `update`, `close`, `reopen`, `edit`, `comment`, `dep`, `label`, `duplicate`, `supersede` (everything but `delete`).

| Input | Type | Required | Role |
|---|---|---|---|
| `request` | `text` (8,000 characters max) | no | What to do; otherwise the session waits for the request |

`entry.selectable` is an additive addition to the `oh/v1` schema: another workflow may declare it; `--agent` is refused on a workflow whose entry is not selectable.

---

## Known limits

Accepted as they are (p6-debts); they change neither the locks nor the checkpoints:

- **Guards on the text of the commands**: the shell rules (`unlocks:` locks, `writes:`, window of the first commit of an `unlock_again`, global guard) compare the raw text of each command, without normalising it: `git 'clean'` or two spaces do not match the pattern. They are guard rails against the mistakes of a model, not a barrier ([SECURITY](../../SECURITY.md#what-the-shell-rules-do-not-protect)). What does not depend on the text: the checkpoint validation, the Beads gateway, oh's verifications, and the `git add`/`git commit` lines of several `writes:` paths, read word by word by oh.
- **No custom refusal message** (opencode 2.0.20): a command refused by a rule gets the generic message of the tool. oh gives its reason only for what the oh plugin relays to it (a write held by `writes_after`, an operation locked while oh has not confirmed the session rules, a `writes:` line read by oh); a `question` cannot be refused with a message.
- **MR only when the branch is pushed**: `release` (`mr=true`) opens the merge request only when the branch is already on the remote; oh never pushes, the report gives the commands.
- **2 pauses per fix round** of the fan-out (`fanout.fix`): the `cp-commit` of the fix session, then `cp-merge` passed again.
- **Workflow editor of the TUI**: it offers `risk: publish` without a message of its own for this level (hub only, never remote, `tag` and `mr`, preconditions only hardened); the refusals show at validation.
- **Remote sessions**: the `wp-…` ids of the wiki proposals cited in the imported transcript are not renumbered (the reports are).

## Limits

```yaml
limits:
  budget_usd: 5                          # budget of each session (USD)
  models: ["eu.anthropic.claude-*"]      # allowed models (wildcard patterns)
```

- Both are optional and add to the hub, team and project restrictions (`oh budget show`, guide [Sessions v5](../guides/sessions-v5.en.md#restrictions)). The most specific value wins; a value enforced by the team is a ceiling.
- `budget_usd` is a soft cap on the cost the tool reports for the session and its subagents: as soon as it is overrun (a subagent spends, a model request), a `$` decision asks to raise the budget or stop, and the session's next requests are refused until it is answered.
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
- `{{ data "request" .request }}` writes an input, truncated to `max_length`. A value oh computes from an outside source (`from:`: merge request discussions, ticket brief) goes inside a data tag `<oh:data-<fingerprint> name="…">…</oh:data-<fingerprint>>`: the agent treats it as data, never as instructions. The tag name holds the first 16 hex characters of the SHA-256 of the content: a value cannot spell out its own closing tag, and the same content always gives the same tag. An `oh:data` tag inside the value is also neutralised, whatever its case. What the user typed at launch is their own request: it is written as it is, so that a short request ("Say only X") is carried out and not taken for an injection. **Every `string` or `text` input goes through `data`** (checked by the tests);
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
