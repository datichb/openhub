# CLI Reference — `oh service` and `oh mcp` commands

> [Lire en français](services.fr.md)

Manage external services and MCP (Model Context Protocol) servers built into the `oh` binary.

---

## Architecture

MCP servers are **built into the Go binary** — there is no separate `servers/` directory or Node.js build step. Each server is implemented natively in `cli/internal/mcp/` and served via stdio JSON-RPC.

Available servers:
- **figma** — Figma API integration (files, components, UI signals)
- **gitlab** — GitLab API integration (issues, MRs, labels, milestones)
- **gslides** — Google Slides integration

---

## `oh mcp` — MCP server management

### `oh mcp list [--json]`

List all available MCP servers and their status.

```bash
oh mcp list
oh mcp list --json
```

### `oh mcp serve <name>`

Start an MCP server via stdio JSON-RPC. This is the command injected into `opencode.json` during deployment.

```bash
oh mcp serve figma
oh mcp serve gitlab
oh mcp serve gslides
```

---

## `oh service` — Service configuration

### Synopsis

```bash
oh service <subcommand> [service] [options]
```

| Subcommand | Description |
|---|---|
| `setup [name]` | Configure a service interactively (token, validation) |
| `remove <name>` | Remove a service configuration |

**Aliases:** `oh figma setup` · `oh gitlab setup` · `oh gslides setup`

### `oh service setup`

Interactively configure a service (credentials stored in macOS Keychain).

```bash
oh service setup [service-name]
```

**Behavior:**
- If `service-name` is omitted, displays a selection menu.
- Guides you through each required credential.
- Validates token format and API connectivity.
- Stores credentials securely in the system keychain.
- Enables the service in `hub.toml`.

**Examples:**

```bash
# Interactive mode — selection menu
oh service setup

# Configure Figma directly
oh service setup figma

# Configure GitLab (alias)
oh gitlab setup
```

### `oh service remove`

Remove a service configuration (removes credentials from keychain).

```bash
oh service remove <service-name>
```

---

## Deployment — `mcpServers` in `opencode.json`

During `oh deploy`, the CLI injects an `mcpServers` block into the project's `opencode.json` for each enabled service:

```json
{
  "mcpServers": {
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

### Per-project MCP selection

Services are selected per-project during `oh init` or via `oh project configure`:

```bash
# During project initialization
oh init
# → Step prompts: "Enable MCP integrations for this project?"

# Change MCP selection for an existing project
oh project configure --services figma,gitlab
```

---

## Runtime environment variables

At runtime, MCP servers read credentials from the environment. The `oh` binary injects these from the keychain automatically.

| Service | Required variables |
|---------|-------------------|
| figma | `FIGMA_TOKEN` |
| gitlab | `GITLAB_TOKEN`, `GITLAB_URL` |
| gslides | `GOOGLE_ACCESS_TOKEN` |

---

## Configuration storage (`hub.toml`)

Service enablement is stored in `~/.oh/hub.toml`:

```toml
[services]
figma = true
gitlab = true
gslides = false
```

Credentials are stored in the system keychain (macOS Keychain / Linux secret-service), not in plaintext files.

---

## See also

- [Figma Integration Guide](../guides/figma-integration.en.md)
- [GitLab Integration Guide](../guides/gitlab-integration.en.md)
- [Full CLI Reference](cli.en.md)
