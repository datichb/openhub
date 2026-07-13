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

# With sub-ticket details
oh team status --detail

# Interactive full-screen kanban board
oh team board

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
├── config.toml           # Notification + takeover config
├── policies.toml         # Team rules (configurable enforcement)
├── projects/
│   └── T-SRU/
│       ├── claims/
│       │   └── SRU-142.toml
│       ├── events/
│       │   └── 2026-07.jsonl
│       ├── takeover-briefs/      # Ticket handover briefs
│       │   ├── bd-42_2026-07-13.toml
│       │   └── bd-42_2026-07-13.md
│       └── policies-override.toml  # Per-project overrides (optional)
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
| `team_policies` | Team rules (enforceable conventions) |
| `team_takeover_brief` | Takeover brief for a transferred ticket |

The `documentarian` agent additionally has `team_wiki_write` to propose wiki entries (always pending human review).

## 5. Configure Team Policies

Policies allow automatic enforcement of conventions (blocking or warning).

Create `policies.toml` in the team-state repo:

```bash
# Interactive — add a custom policy
oh policies add

# Or edit directly
cd ~/.oh/team-state
vim policies.toml
git add policies.toml && git commit -m "policies: initial setup" && git push
```

See the [conventions guide](./team-conventions.en.md#team-policies--configurable-enforcement) for the full format and examples.

```bash
# Check policies
oh policies list                  # Show active policies
oh policies check                 # Check current state
oh policies check --branch main   # Check a branch name
```

## 6. Takeover Briefs

When a ticket is transferred from one member to another, a takeover brief is
automatically generated. It contains the context needed to resume work without
information loss.

### Automatic generation

```bash
# Brief is generated automatically on transfer
oh claim transfer SRU-142 --to alice
# → Takeover brief generated. oh takeover-brief show SRU-142
```

If a ticket has been inactive for several days (configurable via `stale_days`
in `config.toml`), the hub detects it as "stale" and proposes generating a
brief when reclaiming:

```bash
oh claim SRU-142
# → SRU-142 is assigned to benjamin for 5 days with no activity.
# → Generate a takeover brief and transfer? [Y/n]
```

### View and enrich briefs

```bash
# Show a ticket's brief
oh takeover-brief show SRU-142

# List all briefs for the project
oh takeover-brief list

# Enrich a brief with AI code analysis
oh takeover-brief enrich SRU-142
```

Enrichment uses an AI agent (`brief-enricher`) in headless mode to:
- Read the files mentioned in the brief
- Identify architectural decisions
- Spot open questions (TODO, FIXME)
- Suggest next steps

### Stale configuration

In `config.toml` of the team-state repo:

```toml
[takeover]
stale_days = 3   # Days of inactivity before a ticket is considered stale
```

## 7. Patterns Library

Patterns are reusable ticket decompositions. They accelerate planning by
offering a proven base for recurring work types (CRUD, API integration,
DB migration, etc.).

### Managing patterns

```bash
# List available patterns
oh patterns list
oh patterns list --tags backend,api

# View a pattern's content
oh patterns show crud-api

# Add a pattern manually
oh patterns add                 # interactive
oh patterns add my-pattern.md   # from a file

# Validate a pattern proposed by an agent
oh patterns validate crud-api

# Remove a pattern
oh patterns remove crud-api
```

### Automatic feeding

The planner and pathfinder can propose patterns automatically:
- After a successful planning (all tickets completed), the planner proposes the decomposition
- Patterns proposed by agents are `validated=false` until human validation

### Team-state structure

```
team-state/
  patterns/
    index.toml          # Pattern catalog (metadata)
    crud-api.md         # Pattern content
    migration-db.md
    ...
```

## 8. Parallel Sessions

Parallel mode allows launching multiple agents simultaneously on different
tickets. Each agent works in an isolated Git worktree.

### Launch

```bash
# Launch 3 tickets in parallel
oh start --parallel --tickets bd-42,bd-43,bd-44

# With a priority ticket (merged first)
oh start --parallel --tickets bd-42,bd-43,bd-44 --priority bd-42

# Limit session count
oh start --parallel --tickets bd-42,bd-43,bd-44 --max-sessions 2
```

### Monitoring interface

A full-screen TUI displays each session's state:
- Real-time status (pending / running / completed / failed)
- Files modified by each session
- Detected potential conflicts

Navigation:
- `j/k`: navigate between sessions
- `Enter`: attach to a session (full opencode TUI)
- `r`: refresh
- `q`: quit

### Merge

After sessions complete, the hub proposes a sequential merge:
- **Beads tickets** (local, `bd-` prefix): merge proposed with human validation
- **External tickets** (GitLab/Jira): no automatic merge, branches stay ready for MR/PR

### Configuration

In `config.toml` of the team-state repo:

```toml
[parallel]
max_sessions = 3           # Max concurrent sessions
port_range_start = 4100    # Starting port for opencode servers
auto_merge_beads = true    # Propose merge for Beads tickets
```

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
