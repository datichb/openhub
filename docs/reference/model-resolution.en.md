> [Lire en français](model-resolution.fr.md)

# Model and Provider Resolution

---

## Overview

The Go CLI (`oh`) resolves the AI model of each agent of the session bundle through a **9-level cascade** (2 workflow levels, 3 project levels, 3 hub levels, then the agent frontmatter). Opencode does not manage this logic: oh resolves when building the session bundle, at launch, and writes the final model into the agent definitions of the bundle (`agent.<id>.model` in the rendered opencode config).

The provider is resolved separately and used to normalize the model name (provider prefix). I6 restrictions may also limit the usable models (see [allow-list](#model-allow-list-limitsmodels)).

---

## Provider Resolution

The provider is resolved through a 4-level cascade (first match wins):

| Priority | Source | Example |
|----------|--------|---------|
| 1 | CLI flag `--provider` / `-P` | `oh run feature --provider anthropic` |
| 2 | Project provider | `oh.db` database (`oh project configure`) |
| 3 | Hub config | `hub.toml` → `[opencode] default_provider = "bedrock"` |
| 4 | Hardcoded fallback | `bedrock` |

---

## Per-Agent Model Resolution Cascade

Resolution is performed for each agent of the session bundle. First match wins (decreasing priority):

| Priority | Level | Source | Command |
|----------|-------|--------|---------|
| 1 | Workflow · agent | `models.agents.<id>` of the workflow | Workflow YAML ([schema](workflow-schema.en.md#resources)) |
| 2 | Workflow | `models.default` of the workflow | Workflow YAML |
| 3 | Project · agent | Model of an agent in a project | `oh config model agent <id> <model> --project <p>` |
| 4 | Project · family | Model of an agent family in a project | `oh config model family <name> <model> --project <p>` |
| 5 | Project | Global project model | `oh config model default <model> --project <p>` |
| 6 | Hub · agent | Model of an agent at hub level | `oh config model agent <id> <model>` |
| 7 | Hub · family | Model of an agent family at hub level | `oh config model family <name> <model>` |
| 8 | Hub | Global hub model | `oh config model default <model>` |
| 9 | Frontmatter | `model:` field in the agent's `.md` file | Agent file edit |

### Workflow level (decision O9)

- The `models:` block of an `oh/v1` workflow comes **before** any project and hub configuration. It has no families.
- Full identifiers (`amazon-bedrock/eu.anthropic.claude-sonnet-4-6`, `#variant` suffix) are accepted: the Bedrock regional prefix is removed, then added back by the adapter for the session region; the variant is kept.
- A patch (`extends`) replaces `models.default` and merges `models.agents` per agent.

### Team models: not applied

The team-state `config.toml` may hold `[models]` recommendations (`default`, `families`, `agents`, edited in the TUI, Team › Models). The cascade function (`bricks.ResolveAgentModel`) can place them after the hub (team · agent > team · family > team), but **the launch does not fill them in** v5 (`cmd/v5_launch.go`, `modelOverridesFor` only provides the hub and the project): they have no effect on sessions.

### Families

An agent's family is derived from its directory in `agents/`:

| Directory | Family | Agents |
|-----------|--------|--------|
| `agents/planning/` | `planning` | conductor, orchestrator, orchestrator-dev, planner, pathfinder, onboarder |
| `agents/developer/` | `developer` | developer, developer-refactor, developer-migrator, database, infra |
| `agents/quality/` | `quality` | reviewer, debugger, benchmarker, test-generator |
| `agents/auditor/` | `auditor` | auditor, auditor-subagent |
| `agents/design/` | `design` | designer |
| `agents/documentation/` | `documentation` | documentarian |
| `agents/utility/` | `utility` | brief-enricher |

### Model allow-list (`limits.models`)

I6 restrictions may limit the models of a session (patterns with `*`, e.g. `eu.anthropic.claude-*`):

- levels: hub (`[limits] models`, `oh budget set models …`), team (`[limits.recommended]` or `[limits.enforced]`), project (`oh budget set models … -p <project>`), workflow (`limits.models`);
- the most specific list wins (workflow > project > hub > team recommendation); a list **enforced** by the team is a ceiling: only the patterns it covers are kept;
- the list does not change the model picked by the cascade: it is applied by the **credential proxy**, which refuses calls to any other model (id sent to the provider, e.g. `eu.anthropic.claude-sonnet-4-6`).

So pick cascade models that match the list. See `oh budget show` and [ADR-044](../architecture/adr/044-credential-proxy-session-limits.en.md).

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
- `projects.model` → project global model (level 5)
- `projects.model_overrides` → serialized JSON for per-agent and per-family (levels 3 and 4)

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

Opencode requires model names to be prefixed with the provider in `provider/model` format. The CLI applies this prefixing **automatically** when building the session bundle.

The model resolved by the cascade (regardless of input format) is normalized to the project's provider:

| Provider | Input (cascade) | Result in the session config |
|----------|-----------------|-------------------------|
| `anthropic` | `claude-sonnet-4-5` | `anthropic/claude-sonnet-4-5` |
| `bedrock` | `claude-sonnet-4-5` | `amazon-bedrock/anthropic.claude-sonnet-4-5-20250929-v1:0` |
| `bedrock` | `anthropic/claude-opus-4` | `amazon-bedrock/anthropic.claude-opus-4-20250514-v1:0` |
| `github-copilot` | `claude-sonnet-4-5` | `github-copilot/claude-sonnet-4.5` |
| `openrouter` | `claude-opus-4` | `anthropic/claude-opus-4` |

Normalization extracts the "short name" (e.g., `claude-opus-4`) from any input format, then re-formats it for the target provider.

---

## Agent Model Floor (Frontmatter)

Agents can declare a model via the `model:` field of their frontmatter:

```yaml
---
id: orchestrator
model: claude-sonnet-4-6
---
```

This field is **level 9** of the cascade: it only applies when no level above defines a model.

### Agents with declared floor

| Agent | Floor |
|-------|-------|
| `conductor`, `orchestrator`, `orchestrator-dev`, `planner`, `pathfinder`, `onboarder` | `claude-sonnet-4-6` |
| `auditor`, `auditor-subagent`, `designer`, `debugger`, `documentarian` | `claude-sonnet-4-6` |
| `reviewer`, `benchmarker`, `test-generator`, `database`, `infra` | `claude-opus-4-6` |
| `brief-enricher` | `anthropic/claude-sonnet-4-5` |

The `developer*` agents declare none: without configuration, opencode uses its default model.

---

## Result in the session config

At each launch, each agent of the session bundle gets a block in the rendered opencode config (inspect with `oh bundle show <workflow>`); model changes (`oh config model ...`) are applied at the next launch, no redeploy:

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

### What the bundle writes

| Field | Condition |
|-------|-----------|
| `agent.<id>.mode` | Written only if `mode: subagent` (primary is the default) |
| `agent.<id>.model` | Written if a model is resolved (non-empty cascade) |
| `agent.<id>.permission` | Written if permissions are declared in the frontmatter |

---

## Session Bundle Build

The former 5 deploy phases (`oh deploy`, removed in v5) are replaced by the session bundle build: at each launch, `internal/bundle` builds `~/.oh/bundles/<hash>/` from the workflow — agents with their Bucket A skills inlined and their model resolved via the cascade, on-demand skills, permissions, MCP servers and plugin. The adapter renders the opencode config from it; nothing is written into the project.
