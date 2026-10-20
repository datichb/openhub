> [Lire en francais](cli-infra.fr.md)

# CLI Reference — Infrastructure

## Infrastructure

### oh init

Initializes oh for the first time. Interactive wizard, no flags.

```
oh init
```

Configures: interface language, AI provider and credentials, first project (optional), team to join or create (optional) and MCP integrations (Figma, GitLab, Google Slides, optional). Without a team, the project workflows live in a solo space. The opencode version is checked by `oh doctor`.

**Example:**

```bash
oh init
```

---

### oh doctor

Checks the system health and dependencies. Exits with code 1 when a mandatory check fails (✗); warnings (⚠) do not count, and the optional checks (container engine stopped or missing, `bd` missing, gateways seen from a container) are warnings, unless `execution.runtime = container` makes the container mandatory. The TUI Doctor view runs exactly the same checks.

`oh doctor` works even when `hub.toml` does not load or the database does not open (degraded startup): the failing check says so (TOML error with line and column, unreadable database) and the other checks run. The database is opened **without being migrated**: schema version and the version this oh knows, pending migrations (⚠), last failed migration, schema newer than this oh, quick integrity check, WAL size.

```
oh doctor [--fix [--yes]] [--offline]
```

| Flag | Short | Type | Description |
|------|-------|------|-------------|
| `--offline` | | bool | No network call: neither latest published release nor validation of the provider key (also `OH_DOCTOR_OFFLINE=1`) |
| `--fix` | | bool | First repairs what can be repaired, after a summary and a confirmation: a **`hub.toml` that no longer loads** (replaced by the latest valid backup `hub.toml.bak-<date>`, the broken file kept as `hub.toml.broken-<date>`), **Beads zero impact** (content written by `bd init` in the registered projects, committed content included; the commit of `bd init` itself is undone once listed and confirmed in a terminal, or with `--yes`, after a `git fetch`, when it is neither pushed nor the first of the repository) |
| `--yes` | `-y` | bool | With `--fix`: apply without confirmation (required without a terminal) |

Checks: OS and architecture, git, `bd` (optional), oh version (update available: warning; a pre-release such as `5.0.0-rc.1` or a `git describe` development build newer than the latest published release is up to date), `hub.toml`, provider credentials (cascade of a launch from the current directory: project key, team key, hub key, then AWS profile for Bedrock, according to `auth_mode`; key, scope and date it was stored; a Bedrock API key is validated against AWS, ✗ when rejected; a shell `AWS_BEARER_TOKEN_BEDROCK` that is unused or differs from the stored key is reported), database, API keys (keychain), Beads "zero impact" (hooks, `.gitignore`), then the v5 checks: opencode V2 (minimum version; V1 refused with the [migration guide](../guides/migration-v5.en.md)), `ohd` daemon, git, terminal opening, container engine and image, gateways, restrictions and memory, security (socket, capability, tokens), agent environment (⚠ `execution.env_allow` entries named like secrets; the names, never the values, of the variables of your environment held back from the agents), the session shell (the oh `bd` first, also in an interactive shell), leftovers of former deployments, `AGENTS.md` outside the session bundle (⚠: subfolder `AGENTS.md` of the projects, which the opencode read tool adds to the prompt when an agent reads a file of these folders, and `~/.config/opencode/AGENTS.md`, added to local sessions without `[execution] strict_isolation`; the one of the project root is not read), integrity of the team-state workflows, remote targets, orphan team claims (ticket closed, or every session on the ticket ended; with the `oh team release` command to run), MCP URLs imposed by the team that you have not approved (⚠, with the [`oh mcp trust`](cli-mcp.en.md#oh-mcp-trust) command to run), orphan opencode servers (started by oh or its tests, owned by nobody any more; `--fix` stops them after a confirmation).

**Example:**

```bash
oh doctor
```

---

### oh status

Shows the status of the hub and of the current project.

```
oh status [options]
```

| Flag | Type | Description |
|------|------|-------------|
| `--json` | bool | Output in JSON format |

**Example:**

```bash
oh status
oh status --json
```

---

### oh migrate deploy-cleanup

Removes from the registered projects what the former `oh deploy` left: `.opencode/agents`, `.opencode/skills`, `.opencode/.deploy-state`, `.opencode/context-manifest.json`, `.opencode/team.json` and, in `opencode.json`, only the keys oh wrote that are unchanged since the last deployment (reference: `config_snapshot` of `.deploy-state`). Your keys, and oh keys you changed, are kept. Without `.deploy-state`, `opencode.json` is not touched. Summary (and diff) then confirmation. oh offers it once at startup (the TUI has its cleanup screen, omnibar `cleanup`).

```
oh migrate deploy-cleanup [-p <project>] [--dry-run] [--diff] [--yes] [--json]
```

| Flag | Short | Type | Description |
|------|-------|------|-------------|
| `--project` | `-p` | string | A single project (default: every active project) |
| `--dry-run` | | bool | Show what would be removed, with the diff, without changing anything |
| `--diff` | | bool | Show the diff of `opencode.json` |
| `--yes` | `-y` | bool | Apply without confirmation (required without a terminal) |
| `--json` | | bool | Plan in JSON format (nothing is changed) |

See the [v5 migration guide](../guides/migration-v5.en.md#8-clean-the-former-deployments-oh-migrate-deploy-cleanup) and [`oh deploy` / `oh sync`](cli-deploy.en.md).

---

### oh upgrade oh

Updates the `oh` binary in place (atomic replacement), for installs through `install.sh` or direct download. opencode is installed and updated with its own tool (`oh upgrade opencode` no longer exists).

```
oh upgrade oh [version] [--check]
```

| Argument / flag | Type | Description |
|-----------------|------|-------------|
| `version` | argument | Target version (default: the latest published). Without a version, oh never installs a version older than its own; an older version requested explicitly is installed with a downgrade warning |
| `--check` | bool | Only check whether an update is available |

The download only follows a redirect to HTTPS on GitHub and its releases CDN; the version received must be the one requested, and newer than the running one (unless an explicit downgrade was requested, see above). See [SECURITY.md](../../SECURITY.md#self-update).

**Example:**

```bash
oh upgrade oh
oh upgrade oh 5.0.1
oh upgrade oh --check
```

> **Homebrew users:** use `brew upgrade openhub` instead.

---

### oh export

Creates a backup archive of the hub data.

```
oh export [--output <path>]
```

`.tar.gz` archive holding `oh.db`, `hub.toml` and `secrets.enc` (encrypted), with a SHA-256 checksum file.

The database is archived from a consistent copy (`VACUUM INTO`, WAL included), not from `oh.db` itself: the writes not yet checkpointed into `oh.db-wal` are in the archive, and a database in use is not disturbed. If the copy fails (corrupted database), the file is archived as is, with a warning. `oh import` restores `oh.db` and removes the `oh.db-wal` / `oh.db-shm` files left by the database it replaces.

| Flag | Short | Type | Description |
|------|-------|------|-------------|
| `--output` | `-o` | string | Output path (default: `./oh-backup-<date>.tar.gz`) |

**Example:**

```bash
oh export
oh export --output ~/backups/oh-backup.tar.gz
```

---

### oh import

Restores from a backup archive (`oh export`). Checks the SHA-256 checksum before writing.

```
oh import <file> [--overwrite | --merge]
```

| Flag | Type | Description |
|------|------|-------------|
| `--overwrite` | bool | Overwrite existing data without confirmation |
| `--merge` | bool | Merge with existing data (projects only): adds the projects of the archive whose id, name and path do not exist yet; existing projects, `hub.toml` and secrets are left unchanged. Excludes `--overwrite` |

**Example:**

```bash
oh import oh-backup-2026-10-06.tar.gz
oh import ~/backups/oh-backup.tar.gz --overwrite
oh import ~/backups/oh-backup.tar.gz --merge
```

---

### oh gc

Cleans up what oh leaves behind, after a summary (sizes per category) and a confirmation: session bundles no session uses any more, data of the stopped server groups (group lock held, server stopped; its proxy tokens revoked), folders of the **stopped** sessions, session scratch folders included (`scratch/`, [schema](workflow-schema.en.md#scratch-session-scratch-folders); `results/` kept until `history_days`), worktrees **oh created** at launch (clean, untracked files included, unlocked, with no commit missing from every other branch, unused; their branch deleted only when merged), leftovers of the fan-outs merged or whose parent session is stopped (`oh/fanout/…` worktrees and branches, as [`oh fanout clean`](cli-sessions.en.md#oh-fanout) without `--discard`), temporary `.build-*`/`.fetch-*` folders older than an hour, orphan servers, old history of the database (by batches, then `PRAGMA optimize`). Completed or failed sessions, which can still be resumed, are never cleaned up; neither are the worktrees of sessions recorded before v5.0. Retentions: [`[gc]`](config.en.md).

```
oh gc [--dry-run] [--yes] [--json] [--only <categories>] [--older-than <duration>]
```

| Flag | Short | Type | Description |
|------|-------|------|-------------|
| `--dry-run` | | bool | Show what would be removed, remove nothing |
| `--yes` | `-y` | bool | Remove without confirmation (required without a terminal) |
| `--json` | | bool | Plan (or result) as JSON |
| `--only` | | list | `bundles`, `servers`, `sessions`, `worktrees`, `fanout`, `temp`, `orphans`, `history` |
| `--older-than` | | duration | Replaces `session_days` this once (`30d`, `12h`) |

The daemon runs the same clean-up every `[gc] every_hours` (24 h) without staying awake for it. Its **first pass is a simulation**: `oh doctor` then shows "N reclaimable: run oh gc"; after a first `oh gc`, the daemon cleans up on its own. One clean-up at a time (`~/.oh/run/gc.lock`).

**Example:**

```bash
oh gc --dry-run
oh gc --only bundles,servers --yes
oh gc --older-than 7d
```

---

### oh repair

Checks the SQLite database `~/.oh/oh.db` **without migrating it** and without needing a working configuration or database: integrity (`PRAGMA integrity_check`), schema version and the version this oh knows, pending migrations, last failed migration, backups taken before the migrations (`~/.oh/backups/`). On corruption, the database files (with `-wal` and `-shm`) are copied to `oh.db.corrupt-<date>`, then the options are listed: restore a backup taken before a migration, restore an export, and only as a last resort start from an empty database.

```
oh repair [--check-only] [--auto] [--migrate] [--restore <file>]
```

| Flag | Type | Description |
|------|------|-------------|
| `--check-only` | bool | Check only, without repairing |
| `--auto` | bool | Non-interactive mode (scripts); confirms `--restore` without a terminal |
| `--migrate` | bool | Apply the pending migrations; a backup (`VACUUM INTO`, WAL included) is taken first; a failed migration is detailed with the backup to restore |
| `--restore` | string | Put a backup (`.bak` file) back; the daemon must be stopped (`oh daemon stop`); the replaced database is kept as `oh.db.replaced-<date>` |

**Example:**

```bash
oh repair
oh repair --check-only
oh repair --migrate
oh repair --restore ~/.oh/backups/oh.db.v41-20261009T101500.000000000Z.bak
```

---

### oh purge

Complete removal of the hub and its data. Inventories the artifacts (keychain secrets, leftovers of former deployments in the projects (`.opencode/`), hub directory, OpenCode data, binary) then deletes them after confirmation.

```
oh purge [--dry-run] [--force] [--keep-binary] [--include-tool-data]
```

| Flag | Type | Description |
|------|------|-------------|
| `--dry-run` | bool | Show what would be deleted without deleting anything |
| `--force` | bool | Delete without confirmation |
| `--keep-binary` | bool | Keep the `oh` binary |
| `--include-tool-data` | bool | Also delete the global data of the session tool (for opencode: `~/.local/share/opencode/`, `~/.config/opencode/`). Former name: `--include-opencode` |

```bash
oh purge --dry-run
oh purge --force
oh purge --keep-binary
oh purge --include-tool-data --force
```

---

### oh daemon

The `ohd` daemon hosts the LLM credential proxy (one token per group, the real keys stay on the machine), session supervision, live follow-up, decisions, desktop notifications and gateways. oh starts it automatically (`oh daemon run`, internal command). On Windows, it runs inside the oh process: closing oh puts the sessions to sleep. See [ADR-047](../architecture/adr/047-session-interaction-daemon.en.md) and [ADR-044](../architecture/adr/044-credential-proxy-session-limits.en.md).

```
oh daemon status
oh daemon stop [--force]
```

| Command / flag | Type | Description |
|----------------|------|-------------|
| `status` | | Daemon status: version, PID, proxy address, active servers, tokens, pending requests (or "not running") |
| `stop` | | Stops the daemon; refused while sessions run |
| `stop --force` | bool | Stop even while sessions run |

---

### oh remote

Remote execution on GitLab CI: one `oh-runner` project per GitLab group, whose pipeline is generated by oh; the projects' own CI is never changed. Sessions are launched with `oh run --runtime remote` and fetched back with [`oh session fetch`](cli-sessions.en.md#oh-session-fetch) then [`oh session resolve`](cli-sessions.en.md#oh-session-resolve). Guide: [Remote execution](../guides/remote-runners.en.md). The CI side (`oh runner install|run`) is an internal command used by the pipeline.

**Experimental in 5.0, disabled by default**: `oh remote setup` (like `oh run --runtime remote`, `oh session fetch` and `oh session resolve`) is refused until `oh config set remote.experimental true` is done; `oh remote status` and `oh doctor` remind it with a warning.

#### oh remote setup

Checks GitLab API access, creates or validates the `oh-runner` project of the group, writes the pipeline generated by oh, creates the trigger token (kept in the keychain) and sets the masked CI variables: LLM key of the jobs (`--llm`), access token of each target project (`--project`, `write_repository` and `read_api` scopes), team-state token (`--teamstate`). Secrets are read from the terminal (hidden input) or from an environment variable (`--*-env`), never from arguments. Can be run again: only what is missing or changed is modified.

```
oh remote setup [flags]
```

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--url` | string | the one of the current repository | GitLab instance URL |
| `--group` | string | the one of the current repository | Full path of the served group |
| `--name` | string | derived from the group | Target name |
| `--runner-project` | string | `<group>/oh-runner` | Path of the `oh-runner` project |
| `--no-create` | bool | `false` | Do not create the `oh-runner` project when it is missing |
| `--force` | bool | `false` | Replace a `.gitlab-ci.yml` that was not generated by oh |
| `--token-env` | string | | Environment variable holding the GitLab token (`api` scope) |
| `--token-key` | string | | Keychain key of an existing GitLab token (`api` scope) |
| `--llm` | bool | `false` | Set the LLM key of the jobs (hidden input) |
| `--llm-key-env` | string | | Environment variable holding the LLM key of the jobs |
| `--llm-provider` | string | `bedrock` | LLM provider of the jobs: `bedrock`, `anthropic`, `openrouter` |
| `--llm-region` | string | `eu-west-1` (Bedrock) | Provider region |
| `--project` | string (repeatable) | | Target project whose access token to set (full path) |
| `--project-token-env` | string | | Environment variable holding the project token (single `--project`) |
| `--teamstate` | bool | `false` | Set the write token of the team-state repository (hidden input) |
| `--teamstate-token-env` | string | | Environment variable holding the team-state token |
| `--tag` | string | `oh` | Runner tag |
| `--arch` | string | `amd64` | Runner architecture: `amd64` or `arm64` |
| `--builder` | string | `kaniko` | Project image builder: `kaniko` or `dind` |
| `--timeout` | string | `3h` | Maximum job duration |
| `--oh-binary` | string | | Linux oh binary to upload (development builds) |

```bash
oh remote setup --group acme/web --llm --project acme/web/api --teamstate
oh remote setup --token-env GITLAB_TOKEN --llm-key-env OH_LLM_KEY --project acme/web/api --project-token-env API_TOKEN
```

#### oh remote status

Checks the remote targets (`oh-runner` project, pipeline, variables, runners): all of them, or the named one. Exits with an error when a check fails.

```
oh remote status [target]
```

---

### oh serve

Starts a local web dashboard.

```
oh serve [--port 8080]
```

Starts an HTTP server bound to `127.0.0.1` only (never exposed on the network). oh prints the dashboard URL with a random **access token**, specific to this run: `http://127.0.0.1:8080/?token=…`. Open it as it is: the browser keeps the token in a cookie (`HttpOnly`, `SameSite=Strict`) and the address bar then shows `/` only. A request without a valid token gets 401; a request whose `Host` header is not `127.0.0.1:<port>`, `localhost:<port>` or `[::1]:<port>` gets 403. The dashboard shows projects, sessions, metrics, agent telemetry, team board (kanban), timeline and members.

**API endpoints:**
- `GET /api/v1/health`
- `GET /api/v1/projects`
- `GET /api/v1/sessions?project_id=<id>`
- `GET /api/v1/metrics/agents?project_id=<id>`
- `GET /api/v1/platform/stats?period=7d|30d|all`
- `GET /api/v1/platform/sessions?limit=20`
- `GET /api/v1/team/board?project=<id>`
- `GET /api/v1/team/events?limit=50&project=<id>`
- `GET /api/v1/team/members`
- `GET /api/v1/chart/costs?period=30d`
- `GET /sse` (Server-Sent Events, real time)

| Flag | Short | Type | Description |
|------|-------|------|-------------|
| `--port` | | int | Listening port (default: 8080). The former short form `-p` (the project everywhere else) is still accepted, with a warning |

**Example:**

```bash
oh serve
oh serve --port 9090
```

Outside a browser, pass the token in the URL: `curl "http://127.0.0.1:8080/api/v1/health?token=<token>"`.

> **Security:** the server is bound to `127.0.0.1` only and is never reachable from the network. The `Host` header check and the token keep a malicious web page from reading the dashboard (DNS rebinding); the token is not written to disk and changes at every launch. Do not share the printed URL.
