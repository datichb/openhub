> [Lire en français](mcp-github.fr.md)

# GitHub MCP Server Reference

## Activation

The GitHub MCP server is deployed automatically when `[github].enabled = true` in `hub.toml`.

```json
// Injected into opencode.json by oh deploy
{
  "mcpServers": {
    "github": {
      "command": "oh",
      "args": ["mcp", "serve", "github"]
    }
  }
}
```

## Authentication

Set the `GITHUB_TOKEN` environment variable with a personal access token (PAT) or GitHub App token.

```bash
export GITHUB_TOKEN=ghp_xxxxxxxxxxxxxxxxxxxx
```

Required token scopes:
- `repo` — read access to repositories, issues, pull requests
- `workflow` — read access to GitHub Actions workflows and runs

Write mode requires additional scope: `repo` (write)

## Available Tools

| Tool | Description | Access |
|------|-------------|--------|
| `github_get_repo` | Get repository metadata | All agents |
| `github_list_issues` | List issues with filters | All agents |
| `github_get_issue` | Get a single issue by number | All agents |
| `github_list_prs` | List pull requests with filters | All agents |
| `github_get_pr` | Get PR with reviews and check runs | All agents |
| `github_list_workflows` | List repository workflows | All agents |
| `github_get_workflow_run` | Get a specific workflow run | All agents |

---

### `github_get_repo`

Get metadata for a GitHub repository.

**Required parameters:**
- `repo` (string) — repository in `owner/name` format

**Optional parameters:** none

**Example call:**
```json
{
  "repo": "acme/backend"
}
```

**Example response:**
```json
{
  "id": 123456,
  "full_name": "acme/backend",
  "description": "Main backend service",
  "default_branch": "main",
  "private": true,
  "open_issues_count": 14,
  "stargazers_count": 0,
  "language": "Go",
  "topics": ["backend", "api"],
  "updated_at": "2026-07-20T10:00:00Z"
}
```

---

### `github_list_issues`

List issues for a repository, with optional filters.

**Required parameters:**
- `repo` (string) — repository in `owner/name` format

**Optional parameters:**
- `state` (string) — `open` | `closed` | `all` (default: `open`)
- `labels` (string) — comma-separated label names
- `assignee` (string) — GitHub username, or `none`, or `*`

**Example call:**
```json
{
  "repo": "acme/backend",
  "state": "open",
  "labels": "bug,high-priority",
  "assignee": "jdoe"
}
```

**Example response:**
```json
[
  {
    "number": 42,
    "title": "Nil pointer in auth middleware",
    "state": "open",
    "labels": [{"name": "bug"}, {"name": "high-priority"}],
    "assignees": [{"login": "jdoe"}],
    "created_at": "2026-07-15T08:30:00Z",
    "updated_at": "2026-07-20T11:00:00Z",
    "html_url": "https://github.com/acme/backend/issues/42"
  }
]
```

---

### `github_get_issue`

Get a single issue by its number.

**Required parameters:**
- `repo` (string) — repository in `owner/name` format
- `issue_number` (integer) — issue number

**Optional parameters:** none

**Example call:**
```json
{
  "repo": "acme/backend",
  "issue_number": 42
}
```

**Example response:**
```json
{
  "number": 42,
  "title": "Nil pointer in auth middleware",
  "state": "open",
  "body": "Reproducible with the following curl command...",
  "labels": [{"name": "bug"}],
  "assignees": [{"login": "jdoe"}],
  "comments": 3,
  "created_at": "2026-07-15T08:30:00Z",
  "html_url": "https://github.com/acme/backend/issues/42"
}
```

---

### `github_list_prs`

List pull requests for a repository.

**Required parameters:**
- `repo` (string) — repository in `owner/name` format

**Optional parameters:**
- `state` (string) — `open` | `closed` | `all` (default: `open`)
- `base` (string) — filter by base branch name

**Example call:**
```json
{
  "repo": "acme/backend",
  "state": "open",
  "base": "main"
}
```

**Example response:**
```json
[
  {
    "number": 87,
    "title": "feat: add JWT refresh endpoint",
    "state": "open",
    "base": {"ref": "main"},
    "head": {"ref": "feat/SRU-99-jwt-refresh"},
    "draft": false,
    "created_at": "2026-07-19T14:00:00Z",
    "html_url": "https://github.com/acme/backend/pull/87"
  }
]
```

---

### `github_get_pr`

Get a pull request with its reviews and check run statuses.

**Required parameters:**
- `repo` (string) — repository in `owner/name` format
- `pr_number` (integer) — pull request number

**Optional parameters:** none

**Example call:**
```json
{
  "repo": "acme/backend",
  "pr_number": 87
}
```

**Example response:**
```json
{
  "number": 87,
  "title": "feat: add JWT refresh endpoint",
  "state": "open",
  "mergeable": true,
  "reviews": [
    {
      "user": {"login": "alice"},
      "state": "APPROVED",
      "submitted_at": "2026-07-20T09:00:00Z"
    }
  ],
  "check_runs": [
    {
      "name": "ci/build",
      "status": "completed",
      "conclusion": "success",
      "started_at": "2026-07-20T08:50:00Z",
      "completed_at": "2026-07-20T08:55:00Z"
    }
  ]
}
```

---

### `github_list_workflows`

List all GitHub Actions workflows in a repository.

**Required parameters:**
- `repo` (string) — repository in `owner/name` format

**Optional parameters:** none

**Example call:**
```json
{
  "repo": "acme/backend"
}
```

**Example response:**
```json
[
  {
    "id": 1001,
    "name": "CI",
    "path": ".github/workflows/ci.yml",
    "state": "active"
  },
  {
    "id": 1002,
    "name": "Release",
    "path": ".github/workflows/release.yml",
    "state": "active"
  }
]
```

---

### `github_get_workflow_run`

Get details for a specific workflow run.

**Required parameters:**
- `repo` (string) — repository in `owner/name` format
- `workflow_id` (integer) — workflow run ID (not the workflow definition ID)

**Optional parameters:** none

**Example call:**
```json
{
  "repo": "acme/backend",
  "workflow_id": 9876543
}
```

**Example response:**
```json
{
  "id": 9876543,
  "name": "CI",
  "status": "completed",
  "conclusion": "failure",
  "head_branch": "feat/SRU-99-jwt-refresh",
  "head_sha": "abc123def456",
  "event": "push",
  "run_started_at": "2026-07-20T08:50:00Z",
  "updated_at": "2026-07-20T08:58:00Z",
  "html_url": "https://github.com/acme/backend/actions/runs/9876543",
  "jobs_url": "https://api.github.com/repos/acme/backend/actions/runs/9876543/jobs"
}
```

---

## Write Mode

Write mode is disabled by default. Enable it by setting:

```bash
export GITHUB_WRITE_ENABLED=true
```

### `github_create_issue`

Create a new issue in a repository.

**Required parameters:**
- `repo` (string) — repository in `owner/name` format
- `title` (string) — issue title

**Optional parameters:**
- `body` (string) — issue body (Markdown supported)
- `labels` ([]string) — list of label names to apply

**Access:** Write-enabled agents only

**Example call:**
```json
{
  "repo": "acme/backend",
  "title": "Add rate limiting to /api/v2/auth",
  "body": "Currently the endpoint has no rate limiting. Should be 100 req/min per IP.",
  "labels": ["enhancement", "security"]
}
```

**Example response:**
```json
{
  "number": 91,
  "html_url": "https://github.com/acme/backend/issues/91",
  "state": "open",
  "created_at": "2026-07-22T10:00:00Z"
}
```

## Rate Limits

The GitHub REST API enforces the following rate limits:

| Token Type | Requests/hour |
|------------|---------------|
| Personal Access Token (PAT) | 5,000 |
| GitHub App installation token | 15,000 |
| Unauthenticated | 60 |

The MCP server returns HTTP 429 responses when the limit is exceeded. Check the `X-RateLimit-Reset` header for the reset timestamp.

## See Also

- [MCP Team Server Reference](mcp-team.md)
- [MCP Jira Server Reference](mcp-jira.en.md)
- [MCP Linear Server Reference](mcp-linear.en.md)
- [GitHub REST API documentation](https://docs.github.com/en/rest)
