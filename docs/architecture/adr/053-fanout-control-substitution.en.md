> [Lire en français](053-fanout-control-substitution.fr.md)

# ADR-053 — Fan-out: child sessions driven by oh and control substitution (CP-R3)

## Status

Accepted (experimental, behind `[fanout] experimental`)

## Date

2026-10-14

## Context

`implement-spec` (R5) processes the tickets of a spec one after the other in a single session, with one `cp-2` per ticket: N tickets make N + 2 pauses. The modernisation report (§ 9.2 to 9.5) asks for child sessions, one per ticket, each in its own worktree, merged by oh, and a single grouped validation; but a layer may only harden, and the `cp-2` of a child is mandatory. Replacing that `cp-2` is a loosening, defensible only if a control at least as strong replaces it and nobody else can declare it.

Three technical constraints: the daemon launches no session (only the `oh` process has the launcher and the credentials); an agent cannot wait long in a tool call; and an idle session goes to sleep.

## Decision

1. **oh drives, the agent launches nothing.** The fan-out driver (`services/fanout`) runs in an `oh` process: `oh fanout drive`, started in the background by `oh run`, or an evaluation pass. After `cp-plan`, it creates the integration branch, launches one `ticket` child session per ticket (1 to 3 at a time, dependencies first), waits for the end of its work (commit checkpoint passed, a commit, nothing uncommitted; 10 minutes idle at most), merges it itself (`--no-ff`), launches the `merger` on a conflict and checks its result, then writes to the parent session, idle meanwhile (no cost). The whole state is in the `fanout` record of the parent session (migration v46), written by oh only: a restarted driver goes on.
2. **Substitution declared by the hub, validated by the schema.** `fanout.delegates` is accepted only from a published document of the hub layer (`fanout_delegates_layer`), with a mandatory merge checkpoint, `fanout: only`, that unlocks `merge` and is verified by oh on the integration branch. A patch may only lower `parallel`.
3. **Checked again by the daemon at each call.** The `cp-2` of a child does not pause only when the record of the child (written by oh at launch) names a running parent session, in the same project, launched with the flag, that lists it, and whose bundle delegates this checkpoint to a compliant merge checkpoint (`Substitutes`). Its verification still runs; the pass is traced (`delegated`).
4. **The replacing control is stronger.** `cp-merge` passes only once every child is done, on an integration branch where oh left it, after its verification on that branch, never overridden; oh then merges the integration branch and closes the merged tickets.
5. **No session of the fan-out pushes, merges, closes a ticket or moves its branches** (session rules `FanoutRoleRules`, Beads gateway); the parent does not commit.
6. **`merge` operation**, lockable, unlocked by the merge checkpoint of a fan-out only, and done by oh (no window for the agents).

## Consequences

- A feature of N tickets: 3 pauses in semi-auto (`cp-plan`, `cp-merge`, `cp-final`) instead of N + 2. Validation by batch on the command line (`oh session approve --batch`, CP-R6) for the remaining pauses.
- The git configuration fingerprint of a session is now recorded per worktree: before, a session launched in another worktree of the repository made the configuration look changed.
- Not done in R6, added afterwards (lot r6-leftovers): validation by batch in the TUI; fix round when `cp-merge` is refused, by a fix session (`fanout.fix`, role `fixer`) never delegated, `cp-merge` verified again then, the substitution unchanged; clean-up of the `oh/fanout/…` worktrees and branches (never an unmerged branch without `--discard`); inputs given to the children (`fanout.inputs`); end of a child seen by oh (idleness: a 10-minute guard). Then (lot sweep-fanout): `sweep` on the fan-out (`fanout.source: tasks`, `sweep-task` children from the breakdown read and checked by oh at `cp-plan`, `cp-commit` delegated to `cp-merge`, scope of each child checked by oh, no ticket); `feature` is an alias of `spec` → `implement-spec` (lot feature-migration).
- Then (lot fanout-container): fan-out of a parent session in a container; children, merger and fix round in the runtime of the parent (each in a container of its workflow), driver, merges and closings on the machine, verifications in a container of the project image (never on the machine instead), substitution and guards unchanged; remote session still refused (backlog).
