> [Lire en français](providers.fr.md)

# Provider Configuration

This guide covers how OpenCode Hub resolves LLM providers, manages API tokens, and passes provider settings to sessions.

## Supported Providers

| Provider | Backend | Auth Method |
|----------|---------|-------------|
| **bedrock** (default) | AWS Bedrock | AWS Bearer Token (via keychain) |
| **anthropic** | Anthropic API | API Key (via env or keychain) |
| **github-copilot** | GitHub Copilot | OAuth |
| **openrouter** | OpenRouter | API Key |

## Provider Resolution Order

When `oh run` launches a session, the provider is resolved in this order:

1. `--provider` / `-P` flag on `oh run` (highest priority)
2. Project-level override (`project.Provider` in the database)
3. `llm.default_provider` in `~/.oh/hub.toml`
4. `"bedrock"` (hardcoded fallback)

## Hub-Level Configuration

Set the default provider for all projects:

```bash
oh config set llm.default_provider bedrock
```

In `~/.oh/hub.toml`:

```toml
[llm]
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
oh provider setup bedrock
# Select the bearer mode -> enter your bearer token
# Stored under key: bedrock-token-default (or bedrock-token-<project-id>)
```

The token never leaves the machine and is never passed to opencode: the credential proxy of the oh daemon holds it and signs the calls; the session only receives an `ohs_…` token specific to its group (see [Sessions v5 › LLM keys](sessions-v5.en.md#llm-keys)).

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

The provider block is generated in the session bundle at launch (`oh deploy` removed in v5); the key stays in the keychain, on the oh daemon proxy side:

```bash
oh run <workflow> -p my-project --provider anthropic
```

## MCP Service Tokens

MCP servers (Figma, GitLab, Google Slides) need their own tokens:

```bash
oh mcp setup
```

Per-project configuration (overrides hub):

```bash
oh mcp setup --project my-project
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

## Launch Flow

At each launch (`oh run <workflow>`), the provider configuration is written into the session bundle (`~/.oh/bundles/<hash>/`), not into the project:

The bundle builder reads your configured provider and model, then generates the appropriate provider block. This block points to the credential proxy of the oh daemon: it never contains the real key (the session authenticates to the proxy with an `ohs_…` token). To see the bundle: `oh bundle show <workflow>`.

## Switching Providers

```bash
# Temporarily (one session)
oh run <workflow> --provider anthropic

# Permanently (hub default)
oh config set llm.default_provider anthropic

# Permanently (one project)
oh project configure my-project --provider github-copilot   # applied at next launch
```

## Checking Configuration

```bash
oh status                  # shows current project's provider/model
oh doctor                  # validates API keys are configured
oh config list             # shows all hub config including provider
```

## Security Best Practices

- Never store API keys in plain text files
- Use `oh provider setup` and `oh mcp setup`, which store in the OS keychain
- For CI/headless: use `OH_PASSPHRASE` env var for the encrypted fallback
- Provider keys stay in the keychain: opencode only receives an `ohs_…` proxy token per session group
- oh no longer writes an `opencode.json` into the project; if you keep one with provider options, it should be gitignored

## Troubleshooting

| Problem | Solution |
|---------|----------|
| "Token not configured" | Run `oh provider setup` (provider) or `oh mcp setup` (MCP service) |
| Provider not recognized | Check spelling: bedrock, anthropic, openrouter, github-copilot |
| Wrong model | Use `oh project configure --model <name>`; applied at next launch (bundle rebuilt) |
| Keychain access denied | Grant terminal access in System Preferences > Privacy |
| Fallback store errors | Check `OH_PASSPHRASE` or re-enter when prompted |
