# Team Setup Guide

## Prerequisites

- `oh` CLI installed and configured (`oh init` completed)
- A Git repository created on GitLab/GitHub for team-state (empty or with a README)
- All team members have push access to this repository

## 1. Create the team-state repository

One team member creates the repository on GitLab/GitHub:

```bash
# On GitLab/GitHub, create a new repository named "team-state"
# (or any name you prefer)
# Visibility: Internal or Private (accessible to the team)
```

The repository will be populated automatically by `oh team init`.

## 2. Initialize team features

Each team member runs:

```bash
oh team init
```

The wizard asks for:

| Field | Description | Example |
|-------|-------------|---------|
| Repo URL | Git SSH/HTTPS URL of the team-state repo | `git@gitlab.company.com:team/team-state.git` |
| Member ID | Unique identifier (key in members.toml) | `benjamin` |
| Display name | How your name appears in notifications | `Benjamin` |
| GitLab username | For future GitLab integration | `bdatiche` |
| Mattermost username | For mentions in notifications | `benjamin.datiche` |
| Role | Your team role | `lead`, `dev`, or `reviewer` |

This command:
1. Clones the repo to `~/.oh/team-state/`
2. Creates the directory structure (if first member)
3. Adds you to `members.toml`
4. Updates `hub.toml` with `[team]` configuration
5. Pushes changes

## 3. Configure notifications (optional)

Edit `config.toml` in the team-state repo:

```toml
[notification]
mattermost_webhook = "https://mattermost.company.com/hooks/your-webhook-id"
channel = "dev-ai-sessions"
enabled = true
bot_name = "OpenHub"
```

To get the webhook URL: Mattermost > Integrations > Incoming Webhooks > Add.

Commit and push:

```bash
cd ~/.oh/team-state
git add config.toml
git commit -m "config: enable Mattermost notifications"
git push
```

## 4. Deploy to projects

After team init, redeploy to your projects to inject the `team` MCP server:

```bash
oh deploy        # single project
oh sync --all    # all projects
```

This adds the `team` MCP server to `opencode.json`, making team tools available to AI agents.

## Daily Usage

### Claims — Ticket reservation

```bash
# Reserve a ticket before starting work
oh claim SRU-142

# With associated branch
oh claim SRU-142 --worktree feat/SRU-142-user-auth

# Release when done
oh release SRU-142

# Transfer to another member
oh claim transfer SRU-142 --to alice
```

### Team status

```bash
# Who is working on what
oh team status

# Recent activity
oh team activity          # last 24h
oh team activity --today  # today only
oh team activity --week   # last 7 days
oh team activity --member alice  # filter by member
```

### Wiki management

```bash
# Review pending wiki proposals (from AI agents)
oh team wiki review

# List wiki pages
oh team wiki list

# Read a page
oh team wiki read decisions
```

## Team-State Repository Structure

After setup, the repository looks like:

```
team-state/
├── members.toml          # Team registry
├── config.toml           # Notification config
├── projects/
│   └── T-SRU/
│       ├── claims/
│       │   └── SRU-142.toml
│       └── events/
│           └── 2026-07.jsonl
├── wiki/
│   ├── .pending/         # Proposals awaiting review
│   ├── decisions.md      # Architectural decisions
│   └── patterns.md       # Recurring patterns
└── reports/
```

## How AI Agents Use Team Data

When team features are enabled, all agents have access to read-only team tools:

| Tool | What agents see |
|------|----------------|
| `team_members` | Team roster, roles |
| `team_claims` | Who works on what |
| `team_wiki_read` | Shared knowledge |
| `team_events` | Recent activity |

The `documentarian` agent additionally has `team_wiki_write` to propose wiki entries (always pending human review).

## Troubleshooting

### "team-state repo not cloned"

Run `oh team init` to set up team features.

### "sync conflict after retries"

The team-state repo has conflicting changes. Manually resolve:

```bash
cd ~/.oh/team-state
git pull --rebase
# Resolve any conflicts
git push
```

### Notifications not working

1. Verify `config.toml` has `enabled = true`
2. Check the webhook URL is correct
3. Verify the Mattermost channel exists
4. Check `oh team status` works (confirms repo access)
