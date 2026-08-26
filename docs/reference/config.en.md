> 🇫🇷 [Lire en français](config.fr.md)

# Configuration Reference

---

## Hub Configuration File

**Location:** `~/.oh/hub.toml`  
**Created by:** `oh init`  
**View path:** `oh config path`

### Complete TOML Structure

```toml
[cli]
language = "en"                    # "fr" or "en"

[opencode]
version = "latest"                 # pinned version or "latest"
channel = "stable"                 # release channel
auto_update = false                # auto-update opencode binary
install_dir = "~/.oh/bin"          # where opencode is installed
default_provider = "bedrock"       # bedrock | anthropic | openai | openrouter

[provider.bedrock]
aws_profile = "default"            # AWS profile (bedrock only)
aws_region = "eu-west-1"           # AWS region (bedrock only)
auth_mode = "bearer"               # "bearer" | "profile" (bedrock only)

[models]
default = "claude-sonnet-4-20250514"  # hub-level default model

[models.families]
anthropic = "claude-sonnet-4-20250514"

[models.agents]
reviewer = "claude-opus-4-20250514"   # per-agent override

[[teams]]
id = "acme"                        # local short identifier
name = "Equipe ACME"               # display name
enabled = true
state_repo = "git@gitlab.com:acme/team-state.git"
member_id = "alice"

[mcp.figma]
enabled = true                     # enable Figma MCP server
token_key = "openhub.mcp.figma.token"          # keychain key name (NOT the token itself)

[mcp.gitlab]
enabled = true
token_key = "openhub.mcp.gitlab.token"
write_enabled = true
url = ""                           # empty = use team-state URL or built-in default

[mcp.jira]
enabled = false

[mcp.gslides]
enabled = false
token_key = "openhub.mcp.gslides.token"

[worktree]
auto_cleanup = true                # auto-remove merged worktrees on start
base_branch = ""                   # empty = auto-detect (main/master)

[tracker]                          # local overrides for team tracker sync
# enabled = false                  # uncomment to disable sync locally
# push_labels = false              # uncomment to disable label push

[websearch]
enabled = true
```

---

## Config Commands

| Command | Description |
|---------|-------------|
| `oh config list [--json]` | Show all config values |
| `oh config get <key>` | Get a specific value (dot notation: `opencode.version`) |
| `oh config set <key> <value>` | Set a value |
| `oh config unset <key>` | Remove a key |
| `oh config path` | Print config file path |
| `oh config language [fr\|en]` | Get or set display language |
| `oh config websearch [enable\|disable\|status]` | Manage WebSearch for agents |

---

## Project Storage

Projects are stored in a **SQLite database** at `~/.oh/oh.db`.

### Project Fields

| Field | Description |
|-------|-------------|
| ID | Auto-generated slug + UUID prefix |
| Name | Human-readable project name |
| Path | Absolute filesystem path |
| Language | go, typescript, python, rust, java, etc. |
| Tracker | github, gitlab, jira, linear, or empty |
| Provider | Project-level override, or empty for hub default |
| Model | Project-level override, or empty for hub default |
| MCPConfig | Per-project MCP configuration (JSON, overrides hub) |
| ProviderConfig | Per-project provider configuration (JSON, overrides hub) |
| Status | active, archived |
| CreatedAt | Creation timestamp |
| UpdatedAt | Last modification timestamp |

### Project Commands

| Command | Description |
|---------|-------------|
| `oh project list` | List all registered projects |
| `oh project add` | Register a new project |
| `oh project remove` | Unregister a project |
| `oh project rename` | Rename a project |
| `oh project move` | Update a project's path |
| `oh project configure` | Change project settings (provider, model, tracker) |

---

## Notification Configuration

Configure webhooks to send notifications on session events (completion, errors, audit results).

```toml
[notify]
enabled = true
type = "slack"           # mattermost | slack | discord | teams
webhook_url = "https://hooks.slack.com/services/..."
channel = "#dev-ai"      # Mattermost only
bot_name = "OpenHub"

# Multi-destination (optional): notify multiple channels simultaneously
[[notify.destinations]]
type = "slack"
webhook_url = "https://hooks.slack.com/..."
bot_name = "OpenHub"

[[notify.destinations]]
type = "discord"
webhook_url = "https://discord.com/api/webhooks/..."
```

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `enabled` | bool | false | Enable notifications |
| `type` | string | "mattermost" | Backend: mattermost, slack, discord, teams |
| `webhook_url` | string | — | Incoming webhook URL |
| `channel` | string | — | Channel name (Mattermost only) |
| `bot_name` | string | "OpenHub" | Bot display name |
| `destinations` | array | — | Multiple notification targets (overrides `type`/`webhook_url`) |

> **Note:** When `destinations` is set, the top-level `type`/`webhook_url` fields are ignored.

---

## Multi-Team Configuration (ADR-029)

A user can belong to **multiple teams**. Each team is a separate entry in the `[[teams]]` array.
Projects reference a team by its `id` field.

### Hub-level: `[[teams]]`

```toml
[[teams]]
id = "acme"                              # local short identifier
name = "Equipe ACME"                     # display name (optional, falls back to id)
enabled = true                           # enable team features for this team
state_repo = "git@gitlab.com:acme/team-state.git"
state_path = "~/.oh/team-states/acme"    # auto-derived from state_repo if empty
member_id = "alice"                      # your identity in this team

[[teams]]
id = "beta"
name = "Equipe Beta"
state_repo = "git@github.com:beta/oh-team.git"
member_id = "alice.dupont"
```

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `id` | string | Yes | Short local identifier, used by projects |
| `name` | string | No | Display name (falls back to `id`) |
| `enabled` | bool | No | Enable/disable this team (default: true) |
| `state_repo` | string | Yes | Git remote URL of the team-state repo |
| `state_path` | string | No | Local clone path (auto-derived from `state_repo`) |
| `member_id` | string | Yes | Your identity in this team's `members.toml` |

### Project-level: `team_id`

Each project declares its team affiliation via `TeamID`:

| Value | Meaning |
|-------|---------|
| `nil` (not set) | Solo project — no team affiliation |
| `"acme"` | Project belongs to team "acme" |

### Resolution cascade

```
project.TeamID == nil (or "")  →  solo project, team disabled
project.TeamID == "acme"       →  lookup teams[] by id, use that team's config
project.TeamID == unknown      →  graceful degradation (team disabled)
```

### Migration from `[team]` (legacy)

The legacy single `[team]` section is auto-migrated to `[[teams]]` on first load:
- ID is derived from the `state_repo` URL (last path segment, minus `.git`)
- A backup of `hub.toml` is created before writing
- Projects with `Mode: "inherit"` → `TeamID = <derived-id>`
- Projects with `Mode: "disabled"` → `TeamID = nil`

### CLI Commands

| Command | Description |
|---------|-------------|
| `oh teams list` | List configured teams |
| `oh teams add --repo <url> --member-id <id>` | Add a new team |
| `oh teams remove <team-id>` | Remove a team (projects become solo) |

---

## Enforced vs Recommended Configuration (ADR-030)

Team-state settings can be either **recommended** (overridable) or **enforced** (locked).

### Resolution cascade for each setting

```
1. Team ENFORCED?  → YES: use team value (locked, no override possible)
                   → NO: continue
2. Project has explicit override?  → YES: use project value
                                   → NO: continue
3. Hub has a value?  → YES: use hub value
                     → NO: continue
4. Team RECOMMENDED?  → YES: use team recommendation as fallback
                      → NO: use system default
```

### TOML schema in team-state `config.toml`

```toml
[mcp.gitlab]
enabled = true
enabled_enforced = true    # members MUST have GitLab enabled
url = "https://gitlab.company.com"
url_enforced = true        # URL cannot be overridden locally

[mcp.jira]
enabled = true             # recommended (no _enforced = overridable)

[models]
default = "claude-sonnet-4-20250514"  # recommended, overridable

[models.agents]
architect = "claude-opus-4-20250514"  # recommended per-agent model
```

### TUI display

- 🔒 **Enforced**: lock icon, field non-editable, toast on edit attempt
- `[team: recommended]`: source annotation when inherited from team
- `[hub]`, `[project]`: source annotation for local values

---

## Per-project Team Configuration (Legacy)

> **Deprecated:** The `ProjectTeamConfig` with `mode` field is superseded by `project.TeamID`.
> Existing projects using the old format are still supported via backward-compat resolution.

### Deployed artifact: `.opencode/team.json`

`oh deploy` resolves the effective config and writes `.opencode/team.json` in the project root:

```json
{
  "enabled": true,
  "state_repo": "git@gitlab.com:acme/team-state.git",
  "state_path": "/Users/alice/.oh/team-states/team-state",
  "member_id": "alice"
}
```

When mode is `disabled`, the file is removed (or never created) and the `team` MCP server
is not injected into `opencode.json`.

### State path auto-derivation

When `state_path` is empty in a custom config, it is derived from `state_repo`:

```
git@gitlab.com:acme/other-team.git  →  ~/.oh/team-states/other-team
https://github.com/acme/my-team.git →  ~/.oh/team-states/my-team
```

### Setting the mode

- **At project creation:** the `oh project add` wizard includes a Team step.
- **Post-creation:** use `team configure` in the TUI omnibar, then redeploy.

---

## Team-State Configuration (`config.toml`)

**Location:** `~/.oh/team-state/config.toml`  
**Purpose:** runtime behaviour of the team-state MCP server (claims, tracker integration).  
This file lives in the team-state git repository — **not** in `hub.toml`.

> **Credentials are not stored here.** Connection tokens and base URLs are reused from the
> corresponding `[mcp.gitlab]` or `[mcp.jira]` block in `hub.toml` — no secret duplication.

### `[claim]` section

Controls how long completed claims are kept before automatic cleanup.

```toml
[claim]
done_retention_days = 7
```

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `done_retention_days` | int | `7` | Number of days a claim stays in `done` status before being automatically purged by `CleanupDoneClaims`. Set to `0` to disable automatic cleanup. |

### `[tracker]` section

Integrates the team-state with an external issue tracker (GitLab or Jira).

```toml
[tracker]
enabled = true
type = "gitlab"              # "gitlab" or "jira"
auto_sync = true             # automatically sync when opening team views
sync_interval_minutes = 5    # polling interval when board is open (0 = disabled)
auto_plan_assigned = true    # create "planned" claims for tracker-assigned issues
max_auto_plan_per_member = 5 # max auto-planned claims per member (avoid flooding TODO)
push_labels = false          # sync hub labels back to tracker (requires write_enabled on MCP)

# Map hub ticket ID → external IID using regex capture group
ticket_patterns = { "T-SRU" = "SRU-(\\d+)", "T-FRONT" = "FRONT-(\\d+)" }

[tracker.projects]
"T-SRU" = "42"               # hub project ID → GitLab project ID or path
"T-FRONT" = "group/frontend"
```

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `enabled` | bool | `false` | Enable tracker integration |
| `type` | string | — | Tracker backend: `"gitlab"` or `"jira"` |
| `auto_sync` | bool | `false` | Automatically sync when opening team views |
| `sync_interval_minutes` | int | `0` | Polling interval in minutes while the board is open. `0` disables polling. |
| `auto_plan_assigned` | bool | `false` | Automatically create `"planned"` claims for issues assigned to each member in the tracker |
| `max_auto_plan_per_member` | int | `5` | Cap on auto-planned claims per member per sync (prevents flooding the TODO list) |
| `push_labels` | bool | `false` | Sync hub labels back to the tracker. Requires `write_enabled = true` on the corresponding MCP server. |
| `ticket_patterns` | map | `{}` | Map of hub project ID → regex with **exactly one capture group** that extracts the numeric IID (e.g. `"SRU-(\\d+)"`) |
| `[tracker.projects]` | map | `{}` | Map of hub project ID → GitLab project ID / path or Jira project key |

**Notes:**

- **GitLab** (`type = "gitlab"`): credentials and base URL come from `[mcp.gitlab]` in `hub.toml`.
- **Jira** (`type = "jira"`): credentials and base URL come from `[mcp.jira]` in `hub.toml`. Closed issues are detected via `statusCategory.key == "done"` — custom workflow names are ignored.
- `push_labels` requires `write_enabled = true` on the corresponding MCP server in `hub.toml`.
- Each `ticket_patterns` regex must contain **exactly one capture group** that extracts the numeric IID.

---

## Notifications View

The TUI captures every toast notification in an in-memory store and makes the full
history available via a dedicated view.

### Accessing the view

Type any of the following in the omnibar:

| Command | Description |
|---------|-------------|
| `notifications` | Open the notification history |
| `notif` / `logs` / `messages` / `toasts` / `erreurs` | Aliases |

### What it shows

```
14:32:05  ✗  Erreur setup team : git clone https://gitlab.com/...: fatal: repository not found
14:31:58  ✓  Team configurée : custom — redéployez pour appliquer
14:31:52  →  Initialisation team pour ce projet...
```

Each entry has:
- **Timestamp** (`HH:MM:SS`)
- **Level icon** — `✓` success · `✗` error · `!` warning · `→` info
- **Full untruncated message** — toasts on-screen are truncated (80 chars for
  errors/warnings, 50 for others); the store always retains the complete text

The view is **scrollable** (`j` / `k` to navigate).

### Store behaviour

- **Capacity:** 50 entries in-memory per session (FIFO — oldest evicted when full)
- **Persistence:** notifications are appended to `~/.oh/notifications.jsonl` after
  every toast. The file is rotated at TUI startup (max 500 lines, entries older than
  7 days removed). The Notifications view loads the last 50 entries from this file
  on every Mount — history is available across sessions.
- **Stderr log:** error-level notifications are also written to stderr as
  `[ERROR] <message>`, useful for log redirection in CI or remote sessions

---

## Text Selection

The TUI supports native-style text selection anywhere outside interactive widgets
(omnibar, modals). No external dependency is required.

### How to use

1. **Click and drag** to select text — selected cells are highlighted with reverse video
2. **Release** — text is automatically copied to the clipboard
3. **Double-click** — selects the word under the cursor
4. **Triple-click** — selects the entire line
5. **Esc** — clears the selection

### Clipboard

The clipboard write uses a two-strategy pipeline (no external Go dependency):

| Strategy | Platforms |
|----------|-----------|
| OSC 52 escape sequence via `/dev/tty` | All modern terminals (iTerm2, WezTerm, Alacritty, kitty, tmux with `set-clipboard on`), works over SSH |
| `pbcopy` | macOS fallback |
| `wl-copy` | Linux/Wayland fallback |
| `xclip` / `xsel` | Linux/X11 fallback |
| `clip.exe` | Windows fallback |

Both strategies are attempted; the operation succeeds if at least one works.

### Interactive zones (pass-through)

Mouse events in the following areas are passed through to tview unchanged (selection
does not activate in these zones):

- Omnibar container, input field, and suggestions list
- Any inline modal or sub-overlay (prompts, select lists)

### Toast overlays

Toast notifications are **selectable** — clicking on a toast area starts a selection
rather than interacting with the toast.

---

## Secrets / API Keys

Secrets are stored in the **OS keychain** (macOS Keychain, Linux secret-service, Windows Credential Manager).

### Fallback

If the keychain is unavailable, secrets are stored in `~/.oh/secrets.enc` (AES-256-GCM encrypted file with Argon2id KDF).

**Passphrase source (for fallback):**

1. `OH_PASSPHRASE` environment variable
2. Interactive terminal prompt (with confirmation on first use, minimum 8 characters)

### Secret Commands

| Command | Description |
|---------|-------------|
| `oh service setup` | Store a token in the keychain |

### Known Secret Keys

| Key | Purpose |
|-----|---------|
| `bedrock-token-default` | AWS Bearer token for Bedrock |
| `bedrock-token-<project-id>` | Per-project Bedrock token |
| `anthropic-api-key-default` | Global Anthropic API key |
| `anthropic-api-key-<project-id>` | Per-project Anthropic API key |
| `openrouter-api-key-default` | Global OpenRouter API key |
| `openrouter-api-key-<project-id>` | Per-project OpenRouter API key |
| `figma-token` | Figma API token |
| `gitlab-token` | GitLab API token |
| `gslides-token` | Google Slides OAuth token |

---

## Project Configuration (opencode.json)

Each project has an `opencode.json` at its root, generated by `oh deploy`.

```json
{
  "$schema": "https://opencode.ai/config.json",
  "model": "claude-sonnet-4-5",
  "provider": { ... },
  "agent": { ... },
  "plugin": ["context-mode"],
  "compaction": { "auto": true, "prune": true, "reserved": 10000 },
  "mcpServers": {
    "figma": { "command": "oh", "args": ["mcp", "serve", "figma"] },
    "gitlab": { "command": "oh", "args": ["mcp", "serve", "gitlab"] }
  }
}
```

> **Note:** This file is managed by `oh deploy` — do not edit manually unless you know what you're doing.

---

## Environment Variables

| Variable | Purpose |
|----------|---------|
| `OH_PASSPHRASE` | Passphrase for encrypted secret store fallback |
| `FIGMA_TOKEN` | Figma API token (read by MCP server at runtime) |
| `GITLAB_TOKEN` | GitLab API token (read by MCP server at runtime) |
| `GITLAB_URL` | GitLab instance URL (default: `https://gitlab.com`) |
| `GOOGLE_ACCESS_TOKEN` | Google OAuth token (read by MCP server at runtime) |
| `GITHUB_TOKEN` | GitHub API token (read by MCP server at runtime); alias: `GH_TOKEN` |
| `GITHUB_WRITE_ENABLED` | Set to `true` to enable `github_create_issue` tool |
| `JIRA_URL` | Jira instance URL (e.g. `https://mycompany.atlassian.net`) |
| `JIRA_TOKEN` | Jira API token; alternatively use `JIRA_USER` + `JIRA_API_TOKEN` |
| `JIRA_WRITE_ENABLED` | Set to `true` to enable `jira_transition_issue` tool |
| `LINEAR_API_KEY` | Linear API key (read by MCP server at runtime) |
| `LINEAR_WRITE_ENABLED` | Set to `true` to enable `linear_create_issue` / `linear_update_issue` |
| `PAGER` | Custom pager for help output (default: `less`) |

---

## Provider Resolution

When starting a session, the LLM provider is resolved in this order:

1. `--provider` / `-P` flag (highest priority)
2. `project.Provider` in the database
3. `opencode.default_provider` in `hub.toml`
4. `"bedrock"` (hardcoded fallback)

---

## File Locations Summary

| Path | Purpose |
|------|---------|
| `~/.oh/` | Hub configuration directory |
| `~/.oh/hub.toml` | Hub configuration file |
| `~/.oh/hub/` | Extracted hub content (agents, skills) |
| `~/.oh/oh.db` | SQLite database (projects, sessions) |
| `~/.oh/secrets.enc` | Encrypted secrets fallback |
| `~/.oh/bin/` | Managed opencode binary |
| `~/.oh/mcp/<name>/manifest.json` | Custom MCP server manifest (dynamic registry) |
| `<project>/opencode.json` | Project opencode config (generated by deploy) |
| `<project>/.opencode/agents/` | Deployed agent definitions |
| `<project>/.opencode/skills/` | Deployed skill protocols |

> **Database integrity:** Run `oh repair --check-only` to verify the SQLite database. The current schema version is shown by `oh repair`.
>
> **Web dashboard:** `oh serve` binds to `127.0.0.1` only and is never accessible from the network.
