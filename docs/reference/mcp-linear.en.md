> [Lire en français](mcp-linear.fr.md)

# Linear MCP Server Reference

## Activation

The Linear MCP server is deployed automatically when `[linear].enabled = true` in `hub.toml`.

```json
// Injected into opencode.json by oh deploy
{
  "mcpServers": {
    "linear": {
      "command": "oh",
      "args": ["mcp", "serve", "linear"]
    }
  }
}
```

## Authentication

Set the `LINEAR_API_KEY` environment variable with a Linear personal API key.

```bash
export LINEAR_API_KEY=lin_api_xxxxxxxxxxxxxxxxxxxx
```

To generate a key: Linear Settings → API → Personal API keys → Create key

The Linear MCP server communicates with the Linear GraphQL API at:
```
https://api.linear.app/graphql
```

All operations are executed as GraphQL queries and mutations over this endpoint.

## Available Tools

| Tool | Description | Access |
|------|-------------|--------|
| `linear_list_issues` | List issues with GraphQL filters | All agents |
| `linear_get_issue` | Get issue by identifier (e.g. ENG-123) | All agents |

---

### `linear_list_issues`

List Linear issues using GraphQL filters.

**Required parameters:** none (at least one filter recommended)

**Optional parameters:**
- `team_key` (string) — team key (e.g. `ENG`, `OPS`)
- `state` (string) — workflow state name (e.g. `In Progress`, `Todo`, `Done`)
- `assignee` (string) — assignee display name or email
- `first` (integer) — number of results to return (default: 25, max: 100)

**Example call:**
```json
{
  "team_key": "ENG",
  "state": "In Progress",
  "assignee": "John Doe",
  "first": 10
}
```

**Example response:**
```json
{
  "issues": [
    {
      "id": "abc-123-def",
      "identifier": "ENG-142",
      "title": "Implement rate limiting for API gateway",
      "state": {"name": "In Progress", "type": "started"},
      "priority": 2,
      "priorityLabel": "High",
      "assignee": {"name": "John Doe", "email": "jdoe@company.com"},
      "team": {"key": "ENG", "name": "Engineering"},
      "labels": [{"name": "backend"}, {"name": "infrastructure"}],
      "createdAt": "2026-07-15T10:00:00.000Z",
      "updatedAt": "2026-07-20T16:00:00.000Z",
      "url": "https://linear.app/company/issue/ENG-142"
    }
  ],
  "pageInfo": {
    "hasNextPage": false,
    "endCursor": "cursor_xyz"
  }
}
```

**Priority values:**
| Value | Label |
|-------|-------|
| `0` | No priority |
| `1` | Urgent |
| `2` | High |
| `3` | Medium |
| `4` | Low |

---

### `linear_get_issue`

Get a single Linear issue by its identifier.

**Required parameters:**
- `issue_id` (string) — issue identifier in `TEAM-NUMBER` format (e.g. `ENG-123`)

**Optional parameters:** none

**Example call:**
```json
{
  "issue_id": "ENG-142"
}
```

**Example response:**
```json
{
  "id": "abc-123-def",
  "identifier": "ENG-142",
  "title": "Implement rate limiting for API gateway",
  "description": "The API gateway currently has no rate limiting in place...",
  "state": {"name": "In Progress", "type": "started"},
  "priority": 2,
  "priorityLabel": "High",
  "assignee": {"name": "John Doe", "email": "jdoe@company.com"},
  "team": {"id": "team-uuid", "key": "ENG", "name": "Engineering"},
  "labels": [{"name": "backend"}, {"name": "infrastructure"}],
  "parent": null,
  "children": [],
  "comments": [
    {
      "body": "Started working on token bucket implementation",
      "user": {"name": "John Doe"},
      "createdAt": "2026-07-18T09:00:00.000Z"
    }
  ],
  "createdAt": "2026-07-15T10:00:00.000Z",
  "updatedAt": "2026-07-20T16:00:00.000Z",
  "url": "https://linear.app/company/issue/ENG-142"
}
```

---

## Write Mode

Write mode is disabled by default. Enable it by setting:

```bash
export LINEAR_WRITE_ENABLED=true
```

### `linear_create_issue`

Create a new issue in a Linear team.

**Required parameters:**
- `team_id` (string) — team UUID (retrieve via `linear_list_issues` or Linear settings)
- `title` (string) — issue title

**Optional parameters:**
- `description` (string) — issue description (Markdown supported)
- `priority` (integer) — priority level: `0` (none) | `1` (urgent) | `2` (high) | `3` (medium) | `4` (low)

**Access:** Write-enabled agents only

**Example call:**
```json
{
  "team_id": "team-uuid-here",
  "title": "Add circuit breaker for downstream service calls",
  "description": "When downstream services fail, requests pile up...",
  "priority": 2
}
```

**Example response:**
```json
{
  "id": "new-uuid",
  "identifier": "ENG-143",
  "title": "Add circuit breaker for downstream service calls",
  "url": "https://linear.app/company/issue/ENG-143",
  "createdAt": "2026-07-22T10:00:00.000Z"
}
```

---

### `linear_update_issue`

Update an existing issue's state or assignee.

**Required parameters:**
- `issue_id` (string) — issue identifier in `TEAM-NUMBER` format (e.g. `ENG-142`)

**Optional parameters:**
- `state_id` (string) — UUID of the target workflow state
- `assignee_id` (string) — UUID of the user to assign

**Access:** Write-enabled agents only

**Note:** To find state UUIDs, use `linear_list_issues` and inspect the `state.id` field. To find user UUIDs, inspect the `assignee.id` field.

**Example call:**
```json
{
  "issue_id": "ENG-142",
  "state_id": "state-uuid-in-review",
  "assignee_id": "user-uuid-alice"
}
```

**Example response:**
```json
{
  "id": "abc-123-def",
  "identifier": "ENG-142",
  "state": {"name": "In Review", "type": "started"},
  "assignee": {"name": "Alice", "email": "alice@company.com"},
  "updatedAt": "2026-07-22T10:05:00.000Z"
}
```

## Rate Limits

The Linear GraphQL API enforces the following rate limits:

| Plan | Requests/minute | Complexity/minute |
|------|-----------------|-------------------|
| Free | 60 | 50,000 |
| Plus / Pro | 120 | 150,000 |

The MCP server handles `429` responses with automatic retry using exponential backoff (max 3 retries). Complex GraphQL queries (with nested fields) consume more complexity budget.

## See Also

- [MCP Team Server Reference](mcp-team.md)
- [MCP GitHub Server Reference](mcp-github.en.md)
- [MCP Jira Server Reference](mcp-jira.en.md)
- [Linear GraphQL API documentation](https://developers.linear.app/docs/graphql/working-with-the-graphql-api)
