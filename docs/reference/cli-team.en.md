> [Lire en francais](cli-team.fr.md)

# CLI Reference — Team

Team features rely on a shared **team-state** git repository (claims, events, wiki, policies, patterns, team workflows). See [Team workflows](../guides/team-workflows.en.md) for workflows and [ADR-040](../architecture/adr/040-workflows-team-state-governance.en.md).

## Collaboration (oh team)

### oh team init

Interactive wizard configuring the team, in 4 adaptive steps: global config (takeover, sessions), identity (team registration), Mattermost notifications (optional), team policies (automated conventions). The team-state repository must exist beforehand on GitLab or GitHub; when it is already configured, only the relevant steps are offered.

```
oh team init [--solo [--id <id>] [--name <name>] [--member-id <id>] [--project <project>]]
```

#### Solo space (`--solo`)

For a project without a team: creates a local team-state (git repository without remote) in `~/.oh/teams/<id>/` that holds the project workflows. The only member has the `lead` role. Team features (board, claims, notifications, `team` MCP server) stay off; the active team is never a solo space.

```bash
oh team init --solo --project web-app
```

| Flag | Type | Description |
|------|------|-------------|
| `--solo` | bool | Create a solo space instead of running the wizard |
| `--id` | string | Space id (default: `solo`) |
| `--name` | string | Display name |
| `--member-id` | string | Member id (default: the one of another team, else the system user) |
| `--project` | string | Project to attach (id or name; refused if it already has a team) |

In `hub.toml`, the space is appended to the existing teams with `solo = true` (no `state_repo`).

### oh team promote

Shares a solo space: adds the remote and pushes the whole history. The id, folder, history, attached projects and published workflows are kept.

```bash
oh team promote --remote git@gitlab.com:acme/team-state.git
```

| Flag | Type | Description |
|------|------|-------------|
| `--remote` | string | URL of an **empty** git repository (required) |
| `--team` | string | Solo space to share, when you have several |

On failure (missing or non-empty repository, access denied), the space stays solo. Other members then join the team with `oh team init` and enter the repository URL.

### oh team rejoin

Rejoins an existing team after reinstalling the hub: clones the team-state repository, verifies the identity through GitLab and restores the configuration. Do not run it while the TUI performs team operations on the same clone (the lock does not protect against another process).

```
oh team rejoin [--repo <url>] [--member-id <id>] [--no-retro-tag]
```

| Flag | Type | Description |
|------|------|-------------|
| `--repo` | string | Team-state git repository URL |
| `--member-id` | string | Member id (interactive if omitted) |
| `--no-retro-tag` | bool | Do not link existing sessions to your identity |

### oh team config

Wizard configuring the shared MCP services (GitLab, Jira…) and the tracker sync, at team and/or local level. `oh team config status` shows the effective configuration (team + local resolution).

```bash
oh team config
oh team config status
```

---

### oh team status

Shows who is working on what: active claims, members, tickets in progress.

```
oh team status [--detail]
```

| Flag | Type | Description |
|------|------|-------------|
| `--detail` | bool | Show sub-tickets and progress |

---

### oh team activity

Team activity log.

```
oh team activity [options]
```

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--limit` | int | `20` | Maximum number of events |
| `--member` | string | | Filter by member |
| `--project` | string | | Filter by project |
| `--today` | bool | `false` | Today's events only |
| `--week` | bool | `false` | Last 7 days |

```bash
oh team activity --today
oh team activity --member alice --limit 50
```

---

### oh team board

Full-screen team kanban board (current, planned and completed tickets).

```
oh team board [--watch]
```

| Flag | Type | Description |
|------|------|-------------|
| `--watch` | bool | Automatic refresh (`r` key to refresh by hand) |

---

### oh team claim

Claims a ticket in the team-state to tell the team you are working on it. When the ticket is already taken, a warning is shown (non-blocking). When the ticket has been inactive for several days, oh offers to generate a takeover brief before claiming it.

```
oh team claim <ticket-id> [options]
```

| Flag | Short | Type | Description |
|------|-------|------|-------------|
| `--planned` | | bool | Create the claim in `planned` status (TODO column) instead of `in_progress` |
| `--project` | `-p` | string | Project (detected from the current folder if omitted) |
| `--worktree` | | string | Name of the associated branch or worktree |

**Example:**

```bash
oh team claim TICKET-123
oh team claim TICKET-123 --planned
oh team claim TICKET-123 --worktree feat/ticket-123
```

### oh team claim transfer

Changes the owner of an existing claim without releasing it.

```
oh team claim transfer <ticket-id> --to <member-id> [-p <project>]
```

| Flag | Short | Type | Description |
|------|-------|------|-------------|
| `--to` | | string | Target member (required) |
| `--project` | `-p` | string | Project (detected from the current folder if omitted) |

```bash
oh team claim transfer TICKET-123 --to bob
```

### oh team release

Releases a claimed ticket (it becomes available to the other members again).

```
oh team release <ticket-id> [-p <project>]
```

| Flag | Short | Type | Description |
|------|-------|------|-------------|
| `--project` | `-p` | string | Project (detected from the current folder if omitted) |

```bash
oh team release TICKET-123
```

---

### oh team sync-tracker

Synchronizes the claims with the external tracker (GitLab Issues, Jira, Linear): pulls the issue states, updates the claim statuses, mirrors the labels and creates `planned` claims for assigned issues not yet claimed. No flags; the configuration comes from `[tracker]` in the team-state `config.toml`, completed by `[tracker]` of the local `hub.toml`.

```bash
oh team sync-tracker
```

---

### oh team notify test

Sends a test message to the webhook(s) configured in the team-state.

```
oh team notify test [-m <message>]
```

| Flag | Short | Type | Description |
|------|-------|------|-------------|
| `--message` | `-m` | string | Custom message (default: standard test message) |

```bash
oh team notify test
oh team notify test --message "Deployment in progress"
```

---

### oh team wiki

Team wiki: pages and proposals.

```bash
oh team wiki list                  # pages and pending proposals
oh team wiki read <page>           # read a page
oh team wiki review [id]           # review a proposal (interactive without id)
```

---

## Team Management (oh teams)

A user can belong to several teams; each project is linked to 0 or 1 team.

### oh teams list

List the configured teams.

```bash
oh teams list
```

### oh teams add

Add a team to the hub.

```bash
oh teams add --repo git@github.com:org/team-state.git --member-id alice
```

| Flag | Type | Description |
|------|------|-------------|
| `--repo` | string | Team-state git repository URL (`git@…` or `https://…`) |
| `--member-id` | string | Your member id in the team |
| `--id` | string | Local team id (derived from the repository if omitted) |
| `--name` | string | Display name (optional) |

### oh teams remove

Remove a team from the hub; attached projects become projects without a team.

```
oh teams remove [team-id] [-f]
```

| Flag | Short | Type | Description |
|------|-------|------|-------------|
| `--force` | `-f` | bool | Remove without confirmation |

### oh teams detach

Detach a project from its team without deleting the team.

```bash
oh teams detach [project-name]
```

### oh teams archive / restore

Archive a team (disabled, configuration and local team-state kept) or reactivate it (and sync its team-state):

```bash
oh teams archive [team-id]
oh teams restore [team-id]
```

---

## Governance

### oh conventions check

Checks the current branch and the latest commits against the conventions of the project wiki (`docs/wiki/technical/conventions.md`) and of the team wiki. Non-blocking warnings.

```bash
oh conventions check
```

### oh patterns

Library of decomposition patterns, stored in the team-state.

```bash
oh patterns list [--all] [--tags a,b]   # validated patterns (--all: unvalidated ones too)
oh patterns show <name>                 # show a pattern
oh patterns add [file]                  # add (interactive or from a file)
oh patterns validate <name>             # validate a pattern proposed by an agent
oh patterns remove <name>               # remove a pattern
```

### oh policies

Team rules applied to every project (team-state `policies.toml`, overridable per project in `policies-override.toml`).

```bash
oh policies list [-p <project>]                                      # active policies (merged view with -p)
oh policies check [-p <project>] [--branch <name>] [--commit <msg>]  # check the current state
oh policies add                                                      # add a policy (interactive)
```

---

## Takeover Briefs

### oh takeover-brief

Takeover briefs generated during ticket transfers. **Alias:** `oh tb`. Every subcommand accepts `-p, --project <project>`.

```bash
oh takeover-brief show <ticket-id>    # show the takeover context
oh takeover-brief list                # list the briefs
oh takeover-brief enrich <ticket-id>  # deprecated alias: runs `oh run brief-enrich --headless` with the brief and saves the result
```
