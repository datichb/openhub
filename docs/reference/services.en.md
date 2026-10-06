> [Lire en français](services.fr.md)

# CLI Reference — MCP Servers (`oh mcp`)

Manage MCP (Model Context Protocol) servers built into the `oh` binary.

---

## Architecture

MCP servers are **built into the Go binary** — no separate `servers/` directory or Node.js build step. Each server is natively implemented in `cli/internal/mcp/` and served via stdio JSON-RPC.

Available servers:
- **figma** — Figma API integration (files, components, styles)
- **gitlab** — GitLab API integration (issues, MRs, labels, notes, reviewers)
- **gslides** — Google Slides integration
- **team** — Team data (members, wiki, events, claims) — no token required; exposes claim lifecycle (5 statuses: `planned`, `in_progress`, `review`, `blocked`, `done`), claim labels (`agent-reviewed`, `needs-human-review`), `ExternalIID` for tracker linkage, and events emitted on claim/release/transfer operations
- **github** — GitHub API integration (repos, issues, PRs, workflows)
- **jira** — Jira integration (Cloud and Server/Data Center)
- **linear** — Linear integration (issues, projects)

> **Dynamic registry:** Custom servers can be loaded from `~/.oh/mcp/<name>/manifest.json`. Required manifest fields: `name`, `description`, `binary`, `required_tokens`.

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

Start an MCP server via stdio JSON-RPC. This is the command declared in the session bundle at launch.

```bash
oh mcp serve figma
oh mcp serve gitlab
oh mcp serve gslides
oh mcp serve team
oh mcp serve github
oh mcp serve jira
oh mcp serve linear
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
token_key = "openhub.mcp.figma.token"

[mcp.gitlab]
enabled = true
token_key = "openhub.mcp.gitlab.token"
write_enabled = true

[mcp.gslides]
enabled = false
token_key = "openhub.mcp.gslides.token"
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

## Session bundle — `mcp` block

At each launch (`oh run <workflow>`), the CLI puts in the session bundle an `mcp` entry for each **effectively enabled** service (after cascade resolution); the adapter renders it into the opencode config of the session (`oh deploy` removed in v5, nothing is written into the project's `opencode.json`):

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

Only servers with a valid token (env, keychain, or tokenless for `team`) are put in the bundle.

---

## Runtime environment variables

| Service | Variable | Description |
|---------|----------|-------------|
| figma | `FIGMA_TOKEN` | Figma access token |
| gitlab | `GITLAB_TOKEN` | GitLab access token |
| gitlab | `GITLAB_URL` | GitLab instance URL |
| gslides | `GOOGLE_ACCESS_TOKEN` | Google OAuth token |
| github | `GITHUB_TOKEN` | GitHub access token (alias: `GH_TOKEN`) |
| github | `GITHUB_WRITE_ENABLED` | Set `true` to enable write tools |
| jira | `JIRA_URL` | Jira instance URL (e.g. `https://mycompany.atlassian.net`) |
| jira | `JIRA_TOKEN` | Jira API token (or `JIRA_USER` + `JIRA_API_TOKEN`) |
| jira | `JIRA_WRITE_ENABLED` | Set `true` to enable write tools |
| linear | `LINEAR_API_KEY` | Linear API key |
| linear | `LINEAR_WRITE_ENABLED` | Set `true` to enable write tools |

---

## GitLab server

**Required:** `GITLAB_TOKEN` (Personal Access Token with `api` scope)

**Optional:** `GITLAB_URL` for self-hosted instances (default: `https://gitlab.com`)

**Write mode:** Set `GITLAB_WRITE_ENABLED=true` in hub.toml (`write_enabled = true`) to enable write tools.

**Read tools:**

| Tool | Description |
|------|-------------|
| `gitlab_get_project` | Get project metadata |
| `gitlab_list_issues` | List issues with filters |
| `gitlab_list_mrs` | List merge requests |

**Write tools** (requires write mode):

| Tool | Description |
|------|-------------|
| `gitlab_create_mr` | Create a merge request |
| `gitlab_add_mr_note` | Add a note to a merge request |
| `gitlab_update_issue` | Update an issue (labels, assignee, status) |
| `gitlab_assign_reviewer` | Assign a reviewer to an MR |
| `gitlab_add_label` | Add a label to an issue or MR |

---

## GitHub server

**Required:** `GITHUB_TOKEN` or `GH_TOKEN`

**Rate limits:** 60 req/h unauthenticated, 5,000 req/h authenticated.

**Write mode:** Set `GITHUB_WRITE_ENABLED=true` to enable `github_create_issue`.

**Available tools:**

| Tool | Description |
|------|-------------|
| `github_get_repo` | Get repository metadata |
| `github_list_issues` | List issues with filters |
| `github_get_issue` | Get a specific issue |
| `github_list_prs` | List pull requests |
| `github_get_pr` | Get a specific pull request |
| `github_list_workflows` | List GitHub Actions workflows |
| `github_get_workflow_run` | Get runs of a workflow (by filename or ID) |
| `github_create_issue` | Create a new issue *(write mode only)* |

---

## Jira server

**Required:** `JIRA_URL` (e.g. `https://mycompany.atlassian.net`) and `JIRA_TOKEN` (or `JIRA_USER` + `JIRA_API_TOKEN`)

**Supports:** Jira Cloud (API v3) and Jira Server/Data Center.

**Write mode:** Set `JIRA_WRITE_ENABLED=true` to enable write tools.

**Available tools:**

| Tool | Description |
|------|-------------|
| `jira_list_issues` | List issues with JQL filter |
| `jira_get_issue` | Get a specific issue |
| `jira_get_project` | Get project metadata |
| `jira_transition_issue` | Transition issue status *(write mode only)* |
| `jira_create_issue` | Create a new issue *(write mode only)* |

---

## Linear server

**Required:** `LINEAR_API_KEY`

**API:** GraphQL.

**Write mode:** Set `LINEAR_WRITE_ENABLED=true` to enable create/update tools.

**Available tools:**

| Tool | Description |
|------|-------------|
| `linear_list_issues` | List issues with filters |
| `linear_get_issue` | Get a specific issue |
| `linear_create_issue` | Create an issue *(write mode only)* |
| `linear_update_issue` | Update an issue *(write mode only)* |

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
