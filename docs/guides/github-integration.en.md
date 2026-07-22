# GitHub Integration - Getting Started Guide

> 🇫🇷 [Lire en français](github-integration.fr.md)

## Overview

The GitHub integration gives agents read access to your GitHub repositories — issues, pull requests, and Actions workflows — enabling planning and estimation workflows to use real project data as their source of truth.

### Features

- **Issue reading**: full description, labels, milestone, assignees, comments
- **Pull request reading**: title, branches, state, review status, changed files count
- **Actions workflows**: workflow definitions and run history
- **Issue creation** (write mode): create issues directly from the planner agent
- **Cross-repo queries**: access any repo the token has permission to read

---

## Prerequisites

You need a GitHub Personal Access Token (classic or fine-grained):

- **Classic PAT** — `repo` scope (read access to private repos) or `public_repo` (public only)
- **Fine-grained PAT** — repository permissions: Issues (Read), Pull requests (Read), Actions (Read)

Store the token as `GITHUB_TOKEN` (environment variable or keychain — see Setup below).

---

## Setup

### 1. Configure via `oh mcp setup`

```bash
oh mcp setup github
```

The interactive wizard will:
1. Ask for your GitHub **Personal Access Token**
2. Optionally ask for a **GitHub Enterprise base URL** (leave empty for `github.com`)
3. Validate the connection to the GitHub API
4. Store the token securely in the system keychain
5. Update `hub.toml` with the `[mcp.github]` block

Check status at any time:
```bash
oh mcp status github
```

### 2. Create a Personal Access Token

1. Go to `https://github.com/settings/tokens`
2. Click **"Generate new token (classic)"** or **"Fine-grained tokens"**
3. For classic: select the `repo` scope
4. For fine-grained: select Issues (Read), Pull requests (Read), Actions (Read)
5. Copy the generated token (format: `ghp_xxxxxxxxxxxxxxxxxxxx`)

### 3. Manual configuration (alternative)

Set the environment variable before launching `oh`:

```bash
export GITHUB_TOKEN=ghp_xxxxxxxxxxxxxxxxxxxx
```

Or add it to `hub.toml`:

```toml
[mcp.github]
enabled = true
token = "ghp_xxxxxxxxxxxxxxxxxxxx"
# base_url = "https://github.mycompany.com/api/v3"  # for GitHub Enterprise
```

---

## Configuration in hub.toml

```toml
[mcp.github]
enabled = true
# Token can also be set via GITHUB_TOKEN env var (recommended)
# base_url = "https://github.mycompany.com/api/v3"   # GitHub Enterprise only
write_enabled = false  # Set to true to enable issue creation
```

Deploy to a project after updating hub.toml:

```bash
oh deploy
```

---

## Available Tools

| Tool | Description | Used by |
|------|-------------|---------|
| `github_get_repo` | Repository metadata (description, language, topics, stars) | Onboarder |
| `github_list_issues` | List issues with filters (state, labels, assignee, milestone) | Planner, Pathfinder |
| `github_get_issue` | Full issue details (body, labels, comments, linked PRs) | Planner, Pathfinder |
| `github_list_prs` | List pull requests (state, base/head branch, author) | Pathfinder |
| `github_get_pr` | Full PR details (title, body, reviewers, changed files count) | Pathfinder |
| `github_list_workflows` | List Actions workflow definitions | Onboarder |
| `github_get_workflow_run` | Details of a specific workflow run (status, conclusion, logs URL) | Onboarder |

---

## Write Mode

By default the GitHub MCP server is read-only. To enable issue creation:

```bash
export GITHUB_WRITE_ENABLED=true
```

Or in `hub.toml`:

```toml
[mcp.github]
write_enabled = true
```

This unlocks the `github_create_issue` tool, which the planner agent can use to push decomposed sub-tickets directly to GitHub Issues.

> **Note:** Write mode requires a token with `repo` scope (classic PAT) or Issues (Write) permission (fine-grained PAT).

---

## Agent Skills

Three adapter skills integrate GitHub context into existing agent protocols:

| Skill | Agent | What it does |
|-------|-------|-------------|
| `github-planner-protocol` | Planner | Uses issue body as requirements, labels for priority, milestone for deadline |
| `github-pathfinder-protocol` | Pathfinder | Enriches estimation with issue richness, linked PRs, open questions in comments |
| `github-onboarder-protocol` | Onboarder | Maps repo structure, label taxonomy, CI/CD workflow patterns |

---

## Usage Examples

### Planner with a GitHub Issue

```
"Plan issue #42 from owner/my-repo"
"Break down GitHub issue #42 into sub-tickets"
```

The `github-planner-protocol` skill:
- Reads the full issue description as the requirements document
- Uses acceptance criteria (checkboxes) to pre-fill Beads tickets
- Reads the milestone to calibrate delivery priority
- Detects linked issues as dependencies

### Pathfinder with a Pull Request

```
"Pathfinder PR #15 from owner/my-repo"
"Estimate the complexity of pull request #15"
```

The pathfinder reads the PR title, description, number of changed files, and review comments to produce a complexity estimate and risk flags.

### Onboarder detecting CI workflows

```
"Onboard on owner/my-repo (GitHub)"
```

The `github-onboarder-protocol` skill maps:
- Label taxonomy (type, priority, domain labels)
- Open milestone and sprint delivery dates
- Active GitHub Actions workflows (build, test, deploy)
- Backlog volume and distribution by state

---

## Rate Limits

| Authentication | Rate limit |
|---------------|------------|
| Unauthenticated | 60 requests/hour |
| Authenticated (PAT) | 5,000 requests/hour |
| GitHub Enterprise Cloud | 15,000 requests/hour |

The MCP server handles `429` responses automatically with exponential backoff.

---

## Troubleshooting

### 401 Unauthorized

```
Error: GitHub API returned 401 Unauthorized
```

The token is missing or invalid. Reconfigure:
```bash
oh mcp setup github
```

### 404 Not Found

The repository path is wrong or the token lacks access. Check:
- Format: `owner/repo` (case-sensitive)
- The token has at least read access to that repository

### Rate limit exceeded (403 / 429)

You have exhausted your hourly quota. Solutions:
- Wait for the reset (shown in the `X-RateLimit-Reset` header)
- Use an authenticated token (5,000 req/h instead of 60)
- Cache heavy queries with `oh mcp cache enable github`

### MCP server not starting

```bash
# Verify configuration
oh mcp status github

# Reconfigure
oh mcp setup github

# Run server manually to see raw errors
oh mcp serve github
```

---

## Resources

- [GitHub REST API Documentation](https://docs.github.com/en/rest)
- [Managing Personal Access Tokens](https://docs.github.com/en/authentication/keeping-your-account-and-data-secure/managing-your-personal-access-tokens)
- [`oh mcp` CLI Reference](../reference/mcp.en.md)
- [MCP Protocol](https://modelcontextprotocol.io/)

---

## Support

- `oh mcp status github` — check configuration
- `oh mcp setup github` — reconfigure the service
- Persistent issue → report on [GitHub Issues](https://github.com/anomalyco/opencode)
