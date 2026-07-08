# ADR-024: Team State Repository

## Status

Accepted

## Date

2026-07-07

## Context

The opencode-hub was designed as a single-developer orchestration system. As teams of 3-5 developers adopt it, several collaboration gaps emerged:

- No visibility on who works on what ticket
- No way to share cross-project knowledge accessible to AI agents
- No notification mechanism when sessions complete or reviews are ready
- Agent sessions are isolated — no access to team context

We needed a shared state mechanism that:
1. Does NOT pollute the project repository with meta-coordination files
2. Does NOT live in the hub (which is for agents/skills, not runtime state)
3. Is accessible to all team members
4. Is accessible to AI agents during their sessions
5. Supports multi-project teams
6. Remains Git-native (no additional infrastructure)

## Decision

Introduce a **dedicated Git repository** (`team-state`) managed transparently by the `oh` CLI. This repo stores:

- `members.toml` — team member registry
- `config.toml` — notification settings (Mattermost webhook)
- `projects/<name>/claims/` — ticket reservations (TOML files)
- `projects/<name>/events/` — activity journal (monthly JSONL)
- `wiki/` — cross-project knowledge base
- `wiki/.pending/` — wiki proposals awaiting human review
- `reports/` — generated team reports

The repo is cloned to `~/.oh/team-state/` and synchronized automatically (pull before reads, push after writes).

AI agents access team data via a **MCP server** (`team-mcp`) that reads from the local clone. This avoids filesystem access issues and provides a clean tool-based interface.

## Alternatives Considered

| Alternative | Rejected Because |
|---|---|
| Files in project repo | Pollutes code history, merge conflicts on non-functional files |
| Files in hub repo | Hub is for canonical definitions, not runtime state |
| Git notes | Limited API, complex sharing, not for structured data |
| GitLab API as backend | Creates hard dependency, latency for every operation |
| SQLite shared via NFS | Requires infrastructure, not Git-native |
| Orphan branch in project | Exotic workflow, not visible in normal operations |

## Consequences

### Positive
- Zero infrastructure: just a Git repo on GitLab/GitHub
- Full audit trail: Git history shows who did what when
- Offline-capable: works with stale data if network is unavailable
- Multi-project: one repo serves all projects
- AI-accessible: MCP server provides clean read/write interface

### Negative
- Eventual consistency: data is only fresh after `git pull`
- Conflict resolution needed on concurrent writes (mitigated by pull-rebase-retry)
- One additional repo to manage (mitigated: fully transparent to user)
- File-per-claim can generate many small files (acceptable for 3-5 devs)

### Neutral
- Members must provide repo URL during `oh team init`
- Repo must be pre-created manually (documented, one-time setup)

## Implementation

- Package: `cli/internal/teamstate/`
- MCP server: `cli/internal/mcp/team/`
- Notifications: `cli/internal/notify/`
- CLI commands: `oh team init|status|activity`, `oh claim|release`
- Skills: `skills/shared/team-awareness.md`, `skills/orchestrator/team-coordination.md`
