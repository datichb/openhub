> [Lire en français](fan-out.fr.md)

# Fan-out: one child session per ticket or subtask (experimental)

The fan-out has the tickets of a spec (`implement-spec`) or the subtasks of a sweep (`sweep`) implemented by **child sessions**, one per ticket or subtask, each on its own branch and worktree, sometimes at the same time. oh merges them itself into an integration branch, has the `merger` resolve a conflict it cannot resolve alone, and merges the whole into your branch only after **a single validation**, `cp-merge`, verified by oh on the integration branch. It is **experimental** and off by default.

## Enable

```
oh config set fanout.experimental true
oh config set fanout.parallel 2          # 1 (default: one after the other) to 3 child sessions at the same time
oh config set fanout.budget_usd 10       # global budget of the parent session and its children (default 10)
```

Or in `hub.toml`:

```toml
[fanout]
experimental = true
parallel = 2
budget_usd = 10
```

Without `experimental`, a launch with `fanout=true` is refused. The fan-out only runs locally (no container, no remote session).

## Launch

```
oh run implement-spec -i fanout=true                  # tickets and spec of the last spec session of the project
oh run implement-spec -i fanout=true -t ev-1 -t ev-2  # or given tickets
oh run sweep -i goal="migrate the logs to slog" -i fanout=true   # one child session per subtask approved at cp-plan
```

`oh run` also starts, in the background, the **driver** of the fan-out (`oh fanout drive <session>`, log in `~/.oh/sessions/<session>/fanout.log`): it launches, follows and merges the child sessions, whether you keep the client of the parent session open or not. If it stopped before `cp-merge`, the oh daemon starts it again by itself when the parent session goes on or ends a step (`driver` event); `oh fanout drive <session>` starts it by hand and goes on where it was.

## Steps

1. **`cp-plan`** (pause): the `orchestrator-dev` presents the tickets, their dependencies, those already closed. Nothing starts before.
2. **Child sessions**: oh creates the integration branch `oh/fanout/<session>/integration` from your branch, then one `ticket` session per ticket, on `oh/fanout/<session>/<ticket>`, in a worktree next to the project. A ticket waits for those it depends on; no session for a closed ticket. The children get the mode of the parent session and the inputs the workflow gives them (`fanout.inputs`: the `tdd` method and the instructions for `implement-spec`), nothing else.
3. Each child implements, has its work reviewed and passes **its `cp-2` without pause**: your `cp-merge` replaces it (control substitution, CP-R3). oh still runs the verification (build, tests) in the worktree of the child; a failure sends the work back to the child. The child commits on its branch, but does not close its ticket, merge nor push: oh refuses it.
4. **Merge by oh**: a child counts as done when its session ends, or when oh sees the end of its work, its turn over with no decision waiting: commit checkpoint passed (its delegated `cp-2`), at least one commit on its branch, nothing uncommitted. An idle child without this signal is only stopped after 10 minutes (guard). oh then merges its branch into the integration branch. On a conflict, oh launches the `merger` (workflow `merge-conflicts`) in the integration worktree (idle 2 minutes without concluding the merge, it gave up: oh stops it), then checks the result (merge committed, both sides present, no marker). Otherwise the child stays unmerged and the integration branch goes back where it was.
5. Once the children are done, oh writes to the parent session; the `reviewer` reviews the whole integration branch.
6. **`cp-merge`** (pauses in every mode): the table of the children and the verdict of the review. oh runs the verification **on the integration branch**; it cannot be overridden. Once validated, oh merges the integration branch into your branch and closes the merged tickets. **Refused with an instruction** ("Fix first", failed verification or review): oh launches a **fix round** on the integration branch, a `quick` session in its worktree with your instruction and what failed; you validate its `cp-commit` (never delegated), it commits, oh checks the result (commits on top of the integration branch, no merge, nothing uncommitted) and takes it, else the branch goes back where it was. The parent session has the fix reviewed and passes `cp-merge` again, verified again on the integration branch. At most `limits.max_fix_rounds` rounds (3).
7. **`cp-final`**: recap. The unmerged tickets (unresolved conflict, failure, budget) stay open: launch `implement-spec` again for them.

In semi-auto, three pauses whatever the number of tickets: `cp-plan`, `cp-merge`, `cp-final` (two more per fix round: its `cp-commit` and the new `cp-merge`).

## Fan-out sweep

`sweep` follows the same steps, with subtasks instead of tickets (`fanout.source: tasks`):

1. **`cp-plan`** (pause): the `conductor` gives the breakdown of the `pathfinder` in the `tasks` argument of the checkpoint (title, description, file scope, domain per subtask). oh **checks it before showing it to you**: files of the repository (or new files in an existing folder), pairwise **disjoint** scopes, at most 12 subtasks, a known domain; otherwise the call is refused and the `conductor` fixes it. The card shows the breakdown as oh read it, and oh records that one only (`tasks` output).
2. **Child sessions**: one `sweep-task` session per recorded subtask, on `oh/fanout/<session>/t1`, `t2`…, with its subtask, scope, domain, the goal and the verification asked (`verify`). Its `cp-commit` passes without a pause (replaced by your `cp-merge`), oh runs the verification in its worktree there and **refuses the commit of a change outside the scope** of the subtask.
3. **Merge by oh**, review of the whole, **`cp-merge`** verified on the integration branch, a fix round when you refuse it with an instruction (one round): as above. No ticket: nothing to close.
4. **`cp-recap`**: the subtasks merged or not.

Without `[fanout] experimental`, `sweep` keeps its sequential version (subtasks in the session, `cp-commit`) and `-i fanout=true` is refused. Three pauses in semi-auto in both versions.

## Clean-up

oh removes the `oh/fanout/…` worktrees and branches once `cp-merge` passed (everything is on your branch then), and when the parent session is stopped for good (abandon: it also stops the children still running); `oh gc` takes what is left (category `fanout`). A worktree stays while a session of the fan-out works in it or while it has uncommitted changes. A branch goes only when each of its commits is on another branch: after an abandon, the integration branch and the unmerged children stay, and the events of the fan-out say why (`kept`).

```
oh fanout clean <session> --dry-run    # what would be removed, what stays and why
oh fanout clean <session> --discard    # also deletes the unmerged branches, their commit traced (event discarded)
```

A branch deleted with `--discard` comes back with `git branch <branch> <commit>` while git keeps the commit.

## Follow

```
oh fanout status <session>        # integration branch, each child (state, session, merge, detail), budget, last events
oh session list                   # the children marked ↳ (with their ticket), the parent ⑂
oh session trace <session>        # trace of the parent, with the fan-out events and the checkpoints passed in each child
```

States of a child: `pending`, `running`, `merged`, `conflict` (unresolved), `failed` (no commit, uncommitted changes, ticket already closed, dependency not merged), `budget` (stopped or not launched by the global budget).

## Global budget

The parent session and all its sessions (children and merger) share `fanout.budget_usd`. Before each child, oh compares what they spent with the budget: reached, no child starts any more; exceeded during a child, oh stops it. Each child gets what remains as its session budget. Everything is written in the events of the fan-out and in the message to the parent session.

## Validate by batch

The waiting checkpoints of several sessions (children that keep their pause, remote sessions, `cp-merge` of several fan-outs) are validated at once, each one still listed and refusable:

```
oh session approve --batch --fanout <session> --dry-run
oh session approve --batch --fanout <session> --except <decision> --reject <decision>=<instruction>
```

In the TUI, key `B` of the Sessions view: the waiting checkpoints of the fan-out of the selected session (else all of them), each with its session, its summary and the verification by oh, to validate, leave waiting or refuse with an instruction; a confirmation, then the result of each one.

## Guarantees

The guards of the control substitution (declared by the hub layer only, mandatory `cp-merge` verified by oh, children without push, merge nor closing, merge by oh, traceability, flag) are detailed in [Checkpoints](../reference/checkpoints.en.md#fan-out-control-substitution-cp-r3). A team layer cannot loosen the fan-out of `implement-spec` nor that of `sweep`: it can only lower `fanout.parallel` (and, for `sweep`, `tasks.max` or the `tasks.domains` list). A layer that hardens the `cp-2` of `implement-spec` or the `cp-commit` of `sweep` keeps it outside a fan-out, but these checkpoints do not exist in a fan-out session: `oh workflow validate` warns about it (`fanout_never_patched`) and `oh doctor` lists these layers; harden `cp-merge` too.

## Limits (experimental)

- `feature` is an alias (`spec`, or `implement-spec` with `--tickets`): `oh run feature --tickets … -i fanout=true` thus launches this fan-out.
- A child that waits for an answer in its text (without the `question` tool) is only stopped by the 10-minute guard and stays unmerged.
- The driver follows the fan-out until `cp-merge` passes (to launch the fix rounds): if it stopped, the daemon starts it again (or `oh fanout drive <session>`).
