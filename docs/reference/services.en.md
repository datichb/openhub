# CLI Reference — `oc service` command

> [Lire en français](services.fr.md)

Manage external services and integrations connected via the MCP (Model Context Protocol).

---

## Synopsis

```bash
oc service <subcommand> [service] [options]
```

---

## `oc service setup`

Interactively configure a service (credentials, validation, MCP build).

```bash
oc service setup [service-name]
```

**Behavior:**
- If `service-name` is omitted, displays a selection menu from available services.
- Guides the user step-by-step through each required credential.
- Displays contextual help for each credential (how to obtain it).
- If an existing value is detected, offers to keep it.
- Validates token format if a `validation_pattern` is defined in the catalog.
- Performs an API validation call (if a `validation.endpoint` is defined).
- Saves credentials to `~/.config/opencode/config.json` (section `env`).
- Automatically builds the MCP server if needed.

**Arguments:**

| Argument | Description |
|----------|-------------|
| `service-name` | Service identifier (`figma`, `gitlab`, etc.). Optional — interactive menu if omitted. |

**Examples:**

```bash
# Interactive mode — selection menu
oc service setup

# Configure Figma directly
oc service setup figma

# Configure GitLab (alias)
oc gitlab setup

# Non-interactive mode (CI/CD)
FIGMA_PERSONAL_ACCESS_TOKEN=figd_xxx FIGMA_TEAM_ID=123456 \
  OC_NON_INTERACTIVE=1 oc service setup figma
```

---

## `oc service status`

Check the status of one or all services (configuration, token validity, MCP build).

```bash
oc service status [service-name]
```

**Behavior:**
- If `service-name` is omitted, shows the status of all services in the catalog.
- For each service, displays:
  - Each credential: present (masked value for secrets) or missing.
  - Token validation: quick API call if an endpoint is defined.
  - MCP build status: presence of `dist/index.js`.

**Arguments:**

| Argument | Description |
|----------|-------------|
| `service-name` | Service identifier. Optional — all services if omitted. |

**Examples:**

```bash
# Status of all services
oc service status

# Figma status only
oc service status figma

# Via alias
oc figma status
```

---

## `oc service list`

List all available services in the catalog with their configuration status.

```bash
oc service list
```

**Behavior:**
- Displays a table with: service name, description, status (Configured / Not configured).
- Equivalent to `oc service` with no argument.

**Examples:**

```bash
oc service list
oc service
```

---

## `oc service remove`

Remove a service configuration (removes environment variables from `~/.config/opencode/config.json`).

```bash
oc service remove <service-name>
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
oc service remove figma
oc service remove gitlab
```

---

## Aliases

Common services have aliases to shorten commands:

| Alias | Equivalent |
|-------|-----------|
| `oc figma <cmd> [args]` | `oc service <cmd> [args] figma` |
| `oc gitlab <cmd> [args]` | `oc service <cmd> [args] gitlab` |

**Examples with aliases:**

```bash
oc figma setup          # = oc service setup figma
oc figma status         # = oc service status figma
oc gitlab setup         # = oc service setup gitlab
oc gitlab status        # = oc service status gitlab
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

## See also

- [Figma Integration Guide](../guides/figma-integration.en.md)
- [Full CLI Reference](cli.en.md)
- [MCP Servers Architecture](../../servers/README.md)
