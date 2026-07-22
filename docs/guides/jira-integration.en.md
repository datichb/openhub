# Jira Integration - Getting Started Guide

> 🇫🇷 [Lire en français](jira-integration.fr.md)

## Overview

The Jira integration connects agents to your Jira project management data — issues, projects, and workflows — supporting both **Jira Cloud** and **Jira Server / Data Center** deployments.

### Features

- **Issue reading**: full description, subtasks, comments, attachments metadata, custom fields
- **Project metadata**: boards, sprints, components, issue type schemes
- **JQL queries**: full Jira Query Language support for flexible issue filtering
- **Issue transitions** (write mode): move issues through workflow states
- **Jira Cloud and Server/Data Center**: unified API with auth differences handled transparently

---

## Prerequisites

### Jira Cloud

- `JIRA_URL` — your Jira Cloud instance URL (e.g. `https://mycompany.atlassian.net`)
- `JIRA_TOKEN` — Personal Access Token (PAT) generated at `https://id.atlassian.com/manage-profile/security/api-tokens`

### Jira Server / Data Center

- `JIRA_URL` — your Jira Server URL (e.g. `https://jira.mycompany.com`)
- `JIRA_USER` — your Jira username
- `JIRA_API_TOKEN` — Personal Access Token generated in your Jira Server profile (Jira 8.14+) or your password for older versions

---

## Setup

### 1. Configure via `oh mcp setup`

```bash
oh mcp setup jira
```

The interactive wizard will:
1. Ask whether you are using Jira Cloud or Server
2. Ask for your `JIRA_URL`
3. Ask for your token (PAT for Cloud, PAT or password for Server)
4. Validate the connection to the Jira API
5. Store credentials securely in the system keychain
6. Update `hub.toml` with the `[mcp.jira]` block

### 2. Manual configuration

Set environment variables before launching `oh`:

```bash
# Jira Cloud
export JIRA_URL=https://mycompany.atlassian.net
export JIRA_TOKEN=your-api-token

# Jira Server / Data Center
export JIRA_URL=https://jira.mycompany.com
export JIRA_USER=myusername
export JIRA_API_TOKEN=your-pat-or-password
```

---

## Configuration in hub.toml

```toml
[mcp.jira]
enabled = true
# Credentials set via env vars (recommended) or keychain
# jira_url = "https://mycompany.atlassian.net"
write_enabled = false  # Set to true to enable issue transitions
```

Deploy after changes:

```bash
oh deploy
```

---

## Available Tools

| Tool | Description | Used by |
|------|-------------|---------|
| `jira_list_issues` | List issues via JQL query with pagination | Planner, Pathfinder |
| `jira_get_issue` | Full issue details (description, subtasks, comments, transitions) | Planner, Pathfinder |
| `jira_get_project` | Project metadata (components, issue types, versions) | Onboarder |
| `jira_transition_issue` | Move an issue to a new workflow state (write mode only) | Planner |

---

## JQL Examples

JQL (Jira Query Language) lets agents filter issues precisely:

```jql
# Issues in progress assigned to the current user
project = MYPROJ AND status = "In Progress" AND assignee = currentUser()

# Unresolved bugs in the current sprint
project = MYPROJ AND issuetype = Bug AND sprint in openSprints() AND resolution = Unresolved

# Issues updated in the last 7 days
project = MYPROJ AND updated >= -7d ORDER BY updated DESC

# Issues blocking others
issueFunction in linkedIssuesOf("project = MYPROJ", "is blocked by")
```

Pass JQL directly in agent prompts:
```
"List all In Progress tickets in project MYPROJ assigned to me"
"Find all bugs updated this week in project FRONTEND"
```

---

## Write Mode

By default the Jira MCP server is read-only. To enable issue transitions:

```bash
export JIRA_WRITE_ENABLED=true
```

Or in `hub.toml`:

```toml
[mcp.jira]
write_enabled = true
```

This unlocks `jira_transition_issue`, allowing the planner agent to advance issues through your Jira workflow (e.g. "To Do" → "In Progress" → "Done").

---

## Usage Examples

### Planner reading a Jira ticket

```
"Plan Jira ticket MYPROJ-42"
"Break down FRONTEND-15 into sub-tickets"
```

The planner reads the issue description, acceptance criteria, and linked issues to decompose the work into Beads tickets while respecting Jira priority and sprint context.

### Dev mode with Jira tickets

```
"Work on MYPROJ-42"
"Implement ticket BACKEND-8"
```

The pathfinder estimates complexity based on the Jira issue's description richness, subtask count, label taxonomy, and sprint deadline proximity.

---

## Jira Cloud vs Server

| Feature | Jira Cloud | Jira Server / Data Center |
|---------|-----------|--------------------------|
| Auth method | API Token (email + token) | PAT (Jira 8.14+) or Basic auth |
| Token URL | `id.atlassian.com/manage-profile/security/api-tokens` | Jira profile > Personal Access Tokens |
| REST API base | `/rest/api/3/` | `/rest/api/2/` |
| Custom fields | Supported | Supported (field naming may differ) |

The MCP server auto-detects the API version from the `JIRA_URL` format.

---

## Troubleshooting

### 401 Unauthorized

Credentials are missing or wrong:
```bash
oh mcp setup jira  # reconfigure
```
For Jira Cloud: ensure you are using the **API token**, not your account password.

### 403 Forbidden

The token lacks permission for that project. Verify:
- The user has at least **Browse Projects** permission in the target project
- For transitions: the user has **Transition Issues** permission

### Missing JIRA_URL

```
Error: JIRA_URL is required
```

Set the env var or reconfigure with `oh mcp setup jira`.

### JQL syntax error

Test your JQL directly in the Jira issue search UI before using it in a prompt, as the MCP server forwards JQL errors verbatim.

---

## Resources

- [Jira REST API documentation](https://developer.atlassian.com/cloud/jira/platform/rest/v3/)
- [JQL reference](https://support.atlassian.com/jira-service-management-cloud/docs/use-advanced-search-with-jira-query-language-jql/)
- [Atlassian API tokens](https://support.atlassian.com/atlassian-account/docs/manage-api-tokens-for-your-atlassian-account/)

---

## Support

- `oh mcp status jira` — check configuration
- `oh mcp setup jira` — reconfigure the service
- Persistent issue → report on [GitHub Issues](https://github.com/anomalyco/opencode)
