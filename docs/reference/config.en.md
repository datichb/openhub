> 🇫🇷 [Lire en français](config.fr.md)

# Configuration Reference

---

## `config/hub.json`

Global hub configuration. Created by `oc install` and editable manually.

### Complete structure

```json
{
  "version": "1.0.0",
  "default_provider": {
    "name": "anthropic",
    "api_key": "",
    "base_url": "",
    "model": ""
  },
  "opencode": {
    "model": "claude-sonnet-4-5",
    "disabled_native_agents": ["build", "plan"]
  },
}
```

### Key reference

| Key | Type | Default | Description |
|-----|------|---------|-------------|
| `version` | string | — | Hub version (read by `oc version`) |
| `default_provider` | object | — | Default LLM provider configuration for all projects |
| `default_provider.name` | string | `"anthropic"` | Provider name (`anthropic`, `mammouth`, `github-models`, `bedrock`, `ollama`) |
| `default_provider.api_key` | string | `""` | Provider API key (masked in display, auto-gitignored if set) |
| `default_provider.base_url` | string | `""` | Custom base URL (optional for litellm and others) |
| `default_provider.model` | string | `""` | Default AI model for this provider (if empty: fallback to `opencode.model`) |
| `opencode.model` | string | — | AI model injected into `opencode.json` of deployed projects (if `default_provider.model` is empty) |
| `opencode.disabled_native_agents` | array | `[]` | Native OpenCode agents disabled by default (`build`, `plan`, `general`, `explore`, `pathfinder`) — overridable per project via `- Disable agents:` in `projects.md` |

### Minimal example (OpenCode only)

```json
{
  "version": "1.0.0",
  "default_provider": {
    "name": "anthropic",
    "api_key": "",
    "base_url": "",
    "model": ""
  },
  "opencode": {
    "model": "claude-sonnet-4-5"
  }
}
```

### Example with configured default provider

```json
{
  "version": "1.0.0",
  "default_provider": {
    "name": "mammouth",
    "api_key": "sk-xxx...",
    "base_url": "https://api.mammouth.ai/v1",
    "model": "claude-opus-4-5"
  },
  "opencode": {
    "model": "claude-sonnet-4-5"
  }
}
```

### Complete example

```json
{
  "version": "1.0.0",
  "default_provider": {
    "name": "anthropic",
    "api_key": "sk-ant-xxx...",
    "base_url": "",
    "model": ""
  },
  "opencode": {
    "model": "claude-sonnet-4-5"
  }
}
```

---

## `projects/projects.md`

Local project registry. **Git-ignored** — each developer maintains
their own. Automatically created from `projects/projects.example.md` on the first
`oc install` or `oc init`.

### Format

```markdown
## PROJECT_ID
- Name: Human-readable project name
- Stack: Tech stack (e.g. Vue 3 + Laravel)
- Beads Board: Beads board identifier
- Tracker: jira | gitlab | none
- Labels: label1, label2, label3
- Language: english        # optional — if absent: agents respond in French by default
- Agents: all             # optional — all (default) or CSV list of agent-ids
- Modes: agent-id:mode,agent-id:mode  # optional — override primary/subagent modes per agent
- Disable agents: plan,build  # optional — overrides hub.json for this project
```

### Example

```markdown
## MY-APP
- Name: My Application
- Stack: Vue 3 + Laravel 10
- Beads Board: MY-APP
- Tracker: jira
- Labels: feature, fix, front, back

## API-GATEWAY
- Name: API Gateway
- Stack: Node.js + Fastify
- Beads Board: API-GATEWAY
- Tracker: none
- Labels: feature, fix, api
- Language: english
- Agents: orchestrator,orchestrator-dev,developer-backend,developer-api
- Modes: developer-backend:primary,developer-api:primary
```

### Rules

- `PROJECT_ID`: letters, digits, `-` and `_` only — no spaces or slashes
- `Tracker`: `jira`, `gitlab` or `none`
- `Language`: optional — free value (e.g. `english`, `spanish`) — if absent, agents respond in French
- `Agents`: optional — `all` or CSV of agent identifiers — filtered at deployment
- `Modes`: optional — CSV of `agent-id:mode` pairs — overrides agent frontmatter. Modes: `primary`, `subagent`. Leave empty to revert to frontmatter values.
- `Disable agents`: optional — CSV of native OpenCode agents to disable (`build`, `plan`, `general`, `explore`, `pathfinder`) — overrides `opencode.disabled_native_agents` in `hub.json`. Empty = use hub default.
- This file is **local** — never commit it

---

## `projects/projects.example.md`

Versioned template for `projects.md`. Automatically copied to `projects/projects.md`
if that file is absent.

Modify this template to define the default project structure for your team.

---

## `projects/paths.local.md`

Associates each `PROJECT_ID` with a local path on the developer's machine.
**Git-ignored.**

### Format

```
PROJECT_ID=/absolute/path/to/the/project
```

### Example

```
MY-APP=~/workspace/my-app
API-GATEWAY=/home/user/projects/api-gateway
OTHER-APP=~/dev/other-app
```

### Rules

- One `PROJECT_ID` per line
- Absolute paths or with `~` (expanded by the shell)
- Do not commit this file — each developer has their own local paths

## `projects/api-keys.local.md`

Stores API keys and models configured per project via `oc config`.
**Git-ignored** — never commit this file.

### Format

```ini
[PROJECT_ID]
model=claude-opus-4-5
provider=anthropic
api_key=sk-ant-...

[OTHER-PROJECT]
model=claude-sonnet-4-5
provider=mammouth
api_key=sk-bRf...
base_url=https://api.mammouth.ai/v1

[GITHUB-PROJECT]
model=claude-sonnet-4-5
provider=github-models
api_key=ghp_xxx...
base_url=https://models.inference.ai.azure.com
```

### Available keys per section

| Key | Required | Description |
|-----|----------|-------------|
| `model` | yes | AI model (e.g. `claude-opus-4-5`, `claude-haiku-4-5`) |
| `provider` | yes | `anthropic`, `mammouth`, `github-models`, `bedrock`, `ollama`, or `litellm` |
| `api_key` | yes | API key — never displayed in plain text |
| `base_url` | no | Base URL (recommended for `mammouth`, `github-models`, `bedrock`, `ollama`, and required for generic `litellm`) |

### Supported providers

| Provider | API Key required | Default base URL | Description |
|----------|-----------------|-----------------|-------------|
| `anthropic` | yes | — | Direct Anthropic API |
| `mammouth` | yes | `https://api.mammouth.ai/v1` | OpenAI-compatible proxy (FR-hosted) |
| `github-models` | yes | `https://models.inference.ai.azure.com` | GitHub Models API |
| `bedrock` | yes | — (AWS-specific) | AWS Bedrock |
| `ollama` | no | `http://localhost:11434/v1` | OpenAI-compatible local LLM |
| `litellm` | yes | ⚠️ required | Generic litellm proxy (custom) |

### Effects during deployment

During `oc deploy <PROJECT_ID>`, if an entry exists for the project:

- `opencode.json` and `.opencode/` are added to the target project's `.git/info/exclude` **before** writing the file (local exclusion, invisible to other devs)
- `opencode.json` is regenerated with the complete `provider` block
- The file is created with `600` permissions

If `PROJECT_ID` is defined without an API key (or after `oc config unset`), `opencode.json` is
also regenerated to remove any old `provider` block.

For OpenCode, the key is injected as `ANTHROPIC_API_KEY` at `oc start` time (Anthropic only).

---

## `oc config` — CLI command

Manages entries in `projects/api-keys.local.md` as well as hub-level LLM provider configuration (`config/hub.json`).

Also accessible via `oc help 7` or `oc config --help`.

### Sub-commands

| Sub-command | Description |
|---|---|
| `set [PROJECT_ID] [options]` | Create or update a configuration (project or hub) |
| `get <PROJECT_ID>` | Display the configuration (masked key) |
| `list` | List all configurations |
| `list --providers` | List providers in the catalogue |
| `unset <PROJECT_ID>` | Delete a configuration |
| `set language <en\|fr>` | Set CLI display language (global) |
| `init-providers [--force]` | Initialise switcher files in `config/providers/` |
| `websearch <enable\|disable\|status>` | Manage WebSearch (Exa AI) permission |

### `oc config set` options

| Option | Description |
|--------|-------------|
| `--model <model>` | AI model |
| `--provider <provider>` | `anthropic`, `mammouth`, `github-models`, `bedrock`, `ollama`, or `litellm` |
| `--api-key <key>` | API key (if omitted: interactive masked input) |
| `--base-url <url>` | Base URL (optional for most providers) |
| `--family-model <model>` | AI model for `family`-type agents |
| `--agent-model <model>` | AI model for agents |

**Behaviour depending on arguments:**

- `oc config set <PROJECT_ID>` — interactive, configures the provider and key for that project
- `oc config set` (no `PROJECT_ID`) — interactive **hub** provider setup wizard
- `oc config set --provider anthropic --api-key sk-...` — non-interactive hub provider configuration
- `oc config set --provider bedrock` — hub provider without API key
- `oc config set --model claude-opus-4` — update hub default model only
- `oc config set --provider p --api-key k --model m` — configure provider, key and hub model in one command

`oc config list --providers` lists all providers in the catalogue with their hub configuration status.

`oc config init-providers` creates `config/providers/` and generates the JSON files used by `ocp` (`mammouth.json`, `copilot.json`, `openrouter.json`, `ollama.json`, `bedrock.json`) as well as `config/providers/.gitignore`. Without `--force`, existing files are not overwritten.

### Example

```sh
# Interactive hub wizard (default provider)
./oc.sh config set

# Configure hub provider on the command line
./oc.sh config set --provider anthropic --api-key sk-ant-...

# Hub provider without API key (e.g. Bedrock)
./oc.sh config set --provider bedrock

# Update hub default model only
./oc.sh config set --model claude-opus-4

# Interactive project configuration
./oc.sh config set MY-PROJECT

# Project configuration on the command line
./oc.sh config set MY-PROJECT --model claude-opus-4-5 --provider anthropic

# With MammouthAI
./oc.sh config set MY-PROJECT --provider mammouth --api-key sk-xxx

# List providers in the catalogue
./oc.sh config list --providers

# Initialise ocp switcher files
./oc.sh config init-providers

# Check
./oc.sh config get MY-PROJECT

# Delete
./oc.sh config unset MY-PROJECT
```

---

## `oc project` — project management CLI

Manages project registry entries in `projects/projects.md` and `projects/paths.local.md`.

### Sub-commands

```
oc project rename <OLD_ID> <NEW_ID>      Rename a project in all registry files
oc project move <PROJECT_ID> <path>      Change a project's local path
oc project configure [PROJECT_ID]        Reconfigure an existing project's fields
```

### `oc project configure`

Interactive wizard to update any field in `projects.md` for an existing project.
If no `PROJECT_ID` is provided, an interactive numbered list is displayed.

For each field, the current value is shown. Press **Enter** to keep it unchanged.

| Field | Values | Description |
|-------|--------|-------------|
| `Stack` | free text | Technologies used (e.g. `Vue 3 + Laravel`) |
| `Tracker` | `none` \| `jira` \| `gitlab` | External issue tracker |
| `Labels` | CSV | Beads labels (e.g. `feature,fix,front,back`) |
| `Language` | free text | Agent language (`english`, `spanish` — absent = French) |
| `Disable agents` | CSV | Native OpenCode agents to disable (`build`, `plan`, `general`, `explore`, `pathfinder`) — use `none` to clear |
| `MCP` | `all` \| `none` \| CSV | MCP servers to enable |
| `Worktree` | `enabled` \| `disabled` | Enable git worktrees for parallel work |
| `Worktree auto cleanup` | `true` \| `false` | Auto-remove merged worktrees *(only shown if Worktree is enabled)* |
| `Worktree base branch` | branch name | Base branch for cleanup (default: `main`) *(only shown if Worktree is enabled)* |

> Note: `Agents` and `Modes` fields have dedicated commands — use `oc agent select` and `oc agent mode`.

### Examples

```sh
# Interactive wizard (project picker)
./oc.sh project configure

# Configure a specific project
./oc.sh project configure MY-APP

# Rename a project
./oc.sh project rename MY-APP MY-APP-V2

# Move a project to a new path
./oc.sh project move MY-APP ~/workspace/my-app-new
```

---

## `ocp` — interactive provider switcher

Shell function injected into `~/.zshrc` by the hub. Launches opencode with a chosen provider while preserving the full `oc start` logic (agent deployment, `--dev` mode, Beads sync, onboarding, etc.).

Requires `config/providers/` to be initialized via `oc config init-providers`.

### Usage

```sh
ocp                          # interactive provider picker (fzf or native select)
ocp mammouth                 # launch with mammouth (interactive project picker)
ocp mammouth openhub    # launch openhub project with mammouth
ocp bedrock MY-APP --dev     # --dev mode with bedrock
ocp --list                   # list available providers
```

### Behavior

`ocp <provider> [args...]` is equivalent to:
```sh
./oc.sh start --provider <provider> [args...]
```

The `--provider` flag overrides the effective provider for `opencode.json` generation — per-agent models are prefixed and aliased according to the selected provider.

### Installation

The function is automatically injected into `~/.zshrc` during `oc install`.
To add or update it manually, re-run `oc install` or copy the block between the delimiters `# >>> opencode providers switcher (ocp) >>>` / `# <<< opencode providers switcher (ocp) <<<`.

---

## `opencode.json`

OpenCode configuration file at the root of a target project.
Created by `oc deploy opencode` — **regenerated if an API key is configured, if `PROJECT_ID` is
defined (to remove an old provider block), or if the file is absent**; kept as-is otherwise.

### Content without API key

```json
{
  "$schema": "https://opencode.ai/config.json",
  "model": "claude-sonnet-4-5",
  "agent": {
    "auditor-subagent": { "mode": "subagent" },
    "developer-backend": { "mode": "subagent" },
    "build": { "disable": true },
    "plan": { "disable": true }
  }
}
```

The `"agent":` block lists:
- agents whose effective mode is `subagent`
- disabled native OpenCode agents (`"disable": true`) — defined in `hub.json → opencode.disabled_native_agents` and overridable per project in `projects.md` via `- Disable agents:`

`primary` agents that are not disabled are absent — OpenCode considers them visible by default.
If no agent has special configuration, the `"agent":` block is omitted.

### Content with Anthropic key

```json
{
  "$schema": "https://opencode.ai/config.json",
  "model": "claude-opus-4-5",
  "provider": {
    "anthropic": {
      "apiKey": "sk-ant-..."
    }
  }
}
```

### Content with litellm / OpenAI-compatible proxy

```json
{
  "$schema": "https://opencode.ai/config.json",
  "model": "claude-sonnet-4-5",
  "provider": {
    "litellm": {
      "npm": "@ai-sdk/openai-compatible",
      "options": {
        "apiKey": "sk-bRf...",
        "baseURL": "https://api.mammouth.ai/v1"
      }
    }
  }
}
```

Model is resolved by priority:
1. `projects/api-keys.local.md` → project's `model` key (if `PROJECT_ID` defined)
2. Environment variable `$OPENCODE_MODEL`
3. `config/hub.json` → `opencode.model` key
4. Fallback: `claude-sonnet-4-5`

> If an API key is injected, this file **must not be committed** to the target project
> (automatically added to the project's `.git/info/exclude` by `oc deploy` — local exclusion, invisible to other devs).
> Without an API key, the file **can be committed**.

---

## Hub `.gitignore`

Files and folders ignored by git in the hub itself:

```gitignore
config/hub.json             # if default_provider.api_key is set (auto-added)
projects/projects.md        # local project registry
projects/paths.local.md     # local paths
projects/api-keys.local.md  # API keys per project
.opencode/node_modules/     # OpenCode dependencies
.opencode/bun.lock
.opencode/package.json
skills/external/            # skills downloaded via oc skills add
```

---

## Environment variables

The hub does not define any mandatory environment variables.
Credentials for trackers (Jira, GitLab) are stored locally
by `bd config set` (or `bd config set-many` for batch writes) — never in versioned files.

The following variables are read by hub scripts if present in the environment:

| Variable | Default | Required | Description |
|----------|---------|----------|-------------|
| `OPENCODE_HUB_DIR` | `~/.openhub` | no | Hub installation directory. Used by `install.sh` and `uninstall.sh` to choose the installation path. Example: `OPENCODE_HUB_DIR=~/tools/oc bash install.sh` |
| `OPENCODE_MODEL` | *(hub.json cascade)* | no | LLM model to use. Level 2 of the model resolution cascade (after project config `api-keys.local.md`, before `hub.json`). Example: `OPENCODE_MODEL=claude-opus-4-5` |
| `AWS_BEARER_TOKEN_BEDROCK` | — | no | AWS Bedrock authentication token. Automatically injected by `oc start` from the project or hub config when the effective provider is `bedrock` — do not set manually except for advanced use cases. |
| `HUB_DIR` | *(repo root directory)* | no | Runtime override of the hub root directory. Auto-detected from the location of `oc.sh` if absent. Useful for testing or multi-hub configurations. |
