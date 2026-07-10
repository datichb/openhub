# CLI Reference — MCP Servers (`oh mcp`)

> [Lire en français](services.fr.md)

Manage MCP (Model Context Protocol) servers built into the `oh` binary.

---

## Architecture

MCP servers are **built into the Go binary** — no separate `servers/` directory or Node.js build step. Each server is natively implemented in `cli/internal/mcp/` and served via stdio JSON-RPC.

Available servers:
- **figma** — Figma API integration (files, components, UI signals)
- **gitlab** — GitLab API integration (issues, MRs, labels, milestones)
- **gslides** — Google Slides integration
- **team** — Team data (members, wiki, events) — no token required

---

## Commands

### `oh mcp enable <service> [--project <name>]`

Enable an MCP service at the hub level or for a specific project.

```bash
# Enable at hub level
oh mcp enable figma

# Enable for a specific project
oh mcp enable figma --project my-project
```

**Behavior with `--project`:**
- If no token is found (neither project, hub, nor env), a prompt offers:
  - Inherit the hub configuration (use existing hub token)
  - Configure a project-specific token

---

### `oh mcp disable <service> [--project <name>]`

Disable an MCP service.

```bash
# Disable at hub level
oh mcp disable gitlab

# Disable for a project (override: disabled even if hub enables it)
oh mcp disable gitlab --project my-project
```

With `--project`, the service is **explicitly disabled** for that project, regardless of hub configuration.

---

### `oh mcp reset <service> --project <name>`

Remove the project-level override for an MCP service, reverting to hub configuration.

```bash
oh mcp reset figma --project my-project
```

> **Note:** `--project` is required. This command has no meaning at hub level.

After a reset, the project inherits the hub state for that service (enabled/disabled, token, options).

---

### `oh mcp setup [--project <name>]`

Launch an interactive wizard to configure an MCP service (token, options).

```bash
# Hub configuration
oh mcp setup

# Project configuration
oh mcp setup --project my-project
```

**The wizard:**
1. Service selection (Figma, GitLab, Google Slides)
2. Token input (masked)
3. For GitLab: optional write mode activation
4. Secure storage in keychain

---

### `oh mcp status [--project <name>]`

Display the status of all MCP services.

```bash
# Hub status
oh mcp status

# Effective status for a project (includes overrides)
oh mcp status --project my-project
```

**Columns displayed:**

| Column  | Description |
|---------|-------------|
| SERVICE | Service name (Figma, GitLab, etc.) |
| STATUS  | enabled / disabled |
| SOURCE  | hub / project (where the effective config comes from) |
| TOKEN   | env:VAR / keychain / missing / — |

---

### `oh mcp serve <name>`

Start an MCP server via stdio JSON-RPC. This is the command injected into `opencode.json` during deployment.

```bash
oh mcp serve figma
oh mcp serve gitlab
oh mcp serve gslides
oh mcp serve team
```

---

### `oh mcp list [--json]`

List all available MCP servers.

```bash
oh mcp list
oh mcp list --json
```

---

## Configuration

### Hub-level (`~/.oh/hub.toml`)

Global service activation is stored in `hub.toml`:

```toml
[mcp.figma]
enabled = true
token_key = "figma-token"

[mcp.gitlab]
enabled = true
token_key = "gitlab-token"
write_enabled = true

[mcp.gslides]
enabled = false
token_key = "gslides-token"
```

### Project-level (`ProjectMCPConfig`)

Each project can override the hub configuration. Project config is stored in the database via `oh mcp enable/disable/setup --project`.

Per-service fields:

| Field | Type | Description |
|-------|------|-------------|
| `name` | string | Service name (figma, gitlab, gslides, team) |
| `enabled` | *bool | `nil` = inherit hub, `true` = force-enable, `false` = force-disable |
| `token_key` | string | Keychain key override (empty = inherit hub) |
| `write_enabled` | *bool | GitLab write mode (nil = inherit hub) |

### Cascade and inheritance

```
Hub (hub.toml)
  └── Project (MCPConfig)
        └── Environment (env variables)
```

**Resolution rules:**

1. If the project has no `MCPConfig` → fully inherits from hub
2. If the project has an entry for a service:
   - `enabled = nil` → inherits hub state
   - `enabled = true/false` → explicit override
   - `token_key` empty → inherits hub token
   - `token_key` non-empty → uses project token
3. Environment variables (`FIGMA_TOKEN`, etc.) always take priority over keychain

---

## Deployment — `mcp` in `opencode.json`

During `oh deploy`, the CLI injects an `mcp` block into the project's `opencode.json` for each **effectively enabled** service (after cascade resolution):

```json
{
  "mcp": {
    "figma": {
      "command": "oh",
      "args": ["mcp", "serve", "figma"]
    },
    "gitlab": {
      "command": "oh",
      "args": ["mcp", "serve", "gitlab"]
    }
  }
}
```

Only servers with a valid token (env, keychain, or tokenless for `team`) are deployed.

---

## Runtime environment variables

| Service | Variable | Description |
|---------|----------|-------------|
| figma | `FIGMA_TOKEN` | Figma access token |
| gitlab | `GITLAB_TOKEN` | GitLab access token |
| gitlab | `GITLAB_URL` | GitLab instance URL |
| gslides | `GOOGLE_ACCESS_TOKEN` | Google OAuth token |

---

## Migrating from `oh service`

The `oh service` commands are **deprecated**. Use the `oh mcp` equivalents:

| Old command | New command |
|---|---|
| `oh service` | `oh mcp status` |
| `oh service setup` | `oh mcp setup` |
| `oh service setup -p <project>` | `oh mcp setup --project <project>` |
| `oh service remove <service>` | `oh mcp disable <service>` |

The `oh service` commands remain functional but display a deprecation message.

---

## See also

- [Figma integration guide](../guides/figma-integration.en.md)
- [GitLab integration guide](../guides/gitlab-integration.en.md)
- [Full CLI reference](cli.en.md)
