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

`oh mcp serve workflow` is internal: the `workflow` MCP server (see [workflow MCP server](#workflow-mcp-server)) injected into every session bundle, absent from `list`, `setup` and `enable`.

**Example:**

```bash
oh mcp serve gitlab
oh mcp serve figma --token-key openhub.mcp.figma.token
```

---

### workflow MCP server

Injected into every session started from a workflow; stateless, it passes each call to the oh daemon for the calling session (a subagent belongs to the oh session that delegated it). Agents see its tools as `workflow_<tool>`.

| Tool | Parameters | Role |
|------|------------|------|
| `workflow_status` | — | Workflow state: mode, checkpoints, next checkpoint, locked agents and operations, outputs |
| `workflow_checkpoint` | `id`, `summary`, `reports` (optional) | Passes a checkpoint (waits for the validation when it pauses in the mode); `reports` cites reports `rp-…` |
| `workflow_outputs` | `type`, `value`, `id` | Declares a typed output |
| `workflow_report` | `title`, `body` | Files the whole report of the agent; oh answers with its id `rp-<n>` |
| `workflow_report_get` | `id`, `section` (optional) | Reads a report, whole or one section (Markdown heading) |
| `workflow_reports` | — | Lists the reports of the session (id, author, title, size, time, sections) |

**Reports (`workflow_report`).** The agent files its whole report; oh keeps it and the coordinators only pass its id on. The author is the agent oh sees calling the tool (an `agent` parameter would be ignored). The reports filed since the previous checkpoint, and those the agent cites in `reports`, are attached to the checkpoint card; each filing enters the timeline (`▤ rp-2 (developer)`). Limits: 64 KiB per report, 100 reports and 4 MiB per session; beyond them the tool refuses and the agent includes the report in its block. Without the daemon (or when it keeps no report: `OH_WORKFLOW_REPORTS=off`), the tool answers "not recorded" without an error and the agent falls back on the whole report in its block. Agents never read nor write the reports other than through these tools (global guard on `~/.oh/sessions`). User side: [`oh session report`](cli-sessions.en.md#oh-session-report).

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
