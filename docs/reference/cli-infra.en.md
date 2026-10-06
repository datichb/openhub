> [Lire en francais](cli-infra.fr.md)

# CLI Reference — Infrastructure

## Infrastructure

### oh init

First-time setup wizard. Configures language, opencode, project, MCP servers, and deploy targets interactively.

```bash
oh init
```

---

### oh doctor

Run diagnostic checks on the environment. Checks: OS, git, opencode, bd, fzf, compatibility, config, database, API keys. Also checks for available `oh` binary updates.

```bash
oh doctor
```

---

### oh status

Display current environment status.

| Flag | Type | Description |
|------|------|-------------|
| `--json` | bool | Output in JSON format |

```bash
oh status
oh status --json
```

---

### oh migrate deploy-cleanup

Removes from the registered projects what the former `oh deploy` left (v5): `.opencode/agents`, `.opencode/skills`, `.opencode/.deploy-state`, `.opencode/context-manifest.json`, `.opencode/team.json` and, in `opencode.json`, only the keys oh wrote that are unchanged since the last deployment. Summary then confirmation.

```
oh migrate deploy-cleanup [-p <project>] [--dry-run] [--diff] [--yes] [--json]
```

| Flag | Description |
|------|-------------|
| `--project`, `-p` | A single project (default: every active project) |
| `--dry-run` | Show what would be removed, with the diff, without changing anything |
| `--diff` | Show the diff of `opencode.json` |
| `--yes`, `-y` | Apply without confirmation (required without a terminal) |
| `--json` | Plan as JSON (nothing is changed) |

See the [v5 migration guide](../guides/migration-v5.en.md#8-clean-the-former-deployments-oh-migrate-deploy-cleanup).

---

### oh upgrade oh

Update the `oh` binary in-place (atomic replacement). For non-Homebrew installs only.

```
oh upgrade oh [version] [--check]
```

| Flag | Type | Description |
|------|------|-------------|
| `--check` | bool | Verify available version without downloading |
| `version` | string | Target version (default: latest) |

```bash
oh upgrade oh
oh upgrade oh 1.3.0
oh upgrade oh --check
```

> **Homebrew users:** use `brew upgrade openhub` instead.

---


### oh service setup (deprecated)

> **Deprecated:** Use `oh mcp setup` instead.

Interactive wizard to configure MCP service tokens in keychain.

```bash
oh service setup
```

---

### oh service remove (deprecated)

> **Deprecated:** Use `oh mcp disable` instead.

Remove a configured service.

| Flag | Short | Type | Description |
|------|-------|------|-------------|
| `--force` | `-f` | bool | Skip confirmation |

```bash
oh service remove gitlab
oh service remove figma -f
```

---

### oh export

Create a backup archive of the hub data.

```
oh export [--output <path>]
```

Creates a `.tar.gz` archive containing: `oh.db`, `hub.toml`, `secrets.enc` (encrypted). Includes a SHA-256 checksum file.

| Flag | Short | Type | Description |
|------|-------|------|-------------|
| `--output` | `-o` | string | Output path (default: `./oh-backup-YYYY-MM-DD.tar.gz`) |

```bash
oh export
oh export --output ~/backups/oh-backup.tar.gz
```

---

### oh import

Restore from a backup archive.

```
oh import <file> [--overwrite] [--merge]
```

Verifies SHA-256 checksum before writing any data.

| Flag | Type | Description |
|------|------|-------------|
| `--overwrite` | bool | Overwrite existing data |
| `--merge` | bool | Merge projects only (non-destructive) |

```bash
oh import oh-backup-2025-07-22.tar.gz
oh import ~/backups/oh-backup.tar.gz --overwrite
oh import ~/backups/oh-backup.tar.gz --merge
```

---

### oh repair

Diagnose and repair the SQLite database.

```
oh repair [--check-only] [--auto]
```

Runs `PRAGMA integrity_check` on `~/.oh/oh.db`. If corruption is detected, presents recovery options: restore from backup, reinitialize database, or manually re-register projects. Also displays the current schema version.

| Flag | Type | Description |
|------|------|-------------|
| `--check-only` | bool | Diagnose without making changes |
| `--auto` | bool | Non-interactive mode |

```bash
oh repair
oh repair --check-only
oh repair --auto
```

---

### oh serve

Start a local web dashboard.

```
oh serve [--port 8080]
```

Starts an HTTP server bound to `127.0.0.1` only (never exposed to the network). Dashboard shows projects, sessions, agent telemetry, team board, and cost charts.

**API endpoints:**
- `GET /api/v1/health`
- `GET /api/v1/projects`
- `GET /api/v1/sessions?project_id=<id>`
- `GET /api/v1/metrics/agents?project_id=<id>`
- `GET /api/v1/opencode/stats?period=7d|30d|all`
- `GET /api/v1/opencode/sessions?limit=20`
- `GET /api/v1/team/board?project=<id>`
- `GET /api/v1/team/events?limit=50&project=<id>`
- `GET /api/v1/team/members`
- `GET /api/v1/chart/costs?period=30d`
- `GET /sse` (Server-Sent Events, real-time push)

| Flag | Short | Type | Description |
|------|-------|------|-------------|
| `--port` | `-p` | int | Port (default: 8080) |

```bash
oh serve
oh serve --port 9090
```

> **Security:** The server binds to `127.0.0.1` only and is never accessible from the network.

---

---


---

### oh purge

Complete removal of the hub and all its data. Inventories all artifacts (keychain secrets, project deploy artifacts, hub directory, OpenCode data, binary) then deletes them after confirmation.

| Flag | Type | Description |
|------|------|-------------|
| `--dry-run` | bool | Show what would be deleted without deleting |
| `--force` | bool | Skip confirmation prompts |
| `--keep-binary` | bool | Keep the `oh` binary installed |
| `--include-opencode` | bool | Also remove OpenCode global data |

```bash
oh purge --dry-run
oh purge --force
oh purge --keep-binary
oh purge --include-opencode --force
```
