> [Lire en francais](gitlab-integration.fr.md)

# GitLab Integration — Getting Started Guide

## Overview

The GitLab integration connects agents to your GitLab projects — issues, merge requests, discussions, and approvals — supporting both **GitLab.com** and **self-hosted** instances. It provides **read** capabilities for planning and onboarding workflows, and optional **write** capabilities for development feedback and code review.

### Features

**Read capabilities:**

- **Project metadata**: project info, default branch, visibility, namespace
- **Issue listing**: filter by state, labels, search keywords
- **Merge request listing**: open/merged/closed MRs with branch and change info
- **MR discussions**: threaded review comments and conversations
- **MR approvals**: approval state, required approvers, approval rules

**Write capabilities** (requires `GITLAB_WRITE_ENABLED=true`):

- **Create merge requests**: open MRs from a source to a target branch
- **Add MR notes**: post comments on merge requests
- **Update issues**: change state, labels, assignees, milestone
- **Assign reviewers**: set reviewers on merge requests
- **Add labels**: apply labels to issues or MRs
- **Reply to MR discussions**: respond to existing review threads

---

## Prerequisites

1. A GitLab account with access to the target project(s)
2. A **Personal Access Token** (PAT) with the `api` scope:
   - Go to `<your-gitlab>/-/profile/personal_access_tokens`
   - Click **"Add new token"**
   - Name it (e.g. `openhub`)
   - Select the `api` scope — this covers issues, MRs, labels, milestones, and discussions
   - Set an expiry date
   - Copy the generated token (format: `glpat-xxxxxxxxxxxxxxxxxxxx`)
3. Set the `GITLAB_TOKEN` environment variable to your PAT
4. (Optional) For self-hosted instances, set `GITLAB_URL` to your instance URL

---

## Setup

### 1. Configure via `oh mcp setup`

```bash
oh mcp setup gitlab
```

The interactive wizard will:
1. Ask for your GitLab **Personal Access Token**
2. Ask for your **instance URL** (leave empty for gitlab.com)
3. Validate the connection to the GitLab API
4. Store credentials securely in the system keychain
5. Update `hub.toml` with the `[mcp.gitlab]` block

### 2. Manual configuration

Set environment variables before launching `oh`:

```bash
export GITLAB_TOKEN=glpat-xxxxxxxxxxxxxxxxxxxx

# Self-hosted only:
export GITLAB_URL=https://gitlab.mycompany.com

# To enable write tools:
export GITLAB_WRITE_ENABLED=true
```

---

## Configuration in hub.toml

```toml
[mcp.gitlab]
enabled = true
# Credentials set via env vars (recommended) or keychain
# gitlab_url = "https://gitlab.mycompany.com"  # omit for gitlab.com
write_enabled = false  # Set to true to enable write tools
```

No redeploy needed (`oh deploy` removed in v5): the change is applied at the next session launch, when the session bundle is rebuilt:

```bash
oh run <workflow>
```

---

## Available Tools

| Tool | Description | Used by |
|------|-------------|---------|
| `gitlab_get_project` | Project metadata (name, default branch, visibility, namespace) | Onboarder |
| `gitlab_list_issues` | List issues with filters (state, labels, search) | Planner, Pathfinder, Onboarder |
| `gitlab_list_mrs` | List merge requests with filters (state, labels, source/target branch) | Planner, Pathfinder, Onboarder |
| `gitlab_list_mr_discussions` | List threaded discussions on a merge request | Pathfinder |
| `gitlab_get_mr_approvals` | Approval state, required approvers, and approval rules for a MR | Pathfinder |
| `gitlab_create_mr` | Create a merge request (write mode) | Orchestrator-dev (feedback mode) |
| `gitlab_add_mr_note` | Post a comment on a merge request (write mode) | Orchestrator-dev (feedback mode) |
| `gitlab_update_issue` | Update issue state, labels, assignees, milestone (write mode) | Orchestrator-dev (feedback mode) |
| `gitlab_assign_reviewer` | Set reviewers on a merge request (write mode) | Review system |
| `gitlab_add_label` | Apply labels to issues or merge requests (write mode) | Review system |
| `gitlab_reply_to_mr_discussion` | Reply to an existing MR discussion thread (write mode) | Review system |

---

## Write Mode

By default the GitLab MCP server is **read-only**. To enable write tools:

```bash
export GITLAB_WRITE_ENABLED=true
```

Or in `hub.toml`:

```toml
[mcp.gitlab]
write_enabled = true
```

This unlocks the 6 write tools: `gitlab_create_mr`, `gitlab_add_mr_note`, `gitlab_update_issue`, `gitlab_assign_reviewer`, `gitlab_add_label`, and `gitlab_reply_to_mr_discussion`.

Write mode is used by:
- **Orchestrator-dev** in feedback mode — creates MRs, posts review notes, updates issue state
- **Review system** — assigns reviewers, applies labels, replies to discussion threads

---

## Usage Examples

### Listing issues for planning

```
"List open issues in project my-group/my-project"
"Show me bugs labelled priority::high in my-group/my-project"
```

The planner reads issue descriptions, labels, and milestones to decompose work into Beads tickets.

### Creating a merge request

```
"Create a MR from feature/auth to main in my-group/my-project"
"Open a merge request for my changes"
```

Requires write mode. The orchestrator-dev creates the MR and optionally assigns reviewers.

### Reviewing feedback

```
"Post review feedback on MR !42 in my-group/my-project"
"Reply to the discussion about error handling on MR !42"
```

Requires write mode. The review system adds notes and replies to existing discussion threads.

---

## Self-Hosted GitLab

For self-hosted GitLab instances, set `GITLAB_URL`:

```bash
export GITLAB_URL=https://gitlab.mycompany.com
```

Requirements:
- The URL **must use HTTPS** — HTTP connections are rejected
- The URL must point to the GitLab instance root (e.g. `https://gitlab.mycompany.com`, not `https://gitlab.mycompany.com/api/v4`)
- The GitLab instance must be running version **13.0+** (REST API v4)

If `GITLAB_URL` is not set, the server defaults to `https://gitlab.com`.

---

## Troubleshooting

### 401 Unauthorized

The token is invalid or expired:
```bash
oh mcp setup gitlab  # reconfigure
```
Make sure you are using a **Personal Access Token** (starts with `glpat-`), not an OAuth token or deploy token.

### 403 Forbidden

The token lacks the required scope or project permissions:
- Verify the token has the `api` scope
- Verify the token owner has at least **Reporter** role on the target project
- For write operations: the token owner needs **Developer** role or higher

### 404 Project Not Found

The project path is incorrect or the token does not have access:
- Check the format: `my-group/my-subgroup/my-project`
- Verify the token can access the project (try `curl -H "PRIVATE-TOKEN: $GITLAB_TOKEN" "$GITLAB_URL/api/v4/projects/my-group%2Fmy-project"`)

### Self-hosted URL issues

```
Error: GITLAB_URL must use HTTPS
```

Ensure `GITLAB_URL` starts with `https://`. HTTP is not supported.

```
Error: cannot reach GitLab API
```

Verify the URL is correct and the instance is reachable from your machine. Check for VPN or firewall requirements.

### Approvals unavailable on Free tier

The `gitlab_get_mr_approvals` tool requires GitLab **Premium** or **Ultimate**. On GitLab Free (including gitlab.com Free), the API returns empty approval data. The tool will still work but will return no approval rules.

---

## Current Limitations

- No automatic pagination — list tools return up to 100 results per call
- No retry on HTTP 429 (rate limiting) — back off manually if hitting limits
- No dedicated tools for labels or milestones listing — use `gitlab_list_issues` filters or the project metadata

---

## Resources

- [GitLab MCP Reference](../reference/mcp-gitlab.en.md)
- [GitLab REST API Documentation](https://docs.gitlab.com/ee/api/)
- [GitLab Personal Access Tokens](https://docs.gitlab.com/ee/user/profile/personal_access_tokens.html)
- [Jira Integration Guide](jira-integration.en.md)
- [GitHub Integration Guide](github-integration.en.md)

---

## Support

- `oh mcp status gitlab` — check configuration and connection status
- `oh mcp setup gitlab` — reconfigure the service interactively
- Persistent issue → report on [GitHub Issues](https://github.com/anomalyco/opencode)
