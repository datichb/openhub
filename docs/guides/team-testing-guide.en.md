> [Lire en francais](team-testing-guide.fr.md)

# Exhaustive Testing Guide — Team Feature (`oh`)

> Complete guide for thoroughly testing the Team feature of OpenHub.
> CLI / TUI comparison for each action, architecture diagrams, state machines, and end-to-end scenarios.

---

## Table of Contents

- [Global Architecture](#global-architecture)
- [Prerequisites](#prerequisites)
- [1. Team Initialization](#1-team-initialization)
- [2. Configuration Resolution](#2-configuration-resolution)
- [3. Multi-team Management](#3-multi-team-management)
- [4. Per-project Configuration](#4-per-project-configuration)
- [5. Claim Lifecycle](#5-claim-lifecycle)
- [6. Kanban Board](#6-kanban-board)
- [7. Team Status](#7-team-status)
- [8. Activity](#8-activity)
- [9. Policies](#9-policies--verification-and-enforcement)
- [10. Detailed Configuration (Team Detail)](#10-detailed-configuration-team-detail)
- [11. Tracker Synchronization](#11-tracker-synchronization-gitlab--jira)
- [12. Takeover Briefs](#12-takeover-briefs)
- [13. Team Wiki](#13-team-wiki)
- [14. Pattern Library](#14-pattern-library)
- [15. Parallel Sessions](#15-parallel-sessions)
- [16. Notifications](#16-notifications)
- [17. MCP Integration (AI Agents)](#17-mcp-integration-ai-agents)
- [18. Credentials and Security](#18-credentials--security)
- [19. Git Concurrency](#19-git-concurrency-withwritelock)
- [20. End-to-end Scenarios](#20-end-to-end-scenarios)

---

## Global Architecture

```
┌─────────────────────────────────────────────────────────────────────┐
│                           USER                                      │
├──────────────────┬─────────────────────┬────────────────────────────┤
│   CLI (oh ...)   │    TUI (oh --tui)   │    AI Agents (MCP)         │
├──────────────────┴─────────────────────┴────────────────────────────┤
│                         CLI Layer (cobra)                            │
│  team.go │ teams.go │ team_board.go │ team_config.go │ team_sync_*  │
├─────────────────────────────────────────────────────────────────────┤
│                    TUI Layer (tview/tcell/huh)                       │
│  teams_view │ teamstatus_view │ teamboard_view │ team_detail_view   │
├─────────────────────────────────────────────────────────────────────┤
│                      Internal Packages                               │
│  teamstate/  │  config/  │  notify/  │  tracker/  │  mcp/team/      │
├─────────────────────────────────────────────────────────────────────┤
│                      Team-State Git Repo                             │
│  members.toml │ config.toml │ policies.toml │ projects/ │ wiki/     │
├─────────────────────────────────────────────────────────────────────┤
│              External Services                                       │
│  GitLab/Jira │ Mattermost/Slack/Discord/Teams │ OS Keychain         │
└─────────────────────────────────────────────────────────────────────┘
```

### Team-state repo structure

```
team-state/
├── config.toml                          # Shared configuration
├── members.toml                         # Members registry
├── policies.toml                        # Team rules
├── projects/
│   └── <project>/
│       ├── claims/
│       │   └── <ticketID>.toml          # Ticket reservation
│       ├── events/
│       │   └── YYYY-MM.jsonl            # Event journal
│       ├── policies-override.toml       # Project overrides
│       └── takeover-briefs/
│           ├── <ticket>_<date>.toml     # Structured brief
│           ├── <ticket>_<date>.md       # Human-readable brief
│           └── <ticket>_<date>.enriched.md  # AI-enriched brief
├── wiki/
│   ├── .pending/                        # Pending proposals
│   ├── decisions.md
│   └── patterns.md
├── patterns/
│   ├── index.toml                       # Catalog
│   └── <pattern-name>.md               # Pattern content
└── reports/
```

---

## Prerequisites

```
┌──────────────────────────────────────────────────────────┐
│  PREREQUISITES                                            │
├──────────────────────────────────────────────────────────┤
│  1. oh installed            →  oh version                 │
│  2. oh init completed       →  hub.toml exists            │
│  3. Git team-state repo     →  empty or with README       │
│     (GitLab/GitHub)            Visibility: Internal/Private│
│  4. Push access             →  SSH key or token           │
│  5. ≥ 1 project configured →  oh project list             │
│  6. (Optional) Tracker      →  GitLab/Jira + API token   │
│  7. (Optional) Webhook      →  Mattermost/Slack/etc. URL │
└──────────────────────────────────────────────────────────┘
```

| # | Element | Verification command | Expected result |
|---|---------|----------------------|-----------------|
| 1 | `oh` CLI installed | `oh version` | Version displayed |
| 2 | Hub configured | `ls ~/.oh/hub.toml` | File exists |
| 3 | Git repo created | Check on GitLab/GitHub | Repo visible |
| 4 | Push access | `git ls-remote <url>` | No auth error |
| 5 | Existing project | `oh project list` | ≥ 1 project listed |
| 6 | Tracker token | `echo $GITLAB_TOKEN` | Non-empty (optional) |
| 7 | Webhook | curl test to URL | HTTP 200 (optional) |

---

## 1. Team Initialization

### Initialization flow

```
                          oh team init
                               │
                    ┌──────────┴───────────┐
                    │  CLI (tview wizard)   │
                    │  5-step sidebar       │
                    └──────────┬───────────┘
                               │
                    ┌──────────┴───────────┐
                    │  teamInitCore()       │
                    └──────────┬───────────┘
                               │
              ┌────────────────┼────────────────┐
              ▼                ▼                 ▼
    repo.EnsureReady()   repo.InitStructure()  AddMember()
    (clone or pull)      (dirs + .gitkeep)     (members.toml)
              │                │                 │
              └────────────────┼────────────────┘
                               ▼
                    config.Save() → hub.toml
                    [[teams]] updated
```

### CLI / TUI comparison

| Aspect | CLI (`oh team init`) | TUI (Teams view → `a` / omnibar `team init`) |
|--------|---------------------|----------------------------------------------|
| **Format** | Full-screen tview wizard, 5-step sidebar | Step-by-step modal, 3-4 steps |
| **Step 1** | Team-state repo URL | Team-state repo URL |
| **Step 2** | Global config (stale_days) | HTTPS auth (if needed) |
| **Step 3** | Full identity (5 fields) | Identity (member_id, display_name, role) |
| **Step 4** | Notifications (webhook, channel, bot) | — (configurable later via detail) |
| **Step 5** | Policies (checkboxes) | — (configurable later) |
| **Adaptive** | If repo not empty → proposes "Modify?" | Always the same flow |
| **Result** | Commit + push + hub.toml | Same via `teamInitCore()` |

### Identity fields

| Field | Required | Description |
|-------|----------|-------------|
| `member_id` | Yes | Unique key in members.toml |
| `display_name` | No | Name displayed in notifications |
| `gitlab_username` | No | For tracker mapping |
| `mattermost_username` | No | For mentions |
| `role` | Yes | `lead` / `dev` / `reviewer` |
| `default_mode` | No | `manual` / `semi-auto` / `auto` (default: semi-auto) |

### Test points

| # | Scenario | Input | Expected result |
|---|----------|-------|-----------------|
| 1 | First member (empty repo) | Empty repo URL + identity | `config.toml`, `members.toml`, structure created, commit pushed |
| 2 | Next member (existing repo) | Same URL + new ID | `members.toml` enriched, config not overwritten |
| 3 | HTTPS without token | https:// URL | TUI: auth step appears / CLI: 30s timeout, PullWarning |
| 4 | Valid SSH key | git@... URL | Direct clone without auth step |
| 5 | Inaccessible repo | Invalid URL | Explicit error, no crash |
| 6 | member_id already exists | ID already in members.toml | ErrMemberExists, proposes UpdateMember |
| 7 | hub.toml already has team | Relaunch init | Detects existing config, proposes modification |

---

## 2. Configuration Resolution

### Resolution algorithm

```
                    ┌─────────────────┐
                    │  Project requests│
                    │  team config    │
                    └────────┬────────┘
                             │
                   ┌─────────┴──────────┐
                   │  project.TeamID    │
                   │  defined?          │
                   └─────────┬──────────┘
                     yes │         │ no
                         ▼         ▼
              ┌──────────────┐  ┌────────────────────┐
              │ Lookup in    │  │ project.TeamConfig  │
              │ hub.Teams[]  │  │ (legacy mode)       │
              │ by ID        │  └─────────┬──────────┘
              └──────┬───────┘            │
                     │           ┌────────┼─────────┐
                     │           │        │         │
                     │      inherit   custom    disabled
                     │           │        │         │
                     ▼           ▼        ▼         ▼
              ┌──────────┐  hub team   custom    Enabled:
              │ ID found │  config     repo     false
              │ + enabled│  direct    + merge
              └──────────┘
```

### Fallback rules

```
┌────────────────────────────────────────────────────────────┐
│  Fallback MemberID:                                        │
│  project.MemberID || hub.Teams[x].MemberID                 │
│                                                            │
│  Fallback StatePath (auto-derived):                        │
│  ~/.oh/team-states/<host>/<repo-name>                      │
│                                                            │
│  Backward compat:                                          │
│  If ~/.oh/team-states/<repo-name> exists (legacy)          │
│  AND ~/.oh/team-states/<host>/<repo-name> does not exist   │
│  → use the legacy path                                     │
└────────────────────────────────────────────────────────────┘
```

### Test points

| # | Scenario | Expected result |
|---|----------|-----------------|
| 1 | Inherit mode (default) | `project.TeamConfig = nil` → uses hub `ActiveTeam()` |
| 2 | Custom mode, MemberID empty | Fallback to `hub.Teams[0].MemberID` |
| 3 | Disabled mode | `ResolvedTeamConfig.Enabled = false` |
| 4 | TeamID pointing to archived team | `Enabled: false` (graceful) |
| 5 | Unknown TeamID | `Enabled: false` (no panic) |
| 6 | Auto-derived StatePath | URL `git@gitlab.com:acme/ts.git` → `~/.oh/team-states/gitlab.com/ts` |
| 7 | Legacy path exists | Uses the old path if the new one does not exist |

---

## 3. Multi-team Management

### State diagram

```
                    hub.toml
                    ┌────────────────────────────────┐
                    │  [[teams]]                     │
                    │  ├── id = "acme"               │
                    │  │   enabled = true            │──► Project A (TeamID="acme")
                    │  │   state_repo = ...          │──► Project B (TeamID="acme")
                    │  │   member_id = "bd"          │
                    │  │                             │
                    │  ├── id = "beta"               │
                    │  │   enabled = false           │──► (archived, ignored)
                    │  │   state_repo = ...          │
                    │  │                             │
                    │  └── id = "gamma"              │
                    │      enabled = true            │──► Project C (TeamID="gamma")
                    │      state_repo = ...          │
                    └────────────────────────────────┘
```

### CLI / TUI comparison

| Action | CLI | TUI |
|--------|-----|-----|
| **List** | `oh teams list` → table (ID, name, member, repo truncated 40c, status) | `teams` view: interactive list, ✓ green = active |
| **Add** | `oh teams add --repo <url> --member-id <id> [--id x] [--name y]` | Key `a` → 4-modal wizard (URL, member-id, short ID, name) |
| **Remove** | `oh teams remove <id> [-f]` (confirmation unless `-f`) | Key `d` → confirmation modal |
| **Detach project** | `oh teams detach <project-name>` | — |
| **Archive** | `oh teams archive <id>` (sets `enabled = false`) | — |
| **Restore** | `oh teams restore <id>` (sets `enabled = true`) | — |
| **Sync** | — | Key `s` → async git pull |
| **Undo** | — | Key `u` (stack of 10 levels) |
| **Navigation** | — | `:` omnibar → `teams.add`, `teams.refresh`, `teams.board`, `teams.status`, `teams.sync.<id>` |

### Test points

| # | Scenario | Expected result |
|---|----------|-----------------|
| 1 | `oh teams list` | Table with: ID, name, member_id, repo (truncated), status |
| 2 | `oh teams add` without `--repo` | Error: flag required |
| 3 | `oh teams add` without `--id` | ID automatically derived from repo name |
| 4 | `oh teams remove` without `-f` | Asks for interactive confirmation |
| 5 | `oh teams remove` with linked projects | All projects detached |
| 6 | `oh teams archive` | `enabled = false` in hub.toml |
| 7 | `oh teams restore` | `enabled = true` in hub.toml |
| 8 | `oh teams detach` | Project reverts to solo mode |
| 9 | TUI: add → delete → `u` | Undo restores the entry |

---

## 4. Per-project Configuration

### Available modes

| Mode | Behavior | `.opencode/team.json` | MCP team |
|------|----------|----------------------|----------|
| `inherit` | Uses the hub's team | Created with hub config | Injected |
| `custom` | Dedicated team-state repo | Created with custom config | Injected |
| `disabled` | No team | File deleted | Not injected |

### CLI / TUI comparison

| | CLI | TUI |
|---|-----|-----|
| **Configure** | Proposed in `oh project add` / `oh init` | Omnibar `team configure` → 3-choice modal |
| **Deploy** | `oh deploy` / `oh sync --all` | Automatic on deploy |
| **Verify** | Inspect `.opencode/team.json` | `team.detail` view (key `Enter`) |

### Test points

| # | Scenario | Expected result |
|---|----------|-----------------|
| 1 | `oh deploy` with team enabled | `opencode.json` contains MCP server `team` |
| 2 | `oh deploy` with team disabled | No MCP server `team` |
| 3 | Custom mode | Clones a different repo from the hub |
| 4 | Inherit mode, member_id absent | Comes from the hub |
| 5 | Change inherit → disabled + redeploy | MCP team removed |

---

## 5. Claim Lifecycle

### State machine

```
                 oh claim --planned
                        │
                        ▼
              ┌─────────────────┐
              │     PLANNED     │ ◄─── tracker auto_plan
              │     (TODO)      │
              └────────┬────────┘
                       │ oh claim / board 'c'
                       ▼
              ┌─────────────────┐
         ┌───►│  IN_PROGRESS    │◄──────────────────┐
         │    │  (IN PROGRESS)  │                   │
         │    └───┬─────────┬───┘                   │
         │        │         │                       │
         │  board 's'  board 's'                    │
         │        │         │                       │
         │        ▼         ▼                       │
         │  ┌──────────┐  ┌──────────┐             │
         │  │  REVIEW  │  │ BLOCKED  │             │
         │  └────┬─────┘  └────┬─────┘             │
         │       │              │                   │
         │       │ board 's'    │ board 's'         │
         │       ▼              └───────────────────┘
         │  ┌──────────┐
         │  │   DONE   │
         │  └────┬─────┘
         │       │ reopen (tracker)
         └───────┘
```

### Valid transitions

| From | To | Trigger |
|------|----|---------|
| `planned` | `in_progress` | `oh claim` / board `c` / board `s` |
| `in_progress` | `review` | board `s` / agent review.ready |
| `in_progress` | `blocked` | board `s` |
| `review` | `done` | board `s` / tracker issue closed |
| `review` | `in_progress` | board `s` (return) |
| `blocked` | `in_progress` | board `s` |
| `done` | `in_progress` | Tracker issue reopened |

### Claim storage

```
team-state/projects/T-SRU/claims/SRU-142.toml
┌─────────────────────────────────────────────┐
│ claimed_by = "benjamin"                     │
│ claimed_at = 2026-07-15T09:30:00Z           │
│ worktree = "feat/SRU-142-user-auth"         │
│ status = "in_progress"                      │
│ last_activity = 2026-07-16T14:22:00Z        │
│ labels = ["agent-reviewed"]                 │
│ external_iid = 142                          │
└─────────────────────────────────────────────┘
```

### Special labels

| Label | Meaning | Board display |
|-------|---------|---------------|
| `agent-reviewed` | Reviewed by an AI agent | `[AI]` |
| `needs-human-review` | Requires human review | `[!]` |
| `hub:done` | Marked as done by the hub | — (pushed to tracker) |

### CLI / Board comparison

| Action | CLI | Board (CLI `oh team board` / TUI `team.board`) |
|--------|-----|------------------------------------------------|
| Claim | `oh claim SRU-142` | Key `c` on the ticket |
| Plan | `oh claim SRU-142 --planned` | — |
| With branch | `oh claim SRU-142 --worktree feat/...` | — |
| Release | `oh release SRU-142` | Key `x` |
| Transfer | `oh claim transfer SRU-142 --to alice` | Key `t` → member modal |
| Change status | — | Key `s` → 5-choice modal |

### Generated events

| Action | Event | Data |
|--------|-------|------|
| Claim | `claim.taken` | `{actor, ticket, project}` |
| Double-claim | `claim.conflict` | `{actor, ticket, owner}` |
| Transfer | `claim.transferred` | `{actor, ticket, data:{to}}` |
| Release | `claim.released` | `{actor, ticket}` |

### Test points

| # | Scenario | Expected result |
|---|----------|-----------------|
| 1 | Standard claim | TOML file created, status=in_progress, event logged |
| 2 | Claim --planned | status=planned |
| 3 | Double-claim | ErrClaimExists + claim.conflict event |
| 4 | Transfer | ClaimedBy changed + brief auto-generated |
| 5 | Release | Claim file deleted |
| 6 | Max WIP policy | Claim refused if limit reached |
| 7 | Invalid transition (planned→done) | ErrInvalidTransition |
| 8 | Claim with worktree | Worktree field populated |

---

## 6. Kanban Board

### Layout

```
┌──────────────────────────────────────────────────────────────────────────┐
│  oh team board                                            [FILTER] r:5s  │
├──────────────┬──────────────┬──────────────┬──────────────┬─────────────┤
│    TODO      │ IN PROGRESS  │   REVIEW     │   BLOCKED    │    DONE     │
├──────────────┼──────────────┼──────────────┼──────────────┼─────────────┤
│              │              │              │              │             │
│ SRU-145      │▶SRU-142     │ SRU-139      │              │ SRU-138     │
│ @alice       │ @benjamin   │ @charlie [AI]│              │ @alice      │
│              │ [!]         │              │              │             │
│ SRU-146      │ SRU-143     │              │              │ SRU-137     │
│ @bob         │ @alice      │              │              │ @bob        │
│              │              │              │              │             │
├──────────────┴──────────────┴──────────────┴──────────────┴─────────────┤
│  h/l:columns  j/k:items  c:claim  x:release  t:transfer  s:status      │
│  /:search  f:filter  r:refresh  q:quit                                  │
└──────────────────────────────────────────────────────────────────────────┘

Legend:
  [AI] = agent-reviewed       ▶ = current selection
  [!]  = needs-human-review   @name = assigned
```

### Keyboard shortcuts

| Key | Action | Context |
|-----|--------|---------|
| `h` / `←` | Previous column | Navigation |
| `l` / `→` | Next column | Navigation |
| `j` / `↓` | Next item in column | Navigation |
| `k` / `↑` | Previous item in column | Navigation |
| `c` | Claim the selected ticket | Action |
| `x` | Release the ticket | Action |
| `t` | Transfer → member selection modal | Action |
| `s` | Change status → 5-choice modal | Action |
| `r` | Refresh (git pull + reload) | Maintenance |
| `/` | Text search (filter by ID, title, assignee) | Filtering |
| `f` | Filter menu (my tickets, by assignee, by label, clear) | Filtering |
| `Esc` | Clear active filters | Filtering |
| `q` | Quit (CLI board only) | Navigation |

### Filtering system

```
                 ┌─────────────┐
                 │  All        │
                 │  tickets    │
                 └──────┬──────┘
                        │
           ┌────────────┼────────────┐
           │            │            │
     ┌─────┴─────┐ ┌───┴───┐ ┌─────┴─────┐
     │ Text (/)  │ │ Assi- │ │ Label (f) │
     │ ID/title/ │ │ gnee  │ │ exact     │
     │ assignee  │ │ (f)   │ │ match     │
     └─────┬─────┘ └───┬───┘ └─────┬─────┘
           │            │            │
           └────────────┼────────────┘
                        │ AND (combined)
                        ▼
                 ┌─────────────┐
                 │  Tickets    │
                 │  displayed  │
                 └─────────────┘

   [Esc] clears all active filters
   Yellow [FILTER] indicator in hints bar
```

### Auto-refresh and concurrency

```
   ┌──────────────────────────────────────────────┐
   │  Ticker (5s default, configurable)           │
   │                                              │
   │  tick → actionInProgress?                    │
   │          ├── yes → skip (no double exec)     │
   │          └── no  → async {                   │
   │                      SyncFunc() (git pull)   │
   │                      LoadClaims()            │
   │                      RenderBoard()           │
   │                    }                         │
   │                                              │
   │  User action (c/x/t/s):                      │
   │    1. actionInProgress = true                │
   │    2. Execution (commitAndPush)              │
   │    3. Reload board                           │
   │    4. actionInProgress = false               │
   └──────────────────────────────────────────────┘
```

### Test points

| # | Scenario | Expected result |
|---|----------|-----------------|
| 1 | Empty board | Appropriate message, no crash |
| 2 | Claim (`c`) | Ticket moves to IN PROGRESS, assigned to current user |
| 3 | Release (`x`) | Ticket disappears or returns to planned |
| 4 | Transfer (`t`) | Modal listing other members, confirmation |
| 5 | Change status (`s`) | 5-choice modal, ticket moves to new column |
| 6 | Filter `/` | Only matching tickets visible, `[FILTER]` indicator |
| 7 | Filter `f` > "My tickets" | Only my tickets |
| 8 | `--watch` (CLI) | Board auto-refreshes (5s) |
| 9 | Labels displayed | `[AI]` and `[!]` correctly rendered |
| 10 | Double-press action | Debouncing: no simultaneous execution |
| 11 | Combined filter (`/` + `f`) | Intersection (AND) of both filters |

---

## 7. Team Status

### CLI / TUI comparison

| | CLI | TUI |
|---|-----|-----|
| **Command** | `oh team status [--detail]` | `team.status` view (omnibar `teams.status`) |
| **Refresh** | Re-run the command | Key `r` (async pull + re-render) |
| **Back** | — | `Esc` |

### Displayed information

```
┌──────────────────────────────────────────────────────┐
│  Team Status                                         │
│  Repo: git@gitlab.com:acme/team-state.git            │
│  Member: benjamin                                    │
├──────────────────────────────────────────────────────┤
│  --- Members (3) ---                                 │
│  @benjamin [2 tickets] SRU-142 (in_progress),        │
│                         SRU-145 (review)     ← accent│
│  @alice    [1 ticket]  SRU-143 (in_progress)         │
│  @charlie  [0 tickets]                               │
├──────────────────────────────────────────────────────┤
│  --- Summary ---                                     │
│  planned: 3 | in_progress: 2 | review: 1            │
│  blocked: 0 | done: 5                               │
├──────────────────────────────────────────────────────┤
│  --- Recent Activity ---                             │
│  5m ago     benjamin  completed   SRU-138            │
│  1h ago     alice     claimed     SRU-143            │
│  yesterday  charlie   review ready SRU-139           │
│  2d ago     benjamin  transferred SRU-140            │
│  3d ago     alice     released    SRU-137            │
└──────────────────────────────────────────────────────┘
```

### Test points

| # | Scenario | Expected result |
|---|----------|-----------------|
| 1 | Without team configured | Message "Team not configured for this project" |
| 2 | Repo not cloned | Message "Repo not cloned. Run 'oh team init'" |
| 3 | `--detail` | Shows sub-beads with progress |
| 4 | Relative timestamps | "5m ago", "yesterday", etc. correctly computed |
| 5 | Auto git pull | Fresh data on open (TUI) |
| 6 | Current member | Highlighted with accent color |

---

## 8. Activity

### CLI / TUI comparison

| | CLI | TUI |
|---|-----|-----|
| **Command** | `oh team activity [flags]` | Integrated in `team.status` (last 5) |
| **Navigation** | — | Omnibar `teams.activity` |

### CLI flags

| Flag | Description | Example |
|------|-------------|---------|
| `--today` | Today's events only | `oh team activity --today` |
| `--week` | Last 7 days | `oh team activity --week` |
| `--member <id>` | Filter by member | `oh team activity --member alice` |
| `--project <name>` | Filter by project | `oh team activity --project T-SRU` |
| `--limit <n>` | Maximum events (default: 20) | `oh team activity --limit 5` |

### Event types

| Type | Description | Notification format |
|------|-------------|---------------------|
| `session.complete` | Session completed | `[project] actor completed ticket (duration)` |
| `review.ready` | Ready for review | `[project] Review ready for ticket — mr_url` |
| `audit.finding` | Audit result | `[project] Audit domain: N finding(s)` |
| `claim.taken` | Ticket claimed | `[project] actor claimed ticket` |
| `claim.conflict` | Claim conflict | `[project] ⚠ Conflict on ticket (already claimed by owner)` |
| `claim.transferred` | Ticket transferred | `[project] ticket transferred from actor to target` |
| `claim.released` | Ticket released | `[project] actor released ticket` |
| `wiki.proposal` | Wiki proposal | `[Team] Wiki proposal (page) by actor` |
| `wiki.accepted` | Wiki accepted | `[Team] Wiki page updated by actor` |
| `wiki.rejected` | Wiki rejected | — |

### Test points

| # | Scenario | Expected result |
|---|----------|-----------------|
| 1 | Without flag | Last 20 events |
| 2 | `--today` | Filtered to today's date |
| 3 | `--week` | Filtered to 7 days |
| 4 | `--member alice` | Only alice's events |
| 5 | `--project T-SRU` | Only project events |
| 6 | `--limit 5` | Exactly 5 results max |
| 7 | Combination | `--today --member alice` works |

---

## 9. Policies — Verification and Enforcement

### Evaluation diagram

```
              ┌──────────────────┐
              │  PolicyContext   │
              │  ├── BranchName  │
              │  ├── CommitMsg   │
              │  ├── DiffLines   │
              │  ├── ActiveClaims│
              │  ├── HasReview   │
              │  ├── HasTests    │
              │  └── HasCoverage │
              └────────┬─────────┘
                       │
                       ▼
   ┌─────────────────────────────────────┐
   │  LoadPolicies(project)              │
   │  = policies.toml (global)           │
   │  + projects/<p>/policies-override   │
   │                                     │
   │  RULE: override can only make       │
   │  things stricter                    │
   │  (warn→refuse OK,                   │
   │   refuse→warn IMPOSSIBLE)           │
   └────────────────┬────────────────────┘
                    │
        ┌───────────┼───────────┬───────────────┐
        ▼           ▼           ▼               ▼
   ┌─────────┐ ┌─────────┐ ┌──────────┐ ┌────────────────┐
   │  regex  │ │ boolean │ │  limit   │ │ forbidden_pat  │
   │         │ │         │ │          │ │                │
   │ branch? │ │ review? │ │ max_wip? │ │ diff_only?     │
   │ commit? │ │ tests?  │ │          │ │ modified_files?│
   │         │ │ cover?  │ │          │ │ all_files?     │
   └────┬────┘ └────┬────┘ └────┬─────┘ └───────┬────────┘
        │           │            │               │
        └───────────┴────────────┴───────────────┘
                              │
                              ▼
                    ┌──────────────────┐
                    │  []PolicyResult  │
                    │  ├── Passed bool │
                    │  ├── Enforcement │
                    │  └── Message     │
                    └────────┬─────────┘
                             │
                   ┌─────────┴─────────┐
                   ▼                   ▼
             ┌──────────┐       ┌──────────┐
             │  REFUSE  │       │   WARN   │
             │  → block │       │  → log   │
             │  action  │       │  continue│
             └──────────┘       └──────────┘
```

### Policy types

| Type | Parameters | Example | Evaluation |
|------|-----------|---------|------------|
| `regex` | `rule` (pattern) | `^(feat\|fix\|chore)/[A-Z]+-\d+` | Match branch/commit/files |
| `boolean` | `enabled` | Review required | Checks HasReview/HasTests/HasCoverage |
| `limit` | `max`, `unit` | Max WIP = 2 | Compares ActiveClaims >= Max |
| `forbidden_pattern` | `patterns[]`, `scope` | No `console.log` | String-contains on DiffLines |

### Scopes for `forbidden_pattern`

| Scope | Target |
|-------|--------|
| `diff_only` | Added lines in the diff only |
| `modified_files` | Full content of modified files |
| `all_files` | All files in the project |
| `per_feature_branch` | Commits of the feature branch |

### Double enforcement (CLI + Agents)

```
   ┌──────────────────────────────────────────────────────────────┐
   │                                                              │
   │  CLI (hard enforcement)              Agent (soft)            │
   │  ─────────────────────               ─────────────           │
   │  oh claim → max_wip                  Before branch:          │
   │  oh start → branch_naming              branch_naming         │
   │  oh release → review_required        Before commit:          │
   │             → tests_required           commit_format         │
   │                                                              │
   │  Action: BLOCKS if refuse            Action: INFORMS         │
   │          WARN if warn                 (does not block)       │
   │                                                              │
   └──────────────────────────────────────────────────────────────┘
```

### CLI / TUI comparison

| Action | CLI | TUI |
|--------|-----|-----|
| List policies | `oh policies list [--project P]` | — |
| Check | `oh policies check --branch <n> --commit "<m>"` | Automatic (agents) |
| Add | `oh policies add` (interactive) | — |
| View config status | `oh team config status` | `team.detail` view |

### Test points

| # | Scenario | Expected result |
|---|----------|-----------------|
| 1 | `oh policies list` | All policies with type and enforcement |
| 2 | Invalid branch + refuse policy | Action blocked |
| 3 | Invalid branch + warn policy | Warning displayed, action continues |
| 4 | Invalid commit | Rejected or warning depending on enforcement |
| 5 | Max WIP reached | Claim refused |
| 6 | Project override (warn→refuse) | Accepted, stricter policy |
| 7 | Project override (refuse→warn) | Rejected, cannot loosen |
| 8 | `forbidden_pattern` in diff | Pattern detected, reported |
| 9 | Policy disabled | Always Passed=true |

---

## 10. Detailed Configuration (Team Detail)

### TUI layout

```
┌──────────────────────────────────────────────────────────────────┐
│  Team configuration: acme                            [r]efresh   │
├──────────────────────────────────────────────────────────────────┤
│                                                                  │
│  ═══ MCP Services ═══                                            │
│                                                                  │
│  --- gitlab ---                                                  │
│  [✓] Enabled                              (enforced by the team) │
│  URL: https://gitlab.company.com          (enforced by the team) │
│  Token: ●●●●●●●● ✓                       [local]                │
│  [✓] Write enabled                       [local]                │
│                                                                  │
│  --- jira ---                                                    │
│  [ ] Enabled                                                     │
│  URL:                                                            │
│                                                                  │
│  ═══ Tracker ═══                                                 │
│                                                                  │
│  Type: [GitLab ▾]                                                │
│  [✓] Enabled                                                     │
│  [✓] Auto-sync (interval: 5 min)                                │
│  [✓] Push labels                                                 │
│  [✓] Auto-plan assigned (max: 5/member)                          │
│                                                                  │
│  ═══ Project Mappings ═══                                        │
│  T-SRU → 42                                   [a]dd [d]elete     │
│  T-BILLING → 87                                                  │
│                                                                  │
│  ═══ Notifications ═══                                           │
│  Type: [Mattermost ▾]                                            │
│  Webhook: https://mm.company.com/hooks/...                       │
│  Channel: #dev-ai                                                │
│  Bot: OpenHub                                                    │
│                                                                  │
│  ═══ Collaboration ═══                                           │
│  Max sessions: 3                                                 │
│  Stale days: 3                                                   │
│  Done retention: 7 days                                          │
│                                                                  │
│  ═══ Models (recommendations) ═══                                │
│  Default: claude-sonnet-4-20250514                               │
│  --- families ---                                                │
│  anthropic → claude-sonnet-4-20250514                            │
│  --- agents ---                                                  │
│  orchestrator-dev → claude-sonnet-4-20250514                     │
│                                                                  │
│  ═══ Local Overrides ═══                                         │
│  Tracker enabled: [inherit]  (cycle: inherit/yes/no)             │
│  Auto-sync: [inherit]                                            │
│  Push labels: [inherit]                                          │
│                                                                  │
├──────────────────────────────────────────────────────────────────┤
│  [w]save  [s]sync-tracker  [t]test  [a]add  [d]del  [u]undo      │
└──────────────────────────────────────────────────────────────────┘
```

### TUI detail shortcuts

| Key | Action |
|-----|--------|
| `j`/`k` | Navigate between fields (skip headers/spacers) |
| `Space` | Toggle bool / cycle tri-state (inherit→yes→no→inherit) |
| `Enter` | Edit field (input/select/password modal) |
| `w` | Save (2 passes: team commit+push + local hub.toml) |
| `s` | Sync tracker (async pull from external tracker) |
| `t` | Test tracker connection (displays username if OK) |
| `a` | Add dynamic entry (mappings, families, agents) |
| `d` | Delete dynamic entry |
| `u` | Undo (undo last modification via UndoStack) |
| `r` | Refresh (git pull + reload data) |

### Field types

| Kind | Behavior |
|------|----------|
| `bool` | Space toggle true/false, green checkmark / red cross |
| `tri-state` | Cycle: inherit → yes → no → inherit |
| `string` | Enter opens input modal |
| `select` | Enter opens modal with predefined options |
| `password` | Enter opens password modal, masked display + indicator |

### CLI / TUI comparison

| | CLI | TUI |
|---|-----|-----|
| **Access** | `oh team config` (interactive wizard) | `Enter` on a team → `team.detail` view |
| **View status** | `oh team config status` | Integrated in the view |
| **Save** | Automatic at end of wizard | Key `w` (explicit, git push) for team views. Auto-save for hub/project views. |
| **Granularity** | Full wizard | Field by field |

### Test points

| # | Scenario | Expected result |
|---|----------|-----------------|
| 1 | Enforced fields | Non-editable, grayed out with "(enforced)" |
| 2 | Masked token | Displays `●●●●` + ✓/✗ indicator |
| 3 | Tri-state cycle | inherit → yes → no → inherit correctly |
| 4 | `w` save | 2 passes: team (commit+push) + local (hub.toml) |
| 5 | Quit without saving | Warning toast displayed |
| 6 | `s` sync tracker | Summary modal (created/updated/pushed) |
| 7 | `t` test connection | Username displayed if success, error otherwise |
| 8 | `a`/`d` dynamic | Adding/deleting mappings works |
| 9 | `oh team config status` | Displays effective config (team + local merge) |

---

## 11. Tracker Synchronization (GitLab / Jira)

### Detailed algorithm

```
   oh team sync-tracker
          │
          ▼
   ┌────────────────────────────────────────────────┐
   │  1. Resolve team config (hub + project)        │
   │  2. Load config.toml [tracker]                 │
   │  3. Merge: team config + local overrides       │
   │  4. Resolve credentials (env var / keychain)   │
   │  5. Create tracker client (GitLab / Jira)      │
   └───────────────────────┬────────────────────────┘
                           │
                           ▼
   ┌────────────────────────────────────────────────┐
   │  For each project in [tracker.projects]:       │
   │                                                │
   │  ┌──────────────────────────────────────────┐  │
   │  │  PULL (tracker → claims):                │  │
   │  │  • Fetch issue by external_iid           │  │
   │  │  • Issue closed → claim = "done"         │  │
   │  │  • Issue reopened → claim = "in_progress"│  │
   │  │  • Labels mirrored (except hub:*)        │  │
   │  │  • Warning if assignee ≠ claim owner     │  │
   │  └──────────────────────────────────────────┘  │
   │                                                │
   │  ┌──────────────────────────────────────────┐  │
   │  │  PUSH (claims → tracker):                │  │
   │  │  If push_labels enabled:                 │  │
   │  │  • Push "agent-reviewed"                 │  │
   │  │  • Push "hub:done"                       │  │
   │  └──────────────────────────────────────────┘  │
   │                                                │
   │  ┌──────────────────────────────────────────┐  │
   │  │  AUTO-PLAN:                              │  │
   │  │  If auto_plan_assigned enabled:          │  │
   │  │  • Fetch issues assigned per member      │  │
   │  │  • Create "planned" claim if absent      │  │
   │  │  • Respect max_auto_plan_per_member      │  │
   │  └──────────────────────────────────────────┘  │
   └───────────────────────┬────────────────────────┘
                           │
                           ▼
   ┌────────────────────────────────────────────────┐
   │  Result:                                       │
   │  • Claims created: N                           │
   │  • Claims updated: N                           │
   │  • Labels pushed: N                            │
   │  • Warnings: [...]                             │
   │  • Errors: [...]                               │
   └────────────────────────────────────────────────┘
```

### Error handling

| Error | Behavior |
|-------|----------|
| Invalid token | 3 consecutive failures → `ShouldAutoSync() = false` |
| Rate limited | Stops all projects immediately |
| Issue not found | Ignored (deleted on tracker) |
| Error on one project | Log + continue with next project |

### CLI / TUI comparison

| | CLI | TUI |
|---|-----|-----|
| **Launch** | `oh team sync-tracker` | Key `s` in `team.detail` or omnibar |
| **Result** | Text output in terminal | Summary modal |
| **Missing token** | Error + explicit message | Wizard proposes token configuration |
| **Test connection** | — | Key `t` (displays username if OK) |

### Test points

| # | Scenario | Expected result |
|---|----------|-----------------|
| 1 | Without tracker config | Explicit error |
| 2 | Without token | Wizard proposes configuration / CLI error |
| 3 | Successful sync | Summary with counters |
| 4 | Issue closed on tracker | Claim changes to `done` |
| 5 | Issue reopened | Claim changes to `in_progress` |
| 6 | `auto_plan_assigned` | New `planned` claims created |
| 7 | `push_labels` | Labels visible on GitLab/Jira |
| 8 | `max_auto_plan_per_member` | No more than N auto-created claims |
| 9 | Project mappings | Only configured projects synchronized |

---

## 12. Takeover Briefs

### Generation flow

```
   oh claim transfer SRU-142 --to alice
          │
          ├── 1. TransferClaim() → claim.ClaimedBy = "alice"
          ├── 2. AppendEvent(claim.transferred)
          └── 3. GenerateRawBrief()
                    │
                    ├── Collect ticket events (claim.*, session.*)
                    ├── Compute SessionsCount, First/Last session
                    ├── Extract Git info (branch, commits, files)
                    └── SaveBrief()
                          │
                          ├── SRU-142_2026-07-16.toml (structured)
                          └── SRU-142_2026-07-16.md   (human-readable)

   oh takeover-brief enrich SRU-142
          │
          └── Agent brief-enricher (headless)
                    │
                    ├── Analyze modified files
                    ├── Examine architectural decisions
                    ├── Identify open questions
                    └── Write SRU-142_2026-07-16.enriched.md

   Reading priority: .enriched.md > .md > .toml
```

### Brief structure

```
team-state/projects/T-SRU/takeover-briefs/SRU-142_2026-07-16.toml
┌──────────────────────────────────────────────┐
│ [meta]                                       │
│ ticket_id = "SRU-142"                        │
│ project = "T-SRU"                            │
│ transferred_from = "benjamin"                │
│ transferred_to = "alice"                     │
│ transfer_date = 2026-07-16T10:00:00Z         │
│ reason = "transfer"  # or "stale"            │
│                                              │
│ [activity]                                   │
│ sessions_count = 4                           │
│ first_session = 2026-07-12T09:00:00Z         │
│ last_session = 2026-07-15T16:30:00Z          │
│ total_duration_minutes = 240                 │
│                                              │
│ [git]                                        │
│ branch = "feat/SRU-142-user-auth"            │
│ commits_count = 12                           │
│ last_commit_message = "feat: add JWT valid"  │
│ last_commit_date = 2026-07-15T16:25:00Z      │
│                                              │
│ [[git.files_modified]]                       │
│ path = "internal/auth/jwt.go"                │
│ additions = 85                               │
│ deletions = 12                               │
│                                              │
│ [[git.files_created]]                        │
│ path = "internal/auth/jwt_test.go"           │
│ additions = 120                              │
│ deletions = 0                                │
│                                              │
│ [[events]]                                   │
│ timestamp = 2026-07-15T16:30:00Z             │
│ type = "session.complete"                    │
│ summary = "Implemented JWT validation"       │
│                                              │
│ [[events]]                                   │
│ timestamp = 2026-07-14T11:00:00Z             │
│ type = "session.complete"                    │
│ summary = "Added auth middleware"            │
└──────────────────────────────────────────────┘
```

### Stale detection

```
   IsStale(claim, staleDays) :
     lastActivity = claim.LastActivity || claim.ClaimedAt
     return time.Since(lastActivity) > staleDays * 24h

   Default: staleDays = 3 (configurable in config.toml [takeover])
```

### CLI / TUI comparison

| Action | CLI | TUI |
|--------|-----|-----|
| List | `oh takeover-brief list` | — |
| View | `oh takeover-brief show <ticket>` | — |
| Enrich (AI) | `oh takeover-brief enrich <ticket>` | Action `runTakeoverEnrich()` |

### Test points

| # | Scenario | Expected result |
|---|----------|-----------------|
| 1 | Transfer claim | Brief auto-generated (.toml + .md) |
| 2 | Brief contains context | Sessions, commits, files, events |
| 3 | `oh takeover-brief enrich` | `.enriched.md` created by agent |
| 4 | Reading via MCP | `team_takeover_brief` returns the brief |
| 5 | Stale ticket (> stale_days) | Proposal to generate a brief |
| 6 | Reading priority | enriched > md > toml |
| 7 | Brief for nonexistent ticket | ErrBriefNotFound |

---

## 13. Team Wiki

### Contribution flow

```
   Agent documentarian
          │
          ▼
   team_wiki_write(page, content, confidence, project)
          │
          ├── Validation (size, format)
          ├── WikiCreateProposal() → wiki/.pending/<page>.md
          ├── AppendEvent(wiki.proposal)
          └── Notification sent
                    │
                    ▼
   oh team wiki review
          │
          ├── List .pending/
          ├── Human accepts/rejects
          │     ├── Accept → WikiAcceptProposal() → wiki/<page>.md
          │     │            AppendEvent(wiki.accepted)
          │     └── Reject → WikiRejectProposal() → deleted
          │                  AppendEvent(wiki.rejected)
          └── Commit + push
```

### CLI / TUI comparison

| Action | CLI | TUI |
|--------|-----|-----|
| List pages | `oh team wiki list` | — |
| Read | `oh team wiki read <page>` | — |
| Review | `oh team wiki review` | — |
| Write | Via `documentarian` agent only | — |

### Test points

| # | Scenario | Expected result |
|---|----------|-----------------|
| 1 | `wiki list` | Existing pages listed |
| 2 | `wiki read decisions` | Markdown content displayed |
| 3 | Agent proposes page | Appears in `.pending/` |
| 4 | `wiki review` + accept | Page moved to `wiki/` |
| 5 | `wiki review` + reject | Page deleted |
| 6 | Max size exceeded | ErrProposalTooLarge |
| 7 | Agents read wiki | MCP `team_wiki_read` works |

---

## 14. Pattern Library

### Flow

```
   Agent (planner/pathfinder) → successful planning
          │
          ▼
   team_patterns_propose(name, tags, complexity, content)
          │
          ├── CreatePattern() → patterns/<name>.md + index.toml (validated=false)
          └── Commit + push

   oh patterns validate <name>
          │
          └── ValidatePattern() → index.toml: validated=true
```

### CLI commands

| Command | Description |
|---------|-------------|
| `oh patterns list` | List all patterns (filterable by tags) |
| `oh patterns show <name>` | Display a pattern's content |
| `oh patterns add` | Manually add a pattern |
| `oh patterns validate <name>` | Validate a pattern proposed by an agent |
| `oh patterns remove <name>` | Delete a pattern |

### Test points

| # | Scenario | Expected result |
|---|----------|-----------------|
| 1 | Pattern proposed by agent | `validated = false` in index |
| 2 | `oh patterns validate` | Changes to `validated = true` |
| 3 | Storage | `patterns/<name>.md` + entry in `index.toml` |
| 4 | Filtering by tags | `team_patterns_list(tags: ["api"])` returns matches |
| 5 | `oh patterns remove` | File and entry deleted |

---

## 15. Parallel Sessions

### Command

```bash
oh start --parallel --tickets bd-42,bd-43,bd-44 [--priority] [--max-sessions]
```

### Architecture

```
   oh start --parallel --tickets bd-42,bd-43,bd-44
          │
          ├── Claim bd-42 (worktree: feat/bd-42)
          ├── Claim bd-43 (worktree: feat/bd-43)
          └── Claim bd-44 (worktree: feat/bd-44)
                    │
          ┌────────┼────────┐
          ▼        ▼        ▼
     ┌────────┐┌────────┐┌────────┐
     │Session ││Session ││Session │
     │  bd-42 ││  bd-43 ││  bd-44 │
     │worktree││worktree││worktree│
     │isolated││isolated││isolated│
     └────┬───┘└────┬───┘└────┬───┘
          │         │         │
          ▼         ▼         ▼
     TUI full screen: real-time status
     ├── Modified files per session
     ├── Potential conflicts detected
     └── Navigation: j/k, Enter, r, q
```

### Configuration (config.toml)

```toml
[parallel]
max_sessions = 3
port_range_start = 4100
auto_merge_beads = true
```

### Test points

| # | Scenario | Expected result |
|---|----------|-----------------|
| 1 | Parallel launch | Each ticket in an isolated worktree |
| 2 | Real-time TUI | Status, modified files visible |
| 3 | `max_sessions` respected | No more than N simultaneous sessions |
| 4 | Conflicts detected | Warning if same files touched |
| 5 | Navigation | `j/k` (sessions), `Enter` (attach), `q` (quit) |

---

## 16. Notifications

### Dispatch flow

```
   ┌──────────────┐     ┌──────────────┐     ┌──────────────────┐
   │  Event       │     │  AppendEvent │     │  notify.Dispatch │
   │  occurs      │────►│  (JSONL log) │     │  (HTTP POST)     │
   └──────────────┘     └──────────────┘     └────────┬─────────┘
                                                      │
                              ┌────────────────────────┘
                              │ (only for certain events)
                              ▼
   ┌─────────────────────────────────────────────────────────────┐
   │  Events that TRIGGER a notification:                        │
   │  • wiki.proposal   (MCP team_wiki_write)                    │
   │  • review.ready    (audit/review completed)                 │
   │  • custom          (MCP team_notify)                        │
   │                                                             │
   │  Events LOGGED but NOT automatically notified:              │
   │  • claim.taken                                              │
   │  • claim.released                                           │
   │  • claim.transferred                                        │
   │  • claim.conflict                                           │
   │  • session.complete                                         │
   └─────────────────────────────────────────────────────────────┘
```

### Supported platforms

| Platform | Payload | Config fields |
|----------|---------|---------------|
| **Mattermost** | `{"channel", "username", "text"}` | `webhook_url`, `channel`, `bot_name` |
| **Slack** | `{"text", "username"}` | `webhook_url`, `bot_name` |
| **Discord** | `{"content", "username"}` | `webhook_url`, `bot_name` |
| **Microsoft Teams** | MessageCard `{"@type", "text"}` | `webhook_url` |

HTTP timeout: 10 seconds for all clients.

### Multi-destination architecture

```
   config.toml [notification]
   ┌────────────────────────────────┐
   │  enabled = true                │
   │  bot_name = "OpenHub"          │
   │                                │
   │  [[notification.destinations]] │
   │  type = "mattermost"          │──► POST webhook
   │  webhook_url = "https://..."   │
   │  channel = "#dev-ai"          │
   │                                │
   │  [[notification.destinations]] │
   │  type = "slack"               │──► POST webhook
   │  webhook_url = "https://..."   │
   │                                │
   │  [[notification.destinations]] │
   │  type = "discord"             │──► POST webhook
   │  webhook_url = "https://..."   │
   └────────────────────────────────┘
```

### Test points

| # | Scenario | Expected result |
|---|----------|-----------------|
| 1 | wiki.proposal | Notification sent to all destinations |
| 2 | review.ready | Formatted notification with MR link |
| 3 | team_notify (custom) | Raw message sent as-is |
| 4 | `enabled = false` | No HTTP POST |
| 5 | Invalid webhook (404/500) | Error logged, no crash |
| 6 | Multi-destination | Message received on each platform |
| 7 | Correct bot name | Displayed name = config bot_name |
| 8 | Correct channel (Mattermost) | Message in the right channel |

---

## 17. MCP Integration (AI Agents)

### Access diagram

```
   ┌───────────────────────────────────────────────────────────────┐
   │  opencode.json (generated by oh deploy)                       │
   │  ┌─────────────────────────────────────────────────────────┐  │
   │  │  "mcpServers": {                                        │  │
   │  │    "team": {                                            │  │
   │  │      "command": "oh",                                   │  │
   │  │      "args": ["mcp", "team", "--project", "T-SRU"]     │  │
   │  │    }                                                    │  │
   │  │  }                                                      │  │
   │  └─────────────────────────────────────────────────────────┘  │
   └──────────────────────────────┬────────────────────────────────┘
                                  │
                                  ▼
   ┌───────────────────────────────────────────────────────────────┐
   │  MCP Team Server (cli/internal/mcp/team/server.go)            │
   │  Cache TTL: 30s (avoids repeated git pulls)                   │
   └───────────────────────────────────────────────────────────────┘
```

### Available MCP tools

| Tool | Params | Description | Access |
|------|--------|-------------|--------|
| `team_members` | — | List members + roles | All agents |
| `team_claims` | `project?` | Active claims | All agents |
| `team_wiki_list` | — | Wiki pages | All agents |
| `team_wiki_read` | `page` (required) | Read a page | All agents |
| `team_wiki_write` | `page`, `content`, `confidence`, `project` | Propose page | documentarian |
| `team_events` | `project?`, `limit?` (max 200) | Recent events | All agents |
| `team_notify` | `message` (required) | Send notification | All agents |
| `team_policies` | `project?` | Merged policies | All agents |
| `team_takeover_brief` | `project`, `ticket_id` (required) | Read brief | All agents |
| `team_patterns_list` | `tags?` | List patterns | All agents |
| `team_patterns_read` | `name` (required) | Read pattern | All agents |
| `team_patterns_propose` | `name`, `tags`, `complexity`, `content` | Propose pattern | orchestrator + planner |

### Automatically injected skills

```
   ┌─────────────────────────────────────────────────────────────┐
   │  Bucket A (ALL agents when team_enabled):                   │
   │  ├── team-awareness          (collaboration rules)          │
   │  ├── team-policies-enforcement (check before action)        │
   │  └── team-wiki-protocol      (wiki contribution)            │
   │                                                             │
   │  Bucket B (orchestrator-dev only):                          │
   │  └── team-coordination       (ticket selection, claims)     │
   └─────────────────────────────────────────────────────────────┘
```

### Test points

| # | Scenario | Expected result |
|---|----------|-----------------|
| 1 | `oh deploy` with team | MCP `team` in `opencode.json` |
| 2 | Agent calls `team_members` | List of members returned |
| 3 | Agent calls `team_claims` | Active claims returned |
| 4 | `team_wiki_write` by non-documentarian | Refused by permissions |
| 5 | `team_patterns_propose` by agent | Pattern created `validated=false` |
| 6 | Skills Bucket A | Injected in all agents |
| 7 | Skill Bucket B | Injected only in orchestrator-dev |
| 8 | Cache 30s | No git pull on every tool call |

---

## 18. Credentials — Security

### Fundamental principle

> **Never store secrets in the shared team-state repo.**

### Separation architecture

```
   ┌─────────────────┐     ┌─────────────────┐     ┌─────────────────┐
   │  Team-State     │     │  hub.toml       │     │  OS Keychain    │
   │  (shared Git)   │     │  (local)        │     │  (local)        │
   ├─────────────────┤     ├─────────────────┤     ├─────────────────┤
   │ Recommendations │     │ token_key       │     │ Actual secrets  │
   │ • enabled       │     │ (key name)      │────►│ • gitlab token  │
   │ • url           │     │ • url override  │     │ • jira token    │
   │ • write_recomm  │     │ • write_enabled │     │ • git creds     │
   │ • *_enforced    │     │                 │     │                 │
   │                 │     │ Key pattern:    │     │ Key pattern:    │
   │ NO TOKEN        │     │ openhub.mcp.    │     │ openhub.mcp.    │
   │                 │     │ <service>.token │     │ <service>.token │
   └─────────────────┘     └─────────────────┘     └─────────────────┘
```

### Token resolution

```
   1. Environment variable (GITLAB_TOKEN / JIRA_TOKEN)
          │
          ▼ (if absent)
   2. OS Keychain via token_key (hub.toml or project config)
          │
          ▼ (if absent)
   3. Error + wizard proposes configuration
```

### MCP resolution cascade

| Parameter | Priority (high → low) |
|-----------|----------------------|
| `Enabled` | team enforced > project override > hub config > team recommendation |
| `URL` | team enforced > project override > hub config > team recommendation > default |
| `WriteEnabled` | hub `write_enabled` (local gate); team `write_recommended` (advisory) |
| `Token` | Always local (hub or project keychain key) |

### Test points

| # | Scenario | Expected result |
|---|----------|-----------------|
| 1 | Token in env var | Used with priority |
| 2 | Token in keychain | Used if env var absent |
| 3 | Token absent everywhere | Error + wizard (TUI) or message (CLI) |
| 4 | Enforced enabled | No possibility to disable locally |
| 5 | Enforced URL | Local override ignored |
| 6 | No secret in team-state | Check `git log` of the shared repo |

---

## 19. Git Concurrency (withWriteLock)

### Critical pattern

```
   ┌──────────────────────────────────────────────────┐
   │  Every mutation (CreateClaim, Transfer, etc.)    │
   │                                                  │
   │  1. mu.Lock()           ← serializes writes      │
   │  2. git pull --rebase   ← best-effort (fresh)   │
   │  3. fn(ctx)             ← TOML file mutation     │
   │  4. git add + commit                            │
   │  5. git push            ← retry loop:           │
   │     │                                           │
   │     ├── Success → pull (pick up changes)        │
   │     ├── Auth error → permanent error            │
   │     └── Other error:                            │
   │         ├── pull --rebase                       │
   │         ├── backoff: 500ms × 2^attempt          │
   │         └── retry (max 3 attempts)              │
   │                                                  │
   │  6. mu.Unlock()                                  │
   └──────────────────────────────────────────────────┘

   Git environment:
   • GIT_TERMINAL_PROMPT=0 (never interactive prompt)
   • GIT_SSH_COMMAND="ssh -o BatchMode=yes
                          -o StrictHostKeyChecking=accept-new"
```

### Sentinel errors

| Error | Meaning |
|-------|---------|
| `ErrNotCloned` | Repo not yet cloned |
| `ErrSyncConflict` | Push failed after 3 retries |
| `ErrMemberExists` | member_id already in members.toml |
| `ErrMemberNotFound` | Member not found |
| `ErrClaimExists` | Ticket already claimed |
| `ErrClaimNotFound` | Claim does not exist |
| `ErrInvalidStatus` | Unknown claim status |
| `ErrInvalidTransition` | Forbidden status transition |
| `ErrPolicyViolation` | Policy refuses (enforcement=refuse) |
| `ErrBriefNotFound` | Takeover brief not found |
| `ErrUnsafeName` | Path traversal / invalid characters |
| `ErrProposalTooLarge` | Wiki proposal too large |

### Test points

| # | Scenario | Expected result |
|---|----------|-----------------|
| 1 | Concurrent push | Retry with rebase, max 3 attempts |
| 2 | Auth error | Immediate permanent error (no retry) |
| 3 | Unresolvable conflict | ErrSyncConflict after 3 retries |
| 4 | Path traversal (`../`) | ErrUnsafeName rejected |
| 5 | Special characters in ticketID | `/` replaced by `_` |
| 6 | Simultaneous mutations (same process) | Serialized by mutex |
| 7 | Pull warning (network) | PullWarning returned, not a fatal error |

---

## 20. End-to-end Scenarios

### Scenario A: Complete new member journey

```
 1. [  ] oh team init → 5-step wizard
 2. [  ] Check hub.toml ([[teams]] added)
 3. [  ] Check remote: members.toml contains the new member
 4. [  ] oh teams list → shows the active team
 5. [  ] oh deploy → opencode.json contains MCP team
 6. [  ] oh team status → shows members
 7. [  ] oh claim SRU-142 → status in_progress
 8. [  ] oh team board → ticket visible in IN PROGRESS
 9. [  ] Board: key 's' → move to review
10. [  ] oh team activity --today → claim.taken event visible
11. [  ] oh team sync-tracker → claims synchronized
12. [  ] oh claim transfer SRU-142 --to alice → brief generated
13. [  ] oh takeover-brief show SRU-142 → readable brief
14. [  ] oh policies check --branch "feat/SRU-142-auth"
15. [  ] TUI: teams view → key 'a' (add 2nd team)
16. [  ] oh teams archive <team2> → enabled=false
17. [  ] oh teams restore <team2> → enabled=true
18. [  ] TUI: team.detail → modify config + key 'w'
19. [  ] Verify notification on configured webhook
20. [  ] oh teams detach <project> → solo project
```

### Scenario B: Conflict and resolution

```
 1. [  ] Member A: oh claim TICKET-1
 2. [  ] Member B: oh claim TICKET-1 → ErrClaimExists
 3. [  ] Check claim.conflict event in JSONL
 4. [  ] Board: ticket shows assigned = Member A
 5. [  ] Member A: oh claim transfer TICKET-1 --to B
 6. [  ] Takeover brief auto-generated
 7. [  ] Member B: MCP team_takeover_brief → brief accessible
```

### Scenario C: Policy enforcement

```
 1. [  ] Configure branch_naming policy (enforce=refuse)
 2. [  ] oh start with invalid branch → REFUSED
 3. [  ] oh start with valid branch → OK
 4. [  ] Configure policy max_wip=1
 5. [  ] oh claim with 1 ticket already active → REFUSED
 6. [  ] oh release → claim released
 7. [  ] oh claim → OK (under the limit)
```

### Scenario D: Complete tracker workflow

```
 1. [  ] Configure [tracker] in config.toml (type=gitlab, enabled=true)
 2. [  ] Configure [tracker.projects] "T-SRU" = "42"
 3. [  ] oh team sync-tracker → initial sync
 4. [  ] Close an issue on GitLab
 5. [  ] oh team sync-tracker → claim changes to "done"
 6. [  ] Reopen the issue on GitLab
 7. [  ] oh team sync-tracker → claim changes to "in_progress"
 8. [  ] Check labels pushed to GitLab (push_labels=true)
 9. [  ] Check auto-plan (assigned issues → planned claims)
```

### Scenario E: Wiki and patterns

```
 1. [  ] Agent documentarian calls team_wiki_write
 2. [  ] Check wiki/.pending/<page>.md created
 3. [  ] oh team wiki review → accept
 4. [  ] Check wiki/<page>.md moved
 5. [  ] oh team wiki read <page> → content displayed
 6. [  ] Agent planner proposes a pattern
 7. [  ] oh patterns list → pattern visible (validated=false)
 8. [  ] oh patterns validate <name>
 9. [  ] oh patterns list → validated=true
```

### Scenario F: Parallel sessions

```
 1. [  ] oh start --parallel --tickets bd-42,bd-43
 2. [  ] Check 2 isolated Git worktrees created
 3. [  ] Check claims created for both tickets
 4. [  ] TUI: see status of both sessions in real time
 5. [  ] Modify the same file in both sessions
 6. [  ] Check potential conflict detection
 7. [  ] Complete the sessions
 8. [  ] Check proposed merge (Beads) or branches (external)
```

---

## TUI Shortcut Summary by View

### Teams View (`teams`)

| Key | Action |
|-----|--------|
| `Enter` | Open team detail |
| `a` | Add team |
| `d` | Delete team |
| `s` | Sync (git pull) |
| `r` | Refresh list |
| `u` | Undo (stack of 10) |
| `?` | Help |
| `:` | Omnibar |

### Status View (`team.status`)

| Key | Action |
|-----|--------|
| `r` | Refresh |
| `Esc` | Back |

### Board View (`team.board`)

| Key | Action |
|-----|--------|
| `h`/`l` | Columns |
| `j`/`k` | Items |
| `c` | Claim |
| `x` | Release |
| `t` | Transfer |
| `s` | Status |
| `r` | Refresh |
| `/` | Search |
| `f` | Filter |
| `Esc` | Clear filter |
| `q` | Quit (CLI) |

### Detail View (`team.detail`)

| Key | Action |
|-----|--------|
| `j`/`k` | Navigate |
| `Space` | Toggle |
| `Enter` | Edit |
| `w` | Save |
| `s` | Sync tracker |
| `t` | Test connection |
| `a` | Add dynamic |
| `d` | Delete dynamic |
| `u` | Undo |
| `r` | Refresh |
