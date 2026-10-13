> [Lire en français](checkpoints.fr.md)

# Reference — Checkpoints, modes and verification by oh

A checkpoint is a pause of the workflow: the agent calls `workflow_checkpoint`, oh applies the behavior of the mode (wait for the user or let it through) and opens the operations the checkpoint unlocks (`commit`, `close`, `create`, `apply`, `push`). The fields are described in [Workflow schema](workflow-schema.en.md#checkpoints); the three-level execution (prompt, MCP tool, oh plugin) in [ADR-042](../architecture/adr/042-checkpoints-headless-decisions.en.md).

Every pause costs human time and, in the long run, rubber-stamping: the number of pauses is a security setting. The rules below (modernisation report of October 2026, §9, rules CP-R1 to CP-R11) keep few pauses, and better informed ones.

---

## Modes

| Mode | What pauses |
|---|---|
| `manuel` | every checkpoint, except the `skip` ones |
| `semi-auto` | the human decisions (scope, plan) and the mandatory checkpoints; intermediate steps go on alone |
| `auto` | **everything but the mandatory decisions**: a commit, closing a ticket, creating tickets or changing infrastructure always ask the user |

So `auto` is not full autonomy: a mandatory checkpoint (or one that unlocks an operation) is never skipped. At launch, `oh run` and the TUI recap show the **planned pauses** for the chosen mode, for instance:

```
  Planned pauses 2 per run: cp-0 (mandatory) · cp-2 (mandatory) — auto = everything but the mandatory decisions
```

A conditional checkpoint is listed with "if its condition holds". Repeated checkpoints (one `cp-2` per ticket) count once per run.

### Pauses of the shipped workflows (semi-auto)

| Workflow | Pauses in semi-auto | Irreversible ones | Budget |
|---|---|---|---|
| `feature` | `cp-tickets`, `cp-0`, `cp-spec` (if its condition holds), `cp-2` | `cp-tickets`, `cp-2` | 4 |
| `feature`, `tdd` method | the same and `cp-seams` | `cp-tickets`, `cp-2` | 4 + 1 |
| `ticket` | `cp-2` | `cp-2` | 3 |
| `ticket`, `tdd` method | `cp-seams`, `cp-2` | `cp-2` | 3 + 1 |
| `cadrage` | `cp-scope`, `cp-tickets`, `cp-recap` | `cp-tickets` | 3 |
| `sweep` | `cp-plan`, `cp-commit`, `cp-recap` | `cp-commit` | 3 |
| `review-feedback` | `cp-fix`, `cp-2` | `cp-2` | 3 |
| `quick` | `cp-commit` | `cp-commit` | 3 |
| `debug` | `cp-ticket`, `cp-commit` | `cp-ticket`, `cp-commit` | 4 |
| `infra-change` (manual only) | `cp-plan`, `cp-apply` | `cp-apply` | 3 |
| `benchmark` | `cp-target` | — | 2 |

`cp-feature` (feature recap) is automatic in semi-auto: a recap protects nothing (CP-R1); it stays a pause in `manuel`.

---

## Pause budget (CP-R2)

In semi-auto (or in the default mode when semi-auto is not allowed), a workflow pauses at most for:

> **2 human decisions** (scope, understanding, arbitration) **+ 1 per checkpoint that unlocks an irreversible action** (`unlocks:`: commit, close, create, apply, push).

A `conditional` checkpoint, or one with `when:`, counts as a pause (the worst case). The budget is checked:

- on the workflows shipped by the hub, by the `TestCheckpointPauseBudget` test (blocking);
- on team and project workflows, by `oh workflow validate`: warning `pause_budget_exceeded` (the workflow stays usable).

A **method** chosen at launch (`method: tdd`, see [TDD method](#tdd-method-cp-seams-and-cp-red-r7)) adds at most **1 pause**, for its own sessions: the budget is checked for a session without method and for a session of each method (report §9.2: "+1 pause per TDD ticket").

To stay within budget: make the steps that protect nothing automatic, or have oh verify them (`verify:`) instead of pausing.

---

## Verification run by oh (`verify:`)

```yaml
checkpoints:
  cp-2:
    mandatory: true
    unlocks: [commit, close]
    verify: { from: stack, expect: pass }
```

Before a checkpoint passes, hence before its unlocks open, oh runs the verification itself:

1. **Commands**: `from: stack` takes those of the detected stack (`bricks.VerifyRuns`, derived from the patterns of `bricks.VerifyCommands`: a `make test|build|lint|typecheck` target first, else the `package.json` scripts with the package manager of the lockfile, `tsc --noEmit` for TypeScript, `go build ./...` and `go test ./...`, `cargo`, `pytest`, `mvn`, `./gradlew`). `kinds` picks the kinds (default `build`, `test`, `typecheck`; `lint` on request). `cmd` adds commands **written in the workflow document**, never given by the agent nor by an input (no template). Never an install nor a publish.
2. **Run**: without a shell, in the session location (or the `in` subfolder), with a cleaned environment (paths, locale, tool caches; no tokens, credentials or oh variables), an empty stdin, a timeout (`timeout`, 10 min by default, at most 30 min; the process group is killed when it expires) and a bounded output (end of output kept, 30-line excerpt).
3. **Result**: `expect: pass` (default) requires every command to succeed; `expect: fail` the commands that build the code to pass and a test to fail when it runs (test written first, see [TDD method](#tdd-method-cp-seams-and-cp-red-r7)); a timeout is always a failure. `must_touch` also requires the changes of the session (commits since its start, uncommitted changes, new files) to touch one of the paths.
4. **Reuse**: the result is tied to the state of the work (HEAD, changes, untracked files). It is reused while that state does not change; otherwise the verification runs again.

**At a paused checkpoint**, the verification starts as soon as the agent asks for the checkpoint: the decision card shows its result (running, passed, failed with an excerpt). **When it fails**:

- "Validate" is refused; the card offers **"Override the verification"**, with a mandatory instruction in the TUI. The override is traced in the timeline (`⚑ cp-2`) and in the session trace (`overridden` event, author and message);
- a validation given elsewhere (in the tool UI) does not override: the agent call fails with `ErrVerifyFailed`, a summary and the output excerpt, and nothing is unlocked. The agent fixes the work, then asks for the checkpoint again.

**At an automatic checkpoint**, the verification runs during the agent call; past 20 seconds, the call answers "verification running" and the agent calls again a bit later (the result is kept).

A project without a detected stack: "nothing to verify", the checkpoint passes and the card says so.

**Verification impossible: never a silent pass.** When the work location cannot be found (folder deleted, `in` subfolder missing), the verification cannot run: the checkpoint **pauses in every mode**, `auto` included, even when it would usually pass on its own. The card shows "cannot run here"; "Validate" is refused and only **"Override the verification"** lets it pass, traced like a failure (`⚑`, `overridden` event). The agent call fails with `ErrVerifyUnavailable` and nothing is unlocked. The same goes when the location is a git repository git refuses to read (a repository of another owner, for one): with no known state of the work, no result can be tied to it, the card shows "git cannot read the state of the work" and no command runs.

In a read-only workflow (`risk: read`), a verified checkpoint always asks the user: the commands only run once validated.

**Loosening**: a team or project layer may harden an inherited verification (more kinds, commands, `must_touch` paths, a longer timeout), never remove nor weaken it (`loosening`); removing a verified checkpoint is refused (`verified_checkpoint_removed`).

The shipped workflows verify `cp-2` (`ticket`, `feature`, `review-feedback`) and `cp-commit` (`quick`, `debug`, `sweep`) this way.

### Remote sessions

In a [remote session](../guides/remote-runners.en.md), the verification runs **where the work is**: in the job, on the clone of the project, with the same commands and the same guarantees (filtered environment, timeout, bounded output, process group killed at the deadline), as the `oh` account of the agents (the one of the tool server), never as root; the git reads of the verification too, like every other git read of oh in the clone (`commit` window, closing guard, git configuration check…, see [Remote runs › Git reads of oh](../guides/remote-runners.en.md#6-in-the-job-oh-runner)). Without an `oh` account in the image, it runs as the user of the job, like the server.

| Situation in the job | What oh does |
|---|---|
| `remote: auto` checkpoint, verification passed | approved by the policy responder, then passed |
| `remote: auto` checkpoint, verification failed | never overridden in the job: the agent call is refused with the failure and the excerpt, nothing is unlocked (when the verification already ended as the responder answers, the decision is refused: "fix first"); the agent fixes the work and asks again |
| verification impossible (location not found, repository git cannot read) | the checkpoint is **deferred**: the session stops cleanly, the rest (and the choice to override) happens on the machine |
| `remote: defer` checkpoint | deferred, as before; the verification started when it was asked ends before the push, and its result comes back |

The job waits for the running verifications before it stops the agent processes and pushes the branch. The last result of each checkpoint travels in `summary.json` (`verifications`); `oh session fetch` prints and imports it, cleaned and bounded: the checkpoint card shows "In the remote job: …" and the timeline keeps the entry (author `remote`). This result **never counts** as a verification on the machine: when the session resumes locally and asks the checkpoint again, oh verifies again, in the worktree of the pushed branch.

---

## Evidence of the agent (`evidence`)

`workflow_checkpoint` accepts `evidence: [{cmd, exit}]`: the commands the agent says it ran and their exit code. oh checks them against the shell calls it observed in the session (and its subagents):

- **observed**: a shell call ran this command;
- **evidence not observed**: no call ran it (warning on the card, in the timeline and in the answer to the agent);
- **observed failing**: the tool reported the call as failed while the agent says 0.

The tool does not report the exit code of a command that succeeds: oh checks the command, and the failure when the tool reports it. The verification run by oh remains the reference evidence.

---

## Fix rounds limit (`limits.max_fix_rounds`)

oh counts the developer → reviewer pairs per ticket (the ticket taken and not closed yet). The first pair is the implementation; past `max_fix_rounds` fix rounds, when the reviewer starts the last allowed round:

- delegations to the `developer*` agents are refused;
- the commit checkpoint is marked **"unresolved"** on the decision card;
- the timeline shows `⛔ <ticket>`.

The lock is lifted when the user asks for a fix (or gives another instruction) at the checkpoint, or when the ticket is closed.

---

## Conditions evaluated by oh (`when:`)

```yaml
  cp-spec:
    mode: { manuel: pause, semi-auto: conditional, auto: conditional }
    when: { paths: ["docs/specs/**"], labels: [ux] }
```

Conditions (one is enough): `entry` (entry agent of the session), `labels` (label of a ticket of the session), `paths` (path changed by the session, `*` and `**` wildcards), `outputs` (output declared with `workflow_outputs`). Effect:

| Behavior in the mode | Condition holds | Condition does not hold |
|---|---|---|
| `auto` | pause | goes on |
| `conditional` | pause | goes on |
| `pause`, mandatory or unlocking checkpoint | pause | **pause** |
| `skip` | skipped | skipped |

When the changed paths cannot be read (git refuses the repository), a `paths` condition is taken as true: the pause is added rather than lost. `when:` can only **add** a pause, never remove a mandatory one. Without `when:`, a `conditional` checkpoint always asks the user (oh cannot evaluate the `condition` sentence). A layer cannot drop an inherited condition (`loosening`).

---

## TDD method: `cp-seams` and `cp-red` (R7)

`ticket` and `feature` have a **Method** input (`method`, `standard` by default). Launched with `tdd` (`oh run ticket -t <id> -i method=tdd`, or the launch form), they add two checkpoints, passed by the `developer`, once per ticket, before the implementation:

```
cp-1 → cp-seams (pause) → cp-red (verified by oh) → implementation → review → cp-2
```

| Checkpoint | manuel | semi-auto | auto | remote | Role |
|---|---|---|---|---|---|
| `cp-seams` — Seams and planned tests | pause | pause | auto | auto | the developer presents the seams, the interfaces and the planned tests, before writing a test; oh records the commit of the location |
| `cp-red` — Failing tests | auto | auto | auto | auto | oh checks that the tests written fail, for the right reason |

The method is evaluated **at launch** (CP-R7): the bundle of a `tdd` session holds both checkpoints, their descriptions and what oh checks (session contract of the developer, "Method of the session" line for every agent); the bundle of a `standard` session is exactly the one from before, without them. Neither checkpoint is mandatory nor unlocking: they only add a step. A `tdd` session counts **one more pause** in `manuel` and semi-auto (`cp-seams`, report §9.2 "+1 pause per TDD ticket"); none in `auto`.

**Verification of `cp-red`** (`verify: { from: stack, expect: fail, must_touch_tests: true, outside_tests: warn, since: cp-seams }`), in this order:

1. **Changes since `cp-seams`**: the files changed since the commit recorded when `cp-seams` was passed (commits, uncommitted changes, new files; since the start of the session when `cp-seams` was not passed) must include **at least one test file** of the stack (`*_test.go`, `**/testdata/**`; `*.test.*`, `*.spec.*`, `**/__tests__/**`; `test_*.py`, `*_test.py`, `conftest.py`; `**/src/test/**`; `test/`, `tests/`, `spec/`, `fixtures/` folders; those of every stack when none is detected). Otherwise: refused, nothing is run.
2. **Files outside the tests**: they are **noted**, not refused (`outside_tests: warn`): the result (answer to the agent, timeline) names them (5 at most). Why not a refusal: for a test to fail "for the right reason", the code must build, so the stubs (interfaces, signatures, functions that do nothing yet) change files outside the tests; and Rust unit tests live in the code files. A team workflow may still refuse them (`outside_tests: refuse`). A full implementation written too early is caught by step 4.
3. **The code builds**: oh first runs the commands that build (`build`, `typecheck`, `lint` when asked); they must pass. A failure is refused ("the code must build, only the tests must fail") and the tests are not run.
4. **The tests fail when they run**: then the tests. They must fail, for the right reason; refused: tests that do not compile (`go test`: `[build failed]`, `[setup failed]`; `cargo`: `could not compile`; `tsc`: `error TS…`), tests pytest cannot load (exit code 2, `ERROR collecting`) or no test run (code 5); a suite that passes ("a failure was expected"); a timeout. What a tool does not tell apart (jest, vitest, `mvn`, a script) counts as a failing test: the detection reads the output and the exit code, without parsing the language.

When refused, the call of the agent fails with `ErrVerifyFailed`, the reason and an excerpt of the output; it fixes and calls `cp-red` again. The checkpoint being automatic, no card opens: in `manuel`, `cp-seams` remains the human decision. In a remote session, the verification runs in the job, like the one of `cp-2` (verification impossible: checkpoint deferred, never passed silently). Project without a detected stack: "nothing to verify", only step 1 applies.

---

## Bounded unlock windows (CP-R8)

| Operation | Opened by | Closed |
|---|---|---|
| `create` | its checkpoint (`cp-tickets`, `cp-ticket`) | at the **next mandatory checkpoint** (`cp-0`, `cp-commit`…) or when the checkpoint is asked again; not when a ticket is closed |
| `commit` | its checkpoint (`cp-2`, `cp-commit`) | **once the work is committed** (HEAD moved and nothing is left to commit: one commit, or several in a row), when the checkpoint is asked again, or when the ticket is closed |
| `close` | its checkpoint | when a ticket is closed, or when the checkpoint is asked again |
| `apply`, `push` | their checkpoint | when the checkpoint is asked again |

When a git read fails, these protections stay **closed**: when the state of the work cannot be read after a shell call, the `commit` window closes (the timeline keeps the reason; asking the checkpoint again opens it) instead of staying open; closing a ticket (`bd close`) is refused as long as oh cannot see a non-empty commit since the checkpoint and a clean tree.

---

## Who passes a checkpoint (`by:`, CP-R11)

Each checkpoint names the agents that pass it (`by:`; otherwise deduced from the delegation graph). Each agent sees only its own: in its session contract and in the generated checkpoint table. A checkpoint passed by another agent is not refused: oh logs it and the `workflow_checkpoint` answer carries a warning (« check that it is your checkpoint »).

---

## Decision card (CP-R5, verification part)

For a paused checkpoint, the TUI card shows, besides the agent summary, the changes, the last messages and the timeline:

- **Verification run by oh**: status, commands and exit code, output excerpt on failure; for a fetched remote session, the result obtained in the job;
- **Evidence of the agent**: each command reported, observed or not;
- **Unresolved**: the ticket whose fix rounds reached the limit;
- **Wiki proposals** (CP-R4, checkpoint that unlocks the commit only): the `wp-…` proposals of the session (author, page, section, tag, start of the content, escaped), with one action per proposal (accept, edit — on one line in the field, `⏎` for a line break, or on several lines with "Edit in the editor", which opens `$EDITOR` —, refuse, leave queued) and "Accept the N proposals". Outside the TUI: [`oh wiki decide`](cli-projects.en.md#oh-wiki-decide). The accepted ones are applied by oh when the checkpoint is passed, before the commit; see [wiki MCP server](cli-mcp.en.md#wiki-mcp-server). On a `wiki: sync` checkpoint (`doc-sync`), the card shows the whole queue of the project, proposals of the other sessions included.

Choices: Validate · Fix first · Other instruction · **Override the verification** (only when it failed or cannot run).
