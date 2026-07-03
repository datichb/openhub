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
./oh config set
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
./oh init MY-PROJECT
# or
./oh config set MY-PROJECT
```

During `oh init`, you'll be prompted for an optional project-level provider (step 4).

During `oh config set`, you can specify `--provider` and related flags:

```bash
./oh config set MY-PROJECT --provider github-models --api-key sk-xxx
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

### `oh config list --providers`

Display all available providers with their status (default, configured):

```bash
./oh config list --providers
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

### `oh config set`

Interactively configure the hub default provider:

```bash
./oh config set
```

You'll be prompted to:
1. Select a provider
2. Enter API credentials (masked input for security)
3. Optionally enter a custom base URL

The configuration is written to `config/hub.json` **and `opencode.json` is regenerated immediately** — no need to run `oh deploy` manually.

In non-interactive mode, you can pass flags directly:

```bash
# Configure a provider with an API key
./oh config set --provider anthropic --api-key sk-...

# Configure a provider without an API key (e.g. AWS credentials)
./oh config set --provider bedrock

# Update only the hub default model
./oh config set --model claude-opus-4
```

> **Note:** Per-project provider configuration is managed via `oh config set <PROJECT_ID>` — see `./oh config set --help` or the [configuration reference](../reference/config.en.md).

## Provider Setup Guides

### Anthropic (Default)


1. Get your API key from [console.anthropic.com](https://console.anthropic.com)
2. Run `./oh config set` or `./oh config set <PROJECT_ID>`
3. Choose "Anthropic" and enter your API key

### MammouthAI


MammouthAI is an OpenAI-compatible proxy hosted in France that works with Anthropic models.

1. Get your API key from [mammouth.ai](https://mammouth.ai)
2. Run `./oh config set`
3. Choose "MammouthAI" (option 2)
4. Enter your API key (default base URL will be used: `https://api.mammouth.ai/v1`)

```bash
# Or via config:
./oh config set MY-PROJECT --provider mammouth --api-key sk-xxx
```

### GitHub Models


GitHub Models provides access to various models via the GitHub/Copilot API.

1. Get your token from [github.com/settings/tokens](https://github.com/settings/tokens)
2. Run `./oh config set`
3. Choose "GitHub Models" (option 3)
4. Enter your GitHub token
5. Optionally override the base URL (default: `https://models.inference.ai.azure.com`)

```bash
# Or via config:
./oh config set MY-PROJECT \
  --provider github-models \
  --api-key ghp_xxx \
  --base-url https://models.inference.ai.azure.com
```

### AWS Bedrock


AWS Bedrock uses the **native `amazon-bedrock` provider** built into OpenCode. It requires a **Bedrock bearer token** (generated from the Amazon Bedrock console — long-term API key).

**How it works:**
- The bearer token is stored in `config/hub.json` (never in `opencode.json`)
- `opencode.json` is generated with an empty `amazon-bedrock` provider block
- When you run `oh start`, the token is injected as `AWS_BEARER_TOKEN_BEDROCK` automatically

1. Generate a bearer token from the [Amazon Bedrock console](https://console.aws.amazon.com/bedrock/) under **API Keys**
2. Request model access in the **Model catalog** for the models you want
3. Run `./oh config set`
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

At launch, `oh start` injects:
```bash
AWS_BEARER_TOKEN_BEDROCK=<token> opencode
```

```bash
# Or configure per-project:
./oh config set MY-PROJECT --provider bedrock --api-key <bearer-token>
```

### Ollama (Local)


Ollama allows you to run LLMs locally.

1. Install Ollama from [ollama.ai](https://ollama.ai)
2. Start the Ollama server: `ollama serve`
3. Run `./oh config set`
4. Choose "Ollama" (option 5)
5. The default base URL (`http://localhost:11434/v1`) will be used

```bash
# Or via config:
./oh config set MY-PROJECT \
  --provider ollama \
  --base-url http://localhost:11434/v1
```

Note: Ollama doesn't require an API key, but one can be set for custom authentication layers.

### GitHub Copilot


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
./oh config set
# → Choose "GitHub Copilot"
```

3. Or configure it for a specific project:

```bash
./oh config set MY-PROJECT --provider github-copilot
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
./oh config set
# → Choose Anthropic

# Override specific project to use GitHub Models
./oh config set MY-PYTHON-PROJECT --provider github-models --api-key ghp_xxx

# Another project uses MammouthAI
./oh config set MY-JS-PROJECT --provider mammouth --api-key sk-xxx
```

### Switching Providers

To change a provider configuration:

```bash
# For hub default:
./oh config set

# For a project:
./oh config set MY-PROJECT
# → Follow prompts to update provider/key/model
```

### Using Local Ollama for Development

```bash
# Start Ollama (in a separate terminal):
ollama serve

# Configure your project to use Ollama:
./oh config set MY-PROJECT --provider ollama

# Deploy and start:
./oh deploy all MY-PROJECT
./oh start MY-PROJECT
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
./oh config set
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

1. Verify your API key is correct: `./oh config get <PROJECT_ID>`
2. Check the base URL is correct for your provider
3. Ensure the provider service is running (especially for Ollama)
4. Test your API key directly with the provider's CLI or API

### Provider changes not applied

After `oh config set`, `opencode.json` is automatically regenerated — no manual step needed.

For project-level changes (`oh config set`), redeploy:

```bash
./oh deploy all MY-PROJECT
```

## Diagnosing and Resolving Provider Errors

### Status messages at startup (`oh start`)

When you run `oh start`, the hub displays the provider status in the context block:

| Indicator | Meaning | Recommended action |
|-----------|---------|-------------------|
| ✅ `key configured` | Provider is reachable and credentials are valid | None — everything is fine |
| ⚠️ `endpoint unreachable (timeout 3s)` | The LLM server is not responding | Check your network connection or the URL validity |
| ⚠️ `credentials not detected` | No API key / no AWS credentials found | Configure via `oh config set` or `/connect` in OpenCode |
| ⚠️ `API key missing — provider not injected` | Key is missing from hub/project config | Add the key with `oh config set` |
| ⚠️ `model declared without matching provider block` | Model references a provider not configured in opencode.json | Redeploy (`oh deploy`) or use `/connect` |
| ⚠️ `baseURL contains /chat/completions` | Duplicate suffix in URL (the AI SDK adds it automatically) | Fix the URL — use the `/v1` root without suffix |

These messages are **non-blocking**: OpenCode still launches and you can configure the provider from the interface.

### Configuring a provider directly in OpenCode

If the hub cannot inject credentials (key missing, expired, or invalid), you can configure the provider directly in OpenCode using the `/connect` command:

1. Launch OpenCode: `./oh start MY-PROJECT`
2. In the TUI, type `/connect`
3. Select your provider and follow the instructions
4. Credentials are stored in `~/.local/share/opencode/auth.json`

> **Note:** Configuration via `/connect` is complementary to hub configuration. Native OpenCode credentials serve as a fallback when the hub does not inject a provider block into `opencode.json`.

### Common errors reported by OpenCode

| OpenCode error | Likely cause | Solution |
|----------------|--------------|----------|
| `ProviderInitError` | Invalid configuration in `opencode.json` | Run `oh deploy MY-PROJECT` to regenerate, or use `/connect` |
| `ProviderModelNotFoundError` | Incorrect model ID or litellm provider without model declaration | Check with `opencode models` or `/models` in the TUI |
| "provider not available" notification on agent selection | Expired API key, endpoint down, or misconfiguration | Check ⚠️ messages in `oh start`, then use `/connect` or `oh config set provider` |

### Specific case: AWS Bedrock

If the notification appears with Bedrock, check credentials in this order:

```bash
# Option 1: bearer token (recommended with the hub)
export AWS_BEARER_TOKEN_BEDROCK=<your-token>

# Option 2: named AWS profile
export AWS_PROFILE=my-profile

# Option 3: IAM access keys
export AWS_ACCESS_KEY_ID=AKIA...
export AWS_SECRET_ACCESS_KEY=...

# Then restart
./oh start MY-PROJECT
```

### Specific case: MammouthAI (litellm)

The correct URL for MammouthAI is `https://api.mammouth.ai/v1` **without** the `/chat/completions` suffix. The hub detects and reports this issue at startup:

```bash
# Fix in api-keys.local.md or via the command
./oh config set provider mammouth --project MY-PROJECT
# Or at hub level
./oh config set provider mammouth
```

## Related Commands

- `./oh config set` — Manage hub-level or project-level provider and model configuration
- `./oh config list --providers` — Display available providers and their status
- `./oh config get` — View effective configuration for a project
- `./oh config init-providers [--force]` — Initialize provider configuration
- `./oh deploy all` — Deploy agents with current provider config
- `./oh start` — Start OpenCode with the configured provider
- `./oh init` — Set up a new project (includes provider step)
