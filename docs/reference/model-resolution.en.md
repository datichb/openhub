# Model and Provider Resolution

---

## Overview

The Go CLI (`oh`) resolves the AI model for each agent via a 7-level cascade. Opencode does not manage this logic — the CLI resolves at deploy time and writes the final model into `opencode.json` under `agent.<id>.model`.

The provider is resolved separately and used to normalize the model name format (provider prefixing).

---

## Provider Resolution

The provider is resolved via a 3-level cascade (first match wins):

| Priority | Source | Example |
|----------|--------|---------|
| 1 | CLI flag `--provider` | `oh deploy --provider anthropic` |
| 2 | Hub config | `hub.toml` → `[opencode] default_provider = "bedrock"` |
| 3 | Hardcoded fallback | `bedrock` |

---

## Per-Agent Model Resolution Cascade

Resolution is performed for each deployed agent. First match wins (decreasing priority):

| Priority | Level | Source | Command |
|----------|-------|--------|---------|
| 1 | Project agent | Model override for a specific agent in a project | `oh config model agent <id> <model> --project <p>` |
| 2 | Project family | Model override for an agent family in a project | `oh config model family <name> <model> --project <p>` |
| 3 | Project global | Global project model | `oh config model default <model> --project <p>` |
| 4 | Hub agent | Model override for a specific agent at hub level | `oh config model agent <id> <model>` |
| 5 | Hub family | Model override for an agent family at hub level | `oh config model family <name> <model>` |
| 6 | Hub global | Global hub model | `oh config model default <model>` |
| 7 | Frontmatter floor | `model:` field in the agent's `.md` file | Direct file edit |

### Families

An agent's family is derived from its parent directory in `agents/`:

| Directory | Family | Agents |
|-----------|--------|--------|
| `agents/planning/` | `planning` | orchestrator, orchestrator-dev, planner, pathfinder, onboarder |
| `agents/developer/` | `developer` | developer, developer-refactor, developer-migrator |
| `agents/quality/` | `quality` | reviewer, debugger |
| `agents/auditor/` | `auditor` | auditor, auditor-subagent |
| `agents/design/` | `design` | designer |
| `agents/documentation/` | `documentation` | documentarian |

---

## Configuration Storage

### Hub-level (`~/.oh/hub.toml`)

```toml
[opencode]
default_provider = "bedrock"

[models]
default = "claude-sonnet-4-5"

[models.families]
quality = "claude-opus-4"
planning = "claude-sonnet-4-6"

[models.agents]
reviewer = "claude-opus-4"
```

### Project-level (SQLite DB)

Project overrides are stored in the hub database (`~/.oh/oh.db`):
- `projects.model` → project global model (level 3)
- `projects.model_overrides` → serialized JSON for per-agent and per-family (levels 1 and 2)

```json
{
  "families": {"quality": "claude-opus-4"},
  "agents": {"reviewer": "claude-opus-4"}
}
```

---

## Configuration Commands

```bash
# --- Hub-level ---
oh config model default claude-sonnet-4-5
oh config model family quality claude-opus-4
oh config model agent reviewer claude-opus-4

# --- Project-level ---
oh config model default claude-opus-4 --project my-app
oh config model family planning claude-sonnet-4-6 --project my-app
oh config model agent reviewer claude-opus-4 --project my-app

# --- View configuration ---
oh config model show
oh config model show --project my-app

# --- Remove an override ---
oh config model unset default
oh config model unset family quality
oh config model unset agent reviewer --project my-app
```

---

## Provider Prefixing (Normalization)

Opencode requires model names to be prefixed with the provider in `provider/model` format. The CLI applies this prefixing **automatically** during deployment.

The model resolved by the cascade (regardless of input format) is normalized to the project's provider:

| Provider | Input (cascade) | Result in opencode.json |
|----------|-----------------|-------------------------|
| `anthropic` | `claude-sonnet-4-5` | `anthropic/claude-sonnet-4-5` |
| `bedrock` | `claude-sonnet-4-5` | `amazon-bedrock/anthropic.claude-sonnet-4-5-20250929-v1:0` |
| `bedrock` | `anthropic/claude-opus-4` | `amazon-bedrock/anthropic.claude-opus-4-20250514-v1:0` |
| `github-copilot` | `claude-sonnet-4-5` | `github-copilot/claude-sonnet-4.5` |
| `openrouter` | `claude-opus-4` | `anthropic/claude-opus-4` |

Normalization extracts the "short name" (e.g., `claude-opus-4`) from any input format, then re-formats it for the target provider.

---

## Agent Model Floor (Frontmatter)

Agents can declare a minimum model via the `model:` field in their frontmatter:

```yaml
---
id: orchestrator
model: anthropic/claude-sonnet-4-6
---
```

This field is **level 7** of the cascade — it only applies if no override is defined at higher levels.

### Agents with declared floor

| Agent | Floor |
|-------|-------|
| `orchestrator` | `anthropic/claude-sonnet-4-6` |
| `orchestrator-dev` | `anthropic/claude-sonnet-4-6` |
| `planner` | `anthropic/claude-sonnet-4-6` |
| `pathfinder` | `anthropic/claude-sonnet-4-6` |
| `reviewer` | `anthropic/claude-opus-4` |

---

## Result in opencode.json

After `oh deploy`, each selected agent gets a block in `opencode.json`:

```json
{
  "agent": {
    "orchestrator": {
      "model": "amazon-bedrock/anthropic.claude-sonnet-4-6-20250715-v1:0",
      "permission": {
        "question": "allow",
        "bash": "deny",
        "task": { "*": "deny", "planner": "allow" }
      }
    },
    "developer": {
      "mode": "subagent",
      "model": "amazon-bedrock/anthropic.claude-sonnet-4-5-20250929-v1:0",
      "permission": {
        "bash": { "*": "deny", "git *": "allow", "npm *": "allow" },
        "read": "allow",
        "edit": "allow"
      }
    }
  }
}
```

### What the deploy writes

| Field | Condition |
|-------|-----------|
| `agent.<id>.mode` | Written only if `mode: subagent` (primary is the default) |
| `agent.<id>.model` | Written if a model is resolved (non-empty cascade) |
| `agent.<id>.permission` | Written if permissions are declared in the frontmatter |

---

## Deploy Phases

Deployment executes in 5 transactional phases (automatic rollback on error):

| # | Phase | Role |
|---|-------|------|
| 1 | **Agents** | Copies selected agent `.md` files to `.opencode/agents/` |
| 2 | **Skills** | Copies skills to `.opencode/skills/` |
| 3 | **Configuration** | Writes global provider/model + disables native agents |
| 4 | **Agent Configuration** | Parses frontmatter, resolves model via cascade, writes per-agent in opencode.json |
| 5 | **MCP** | Injects configured MCP servers |
