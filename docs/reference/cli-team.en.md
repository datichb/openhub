> [Lire en francais](cli-team.fr.md)

# CLI Reference — Team

> **Beta (v5.0).** The team mode is published as a beta: it works, but a few limits are known (a claim released when its session stops, no expiry, a release that can be lost). Read the [known limits](../guides/team-setup.en.md#known-limits) before relying on it.

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
oh team rejoin [--repo <url>] [--member-id <id>] [--gitlab-url <url>] [--tracker-username <name>] [--no-retro-tag]
```

| Flag | Type | Description |
|------|------|-------------|
| `--repo` | string | Team-state git repository URL |
| `--member-id` | string | Member id (interactive if omitted) |
| `--gitlab-url` | string | Real GitLab of the repository when its address is an SSH alias (`git@my-alias:…`); saved in `hub.toml` (`gitlab_url`) and used for the identity check |
| `--tracker-username` | string | Your user name on the tracker, written to your member profile in the team repository (empty = unchanged). Without the option, the command asks for it when the team tracker is on another server than the team repository and your profile has no name (Enter = leave empty) |
| `--no-retro-tag` | bool | Do not link existing sessions to your identity |

A name written with a leading `@` is saved without it (`@benjamin.datiche` → `benjamin.datiche`). If the write fails (offline), the rejoin succeeds anyway and the command to run again is shown: `oh team profile --tracker-username <name>`. The HTTPS step of the wizards (repository token) is skipped when the cloned repository answers `git ls-remote` without a token.

The `oh init` (team group) and `oh team rejoin` (TUI) wizards ask for the GitLab of the repository (optional) and, if you keep the team's tracker while no token serves that tracker's server, where your token is: keychain key, server and token. The key and the server are written to `[tracker]` of `hub.toml` (`tracker_token_key`, `tracker_token_url`) when the configuration in force does not already give them; the default GitLab key (`openhub.tracker.gitlab.token`) needs no declaration, the Jira one does.

### oh team profile

Shows or changes **your member profile** in the team repository (`members.toml`): display name, GitLab, Mattermost and tracker user names. Without options, it shows the profile; with an option, it changes it in **a single commit** (pushed) and leaves the other fields alone.

```
oh team profile [--team <id>] [--display-name <name>] [--gitlab-username <name>] [--mattermost-username <name>] [--tracker-username <name>]
```

| Flag | Type | Description |
|------|------|-------------|
| `--team` | string | Team (identifier); defaults to the hub team |
| `--display-name` | string | Display name (cannot be empty) |
| `--gitlab-username` | string | User name on the GitLab of the team repository (identity check at rejoin) |
| `--mattermost-username` | string | Mattermost user name (mentions of the notifications) |
| `--tracker-username` | string | User name on the tracker (GitLab or Jira) |

An option given **empty** clears the field (`--tracker-username ""`); a leading `@` is removed from GitLab and tracker names. `tracker_username` is required when the tracker is on another server than the team repository: without it the sync (`oh team sync-tracker`) attributes no tracker issue to the member (empty, the GitLab name is used). `oh doctor` checks it ("Tracker name" and "Identity on the tracker" checks). The role and the identifier of the member are not changed here (`oh team rejoin`). In the TUI, the `team-detail` view has the same profile ("My member profile" section).

```bash
oh team profile --tracker-username benjamin.datiche
```

### oh team config

Wizard configuring the shared MCP services (GitLab, Jira…) and the tracker sync, at team and/or local level. `oh team config status` shows the effective configuration (team + local resolution).

For *Tracker sync*, the wizard asks where the tracker token is: the keychain key (`tracker_token_key`) and, for GitLab, the server it is valid for (`tracker_token_url`, empty = the tracker's server). **It no longer fills in the key by itself**: a key left empty stays empty, and the wizard says what is read then (GitLab: `openhub.tracker.gitlab.token` for the tracker's server; Jira: nothing). At local level the personal declaration replaces the team's (key and server together; `-` clears), and, if the team repository is reached through an SSH alias, the real GitLab of the repository (`gitlab_url`).

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

Claims a ticket in the team-state to tell the team you are working on it. When the ticket is already taken, a warning is shown (non-blocking). When the claim of another member has expired (its lease is past; without a lease, inactive for `takeover.stale_days` days), you can take it over: after the confirmation, a takeover brief is generated and the claim comes to you with a new lease. When the expired claim is yours, its lease is renewed.

```
oh team claim <ticket-id> [options]
```

| Flag | Short | Type | Description |
|------|-------|------|-------------|
| `--planned` | | bool | Create the claim in `planned` status (TODO column) instead of `in_progress` |
| `--project` | `-p` | string | Project (detected from the current folder if omitted) |
| `--worktree` | | string | Name of the associated branch or worktree |
| `--yes` | `-y` | bool | Confirms the takeover of an expired claim without asking (required without a terminal: exit code 2 otherwise) |

**Example:**

```bash
oh team claim TICKET-123
oh team claim TICKET-123 --yes   # take over an expired claim without a question
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

The command, the sync key of the team screens (`s`) and the automatic sync follow the same rules (team service):

- **Attribution by name.** Issues are attributed to the members by `tracker_username`, else `gitlab_username`. When issues seen by the sync (issues of your claims, `auto_plan_unassigned` pool) are assigned to names that belong to no member, and your profile has no `tracker_username` (or no name at all, on a tracker on the same server as the repository), the summary adds a warning that lists these names (five at most) and gives the command: `oh team profile --team <team> --tracker-username <name>`. With `auto_plan_assigned` alone, the sync only queries the known names and does not see the others: `oh doctor` ("Identity on the tracker" check) covers that case.

- **`[tracker] enabled = false`** (in the local `hub.toml` or in the team config) turns the sync off everywhere. The command prints a notice and exits 0; the screens refuse with a message; the automatic sync does nothing. Before v5.0 the screens synchronized anyway.
- A project without team uses the hub team (default team of `hub.toml`) for its tracker.
- **GitLab token search order** (first hit wins; `GITLAB_TOKEN` always first):

  | # | Source | Keychain key |
  |---|--------|--------------|
  | 1 | Environment variable | `GITLAB_TOKEN` |
  | 2 | `tracker_token_key` of the tracker config | the key you set |
  | 3 | Token of the project | `openhub.mcp.gitlab.token.<project>` |
  | 4 | Token of the team | `openhub.team.<team>.gitlab.token` |
  | 5 | Hub MCP token | key of `[mcp.gitlab] token`, then `openhub.mcp.gitlab.token` (and its former name `gitlab-token`) |
  | 6 | Former tracker key | `openhub.tracker.gitlab.token` |

  **Every source is tied to a server and only answers for it** (v5.1): the token of the team repository is never sent to a tracker that is on another GitLab. The server of the environment variable is `OH_GITLAB_URL` (otherwise `GITLAB_URL`): with neither, `GITLAB_TOKEN` is used nowhere. That of `tracker_token_key` and of the former tracker key is `tracker_token_url`, otherwise (implicit) the tracker; that of the MCP tokens is `[mcp.gitlab] url`, otherwise (implicit) `https://gitlab.com`; that of the team token is the `gitlab_url` of the team when declared (repository reached through an SSH alias), otherwise the host of `state_repo`. With no token for the server being called, the command stops naming the server, the sources read and the command that declares the key; see [the server of each GitLab token](config.en.md#the-server-of-each-gitlab-token). A refusal by the server (HTTP 401 / 403) names the source, the key and the server. `oh team config status` reads the same order (key and server of `hub.toml` included) and prints `token_key` and `token_url`.

  Jira keeps its own order (`JIRA_TOKEN`, then `tracker_token_key`).

  **What changes in v5.0** (breaking: the same order now applies to every command):

  | Command | Before | Now |
  |---------|--------|-----|
  | `oh team sync-tracker`, team screens, automatic sync | `GITLAB_TOKEN`, then the `tracker_token_key` only | the whole order above: a team or MCP token is now found without `tracker_token_key` |
  | `oh mr create` (`oh review --publish`), `oh review feedback` and the inputs of the `review-feedback` workflow | `GITLAB_TOKEN`, then the MCP key of `hub.toml`, then `openhub.mcp.gitlab.token` | the whole order above: **the team token (and the project token) now comes before the hub MCP token**, and the team of the project is found; the former tracker key is the last resort |
  | Identity check of `oh team init` / `rejoin` | `GITLAB_TOKEN`, team, `openhub.mcp.gitlab.token`, former tracker key | same order; the key of `[mcp.gitlab] token` and the `gitlab-token` name are now read too |
  | `oh mcp enable --project` (token present?) | `GITLAB_TOKEN`, project token, hub token | same order; the key of `[mcp.gitlab] token` is now read. Team and former tracker tokens do not count: the MCP server is not given them |

  If you keep a different token for the team than for the MCP server, check which one wins for the commands above.

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

`oh teams` manages the teams of this machine (list, add, remove, detach a project, archive, restore); `oh team` (singular) acts on the active team (status, board, activity, config, tracker sync). The two commands share no subcommand, so `oh teams` is not an alias of `oh team`.

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
| `--gitlab-url` | string | Real GitLab of the repository when its address is an SSH alias (optional) |

### oh teams remove

Remove a team from the hub; attached projects become projects without a team.

```
oh teams remove [team-id] [--yes]
```

| Flag | Short | Type | Description |
|------|-------|------|-------------|
| `--yes` | `-y` | bool | Remove without confirmation (required without a terminal). Former names: `--force`, `-f` (deprecated, same effect, with a warning) |

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

## Known limits

The team mode is a beta in v5.0. These limits are known; the complete fix is planned for v5.1.

- **A claim is released when its session stops**, even if a review (checkpoint `cp-2`) is still pending: the ticket becomes available to the other members again before the review is done. If the ticket is closed in Beads, the claim is completed instead of released.
- **Lease.** A claim being worked on has a lease (`lease_hours`, 24 h by default) that the daemon renews (`heartbeat_minutes`) while an open session works on the ticket. Once the lease is past, another member can take the claim over with `oh team claim <ticket>` (common confirmation, `--yes` without a terminal): a takeover brief is generated and the claim comes to them with a new lease. A claim without a lease (created before v5.1) is judged on `takeover.stale_days` days of inactivity (3 by default, `[takeover]` section of the team-state `config.toml`). A claim taken by hand, without a session, is not renewed by the daemon: `oh team claim <ticket>` renews its lease.
- **Offline, the end of a session waits for the network.** What the end of a session decides (release, complete, label `session-ended`) is written at once to a local queue (`.git/oh-claims-queue.json` of the team-state clone), then applied in one commit as soon as the team repository answers: at each beat of the daemon, by its `claims-queue` task (also when it starts), or at the end of a session. A session that starts offline on a ticket you already claimed takes the claim over when the network is back, ahead of what the previous session had queued (which is cancelled).
- **Forgotten claim:** `oh team release <ticket-id> -p <project>` frees it by hand.
- **`oh doctor` reports orphan claims** (ticket closed, or every session that worked on it ended) with the command to run. Claims taken under the former ID of a project created again are reported too, and `oh doctor --fix` attaches them to the local project (see [`oh doctor`](cli-infra.en.md#oh-doctor)).
- **The REVIEW column follows the `ready-for-review` label.** In `oh board` and in the Board TUI, an `in_progress` Beads ticket carrying this label is in REVIEW (Beads has no `review` status). The team board shows the `review` status of the claims.

oh warns once per process on stderr (team commands, first claim taken by a launch), and `oh doctor` lists the beta mode as a warning that does not change its exit code.
