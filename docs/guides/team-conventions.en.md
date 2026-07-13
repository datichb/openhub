# Team Conventions Guide

## Principle

Team conventions are shared rules that apply to code, branches, and commits. They are automatically verified by AI agents and by the `oh conventions check` command.

## Where to document conventions

### Project-specific conventions

Location: `docs/wiki/technical/conventions.md` in the project repo.

This file is read directly by agents during their session (filesystem access).

### Cross-project conventions (team)

Location: `conventions` page in the team-state wiki.

Accessible to agents via `team_wiki_read("conventions")`.

## Expected format

Conventions must include machine-readable patterns for automatic verification.

### Branch pattern

```markdown
## Branches

branch_pattern = `^(feat|fix|chore|refactor|docs)/[A-Z]+-\d+-.+`

Convention: `<type>/<TICKET-ID>-<description-slug>`

Examples:
- `feat/SRU-142-user-authentication`
- `fix/SRU-99-token-refresh`
- `chore/SRU-200-cleanup-deps`
```

### Commit format

```markdown
## Commits

We use Conventional Commits.

commit_pattern = `^(feat|fix|docs|style|refactor|perf|test|chore|ci|build|revert)(\(.+\))?!?:\s.+`

Examples:
- `feat(auth): implement JWT middleware`
- `fix(api): handle null response body`
- `docs: update API reference`
```

If the file simply mentions "Conventional Commits" without an explicit pattern, the checker automatically applies the standard pattern.

## Verification

### Standalone command

```bash
oh conventions check
```

Displays:
- Current branch status vs the pattern
- Recent commits status vs the format
- Claim status (if team enabled)

### Agent verification

The orchestrator-dev automatically checks conventions:
- **Before creating a branch**: applies the naming pattern
- **Before each commit**: respects the documented format
- **Non-blocking warnings**: the agent informs but does not block

## Enforcement

Enforcement level is **medium**:
- Warnings in the terminal and in the agent session
- No hard blocking (dev can ignore a warning)
- No imposed pre-commit hook (optional)

> **Note**: For configurable enforcement (blocking or warning per rule),
> use **Team Policies** described below. Conventions remain the reference
> documentation; policies are their automated enforcement.

## Team Policies — Configurable Enforcement

Team Policies extend conventions with a configurable enforcement mechanism:
each rule can be a **warning** (informational) or **refuse** (blocking).
They are stored in the team-state repo and apply to all members and projects.

### `policies.toml` file

Create this file at the root of the team-state repo:

```toml
# Structured rules — standard categories

[policies.branch_naming]
type = "regex"
rule = "^(feat|fix|hotfix|chore|refactor)/[a-z0-9-]+"
enforcement = "refuse"
message = "Branch must follow pattern: feat/xxx, fix/xxx, etc."

[policies.commit_format]
type = "regex"
rule = "^(feat|fix|docs|style|refactor|test|chore)(\\(.+\\))?: .+"
enforcement = "refuse"
message = "Commit must follow Conventional Commits"

[policies.review_required]
type = "boolean"
enabled = true
enforcement = "refuse"
message = "Human review required before merge"

[policies.tests_required]
type = "boolean"
enabled = true
enforcement = "warn"
message = "Tests should pass before review"

[policies.max_ticket_wip]
type = "limit"
max = 2
enforcement = "warn"
message = "Limit WIP to 2 tickets per member"

# Custom rules — add on demand

[policies.custom_no_console_log]
type = "forbidden_pattern"
patterns = ["console.log", "console.warn"]
scope = "diff_only"
enforcement = "warn"
message = "Remove console.log before commit"
```

### Rule types

| Type | Usage | Parameters |
|------|-------|-----------|
| `regex` | Validates a value against a pattern | `rule` (regex) |
| `boolean` | Enables/disables a check | `enabled` |
| `limit` | Imposes a maximum value | `max`, `unit` (optional) |
| `forbidden_pattern` | Forbids patterns in code | `patterns` (list), `scope` |

### Scopes for `forbidden_pattern`

| Scope | Description |
|-------|-------------|
| `diff_only` | Only added lines in the diff |
| `modified_files` | Full content of modified files |
| `all_files` | All project files |

### Per-project overrides

To make a policy stricter on a specific project, create
`projects/<project>/policies-override.toml`:

```toml
# Only enforcement can be hardened (warn → refuse)
[policies.tests_required]
enforcement = "refuse"
message = "Tests MUST pass on T-SRU"
```

> **Important**: overrides can only make stricter. A global `refuse` policy
> cannot be softened to `warn` by an override.

### CLI commands

```bash
# Show active policies (global + project overrides)
oh policies list
oh policies list --project T-SRU

# Check policies against current state
oh policies check --branch feat/my-feature --commit "feat: add login"

# Add a custom policy (interactive)
oh policies add
```

### Dual enforcement (CLI + Agents)

Enforcement works at two levels:

| Level | Who | When | Behavior |
|-------|-----|------|----------|
| **CLI (hard)** | The `oh` binary | `oh claim`, `oh start`, `oh release` | Blocks or warns per policy |
| **Agent (soft)** | AI agents via skill | During session | Checks before each relevant action |

#### Automatic CLI checks

| Command | Policies checked |
|---------|-----------------|
| `oh claim <ticket>` | `max_ticket_wip` |
| `oh start` (branch creation) | `branch_naming` |
| `oh release <ticket>` | `review_required`, `tests_required` |

#### Agent checks

Each agent only checks policies relevant to its actions.
See the `team-policies-enforcement` skill for the detailed matrix.

### Relationship with conventions

**Conventions** (`docs/wiki/technical/conventions.md` + wiki) remain the
human-readable reference documentation. **Policies** are their machine-enforceable
formalization.

Recommendation: document your conventions normally, then formalize those that
must be blocking in `policies.toml`.
