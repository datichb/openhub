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
| `ticket` | `cp-2` | `cp-2` | 3 |
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
3. **Result**: `expect: pass` (default) requires every command to succeed; `expect: fail` one to fail (test written first, TDD); a timeout is always a failure. `must_touch` also requires the changes of the session (commits since its start, uncommitted changes, new files) to touch one of the paths.
4. **Reuse**: the result is tied to the state of the work (HEAD, changes, untracked files). It is reused while that state does not change; otherwise the verification runs again.

**At a paused checkpoint**, the verification starts as soon as the agent asks for the checkpoint: the decision card shows its result (running, passed, failed with an excerpt). **When it fails**:

- "Validate" is refused; the card offers **"Override the verification"**, with a mandatory instruction in the TUI. The override is traced in the timeline (`⚑ cp-2`) and in the session trace (`overridden` event, author and message);
- a validation given elsewhere (in the tool UI) does not override: the agent call fails with `ErrVerifyFailed`, a summary and the output excerpt, and nothing is unlocked. The agent fixes the work, then asks for the checkpoint again.

**At an automatic checkpoint**, the verification runs during the agent call; past 20 seconds, the call answers "verification running" and the agent calls again a bit later (the result is kept).

A project without a detected stack, or a location not on the machine (remote session): "nothing to verify", the checkpoint passes and the card says so.

In a read-only workflow (`risk: read`), a verified checkpoint always asks the user: the commands only run once validated.

**Loosening**: a team or project layer may harden an inherited verification (more kinds, commands, `must_touch` paths, a longer timeout), never remove nor weaken it (`loosening`); removing a verified checkpoint is refused (`verified_checkpoint_removed`).

The shipped workflows verify `cp-2` (`ticket`, `feature`, `review-feedback`) and `cp-commit` (`quick`, `debug`, `sweep`) this way.

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

`when:` can only **add** a pause, never remove a mandatory one. Without `when:`, a `conditional` checkpoint always asks the user (oh cannot evaluate the `condition` sentence). A layer cannot drop an inherited condition (`loosening`).

---

## Bounded unlock windows (CP-R8)

| Operation | Opened by | Closed |
|---|---|---|
| `create` | its checkpoint (`cp-tickets`, `cp-ticket`) | at the **next mandatory checkpoint** (`cp-0`, `cp-commit`…) or when the checkpoint is asked again; not when a ticket is closed |
| `commit` | its checkpoint (`cp-2`, `cp-commit`) | **once the work is committed** (HEAD moved and nothing is left to commit: one commit, or several in a row), when the checkpoint is asked again, or when the ticket is closed |
| `close` | its checkpoint | when a ticket is closed, or when the checkpoint is asked again |
| `apply`, `push` | their checkpoint | when the checkpoint is asked again |

---

## Decision card (CP-R5, verification part)

For a paused checkpoint, the TUI card shows, besides the agent summary, the changes, the last messages and the timeline:

- **Verification run by oh**: status, commands and exit code, output excerpt on failure;
- **Evidence of the agent**: each command reported, observed or not;
- **Unresolved**: the ticket whose fix rounds reached the limit.

Choices: Validate · Fix first · Other instruction · **Override the verification** (only when it failed).
