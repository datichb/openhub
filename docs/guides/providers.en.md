> 🇫🇷 [Lire en français](providers.fr.md)

# Multi-Provider LLM Support

OpenCode Hub supports multiple LLM providers, enabling you to choose the best solution for your needs. This guide explains how to configure and use different providers.

## Overview

### Supported Providers

| Provider | Type | Targets | Credential | Default Base URL |
|----------|------|---------|------------|------------------|
| **Anthropic** | Native | OpenCode, OpenCode | API key | N/A |
| **MammouthAI** | OpenAI-compatible (litellm) | OpenCode | API key | `https://api.mammouth.ai/v1` |
| **GitHub Models** | OpenAI-compatible (litellm) | OpenCode | API key | `https://models.inference.ai.azure.com` |
| **AWS Bedrock** | Native (`amazon-bedrock`) | OpenCode | Bearer token | N/A |
| **Ollama** | OpenAI-compatible (litellm) | OpenCode | Optional | `http://localhost:11434/v1` |
| **GitHub Copilot** | Native (`github-copilot`) | OpenCode | OAuth (no API key) | N/A |

### Important Notes

- **OpenCode limitation**: OpenCode only supports the `anthropic` provider (architectural constraint). Using other providers will trigger a warning.
- **Model priority**: Models are resolved in this order: 1) Project config → 2) Hub default → 3) Environment variable → 4) Hub opencode.model → 5) Default fallback

## Configuration Levels

OpenCode Hub supports provider configuration at two levels:

### 1. Hub Level (Default for All Projects)

Set a provider that applies to all projects by default:

```bash
./oc.sh provider set-default
```

This prompts you to:
- Select a provider (1-5 or skip)
- Provide API credentials (if required)
- Optionally set a custom base URL

The configuration is stored in `config/hub.json` in the `default_provider` block:

```json
{
  "default_provider": {
    "name": "mammouth",
    "api_key": "sk-xxx...",
    "base_url": "https://api.mammouth.ai/v1",
    "model": ""
  }
}
```

**Note**: If an API key is configured, `config/hub.json` is automatically added to `.gitignore`.

### 2. Project Level (Per-Project Override)

Configure a different provider for a specific project:

```bash
./oc.sh init MY-PROJECT
# or
./oc.sh config set MY-PROJECT
```

During `oc init`, you'll be prompted for an optional project-level provider (step 4).

During `oc config set`, you can specify `--provider` and related flags:

```bash
./oc.sh config set MY-PROJECT --provider github-models --api-key sk-xxx
```

Project-level config is stored in `projects/api-keys.local.md` (not committed to git):

```
[MY-PROJECT]
provider=github-models
api_key=sk-xxx...
base_url=https://models.inference.ai.azure.com
model=claude-opus
```

## Command Reference

### `oc provider list`

Display all available providers with their status (default, configured, supported targets):

```bash
./oc.sh provider list
```

Example output:
```
Available LLM providers

Anthropic (direct) ◆ (hub default)
  Direct Anthropic API for Claude models
  Targets: ["opencode", "opencode"]

MammouthAI
  OpenAI-compatible proxy to Claude (FR-hosted)
  Targets: ["opencode"]
  Base URL: https://api.mammouth.ai/v1

...
```

### `oc provider set-default`

Interactively configure the hub default provider:

```bash
./oc.sh provider set-default
```

You'll be prompted to:
1. Select a provider
2. Enter API credentials (masked input for security)
3. Optionally enter a custom base URL

The configuration is written to `config/hub.json` **and `opencode.json` is regenerated immediately** — no need to run `oc deploy` manually.

### `oc provider set <PROJECT_ID> [PROVIDER] [API_KEY] [BASE_URL]`

Configure a provider for a specific project:

```bash
# Interactive
./oc.sh provider set MY-PROJECT

# Non-interactive (direct)
./oc.sh provider set MY-PROJECT mammouth "sk-xxx" "https://api.mammouth.ai/v1"
```

If `PROVIDER`, `API_KEY`, or `BASE_URL` are omitted, you'll be prompted.

The configuration is written to `projects/api-keys.local.md`.

### `oc provider get <PROJECT_ID>`

Display the effective provider configuration for a project:

```bash
./oc.sh provider get MY-PROJECT
```

Example output:
```
Effective configuration for MY-PROJECT

  Provider : mammouth
  Model    : claude-opus
  API Key  : sk-xxx****
  Base URL : https://api.mammouth.ai/v1
```

Shows the resolved configuration after merging project-level and hub-level settings.

## Provider Setup Guides

### Anthropic (Default)

**Supported targets**: OpenCode, OpenCode

1. Get your API key from [console.anthropic.com](https://console.anthropic.com)
2. Run `./oc.sh provider set-default` or `./oc.sh config set <PROJECT_ID>`
3. Choose "Anthropic" and enter your API key

### MammouthAI

**Supported targets**: OpenCode

MammouthAI is an OpenAI-compatible proxy hosted in France that works with Anthropic models.

1. Get your API key from [mammouth.ai](https://mammouth.ai)
2. Run `./oc.sh provider set-default`
3. Choose "MammouthAI" (option 2)
4. Enter your API key (default base URL will be used: `https://api.mammouth.ai/v1`)

```bash
# Or via config:
./oc.sh config set MY-PROJECT --provider mammouth --api-key sk-xxx
```

### GitHub Models

**Supported targets**: OpenCode

GitHub Models provides access to various models via the GitHub/Copilot API.

1. Get your token from [github.com/settings/tokens](https://github.com/settings/tokens)
2. Run `./oc.sh provider set-default`
3. Choose "GitHub Models" (option 3)
4. Enter your GitHub token
5. Optionally override the base URL (default: `https://models.inference.ai.azure.com`)

```bash
# Or via config:
./oc.sh config set MY-PROJECT \
  --provider github-models \
  --api-key ghp_xxx \
  --base-url https://models.inference.ai.azure.com
```

### AWS Bedrock

**Supported targets**: OpenCode

AWS Bedrock uses the **native `amazon-bedrock` provider** built into OpenCode. It requires a **Bedrock bearer token** (generated from the Amazon Bedrock console — long-term API key).

**How it works:**
- The bearer token is stored in `config/hub.json` (never in `opencode.json`)
- `opencode.json` is generated with an empty `amazon-bedrock` provider block
- When you run `oc start`, the token is injected as `AWS_BEARER_TOKEN_BEDROCK` automatically

1. Generate a bearer token from the [Amazon Bedrock console](https://console.aws.amazon.com/bedrock/) under **API Keys**
2. Request model access in the **Model catalog** for the models you want
3. Run `./oc.sh provider set-default`
4. Choose "AWS Bedrock (natif)" and enter your bearer token

The generated `opencode.json` will look like:
```json
{
  "$schema": "https://opencode.ai/config.json",
  "model": "amazon-bedrock/anthropic.claude-sonnet-4-5",
  "provider": {
    "amazon-bedrock": {}
  }
}
```

At launch, `oc start` injects:
```bash
AWS_BEARER_TOKEN_BEDROCK=<token> opencode
```

```bash
# Or configure per-project:
./oc.sh config set MY-PROJECT --provider bedrock --api-key <bearer-token>
```

### Ollama (Local)

**Supported targets**: OpenCode

Ollama allows you to run LLMs locally.

1. Install Ollama from [ollama.ai](https://ollama.ai)
2. Start the Ollama server: `ollama serve`
3. Run `./oc.sh provider set-default`
4. Choose "Ollama" (option 5)
5. The default base URL (`http://localhost:11434/v1`) will be used

```bash
# Or via config:
./oc.sh config set MY-PROJECT \
  --provider ollama \
  --base-url http://localhost:11434/v1
```

Note: Ollama doesn't require an API key, but one can be set for custom authentication layers.

### GitHub Copilot

**Supported targets**: OpenCode

GitHub Copilot uses **OAuth authentication** — no API key is required. Authentication is handled via `opencode auth`, which opens an OAuth flow with GitHub directly.

**Prerequisite:** an active GitHub Copilot subscription.

**How it works:**
- No API key to manage — the OAuth token is handled by OpenCode
- The `opencode_prefix` is `github-copilot` in the generated configuration
- `config/hub.json` does not need an `api_key` field for this provider

**Setup:**

1. Authenticate via `opencode auth` (once, can be done before or after configuring the hub):

```bash
opencode auth
# Follows the GitHub OAuth flow — opens a browser to authorize
```

2. Set GitHub Copilot as the hub default provider:

```bash
./oc.sh provider set-default
# → Choose "GitHub Copilot"
```

3. Or configure it for a specific project:

```bash
./oc.sh config set MY-PROJECT --provider github-copilot
# No --api-key required
```

The generated `opencode.json` will look like:

```json
{
  "$schema": "https://opencode.ai/config.json",
  "model": "github-copilot/claude-sonnet-4.5",
  "provider": {
    "github-copilot": {}
  }
}
```

> **Technical note:** The hub accepts both `claude-sonnet-4-5` (internal format) and `claude-sonnet-4.5` (GitHub Copilot API format). The provider's `model_aliases` perform the transformation automatically.

**Available models via GitHub Copilot:**

| Internal name (hub) | GitHub Copilot API name |
|---------------------|------------------------|
| `claude-sonnet-4-5` | `claude-sonnet-4.5` |
| `claude-sonnet-4-5-v2` | `claude-sonnet-4.5-v2` |
| `claude-opus-4` | `claude-opus-4` |
| `claude-haiku-3-5` | `claude-haiku-3.5` |

> These model names are automatically transformed from the hub's internal format (e.g. `claude-sonnet-4-5`) via `model_aliases`.

**Note:** unlike other providers, `config/hub.json` does not contain an `api_key` field for GitHub Copilot. The `hub.json` file is still gitignored as a security measure.

## Workflows

### Using Different Providers for Different Projects

```bash
# Set hub default to Anthropic
./oc.sh provider set-default
# → Choose Anthropic

# Override specific project to use GitHub Models
./oc.sh config set MY-PYTHON-PROJECT --provider github-models --api-key ghp_xxx

# Another project uses MammouthAI
./oc.sh config set MY-JS-PROJECT --provider mammouth --api-key sk-xxx
```

### Switching Providers

To change a provider configuration:

```bash
# For hub default:
./oc.sh provider set-default

# For a project:
./oc.sh config set MY-PROJECT
# → Follow prompts to update provider/key/model
```

### Using Local Ollama for Development

```bash
# Start Ollama (in a separate terminal):
ollama serve

# Configure your project to use Ollama:
./oc.sh config set MY-PROJECT --provider ollama

# Deploy and start:
./oc.sh deploy all MY-PROJECT
./oc.sh start MY-PROJECT
```

## Security

- **API Keys**: All API keys are stored in local files (`.gitignore`d) and never committed to git.
- **Masking**: When viewing configurations, API keys are masked to show only the first 8 characters.
- **Environment-specific**: Each environment can have different provider configurations.

### Files with Secrets

The following files contain API credentials and are **never committed to git**:

| File | Why gitignored |
|------|---------------|
| `config/hub.json` | Contains `api_key` / bearer token — always gitignored |
| `opencode.json` | Generated by `adapter_deploy`, reflects local provider config — always gitignored |
| `projects/api-keys.local.md` | Per-project API keys — always gitignored by design |

A safe, secret-free template is committed at `config/hub.json.example`. On first run (or after a fresh clone), `hub.json` is automatically created from this template if it does not exist.

```bash
# After a fresh clone, run this to configure your provider:
./oc.sh provider set-default
```

## Troubleshooting

### "Provider not supported"

If you see this error, ensure you're using one of the supported providers:
- `anthropic`
- `mammouth`
- `github-models`
- `bedrock`
- `ollama`
- `github-copilot`

### OpenCode shows "provider not supported" warning

This is expected. OpenCode only supports Anthropic. If you need to use OpenCode:
1. Configure an Anthropic API key at the hub level, or
2. Override your project to use `anthropic` provider

### Model not found / API errors

1. Verify your API key is correct: `./oc.sh provider get <PROJECT_ID>`
2. Check the base URL is correct for your provider
3. Ensure the provider service is running (especially for Ollama)
4. Test your API key directly with the provider's CLI or API

### Provider changes not applied

After `oc provider set-default`, `opencode.json` is automatically regenerated — no manual step needed.

For project-level changes (`oc config set` or `oc provider set`), redeploy:

```bash
./oc.sh deploy all MY-PROJECT
```

## Related Commands

- `./oc.sh config set` — Manage project-level provider and model configuration
- `./oc.sh config get` — View effective configuration for a project
- `./oc.sh deploy all` — Deploy agents with current provider config
- `./oc.sh start` — Start OpenCode with the configured provider
- `./oc.sh init` — Set up a new project (includes provider step)
