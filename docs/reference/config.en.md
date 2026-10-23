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
compact = "auto"                   # compact TUI: auto (under 32 rows, default) | always | never

[llm]
default_provider = "bedrock"       # bedrock | anthropic | openrouter | github-copilot

[provider.bedrock]
aws_profile = "default"            # AWS profile (bedrock only)
aws_region = "eu-west-1"           # AWS region (bedrock only)
auth_mode = "bearer"               # "bearer" (stored API key only) | "profile" | "env" (SigV4, stored key ignored) — bedrock only; absent: stored keys then AWS profile

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
trusted_url = ""                   # URL imposed by the team that you approved (oh mcp trust)

[mcp.jira]
enabled = false

[mcp.gslides]
enabled = false
token_key = "openhub.mcp.gslides.token"

[worktree]
auto_cleanup = true                # oh gc periodically removes the worktrees oh created (clean, pushed, unused)
base_branch = ""                   # empty = auto-detect (main/master)
branch_pattern = "oh/%s"           # branch naming (%s = worktree name)

[deploy]
instruction_files = []             # project files added to the agent instructions (besides ONBOARDING.md, CONVENTIONS.md, .claude/CLAUDE.md)

[tracker]                          # local overrides of the team tracker sync
# enabled = false                  # uncomment to disable sync locally
# tracker_token_key = "my.token"   # keychain key of the tracker token
# tracker_token_url = "https://gitlab.example.com"  # server of that key (default: the tracker)
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
tool_version = ""                  # tool (opencode) version of the images; empty = the machine's
strict_isolation = false           # also hide the personal opencode config locally
env_allow = []                     # variables of your environment passed to the agents besides the base list: names or PREFIX_*
proxy_first_byte_timeout_minutes = 5  # wait of the credential proxy for a provider to start answering

[fanout]
experimental = false               # experimental fan-out (R6): child sessions of implement-spec
parallel = 1                       # child sessions at the same time (1 to 3)
budget_usd = 10                    # global budget of a parent session and its children

[remote]
experimental = false              # experimental remote mode (5.0): true to try it
[[remote.targets]]                 # oh-runner projects (one per GitLab instance and group)
name = "acme"
url = "https://gitlab.com"
group = "acme"
# runner_project = "acme/oh-runner"   # default: <group>/oh-runner
# tag = "oh"  builder = "kaniko"  arch = "amd64"  timeout = "3h"
[remote.projects]                  # oh project → target (otherwise automatic choice)
# web-app = "acme"

[beads]
# zero_impact = "ask"              # ask | auto | off (files written by bd init)

[gc]                               # clean-up (oh gc, and the daemon every every_hours)
enabled = true
# every_hours = 24  session_days = 30  history_days = 180  usage_days = 400
# bundle_grace_hours = 24  log_max_mb = 20  log_keep = 3

[limits]                           # hub I6 restrictions (off by default)
# max_active_sessions = 4
# session_budget_usd = 5
# daily_budget_usd = 30
# memory_mb = 4096
# models = ["eu.anthropic.claude-*"]
```

Removed in v5 (ignored if still in the file): `[opencode] version`, `channel`, `auto_update`, `install_dir` (opencode V2 is installed separately, see [v5 migration](../guides/migration-v5.en.md)), `[deploy] disable_native_agents` (the closed world always disables native agents), `[workflow.overrides]` (migrated to `~/.oh/migrated/`, see [team workflows](../guides/team-workflows.en.md#migration-of-the-former-workflow-overrides-v5)). The former `[team]` section is migrated to `[[teams]]` on load. Renamed in v5 (former names still read, written under the new name at the next save): `[opencode] default_provider` → `[llm] default_provider` (`oh config set llm.default_provider`; the former key is accepted), `[execution] opencode_version` → `[execution] tool_version`. Reading a former key is announced once per command (a warning with the file, the line and the current key), `oh doctor` lists them, and **v6 stops reading them** (same for the former keychain names of the MCP tokens, `<service>-token`, which `oh doctor` also lists). Any write of `hub.toml` (`oh config set …`, the TUI) replaces them with the current keys. See [Strict reading of `hub.toml`](#strict-reading-of-hubtoml).

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
| | `tool_version` | empty | opencode version of the images; when it differs from the machine client, container launches are refused |
| | `strict_isolation` | `false` | Also hides the personal opencode configuration (`XDG_CONFIG_HOME`) from local servers; `Attest` then refuses any plugin the bundle does not bring (without it: a warning only); also refuses an opencode Linux package (container images) whose version has no integrity pinned in oh (without it: a warning) |
| | `env_allow` | `[]` | Variables of your environment that reach the tool server and the shells of the agents of local sessions, **besides a base list**: names (`SSH_AUTH_SOCK`) or `PREFIX_*` patterns (`MYCO_*`). Applies when a server starts (a running group keeps its environment until it sleeps). Containers and remote jobs never inherit your environment. See [The environment of the agents](#the-environment-of-the-agents) Editable in Settings › Execution (same rule as `oh config set`). |
| | `proxy_first_byte_timeout_minutes` | `5` | How long the credential proxy waits for a model provider to start answering (headers) before it gives up with a 504. A stream that has started has no time limit. Raise it for very slow models or non-streamed requests that generate a lot Editable in Settings › Execution (same rule as `oh config set`). |
| `[fanout]` | `experimental` | `false` | Enables the fan-out, **experimental** (R6): without it, `oh run implement-spec -i fanout=true` is refused and the daemon delegates no checkpoint of a child ([guide](../guides/fan-out.en.md)) |
| | `parallel` | `1` | Child sessions at the same time (1 to 3; at most the `fanout.parallel` of the workflow) |
| | `budget_usd` | `10` | Global budget of a parent session and all its sessions (children, merger), in dollars (F-04) |
| `[remote]` | `experimental` | `false` | Enables the remote mode, **experimental** in 5.0 (never validated on a real runner): without it, `oh remote setup`, `oh run --runtime remote`, the ☁ option, `oh session fetch` and `oh session resolve` are refused |
| | `targets[]` | — | `oh-runner` targets (`oh remote setup`): `name`, `url`, `group`, `runner_project`, `token_key`, `trigger_key`, `tag`, `builder` (`kaniko`\|`dind`), `arch` (`amd64`\|`arm64`), `timeout` |
| | `projects` | — | oh project → target name |
| `[limits]` | `max_active_sessions`, `session_budget_usd`, `daily_budget_usd`, `memory_mb`, `models` | not set | Hub I6 restrictions (`oh budget set`) |
| `[deploy]` | `instruction_files` | `[]` | Project files added to the instructions of the bundle agents |
| `[websearch]` | `enabled` | `false` | WebSearch / WebFetch permissions |
| `[beads]` | `zero_impact` | `ask` | Files written by `bd init` outside `.beads/` when a project is registered or Beads initialized: `ask` (listed, then a confirmation; without a terminal, nothing), `auto` (no question; untracked settings files left alone), `off` (nothing). The commit of `bd init` is only undone after a confirmation in a terminal (`ask`) or with `--yes`, never in `auto` ([Getting started](../guides/getting-started.en.md)) |
| `[gc]` | `enabled` | `true` | Periodic clean-up by the daemon (`oh gc` always works) |
| | `every_hours` | `24` | Interval of the daemon clean-up (its first pass is a simulation; it cleans up after a first `oh gc`) |
| | `session_days` | `30` | Files of a **stopped** session kept (bundle, server data, folder, automatic worktree); completed or failed sessions can still be resumed |
| | `history_days` | `180` | History kept (stopped sessions, agent events, settled decisions, session results, applied or refused wiki proposals) |
| | `usage_days` | `400` | Usage ledger (costs, budget raises) kept |
| | `bundle_grace_hours` | `24` | A bundle used less than that ago is kept |
| | `log_max_mb`, `log_keep` | `20`, `3` | Size above which a log is shifted, copies kept |

See [container](../guides/container.en.md), [remote runners](../guides/remote-runners.en.md) and [v5 sessions](../guides/sessions-v5.en.md#restrictions).

### Strict reading of `hub.toml`

**Breaking change of v5.0.** Until v4, `hub.toml` went through a weakly typed reader (viper, mapstructure) that converted values silently: `"true"` was read as a boolean, `"3"` as a number, `"a,b"` as a list. v5.0 reads it with a strict TOML decoder: **each value must have the type of its setting**. A `hub.toml` written by oh (`oh init`, `oh config set`, the TUI) is always valid; only the files edited by hand are concerned.

| Written in `hub.toml` | Until v4 | From v5.0 | To do |
|---|---|---|---|
| `setup_done = "true"`, `enabled = "false"`… (a boolean in quotes, also `"1"`) | read as a boolean | **error** | write `true` / `false` without quotes |
| `setup_done = 1` (a number for a boolean) | read as `true` | **error** | `true` |
| `keep_images = "3"`, `runner_project_id = "42"` (a number in quotes) | read as a number | **error** | `3` without quotes |
| `keep_images = 2.7` (a decimal for an integer) | truncated to `2` | **error** | an integer |
| `budget_usd = "12.5"` | read as `12.5` | **error** | `12.5` |
| `member_id = 123`, `language = true` (a number or a boolean for a text) | converted to text | **error** | `"123"` |
| `env_allow = "A,B"`, `instruction_files = "X.md"`, `models = "a,b"` (a text for a list) | cut at the commas, or a list of one | **error** | `["A", "B"]` |
| `instruction_files = [1, 2]` (numbers in a list of texts) | converted to text | **error** | `["1", "2"]` |
| a date or a time where a text is expected | error | **error** | quote it |
| `[team]` (legacy section) | read, migrated to `[[teams]]` at startup | unchanged, but its values are typed too (`enabled = "true"` is an error) | |
| `[CLI]`, `Language = "fr"` (capitals in the names) | read | unchanged: names are matched without regard to case | |
| `budget_usd = 10` (an integer for a decimal) | read | unchanged | |
| `[models.agents] Reviewer = …`, `[models.families] Quality = …` | key lowercased | unchanged: key lowercased | |
| `[remote.projects] MyProj = …`, `[remote.targets.binaries]` | keys lowercased | keys **kept as written** | write them as the project ID is |
| `"cli.language" = "fr"` (a quoted key holding a dot) | split at the dot | **warning**: a key no setting uses, ignored | `[cli]` then `language = "fr"`, or `cli.language = "fr"` |
| a key or a table no setting uses (typo, key of a newer oh) | ignored without a word | **warning**, ignored | fix it or remove it |
| `[opencode]` (except `default_provider`), `[deploy] disable_native_agents`, `[workflow]` | ignored | ignored, without a warning (historical keys) | |

An error stops oh and names the file, the line, the column and the key of the first value at fault:

```
/home/me/.oh/hub.toml does not load: line 3, column 14, key cli.setup_done: expected a boolean, found a string (write the value without quotes)
```

`oh doctor` and `oh doctor --fix` work with such a file: the check shows the same message, and `--fix` restores the latest backup (`hub.toml.bak-*`) that loads. A key no setting uses is **only a warning**, so that a `hub.toml` written by a newer oh still loads; `oh doctor` lists those keys. They are not kept: the next write of `hub.toml` (a full rewrite, comments included) drops them.

**Environment variables.** `hub.toml` was never read through the environment (no automatic binding, no prefix): no variable sets a key, before or after v5.0. The variables oh reads are documented where they apply (`OH_HOME`, `OH_SESSION_ATTACH`, [the environment of the agents](#the-environment-of-the-agents)…).

**Defaults.** A key left out keeps its default (`name = "OpenHub"`, `cli.language = "en"`, `worktree.auto_cleanup = true`, `gc.enabled = true`), also inside a table that is present; a key written empty (`language = ""`) overrides it.

---

## The environment of the agents

In a local session, the tool server (opencode) and the shells of the agents do not inherit your whole environment: **only an allow-list passes**. A list of forbidden names always misses the secret of the day (`GEMINI_API_KEY`, `GH_TOKEN`, `NPM_TOKEN`, a database URL…); here what is needed is named, and the rest does not pass.

**The base list** (always passes):

| Family | Variables |
|--------|-----------|
| Process and identity | `PATH`, `HOME`, `USER`, `LOGNAME`, `SHELL`, `TERM`, `COLORTERM`, `TMPDIR`, `TMP`, `TEMP`, `TZ`, `NO_COLOR`, `CLICOLOR`, `CLICOLOR_FORCE`, `FORCE_COLOR` |
| Language | `LANG`, `LANGUAGE`, `LC_*` |
| XDG folders | `XDG_*` (the data folder of the tool server is set by oh) |
| Network | `HTTP_PROXY`, `HTTPS_PROXY`, `NO_PROXY`, `ALL_PROXY` (and lowercase), `SSL_CERT_FILE`, `SSL_CERT_DIR`, `NODE_EXTRA_CA_CERTS`, `REQUESTS_CA_BUNDLE`, `CURL_CA_BUNDLE`, `GIT_SSL_CAINFO` |
| Toolchains (locations, never credentials) | `GOPATH`, `GOROOT`, `GOCACHE`, `GOMODCACHE`, `GOBIN`, `JAVA_HOME`, `MAVEN_HOME`, `M2_HOME`, `GRADLE_HOME`, `GRADLE_USER_HOME`, `CARGO_HOME`, `RUSTUP_HOME`, `NVM_DIR`, `PNPM_HOME`, `VOLTA_HOME`, `BUN_INSTALL`, `PYENV_ROOT`, `VIRTUAL_ENV`, `CONDA_PREFIX`, `RBENV_ROOT`, `GEM_HOME`, `GEM_PATH`, `ASDF_DIR`, `ASDF_DATA_DIR`, `DOTNET_ROOT`, `ANDROID_HOME`, `ANDROID_SDK_ROOT`, `HOMEBREW_PREFIX` |
| oh (read by the processes oh starts under the server) | `OH_HOME`, `OH_WORKFLOWS_DIR`, `OH_WORKFLOW_REPORTS`, `OH_ALLOW_NEWER_SCHEMA`, `OH_SKIP_DB_BACKUP`, `OH_DAEMON_INPROCESS` |
| Windows | `SystemRoot`, `COMSPEC`, `PATHEXT`, `USERPROFILE`, `APPDATA`, `LOCALAPPDATA`, `ProgramFiles*`, `ProgramData`, `HOMEDRIVE`, `HOMEPATH`… |

The exact list is in `cli/internal/envfilter/envfilter.go`. Not in it, on purpose: `SSH_AUTH_SOCK` (it hands your ssh-agent keys to the agents), `ZDOTDIR`, `BASH_ENV` and `ENV` (shell start-up: oh sets them and loads yours, see [SECURITY](../../SECURITY.md)), every `*_KEY`, `*_TOKEN`, `*_SECRET`, `*_PASSWORD`.

**Adding variables**: `execution.env_allow` is a list of names or `PREFIX_*` patterns.

```bash
oh config set execution.env_allow "SSH_AUTH_SOCK, MYCO_*"   # a name, and every variable starting with MYCO_
oh config unset execution.env_allow
```

- A bare `*`, a pattern shorter than two characters before the star, or anything that is not a name are refused.
- The credentials that oh manages itself (`AWS_BEARER_TOKEN_BEDROCK`, `AWS_ACCESS_KEY_ID`, `AWS_SECRET_ACCESS_KEY`, `AWS_SESSION_TOKEN`, `AWS_PROFILE`, `ANTHROPIC_API_KEY`, `OPENAI_API_KEY`, `OPENROUTER_API_KEY`, `GITLAB_TOKEN`, `GITHUB_TOKEN`, `FIGMA_TOKEN`, `JIRA_TOKEN`, `LINEAR_API_KEY`, `GOOGLE_ACCESS_TOKEN`, `OH_PASSPHRASE`) stay removed whatever the list says: the agents only get the session token of the oh proxy.
- A secret-looking name you add (`*_KEY`, `*_TOKEN`, `*_SECRET`, `*_PASSWORD`…) is allowed but reported by `oh doctor` (⚠): the agents can read it and send it wherever their tools reach.
- `oh doctor` also lists, by name only, the variables of your environment that are held back, to help you decide what to add.
- The setting applies when a server starts (new group, restart, resume of a sleeping session). A running group keeps its environment.
- **Containers and remote jobs** never inherit your environment (they get oh's settings only): `env_allow` does not apply there.

---

## Where each setting is set

A setting can live in several places; the most specific wins, except what the team **enforces**. This page says where to look.

### The sources

| Source | Who sets it | Where | How |
|---|---|---|---|
| **Launch** | you, for one launch | flags and a few variables | `--provider`, `--attach`, `--runtime`, `OH_SESSION_ATTACH`… (`oh <command> --help`) |
| **Workflow** | the workflow author | `models:`, `limits:` of the YAML ([schema](workflow-schema.en.md)) | edit the workflow, `extends` |
| **Project** | you, for one project | database `oh.db` (preferences) | `oh project configure`, `oh config model … --project`, `oh budget set … -p` |
| **Hub** | you, for the machine | `~/.oh/hub.toml` (`$OH_HOME`) | `oh config`, `oh budget`, the TUI (Settings), editing the file ([strict reading](#strict-reading-of-hubtoml)) |
| **Team** | the team | `config.toml` of the team-state | the TUI (Team), `oh team` ([recommended or enforced](#recommended-or-enforced-configuration-adr-030)) |
| **Built in** | oh | the code, the agent files (`model:`) | — |

### The model of an agent

Resolved for each agent when the session bundle is built (first match wins):

| Order | Level | Set by |
|---|---|---|
| 1-2 | Workflow (`models.agents.<id>`, then `models.default`) | the workflow |
| 3-5 | Project (agent, family, default) | `oh config model … --project <p>` |
| 6-8 | Hub (agent, family, default) | `oh config model …`, `[models]` of `hub.toml` |
| 9-11 | Team recommendation (agent, family, default) | `[models]` of the team-state, TUI Team › Models |
| 12 | Frontmatter of the agent | `model:` of the agent file |

The workflow has no family level. The provider (`--provider`, project, `[llm] default_provider`, `bedrock`) normalizes the name. Detail, families and examples: [model resolution](model-resolution.en.md). `oh config model show` prints the levels in order, with what each one sets.

### The restrictions (I6)

Maximum of working sessions, budgets, memory and allowed models (`limits`). Off by default; for each field the most specific level that sets it wins:

| Order | Level | Set by |
|---|---|---|
| 1 | Workflow (`limits:`) — only `session_budget_usd` and `models` | the workflow |
| 2 | Project (memory excluded) | `oh budget set … -p <project>` |
| 3 | Hub | `[limits]` of `hub.toml`, `oh budget set …` |
| 4 | Team **recommended** | `[limits.recommended]` of the team-state |
| ceiling | Team **enforced** | `[limits.enforced]` of the team-state |

A value enforced by the team is a ceiling that no other level can loosen: a smaller number wins, and the model list keeps only the patterns the ceiling covers (the ceiling itself when none is). The memory cap is a machine setting (hub and team only). The daily budget and the maximum of sessions count all the sessions of the machine when the hub or the team sets them, and those of the project when the project does. `oh budget show` prints each effective value with the level it comes from.

---

## Config Commands

| Command | Description |
|---------|-------------|
| `oh config list [--json]` | Show all values |
| `oh config get <key>` | Get a value (dot notation: `llm.default_provider`) |
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
url_enforced = true        # the URL cannot be overridden locally (used once each member approved it: oh mcp trust)
write_recommended = true   # informational: writing stays set by the hub write_enabled

[mcp.jira]
enabled = true             # recommended (no _enforced = overridable)
```

### A URL imposed by the team must be approved

The token of an MCP service is sent to its URL, and the team-state is shared: a URL a team **imposes** (`url_enforced = true`) is not used as soon as it is written. Each member approves it with [`oh mcp trust <service>`](cli-mcp.en.md#oh-mcp-trust), which pins it in the personal `hub.toml` (`[mcp.<service>] trusted_url`, never in the team-state). Step 1 of the cascade uses the imposed URL only if it equals the pinned one after normalization (scheme and host in lower case, no trailing slash); otherwise it is skipped, the token is not sent to it, the cascade goes on (project, hub, default) and `oh doctor`, `oh mcp status` and the MCP view of the TUI warn that a URL is waiting. A URL imposed by the team that is not `https` (`http` is accepted for `localhost`, `127.0.0.1` and `::1`) is refused and cannot be pinned. A URL that the team only *recommends* is not concerned: it is still used when you have no URL of your own.

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

A former `~/.oh/team-states/<repo>` clone (without host) is reused when it exists. With `OH_HOME`, the clones live under `$OH_HOME/team-states/`.

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
| `tracker_token_key` | string | `openhub.tracker.<type>.token` | Keychain key of the token (otherwise the token of the team or of the MCP, for their own server) |
| `tracker_token_url` | string | empty | Server the token of `tracker_token_key` (and of the former tracker key) is valid for, e.g. `https://gitlab.example.com`. Without it the key is valid for the tracker (see [the server of each token](#the-server-of-each-gitlab-token)) |
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
- The `hub.toml` `[tracker]` section lets a member turn off `enabled`, `auto_sync`, `push_labels`, `auto_plan_assigned`, etc. locally, and set their own token key (`tracker_token_key` + `tracker_token_url`, which go together).

### The server of each GitLab token

A GitLab token is only sent to the server it was issued for. When the team repository and the tracker are on two different GitLab instances, the token of one is never shown to the other. Every token source is therefore tied to a server, and the lookup order (see [`oh team sync-tracker`](cli-team.en.md#oh-team-sync-tracker)) only keeps a source whose server is the one being called:

| Source | Key | Server of the source | To change it |
|--------|-----|----------------------|--------------|
| Environment variable | `GITLAB_TOKEN` | **must be declared**: `OH_GITLAB_URL`, otherwise `GITLAB_URL` | `export OH_GITLAB_URL=https://gitlab.example.com` |
| `tracker_token_key` and former tracker key | the key you set, `openhub.tracker.gitlab.token` | `tracker_token_url` when declared; otherwise (**implicit**) the tracker: the `tracker_url` of the level that sets the key, else the effective `tracker_url` (then `GITLAB_URL`, then gitlab.com) | `[tracker] tracker_token_url` of `hub.toml` or of the team config; `oh config set tracker.tracker_token_url <url>` |
| Project token, hub MCP token | `openhub.mcp.gitlab.token.<project>`, key of `[mcp.gitlab] token_key`, `openhub.mcp.gitlab.token` | the URL of the GitLab MCP (project, `[mcp.gitlab] url`, otherwise the URL recommended by the team); with no URL (**implicit**) `https://gitlab.com`, where the GitLab MCP server goes then | `oh config set mcp.gitlab.url <url>` |
| Team token | `openhub.team.<team>.gitlab.token` | the host of the team repository (`state_repo`) | nothing to declare |

**Rule.** A source whose server is neither declared nor implicit is never used: only a lone `GITLAB_TOKEN` is strict (it needs `OH_GITLAB_URL` or `GITLAB_URL`). A declared server beats the implicit one. The implicit server of the tracker key only serves the tracker: a key set next to a `tracker_url` does not answer for another server (nor for the GitLab of the MCP). Servers are compared ignoring case, scheme (`https://h`, `http://h` and `h` are the same server), a default port (`80`, `443`), a trailing `/`, the path and the user; a non-standard port counts (`h:8443` ≠ `h`). For an `ssh://` or `git@host:path` repository only the host counts (the port is git's, not the API's). Two GitLab instances on one host that differ only by a path prefix are therefore the same server.

`tracker_token_url` is set together with the key (`tracker_token_key`): a `tracker_token_key` in `hub.toml` does not inherit the server declared next to the team's one. With no token for the server being called, the command stops with the server, the sources read (key, empty / for another server / server not declared, flagged "implicit" when the server is the tracker's or gitlab.com), how to declare the key and the command to run; an authentication error (HTTP 401 or 403) names the source of the token, its key and the server.

> **Migration (v5.1).** Only a lone `GITLAB_TOKEN` needs a declaration: add `export OH_GITLAB_URL=https://…` (or `GITLAB_URL`). Tracker keys and MCP tokens keep working for their implicit server; declare `tracker_token_url` or `mcp.gitlab.url` only if their server is another one.

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

Team model recommendations, edited in the TUI (Team › Models). Applied at launch below the workflow, the project and the hub, above the agent frontmatter (see [Where each setting is set](#where-each-setting-is-set) and [model resolution](model-resolution.en.md)).

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
| `OH_DOCTOR_OFFLINE` | `1`: `oh doctor` without network calls (as `--offline`) |
| `CI` | Not empty: no TUI |
| `TERM` | `dumb`: no TUI |
| `TMUX`, `TERM_PROGRAM` | tmux and iTerm2 detection to open sessions |
| `OH_SESSION_ID`, `OH_TEAM_ID`, `OH_PROJECT_ID` | Set by oh in the environment of sessions and MCP servers (do not set by hand) |
| `AWS_PROFILE`, `AWS_REGION`, `AWS_DEFAULT_REGION`, `AWS_ACCESS_KEY_ID`, `AWS_SECRET_ACCESS_KEY`, `AWS_BEARER_TOKEN_BEDROCK` | Bedrock credential detection (`oh init`, `oh doctor`); `AWS_BEARER_TOKEN_BEDROCK` is never passed to the sessions: `oh provider setup bedrock --from-env` stores it in the keychain |
| `ANTHROPIC_API_KEY`, `OPENROUTER_API_KEY` | Anthropic and OpenRouter credential detection |
| `FIGMA_TOKEN` | Figma API token (MCP server) |
| `GITLAB_TOKEN`, `GITLAB_URL`, `GITLAB_WRITE_ENABLED` | GitLab MCP server (default URL: `https://gitlab.com`) |
| `OH_GITLAB_URL` | Server `GITLAB_TOKEN` is valid for, when oh picks a GitLab token (otherwise `GITLAB_URL`). With neither, `GITLAB_TOKEN` is sent to no server |
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
3. `llm.default_provider` in `hub.toml`
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
> **Web dashboard:** `oh serve` binds to `127.0.0.1` only, never to the network, and requires the access token printed at startup.
