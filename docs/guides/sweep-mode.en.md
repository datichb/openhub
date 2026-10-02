> [Lire en francais](sweep-mode.fr.md)

# Sweep Mode — Guide

## Overview

Sweep mode is a goal-driven execution model: you describe a high-level objective, and openhub decomposes it into independent sub-tasks, executes them in parallel, and optionally verifies the results.

```
Goal -> Decompose -> Execute (parallel) -> Collect -> Merge -> Verify
```

---

## Quick Start

```bash
oh start --sweep "Migrate all deprecated API calls to v2" --sweep-strategy llm
```

---

## Decomposition Strategies

| Strategy | Description | Best for |
|----------|-------------|----------|
| `manual` | User-provided task list via `--sweep-tasks` | Known, well-defined sub-tasks |
| `by-file` | Groups files matching glob patterns | File-level transformations (headers, formatting) |
| `by-package` | One task per Go package (via `go list`) | Package-level refactoring |
| `llm` | LLM planner decomposes the goal into tasks | Complex goals requiring analysis |

---

## Command Reference

```bash
oh start --sweep "goal" --sweep-strategy <strategy> [options]
```

| Flag | Description | Default |
|------|-------------|---------|
| `--sweep` | High-level goal description (required) | — |
| `--sweep-strategy` | Decomposition strategy (required) | — |
| `--sweep-tasks` | Manual task list, comma-separated (requires `manual` strategy) | — |
| `--sweep-include` | Glob patterns to include | — |
| `--sweep-exclude` | Glob patterns to exclude | — |
| `--sweep-verify` | Post-sweep verification: `none`, `tests`, `lint`, `build`, `all`, `custom` | `none` |
| `--sweep-verify-cmd` | Custom verification command (requires `custom` verify) | — |
| `--sweep-dry-run` | Show task plan without executing | false |
| `--sweep-branch-prefix` | Branch name prefix | `sweep/` |
| `--max-sessions` | Max concurrent sessions | Config default |
| `--project` / `-p` | Project ID | Auto-detected |

---

## Pipeline

### 1. Decomposition (Split)

The splitter analyzes the goal and creates a list of independent sub-tasks. Each task includes:
- A description of what to do
- A scope (list of files or packages to modify)
- An isolation constraint (the agent is told to only modify files in its assigned scope)

### 2. Single-Task Fast Path

When decomposition yields exactly **1 task**, sweep mode skips the full parallel infrastructure and runs headless directly. This saves ~5 seconds of overhead.

### 3. Multi-Task Execution

For 2+ tasks, a full parallel coordinator is created:
- One worktree per task
- Same TUI monitor as parallel mode (navigate, attach, refresh)
- Same conflict detection and notification system

### 4. Collection and Merge

After execution, results are collected and branches are merged back:
- Auto-merge proposed for clean merges
- Manual confirmation for conflicting merges
- Configurable merge policy

### 5. Verification

Post-sweep verification runs on the merged result:

| Strategy | Command | Stops on failure |
|----------|---------|:---:|
| `none` | (skipped) | — |
| `tests` | `go test ./...` | Yes |
| `lint` | `golangci-lint run ./...` | Yes |
| `build` | `go build ./...` | Yes |
| `all` | build + tests + lint (in order) | Yes |
| `custom` | User-provided command | Yes |

---

## Dry Run

Preview the decomposition plan without executing:

```bash
oh start --sweep "Add error handling to all HTTP handlers" \
  --sweep-strategy by-package \
  --sweep-include "cli/internal/mcp/*" \
  --sweep-dry-run
```

This shows the task list, file assignments, and estimated scope per task.

---

## Examples

### By file — Update copyright headers

```bash
oh start --sweep "Update copyright year to 2026 in all Go files" \
  --sweep-strategy by-file \
  --sweep-include "**/*.go" \
  --sweep-verify build
```

### By package — Add observability

```bash
oh start --sweep "Add structured logging to all MCP servers" \
  --sweep-strategy by-package \
  --sweep-include "cli/internal/mcp/*" \
  --sweep-verify tests
```

### LLM planner — Complex migration

```bash
oh start --sweep "Migrate from log.Printf to slog structured logging" \
  --sweep-strategy llm \
  --sweep-verify all
```

### Manual — Known tasks

```bash
oh start --sweep "Fix lint warnings" \
  --sweep-strategy manual \
  --sweep-tasks "fix-bodyclose,fix-nilerr,fix-misspell" \
  --sweep-verify lint
```

---

## Scope Isolation

Each sub-task agent receives a prompt that enforces scope boundaries:
- The agent is told which files it is allowed to modify
- Modifications outside the assigned scope are flagged as violations
- This prevents overlapping changes between parallel sessions

---

## Troubleshooting

### No tasks generated

```
Sweep decomposition produced 0 tasks
```

- For `by-file`: verify the `--sweep-include` glob matches existing files
- For `by-package`: verify `go list` can find packages matching the include pattern
- For `llm`: rephrase the goal with more specific instructions

### Verification failed

```
Sweep verification failed: tests
```

The merged result has test failures. Fix manually or run another sweep targeting the failing tests.

### Merge conflicts between tasks

Two sub-tasks modified the same file. The merge view will show the conflict. Consider:
- Reducing task granularity (fewer, larger tasks)
- Using `--sweep-exclude` to prevent overlap
- Switching to `manual` strategy with explicit file assignments

---

## Resources

- [Parallel Mode Guide](parallel-mode.en.md)
- [CLI Reference](../reference/cli.en.md)
