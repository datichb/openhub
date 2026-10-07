> [Lire en français](050-session-context-capability.fr.md)

# ADR-050 — Evolving Session State through an Adapter Capability

## Status

Accepted

## Date

2026-10-07

## Context

The entry agent of a session needs a state that changes during the session: checkpoints passed and current, remaining budget after a `$` decision, resume instruction after a sleep. Until now:

- the checkpoint state was only readable on demand, through the `workflow_status` MCP tool;
- nothing reported the budget or the resume;
- only the refusal of a checkpoint reached the agent, as a `steer` instruction (`SessionService.Send`), because opencode 2.0.20 does not pass the message of a refused permission.

The S8 study (07/10/2026, `02-findings` § "Étude S8") measured the **per-session instruction entries** of opencode 2.0.20 (experimental API `PUT|DELETE …/session/{id}/instructions/entries/{key}`):

- an entry set before the first turn is taken into account;
- a change or a removal is announced at the next turn, with a system message that stays in the history and invalidates the prompt cache from there;
- subagents do not receive it.

The agent bodies and the workflow map stay injected by the plugin (O2): their scope is the agent, subagents included, with a stable cache prefix. Decision D19 ([ADR-049](049-tool-independence-architecture-guard.en.md)) requires going through a capability of the adapter.

## Decision

1. **Neutral capability** (`internal/adapters`):
   - `Capabilities.SessionContext`;
   - the optional interface `SessionContextSetter { SetSessionContext(ctx, h, sessionID, key, value any); ClearSessionContext(ctx, h, sessionID, key) }`;
   - the `adapters.ErrUnsupported` error.
   The opencode adapter implements it with the instruction entries: key `^[a-z0-9][a-z0-9._-]*$`, JSON value of at most 256 KiB. A missing route (404 without a typed error, or 405) gives `ErrUnsupported`, and the capability is then switched off for that server.
2. **Writing** (`internal/sessionctx`): the value is compared with the last one written (`~/.oh/sessions/<id>/context.json`), and nothing is written while it does not change.
3. **Neutral and stable keys**:
   - `oh.checkpoints` (workflow, mode, passed, current, next, circuit breaker): written by the daemon at each transition;
   - `oh.budget` (limit, raises included, spent, remaining, exhausted): written at the `$` decision, then when the limit changes (raise), never at each step;
   - `oh.resume` (instruction): set by the RunService when `oh session resume` restarts the server, removed by the daemon after the next step.
4. **Fallback** without the capability, or after `ErrUnsupported`:
   - `oh.resume` is sent as a `synthetic` oh message;
   - `oh.checkpoints` and `oh.budget` are not sent: the agent reads them with `workflow_status`, as before.
5. **The refusal of a checkpoint stays a `steer` instruction**: it is a one-off order, not a state. Subagents do not receive the session state (tool limit, documented).

## Consequences

### Positive

- The entry agent knows the state of the workflow, the budget and the resume without asking, from the step that follows the change.
- Few messages: one entry per real change, and the budget is not rewritten at each step.
- The experimental API is isolated in the adapter, with an explicit fallback if it disappears.

### Negative / Trade-offs

- Each change adds a message to the history and invalidates the prompt cache from it.
- Subagents do not see the state: a subagent that needs it must call `workflow_status`.
- A write made during a step is only announced at the next step.
- The last value written is kept in a file shared by the CLI and the daemon; two simultaneous writes of the same key may overlap, and the next one corrects it.

## Alternatives Considered

| Alternative | Rejected because |
|---|---|
| Inject the state through the plugin (`session.hook("context")`, O2) | The plugin body comes from the immutable bundle: the state does not change during the session. |
| Send each change as a `synthetic` message | One message at each transition and at each step for the budget, with no gain on tools that keep a state. |
| Agent bodies and the workflow map as instruction entries | Root-session scope only (not the subagents) and invalidated cache (S8 study). |
| Rewrite the budget at each step | One message per step in the history. |
