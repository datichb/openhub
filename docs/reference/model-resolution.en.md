# Model resolution per agent

---

## Overview

Each agent can receive a specific AI model through a 7-level resolution cascade.
The first level that returns a value wins. A floor (clamp) mechanism ensures that
critical agents never receive a model below their declared minimum.

---

## Resolution cascade (7 levels)

For an agent `X` in family `F` within project `P`:

| Priority | Source | Key |
|----------|--------|-----|
| 1 | Project — specific agent | `api-keys.local.md` → `agent_models.agents.X=...` |
| 2 | Project — family | `api-keys.local.md` → `agent_models.families.F=...` |
| 3 | Project — global model | `api-keys.local.md` → `model=...` |
| 4 | Hub — specific agent | `config/hub.json` → `.agent_models.agents.X` |
| 5 | Hub — family | `config/hub.json` → `.agent_models.families.F` |
| 6 | Hub — global model | `config/hub.json` → `.opencode.model` |
| 7 | Hardcoded fallback | `claude-sonnet-4-5` (current value — see `DEFAULT_MODEL` in `prompt-builder.sh`) |

**Example:** if the project defines a model for the `planning` family (level 2) and the hub defines a model for the `orchestrator` agent (level 4), level 2 wins because it has higher priority.

> **Note — provider prefixes:** provider prefixes (e.g. `anthropic/`) are optional in the resolution cascade. The hardcoded fallback (level 7) does not include one (`claude-sonnet-4-5`), while frontmatter or configuration values may include one (e.g. `anthropic/claude-opus-4`). Both forms are accepted.

> **Note — `default_provider.model`:** the `default_provider.model` field in `hub.json` is NOT used in this cascade. It only serves to configure the OpenCode provider, not for per-agent model resolution.

---

## Floor (clamp) via frontmatter

Agents can declare a minimum model via the `model:` field in their frontmatter.
This field defines a **floor** — not an override. The model resolved by the cascade is kept
if it is greater than or equal to the floor; otherwise the floor is applied.

```yaml
---
id: orchestrator
model: anthropic/claude-opus-4
skills: [skill-a, skill-b]
---
```

> **Frontmatter ordering constraint:** the `model:` field **must appear before** `skills:` in the frontmatter. The parser uses an early exit after reading `id`, `targets` and `skills` — if `model:` is placed after `skills:`, it will not be read.

After cascade resolution, if the resolved model is **lower** than the declared floor,
the floor is applied and a warning is emitted:

```
WARN  Modèle résolu 'claude-haiku-4-5' inférieur au plancher 'anthropic/claude-opus-4' pour l'agent 'orchestrator' — plancher appliqué
```

> **Note:** the warning message is emitted in French by the implementation — this is the actual output you will see in the logs.

### Model hierarchy (ranks)

Each model is assigned a numeric rank for comparison:

| Model | Rank |
|-------|------|
| `claude-opus-4` | 3 |
| `claude-sonnet-4-5` | 2 |
| `claude-haiku-4-5` | 1 |
| Any other model | 0 |

An unknown model (rank 0) is **always lower** than haiku, which systematically forces
the clamp to the declared floor. This prevents an unrecognized model from silently
bypassing an opus or sonnet floor.

### Agents with a default floor

| Agent | Floor | Rank |
|-------|-------|------|
| `orchestrator` | `anthropic/claude-opus-4` | 3 |
| `orchestrator-dev` | `anthropic/claude-opus-4` | 3 |
| `reviewer` | `anthropic/claude-opus-4` | 3 |
| `planner` | `anthropic/claude-opus-4` | 3 |

---

## Agent family

The family is inferred from the parent subfolder in `agents/`:

- `agents/planning/orchestrator.md` → family `planning`
- `agents/developer/developer-frontend.md` → family `developer`
- `agents/quality/reviewer.md` → family `quality`

---

## Configuration examples

### hub.json — model per family and per agent

```json
{
  "opencode": {
    "model": "claude-sonnet-4-5"
  },
  "agent_models": {
    "families": {
      "planning": "claude-opus-4",
      "developer": "claude-sonnet-4-5"
    },
    "agents": {
      "debugger": "claude-opus-4",
      "documentarian": "claude-haiku-4-5"
    }
  }
}
```

In this example:
- All agents in the `planning` family (orchestrator, planner…) receive `claude-opus-4` (level 5)
- The `debugger` agent receives `claude-opus-4` (level 4 — takes priority over the `developer` family)
- The `documentarian` agent receives `claude-haiku-4-5` (level 4)
- Agents without an override use the global model `claude-sonnet-4-5` (level 6)

### api-keys.local.md — project override

In a specific project's `api-keys.local.md` file:

```markdown
# API Keys

model=claude-opus-4
agent_models.families.quality=claude-opus-4
agent_models.agents.developer-frontend=claude-haiku-4-5
```

In this example:
- The `developer-frontend` agent receives `claude-haiku-4-5` (level 1 — project agent override)
- All agents in the `quality` family receive `claude-opus-4` (level 2)
- All other project agents receive `claude-opus-4` (level 3 — project global model)
- Levels 4-7 (hub) are never consulted since level 3 answers for all

---

## CLI configuration

> **Note:** the `--family-model` and `--agent-model` flags require the `oc config set` enhancement (ticket .6). If not yet implemented, configure `hub.json` and `api-keys.local.md` manually as shown in the examples above.

```bash
# Hub level
oc config set --family-model planning=claude-opus-4
oc config set --agent-model debugger=claude-sonnet-4-5

# Project level
oc config set MY-APP --family-model planning=claude-opus-4
oc config set MY-APP --agent-model reviewer=claude-sonnet-4-5
```

---

## Injection rule in opencode.json

- If resolved model (after clamp) == project's global model → **no injection** (agent uses the default model, avoiding noise in the configuration)
- If resolved model ≠ global model → `"model": "<value>"` is injected into the agent's entry
