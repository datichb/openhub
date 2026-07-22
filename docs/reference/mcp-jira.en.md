> [Lire en français](mcp-jira.fr.md)

# Jira MCP Server Reference

## Activation

The Jira MCP server is deployed automatically when `[jira].enabled = true` in `hub.toml`.

```json
// Injected into opencode.json by oh deploy
{
  "mcpServers": {
    "jira": {
      "command": "oh",
      "args": ["mcp", "serve", "jira"]
    }
  }
}
```

## Authentication

Two authentication methods are supported:

**Bearer token (recommended):**
```bash
export JIRA_TOKEN=your_personal_access_token
```

**Basic authentication (Atlassian Cloud):**
```bash
export JIRA_USER=user@company.com
export JIRA_API_TOKEN=your_api_token
```

**Required configuration:**
```bash
export JIRA_URL=https://company.atlassian.net
```

The `JIRA_URL` variable is mandatory. It must point to the root of your Jira instance (no trailing slash).

## Available Tools

| Tool | Description | Access |
|------|-------------|--------|
| `jira_list_issues` | JQL-based issue search | All agents |
| `jira_get_issue` | Get issue by key (e.g. PROJ-123) | All agents |
| `jira_get_project` | Get project metadata | All agents |

---

### `jira_list_issues`

Search Jira issues using a JQL query.

**Required parameters:**
- `jql` (string) — JQL query string

**Optional parameters:**
- `max_results` (integer) — maximum number of results to return (default: 50, max: 100)

**Example call:**
```json
{
  "jql": "project = PROJ AND status = 'In Progress' AND assignee = currentUser()",
  "max_results": 20
}
```

**Example response:**
```json
{
  "total": 3,
  "issues": [
    {
      "id": "10042",
      "key": "PROJ-123",
      "summary": "Implement OAuth2 login flow",
      "status": "In Progress",
      "priority": "High",
      "assignee": {"displayName": "John Doe", "emailAddress": "jdoe@company.com"},
      "reporter": {"displayName": "Jane Smith"},
      "labels": ["auth", "backend"],
      "created": "2026-07-10T09:00:00.000+0000",
      "updated": "2026-07-20T14:30:00.000+0000"
    }
  ]
}
```

**Common JQL patterns:**
```
project = PROJ AND sprint in openSprints()
status != Done AND priority = High
labels = "needs-review" ORDER BY updated DESC
assignee = "jdoe" AND created >= -7d
```

---

### `jira_get_issue`

Get a single Jira issue by its key.

**Required parameters:**
- `issue_key` (string) — issue key in `PROJECT-NUMBER` format (e.g. `PROJ-123`)

**Optional parameters:** none

**Example call:**
```json
{
  "issue_key": "PROJ-123"
}
```

**Example response:**
```json
{
  "id": "10042",
  "key": "PROJ-123",
  "summary": "Implement OAuth2 login flow",
  "description": "We need to implement the full OAuth2 authorization code flow...",
  "status": "In Progress",
  "priority": "High",
  "issuetype": "Story",
  "assignee": {"displayName": "John Doe", "emailAddress": "jdoe@company.com"},
  "reporter": {"displayName": "Jane Smith"},
  "labels": ["auth", "backend"],
  "components": [{"name": "Authentication"}],
  "sprint": {"name": "Sprint 14", "state": "active"},
  "story_points": 5,
  "created": "2026-07-10T09:00:00.000+0000",
  "updated": "2026-07-20T14:30:00.000+0000",
  "comments": [
    {
      "author": {"displayName": "Alice"},
      "body": "PR is ready for review",
      "created": "2026-07-20T12:00:00.000+0000"
    }
  ]
}
```

---

### `jira_get_project`

Get metadata for a Jira project.

**Required parameters:**
- `project_key` (string) — project key (e.g. `PROJ`)

**Optional parameters:** none

**Example call:**
```json
{
  "project_key": "PROJ"
}
```

**Example response:**
```json
{
  "id": "10001",
  "key": "PROJ",
  "name": "Main Product",
  "description": "Main product development project",
  "projectTypeKey": "software",
  "lead": {"displayName": "Alice Manager", "emailAddress": "alice@company.com"},
  "issueTypes": [
    {"name": "Epic", "subtask": false},
    {"name": "Story", "subtask": false},
    {"name": "Bug", "subtask": false},
    {"name": "Task", "subtask": false},
    {"name": "Sub-task", "subtask": true}
  ],
  "statuses": ["Backlog", "To Do", "In Progress", "In Review", "Done"]
}
```

---

## Write Mode

Write mode is disabled by default. Enable it by setting:

```bash
export JIRA_WRITE_ENABLED=true
```

### `jira_transition_issue`

Transition an issue to a new status using a transition ID.

**Required parameters:**
- `issue_key` (string) — issue key in `PROJECT-NUMBER` format
- `transition_id` (string) — ID of the transition to apply

**Access:** Write-enabled agents only

**Note:** To find available transition IDs for an issue, use `jira_get_issue` and inspect the `transitions` field, or consult your Jira project administrator.

**Example call:**
```json
{
  "issue_key": "PROJ-123",
  "transition_id": "31"
}
```

**Example response:**
```json
{
  "success": true,
  "issue_key": "PROJ-123",
  "new_status": "In Review"
}
```

**Common transition IDs** (vary by project workflow):
| Transition | Typical ID |
|------------|------------|
| Start Progress | `11` |
| Send to Review | `31` |
| Done | `41` |
| Reopen | `51` |

## Rate Limits

Jira Cloud enforces API rate limits per user per instance:

- **Standard tier**: 100 requests/10 seconds
- **Premium tier**: 1,000 requests/10 seconds

The MCP server handles 429 responses with automatic retry using exponential backoff (max 3 retries).

## See Also

- [MCP Team Server Reference](mcp-team.md)
- [MCP GitHub Server Reference](mcp-github.en.md)
- [MCP Linear Server Reference](mcp-linear.en.md)
- [Jira REST API documentation](https://developer.atlassian.com/cloud/jira/platform/rest/v3/)
