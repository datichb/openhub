> [Lire en francais](troubleshooting.fr.md)

# Troubleshooting Guide

## Overview

This guide covers common issues with openhub, how to diagnose them with `oh doctor`, and how to repair a broken installation.

---

## Quick Diagnostics

### `oh doctor`

Run `oh doctor` to check your installation health. It performs about twenty checks (the TUI Doctor view runs exactly the same):

| # | Check | What it verifies | Common fix |
|---|-------|-----------------|------------|
| 1 | OS / Architecture | System info (always passes) | — |
| 2 | Go runtime | Go version (always passes) | — |
| 3 | git | `git` binary on PATH | Install git |
| 4 | bd (beads) | Beads CLI (optional) | `brew install datichb/tap/bd` |
| 5 | oh version | Latest published oh release: a newer release is a warning (⚠, not a failure); a pre-release or a development build newer than the latest published release is "up to date" | `oh upgrade oh` (or `brew upgrade openhub`) |
| 6 | Configuration | `hub.toml` loads correctly | `oh init` to reinitialize |
| 7 | Provider credentials | Credentials found through the cascade of a launch from the current directory: project key, team key, hub key, then AWS profile (Bedrock); the source is shown | `oh provider setup`, `oh secrets set` or env var |
| 8 | Database | SQLite database accessible | `oh repair` |
| 9 | API keys | MCP service tokens (Figma, GitLab, etc.) | `oh mcp setup <service>` |
| 10 | Beads zero-impact | No side effects from Beads (hooks, gitignore, agent files written by `bd init`: `AGENTS.md`, `CLAUDE.md`, `.claude/`, `.codex/`, `.cursor/`, `.agents/`; `.beads/` in `.git/info/exclude`) | `oh doctor --fix` (see [Getting started](getting-started.en.md#initial-setup-oh-init)) |
| 11 | opencode V2 | opencode installed, minimum version 2.0.0 (V1 refused) | `brew install anomalyco/tap/opencode`; see the [v5 migration guide](migration-v5.en.md) |
| 12 | Former deployments | Leftovers of `oh deploy` in the projects | `oh migrate deploy-cleanup` |
| 13 | oh daemon | State of the `ohd` daemon (started on the first launch) | `oh daemon status` |
| 14 | Security | Daemon issuing capability in the keychain, proxy tokens stored as hashes | — |
| 15 | Session restrictions | Hub restrictions (off by default) | `oh budget show` |
| 16 | git (relative worktrees) | git version recent enough for worktrees | Upgrade git |
| 17 | Opening sessions | How sessions are opened (terminal, iTerm, tmux…) | — |
| 18 | Container | Engine, file sharing, project images | see [Container](container.en.md) |
| 19 | Gateways | `bd` on the machine (Beads gateway), daemon gateways | Install `bd`; `oh daemon status` |
| 20 | Orphan opencode servers | Servers started by oh that nobody owns any more: interrupted test or native agents discovery, server this hub no longer runs, server of a deleted oh directory (servers of another `OH_HOME` are left alone) | `oh doctor --fix` |

### `oh repair`

Diagnoses and repairs the SQLite database:

```bash
oh repair                 # Diagnosis (without migrating the database)
oh repair --check-only    # Verify without modifying
oh repair --migrate       # Apply the pending migrations, after a backup
oh repair --restore <file.bak> # Put a backup back (daemon stopped)
oh repair --auto          # Non-interactive (for scripts)
```

`oh repair` and `oh doctor` start even when `hub.toml` does not load or the database does not open: they never migrate the database on their own.

If the database is corrupt, `oh repair` will:
1. Copy the database files (with `-wal` and `-shm`) to `<db>.corrupt-<date>`
2. List the backups taken before the migrations (`~/.oh/backups/`) and the exports (`oh-backup-*.tar.gz`)
3. Offer, in this order: restore a backup, restore an export, and only as a last resort start from an empty database

**Unreadable `hub.toml`**: `oh doctor` shows the error with its line and column. `oh` keeps a copy of `hub.toml` before each save (`hub.toml.bak-<date>`, three kept); `oh doctor --fix` restores the most recent valid one after confirmation (`--yes` without a terminal), the broken file is kept as `hub.toml.broken-<date>`.

---

## Common Errors

### Provider and Authentication

**No credentials found**
```
Error: no credentials found for provider "anthropic"
```
Set your API key:
```bash
oh secrets set ANTHROPIC_API_KEY    # Via keychain (recommended)
export ANTHROPIC_API_KEY="sk-..."   # Via environment variable
```

**Launch refused for want of credentials**
```
cannot launch, no credentials found for the provider (project, team, hub; AWS profile for Bedrock): …
```
`oh run` (and the launch form of the TUI, at the Recap step) checks the provider credentials before building the bundle or asking anything of the daemon, with the same cascade as `oh doctor`: project key, team key, hub key, then AWS profile for Bedrock. Set them up with `oh provider setup` (or `-P` for another provider). A refusal of the daemon ("the oh daemon could not prepare the provider credentials…") is diagnosed with `oh doctor`.

**Command run outside of a project directory**
```
you are not in the directory of a project: run the command from the project directory or use -p <project>
```
Outside of a project directory, `oh` no longer silently takes the only registered project. A command that writes (`oh run` and its aliases, `oh review feedback`, `oh mcp setup|enable|disable|reset`, `oh project rename|move|configure`, `oh budget set`…) asks for confirmation in a terminal and refuses without one; a command that reads (`oh bundle show`, `oh mcp status`…) names the project it uses on stderr. With several projects, a list lets you choose. Use `-p <project>` (`oh project list`).

**Invalid token (MCP services)**
```
Error: 401 Unauthorized
```
Refresh or reset the token:
```bash
oh mcp setup gitlab                 # Re-run setup wizard
oh secrets set GITLAB_TOKEN         # Set token directly
```

---

### Configuration

**External modification**
```
Error: hub.toml was modified by another process
```
The config file was changed outside of `oh`. Restart your command — it will reload the config automatically.

**Config parse error**
```
Error: failed to parse hub.toml
```
Check `hub.toml` syntax. Common causes: unclosed quotes, invalid TOML. Run `oh init` to regenerate if needed.

---

### Team State

**Not cloned**
```
Error: team-state repository not cloned
```
Initialize or rejoin the team:
```bash
oh team init       # Create a new team
oh team rejoin     # Join an existing team
```

**Sync conflict**
```
Error: push failed after retries (concurrent edits)
```
Another team member pushed changes simultaneously. Pull and retry:
```bash
cd ~/.oh/team-state && git pull --rebase && git push
```

**Claim already owned**
```
Error: ticket BD-42 is already claimed by alice
```
The ticket is taken. Use `oh team status` to see current claims, or request a transfer.

---

### Tracker

**Write disabled**
```
Error: write operations disabled
```
Enable write mode in your tracker config:
```toml
[tracker]
write_enabled = true
```

**Rate limited**
```
Error: rate limited (retry after 30s)
```
Wait for the specified duration. GitLab and Jira enforce per-user rate limits.

---

### Database

**Corrupt database**
```
Error: database disk image is malformed
```
Run the repair tool:
```bash
oh repair
```

If repair fails, restore from a backup:
```bash
oh import oh-backup-2026-10-01.tar.gz
```

**Database upgrade (migrations)**

On the first run after upgrading `oh`, the `~/.oh/oh.db` database is migrated. Several processes may open it at once (TUI, daemon, MCP servers): one migrates, the others wait (lock `oh.db.migrate.lock`, 30 s at most) and then find it up to date. Before migrating an existing database, `oh` copies it to `~/.oh/backups/oh.db.v<version>-<date>.bak` (the last three are kept). If the copy fails, the migration is refused (`OH_SKIP_DB_BACKUP=1` to override).

If a migration fails, the message names the backup to restore:
```
Database migration v42 failed: …
A backup was taken before migrating: ~/.oh/backups/oh.db.v41-….bak
To restore it: oh repair --restore ~/.oh/backups/oh.db.v41-….bak
```

**Database upgraded by a more recent `oh`**
```
The database ~/.oh/oh.db was upgraded by a more recent oh (migrations v44; …)
```
An older `oh` refuses to work on a database whose schema it does not know: upgrade `oh`. `oh doctor` and `oh repair` still work. To override at your own risk: `OH_ALLOW_NEWER_SCHEMA=1` (no migration is applied then).

---

### team-state

**Busy team-state**
```
The team-state ~/.oh/teams/<team> is busy (held by pid 4242 (oh tui) since 10:02:03): try again in a moment.
```
Every git write of `oh` in a team-state clone (CLI, TUI, daemon, MCP servers) goes through a lock: `.git/oh.lock` between processes, and a lock per path within a process. A command waits up to 30 s, the daemon and the tracker synchronization 2 min; the periodic synchronizations of the TUI skip a turn. A `.git/index.lock` older than 2 minutes (interrupted git) is removed with a warning; a more recent one means a git started by hand in the clone: wait for it to finish. Stalled network transfers give up after 30 s without throughput. A rejected push now only puts back the files of the current publication (no more `reset --hard`).

---

### Worktree

**`.opencode/ directory missing` (oh < v5)**
```
Error: .opencode/ directory missing
```
Since v5 the project no longer needs a `.opencode/` directory (`oh deploy` removed in v5): the session bundle is built at launch. Update `oh` and relaunch, or inspect the bundle:
```bash
oh run <workflow>
oh bundle show <workflow>
```

**Leftovers of former deployments** (reported by `oh doctor`)
```bash
oh migrate deploy-cleanup --dry-run --diff   # preview what would be removed
oh migrate deploy-cleanup                    # remove .opencode/agents, .opencode/skills, oh keys in opencode.json…
```

**Orphan worktrees**
```bash
oh worktree cleanup        # Remove merged worktrees (safe mode)
oh worktree cleanup -f     # Force remove all merged worktrees
oh worktree list           # List all active worktrees
```

---

### Disk space

`oh gc --dry-run` shows what oh can free (session bundles, server data, folders of stopped sessions, worktrees oh created, temporary folders, orphan servers, old history); `oh gc` removes it after a confirmation. The daemon does it every 24 h once a first `oh gc` was run (`oh doctor` › "Clean-up" reminds you). See [`oh gc`](../reference/cli-infra.en.md#oh-gc).

### Logs

| Log | Location |
|---|---|
| `ohd` daemon | `~/.oh/run/ohd.log` |
| Server of a session group | `~/.oh/servers/<group>/data/oh-server.log` |

Logs are bounded: above 20 MB a log is shifted to `<file>.1` (older copies to `.2`, `.3`; three kept). `ohd.log` is rotated when the daemon starts, a server log when its server starts; while they run, the daemon copies then empties the logs that grew too large every 10 minutes.

---

## Diagnostic Flowchart

If `oh` is not working:

1. Run `oh doctor` — check all health checks
2. If database fails → `oh repair`
3. If credentials fail → `oh secrets set` or `oh mcp setup`
4. If config fails → `oh init` to regenerate
5. If team-state fails → `oh team init` or `oh team rejoin`
6. If all checks pass but issue persists → check logs with `oh --log-format json <command>`

---

## Getting Help

```bash
oh doctor          # Full health check
oh repair          # Database repair
oh --help          # Command reference
```

For unresolved issues: [GitHub Issues](https://github.com/datichb/openhub/issues)
