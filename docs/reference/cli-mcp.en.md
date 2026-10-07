> [Lire en francais](cli-mcp.fr.md)

# CLI Reference — MCP & Plugins

## MCP Management

MCP servers built into oh: `figma`, `github`, `gitlab`, `gslides`, `jira`, `linear`, `team`, plus the custom servers declared in `~/.oh/mcp/<name>/manifest.json`. A session gets the project MCP servers (or only those listed by the workflow `mcp:` field) in its session bundle. Per-server pages: [Figma](mcp-figma.en.md), [GitHub](mcp-github.en.md), [GitLab](mcp-gitlab.en.md), [Google Slides](mcp-gslides.en.md), [Jira](mcp-jira.en.md), [Linear](mcp-linear.en.md), [Team](mcp-team.en.md).

Every command below (except `list` and `serve`) accepts `-p, --project <name or ID>`.

### oh mcp enable

Enables an MCP service at hub level or for a project. With `--project` and no token, offers to inherit the hub token or to configure a new one.

```
oh mcp enable <service> [options]
```

| Flag | Short | Type | Description |
|------|-------|------|-------------|
| `--project` | `-p` | string | Project name or ID |

**Example:**

```bash
oh mcp enable figma
oh mcp enable gitlab --project my-project
```

---

### oh mcp disable

Disables an MCP service at hub level or for a project. With `--project`, the service is disabled for that project whatever the hub config.

```
oh mcp disable <service> [options]
```

| Flag | Short | Type | Description |
|------|-------|------|-------------|
| `--project` | `-p` | string | Project name or ID |

**Example:**

```bash
oh mcp disable figma
oh mcp disable gitlab --project my-project
```

---

### oh mcp reset

Removes the project override of a service (back to the hub config). Requires `--project`.

```
oh mcp reset <service> --project <name>
```

| Flag | Short | Type | Description |
|------|-------|------|-------------|
| `--project` | `-p` | string | **(required)** Project name or ID |

**Example:**

```bash
oh mcp reset figma --project my-project
```

---

### oh mcp setup

Configures an MCP service (interactive wizard: token, options). Services: `figma`, `gitlab`, `gslides`, `jira`; without argument, the service is chosen from a list. The write mode is offered for the services that have write tools (`gitlab`, `jira`). With `--project`, the token is stored in the keychain under a project-scoped key.

```
oh mcp setup [service] [options]
```

| Flag | Short | Type | Description |
|------|-------|------|-------------|
| `--project` | `-p` | string | Project name or ID |

**Example:**

```bash
oh mcp setup
oh mcp setup jira --project my-project
```

---

### oh mcp status

Shows the status of every MCP service. With `--project`, shows the effective config (project overrides included).

```
oh mcp status [options]
```

| Flag | Short | Type | Description |
|------|-------|------|-------------|
| `--project` | `-p` | string | Project name or ID |

**Example:**

```bash
oh mcp status
oh mcp status --project my-project
```

---

### oh mcp serve

Runs a built-in MCP server on stdin/stdout (JSON-RPC). Used by opencode, through the server declaration in the session bundle; rarely run by hand.

```
oh mcp serve <name> [--token-key <key>]
```

| Flag | Type | Description |
|------|------|-------------|
| `--token-key` | string | Keychain key to read the service token from (sets `FIGMA_TOKEN`, `GITLAB_TOKEN` or `GOOGLE_ACCESS_TOKEN` when not already set) |

Tools are exposed without the server name (`get_project`): the tool (opencode) prefixes it, the session sees `gitlab_get_project` (the name used by agents, skills and permission rules). A call under the former prefixed name (`gitlab_get_project` on the server side) is still accepted.

`oh mcp serve workflow` is internal: the `workflow` MCP server (`workflow_status`, `workflow_checkpoint`, `workflow_outputs`) injected into every session bundle, absent from `list`, `setup` and `enable`.

**Example:**

```bash
oh mcp serve gitlab
oh mcp serve figma --token-key openhub.mcp.figma.token
```

---

### oh mcp list

Lists every oh MCP server (`figma`, `github`, `gitlab`, `gslides`, `jira`, `linear`, `team`, then the custom servers of `~/.oh/mcp/`) and their command.

**Alias:** `oh mcp ls`

```
oh mcp list [options]
```

| Flag | Type | Description |
|------|------|-------------|
| `--json` | bool | Output in JSON format |

**Example:**

```bash
oh mcp list
oh mcp ls --json
```

---

### oh service (deprecated)

`oh service`, `oh service setup` and `oh service remove` still exist but are deprecated and hidden from the help: use `oh mcp status`, `oh mcp setup` and `oh mcp disable`.

---

## Plugin management

`oh plugin` (global opencode V1 plugins, RTK included) was removed in v5: plugins are declared per workflow (`plugins:`), see [Shipped workflows](workflows.en.md#plugins-and-code-mode).
