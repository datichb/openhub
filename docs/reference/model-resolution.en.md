# Model and provider resolution

---

## Overview

The Go CLI (`oh`) uses a simple resolution system for determining the AI provider and model used by each project. There is no per-agent or per-family model assignment — opencode handles model routing internally.

---

## Provider resolution

The provider is resolved via a 3-level cascade (first match wins):

| Priority | Source | Example |
|----------|--------|---------|
| 1 | CLI flag `--provider` | `oh deploy --provider anthropic` |
| 2 | Hub config | `hub.toml` → `[opencode] default_provider = "bedrock"` |
| 3 | Hardcoded fallback | `bedrock` |

---

## Model configuration

The model is configured at the project level, not per-agent. It is set in the project's `opencode.json` (deployed by the CLI):

```json
{
  "model": "amazon-bedrock/anthropic.claude-sonnet-4-5-20250929-v1:0"
}
```

### Setting the model

**During deployment:**

```bash
oh deploy --model claude-sonnet-4-5
```

**Per-project override:**

```bash
oh project configure --provider anthropic --model claude-opus-4
```

The CLI applies the correct provider prefix automatically based on the provider (e.g. `anthropic/claude-opus-4`, `amazon-bedrock/anthropic.claude-opus-4-20250514-v1:0`).

---

## Provider prefixing

Opencode requires model names to be prefixed with the provider in the `provider/model` format. The CLI applies this prefixing **automatically** during deployment, using internal provider configuration.

### Examples per provider

| Provider | Internal model | Result in opencode.json |
|----------|----------------|-------------------------|
| `anthropic` | `claude-sonnet-4-5` | `anthropic/claude-sonnet-4-5` |
| `bedrock` | `claude-sonnet-4-5` | `amazon-bedrock/anthropic.claude-sonnet-4-5-20250929-v1:0` |
| `github-copilot` | `claude-sonnet-4-5` | `github-copilot/claude-sonnet-4.5` |

---

## Hub configuration (`hub.toml`)

The provider and default model are stored in `~/.oh/hub.toml`:

```toml
[opencode]
default_provider = "bedrock"
```

This file is managed by the CLI:

```bash
# View current config
oh config show

# Change default provider
oh config set opencode.default_provider anthropic
```

---

## Agent model floor (frontmatter)

Agents can declare a minimum model via the `model:` field in their frontmatter. This field defines a **floor** — not an override. Opencode respects this floor internally when routing requests to agents.

```yaml
---
id: orchestrator
model: anthropic/claude-opus-4
skills: [skill-a, skill-b]
---
```

### Agents with a default floor

| Agent | Floor |
|-------|-------|
| `orchestrator` | `anthropic/claude-opus-4` |
| `orchestrator-dev` | `anthropic/claude-opus-4` |
| `reviewer` | `anthropic/claude-opus-4` |
| `planner` | `anthropic/claude-opus-4` |

---

## Key differences from the old system

| Old (bash era) | Current (Go CLI) |
|---|---|
| 7-level cascade per agent | Simple provider + model at project level |
| `api-keys.local.md` per project | `hub.toml` + `opencode.json` |
| `config/hub.json` | `~/.oh/hub.toml` |
| Per-agent and per-family model | No per-agent model — opencode routes internally |
| `prompt-builder.sh` fallback | Go binary hardcoded fallback |
| `config/providers.json` | Provider logic built into the Go CLI |
