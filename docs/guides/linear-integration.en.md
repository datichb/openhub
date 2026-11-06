> [Lire en français](linear-integration.fr.md)

# Linear Integration - Getting Started Guide

## Overview

The Linear integration connects agents to your Linear workspace via the **Linear GraphQL API**, enabling modern issue tracking workflows. Linear is designed for fast-moving engineering teams with a clean data model and powerful filtering.

### Features

- **Issue reading**: full description, state, priority, assignee, comments
- **Issue creation** (write mode): create issues with title, description, team, priority
- **Issue updates** (write mode): change state or assignee
- **Flexible filtering**: by team, state, or assignee

---

## Prerequisites

You need a **Linear Personal API Key**:

1. Go to Linear Settings > API > Personal API Keys
2. Click **"Create new API key"**
3. Give it a label (e.g. `openhub`)
4. Copy the generated key (format: `lin_api_xxxxxxxxxxxxxxxxxxxx`)

Keep it in the keychain of `oh` (see below): it is the only route that works in a session (the environment of the agents is filtered, `LINEAR_API_KEY` never gets through, even with `env_allow`).

---

## Setup

> **The shortest way: `oh mcp setup linear`.** The wizard asks for the key (masked field, never displayed), stores it in the keychain as `openhub.mcp.linear.token`, then writes `[mcp.linear]` to `hub.toml` (`enabled = true`, `token_key`, and `write_enabled` according to your answer). With `-p <project>` the key belongs to the project. The steps below do the same by hand, without a terminal (`oh config set` does not know the keys of `[mcp.linear]`).

### 1. Put the key in the keychain

```bash
oh secrets set --global openhub.mcp.linear.token
```

The value is typed at the terminal (masked). `openhub.mcp.linear.token` is the conventional keychain key.

### 2. Declare the server in `hub.toml`

```toml
[mcp.linear]
enabled = true
token_key = "openhub.mcp.linear.token"   # name of the keychain key, NOT the API key
write_enabled = false                    # true: issue creation and updates
```

- `token_key` is **required**: without it (and without a `LINEAR_API_KEY` variable), the server is not placed in the session bundle. It is not silent: a warning is written when the session starts, `oh mcp status linear` and `oh doctor` report it, with the command `oh mcp setup linear`.
- `url` and `trusted_url` have no effect for Linear: the server always calls `https://api.linear.app/graphql`.
- `oh mcp enable linear` and `oh mcp disable linear` change `enabled` (with `-p <project>`, for one project only); the rest is written in the file.

### 3. Start a session

No redeploy needed (`oh deploy` removed in v5): the change is applied at the next session launch, when the session bundle is rebuilt:

```bash
oh run <workflow>
```

`oh mcp list` shows that the `linear` server exists. `oh mcp status` does not show it yet: check by looking for the `linear_*` tools in the session. If the keychain key cannot be found, `oh mcp serve linear` writes a warning and the server answers "`LINEAR_API_KEY` environment variable not set".

---

## Available Tools

| Tool | Description | Used by |
|------|-------------|---------|
| `linear_list_issues` | List issues with filters (team, state, assignee) | Planner, Pathfinder |
| `linear_get_issue` | Full issue details (description, comments) | Planner, Pathfinder |
| `linear_create_issue` | Create a new issue (write mode only) | Planner |
| `linear_update_issue` | Update state or assignee (write mode only) | Planner |

---

## Filtering Examples

Filter issues by team, state, or assignee in agent prompts:

```
"List all In Progress issues in team BACKEND"
"Show me issues assigned to alice in the FRONTEND team"
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
  "state": "Todo",
  "first": 25
}
```

---

## Write Mode

By default the Linear MCP server is read-only. To enable issue creation and updates, in `hub.toml`:

```toml
[mcp.linear]
write_enabled = true
```

oh then passes `LINEAR_WRITE_ENABLED=true` to the server when the next session starts (no need to export it yourself: the environment of the session is filtered). This unlocks `linear_create_issue` and `linear_update_issue`.

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

Regenerate the API key in Linear Settings > API > Personal API Keys, then put the new one in the keychain:
```bash
oh secrets set --global openhub.mcp.linear.token
```

### The `linear_*` tools do not show up

Check in `hub.toml` that `[mcp.linear]` has `enabled = true` **and** `token_key = "openhub.mcp.linear.token"`, and that the key exists in the keychain (`oh secrets get openhub.mcp.linear.token`). Then restart the session.

### Team not found

```
Error: Team "XYZ" not found in your workspace
```

Verify the team key in your Linear workspace settings. Team keys are case-sensitive (e.g. `BACKEND`, not `backend`).

### GraphQL errors

Linear's GraphQL API returns detailed error messages. Common causes:
- Malformed filter parameters (check types: `first` is an integer)
- Trying to use write tools without `write_enabled = true`

---

## Resources

- [Linear API documentation](https://developers.linear.app/docs/graphql/working-with-the-graphql-api)
- [Linear Personal API Keys](https://linear.app/settings/api)
- [`oh mcp` CLI Reference](../reference/cli-mcp.en.md)

---

## Support

- `oh mcp list` — is the `linear` server known?
- `oh mcp enable linear` / `oh mcp disable linear` — turn the service on or off
- `oh mcp setup linear` — configure or reconfigure the service
- `oh mcp status linear` — state, source of the state and origin of the token
- Persistent issue → report on [GitHub Issues](https://github.com/anomalyco/opencode)
