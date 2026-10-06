> [Lire en français](mcp-gitlab.fr.md)

# MCP GitLab Server Reference

Server name: `gitlab-mcp` | Version: `2.0.0`

---

## Activation

```toml
[mcp.gitlab]
enabled = true
env = { GITLAB_TOKEN = "glpat-...", GITLAB_URL = "https://gitlab.example.com" }
```

Changes to `hub.toml` are applied at the next session launch (bundle rebuilt) — no redeploy needed.

---

## Authentication

| Variable | Required | Description |
|----------|----------|-------------|
| `GITLAB_TOKEN` | Yes | GitLab Personal Access Token (header: `PRIVATE-TOKEN`) |
| `GITLAB_URL` | No | GitLab instance URL (default: `https://gitlab.com`) |

> **Security:** Store tokens in the OS keychain via `oh secrets set GITLAB_TOKEN` instead of writing them in `hub.toml`.

---

## URL Security

All API requests are validated before execution:
- Scheme must be `https` (plaintext HTTP is rejected)
- Hostname must NOT resolve to a private IP (127.0.0.1, 10.x, 172.16.x, 192.168.x, 169.254.x)
- DNS resolution is checked for non-IP hostnames

---

## HTTP Client

| Property | Value |
|----------|-------|
| Timeout | 30 seconds |
| Max response size | 50 MB |
| Retry | None |
| Pagination | None |
| Logging | Structured HTTP logging (`mcp.gitlab`), webhook URLs masked |

---

## Read-Only Tools

These tools are always available when the GitLab MCP server is enabled.

### `gitlab_get_project`

Get a GitLab project by ID or path.

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `project_id` | string | Yes | Project ID or URL-encoded path |

**API:** `GET /api/v4/projects/{project_id}`

---

### `gitlab_list_issues`

List issues for a project.

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `project_id` | string | Yes | Project ID |
| `state` | string | No | Filter by state: `opened`, `closed`, `all` |

**API:** `GET /api/v4/projects/{project_id}/issues`

---

### `gitlab_list_mrs`

List merge requests for a project.

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `project_id` | string | Yes | Project ID |
| `state` | string | No | Filter by state: `opened`, `merged`, `closed`, `all` |

**API:** `GET /api/v4/projects/{project_id}/merge_requests`

---

### `gitlab_list_mr_discussions`

List discussion threads on a merge request. Returns inline code comments and general discussions with author, body, resolved status, and file position. System notes are excluded.

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `project_id` | string | Yes | Project ID or URL-encoded path |
| `mr_iid` | integer | Yes | Merge request IID (internal ID) |
| `unresolved_only` | boolean | No | Return only unresolved discussions (default: `true`) |

**API:** `GET /api/v4/projects/{project_id}/merge_requests/{mr_iid}/discussions`

**Behavior:**
1. Fetches all discussions from the API
2. Filters out system-only discussions (keeps only those with human notes)
3. If `unresolved_only` is true (default), keeps only discussions with at least one unresolved note

---

### `gitlab_get_mr_approvals`

Get approval status for a merge request. Returns who approved, how many approvals are required, and how many remain.

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `project_id` | string | Yes | Project ID or URL-encoded path |
| `mr_iid` | integer | Yes | Merge request IID (internal ID) |

**API:** `GET /api/v4/projects/{project_id}/merge_requests/{mr_iid}/approvals`

> **Note:** Requires GitLab Premium or Ultimate. On GitLab Free, returns an explicit message instead of an error.

---

## Write Tools

These tools are only available when `GITLAB_WRITE_ENABLED=true` is set. They are invisible to agents otherwise.

```toml
[mcp.gitlab]
enabled = true
env = { GITLAB_TOKEN = "glpat-...", GITLAB_WRITE_ENABLED = "true" }
```

### `gitlab_create_mr`

Create a merge request. Automatically checks if one already exists for the source branch before creating.

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `project_id` | string | Yes | Project ID or URL-encoded path |
| `source_branch` | string | Yes | Source branch name |
| `target_branch` | string | No | Target branch (default: `main`) |
| `title` | string | Yes | MR title |
| `description` | string | No | MR description (markdown) |

**API:** `POST /api/v4/projects/{project_id}/merge_requests`

**Duplicate detection:** Before creating, checks for existing open MRs on the same source branch. If one exists, returns it instead of creating a duplicate.

---

### `gitlab_add_mr_note`

Add a comment/note to a merge request.

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `project_id` | string | Yes | Project ID |
| `mr_iid` | integer | Yes | MR internal ID |
| `body` | string | Yes | Comment body (markdown) |

**API:** `POST /api/v4/projects/{project_id}/merge_requests/{mr_iid}/notes`

---

### `gitlab_update_issue`

Update an issue (labels, assignees, state).

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `project_id` | string | Yes | Project ID |
| `issue_iid` | integer | Yes | Issue internal ID |
| `state_event` | string | No | State transition: `reopen` or `close` |
| `add_labels` | string | No | Comma-separated labels to add |
| `assignee_ids` | array of integer | No | User IDs to assign |

**API:** `PUT /api/v4/projects/{project_id}/issues/{issue_iid}`

Only non-empty fields are included in the request. You can update any combination in a single call.

---

### `gitlab_assign_reviewer`

Assign reviewer(s) to a merge request.

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `project_id` | string | Yes | Project ID |
| `mr_iid` | integer | Yes | MR internal ID |
| `reviewer_ids` | array of integer | Yes | User IDs to assign as reviewers |

**API:** `PUT /api/v4/projects/{project_id}/merge_requests/{mr_iid}`

---

### `gitlab_add_label`

Add labels to an issue (preserves existing labels).

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `project_id` | string | Yes | Project ID |
| `issue_iid` | integer | Yes | Issue internal ID |
| `labels` | string | Yes | Comma-separated labels to add |

**API:** `PUT /api/v4/projects/{project_id}/issues/{issue_iid}`

---

### `gitlab_reply_to_mr_discussion`

Reply to a specific discussion thread on a merge request. Use this to respond to reviewer comments after applying corrections.

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `project_id` | string | Yes | Project ID or URL-encoded path |
| `mr_iid` | integer | Yes | MR internal ID |
| `discussion_id` | string | Yes | Discussion thread ID (from `gitlab_list_mr_discussions`) |
| `body` | string | Yes | Reply body (markdown) |

**API:** `POST /api/v4/projects/{project_id}/merge_requests/{mr_iid}/discussions/{discussion_id}/notes`

---

## Tool Summary

| # | Tool | Mode | HTTP | Key Parameters |
|---|------|------|------|----------------|
| 1 | `gitlab_get_project` | Read | GET | project_id |
| 2 | `gitlab_list_issues` | Read | GET | project_id, state |
| 3 | `gitlab_list_mrs` | Read | GET | project_id, state |
| 4 | `gitlab_list_mr_discussions` | Read | GET | project_id, mr_iid, unresolved_only |
| 5 | `gitlab_get_mr_approvals` | Read | GET | project_id, mr_iid |
| 6 | `gitlab_create_mr` | Write | POST | project_id, source_branch, title |
| 7 | `gitlab_add_mr_note` | Write | POST | project_id, mr_iid, body |
| 8 | `gitlab_update_issue` | Write | PUT | project_id, issue_iid |
| 9 | `gitlab_assign_reviewer` | Write | PUT | project_id, mr_iid, reviewer_ids |
| 10 | `gitlab_add_label` | Write | PUT | project_id, issue_iid, labels |
| 11 | `gitlab_reply_to_mr_discussion` | Write | POST | project_id, mr_iid, discussion_id, body |

---

## See Also

- [GitLab Integration Guide](../guides/gitlab-integration.en.md)
- [MCP Team Server Reference](mcp-team.en.md)
- [MCP GitHub Server Reference](mcp-github.en.md)
- [Review & Feedback Guide](../guides/review-feedback.en.md)
