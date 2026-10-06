> [Lire en français](042-checkpoints-headless-decisions.fr.md)

# ADR-042 — Three-Level Checkpoints and Headless Decisions

## Status

Accepted

## Date

2026-10-05

## Context

The checkpoints of [ADR-003](./003-orchestrator-checkpoints.en.md) (`[CP-0]` to `[CP-3]`) and the modes of [ADR-006](./006-orchestrator-configurable-mode.en.md) only existed in the prompts. The agent asked the question in the conversation and waited for an answer.

Nothing forced it to:

- a model could skip a checkpoint or launch the developer before validation;
- a decision was only visible in the session's interface;
- with several sessions, one had to go from window to window.

opencode V2 provides what is needed to control this from outside: permission requests and forms that can be driven through the API (F17), several clients on the same session where the first answer wins (F20), session rules (F26), plugin hooks (F27).

Decision D8: checkpoints on three levels, state machine in `oh`. Server-mode usages S3 to S6.

## Decision

### 1. Level 1 — generated instructions

- The skills generated from the workflow ([ADR-039](./039-declarative-workflows-oh-v1.en.md)) ask the agent to call `workflow_checkpoint {id, summary}` at each checkpoint, and not the `question` tool.
- The initial prompt ends with the list of checkpoints to report before each locked agent.

### 2. Level 2 — MCP tool and permissions

- **`workflow` MCP server** (`oh mcp serve workflow`, added to every bundle, stateless): `workflow_status`, `workflow_checkpoint` and `workflow_outputs`. It queries the `ohd` daemon, which identifies the calling session from the call's `_meta` and attaches a sub-session to its root session.
- **Bundle rules**: `workflow_checkpoint` as `ask`, `workflow_status` and `workflow_outputs` as `allow`.
- **Session rules**, recomputed from the mode and the state:
  - `workflow_checkpoint` as `ask` if a checkpoint waits for the user in the mode, otherwise `allow`;
  - `subagent <agent>` refused as long as its `after:` lock is not lifted;
  - `subagent *` refused during the circuit breaker.
- A rule cannot target a specific checkpoint: opencode 2.0.20 names the action `workflow_workflow_checkpoint` on all resources. The daemon therefore reads the call's input:
  - automatic checkpoint in the mode: it answers `once` itself;
  - paused checkpoint: it raises a ⏸ decision.
- **Dynamic gating** through the API (`PATCH /api/session/{id}` with the rules), at every resync, checkpoint passed, finished delegation that lifts a lock, and circuit breaker set or lifted. The rules are also applied to sub-sessions.

### 3. Level 3 — oh plugin

- `permission.hook("evaluate")` is only called for `ask` requests. The plugin relays the request to the daemon (`POST /oh/v1/hooks/permission`, on the proxy listeners, authenticated by the group's proxy token, 2 s timeout).
- The only decision it takes is `allow` for a checkpoint that does not wait for the user in the mode: it is then never shown. Otherwise the rule stands.
- Without the plugin, level 2 is enough (answer from the daemon).

### 4. CheckpointService

`internal/services/checkpoint` runs in the daemon. It keeps a state machine per session, stored in `sessions.checkpoint_state` (migration v35, updated by compare-and-swap, since the daemon, the CLI and the TUI write):

- passed or pending checkpoints;
- approvals not yet seen by the agent;
- agents that ran;
- circuit breaker counter;
- timeline (200 entries at most).

Checkpoint IDs are case-insensitive.

### 5. Circuit breaker

Beyond `circuit_breaker.max_consecutive_subagents` consecutive `subagent` calls without user intervention, a dedicated ✗ decision (`circuit`) is raised and `subagent *` is refused. Dismissing the decision lifts the refusal, with an optional instruction.

### 6. Headless decisions

- The daemon copies the tool's pending requests into `pending_decisions` (migration v33), from the SSE stream and at every resync. Kinds are: checkpoint ⏸, question ?, permission !, budget $, error ✗ and circuit breaker.
- A request settled elsewhere (opencode interface, browser) is marked as resolved by the tool. Sub-session requests are filed under the root session.
- `SessionService.Decide` **reserves** the decision in the database (atomic operation, the first answer wins) before answering the tool, and reopens it if delivery fails.
- Possible answers:
  - **permission**: `once`, `always` (forbidden with `isolation: strict`) or `reject`, with a message;
  - **question**: typed and validated answers;
  - **checkpoint**: `approve`, `fix`, `other` or `reject`.
- opencode 2.0.20 does not pass an answer's message to the agent:
  - the note of an approval is returned by the MCP tool when the call runs;
  - the instruction of a refusal is sent as a `steer` message.
- oh can also send a message or a `synthetic` message (S6), interrupt, change the model, compact or fork.

### 7. Surfaces

- **TUI**:
  - "To handle" section of the Sessions view;
  - checkpoint card: agent summary, changes, last messages, timeline `✔ cp-1 → developer → ⏸ cp-2 → ○ cp-3`, and Approve, Fix first or Other instruction actions;
  - `● N ⏸ M` badge on every screen.
- **System notifications** sent by the daemon, whether the TUI is open or not (`terminal-notifier`, `osascript`, `notify-send`), without any session content.
- **CLI**: `oh session inbox|approve|answer|dismiss|send|interrupt|model|compact|fork`.
- **Remote**: a policy responder replaces the user ([ADR-045](./045-execution-environments.en.md)).

## Consequences

### Positive

- Checkpoints and `after:` locks are enforced by oh, not by the model's goodwill: `developer` is refused before `cp-1` and allowed after approval (Haiku e2e `TestE2ECheckpointGating`).
- A decision can be taken from any client, and the first answer wins.
- Several sessions are followed from one place, with notifications.
- The same chain works in a container: the `workflow` MCP server goes through the MCP gateway.
- Level 2 remains a fallback if the plugin does not load.

### Negative / Trade-offs

- The mechanism relies on specific opencode 2.0.20 behaviors: `<server>_<tool>` action naming, `evaluate` called only for `ask`, refusal message not passed on, the agent only seeing "Unable to execute". These points must be checked again with each version.
- An agent that does not call the checkpoint stays blocked in front of a locked agent: the refusal is safe, but the session may stall. During acceptance testing, the reminder at the end of the prompt had to be added, or an instruction sent. The tool name sometimes wavers (`workflow_checkpoint` / `workflow_workflow_checkpoint`, issue Q3-2).
- The `condition` of a `conditional` checkpoint is not evaluated: it always asks the user.
- Three levels, three writers of the state: the complexity is paid for in tests (unit, contract, e2e).
- The circuit breaker counter is a heuristic: it does not tell useful delegations from loops.

## Alternatives Considered

| Alternative | Rejected because |
|---|---|
| Keep checkpoints in the conversation (`question` tool or text) | Nothing prevents the agent from skipping them, and the decision is only visible in the session's interface. |
| One permission rule per checkpoint | opencode does not distinguish checkpoints in the action (same action, `*` resources). |
| Dynamic gating through `ctx.permission.rules` in the plugin (F26) | Documented but unnecessary: `PATCH` of the session rules through the API was verified and does not depend on the plugin. |
| State machine in the plugin | Business logic in the tool, lost at every server restart, invisible to other clients (O6). |
| opencode's `--auto` for headless sessions | Blindly approves every request (F25); the remote uses a policy responder. |
