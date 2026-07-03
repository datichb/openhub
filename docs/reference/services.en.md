# CLI Reference — `oh service` command

> [Lire en français](services.fr.md)

Manage external services and integrations connected via the MCP (Model Context Protocol).

---

## Synopsis

```bash
oh service <subcommand> [service] [options]
```

**Quick reference** — also accessible via `oh help 8` or `oh service --help`:

| Subcommand | Description |
|---|---|
| `setup [name]` | Configure a service interactively (Figma, GitLab…) |
| `status [name]` | Check a service status (config, token, MCP) |
| `list` | List available services and their status |
| `remove <name>` | Remove a service configuration |
| `deploy <name> [--project ID]` | Deploy the MCP server into a project |

**Aliases:** `oh figma` · `oh gitlab` · `oh gslides` → `oh service ... <name>`

---

## `oh service setup`

Interactively configure a service (credentials, validation, MCP build).

```bash
oh service setup [service-name]
```

**Behavior:**
- If `service-name` is omitted, displays a selection menu from available services.
- Guides the user step-by-step through each required credential.
- Displays contextual help for each credential (how to obtain it).
- If an existing value is detected, offers to keep it.
- Validates token format if a `validation_pattern` is defined in the catalog.
- Performs an API validation call (if a `validation.endpoint` is defined).
- Validates team/resource accessibility (if a `team_validation.endpoint` is defined) — **blocking**: setup cannot complete if team ID is invalid.
- Saves credentials to `~/.config/opencode/config.json` (section `env`).
- Automatically builds the MCP server if needed.

**Arguments:**

| Argument | Description |
|----------|-------------|
| `service-name` | Service identifier (`figma`, `gitlab`, etc.). Optional — interactive menu if omitted. |

**Examples:**

```bash
# Interactive mode — selection menu
oh service setup

# Configure Figma directly
oh service setup figma

# Configure GitLab (alias)
oh gitlab setup

# Non-interactive mode (CI/CD)
FIGMA_PERSONAL_ACCESS_TOKEN=figd_xxx FIGMA_TEAM_ID=123456 \
  OH_NON_INTERACTIVE=1 oh service setup figma
```

---

## `oh service status`

Check the status of one or all services (configuration, token validity, MCP build).

```bash
oh service status [service-name]
```

**Behavior:**
- If `service-name` is omitted, shows the status of all services in the catalog.
- For each service, displays:
  - Each credential: present (masked value for secrets) or missing.
  - Token validation: quick API call if a `validation.endpoint` is defined.
  - Team ID validation: accessibility check if a `team_validation.endpoint` is defined (Figma only).
  - MCP build status: presence of `dist/index.js`.

**Arguments:**

| Argument | Description |
|----------|-------------|
| `service-name` | Service identifier. Optional — all services if omitted. |

**Examples:**

```bash
# Status of all services
oh service status

# Figma status only
oh service status figma

# Via alias
oh figma status
```

---

## `oh service list`

List all available services in the catalog with their configuration status.

```bash
oh service list
```

**Behavior:**
- Displays a table with: service name, description, status (Configured / Not configured).
- Equivalent to `oh service` with no argument.

**Examples:**

```bash
oh service list
oh service
```

---

## `oh service remove`

Remove a service configuration (removes environment variables from `~/.config/opencode/config.json`).

```bash
oh service remove <service-name>
```

**Behavior:**
- Asks for confirmation before removal.
- Only removes keys belonging to the service (other services are not affected).
- If the service is not configured, displays a warning and does nothing.

**Arguments:**

| Argument | Description |
|----------|-------------|
| `service-name` | Service identifier to remove. Required. |

**Examples:**

```bash
oh service remove figma
oh service remove gitlab
```

---

## Aliases

Common services have aliases to shorten commands:

| Alias | Equivalent |
|-------|-----------|
| `oh figma <cmd> [args]` | `oh service <cmd> [args] figma` |
| `oh gitlab <cmd> [args]` | `oh service <cmd> [args] gitlab` |

**Examples with aliases:**

```bash
oh figma setup          # = oh service setup figma
oh figma status         # = oh service status figma
oh gitlab setup         # = oh service setup gitlab
oh gitlab status        # = oh service status gitlab
```

---

## Configuration storage

Credentials are stored in `~/.config/opencode/config.json`, under the `env` key. This file is automatically read by opencode at startup and variables are injected into the MCP server environment.

File structure:

```json
{
  "$schema": "https://opencode.ai/config.json",
  "env": {
    "FIGMA_PERSONAL_ACCESS_TOKEN": "figd_xxx",
    "FIGMA_TEAM_ID": "123456",
    "GITLAB_PERSONAL_ACCESS_TOKEN": "glpat-xxx",
    "GITLAB_BASE_URL": "https://gitlab.mycompany.com"
  }
}
```

---

## Service catalog (`config/services.json`)

The command is driven by the `config/services.json` catalog. Each entry defines:

| Field | Description |
|-------|-------------|
| `label` | Display name |
| `description_fr` / `description_en` | Bilingual description |
| `mcp_server` | Folder name under `servers/` |
| `docs_url` | Official documentation URL |
| `validation.endpoint` | URL to validate the token (optional) |
| `validation.header` | HTTP header name for the token |
| `team_validation.endpoint` | URL to validate team/resource accessibility (optional, URL template with `{FIELD_NAME}`) |
| `team_validation.team_field` | Name of the credential holding the team/resource ID |
| `credentials[]` | List of required credentials |

**Adding a new service:**

Simply add an entry to `config/services.json`. No code changes required.

```json
{
  "services": {
    "my-service": {
      "label": "My Service",
      "description_fr": "Description en français",
      "description_en": "English description",
      "mcp_server": "my-service-mcp",
      "credentials": [
        {
          "key": "MY_SERVICE_API_TOKEN",
          "label_fr": "Token API",
          "label_en": "API Token",
          "secret": true,
          "required": true,
          "help_fr": "How to get this token (FR)...",
          "help_en": "How to get this token (EN)..."
        }
      ]
    }
  }
}
```

---

## Per-project MCP selection

By default, **no MCP server is deployed** to a project (opt-in). The selection is stored as the `- MCP :` field in `projects/projects.md` and applied on every `oh deploy`.

**Setting up during `oh init`:**

Step 4 of the `oh init` wizard prompts:

```
◇  Step 4/6 — MCP Services
│
│  Enable MCP integrations for this project? [y/N]:
```

Answer `Y` to open a multi-select picker listing all configured services. The result is persisted immediately.

**Manual configuration:**

Edit `projects/projects.md` directly and re-run `oh deploy <PROJECT_ID>`:

```markdown
## MY-APP
- MCP : figma-mcp,gitlab-mcp    # CSV list of mcp_server names
# or:
- MCP : all                      # deploy all available MCP servers
# or:
- MCP : none                     # deploy no MCP servers (default if field absent)
```

**Applying a change:**

```bash
# After editing projects.md:
oh deploy MY-APP
```

The `- MCP :` field is read during the deploy phase and controls which servers are copied to `.opencode/servers/` and configured in `opencode.json`.

---

## See also

- [Figma Integration Guide](../guides/figma-integration.en.md)
- [GitLab Integration Guide](../guides/gitlab-integration.en.md)
- [Full CLI Reference](cli.en.md)
- [MCP Servers Architecture](../../servers/README.md)
