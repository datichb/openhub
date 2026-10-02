> [Lire en francais](parallel-mode.fr.md)

# Parallel Mode — Guide

## Overview

Parallel mode runs multiple AI coding sessions concurrently, each in an isolated git worktree. It is designed for processing batches of independent tickets simultaneously, dramatically reducing total implementation time.

---

## When to Use

- **Batch of independent tickets** — Multiple bug fixes or features that touch different files
- **Migration tasks** — Apply changes across multiple packages or modules
- **Sprint velocity** — Process an entire sprint backlog in parallel

Parallel mode is NOT recommended when tickets have strong interdependencies or touch the same files extensively.

---

## Quick Start

```bash
oh start --parallel --tickets BD-42,BD-43,BD-44
```

This will:
1. Validate tickets and check budget admission
2. Create a sibling worktree directory for each ticket
3. Launch isolated `opencode serve` sessions (ports 4100+)
4. Open the TUI parallel monitor
5. After all sessions complete, offer interactive merge

---

## Command Reference

```bash
oh start --parallel --tickets BD-42,BD-43,BD-44 [options]
```

| Flag | Description | Default |
|------|-------------|---------|
| `--parallel` | Enable parallel mode | — |
| `--tickets` | Comma-separated ticket IDs (required) | — |
| `--max-sessions` | Max concurrent sessions (cap: 10) | Config default (5) |
| `--priority` | Priority ticket ID (merged first) | — |
| `--project` / `-p` | Project ID | Auto-detected |
| `--yes` / `-y` | Skip confirmation prompts | false |

---

## Workflow

### 1. Admission Control

Before launching, the coordinator validates:
- Total estimated minutes across all tickets does not exceed `MaxBudgetMinutes` (default: 180)
- Unestimated tickets use `DefaultTicketWeightMin` (default: 60 min) as fallback
- Number of tickets does not exceed `MaxSessions`

### 2. Worktree Creation

Each ticket gets a **sibling directory** next to the main project:

```
/home/user/myrepo/                  <- main project
/home/user/myrepo-feat-bd-42/       <- worktree for BD-42
/home/user/myrepo-feat-bd-43/       <- worktree for BD-43
```

Branch naming follows `[worktree].branch_pattern` from `hub.toml` (default: `feat/%s`).

### 3. Session Execution

Each session runs as an independent `opencode serve` subprocess:
- Isolated ports starting at 4100 (configurable via `port_range_start`)
- Full agent stack deployed per worktree
- Sessions cannot interfere with each other's filesystem

### 4. TUI Monitor

The parallel monitor shows all sessions live:

| Key | Action |
|-----|--------|
| Up/Down arrows | Navigate between sessions |
| Enter | Attach to a session (interactive) |
| `r` | Refresh status |
| `q` | Quit monitor (sessions continue in background) |

Information displayed per session: status, duration, files changed, conflict severity.

### 5. Conflict Detection

The coordinator tracks which files each session modifies in real-time:
- **Low** — Different files in the same directory
- **Medium** — Same file modified by multiple sessions
- **High** — Same lines modified by multiple sessions

Running sessions are notified when conflicts are detected.

### 6. Auto-Recovery

Failed sessions are automatically retried:
- Up to `max_retries` attempts (default: 2, cap: 5)
- Delay between retries: `retry_delay_seconds` (default: 5, cap: 60)
- Recovery prompts include context from prior failure

### 7. Merge

After all sessions complete, an interactive merge view allows:
- Sequential merging of completed branches into the base branch
- Diff preview before each merge
- Manual conflict resolution when needed
- Priority ticket (if specified) is merged first

---

## Team Configuration

Configure parallel mode defaults in the team-state `config.toml`:

```toml
[parallel]
max_sessions = 5                    # Max concurrent sessions (cap: 10)
max_budget_minutes = 180            # Budget cap (0 = disabled)
default_ticket_weight_min = 60      # Fallback weight for unestimated tickets
port_range_start = 4100             # Starting port for opencode serve
auto_merge_beads = true             # Propose auto-merge for Beads tickets
max_retries = 2                     # Retry cap (max: 5)
retry_delay_seconds = 5             # Delay between retries (max: 60)
```

---

## REST API and SSE

When running `oh serve`, the parallel execution state is exposed:
- **REST API** — Query session status programmatically
- **Server-Sent Events (SSE)** — Real-time status stream for dashboards

---

## Examples

**Process 3 tickets with priority:**
```bash
oh start --parallel --tickets BD-42,BD-43,BD-44 --priority BD-42
```

**Limit concurrent sessions:**
```bash
oh start --parallel --tickets BD-42,BD-43,BD-44,BD-45 --max-sessions 2
```

**Skip confirmation:**
```bash
oh start --parallel --tickets BD-42,BD-43 --yes
```

---

## Troubleshooting

### Port conflict

```
Error: port 4100 already in use
```

Change the starting port in team config (`port_range_start`) or kill the process occupying the port.

### Budget exceeded

```
Error: total estimated time (240 min) exceeds budget (180 min)
```

Reduce the number of tickets, increase `max_budget_minutes`, or set `max_budget_minutes = 0` to disable the check.

### Orphan worktrees

After a crash, worktrees may be left behind:
```bash
oh worktree list               # List all worktrees
oh worktree cleanup            # Remove merged worktrees
oh worktree cleanup -f         # Force remove (including dirty)
```

### Merge conflicts

If two sessions modified the same files, the merge view will flag the conflict. Resolve manually in the worktree before completing the merge.

---

## Resources

- [Worktree Documentation](../worktree.md)
- [ADR-012 — Git Worktrees](../architecture/adr/012-git-worktree.en.md)
- [ADR-036 — Platform Abstraction](../architecture/adr/036-platform-abstraction.en.md)
- [CLI Reference](../reference/cli.en.md)
