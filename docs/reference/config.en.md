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
| `opencode.disabled_native_agents` | array | `[]` | Native OpenCode agents disabled by default (`build`, `plan`, `general`, `explore`) — overridable per project via `- Disable agents:` in `projects.md` |

### Available targets

| Value | Target tool |
|-------|-------------|
| `opencode` | OpenCode (`opencode run`) |

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
- Targets: opencode  # optional — target(s) to use for this project
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
- Targets: opencode
- Modes: developer-backend:primary,developer-api:primary
```

### Rules

- `PROJECT_ID`: letters, digits, `-` and `_` only — no spaces or slashes
- `Tracker`: `jira`, `gitlab` or `none`
- `Language`: optional — free value (e.g. `english`, `spanish`) — if absent, agents respond in French
- `Agents`: optional — `all` or CSV of agent identifiers — filtered at deployment
- `Targets`: optional — CSV of targets (`opencode`) — overrides the hub default target
- `Modes`: optional — CSV of `agent-id:mode` pairs — overrides agent frontmatter. Modes: `primary`, `subagent`. Leave empty to revert to frontmatter values.
- `Disable agents`: optional — CSV of native OpenCode agents to disable (`build`, `plan`, `general`, `explore`) — overrides `opencode.disabled_native_agents` in `hub.json`. Empty = use hub default.
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

Stores API keys and models configured per project via `oc config` or `oc provider`.
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

| Provider | Targets | API Key required | Default base URL | Description |
|----------|---------|-----------------|-----------------|-------------|
| `anthropic` | OpenCode | yes | — | Direct Anthropic API |
| `mammouth` | OpenCode | yes | `https://api.mammouth.ai/v1` | OpenAI-compatible proxy (FR-hosted) |
| `github-models` | OpenCode | yes | `https://models.inference.ai.azure.com` | GitHub Models API |
| `bedrock` | OpenCode | yes | — (AWS-specific) | AWS Bedrock |
| `ollama` | OpenCode | no | `http://localhost:11434/v1` | OpenAI-compatible local LLM |
| `litellm` | OpenCode | yes | ⚠️ required | Generic litellm proxy (custom) |

### Effects during deployment

During `oc deploy opencode <PROJECT_ID>`, if an entry exists for the project:

- `opencode.json` and `.opencode/` are added to the target project's `.git/info/exclude` **before** writing the file (local exclusion, invisible to other devs)
- `opencode.json` is regenerated with the complete `provider` block
- The file is created with `600` permissions

If `PROJECT_ID` is defined without an API key (or after `oc config unset`), `opencode.json` is
also regenerated to remove any old `provider` block.

For OpenCode, the key is injected as `ANTHROPIC_API_KEY` at `oc start` time (Anthropic only).

---

## `oc config` — CLI command

Manages entries in `projects/api-keys.local.md`.

### Sub-commands

```
oc config set <PROJECT_ID> [options]   Create or update a configuration
oc config get <PROJECT_ID>             Display the configuration (masked key)
oc config list                         List all configurations
oc config unset <PROJECT_ID>           Delete a configuration
```

### `oc config set` options

| Option | Description |
|--------|-------------|
| `--model <model>` | AI model |
| `--provider <provider>` | `anthropic`, `mammouth`, `github-models`, `bedrock`, `ollama`, or `litellm` |
| `--api-key <key>` | API key (if omitted: interactive masked input) |
| `--base-url <url>` | Base URL (optional for most providers) |

If called without flags, the flow is interactive with current values as defaults.

### Example

```sh
# Interactive flow
./oc.sh config set MY-PROJECT

# Command line (outside CI: prefer interactive flow for the key)
./oc.sh config set MY-PROJECT --model claude-opus-4-5 --provider anthropic

# With MammouthAI
./oc.sh config set MY-PROJECT --provider mammouth --api-key sk-xxx

# Check
./oc.sh config get MY-PROJECT

# Delete
./oc.sh config unset MY-PROJECT
```

---

## `oc provider` — CLI command

Manages LLM provider configuration at the hub (default) and project levels.

### Sub-commands

```
oc provider list                          List all available providers
oc provider set-default                   Configure the hub default provider
```

### Example

```sh
# List providers
./oc.sh provider list

# Configure hub default (interactive)
./oc.sh provider set-default
```

> **Note:** Per-project provider configuration is managed via `oc config set <PROJECT_ID>` (see the `oc config` section above).

### `oc provider init` — initializing switcher files

Creates the `config/providers/` directory and generates 5 JSON files used by `ocp`:
`mammouth.json`, `copilot.json`, `openrouter.json`, `ollama.json`, `bedrock.json`.

Also creates `config/providers/.gitignore` to protect API keys.

Idempotent: without `--force`, existing files are not overwritten.

```sh
# First initialization
./oc.sh provider init

# Reinitialize all files
./oc.sh provider init --force
```

### `oc provider set-key` and `oc provider set-model`

Modify an existing provider file in `config/providers/` atomically (tmpfile + mv).

```sh
# Set openrouter key
./oc.sh provider set-key openrouter sk-or-v1-xxx...

# Set ollama model
./oc.sh provider set-model ollama qwen2.5-coder:7b
```

---

## `ocp` — interactive provider switcher

Shell function injected into `~/.zshrc` by the hub. Launches opencode with a chosen provider while preserving the full `oc start` logic (agent deployment, `--dev` mode, Beads sync, onboarding, etc.).

Requires `config/providers/` to be initialized via `oc provider init`.

### Usage

```sh
ocp                          # interactive provider picker (fzf or native select)
ocp mammouth                 # launch with mammouth (interactive project picker)
ocp mammouth opencode-hub    # launch opencode-hub project with mammouth
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
    "auditor-security": { "mode": "subagent" },
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
by `bd config set` — never in versioned files.

The following variables are read by hub scripts if present in the environment:

| Variable | Default | Required | Description |
|----------|---------|----------|-------------|
| `OPENCODE_HUB_DIR` | `~/.opencode-hub` | no | Hub installation directory. Used by `install.sh` and `uninstall.sh` to choose the installation path. Example: `OPENCODE_HUB_DIR=~/tools/oc bash install.sh` |
| `OPENCODE_MODEL` | *(hub.json cascade)* | no | LLM model to use. Level 2 of the model resolution cascade (after project config `api-keys.local.md`, before `hub.json`). Example: `OPENCODE_MODEL=claude-opus-4-5` |
| `AWS_BEARER_TOKEN_BEDROCK` | — | no | AWS Bedrock authentication token. Automatically injected by `oc start` from the project or hub config when the effective provider is `bedrock` — do not set manually except for advanced use cases. |
| `HUB_DIR` | *(repo root directory)* | no | Runtime override of the hub root directory. Auto-detected from the location of `oc.sh` if absent. Useful for testing or multi-hub configurations. |
