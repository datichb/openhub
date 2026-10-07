> 🇫🇷 [Lire en français](config.fr.md)

# Configuration Reference

oh v5 configuration files: `hub.toml` (your machine), the project database (`oh.db`), the team-state `config.toml` (team) and the workflows (`oh/v1`, see [schema](workflow-schema.en.md)).

---

## Hub Configuration File

**Location:** `~/.oh/hub.toml` (or `$OH_HOME/hub.toml`)  
**Created by:** `oh init`  
**View path:** `oh config path`

### Complete TOML Structure

```toml
name = "OpenHub"                   # name shown in the TUI title

[cli]
language = "en"                    # "fr" or "en" (default: "en")

[opencode]
default_provider = "bedrock"       # bedrock | anthropic | openrouter | github-copilot

[provider.bedrock]
aws_profile = "default"            # AWS profile (bedrock only)
aws_region = "eu-west-1"           # AWS region (bedrock only)
auth_mode = "bearer"               # "bearer" | "profile" | "env" (bedrock only)

[provider.anthropic]
# Uses the API key stored in the keychain (no field)

[provider.openrouter]
# Uses the API key stored in the keychain (no field)

[models]
default = "claude-sonnet-4-20250514"  # hub default model

[models.families]
quality = "claude-opus-4-20250514"    # per agent family

[models.agents]
reviewer = "claude-opus-4-20250514"   # per agent

[[teams]]
id = "acme"                        # local short identifier
name = "Equipe ACME"               # display name
enabled = true
state_repo = "git@gitlab.com:acme/team-state.git"
member_id = "alice"

[mcp.figma]
enabled = true                     # enable the Figma MCP server
token_key = "openhub.mcp.figma.token"   # keychain key name (NOT the token)

[mcp.gitlab]
enabled = true
token_key = "openhub.mcp.gitlab.token"
write_enabled = true
url = ""                           # empty = team-state URL or default

[mcp.jira]
enabled = false

[mcp.gslides]
enabled = false
token_key = "openhub.mcp.gslides.token"

[worktree]
auto_cleanup = true                # remove merged worktrees on start
base_branch = ""                   # empty = auto-detect (main/master)
branch_pattern = "oh/%s"           # branch naming (%s = worktree name)

[deploy]
instruction_files = []             # project files added to the agent instructions (besides ONBOARDING.md, CONVENTIONS.md, .claude/CLAUDE.md)

[tracker]                          # local overrides of the team tracker sync
# enabled = false                  # uncomment to disable sync locally
# push_labels = false              # uncomment to disable label push

[websearch]
enabled = false                    # WebSearch / WebFetch (Exa AI) for agents

[session]
attach = "auto"                    # auto | iterm | terminal | tmux | browser | suspend
iterm_style = "tab"                # tab | split | window (iTerm2)
idle_sleep_minutes = 5             # idle server goes to sleep
notify = "on"                      # system notifications: on | off

[execution]
runtime = ""                       # local | container; empty = workflow default
engine = "auto"                    # auto | colima | podman | docker
keep_images = 2                    # images kept per project and role
opencode_version = ""              # opencode version of the images; empty = the machine's
strict_isolation = false           # also hide the personal opencode config locally

[remote]
[[remote.targets]]                 # oh-runner projects (one per GitLab instance and group)
name = "acme"
url = "https://gitlab.com"
group = "acme"
# runner_project = "acme/oh-runner"   # default: <group>/oh-runner
# tag = "oh"  builder = "kaniko"  arch = "amd64"  timeout = "3h"
[remote.projects]                  # oh project → target (otherwise automatic choice)
# web-app = "acme"

[limits]                           # hub I6 restrictions (off by default)
# max_active_sessions = 4
# session_budget_usd = 5
# daily_budget_usd = 30
# memory_mb = 4096
# models = ["eu.anthropic.claude-*"]
```

Removed in v5 (ignored if still in the file): `[opencode] version`, `channel`, `auto_update`, `install_dir` (opencode V2 is installed separately, see [v5 migration](../guides/migration-v5.en.md)), `[deploy] disable_native_agents` (the closed world always disables native agents), `[workflow.overrides]` (migrated to `~/.oh/migrated/`, see [team workflows](../guides/team-workflows.en.md#migration-of-the-former-workflow-overrides-v5)). The former `[team]` section is migrated to `[[teams]]` on load.

### v5 sections

| Section | Key | Default | Role |
|---|---|---|---|
| `[session]` | `attach` | `auto` | How a session is opened (`OH_SESSION_ATTACH` and `--attach` come first) |
| | `iterm_style` | `tab` | iTerm2 tab, split or window |
| | `idle_sleep_minutes` | `5` | Sleep of a server without activity or pending decision |
| | `notify` | `on` | System notifications of the daemon (pending decision, end of turn) |
| `[execution]` | `runtime` | empty | Preferred environment (Settings › Execution), used when the workflow allows it |
| | `engine` | `auto` | Container engine |
| | `keep_images` | `2` | Images kept per project and role (base, dev) |
| | `opencode_version` | empty | opencode version of the images; when it differs from the machine client, container launches are refused |
| | `strict_isolation` | `false` | Also hides the personal opencode configuration (`XDG_CONFIG_HOME`) from local servers |
| `[remote]` | `targets[]` | — | `oh-runner` targets (`oh remote setup`): `name`, `url`, `group`, `runner_project`, `token_key`, `trigger_key`, `tag`, `builder` (`kaniko`\|`dind`), `arch` (`amd64`\|`arm64`), `timeout` |
| | `projects` | — | oh project → target name |
| `[limits]` | `max_active_sessions`, `session_budget_usd`, `daily_budget_usd`, `memory_mb`, `models` | not set | Hub I6 restrictions (`oh budget set`) |
| `[deploy]` | `instruction_files` | `[]` | Project files added to the instructions of the bundle agents |
| `[websearch]` | `enabled` | `false` | WebSearch / WebFetch permissions |

See [container](../guides/container.en.md), [remote runners](../guides/remote-runners.en.md) and [v5 sessions](../guides/sessions-v5.en.md#restrictions).

---

## Config Commands

| Command | Description |
|---------|-------------|
| `oh config list [--json]` | Show all values |
| `oh config get <key>` | Get a value (dot notation: `opencode.default_provider`) |
| `oh config set <key> <value>` | Set a value |
| `oh config unset <key>` | Remove a key |
| `oh config path` | Print the file path |
| `oh config language [fr\|en]` | Get or set the display language |
| `oh config websearch [enable\|disable\|status]` | Manage WebSearch for agents |
| `oh config model agent\|family\|default\|show\|unset` | Model cascade (see [model resolution](model-resolution.en.md)) |
| `oh budget show\|set\|unset\|raise` | I6 restrictions of the hub or of a project |
| `oh remote setup\|status` | Remote execution targets |

---

## Project Storage

Projects are stored in a **SQLite database**: `~/.oh/oh.db`.

### Project Fields

| Field | Description |
|-------|-------------|
| ID | Auto-generated slug + UUID prefix |
| Name | Human-readable project name |
| Path | Absolute path |
| Language | go, typescript, python, rust, java, etc. |
| Provider | Provider override, or empty for the hub default |
| Model | Project model, or empty for the hub default |
| ModelOverrides | Per-family and per-agent models of the project |
| MCPConfig | Project MCP configuration (overrides the hub) |
| ProviderConfig | Project provider configuration (overrides the hub) |
| TrackerConfig | Project tracker overrides (tracker project and pattern) |
| ExecConfig | Execution configuration (see below) |
| TeamID | Team of the project (`[[teams]].id`), empty = project without a team |
| Status | active, archived |
| CreatedAt / UpdatedAt | Timestamps |

### Project Execution configuration

Set in the TUI (Project config › Execution):

| Field | Default | Role |
|---|---|---|
| Dockerfile | detected: `Dockerfile.dev`, `dev.Dockerfile`, `.devcontainer/Dockerfile`, `Dockerfile` | Dev Dockerfile of the project image (container) |
| Build arguments | — | `build_args` of the dev image |
| Volumes | — | Persistent cache volumes (absolute path in the container, or relative to each mounted location, e.g. `node_modules`) |
| Default workflow | none | Launched by `oh run` without argument, first in the "Start" block |
| Default runtime | empty | `local`, `container` or `remote`, used when the workflow allows it |

Runtime order: `--runtime` > project > Settings (`[execution] runtime`) > workflow `runtime.default`.

### Project Commands

| Command | Description |
|---------|-------------|
| `oh project list` | List projects |
| `oh project add` | Register a project |
| `oh project remove` | Unregister a project |
| `oh project rename` | Rename a project |
| `oh project move` | Update a project's path |
| `oh project configure` | Change project settings (provider, model, tracker) |

---

## Notification Configuration

Team notifications are configured in the **team-state** `config.toml` (not in `hub.toml`). They are managed via `oh team init` or by editing the team-state configuration. Session **system** notifications are set in `[session] notify` of `hub.toml`.

See the [team setup guide](../guides/team-setup.en.md) and the [configuration guide](../guides/configuration-guide.en.md#5-team-configuration).

```toml
# team-state config.toml (NOT hub.toml)
[notification]
enabled = true

[[notification.destinations]]
type = "slack"           # mattermost | slack | discord | teams
webhook_url = "https://hooks.slack.com/services/..."
bot_name = "OpenHub"

[[notification.destinations]]
type = "discord"
webhook_url = "https://discord.com/api/webhooks/..."
```

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `enabled` | bool | false | Enable notifications |
| `destinations[].type` | string | — | Backend: mattermost, slack, discord, teams |
| `destinations[].webhook_url` | string | — | Incoming webhook URL |
| `destinations[].channel` | string | — | Channel name (Mattermost only) |
| `destinations[].bot_name` | string | "OpenHub" | Bot display name |

---

## Multi-Team Configuration (ADR-029)

A user can belong to **several teams**. Each team is an entry of the `[[teams]]` array; a project references a team by its `id`.

### Hub level: `[[teams]]`

```toml
[[teams]]
id = "acme"                              # local short identifier
name = "Equipe ACME"                     # display name (optional, falls back to id)
enabled = true                           # enable team features
state_repo = "git@gitlab.com:acme/team-state.git"
state_path = ""                          # empty = ~/.oh/team-states/<host>/<repo>
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
| `enabled` | bool | No | Enable / disable this team |
| `state_repo` | string | Yes (except solo) | Git URL of the team-state repo |
| `state_path` | string | No | Local clone path (derived from `state_repo`) |
| `member_id` | string | Yes | Your identity in `members.toml` |
| `solo` | bool | No | Solo space (see below) |

### Solo space (`solo = true`)

`oh team init --solo` adds a local team, without `state_repo`:

```toml
[[teams]]
id = "solo"
enabled = true
solo = true
state_path = "~/.oh/teams/solo"
member_id = "alice"
```

A solo space only holds workflows: team features stay off and it is never the active team. `oh team promote --remote <url>` clears `solo` and sets `state_repo`.

### Project level: `team_id`

| Value | Meaning |
|-------|---------|
| empty | Project without a team |
| `"acme"` | The project belongs to team "acme" |

### Resolution cascade

```
project.TeamID empty         →  project without a team, team features off
project.TeamID == "acme"     →  lookup teams[] by id, use that team's config
project.TeamID unknown       →  graceful degradation (team disabled)
```

### Migration from `[team]` (legacy)

The legacy single `[team]` section is migrated to `[[teams]]` on first load:
- the ID is derived from the `state_repo` URL (last segment, without `.git`);
- a backup of `hub.toml` is made before writing;
- projects with `Mode: "inherit"` → `TeamID = <derived id>`;
- projects with `Mode: "disabled"` → empty `TeamID`.

### CLI Commands

| Command | Description |
|---------|-------------|
| `oh teams list` | List configured teams |
| `oh teams add --repo <url> --member-id <id>` | Add a team |
| `oh teams remove <team-id>` | Remove a team (projects lose their team) |

---

## Recommended or Enforced Configuration (ADR-030)

A team-state setting is either **recommended** (overridable) or **enforced** (locked).

### Resolution cascade of a setting

```
1. Enforced by the team?        → YES: team value (locked)
                                → NO: continue
2. Explicit project override?   → YES: project value
                                → NO: continue
3. Value in the hub?            → YES: hub value
                                → NO: continue
4. Recommended by the team?     → YES: team recommendation
                                → NO: system default
```

I6 restrictions (`[limits.*]`) follow their own rule: the most specific value wins, a value enforced by the team is a ceiling (see [`[limits]`](#limits-section)).

### TOML schema in the team-state `config.toml`

```toml
[mcp.gitlab]
enabled = true
enabled_enforced = true    # members MUST have GitLab enabled
url = "https://gitlab.company.com"
url_enforced = true        # the URL cannot be overridden locally
write_recommended = true   # informational: writing stays set by the hub write_enabled

[mcp.jira]
enabled = true             # recommended (no _enforced = overridable)
```

### TUI display

- 🔒 **Enforced**: lock icon, field not editable, toast on edit attempt
- `[team: recommended]`: origin when the value comes from the team
- `[hub]`, `[project]`: origin of local values

---

## Per-project Team Configuration (Legacy)

> **Deprecated:** `ProjectTeamConfig` (`mode` field) is superseded by `project.TeamID`. Projects in the old format are still supported.

| Field | Type | Description |
|-------|------|-------------|
| `mode` | string | `"inherit"` (default) · `"custom"` · `"disabled"` |
| `state_repo` | string | Git URL of the team-state _(custom only)_ |
| `state_path` | string | Local clone path, derived from `state_repo` when empty _(custom only)_ |
| `member_id` | string | Identity, otherwise the hub `member_id` _(custom only)_ |

### Session bundle: `OH_TEAM_ID` (formerly `.opencode/team.json`)

At launch, oh resolves the effective config and declares the `team` MCP server in the session bundle with `OH_TEAM_ID` and `OH_PROJECT_ID` in its environment; the server reads them to find the team of the session project. `.opencode/team.json` is no longer written nor read (`oh deploy` removed in v5; leftovers are removed by `oh migrate deploy-cleanup`).

When the team is disabled for the project, the `team` MCP server is not put in the bundle.

### Clone path auto-derivation

When `state_path` is empty, it is derived from `state_repo`:

```
git@gitlab.com:acme/other-team.git  →  ~/.oh/team-states/gitlab.com/other-team
https://github.com/acme/my-team.git →  ~/.oh/team-states/github.com/my-team
```

A former `~/.oh/team-states/<repo>` clone (without host) is reused when it exists.

### Setting the team of a project

- **At creation:** the `oh project add` wizard has a Team step.
- **Afterwards:** `team configure` in the TUI omnibar; applied at the next launch (bundle rebuilt).

---

## Team-State Configuration (`config.toml`)

**Location:** root of the team-state clone (`~/.oh/team-states/<host>/<repo>/config.toml`, solo space: `~/.oh/teams/<id>/config.toml`)  
**Purpose:** shared team settings (notifications, claims, tracker, board, MCP, models, workflow governance, restrictions).  
This file is versioned in the team-state repository — **not** in `hub.toml`.

> **Credentials are not stored here.** Tokens are read from each member's keychain (`[mcp.gitlab]`, `[mcp.jira]` of `hub.toml`, or `tracker_token_key`).

### `[claim]` section

```toml
[claim]
done_retention_days = 7
```

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `done_retention_days` | int | `7` | Days a claim stays `done` before being purged (`CleanupDoneClaims`) |

### `[tracker]` section

Links the team-state to an external tracker (GitLab or Jira).

```toml
[tracker]
enabled = true
type = "gitlab"              # "gitlab" or "jira"
tracker_url = ""             # tracker instance; empty = MCP URL
tracker_project = "group/app"   # GitLab project (ID or path) or Jira key
ticket_pattern = "APP-(\\d+)"   # regex with one capture group → external IID
auto_sync = true             # sync when opening team views
sync_interval_minutes = 5    # interval while the board is open
auto_plan_assigned = true    # "planned" claims for assigned issues
max_auto_plan_per_member = 5
auto_plan_unassigned = false # pool claims for unassigned issues
push_labels = false          # push labels back to the tracker
write_enabled = false        # tracker writes (labels, assignment)

[tracker.label_status_mapping]
"DEV DOING" = "in_progress"
"TO REVIEW" = "review"
```

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `enabled` | bool | `false` | Enable the integration |
| `type` | string | — | `"gitlab"` or `"jira"` (`type_enforced` to enforce it) |
| `tracker_url` | string | empty | Tracker instance, independent from the MCP |
| `tracker_token_key` | string | `openhub.tracker.<type>.token` | Keychain key of the token (otherwise the MCP token) |
| `tracker_project` | string | empty | Default tracker project (overridable per project) |
| `ticket_pattern` | string | empty | Regex with **one capture group** extracting the IID |
| `auto_sync` | bool | `false` | Sync when opening team views |
| `sync_interval_minutes` | int | `5` with `auto_sync`, otherwise `0` | Sync interval while the board is open (`0` = disabled) |
| `auto_plan_assigned` | bool | `false` | `planned` claims for issues assigned to members |
| `max_auto_plan_per_member` | int | `5` | Cap on auto-planned claims per member |
| `auto_plan_unassigned` | bool | `false` | Pool claims (taken with `c`) for unassigned issues |
| `unassigned_labels` | list | `[]` | Labels required on unassigned issues |
| `max_unassigned_issues` | int | `20` | Cap on unassigned issues per project |
| `push_labels` | bool | `false` | Push claim labels back to the tracker (`push_labels_enforced` to enforce it) |
| `write_enabled` | bool | `false` | Allow tracker writes |
| `status_mapping` | map | `{}` | Tracker status → claim status |
| `label_status_mapping` | map | `{}` | Label → claim status (the first matching label wins) |
| `ticket_patterns`, `[tracker.projects]` | map | `{}` | Former per-project format (deprecated, still read) |

**Notes:**

- Jira: closed issues are detected via `statusCategory.key == "done"`.
- Valid claim statuses: `planned`, `in_progress`, `review`, `validation`, `blocked`, `done`.
- The `hub.toml` `[tracker]` section lets a member turn off `enabled`, `auto_sync`, `push_labels`, `auto_plan_assigned`, etc. locally.

### `[board]` section

Column layout of the team board. Without this section: 6 columns (TODO, IN PROGRESS, REVIEW, VALIDATION, DONE, BLOCKED).

| Role | Purpose |
|------|---------|
| `initial` | Entry column: new tickets land here (pool claims, `--planned` flag) |
| `active` | Work in progress: counted in badges and summaries |
| `terminal` | Completion: triggers the cleanup of done claims |
| `blocked` | Impediment: counted separately in summaries |

```toml
[board]
columns = [
    { id = "todo",        name = "TODO",         role = "initial" },
    { id = "in_progress", name = "IN PROGRESS",  role = "active" },
    { id = "review",      name = "CODE REVIEW",  role = "active" },
    { id = "testing",     name = "TESTING",      role = "active",  color = "cyan" },
    { id = "done",        name = "DONE",         role = "terminal" },
    { id = "blocked",     name = "BLOCKED",      role = "blocked" },
]
```

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `columns` | array | 6 columns | Ordered columns |
| `columns[].id` | string | — | Internal id, lowercase, no space, `..` or `/`; claim status value |
| `columns[].name` | string | — | Header label |
| `columns[].color` | string | role-based | `orange`, `blue`, `gray`, `cyan`, `green`, `red`, `purple`, `yellow` |
| `columns[].role` | string | `"active"` | `initial`, `active`, `terminal`, `blocked` |

> **Tip:** the `Discover Tracker` command (omnibar or `y` key in team config) configures the columns from the tracker labels. Without `[board]`, existing `"planned"` claims go to the first column.

### `[mcp.<service>]` section

Recommendations or obligations on MCP servers (see [recommended or enforced](#recommended-or-enforced-configuration-adr-030)): `enabled`, `enabled_enforced`, `url`, `url_enforced`, `write_recommended`.

### `[models]` section

```toml
[models]
default = "claude-sonnet-4-20250514"
[models.families]
quality = "claude-opus-4-20250514"
[models.agents]
reviewer = "claude-opus-4-20250514"
```

Team model recommendations, edited in the TUI (Team › Models). **Not applied at launch in v5**: the cascade of a session has no team level (see [model resolution](model-resolution.en.md)).

### `[governance]` section

```toml
[governance]
publish = "any_member"   # default value, the only one supported
```

Who may publish the team workflows. An unknown value blocks publication. See [team workflows](../guides/team-workflows.en.md#governance).

### `[limits]` section

```toml
[limits.recommended]     # default value, overridable by the hub, project or workflow
session_budget_usd = 5

[limits.enforced]        # ceiling no other level can loosen
daily_budget_usd = 50
models = ["eu.anthropic.claude-*"]
```

Keys: `max_active_sessions`, `session_budget_usd`, `daily_budget_usd`, `memory_mb`, `models`. Cascade: hub → team (recommended / enforced) → project → workflow (`limits:`). See [ADR-044](../architecture/adr/044-credential-proxy-session-limits.en.md) and `oh budget show`.

### `[takeover]` and `[parallel]` sections

| Field | Default | Description |
|---|---|---|
| `takeover.stale_days` | `3` | Days of inactivity before a claim is considered stale (takeover briefs) |
| `parallel.max_sessions` | `5` | Left over from the former parallel mode (removed in v5); read but without effect, no longer editable in the team detail (to limit sessions: `[limits]`, `oh budget`) |

---

## Notifications View

The TUI keeps every toast in an in-memory store and makes the full history available in a dedicated view.

### Accessing the view

| Command (omnibar) | Description |
|---------|-------------|
| `notifications` | Open the notification history |
| `notif` / `logs` / `messages` / `toasts` / `erreurs` | Aliases |

### What it shows

```
14:32:05  ✗  Erreur setup team : git clone https://gitlab.com/...: fatal: repository not found
14:31:58  ✓  Team configured: custom — applied at the next session launch
14:31:52  →  Initialisation team pour ce projet...
```

Each entry has:
- **Timestamp** (`HH:MM:SS`)
- **Level icon**: `✓` success · `✗` error · `!` warning · `→` info
- **Full message**: on-screen toasts are truncated (80 characters for errors and warnings, 50 for others); the store keeps the full text

The view scrolls with `j` / `k`.

### Store behaviour

- **Capacity:** 50 entries in memory (FIFO).
- **Persistence:** appended to `~/.oh/notifications.jsonl` after each toast. Rotated at TUI startup (500 lines max, entries older than 7 days removed). The view loads the last 50 entries each time it opens.
- **Stderr log:** errors are also written to stderr (`[ERROR] <message>`).

---

## Text Selection

The TUI lets you select text with the mouse outside interactive widgets (omnibar, modals), with no external dependency.

### How to use

1. **Click and drag** to select (reverse video)
2. **Release**: the text is copied to the clipboard
3. **Double-click**: selects the word
4. **Triple-click**: selects the line
5. **Esc**: clears the selection

### Clipboard

| Strategy | Platforms |
|----------|-----------|
| OSC 52 escape sequence via `/dev/tty` | Modern terminals (iTerm2, WezTerm, Alacritty, kitty, tmux with `set-clipboard on`), works over SSH |
| `pbcopy` | macOS fallback |
| `wl-copy` | Linux/Wayland fallback |
| `xclip` / `xsel` | Linux/X11 fallback |
| `clip.exe` | Windows fallback |

Both strategies are attempted; one success is enough.

### Interactive zones

Mouse events in these zones are passed to tview without starting a selection:

- omnibar (container, input field, suggestions);
- any modal (prompts, select lists).

### Toasts

Toasts are **selectable**: clicking on one starts a selection.

---

## Secrets / API Keys

Secrets are stored in the **OS keychain** (macOS Keychain, Linux secret-service, Windows Credential Manager); `~/.oh/secrets-index.json` lists the known keys (without values).

### Fallback

When the keychain is unavailable, secrets are stored in `~/.oh/secrets.enc` (AES-256-GCM, Argon2id key derivation).

**Passphrase source:**

1. `OH_PASSPHRASE` environment variable
2. Terminal prompt (confirmation on first use, 8 characters minimum)

### Commands

| Command | Description |
|---------|-------------|
| `oh mcp setup` | Store an MCP service token |
| `oh provider` | Configure provider credentials |
| `oh secrets` | List and manage secrets |

### Known keys

| Key | Purpose |
|-----|---------|
| `openhub.provider.bedrock.token` | AWS Bearer token for Bedrock |
| `openhub.provider.anthropic.token` | Anthropic API key |
| `openhub.provider.openrouter.token` | OpenRouter API key |
| `openhub.provider.<provider>.token.<project>` | Project-specific credentials |
| `openhub.team.<team>.provider.<provider>.token` | Credentials provided by a team |
| `openhub.mcp.<service>.token` | MCP server token (`figma`, `gitlab`, `jira`, `gslides`…) |
| `openhub.team.<team>.gitlab.token` | Team-specific GitLab token |
| `openhub.tracker.<type>.token` | Team tracker token |
| `openhub.remote.<target>.token` / `.trigger` | API token and trigger token of a remote target |
| `openhub.daemon.capability` | Local capability of the `ohd` daemon |

Former names (`bedrock-token-default`, `gitlab-token`, `figma-token`…) are renamed automatically on load.

The real provider keys are never given to sessions: the daemon's **credential proxy** gives each server group an `ohs_…` token ([ADR-044](../architecture/adr/044-credential-proxy-session-limits.en.md)).

---

## Session Configuration (session bundle)

The project's `opencode.json` is no longer generated (`oh deploy` removed in v5). At each launch, oh builds a **session bundle** from the workflow (`~/.oh/bundles/<hash>/`, immutable); the adapter derives the opencode configuration of the session from it (bundle agents only, permissions, MCP, workflow plugins, model) and passes it to the `opencode serve` server.

> **Note:** inspect it with `oh bundle show <workflow>` (`--json`, `--budget`). In a project deployed by an older version, `oh migrate deploy-cleanup` removes from `opencode.json` only the keys oh wrote that are unchanged since the last deploy (your own keys are kept).

---

## Environment Variables

| Variable | Purpose |
|----------|---------|
| `OH_HOME` | Relocates the `~/.oh` directory (tests, isolated environments) |
| `OH_PASSPHRASE` | Passphrase of the encrypted secret store (fallback) |
| `OH_SESSION_ATTACH` | How sessions are opened (comes before `[session] attach`) |
| `OH_RICH_TUI` | `0` disables the TUI (inline prompts only) |
| `OH_DAEMON_INPROCESS` | `1` runs the daemon inside the oh process (always the case on Windows) |
| `CI` | Not empty: no TUI |
| `TERM` | `dumb`: no TUI |
| `TMUX`, `TERM_PROGRAM` | tmux and iTerm2 detection to open sessions |
| `OH_SESSION_ID`, `OH_TEAM_ID`, `OH_PROJECT_ID` | Set by oh in the environment of sessions and MCP servers (do not set by hand) |
| `AWS_PROFILE`, `AWS_REGION`, `AWS_DEFAULT_REGION`, `AWS_ACCESS_KEY_ID`, `AWS_SECRET_ACCESS_KEY`, `AWS_BEARER_TOKEN_BEDROCK` | Bedrock credential detection (`oh init`, `oh doctor`) |
| `ANTHROPIC_API_KEY`, `OPENROUTER_API_KEY` | Anthropic and OpenRouter credential detection |
| `FIGMA_TOKEN` | Figma API token (MCP server) |
| `GITLAB_TOKEN`, `GITLAB_URL`, `GITLAB_WRITE_ENABLED` | GitLab MCP server (default URL: `https://gitlab.com`) |
| `GOOGLE_ACCESS_TOKEN` | Google OAuth token (MCP server) |
| `GITHUB_TOKEN` (alias `GH_TOKEN`), `GITHUB_WRITE_ENABLED` | GitHub MCP server |
| `JIRA_URL`, `JIRA_TOKEN` (or `JIRA_USER` + `JIRA_API_TOKEN`), `JIRA_WRITE_ENABLED` | Jira MCP server |
| `LINEAR_API_KEY`, `LINEAR_WRITE_ENABLED` | Linear MCP server |
| `SSL_CERT_FILE` | Certificates passed to the server of remote sessions (on the runner) |
| `PAGER` | Help pager (default: `less`) |

MCP server environment variables come before the keychain (see [MCP servers](services.en.md#runtime-environment-variables)).

---

## Provider Resolution

When a session starts, the LLM provider is resolved in this order:

1. `--provider` / `-P` flag
2. `project.Provider` in the database
3. `opencode.default_provider` in `hub.toml`
4. `"bedrock"` (hardcoded fallback)

---

## File Locations Summary

| Path | Purpose |
|------|---------|
| `~/.oh/` | Configuration directory (`OH_HOME`) |
| `~/.oh/hub.toml` | Hub configuration |
| `~/.oh/hub/` | Extracted hub content (agents, skills, workflows) |
| `~/.oh/oh.db` | SQLite database (projects, sessions, decisions) |
| `~/.oh/secrets.enc` | Encrypted secrets (fallback) |
| `~/.oh/secrets-index.json` | Index of the keychain keys |
| `~/.oh/bundles/<hash>/` | Session bundles built at launch — nothing is deployed into `<project>/.opencode/` or `<project>/opencode.json` anymore |
| `~/.oh/run/` | `ohd` daemon (socket, logs) |
| `~/.oh/servers/`, `~/.oh/sessions/<id>/` | State of servers and sessions (including `limits.json`) |
| `~/.oh/cache/` | Cache (container images…) |
| `~/.oh/team-states/<host>/<repo>/` | Team-state clones |
| `~/.oh/teams/<id>/` | Solo spaces |
| `~/.oh/migrated/` | Archived former workflow overrides |
| `~/.oh/notifications.jsonl` | TUI notification history |
| `~/.oh/mcp/<name>/manifest.json` | Custom MCP server manifest |

> **Database integrity:** `oh repair --check-only` checks the SQLite database; `oh repair` shows the schema version.
>
> **Web dashboard:** `oh serve` binds to `127.0.0.1` only, never to the network.
