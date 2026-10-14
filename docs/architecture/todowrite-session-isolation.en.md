> 🇫🇷 [Lire en français](todowrite-session-isolation.fr.md)

# Todo List and Session Isolation — OpenCode Behavior

This document describes OpenCode's internal behavior for the `todowrite` tool,
the isolation constraints that follow from it, and the architectural decisions
made to adapt todo list usage in the multi-agent hub.

> See also: [task-delegation.en.md](task-delegation.en.md) (delegation mechanism),
> [`skills/posture/coordinator-posture.md`](../../skills/posture/coordinator-posture.md) (usage rules, « Suivi » section; `tool-todowrite` was merged into it in L5).

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
| `conductor` | Entry agent (`spec`, `sweep`, `security-fix`) | Root | ✅ Yes | Maintains the list of steps (one entry per agent and per checkpoint of the workflow map) |
| `orchestrator` | Entry agent (`libre`) | Root | ✅ Yes | Maintains the list of the phases of the feature (planning, specs, audits, implementation) |
| `orchestrator-dev` | Entry agent (`ticket`, `implement-spec`, `review-feedback`) | Root | ✅ Yes | Maintains the ticket list with phase labels |
| `release-manager` | Entry agent (`release`) | Root | ✅ Yes | Steps of the release, if it keeps a list |
| `orchestrator-dev` | Invoked via `task` from `orchestrator` (`libre`) | Isolated (child) | ❌ No | May maintain an internal list (debugging) — not visible |
| `planner`, `pathfinder`, `onboarder`, `debugger`, `designer`, `auditor-subagent` | Invoked via `task` from `orchestrator` or `conductor` | Isolated (child) | ❌ No | No todowrite list |
| `developer`, `reviewer`, `documentarian` | Invoked via `task` from `orchestrator-dev` or `conductor` | Isolated (child; grandchild in `libre`) | ❌ No | No todowrite list |

An agent that is the entry agent of its workflow (`auditor` in `audit`, `debugger` in `debug` and `incident`,
`pathfinder` in `spike`, `developer` in `quick`…) runs in the root session: its todo list, if it keeps one, is
visible. A child session of a fan-out (`ticket`, `sweep-task`) is a full oh session: its todo list is that of its
own root session, not the parent's.

---

## Responsibility Rule

> **The root session agent (the workflow entry agent) is always
> the sole owner of the visible todo list.**

This rule implies:

1. **The orchestrator** (`libre`) maintains a list with **1 task per phase of the feature** (planning, UX spec,
   UI spec, audit, implementation): the granularity is deliberately high, it follows the phases, not the tickets.
   It updates it at each phase change, when it reads the envelope of the agent it launched.

2. **Entry-agent orchestrator-dev** maintains its list with **dynamic phase labels**
   to reflect what is actually happening in the currently active developer subagent.

3. **Subagent orchestrator-dev** may maintain an internal list for session debugging,
   but this list is invisible and does not replace the orchestrator's one.

---

## Update Rules — Orchestrator (`libre`)

When the orchestrator receives the envelope of an agent it launched, it updates its list **before displaying the
envelope and before calling the `question` tool**:

| Envelope received | Status | Todowrite action |
|-------------------|--------|-----------------|
| `## Retour vers orchestrator` from the planner or the pathfinder | `découpage-à-valider`, `planification-complète`, `direct` | Planning → `completed`, next phase → `in_progress` |
| `## Retour vers orchestrator` from the designer | `spec-complète` | UX or UI spec → `completed` |
| `## Retour vers orchestrator` from orchestrator-dev | `en-attente` | None (blocking questions relayed, resumption with `task_id`) |
| `## Retour vers orchestrator` from orchestrator-dev | `succès`, `partiel`, `bloqué` | Implementation → `completed` (or left `in_progress` with the blocking point) |

**Example — planning done, implementation in progress:**

```
todowrite({
  todos: [
    { content: "Planning", status: "completed", priority: "high" },
    { content: "UI spec", status: "completed", priority: "high" },
    { content: "Implementation", status: "in_progress", priority: "high" }
  ]
})
```

---

## Update Rules — Entry-agent Orchestrator-dev

When `orchestrator-dev` is invoked directly, update the task label at each key phase
so the user knows where the implementation stands:

| Step | Label | Status |
|------|-------|--------|
| `cp-plan` (initialization, `implement-spec`) or start | `#bd-12 — <title>` | `pending` |
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
- **Permissions**: `todowrite` is explicitly allowed for `conductor`, `orchestrator`, `orchestrator-dev` and `release-manager`,
  and denied for `brief-enricher`.

---

## What Does Not Change

- The fundamental rules of the coordinator posture apply without exception: exactly
  1 task `in_progress` at a time, real-time updates, full list on every call.
- Agents `developer`, `reviewer`, `documentarian` do not use
  `todowrite` when invoked as subagents: their sessions are invisible.
- The return envelope (`## Retour vers <caller>`, blocking questions with status `en-attente`) and the report filed
  with `workflow_report` remain the only communication channel between isolated sessions.

---

## References

- [`skills/posture/coordinator-posture.md`](../../skills/posture/coordinator-posture.md) — Usage rules (phase suffixes are in `orchestrator-dev-ticket-workflow`)
- [`skills/orchestrator/orchestrator-protocol.md`](../../skills/orchestrator/orchestrator-protocol.md) — List of the phases of the orchestrator (`libre`)
- [`skills/orchestrator/orchestrator-dev-protocol.md`](../../skills/orchestrator/orchestrator-dev-protocol.md) — Dynamic labels for entry-agent orchestrator-dev
- [`docs/architecture/task-delegation.en.md`](task-delegation.en.md) — Delegation via `task`, session isolation, `task_id`
