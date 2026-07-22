# Linear Integration - Getting Started Guide

> 🇫🇷 [Lire en français](linear-integration.fr.md)

## Overview

The Linear integration connects agents to your Linear workspace via the **Linear GraphQL API**, enabling modern issue tracking workflows. Linear is designed for fast-moving engineering teams with a clean data model and powerful filtering.

### Features

- **Issue reading**: full description, state, priority, labels, assignee, cycle, project
- **Issue creation** (write mode): create issues with title, description, team, state, priority
- **Issue updates** (write mode): change state, assignee, priority, or add comments
- **Flexible filtering**: by team, state, assignee, label, priority, or cycle

---

## Prerequisites

You need a **Linear Personal API Key**:

1. Go to Linear Settings > API > Personal API Keys
2. Click **"Create new API key"**
3. Give it a label (e.g. `openhub`)
4. Copy the generated key (format: `lin_api_xxxxxxxxxxxxxxxxxxxx`)

Store it as `LINEAR_API_KEY`.

---

## Setup

### 1. Configure via `oh mcp setup`

```bash
oh mcp setup linear
```

The wizard will:
1. Ask for your **Linear Personal API Key**
2. Validate the connection to the Linear GraphQL API
3. Store the key securely in the system keychain
4. Update `hub.toml` with the `[mcp.linear]` block

Check status:
```bash
oh mcp status linear
```

### 2. Manual configuration (alternative)

```bash
export LINEAR_API_KEY=lin_api_xxxxxxxxxxxxxxxxxxxx
```

Or in `hub.toml`:

```toml
[mcp.linear]
enabled = true
# api_key can also be set via LINEAR_API_KEY env var (recommended)
write_enabled = false
```

---

## Configuration in hub.toml

```toml
[mcp.linear]
enabled = true
write_enabled = false  # Set to true to enable issue creation and updates
```

Deploy after changes:

```bash
oh deploy
```

---

## Available Tools

| Tool | Description | Used by |
|------|-------------|---------|
| `linear_list_issues` | List issues with filters (team, state, assignee, label, priority) | Planner, Pathfinder |
| `linear_get_issue` | Full issue details (description, comments, sub-issues, relations) | Planner, Pathfinder |
| `linear_create_issue` | Create a new issue (write mode only) | Planner |
| `linear_update_issue` | Update state, assignee, priority, or add comment (write mode only) | Planner |

---

## Filtering Examples

Filter issues by team, state, or assignee in agent prompts:

```
"List all In Progress issues in team BACKEND"
"Show me high priority issues assigned to alice in the FRONTEND team"
"Find all issues in the current cycle for team PLATFORM"
```

Or with explicit filter parameters passed through the MCP tool:

```json
{
  "team_key": "BACKEND",
  "state": "In Progress",
  "assignee": "alice@company.com"
}
```

```json
{
  "team_key": "FRONTEND",
  "priority": 1,
  "label": "bug"
}
```

---

## Write Mode

By default the Linear MCP server is read-only. To enable issue creation and updates:

```bash
export LINEAR_WRITE_ENABLED=true
```

Or in `hub.toml`:

```toml
[mcp.linear]
write_enabled = true
```

This unlocks `linear_create_issue` and `linear_update_issue`.

> **Note:** The API key must belong to a workspace member with permission to create/edit issues in the target team.

---

## Usage Examples

### Listing in-progress tickets

```
"What tickets is the BACKEND team currently working on?"
"List all In Progress issues in team PLATFORM assigned to me"
```

The agent calls `linear_list_issues` with `team_key=BACKEND, state=In Progress` and presents a summary table with issue IDs, titles, assignees, and priorities.

### Updating issue state

With write mode enabled:

```
"Mark LINEAR-42 as Done"
"Move FRONTEND-15 to In Review"
```

The planner calls `linear_update_issue` to transition the issue state, providing a comment with a summary of the work done.

---

## Troubleshooting

### Invalid API key

```
Error: Authentication failed — invalid Linear API key
```

Regenerate the API key in Linear Settings > API > Personal API Keys, then reconfigure:
```bash
oh mcp setup linear
```

### Team not found

```
Error: Team "XYZ" not found in your workspace
```

Use `linear_list_teams` (available via `oh mcp tools linear`) to see all team keys in your workspace. Team keys are case-sensitive (e.g. `BACKEND`, not `backend`).

### GraphQL errors

Linear's GraphQL API returns detailed error messages. Common causes:
- Malformed filter parameters (check types: priority is an integer 0–4)
- Trying to use write tools without `write_enabled = true`
- Cycle/project IDs not matching the target team

---

## Resources

- [Linear API documentation](https://developers.linear.app/docs/graphql/working-with-the-graphql-api)
- [Linear Personal API Keys](https://linear.app/settings/api)
- [`oh mcp` CLI Reference](../reference/mcp.en.md)

---

## Support

- `oh mcp status linear` — check configuration
- `oh mcp setup linear` — reconfigure the service
- Persistent issue → report on [GitHub Issues](https://github.com/anomalyco/opencode)
