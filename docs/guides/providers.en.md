> [Lire en français](providers.fr.md)

# Provider Configuration

This guide covers how OpenCode Hub resolves LLM providers, manages API tokens, and deploys provider settings to projects.

## Supported Providers

| Provider | Backend | Auth Method |
|----------|---------|-------------|
| **bedrock** (default) | AWS Bedrock | AWS Bearer Token (via keychain) |
| **anthropic** | Anthropic API | API Key (via env or keychain) |
| **openai** | OpenAI / OpenRouter | API Key |
| **openrouter** | OpenRouter | API Key |

## Provider Resolution Order

When `oh start` launches opencode, the provider is resolved in this order:

1. `--provider` / `-P` flag on `oh start` (highest priority)
2. Project-level override (`project.Provider` in the database)
3. `opencode.default_provider` in `~/.oh/hub.toml`
4. `"bedrock"` (hardcoded fallback)

## Hub-Level Configuration

Set the default provider for all projects:

```bash
oh config set opencode.default_provider bedrock
```

In `~/.oh/hub.toml`:

```toml
[opencode]
default_provider = "bedrock"
```

## Detailed Provider Configuration (hub.toml)

```toml
[provider.bedrock]
aws_profile = "default"           # AWS profile to use
aws_region = "eu-west-1"          # Bedrock region
auth_mode = "bearer"              # "bearer" | "profile"
```

This section is optional — if absent, `oh` uses environment credentials.

## Project-Level Override

Override the provider for a specific project:

```bash
oh project configure my-project --provider anthropic --model claude-sonnet-4-5
```

## Token Management

### AWS Bedrock (Bearer Token)

Tokens are stored in the OS keychain. Configure via:

```bash
oh service setup
# Select the provider -> enter your bearer token
# Stored under key: bedrock-token-default (or bedrock-token-<project-id>)
```

At launch, `oh start` retrieves the token from keychain and passes it as the `AWS_BEARER_TOKEN_BEDROCK` environment variable to opencode.

Configure via the dedicated command:

```bash
oh provider setup              # interactive wizard (hub-level)
oh provider setup --project X  # per-project override
oh provider setup bedrock      # configure a specific provider
```

Resolution order for the bearer token:

1. `bedrock-token-<project-id>` (per-project)
2. `bedrock-token-default` (global)

### Anthropic / OpenAI / OpenRouter

Set your API key in the project's `opencode.json` provider block (injected by `oh deploy`):

```bash
oh deploy -p my-project --provider anthropic
```

Or configure via environment variables that opencode reads directly.

## MCP Service Tokens

MCP servers (Figma, GitLab, Google Slides) need their own tokens:

```bash
oh service setup
```

Per-project configuration (overrides hub):

```bash
oh service setup --project my-project
```
The token is stored in the keychain with a project-specific key.

Interactive wizard that:

1. Asks which service to configure (Figma, GitLab, Google Slides)
2. Prompts for the API token (masked input)
3. Stores in OS keychain
4. Enables the service in `hub.toml`

Tokens are read by MCP servers at runtime via environment variables:

- `FIGMA_TOKEN`
- `GITLAB_TOKEN` (+ `GITLAB_URL` for self-hosted)
- `GOOGLE_ACCESS_TOKEN`

## Secret Storage

**Primary:** OS Keychain (macOS Keychain, Linux secret-service, Windows Credential Manager)

**Known secret keys:**

| Key | Purpose |
|-----|---------|
| `bedrock-token-default` | AWS Bearer token for Bedrock |
| `bedrock-token-<project-id>` | Per-project Bedrock token |
| `anthropic-api-key-default` | Anthropic API key |
| `anthropic-api-key-<project-id>` | Per-project Anthropic API key |
| `openrouter-api-key-default` | OpenRouter API key |
| `openrouter-api-key-<project-id>` | Per-project OpenRouter API key |
| `figma-token` | Figma API token |
| `gitlab-token` | GitLab API token |
| `gslides-token` | Google Slides OAuth token |

**Fallback:** Encrypted file at `~/.oh/secrets.enc`

- Encryption: AES-256-GCM
- Key derivation: Argon2id (t=3, memory=64MB, threads=4)
- Passphrase: `OH_PASSPHRASE` env var or interactive prompt (min 8 chars)

## Deployment Flow

When you run `oh deploy`, the provider configuration is written into the project's `opencode.json`:

```json
{
  "model": "claude-sonnet-4-5",
  "provider": {
    "anthropic": {
      "options": { "apiKey": "..." }
    }
  }
}
```

The deploy engine reads your configured provider and model, then generates the appropriate provider block.

## Switching Providers

```bash
# Temporarily (one session)
oh start --provider anthropic

# Permanently (hub default)
oh config set opencode.default_provider anthropic

# Permanently (one project)
oh project configure my-project --provider openai
oh deploy -p my-project   # re-deploy to apply
```

## Checking Configuration

```bash
oh status                  # shows current project's provider/model
oh doctor                  # validates API keys are configured
oh config list             # shows all hub config including provider
```

## Security Best Practices

- Never store API keys in plain text files
- Use `oh service setup` which stores in OS keychain
- For CI/headless: use `OH_PASSPHRASE` env var for the encrypted fallback
- Bedrock tokens are injected per-session, never written to disk
- `opencode.json` may contain provider options but should be gitignored

## Troubleshooting

| Problem | Solution |
|---------|----------|
| "Token not configured" | Run `oh service setup` |
| Provider not recognized | Check spelling: bedrock, anthropic, openai, openrouter |
| Wrong model | Use `oh project configure --model <name>` then `oh deploy` |
| Keychain access denied | Grant terminal access in System Preferences > Privacy |
| Fallback store errors | Check `OH_PASSPHRASE` or re-enter when prompted |
