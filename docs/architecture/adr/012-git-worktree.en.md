> [Lire en francais](012-git-worktree.fr.md)

# ADR-012 — Git Worktrees for parallel work isolation

**Date:** 2026-06-04
**Status:** Accepted *(updated 2026-07-22)* — **Evolved by [ADR-043](./043-session-bundle-deploy-removal.en.md)** and **[ADR-047](./047-session-interaction-daemon.en.md)**
**Authors:** openhub

---

## Context

The hub has a conditional parallelism mechanism in `auto` mode: `orchestrator-dev` can process up to 3 tickets simultaneously via the `task` tool (ADR-006). However, this parallelism is **purely semantic** — all `developer-*` agents share the same working tree. Isolation relies on conventions (distinct domains, dependency graph) but remains vulnerable to unexpected filesystem conflicts.

Additionally, developers sometimes want to work on a new feature without disrupting their current in-progress branch — which requires an intrusive `git stash` / `git checkout`.

---

## Decision

Introduce **`git worktree`** support in openhub along two complementary axes:

### Axis 1 — Auto mode isolation (complement)

When `worktree.enabled = true` in the project config, `orchestrator-dev` step 1b uses `git worktree add` instead of `git checkout -b`. Each ticket receives a dedicated directory `.worktrees/<slug>/`. The `developer-*` agents work in their isolated directory — with zero risk of filesystem conflicts between parallel sessions.

The existing auto mode remains unchanged for projects where `worktree.enabled` is absent or `disabled`.

### Axis 2 — Free parallel sessions (new mode)

Two new commands:
- `oh start --worktree [BRANCH]`: isolated OpenCode session in a new worktree on a given branch, with no mandatory Beads link. Allows developing a feature in parallel with the current branch.
- `oh start --parallel`: launches `orchestrator-dev` in a dedicated worktree with a batch of `ai-delegated` tickets. The full workflow is preserved (implementation -> QA -> review -> CP-2).

### Axis 3 — Lifecycle management

- Worktrees are stored in `.worktrees/<slug>/` at the project root
- `.worktrees/` is automatically added to `.git/info/exclude` (never to `.gitignore`)
- An optional auto-cleanup (`worktree.auto_cleanup = true`) removes worktrees whose branch is merged at the start of any session
- The `oh worktree` command exposes the full lifecycle (list, create, remove, cleanup, status)

---

## Consequences

### Positive

- Real filesystem isolation between parallel agents -> zero file conflicts
- Multi-feature development without intrusive stash/checkout
- Auto mode without worktrees remains identical -> full backward compatibility
- Auto-cleanup keeps the working directory clean

### Negative / risks

- Dependency on `git worktree` (available since git 2.5, March 2015 — negligible risk)
- Proliferation of `.worktrees/` directories if auto-cleanup is disabled
- Worktrees share the main repository's git index — operations on sensitive branches (merge, rebase) must be done with caution

### Impact on agent permissions

`orchestrator-dev` receives four additional bash permissions:
```yaml
"git worktree add *": allow
"git worktree remove *": allow
"git worktree list": allow
"git worktree prune": allow
```

These permissions are limited to worktree operations — `git push`, `git merge` remain forbidden (least privilege principle preserved).

### Impact on session-state

The `worktree_path` field is added to `tickets[]` and `current_ticket` entries in `session-state.json` to allow the dashboard to display it. This field is optional (`null` if no worktree).

---

## Alternatives considered

| Alternative | Rejected because |
|-------------|-----------------|
| Temporary directories (`/tmp/`) | Outside the git repo — impossible to commit from there |
| Git submodules | Excessive complexity, lifecycle coupled to parent repo |
| Automatic `git stash` | Intrusive, loses current work context |
| Branches only (without worktrees) | Does not solve filesystem isolation |

---

## Files affected

| File | Change |
|------|--------|
| `scripts/lib/worktree.sh` | New lib — all worktree operations |
| `scripts/cmd-worktree.sh` | New `oh worktree` command |
| `scripts/cmd-start.sh` | `--parallel` and `--worktree` flags |
| `scripts/cmd-init.sh` | Interactive worktree config at init |
| `scripts/adapters/opencode.adapter.sh` | `.worktrees/` in `.git/info/exclude` |
| `scripts/lib/project.sh` | `get_project_worktree_*` getters |
| `scripts/lib/session-state.sh` | `worktree_path` field |
| `agents/planning/orchestrator-dev.md` | `git worktree *` permissions |
| `skills/orchestrator/orchestrator-dev-protocol.md` | Conditional step 1b |
| `skills/orchestrator/orchestrator-workflow-modes.md` | Worktrees documentation |
| `oc.sh` | `worktree)` routing |

---

## Addendum — July 2026 update

The shipped Go implementation differs on several points from the initial design described above. This addendum documents the actual state of the code.

### A. Sibling directories instead of `.worktrees/<slug>/`

**Initial design:** worktrees were stored in `.worktrees/<slug>/` at the project root, with automatic addition of `.worktrees/` to `.git/info/exclude`.

**Actual implementation:** worktrees are created as **sibling directories** of the main project:

```
/home/user/myrepo/              <- main project
/home/user/myrepo-feat-bd42/    <- worktree for feat/BD-42
/home/user/myrepo-fix-auth/     <- worktree for fix/auth
```

The path is computed by `SiblingPath(projectPath, branch)` in `cli/internal/worktree/worktree.go`. This approach makes `.git/info/exclude` manipulation unnecessary (sibling directories are outside the project tree), so the `EnsureExclude` function was removed.

**Rationale:** sibling directories never appear in `git status` of the main project, eliminating the need for exclude management. They remain accessible via predictable relative paths.

### B. Go implementation instead of shell scripts

**Initial design:** the implementation relied on shell scripts (`scripts/lib/worktree.sh`, etc.).

**Actual implementation:** all logic is in the Go package `cli/internal/worktree/worktree.go`. The shell files referenced in the original "Files affected" table no longer exist.

**Actual files:**

| File | Role |
|------|------|
| `cli/internal/worktree/worktree.go` | Core package — all operations |
| `cli/internal/worktree/worktree_test.go` | Unit and integration tests |
| `cli/cmd/worktree.go` | `oh worktree` subcommands |
| `cli/cmd/start.go` | `--worktree` flags for `oh start` |
| `cli/cmd/start_parallel_mode.go` | `oh start --parallel` mode |
| `cli/internal/parallel/coordinator.go` | Sequential worktree creation |
| `cli/internal/parallel/config.go` | Parallel config (incl. `cleanup_completed_worktrees`) |
| `cli/internal/tui/v2/views/worktree_view.go` | TUI management view |
| `cli/internal/config/config.go` | `WorktreeConfig` struct |

### C. `CleanupMerged`: safe-by-default

Worktree cleanup is now **safe by default**: without the `-f` flag, worktrees with uncommitted changes are skipped rather than destroyed. The signature evolved:

```go
// Before
func CleanupMerged(projectPath, baseBranch string) ([]string, error)

// After
func CleanupMerged(projectPath, baseBranch string, force bool) (CleanupResult, error)
// CleanupResult.Removed: deleted branches
// CleanupResult.Skipped: merged but preserved branches (dirty, force=false)
```

Auto-cleanup from `oh start` uses `force=false`. The `oh worktree cleanup -f` command uses `force=true`.

### D. Remote existence check before creation

`ResolveOrCreate` now checks whether the branch exists on the remote (`origin`) before attempting to create it. If it exists, a `git fetch` is performed and the worktree points to the remote branch — enabling resumption of work on an already-pushed branch.

```
1. Worktree already registered locally -> reuse
2. Branch exists on origin -> fetch + checkout
3. Branch exists locally -> checkout
4. None of the above -> create new branch
```

### E. Configurable branch naming convention

The branch naming pattern (previously hardcoded to `feat/%s` in the parallel coordinator) is now:

1. Configurable via `[worktree].branch_pattern` in `hub.toml`
2. Auto-detected at onboarding (`oh init`) by heuristic on existing branches
3. Managed by `worktree.BranchName(pattern, ticketID)` which validates that the pattern contains `%s`

### F. Worktree cleanup after parallel session

The `Coordinator` now supports the `cleanup_completed_worktrees` option in its config: when enabled, worktrees for **successfully completed** sessions are deleted at shutdown. Failed sessions are intentionally preserved for post-mortem inspection.
