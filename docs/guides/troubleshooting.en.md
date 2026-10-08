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
| 10 | Beads zero-impact | No side effects from Beads (hooks, gitignore, agent files written by `bd init`: `AGENTS.md`, `CLAUDE.md`, `.claude/`, `.codex/`, `.cursor/`, `.agents/`; `.beads/` in `.git/info/exclude`) | see [Getting started](getting-started.en.md#initial-setup-oh-init): initialize with oh, or remove by hand what `bd init` added |
| 11 | opencode V2 | opencode installed, minimum version 2.0.0 (V1 refused) | `brew install anomalyco/tap/opencode`; see the [v5 migration guide](migration-v5.en.md) |
| 12 | Former deployments | Leftovers of `oh deploy` in the projects | `oh migrate deploy-cleanup` |
| 13 | oh daemon | State of the `ohd` daemon (started on the first launch) | `oh daemon status` |
| 14 | Security | Daemon issuing capability in the keychain, proxy tokens stored as hashes | — |
| 15 | Session restrictions | Hub restrictions (off by default) | `oh budget show` |
| 16 | git (relative worktrees) | git version recent enough for worktrees | Upgrade git |
| 17 | Opening sessions | How sessions are opened (terminal, iTerm, tmux…) | — |
| 18 | Container | Engine, file sharing, project images | see [Container](container.en.md) |
| 19 | Gateways | `bd` on the machine (Beads gateway), daemon gateways | Install `bd`; `oh daemon status` |

### `oh repair`

Diagnoses and repairs the SQLite database:

```bash
oh repair                 # Interactive diagnosis and repair
oh repair --check-only    # Verify without modifying
oh repair --auto          # Non-interactive (for scripts)
```

If the database is corrupt, `oh repair` will:
1. Back up the corrupt file to `<db>.corrupt-backup`
2. Search for existing export backups (`oh-backup-*.tar.gz`)
3. Offer recovery options: restore from backup, reinitialize, or manual re-registration

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
