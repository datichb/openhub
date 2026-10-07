> [Lire en français](services.fr.md)

# CLI Reference — MCP Servers (`oh mcp`)

Manage MCP (Model Context Protocol) servers built into the `oh` binary.

---

## Architecture

MCP servers are **built into the Go binary** — no separate `servers/` directory or Node.js build step. Each server is natively implemented in `cli/internal/mcp/` and served via stdio JSON-RPC.

Available servers:
- **figma** — Figma API integration (files, nodes, styles)
- **gitlab** — GitLab API integration (issues, MRs, discussions, labels, reviewers)
- **gslides** — Google Slides integration
- **team** — Team data (members, wiki, events, claims, policies, patterns, takeover briefs) — no token required; exposes claim lifecycle (5 statuses: `planned`, `in_progress`, `review`, `blocked`, `done`), claim labels (`agent-reviewed`, `needs-human-review`), `ExternalIID` for tracker linkage, and events emitted on claim/release/transfer operations
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

[mcp.jira]
enabled = false
token_key = "openhub.mcp.jira.token"
url = "https://mycompany.atlassian.net"   # optional (GitLab, Jira); empty = team-state URL or default

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
| `url` | string | Instance URL (empty = inherit hub or team) |

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

At each launch (`oh run <workflow>`), oh puts in the session bundle an entry for each **effectively enabled** service (after cascade resolution) whose token is available (environment variable, keychain, or tokenless for `team`). The adapter renders it into the opencode config of the session; nothing is written into the project's `opencode.json` (`oh deploy` removed in v5):

```json
{
  "mcp": [
    { "name": "gitlab", "type": "local",
      "command": ["oh", "mcp", "serve", "gitlab", "--token-key", "openhub.mcp.gitlab.token"],
      "environment": { "GITLAB_WRITE_ENABLED": "true", "GITLAB_URL": "https://gitlab.example.com" } },
    { "name": "team", "type": "local",
      "command": ["oh", "mcp", "serve", "team"],
      "environment": { "OH_TEAM_ID": "acme", "OH_PROJECT_ID": "web-app" } }
  ]
}
```

- **Selection by the workflow**: when the workflow declares `mcp:` ([schema](workflow-schema.en.md#resources)), only the listed servers are kept; absent, the session keeps the project's servers. The `workflow` server (checkpoints, outputs) is always added.
- **`team` server**: it reads the team and project of the session from `OH_TEAM_ID` and `OH_PROJECT_ID` (`.opencode/team.json` is no longer read).
- **Container and remote**: oh MCP servers do not run outside the machine. The `ohd` daemon serves them through the **MCP gateway** (HTTP); tokens stay in the machine keychain ([ADR-046](../architecture/adr/046-beads-gateways.en.md)).
- Inspect: `oh bundle show <workflow>`.

---

## Runtime environment variables

Read by `oh mcp serve <name>`; oh fills them in the bundle from the configuration (tokens come from the keychain, `--token-key`).

| Service | Variable | Description |
|---------|----------|-------------|
| figma | `FIGMA_TOKEN` | Figma access token |
| gitlab | `GITLAB_TOKEN` | GitLab access token |
| gitlab | `GITLAB_URL` | GitLab instance URL (default: `https://gitlab.com`) |
| gitlab | `GITLAB_WRITE_ENABLED` | `true` to enable write tools (set by oh when `write_enabled = true`) |
| gslides | `GOOGLE_ACCESS_TOKEN` | Google OAuth token |
| github | `GITHUB_TOKEN` | GitHub access token (alias: `GH_TOKEN`) |
| github | `GITHUB_WRITE_ENABLED` | `true` to enable write tools |
| jira | `JIRA_URL` | Jira instance URL (e.g. `https://mycompany.atlassian.net`) |
| jira | `JIRA_TOKEN` | Jira API token (or `JIRA_USER` + `JIRA_API_TOKEN`) |
| jira | `JIRA_WRITE_ENABLED` | `true` to enable write tools |
| linear | `LINEAR_API_KEY` | Linear API key |
| linear | `LINEAR_WRITE_ENABLED` | `true` to enable write tools |
| team | `OH_TEAM_ID`, `OH_PROJECT_ID` | Team and project of the session (set by oh) |

---

## GitLab server

**Required:** `GITLAB_TOKEN` (Personal Access Token with `api` scope)

**Optional:** `GITLAB_URL` for self-hosted instances (default: `https://gitlab.com`)

**Write mode:** `write_enabled = true` in `hub.toml` (or for the project); oh then sets `GITLAB_WRITE_ENABLED=true`.

**Read tools:**

| Tool | Description |
|------|-------------|
| `gitlab_get_project` | Project metadata |
| `gitlab_list_issues` | List issues with filters |
| `gitlab_list_mrs` | List merge requests |
| `gitlab_list_mr_discussions` | List the discussions of an MR |
| `gitlab_get_mr_approvals` | Approvals of an MR |

**Write tools** (write mode):

| Tool | Description |
|------|-------------|
| `gitlab_create_mr` | Create a merge request |
| `gitlab_add_mr_note` | Add a note to an MR |
| `gitlab_update_issue` | Update an issue (labels, assignee, status) |
| `gitlab_assign_reviewer` | Assign a reviewer to an MR |
| `gitlab_add_label` | Add a label to an issue or MR |
| `gitlab_reply_to_mr_discussion` | Reply to an MR discussion |

---

## GitHub server

**Required:** `GITHUB_TOKEN` or `GH_TOKEN`

**Rate limits:** 60 req/h unauthenticated, 5,000 req/h authenticated.

**Write mode:** `GITHUB_WRITE_ENABLED=true` enables `github_create_issue`.

**Available tools:**

| Tool | Description |
|------|-------------|
| `github_get_repo` | Repository metadata |
| `github_list_issues` | List issues with filters |
| `github_get_issue` | Get an issue |
| `github_list_prs` | List pull requests |
| `github_get_pr` | Get a pull request |
| `github_list_workflows` | List GitHub Actions workflows |
| `github_get_workflow_run` | Runs of a workflow (by filename or ID) |
| `github_create_issue` | Create an issue *(write mode only)* |

---

## Jira server

**Required:** `JIRA_URL` (e.g. `https://mycompany.atlassian.net`) and `JIRA_TOKEN` (or `JIRA_USER` + `JIRA_API_TOKEN`)

**Supports:** Jira Cloud (API v3) and Jira Server/Data Center.

**Write mode:** `JIRA_WRITE_ENABLED=true` enables the write tools.

**Available tools:**

| Tool | Description |
|------|-------------|
| `jira_list_issues` | List issues with a JQL filter |
| `jira_get_issue` | Get an issue |
| `jira_get_project` | Project metadata |
| `jira_list_comments` | Comments of an issue |
| `jira_transition_issue` | Transition issue status *(write mode only)* |
| `jira_create_issue` | Create an issue *(write mode only)* |

---

## Linear server

**Required:** `LINEAR_API_KEY`

**API:** GraphQL.

**Write mode:** `LINEAR_WRITE_ENABLED=true` enables the create and update tools.

**Available tools:**

| Tool | Description |
|------|-------------|
| `linear_list_issues` | List issues with filters |
| `linear_get_issue` | Get an issue |
| `linear_create_issue` | Create an issue *(write mode only)* |
| `linear_update_issue` | Update an issue *(write mode only)* |

---

## Migrating from `oh service`

The `oh service` commands are **deprecated** (hidden from the help). Use the `oh mcp` equivalents:

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
- [Workflow schema](workflow-schema.en.md) (`mcp` field)
- [Full CLI reference](cli.en.md)
