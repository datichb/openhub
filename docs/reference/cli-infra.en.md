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

Checks the system health and dependencies. Exits with code 1 when a check fails (✗); warnings (⚠) do not count. The TUI Doctor view runs exactly the same checks.

```
oh doctor [--fix [--yes]]
```

| Flag | Short | Type | Description |
|------|-------|------|-------------|
| `--fix` | | bool | First repairs what can be repaired: **Beads zero impact** (content written by `bd init` in the registered projects, committed content included), after a summary and a confirmation |
| `--yes` | `-y` | bool | With `--fix`: apply without confirmation (required without a terminal) |

Checks: OS and architecture, git, `bd` (optional), oh version (update available: warning; a pre-release such as `5.0.0-rc.1` or a `git describe` development build newer than the latest published release is up to date), `hub.toml`, provider credentials (cascade of a launch from the current directory: project key, team key, hub key, then AWS profile for Bedrock), database, API keys (keychain), Beads "zero impact" (hooks, `.gitignore`), then the v5 checks: opencode V2 (minimum version; V1 refused with the [migration guide](../guides/migration-v5.en.md)), `ohd` daemon, git, terminal opening, container engine and image, gateways, restrictions and memory, security (socket, capability, tokens), leftovers of former deployments, integrity of the team-state workflows, remote targets, orphan team claims (ticket closed, or every session on the ticket ended; with the `oh team release` command to run).

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
| `--merge` | bool | Merge with existing data (projects only) |

**Example:**

```bash
oh import oh-backup-2026-10-06.tar.gz
oh import ~/backups/oh-backup.tar.gz --overwrite
oh import ~/backups/oh-backup.tar.gz --merge
```

---

### oh repair

Checks the integrity of the SQLite database (`PRAGMA integrity_check` on `~/.oh/oh.db`). On corruption, attempts a recovery and offers: restore a backup, reset the database, or register the projects again. Also prints the schema version.

```
oh repair [--check-only] [--auto]
```

| Flag | Type | Description |
|------|------|-------------|
| `--check-only` | bool | Check only, without repairing |
| `--auto` | bool | Non-interactive mode (scripts) |

**Example:**

```bash
oh repair
oh repair --check-only
oh repair --auto
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

Starts an HTTP server bound to `127.0.0.1` only (never exposed on the network). The dashboard shows projects, sessions, metrics, agent telemetry, team board (kanban), timeline and members.

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
| `--port` | `-p` | int | Listening port (default: 8080) |

**Example:**

```bash
oh serve
oh serve --port 9090
```

> **Security:** the server is bound to `127.0.0.1` only and is never reachable from the network.
