> 🇫🇷 [Lire en français](todowrite-session-isolation.fr.md)

# Todo List and Session Isolation — OpenCode Behavior

This document describes OpenCode's internal behavior for the `todowrite` tool,
the isolation constraints that follow from it, and the architectural decisions
made to adapt todo list usage in the multi-agent hub.

> See also: [task-delegation.en.md](task-delegation.en.md) (delegation mechanism),
> [`skills/posture/tool-todowrite.md`](../../skills/posture/tool-todowrite.md) (usage rules).

---

## Context — The Discovery

During the design of progress tracking improvements (v1.5.0+), a fundamental question
arose: **can a subagent invoked via `task` update the todo list visible to the user?**

Investigation of OpenCode's source code (`packages/opencode/src/tool/todo.ts`,
`packages/opencode/src/session/todo.ts`, `packages/core/src/session/sql.ts`)
revealed a hard constraint that directly impacts the responsibility architecture
between agents.

> The code excerpts on this page were taken from an opencode version older than V2. oh v5 requires
> opencode V2; the principle (one todo list per session, child sessions for `task`) remains the one described here.
> What changes in v5 is summarized in [v5 Sessions](#v5-sessions).

---

## Internal Architecture — How OpenCode Stores the Todo List

### Per-session SQLite Storage

The todo list is persisted in a SQLite `todo` table with the following schema:

```typescript
export const TodoTable = sqliteTable("todo", {
  session_id: text().notNull().references(() => SessionTable.id, { onDelete: "cascade" }),
  content:    text().notNull(),
  status:     text().notNull(),
  priority:   text().notNull(),
  position:   integer().notNull(),
}, (table) => [
  primaryKey({ columns: [table.session_id, table.position] }),
])
```

**Key points:**

- The primary key is `(session_id, position)` — each row belongs to a specific session
- All reads and writes use `WHERE session_id = ?`
- An update is a **full replacement**: DELETE all rows for `session_id`, then INSERT the new ones
- Todos are cascade-deleted when the session is deleted

### Strict Per-session Isolation

When the `todowrite` tool is called, it uses `ctx.sessionID` as the key:

```typescript
// todo.ts — execute()
yield* todo.update({ sessionID: ctx.sessionID, todos: params.todos })
```

This `ctx.sessionID` is **the current agent's session ID** — not a global or shared session.

---

## The Task Mechanism Creates an Isolated Session

When a parent agent invokes the `task` tool, OpenCode creates **a new session**:

```typescript
// task.ts
const nextSession = session ?? (yield* sessions.create({
  parentID: ctx.sessionID,     // hierarchical link — navigable in the TUI
  title: params.description + ` (@${next.name} subagent)`,
  agent: next.name,
}))

yield* ops.prompt({
  sessionID: nextSession.id,   // ← the subagent uses THIS ID, not the parent's
})
```

The `parentID` creates a hierarchical link navigable in the OpenCode TUI, but **creates
no shared state** between sessions. It is purely a reference relationship.

---

## Visibility Schema by Invocation Level

```
User
    │
    ▼
Agent A (session_id = A)  →  todo list A  ← VISIBLE to the user
    │
    │ task("agent-b", ...)
    ▼
Agent B (session_id = B)  →  todo list B  ← ISOLATED, invisible
    │
    │ task("agent-c", ...)
    ▼
Agent C (session_id = C)  →  todo list C  ← ISOLATED, invisible
```

**Direct consequence:** only the todo list of the root session agent (the workflow entry agent)
is visible in the OpenCode interface.

---

## Agent → Behavior Mapping

| Agent | Invocation context | Session | Todo list visible? | Responsibility |
|-------|--------------------|---------|-------------------|----------------|
| `conductor` | Entry agent (`cadrage`, `sweep`) | Root | ✅ Yes | Maintains the list of steps (one entry per agent and per checkpoint of the workflow map) |
| `orchestrator` | Entry agent (`feature`, `libre`) | Root | ✅ Yes | Maintains the ticket list (1 task per ticket) |
| `orchestrator-dev` | Entry agent (`ticket`, `review-feedback`) | Root | ✅ Yes | Maintains the ticket list with phase labels |
| `orchestrator-dev` | Invoked via `task` from `orchestrator` | Isolated (child) | ❌ No | May maintain an internal list (debugging) — not visible |
| `planner`, `pathfinder`, `onboarder`, `debugger`, `designer` | Invoked via `task` from `orchestrator` or `conductor` | Isolated (child) | ❌ No | No todowrite list |
| `developer`, `developer-refactor`, `developer-migrator`, `reviewer`, `documentarian` | Invoked via `task` from `orchestrator-dev` | Isolated (grandchild in `feature`) | ❌ No | No todowrite list |

An agent that is the entry agent of its workflow (`auditor` in `audit`, `debugger` in `debug`, `developer`
in `quick`…) runs in the root session: its todo list, if it keeps one, is visible.

---

## Responsibility Rule

> **The root session agent (the workflow entry agent) is always
> the sole owner of the visible todo list.**

This rule implies:

1. **The feature orchestrator** maintains a list with **1 task per ticket** (not an
   aggregated "Implementation — N tickets" task). It updates this list at each checkpoint
   received from `orchestrator-dev` via `## Question for orchestrator` blocks.

2. **Standalone orchestrator-dev** maintains its list with **dynamic phase labels**
   to reflect what is actually happening in the currently active developer subagent.

3. **Subagent orchestrator-dev** may maintain an internal list for session debugging,
   but this list is invisible and does not replace the feature orchestrator's responsibility.

---

## Update Rules — Feature Orchestrator

When the feature orchestrator receives a return block from `orchestrator-dev`, it updates
its list **before displaying the report and before calling the `question` tool**:

| Block received | Phase in block | Todowrite action |
|----------------|----------------|-----------------|
| `## Question for orchestrator` | `CP-1` (ticket starting) | Ticket `#bd-XX` → `in_progress` |
| `## Question for orchestrator` | `CP-2` | None (already `in_progress`) |
| `## Question for orchestrator` | `CP-3` if ticket skipped | Ticket `#bd-XX` → `cancelled` |
| `## Return to orchestrator` (partial, after CP-2 commit) | — | Committed ticket → `completed`, next → `in_progress` if applicable |
| `## Return to orchestrator` (final) | — | All remaining tickets → final status (`completed` / `cancelled`) |

**Example — state after CP-2 commit validated for #bd-12, CP-3 in semi-auto toward #bd-14:**

```
todowrite({
  todos: [
    { content: "Feature planning", status: "completed", priority: "high" },
    { content: "#bd-12 — POST /users endpoint", status: "completed", priority: "high" },
    { content: "#bd-14 — DB users migration", status: "in_progress", priority: "medium" }
  ]
})
```

---

## Update Rules — Standalone Orchestrator-dev

When `orchestrator-dev` is invoked directly, update the task label at each key phase
so the user knows where the implementation stands:

| Step | Label | Status |
|------|-------|--------|
| CP-0 (initialization) | `#bd-12 — <title>` | `pending` |
| CP-1 start | `#bd-12 — <title> [dev]` | `in_progress` |
| Step 4 review launched | `#bd-12 — <title> [review]` | `in_progress` |
| Step 5 CP-2 awaiting decision | `#bd-12 — <title> [CP-2]` | `in_progress` |
| CP-2 commit validated | `#bd-12 — <title>` | `completed` |
| CP-1 skip | `#bd-12 — <title>` | `cancelled` |

---

## v5 Sessions

- **Root session**: created by oh at launch (`oh run <workflow>`). Its agent is the workflow entry agent
  (`entry.agent`, or `conductor` by default). It can be open in several clients at once (opencode, browser);
  closing opencode does not stop the session. The visible todo list is that of this session.
- **Child sessions**: each `task` creates a child session. The `ohd` daemon attaches it to the root session
  for activity and decisions ("To handle" section of the Sessions view), but the todo list is not shared:
  it stays specific to each session.
- **Tracking on the oh side**: workflow progress is also visible outside opencode, in the oh Sessions view and the
  checkpoint card (timeline `✔ cp-1 → developer → ⏸ cp-2 → ○ cp-3`). This timeline comes from the checkpoint
  state machine, not from `todowrite` ([ADR-042](./adr/042-checkpoints-headless-decisions.en.md),
  [ADR-047](./adr/047-session-interaction-daemon.en.md)).
- **Permissions**: `todowrite` is explicitly allowed for `conductor`, `orchestrator`, and `orchestrator-dev`,
  and denied for `brief-enricher`.

---

## What Does Not Change

- The fundamental rules of `tool-todowrite.md` apply without exception: exactly
  1 task `in_progress` at a time, real-time updates, full list on every call.
- Agents `developer`, `developer-refactor`, `developer-migrator`, `reviewer`, `documentarian` do not use
  `todowrite` when invoked as subagents: their sessions are invisible.
- The inter-agent handoff mechanism (`## Return to <parent>`, `## Question for <parent>`)
  remains the only communication channel between isolated sessions.

---

## References

- [`skills/posture/tool-todowrite.md`](../../skills/posture/tool-todowrite.md) — Usage rules and phase suffixes
- [`skills/orchestrator/orchestrator-protocol.md`](../../skills/orchestrator/orchestrator-protocol.md) — Update rules for the feature orchestrator
- [`skills/orchestrator/orchestrator-dev-protocol.md`](../../skills/orchestrator/orchestrator-dev-protocol.md) — Dynamic labels for standalone orchestrator-dev
- [`docs/architecture/task-delegation.en.md`](task-delegation.en.md) — Delegation via `task`, session isolation, `task_id`
