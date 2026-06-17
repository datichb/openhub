# GitLab Integration - Getting Started Guide

> 🇫🇷 [Lire en français](gitlab-integration.fr.md)

## Overview

The GitLab integration enriches planning workflows (Orchestrator, Pathfinder, Planner and Onboarder) with project context by automatically querying the GitLab API to read tickets, merge requests, labels and milestones.

### Features

- **Ticket reading**: full description, labels, milestone, human comments
- **MR reading**: title, branches, state, number of changed files
- **Label taxonomy**: automatic understanding of the project's classification system
- **Active milestones**: sprint context and delivery dates
- **Issue search**: filtering by state, labels, keywords

---

## Quick Setup

### 1. Configure via `oc service`

The recommended method is to use the `oc service setup` command which guides you interactively:

```bash
oc service setup gitlab
# or via the alias:
oc gitlab setup
```

This command will:
1. Ask for your GitLab **Personal Access Token**
2. Ask for your **instance URL** (leave empty for gitlab.com)
3. Validate the connection to the GitLab API
4. Save the configuration in `~/.config/opencode/config.json`
5. Automatically build the MCP server if needed

Check status at any time:
```bash
oc service status gitlab
# or:
oc gitlab status
```

### 2. Obtain your Personal Access Token

1. Go to `<your-gitlab>/-/profile/personal_access_tokens`
2. Click **"Add new token"**
3. Choose a name (e.g. `openhub`)
4. Select the required scopes:
   - `api` — full access to issues, MRs, labels, milestones
   - `read_user` — identity validation
5. Set an expiry date
6. Copy the generated token (format: `glpat-xxxxxxxxxxxxxxxxxxxx`)

> For a **self-hosted** instance, replace the URL with your GitLab instance URL during setup.

### 3. Manual configuration (alternative)

Create or edit `~/.config/opencode/config.json`:

```json
{
  "env": {
    "GITLAB_PERSONAL_ACCESS_TOKEN": "glpat-xxxxxxxxxxxxxxxxxxxx",
    "GITLAB_BASE_URL": "https://gitlab.mycompany.com"
  }
}
```

> `GITLAB_BASE_URL` is optional. Leave empty or omit to use `gitlab.com`.

### 4. Deploy to a project

```bash
oc deploy opencode MY-PROJECT
# or only the GitLab MCP:
oc service deploy gitlab --project MY-PROJECT
# or via the alias:
oc gitlab deploy --project MY-PROJECT
```

---

## Usage

### With the Orchestrator

The Orchestrator does not read GitLab tickets directly. When the user provides a ticket ID, it passes it as-is to the `pathfinder` or `planner`, which perform the read in their own session:

```
"Implement ticket #42 from project my-group/my-project"
"Handle issue #42"
"Work on MR !15"
```

The Orchestrator forwards the raw ID (`#42`, `!15`) to the `pathfinder` or `planner` — those agents read the ticket via their own GitLab MCP access and route accordingly.

### With the Pathfinder

The Pathfinder enriches its estimation with GitLab context:

```
"Pathfinder ticket #42"
"Estimate the complexity of issue #42 from project my-group/my-project"
```

The `gitlab-pathfinder-protocol` skill adjusts the estimate based on:
- The richness of the description and acceptance criteria
- Type and priority labels
- Milestone and its due date
- Comments with blockers or open questions

### With the Planner

The Planner uses the ticket as the source of truth for decomposition:

```
"Plan issue #42 from project my-group/my-project"
"Break down ticket #42 into sub-tickets"
```

The `gitlab-planner-protocol` skill leverages:
- The **description** as the requirements document
- **Acceptance criteria** to pre-fill Beads tickets
- The **milestone** to calibrate priority
- **Linked tickets** to detect dependencies

### With the Onboarder

The Onboarder maps the GitLab project during discovery:

```
"Onboard on project my-group/my-project (GitLab)"
```

The `gitlab-onboarder-protocol` skill produces in `ONBOARDING.md`:
- Label taxonomy (types, priorities, domains)
- Delivery cadence (sprints, milestones)
- Backlog state (volume and distribution)

And in `CONVENTIONS.md`:
- Project labelling conventions
- Ticket workflow (triage → in-progress → review → done)

---

## Available MCP Tools

| Tool | Description | Used by |
|---|---|---|
| `get_gitlab_issue` | Reads a full ticket (title, description, labels, milestone, comments) | Pathfinder, Planner |
| `list_gitlab_issues` | Lists tickets with filters (state, labels, search) | Planner, Pathfinder, Onboarder |
| `get_gitlab_merge_request` | Reads an MR (title, branches, state, changes count) | Pathfinder |
| `list_gitlab_labels` | Lists all project labels | Onboarder, Planner |
| `list_gitlab_milestones` | Lists active/closed milestones | Onboarder, Planner |

---

## Architecture

```
servers/gitlab-mcp/
├── src/
│   ├── index.ts              ← MCP entry point (5 tools)
│   ├── config.ts             ← Environment variables
│   ├── client.ts             ← GitLabClient (axios + retry)
│   └── tools/
│       ├── get-issue.ts
│       ├── list-issues.ts
│       ├── get-merge-request.ts
│       ├── list-labels.ts
│       └── list-milestones.ts
├── dist/                     ← Compiled output (gitignored)
└── package.json

skills/adapters/
├── gitlab-planner-protocol.md
├── gitlab-pathfinder-protocol.md
└── gitlab-onboarder-protocol.md
```

---

## Troubleshooting

### Token not recognized

```
Error: GITLAB_PERSONAL_ACCESS_TOKEN is required
```

**Solution:** Check that the token is configured:
```bash
oc gitlab status
```

### Access denied (403)

The token is missing required scopes. Create a new token with `api` and `read_user` scopes.

### Project not found (404)

The project path is incorrect or the token doesn't have access to that project. Check:
- The format: `my-group/my-subgroup/my-project`
- Permissions: the token must have at least the **Reporter** role on the project

### Self-hosted instance unreachable

Check that `GITLAB_BASE_URL` is set:
```bash
oc gitlab status
# If missing:
oc gitlab setup
```

### Request timeouts

Increase the timeout for slow instances:
```bash
# During setup
GITLAB_TIMEOUT=60000 oc gitlab setup
```

### MCP server build fails

```bash
cd servers/gitlab-mcp
npm install
npm run build
```

---

## Current limitations (v1)

- ❌ Read-only — no ticket, comment or MR creation
- ❌ MR diffs not included (code content too large for agent context)
- ❌ Pagination not exposed for `list_gitlab_issues` beyond 100 tickets
- ❌ GitLab webhooks not supported
- ❌ GitLab GraphQL not used (REST only)

---

## Future roadmap

- **v2**: Ticket creation via the planner agent (`create_gitlab_issue`)
- **v3**: MR diff reading for the reviewer agent
- **v4**: Bidirectional links between Beads tickets and GitLab issues

---

## Resources

- [GitLab API Documentation](https://docs.gitlab.com/ee/api/)
- [GitLab Personal Access Tokens](https://docs.gitlab.com/ee/user/profile/personal_access_tokens.html)
- [`oc service` CLI Reference](../reference/services.en.md)
- [MCP Servers Architecture](../../servers/README.md)

---

## Support

- `oc gitlab status` — check configuration
- `oc gitlab setup` — reconfigure the service
- Persistent issue → report on [GitHub Issues](https://github.com/anomalyco/opencode)
