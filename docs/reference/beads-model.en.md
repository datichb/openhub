> 🇫🇷 [Lire en français](beads-model.fr.md)

# Beads Data Model Reference

Canonical document describing the data model used by `bd` (Beads CLI)
in the hub context. Every skill, agent or hub script must conform
to this model.

---

## Statuses (6)

A ticket passes through a subset of these statuses during its lifecycle.

| Status | Terminal | Description | `bd` command |
|--------|----------|-------------|--------------|
| `open` | no | Created, not yet taken up | Default state at creation |
| `in_progress` | no | Being implemented | `bd update <ID> --claim` (atomic: assigns + sets `in_progress`) |
| `review` | no | Implementation complete, awaiting human reviewer validation. The reviewer either closes the ticket or sends it back to `in_progress` with feedback. | `bd update <ID> -s review` |
| `blocked` | no | Blocked by a dependency or external factor | `bd update <ID> -s blocked` |
| `cancelled` | **yes** | Abandoned — will not be implemented | `bd update <ID> -s cancelled` |
| `closed` | **yes** | Completed and validated | `bd close <ID>` |

### Allowed transitions

```
open ──────────→ in_progress ──→ review ──→ closed
  │                  │              │
  │                  ↓              ↓
  │               blocked      in_progress  (rejection → back to dev)
  │                  │
  │                  ↓
  │              in_progress  (unblocking)
  │
  ↓
cancelled
```

**Rules:**

- **No reopening.** A `closed` or `cancelled` ticket is never reopened.
  If additional work is needed, create a new ticket.
- **`cancelled` does not use `bd close`** — use `bd update <ID> -s cancelled`.
- **`review`** is a custom status natively accepted by `bd`.
  A ticket enters `review` when the developer considers their implementation complete.
  The **human reviewer** (or reviewer agent) then either:
  - **Accepts** → `bd close <ID> --reason "..."` — ticket moves to `closed`
  - **Rejects** → leaves feedback via `bd comments add <ID> "Feedback: ..."`, then
    `bd update <ID> -s in_progress` — ticket returns to the developer for a correction cycle
- **`blocked`** can only occur from `in_progress`.
  Unblocking moves back to `in_progress`.

---

## Types (5)

Each ticket has exactly one type, defined at creation.

| Type | `bd create` flag | Description |
|------|-----------------|-------------|
| `epic` | `-t epic` | Ticket container — does not carry direct implementation |
| `feature` | `-t feature` | New functionality |
| `task` | `-t task` | Technical task (refactoring, migration, configuration) |
| `bug` | `-t bug` | Bug fix |
| `chore` | `-t chore` | Maintenance, CI/CD, documentation, cleanup |

> **`decision` is not a type.** Architectural decisions (ADRs) are
> carried by a `-t task` ticket with the appropriate label if needed.

---

## Priorities (4)

P0–P3 scale. The `bd` format accepts `-p <N>` with N = 0 to 3.

| Priority | Flag | Semantics |
|----------|------|-----------|
| **P0** | `-p 0` | Critical — blocking for production or for all other tickets |
| **P1** | `-p 1` | High — critical path of the feature, main business value |
| **P2** | `-p 2` | Normal (default) — functional enrichment, user comfort |
| **P3** | `-p 3` | Low — nice-to-have, backlog, optional improvements |

> **`--priority high` / `--priority medium` are not valid syntaxes.**
> Always use the numeric form: `-p 0`, `-p 1`, `-p 2`, `-p 3`.

> **P4 does not exist** in this model. Far-backlog items stay at P3
> or are simply not created.

---

## System labels (5)

Labels reserved by the hub. They must not be repurposed for another use.

| Label | Set by | Description |
|-------|--------|-------------|
| `ai-delegated` | Human only | Marks a ticket as delegated to an AI agent. The agent **never** sets this label itself unless explicitly agreed. |
| `needs-decision` | Agent or human | The ticket is blocked by a human decision (technical choice, business arbitration). |
| `needs-clarification` | Agent or human | The ticket lacks information — the description or acceptance criteria are insufficient. |
| `from-diagnostic` | Debugger agent | The ticket was created following a bug diagnostic (debugger report). |
| `split-from-<ID>` | Planner agent | The ticket results from splitting an oversized ticket. `<ID>` is the identifier of the original ticket. |

### Label commands

```bash
# Create a project-level label (register it for use in the project)
bd label create <label>

# Add a label to a ticket
bd label add <ID> <label>
# or
bd update <ID> --add-label <label>

# Remove a label from a ticket
bd update <ID> --remove-label <label>

# List available labels in the project
bd label list-all
```

> **Automatic import at init** — when `oh beads init` is run and a tracker (GitLab or Jira) is already configured, labels from the remote tracker are automatically fetched and registered in Beads via `bd label create`. Labels defined in `projects.md` are always registered first; remote labels are merged on top (union). `projects.md` is never modified automatically.

---

## Ticket fields (10)

| Field | Creation / update flag | Description |
|-------|----------------------|-------------|
| `title` | Positional arg of `bd create` | Short, actionable title |
| `description` | `--description` | Detailed description in natural language |
| `acceptance` | `--acceptance` | Observable and verifiable acceptance criteria |
| `notes` | `--notes` | Technical context, risks, points of attention |
| `design` | `--design` | Design notes (mockups, UI/UX specs) |
| `estimate` | `--estimate <minutes>` | Estimate in minutes (60 = 1h, 480 = 1 day) |
| `external-ref` | `--external-ref <ref>` | External tracker reference (e.g. `jira-PROJ-42`, `gitlab-17`) |
| `assignee` | `-a <name>` or `--claim` | Ticket owner. `--claim` is atomic (assigns + `in_progress`). |
| `close-reason` | `--reason "..."` on `bd close` | Closure reason — commit, PR, explanation |
| `labels` | `--add-label` / `--remove-label` / `bd label add` | Labels attached to the ticket |

---

## Ticket relations (5 types)

| Relation | Command | Effect |
|----------|---------|--------|
| **Parent / child** | `bd create "..." --parent <EPIC_ID>` | Epic → tickets hierarchy. `bd children <ID>` to list. |
| **Dependency** | `bd dep add <ID> <DEP_ID>` | `<ID>` is blocked until `<DEP_ID>` is closed. `bd ready` respects these blockers. |
| **Duplication** | `bd duplicate <ID> --of <CANONICAL>` | Marks `<ID>` as a duplicate of `<CANONICAL>` (auto-closes `<ID>`). |
| **Supersession** | `bd supersede <ID> --with <NEW>` | `<ID>` is replaced by `<NEW>` (auto-closes `<ID>`). |
| **Free relation** | `bd dep relate <ID> <OTHER>` | Informative link without blocking. `bd dep unrelate` to remove. |

### Dependency commands

```bash
# Add a dependency
bd dep add <ID> <DEP_ID>

# Remove a dependency
bd dep remove <ID> <DEP_ID>

# List dependencies of a ticket
bd dep list <ID>

# Full dependency tree
bd dep tree

# Detect cycles
bd dep cycles
```

---

## Ready tickets — `bd ready`

`bd ready` returns tickets that are **unblocked** (all dependencies are closed)
and **non-terminal** (neither `closed` nor `cancelled`).

```bash
# Ready-to-work tickets
bd ready --json

# Ready tickets with a specific label
bd ready --label ai-delegated --json
```

> **`bd ready` is the recommended command.**
> It applies a more complete blocker-aware semantics than the `--ready` filter of `bd list`.

---

## Comments

```bash
# Add a comment to a ticket
bd comments add <ID> "Comment text"
```

Comments are used to trace decisions, blockers and exchanges without modifying
the ticket's description or notes.

---

## Full lifecycle — typical workflow

```
 ┌─────────────────────────────────────────────────────────────┐
 │  PLANNING (planner agent / human)                           │
 │                                                             │
 │  1. bd create "Title" -t feature -p 1 --parent $EPIC --json│
 │  2. bd update $ID --description "..." --acceptance "..."    │
 │  3. bd label add $ID ai-delegated  (human only)             │
 └──────────────────────────┬──────────────────────────────────┘
                            ↓
 ┌─────────────────────────────────────────────────────────────┐
 │  EXECUTION (developer agent)                                │
 │                                                             │
 │  4. bd ready --label ai-delegated --json                    │
 │  5. bd show $ID                                             │
 │  6. bd update $ID --claim                  → in_progress    │
 │  7. [implement, test, commit]                               │
 │  8. bd update $ID -s review                → review         │
 └──────────────────────────┬──────────────────────────────────┘
                            ↓
 ┌─────────────────────────────────────────────────────────────┐
  │  REVIEW (reviewer agent / human)                            │
  │                                                             │
  │  9a. Accepted  → bd close $ID --reason "..." → closed       │
  │  9b. Rejected  → bd comments add $ID "Feedback: ..."        │
  │                  bd update $ID -s in_progress               │
  │                  → back to step 7 (correction cycle)        │
 └──────────────────────────┬──────────────────────────────────┘
                            ↓
 ┌─────────────────────────────────────────────────────────────┐
 │  BLOCKING (if dependency or decision needed)                │
 │                                                             │
 │  bd update $ID -s blocked                                   │
 │  bd comments add $ID "Blocked by: <reason>"                 │
 │  bd update $ID --add-label needs-decision  (if applicable)  │
 │  ... resolution ...                                         │
 │  bd update $ID -s in_progress              → back to dev    │
 └─────────────────────────────────────────────────────────────┘

 ┌─────────────────────────────────────────────────────────────┐
 │  CANCELLATION (human only)                                  │
 │                                                             │
 │  bd update $ID -s cancelled                                 │
 │  bd comments add $ID "Reason: ..."                          │
 └─────────────────────────────────────────────────────────────┘
```

---

## External tracker synchronisation

The hub supports two trackers: **Jira** and **GitLab**.

| Command | Description |
|---------|-------------|
| `oh beads tracker setup <PROJECT_ID>` | Interactive credential configuration |
| `oh beads tracker set-sync-mode <PROJECT_ID> [mode]` | Set default sync direction (`bidirectional` \| `pull-only` \| `push-only`) |
| `oh beads sync <PROJECT_ID>` | Synchronise using the configured `Sync mode` (default: bidirectional) |
| `oh beads sync <PROJECT_ID> pull` | Import only from the tracker (overrides `Sync mode`) |
| `oh beads sync <PROJECT_ID> push` | Export only to the tracker (overrides `Sync mode`) |
| `oh beads sync <PROJECT_ID> --dry-run` | Simulation without writing |
| `oh beads tracker status <PROJECT_ID>` | Tracker connection status |

#### Local exclusion of `.beads/`

`oh beads init` automatically adds `.beads/` to the target project's `.git/info/exclude` file.
This file is local to the machine and never versioned — tracker credentials (GitLab token, Jira token) stored by `bd config set` are never exposed in the shared repository.

> This behaviour mirrors the exclusion of `opencode.json` and `.opencode/` applied by `oh init` / `oh deploy`.

#### GitLab configuration — `gitlab.project_id`

During `tracker setup`, the **GitLab project ID or path** field accepts three formats:

| Format | Example | Behaviour |
|--------|---------|-----------|
| Numeric ID | `12345` | Used as-is |
| Namespace/project path | `my-group/my-project` | Used as-is |
| Full URL | `https://gitlab.com/my-group/my-project` | Path extracted automatically with a warning |

> **Tip**: prefer the numeric ID or the `namespace/project` path.
> The numeric ID is visible in GitLab under **Settings → General** (at the top of the page).

After setup, the connection is tested automatically via `bd gitlab status`.
If it fails, the configured values are displayed to help diagnose the issue.

> **`Sync mode` field** — stored in `projects.md` under `- Sync mode : <bidirectional|pull-only|push-only>`.
> Default value when absent: `bidirectional`. A CLI subcommand (`pull`, `push`) always takes precedence over the configured mode.

### External references

```bash
# At creation
bd create "Title" --external-ref jira-PROJ-42 --json

# On an existing ticket
bd update <ID> --external-ref gitlab-17
```

Naming convention:
- Jira: `jira-<PROJECT>-<NUMBER>` (e.g. `jira-MYAPP-42`)
- GitLab: `gitlab-<NUMBER>` (e.g. `gitlab-17`)

---

## Summary of allowed `bd` commands

### Read

| Command | Description |
|---------|-------------|
| `bd list -s <status> --json` | List by status |
| `bd list -s <status> --label <label> --json` | List by status + label |
| `bd ready --json` | Ready tickets (blocker-aware) |
| `bd ready --label <label> --json` | Ready tickets with label |
| `bd show <ID>` | Full ticket detail |
| `bd children <EPIC_ID>` | Child tickets of an epic |
| `bd search <query>` | Text search |
| `bd count` | Number of tickets |
| `bd label list-all` | Available labels |
| `bd dep list <ID>` | Dependencies of a ticket |
| `bd dep tree` | Full tree |
| `bd dep cycles` | Cycle detection |

### Write

| Command | Description |
|---------|-------------|
| `bd create "Title" -t <type> -p <N> [options] --json` | Create a ticket |
| `bd update <ID> --claim` | Claim (assign + `in_progress`) |
| `bd update <ID> -s <status>` | Change status |
| `bd update <ID> --description / --acceptance / --notes / --design` | Update fields |
| `bd update <ID> -a <assignee>` | Assign |
| `bd update <ID> --add-label <label>` | Add a label |
| `bd update <ID> --remove-label <label>` | Remove a label |
| `bd label add <ID> <label>` | Add a label (alternative syntax) |
| `bd close <ID> [--reason "..."] [--suggest-next]` | Close a ticket |
| `bd dep add <ID> <DEP_ID>` | Add a dependency |
| `bd dep remove <ID> <DEP_ID>` | Remove a dependency |
| `bd dep relate <ID> <OTHER>` | Free relation (without blocking) |
| `bd dep unrelate <ID> <OTHER>` | Remove a free relation |
| `bd duplicate <ID> --of <CANONICAL>` | Mark as duplicate |
| `bd supersede <ID> --with <NEW>` | Mark as superseded |
| `bd comments add <ID> "..."` | Add a comment |
