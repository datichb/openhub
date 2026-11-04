> [Lire en français](team-setup.fr.md)

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

Or from the TUI, type `team init` in the omnibar (`Ctrl+P`).

### CLI mode (terminal)

The wizard runs full-screen with 5 adaptive steps and a side info panel.

### TUI mode (inline wizard)

The wizard runs directly inside the TUI shell without leaving the interface. The omnibar
and toasts remain accessible. Shortcuts: `Ctrl+S` submit, `Ctrl+B` (or `Alt+←`, `PgUp`) back,
`Esc` for « Quit the wizard? » (quit, skip an optional step or continue).

### Phase 1 — Repository, access and identity (sequential)

The wizard follows the same order for "Create" and "Rejoin", in `oh init` and in the palette
(`team init`, `team rejoin`):

1. **Repository**: the team-state repo URL and, optionally, the GitLab server when the URL is an SSH alias.
2. **Access probe**: `git ls-remote`, read-only, with every prompt disabled and a waiting
   indicator. It uses what git already has (SSH agent, credential helper, `netrc`, a public
   repository) and the token `oh` keeps for that repository. Nothing is cloned or written.
3. **Access**: this step only appears when the probe is refused for an access reason.
   - **HTTPS**: the wizard asks for a token (user name `oauth2` by default for GitLab),
     **checks** it against the repository, then keeps it in the keychain of `oh`
     (`openhub.git.<host>/<repo>.token`, see [config](../reference/config.en.md#known-keys)).
     `oh` hands it to git through a one-shot `GIT_ASKPASS` for every operation of the team-state
     (clone, pull, push, probe, daemon); it is never in a URL, on a command line or in git's
     `credential.helper`. A token that is refused is not kept.
   - **SSH**: the message gives git's answer and recalls `ssh-add`; "Retry" probes again.
4. **Identity**: "Create" asks your card (ID, display name, optional GitLab handle, role).
   "Rejoin" lists the members ("Team members (N)"): you pick your card, or **"+ I am not in the
   list: create my card"**, which opens the same form. See [Your member card](#your-member-card).
5. **Clone** into `~/.oh/team-states/<host>/<repo>/` (or pull if already cloned), creation of the
   directory structure (if first member) and detection of duplicate clones.

Every screen of a wizard carries its counter: "Step 2/5 — Identity".

#### Empty repository

A remote repository with no commit at all (a brand-new GitLab project) is valid for **"Create"**:
there is nothing to pull, the structure and then the card are the first two commits, pushed with
the branch (`push -u`). **"Rejoin"** on an empty repository says the team has not been created yet
and what to do (create it, or have its lead create it); a repository that has commits but no
`members.toml` gets a distinct message ("this repository holds no oh team").

A folder that already holds the clone **of another repository** is never reused: the wizard
says so (folder, repository found, repository asked for) and leaves the choice (delete the
folder, or type another URL).

When a step fails, the error screen has buttons (Tab or ←/→ to choose, `Enter` retries, `Ctrl+B` goes back):

| Button | Effect |
|--------|--------|
| **Set up access** | goes back to the Access step (first when git refused the access) |
| **Retry** | does the step again (default focus) |
| **Change the URL** | goes back to the repository input |
| **Skip the team** | gives the team up: every step of the group is skipped and the recap says "team not configured" (in the palette the button leaves the wizard) |

In `oh init`, skipping a step of the team (or "Skip the team") skips the **whole** group and the
rest of the installation goes on: `setup_done` is written, no team is added to `hub.toml`. In
the "team" setup type, where the provider is supposed to come from the team, the wizard asks
for it again after the group, then offers a solo space for the project (not created without
your yes).

### Phase 2 — Configuration (interactive wizard)

The wizard automatically detects the repository state and adapts behavior:

| Step | If repo is empty (1st member) | If repo already configured |
|------|------------------------------|---------------------------|
| **Global config** | Creates `config.toml` (stale_days, sessions) | Updates if changed |
| **Identity** | Registration of your card + commit-and-push | Update of **your** card (the one in `hub.toml`) + commit-and-push |
| **Notifications** | Mattermost webhook configuration | Updates if changed |
| **Policies** | Multi-select of recommended policies | Merges with existing |

#### Profile fields (Identity step)

| Field | Description | Example |
|-------|-------------|---------|
| Member ID | Unique key in members.toml | `benjamin` |
| Display name | How your name appears in notifications | `Benjamin` |
| GitLab handle | Optional. For the GitLab integration and the check of your token; **unique in the team** | `bdatiche` |
| Role | Your team role | `lead`, `dev`, or `reviewer` |
| Mattermost username | For mentions in notifications (`oh team init` and "Initialize the team" only) | `benjamin.datiche` |
| Tracker username | Your name on the tracker (`tracker_username`), same screens. **Required when the tracker is on another server than the team repository** (see below) | `benjamin.datiche` |

#### Your member card

Everyone creates **their own card** when joining: no lead validates it. The five paths ("Create" and
"Rejoin" of `oh init`, `oh team init`, "Initialize the team" and "Rejoin a team" of the palette) ask
the same form and write the same card. The defaults are aligned: the **creator** of the team is
`lead` (as the member of a solo space is: the first one, who sets the team up); whoever **joins** is
`dev` (the least power, one drop-down away from another role); the default mode of every new card is
`semi-auto`.

The card of a colleague is **never replaced**:

- **ID already taken**: refused, with the name of the colleague who has it. In "Create", the error
  screen offers **"That's me"**: the existing card is attached as it is to your `hub.toml`; its empty
  fields (name, handle, role) are filled, the filled ones do not change. "Change my identity" goes back
  to the form. In "Rejoin", picking your card in the list is that "That's me"; the form of a new card
  refuses a taken ID and points to the list. Your own ID (the one `hub.toml` already has for this team,
  on a reinstallation) stays accepted.
- **GitLab handle already carried by another member**: refused (case and `@` ignored), whatever the ID.
- Picking in "Rejoin" a card **with no handle** (made before the handle was asked) adds an optional
  "GitLab handle" step that completes it; without a handle the identity check of the token is skipped.
- The form of a new card starts from who you are in your other team (ID, name, handle), if any.
- When the list of members comes from a clone that could not be updated (offline), a warning sits
  above it: "The list may not be up to date: …".

#### Your name on the tracker (`tracker_username`)

The tracker sync attributes the issues to the members **by user name**: the `tracker_username` of the member's profile, else its `gitlab_username`. When the tracker is on another GitLab (or Jira) than the team repository, your name there is not necessarily the same (`benjamin.datiche` on one, `benjamin.datiche1` on the other): without `tracker_username`, no tracker issue is attributed to you. It is written to your profile in the team repository (`members.toml`, one commit) and set in these places:

| Where | How |
|-------|-----|
| `oh team init` → Identity step | "Tracker username" field (CLI and TUI) |
| `oh team rejoin` | the `--tracker-username <name>` option, or the question asked when the team tracker is on another server than the repository and your profile has no name |
| `oh init` wizard (team group, "rejoin") and TUI `team rejoin` | "Tracker name" step, same conditions |
| `oh team profile` | `oh team profile --tracker-username <name>`; without options, shows your profile; an option given empty clears the field |
| TUI, `team-detail` view | "My member profile" section: display name, GitLab, Mattermost and tracker names, saved at once (one commit), without `Ctrl+S` |

`oh doctor` checks it: ✗ when the tracker is on another server than the team repository and your `tracker_username` is empty; ⚠ (online) when the name the tracker returns for your token matches no member. The sync (`oh team sync-tracker`) also says so in its summary when issues are assigned to names unknown to the team and your profile has no `tracker_username`. See [`oh doctor`](../reference/cli-infra.en.md#oh-doctor).

#### Recommended policies (Policies step)

The wizard offers a recommended set to multi-select:
- **Branch naming** — `feat/fix/chore/.../[a-z0-9-]+` (enforcement: refuse)
- **Commit format** — Conventional Commits (enforcement: warn)
- **Max WIP** — 2 tickets per member (enforcement: warn)
- **Review required** — Review required before merge (enforcement: refuse)

### Result

At the end of the wizard:
1. `config.toml` is created/updated in the team-state repo
2. The member is registered in `members.toml` and committed + pushed
3. Notifications are configured (optional)
4. Policies are created in `policies.toml` and committed + pushed (optional)
5. `hub.toml` is updated with `[[teams]]` configuration

In TUI mode, a summary screen displays all configured values and offers
to navigate to the team detail view (`Enter`) or return home (`Esc`).

## 3. Configure notifications (reference)

> **Note:** Notifications are offered automatically during `oh team init`.
> This section describes how to modify them manually after initial setup.

Edit `config.toml` in the team-state repo (or `hub.toml` for hub-level config).

### Mattermost

```toml
[notification]
enabled = true
type = "mattermost"
mattermost_webhook = "https://mattermost.example.com/hooks/..."
channel = "#dev-ai"
bot_name = "OpenHub"
```

To get the webhook URL: Mattermost > Integrations > Incoming Webhooks > Add.

### Slack

```toml
[notification]
enabled = true
type = "slack"
webhook_url = "https://hooks.slack.com/services/T.../B.../..."
bot_name = "OpenHub"
```

To get the webhook URL: Slack > Your App > Incoming Webhooks > Add New Webhook to Workspace.

### Discord

```toml
[notification]
enabled = true
type = "discord"
webhook_url = "https://discord.com/api/webhooks/.../..."
bot_name = "OpenHub"
```

To get the webhook URL: Discord channel settings > Integrations > Webhooks > New Webhook.

### Microsoft Teams

```toml
[notification]
enabled = true
type = "teams"
webhook_url = "https://outlook.office.com/webhook/..."
```

To get the webhook URL: Teams channel > Connectors > Incoming Webhook > Configure.

### Multi-destination (notify multiple platforms simultaneously)

```toml
[[notification.destinations]]
type = "slack"
webhook_url = "https://hooks.slack.com/services/T.../B.../..."

[[notification.destinations]]
type = "discord"
webhook_url = "https://discord.com/api/webhooks/.../..."
```

Commit and push:

```bash
cd ~/.oh/team-state
git add config.toml
git commit -m "config: enable notifications"
git push
```

## 4. Use it in sessions

No redeploy is needed (`oh deploy` / `oh sync` removed in v5): from the next launch (`oh run <workflow>`), the `team` MCP server is put in the session bundle, making team tools available to AI agents.

## 4b. Per-project Team Configuration

By default, every project **inherits** the hub's team configuration (the `[team]` block in
`hub.toml`). Each project can independently
choose its team mode.

### Three modes

| Mode | Description |
|------|-------------|
| `inherit` | Use the hub's team-state repo and member ID _(default)_ |
| `custom` | Use a different team-state repo — member ID falls back to hub if unset |
| `disabled` | Explicitly opt out — MCP team not put in the session bundle |

### How the mode is applied

At launch, oh resolves the effective team config
(project override → hub fallback) and declares the `team` MCP server in the session
bundle with `OH_TEAM_ID` in its environment. The server reads `OH_TEAM_ID` to find the
team of the session project (no more `.opencode/team.json`).

When mode is `disabled`, the `team` MCP server is not put in the session bundle.

Outside a session (no `OH_TEAM_ID`), the team MCP server falls back to the active team
of `hub.toml`.

### Choosing the mode during project creation

`oh project add` (or the `oh init` wizard when creating the first project) includes a
**Team** step that presents the choice explicitly:

```
Hub team: git@gitlab.com:acme/team-state.git (member: alice)

Mode team:
  ▶ Use hub team (alice)          ← default when hub has a team
    Custom configuration
    No team for this project
```

If the hub has **no team configured**, the step defaults to *"No team for this project"*
and offers opt-in to a custom configuration.

### Changing the mode after creation

**From the TUI omnibar** (available at any time, regardless of hub team status):

```
team configure
```

This opens a modal with the three choices. The change is taken into account at the
next session launch (bundle rebuilt) — no redeploy needed.

### Custom team: state path resolution

For `custom` mode, `StatePath` is auto-derived from the remote URL if not set explicitly:

```
git@gitlab.com:acme/other-team.git  →  ~/.oh/team-states/other-team
https://github.com/acme/my-team.git →  ~/.oh/team-states/my-team
```

Each distinct team-state remote gets its own local clone directory under
`~/.oh/team-states/` to avoid collisions.

### Member ID resolution

For `custom` mode, if `MemberID` is left blank, it falls back to the hub's `member_id`.
This is the recommended approach when a developer participates in multiple teams using
the same identity.

```toml
# hub.toml — global identity
[team]
enabled = true
state_repo = "git@gitlab.com:acme/main-team.git"
member_id  = "alice"           # ← used as fallback for all custom projects

# per-project override stored in SQLite (team_config column):
# { "mode": "custom", "state_repo": "git@github.com:beta/other.git" }
# member_id is empty → runtime resolves to "alice" from hub
```

### Troubleshooting

**Clone fails with "terminal prompts disabled" or "Authentication failed":**

The remote requires authentication and git has nothing to answer with. `oh` sets
`GIT_TERMINAL_PROMPT=0` to prevent blocking on a prompt — the message is intentional. In the
wizard, the **Access** step (or the "Set up access" button of the error screen) asks for the
token and keeps it in the keychain of `oh`.

Solutions:
- **HTTPS:** give the token to the wizard (Access step), or store it by hand:
  `oh secrets set openhub.git.gitlab.com/org/team-state.token` (and `…/team-state.user` if the
  user name is not `oauth2`). A git `credential.helper` that already answers is still used
  when `oh` has no token for that repository.
- **SSH instead of HTTPS:** use `git@gitlab.com:org/team-state.git`
- **SSH key not loaded:** run `ssh-add ~/.ssh/id_ed25519` or `eval "$(ssh-agent)"`

**The folder holds the clone of another repository:**

An existing clone whose origin is not the URL typed is never reused. Delete the folder (or pick
the right repository). Teams configured before 5.3.2 keep their `state_path` (the former
`~/.oh/team-state/` included): nothing is moved.

**Clone times out after 30 seconds:**

The remote host is unreachable (network, firewall, or wrong URL). The setup will
surface an error toast after the timeout.

**Error toast is truncated:**

Open the full error via the omnibar: type `notifications` (or `notif`, `logs`).
The Notifications view loads the last 50 entries from `~/.oh/notifications.jsonl`
(persistent across sessions) — the complete, untruncated message is always available.

**TUI appears frozen during setup:**

This should no longer occur after the Phase 3 deadlock fix. If it does, press
`Ctrl+C` to exit and check `~/.oh/team-states/` to see if the clone was attempted.
The error will appear in the Notifications view on the next launch (`notifications` in
the omnibar).

## Daily Usage

### Claims — Ticket reservation

Claims follow a 6-status lifecycle:

```
oh team claim --planned → planned → (ticket started) → in_progress → review → (validation) → done
                                                              ↘ blocked
```

| Status | Column | Description |
|--------|--------|-------------|
| `planned` | TODO | Reserved but not yet started |
| `in_progress` | IN PROGRESS | Actively being worked on (default when claiming) |
| `review` | REVIEW | Awaiting review |
| `validation` | VALIDATION | Review passed, being validated (test, QA, pre-production) |
| `blocked` | BLOCKED | Blocked |
| `done` | DONE | Completed (kept until retention period expires) |

```bash
# Reserve a ticket and start immediately (status: in_progress)
oh team claim SRU-142

# Reserve for later without starting (status: planned, shows in TODO column)
oh team claim SRU-142 --planned

# With associated branch
oh team claim SRU-142 --worktree feat/SRU-142-user-auth

# Release when done
oh team release SRU-142

# Transfer to another member
oh team claim transfer SRU-142 --to alice
```

The table of the statuses, what puts a ticket in REVIEW, the lease of a claim and what becomes of the claim when a session ends are described at the end of this guide: [Status model](#status-model) and [Lease and end of session](#lease-and-end-of-session).

> **Note:** Starting a session on a ticket already claimed as `planned` with `oh run ticket --tickets <id>` automatically transitions it to `in_progress`. Otherwise, change the status from the team board.

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

By default the board displays 6 columns: **TODO** · **IN PROGRESS** · **REVIEW** · **VALIDATION** · **DONE** · **BLOCKED**. The columns and the mapping of the tracker labels are edited in the TUI editor, from the team home (*Board Columns*) or with `C` on the team board (`n` add, `e` rename, `d` delete after a confirmation, `o` role, `K` / `J` order, `m` mappings, in which `v` moves a label or a status to another column, `Ctrl+S` save): see [the columns editor](../reference/tui.en.md#board-columns-editor). Back from the editor, the board reads its tickets again with the new columns.

The **role** of a column decides what `a` offers on its cards: `initial` (frame, do the ticket), the first `active` column (do the ticket, instruct the running session), `review` (review of the claim's branch, review feedback of its MR), the others (detail, status, transfer). The `review` role is new: give it to the column where work waits for a review. A team that has not set it keeps the expected behaviour, because the column whose id is `review` (the one of the default board) stands in; a `review` column does not count towards `max_in_progress_per_member`. See [`[board]`](../reference/config.en.md#board-section).

Keyboard shortcuts in the board (and other team views):

| Key | Action |
|-----|--------|
| `h` / `l`, `j` / `k` | Columns, cards |
| `Enter` | Detail of the ticket |
| `a` | Actions of the column of the card: frame, do the ticket, review, instruct the session… (see [the `a` menu](../reference/tui.en.md#the-a-menu-of-a-card)) |
| `[` / `]` | Move between the projects |
| `/` | Filter: text, **My tickets**, **Unassigned** (the pool, to take with `c`), assignee, label, clear the filters |
| `c` | Take the ticket (refused at `max_in_progress_per_member`) |
| `L` | Release the selected claim |
| `t` | Transfer the selected claim |
| `s` | Change the status (refused at the limit when it puts your card in progress) |
| `C` | Edit the columns of the board and the labels mapped to them |
| `r` | Sync the tracker, pull and refresh |

> **Async pull:** all team views (board, status, activity, etc.) automatically pull git on open and on `r`. A toast is only shown if it takes more than 1 second.

### External tracker sync

```bash
oh team sync-tracker   # sync claims with external tracker (GitLab/Jira)
```

Pulls issue states from the tracker and updates claim statuses:
- Closed issue → claim transitioned to `done`
- Reopened issue → claim transitioned back to `in_progress`
- Labels are mirrored
- With `auto_plan_assigned = true`, every open issue assigned to a member comes onto the board as a claim, in the column its status maps to, with no cap per member
- With `auto_plan_unassigned = true`, the issues that carry **any** of the `unassigned_labels` come onto the board as pool claims (to take with `c`), or as claims of the member they are assigned to. The pool is first scanned in full over as many runs as it needs, each creating at most `max_unassigned_issues` claims (a pool of 70 with the default 20 takes four runs); then the runs only look at what was updated. Changing `unassigned_labels` scans it in full again. A claim you released stays released during the scan
- The column of an issue comes from `label_status_mapping`, then `status_mapping`. With several mapped labels, a blocked column always wins, otherwise the most advanced column (the furthest in the order of the board), then the alphabetical order of the labels: an issue "Blocked" + "Ready to dev" is blocked, and one that still has an old "Ready to dev" next to "Dev doing" stays in progress. A value that is not a status of the board (`todo` on a board without a todo column) is read as the status of the initial column; `oh doctor` names it and `--fix` rewrites it (see [`[tracker]`](../reference/config.en.md#tracker-section))
- If issues are assigned to names that belong to no member while your profile has no `tracker_username`, the sync summary warns you and gives the command (`oh team profile --tracker-username <name>`)

### Wiki management

```bash
# Review pending wiki proposals (from AI agents); refusing one asks for a reason
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
├── config.toml           # Notification + takeover config + [governance]
├── workflows.lock        # Integrity of the published workflows
├── workflows/            # v5 workflows (see team-workflows.en.md)
├── catalog/              # Team bricks (agents, skills), committed with the structure
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
│       ├── workflows/            # Project workflows
│       └── policies-override.toml  # Per-project overrides (optional)
├── wiki/
│   ├── .pending/         # Proposals awaiting review
│   ├── decisions.md      # Architectural decisions
│   └── patterns.md       # Recurring patterns
└── reports/
```

`oh team init` creates the folders of this structure (each with a `.gitkeep`) and **commits** them in one "team: init structure" commit, pushed like the other writes of the team; `catalog/` (the shared catalogue of team bricks) is part of it and is therefore not left untracked in your clone. Nothing is committed when everything is already tracked, and a commit that cannot be made (offline) is retried by the next command.

The [Team workflows](team-workflows.en.md) guide covers `workflows/`, `workflows.lock` and governance (`[governance]` in `config.toml`).

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
oh team claim transfer SRU-142 --to alice
# → Takeover brief generated. oh takeover-brief show SRU-142
```

If the claim of another member has expired (its lease is past, see
`lease_hours`; without a lease, `stale_days` days of inactivity in
`config.toml`), `oh team claim` offers to take it over: after the confirmation
(`--yes` without a terminal), a takeover brief is generated and the claim comes
to you with a new lease:

```bash
oh team claim SRU-142
# → SRU-142 is claimed by benjamin, but its lease expired on 22/10 14:30.
# → Take over myproject/SRU-142 from benjamin?  Yes / No
```

### View and enrich briefs

```bash
# Show a ticket's brief
oh takeover-brief show SRU-142

# List all briefs for the project
oh takeover-brief list

# Enrich a brief with AI code analysis
# (or from the command line, without saving the brief:)
oh run brief-enrich --headless -i ticket=SRU-142
```

The enrichment runs the `brief-enrich` workflow without interface (`oh run brief-enrich --headless -i ticket=<id>`; the workflow reads the brief from the team space itself); the Takeover view of the TUI (key `e`) then saves the enriched brief in team-state (`oh takeover-brief enrich`, removed in 5.1, did it from the command line). The `brief-enricher` agent is used to:
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

`oh run ticket --tickets` launches **one session per ticket**, in a single server group; each session that writes works in its own Git worktree. The former parallel mode (`oh start --parallel`, full-screen monitor, merge view) was removed in v5: see [Parallel mode — replaced in v5](parallel-mode.en.md).

### Launch

```bash
# Launch 3 tickets, one session per ticket
oh run ticket --tickets bd-42,bd-43,bd-44

# All tickets in a single session
oh run ticket --tickets bd-42,bd-43,bd-44 --one-session
```

`oh start --parallel --tickets …` was removed in 5.1: use `oh run ticket --tickets …`. `--priority` and `--max-sessions` no longer have any effect; to limit the sessions working at the same time, use the `max_active_sessions` restriction (`oh budget set`, see [v5 sessions › Restrictions](sessions-v5.en.md#restrictions)).

### Monitoring

Sessions show up in the TUI **Sessions** view (omnibar `sessions`) and in `oh session list`: state, waiting decisions, live feed, results (`oh session results <id> --mr`). See [v5 sessions](sessions-v5.en.md).

### Merge

oh no longer proposes a merge: each session leaves its branch, ready for an MR/PR.

### Configuration

In `config.toml` of the team-state repo:

```toml
[claim]
done_retention_days = 7    # Days before done claims are cleaned up

[tracker]
enabled = true
type = "gitlab"             # "gitlab" or "jira"
auto_sync = true            # sync when opening team views
sync_interval_minutes = 5
auto_plan_assigned = true   # every assigned issue comes onto the board
max_in_progress_per_member = 5   # tickets a member has in progress at once (0 = no limit)
push_labels = true          # push hub labels back to tracker (requires write_enabled)
ticket_patterns = { "T-SRU" = "SRU-(\\d+)" }

[tracker.projects]
"T-SRU" = "42"              # hub project ID → GitLab project ID/path
```

> **Note:** Connection credentials (token, URL) are reused from `[mcp.gitlab]` / `[mcp.jira]` in `hub.toml` — no duplication. The GitLab token is looked for in a single order for every command (`GITLAB_TOKEN`, `tracker_token_key`, project, team, MCP, former tracker key): see [`oh team sync-tracker`](../reference/cli-team.en.md#oh-team-sync-tracker). `oh mr create` finds the team token too.
>
> **Two GitLab instances (team repository and tracker).** Each token is only sent to the server it is declared for: the team repository token (`openhub.team.<team>.gitlab.token`) only serves the GitLab of the repository (the host of the `state_repo`, or the declared `gitlab_url` when the repository is reached through an SSH alias). For the tracker, store its token (`oh secrets set <key>`), then set the key in `[tracker]` of the team config or of `hub.toml` (`oh config set tracker.tracker_token_key <key>`, the `oh team config` → Tracker sync assistant, the `oh init` and `oh team rejoin` wizards, or the `team-detail` view of the TUI, whose "Tracker token" field shows the key and server really declared — "not configured" otherwise — and keeps the token under that key): it is valid for the tracker with no further declaration; `tracker_token_url` is only needed when the token is for another server. An MCP token without `mcp.gitlab.url` is valid for `https://gitlab.com` only; set `mcp.gitlab.url` for another GitLab. `GITLAB_TOKEN` still needs `OH_GITLAB_URL` (or `GITLAB_URL`). If `state_repo` goes through an SSH alias (`git@my-alias:…`), declare the real GitLab of the repository (`gitlab_url` of your team in `hub.toml`, asked by `oh init` and `oh team rejoin`, or `--gitlab-url`): the team token is then valid for that GitLab and not for the alias. See [the server of each GitLab token](../reference/config.en.md#the-server-of-each-gitlab-token).
>
> **`enabled = false`:** a `[tracker]` section of the local `hub.toml` with `enabled = false` turns the sync off in the command, the team screens (`s`) and the automatic sync. Since v5.0 the screens honor it too.

## 9. Team Lifecycle Management

### Multi-team management

List all configured teams:

```bash
oh teams list
```

Add a new team:

```bash
oh teams add --repo git@gitlab.com:org/team-state.git --member-id alice
```

Remove a team (detaches all associated projects):

```bash
oh teams remove <team-id>
```

### Detach a project from its team

Remove the team affiliation from a project without deleting the team itself:

```bash
oh teams detach <project-name>
```

### Archive / Restore a team

Temporarily disable a team without removing it. Archived teams are ignored by all commands and agents:

```bash
oh teams archive <team-id>
oh teams restore <team-id>
```

Archiving sets `enabled = false` in hub.toml. Restoring re-enables it.

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
3. For Mattermost: verify the channel exists
4. For Slack/Discord/Teams: test the webhook URL with `curl -X POST -d '{"text":"test"}' <webhook_url>`
5. Check `oh team status` works (confirms repo access)

## Status model

A ticket exists in two places: in **Beads** (or the tracker), which `oh board` and the TUI Board read, and in a **claim** of the team-state, which the team board reads. Each has its own statuses; this table (default columns) says how they match.

| Column | Beads ticket | Claim (team-state) |
|---|---|---|
| TODO | `open` | `planned` |
| IN PROGRESS | `in_progress` | `in_progress` |
| **REVIEW** | **`in_progress` + label `ready-for-review`** | **`review`** |
| VALIDATION | custom status (`validation`, `testing`, `qa`, `pre-release`) | `validation` |
| BLOCKED | `blocked` | `blocked` |
| DONE | `closed` | `done` |

The contract:

- **In Beads, REVIEW is a label, not a status.** Beads 1.3 has no `review` status (unless the project declares one with `bd config set status.custom`). An agent that has finished adds the label (`bd update <ID> --add-label ready-for-review`) and leaves the status at `in_progress`; the review removes the label to send the ticket back to development, or closes the ticket. `oh board` and the TUI Board put an `in_progress` ticket carrying this label in REVIEW.
- **On the claim side, REVIEW is the `review` status.** It is set when a review is published (`oh mr create`), from the team board, or by the MCP review verdict (approved: `review` then `done`; rejected: back to `in_progress`). The team board shows this status.
- **The two are not copied into each other.** Adding the label in Beads does not change the status of the claim, and moving the claim to `review` does not add the label.
- **A single equivalence, at the end of a session**: for `teamstate.DecideClaimEnd`, the `ready-for-review` (or `review`) label, on the ticket or on the claim, is worth the `review` status of the claim. Either way the claim outlives the session (see below).
- **A remote session does not write a status the project does not know.** The fake `bd` of the job refuses at once, with the error message of `bd` (exit code 1, before any write), a `bd update` or `bd create` whose `--status` is not in the Beads configuration of the project (the statuses are in the snapshot sent to the job; the case must match exactly). A remote agent that writes `--status review` with no configured `review` status is therefore stopped, instead of failing at replay; it must use the label.

## Lease and end of session

A claim being worked on has a **lease**: a deadline (`lease_until` in the claim file) after which it is *expired*. Only a claim with an owner and work started has a lease: a pool claim (no owner), a `planned` one, one in the initial column or a finished one has none, and neither `oh doctor` nor the synchronization ever judges it expired. **A claim in review has none that can run out either** (see *Claim in review* below).

**Settings** (`[claim]` section of the team-state `config.toml`, editable in `team-detail`):

| Key | Default | Bounds | Role |
|---|---|---|---|
| `lease_hours` | 24 | 1 to 168 | duration of the lease of a claim worked on by a session |
| `heartbeat_minutes` | 60 | 0 (off) or 5 to 1440, at most half the lease | period of the signal of life of the daemon that renews the leases |
| `sleeping_renewal_days` | 3 | 1 to 90 | a session asleep for longer no longer renews |
| `remote_renewal_days` | 7 | 1 to 90 | a remote session launched longer ago no longer renews |
| `lease_hours_manual` | 72 | `lease_hours` to 720 | duration of the lease of a claim taken by hand |
| `release_expired_on_sync` | `false` | | the tracker synchronization releases the expired claims |

**Session lease.** When a session starts on a ticket, the claim gets the session and a lease of `lease_hours`; if you already held the claim (taken by hand, or kept by a previous session), the session takes it over and the lease starts again from now. The daemon (`ohd`, or the `oh` process on Windows) sends a signal of life every `heartbeat_minutes`: it renews the lease of the claims of the open sessions (neither completed, failed nor stopped; a sleeping session is open) when less than half of the lease is left, in one commit per team repository (none while the leases are long). With `heartbeat_minutes = 0` there is no renewal: the leases are then only extended by a session that takes the claim over or by `oh team claim`.

**Renewal ceilings.** So that a claim does not outlive for ever a session nobody comes back to, the signal of life stops: for a sleeping session, `sleeping_renewal_days` days after it was put to sleep; for a remote session, `remote_renewal_days` days after it was launched. The lease is no longer extended, the claim expires normally and `oh doctor` reports it.

**Claim taken by hand.** A claim without a session (`oh team claim`, the team board, the tracker plan) has a lease of `lease_hours_manual` that nothing renews on its own. `oh team claim <ticket>` on your own claim renews it, expired or not, without confirmation (`lease_hours_manual` when no session is attached to the claim, `lease_hours` otherwise). Starting a session on the ticket replaces this lease with the one of the session.

**Expiry and takeover.** A claim is expired when its lease is past; a claim that should have a lease but has none (written before v5.1) is expired when it has been inactive for more than `takeover.stale_days` days (3 by default, `[takeover]` section of the same file). For a takeover, `oh team claim` applies this inactivity rule to any claim without a lease. An expired claim is not deleted by itself:

- `oh doctor` (`claims_expired`, a warning that does not change the exit code) lists your expired claims and those the end of a session labelled `session-ended`, with the command to keep them (`oh team claim <ticket>`) or release them (`oh team release <ticket> -p <project>`);
- another member can take it over with `oh team claim <ticket>`: after the common confirmation (`--yes` without a terminal, otherwise refused, exit code 2), a takeover brief is generated and the claim comes to them with a new lease;
- if the team turns on `release_expired_on_sync`, the tracker synchronization releases the expired claims (one `release: N expired claim(s)` commit). A claim in review (`review` status or `review` / `ready-for-review` label) is never one of them.

**End of a session.** When a session ends, `teamstate.DecideClaimEnd` decides, for each claim of its tickets (those of another member or of another session are left as they are), with the first rule that applies:

| Case | Decision |
|---|---|
| the claim is already finished | kept (the retention of the finished claims cleans it up) |
| the ticket is closed | the claim moves to the terminal status |
| the ticket or the claim is in review (label `review` or `ready-for-review`, or `review` status) | **kept** |
| a checkpoint waits for a decision | kept, label `session-ended` |
| the commit checkpoint (`cp-2`) was put off while the session left work (or that cannot be told) | kept, label `session-ended` |
| the state of the checkpoints cannot be read | kept, label `session-ended` |
| otherwise | released |

A kept claim is no longer renewed by the session that ended: it expires when its lease ends if nobody takes it up again (`oh team claim <ticket>` renews it; so does a new session on the ticket, which also removes the `session-ended` label). A claim in review is the exception: it does not expire (next paragraph).

**Claim in review.** A claim whose status is `review` or that of a column with the `review` role (whatever its ID), or that carries the `review` or `ready-for-review` label, **does not expire**, whatever its lease: it waits for a person (the reviewer, the merge), not for a session. It stays reserved until the merge (the claim is then finished) or until its owner releases it (`oh team release <ticket> -p <project>`); neither `oh team claim` by another member nor `release_expired_on_sync` takes it. If the review sends the work back (status `in_progress`, label removed) and its lease is past, it gets a new one from that moment. `oh doctor` (`claims_review`, a warning) lists your claims that have been in review for longer than `takeover.stale_days` days, without calling them expired, with the command to release them if the work is dropped.

**Durable queue.** What the end of a session decides (release, complete, label `session-ended`) and the lease renewals that could not be pushed are first written to a local queue, `.git/oh-claims-queue.json` of the team-state clone (never committed). Each operation carries its conditions: the claim must still belong to the same member and the same session, otherwise it is dropped and logged. The queue is applied in one commit (one pull, one push) as soon as the team repository answers: at the end of the session (a wait bounded to 15 s, 10 s of which for the lock of the clone; the daemon waits up to 2 min), at each beat of the daemon, and by the `claims-queue` task of the daemon (when it starts, then every 15 minutes). Offline, or if the repository stays busy, nothing is lost: the operations stay in the queue and oh says so at the end of the session. A session that starts offline on a ticket you already held takes the claim over when the network is back; what the previous session had queued on that claim is cancelled. `oh doctor` (`claims_queue`) warns when operations have been waiting for more than 15 minutes, or when the file cannot be read (it is then set aside, not deleted); `oh doctor --fix` applies the queue after a confirmation.

**A claim released on purpose stays released.** The tracker synchronization does not take a claim released by its owner (`oh team release`, end of a session, board, retention of the done claims) for a claim lost by accident and no longer recreates it. An issue assigned to a member that was released comes back as a `planned` claim only if it is updated on the tracker afterwards.

**Work-in-progress limit.** The sync puts **every** open issue assigned to a member on the board, with no cap per member: the board reflects the tracker. What bounds a member is the number of tickets they work on at once, `max_in_progress_per_member` (5 by default; `0` = no limit; `[tracker]` of the team `config.toml`, or `[tracker]` of the member's `hub.toml`, which wins). It counts the member's claims in progress (the `active` columns of the board, `in_progress` by default); a claim in review (`review` status or `review` / `ready-for-review` label) or in validation, a planned, blocked or finished claim, and a ticket of the pool do not count. At the limit, oh **refuses** that the member takes or starts one more ticket, with the count and the key: `oh team claim` (also the takeover of an expired claim and `oh team claim transfer` to oneself), the take key (`c`) of the team board, a card moved to an active column or handed to oneself on the board, and a session launched on tickets (local or remote, before anything is created). Handing a claim to another member and the merge-request review verdict (a rejected review sends the claim back to `in_progress`) are not checked. The sync never refuses anything: a ticket already in progress on the tracker arrives in progress, even past the limit, and counts from then on. This limit replaces `max_auto_plan_per_member`, which capped what a sync created: it is **deprecated**, still read when `max_in_progress_per_member` is absent (and `oh config set tracker.max_auto_plan_per_member` writes the new key), reported by `oh doctor` with the command to run, and dropped by the next write of `config.toml` or `hub.toml`. It is a different setting from the `max_ticket_wip` policy ([team conventions](team-conventions.en.md)), which only checks `oh team claim`.

**Other reports.** `oh doctor` also reports orphan claims (ticket closed, or every session that worked on it ended) with the command to run; claims taken under the former ID of a recreated project are attached to the local project by `oh doctor --fix` (see [`oh doctor`](../reference/cli-infra.en.md#oh-doctor)). A forgotten claim is freed by hand: `oh team release <ticket-id> -p <project>`.
