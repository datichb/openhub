> [Lire en francais](cli-team.fr.md)

# CLI Reference — Team

Team features rely on a shared **team-state** git repository (claims, events, wiki, policies, patterns, team workflows). See [Team workflows](../guides/team-workflows.en.md) for workflows and [ADR-040](../architecture/adr/040-workflows-team-state-governance.en.md).

## Collaboration (oh team)

### oh team init

Interactive wizard configuring the team, in 4 adaptive steps: global config (takeover, sessions), identity (team registration), Mattermost notifications (optional), team policies (automated conventions). The team-state repository must exist beforehand on GitLab or GitHub; when it is already configured, only the relevant steps are offered. The wizard creates the folders of the repository (`projects/`, `wiki/`, `catalog/`…) and commits them (« team: init structure »). If that commit fails (offline, remote refusal), the join goes on and the wizard says so (warning line, then in the repository step): the folders exist locally and are committed the next time `oh team init` runs.

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

**Questions are answered in a terminal.** `Enter` keeps the value proposed in brackets, which is the current value (a team on Jira stays on Jira); `-` clears a text value; an answer that is not understood is said and the question asked again. For the settings of this machine (`tracker.enabled`, `auto_sync`, `push_labels`), the **“inherit — team: …”** choice removes the key from `hub.toml`; as long as it is not chosen, nothing is written there. For the MCP recommendations of the team, “no recommendation” leaves `enabled` absent. A limit that is not an integer ≥ 0 is reported and the limit stays. Without a terminal (standard input redirected) the command refuses to ask the questions: change a single setting with `oh config set`. If the input closes (Ctrl+D) before the end, **nothing is written**. Answers are read on the input of the application and never guessed.

**Writes are safe.** The settings of `hub.toml` go through the same path as `oh config set` (lock, backup, typed values: a keychain key made of digits stays text). The team configuration is written **on top of the latest version** of `config.toml`: what a colleague changed meanwhile elsewhere in the file is kept; if the same setting was changed on both sides, nothing is overwritten, the command says so and can simply be run again. An unreadable `config.toml` stops the wizard with the cause (`oh doctor`) instead of starting from an empty configuration.

**`oh team config status` only reads**: it writes neither the team-state nor `hub.toml`. It no longer offers to delete or merge “orphan members” (colleagues are not in your local config, so all of them were taken for orphans).

```bash
oh team config
oh team config status
```

---

### oh team status

Shows who is working on what: active claims, members, tickets in progress. The summary (active, in review, blocked) counts by the **role of the columns of the board** of the team (`active`, `review`, `blocked`), not by the name of fixed statuses. The tickets of the reserve (nobody holds them) and those of a member who is no longer in the team are listed after the members.

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

Opens the TUI on the **team board** (the `team.board` view: current, planned and completed tickets, projects on `[` / `]`, filter on `/`, claim, release, transfer). `Esc` goes back to the team mode, then to the hub. The team is the one of the hub (`oh team init` first: the team-state must be cloned, as before). The board refreshes every 5 s and on `r`.

```
oh team board
```

| Flag | Type | Description |
|------|------|-------------|
| `--watch` | bool | Accepted and without effect (hidden): the board always refreshes on its own |

Without a terminal the command exits with code **2** and names `oh team status`. Differences with the former full-screen board: the board is the one of the TUI, with its custom columns (`[board]`), its filters and its actions.

---

### oh team claim

Claims a ticket in the team-state to tell the team you are working on it. When the ticket is already taken, a warning is shown (non-blocking). When the claim of another member has expired (its lease is past; without a lease, inactive for `takeover.stale_days` days), you can take it over: after the confirmation, a takeover brief is generated and the claim comes to you with a new lease. When the claim is yours, its lease is renewed (expired or not, without confirmation).

**Work-in-progress limit.** A member has at most `max_in_progress_per_member` tickets in progress (5 by default, `0` = no limit; see [`[tracker]`](config.en.md#tracker-section)); claims in review or in validation do not count. Taking one more ticket in progress is **refused** (exit code 1) with the count and the key: `Work-in-progress limit reached: alice already has 5 ticket(s) in progress (max_in_progress_per_member = 5)…`. The same limit applies to the takeover of an expired claim, to `oh team claim transfer` to yourself, to the take key and the status change of the team board, and to a session launched on tickets. `--planned` is not bound by it. Raise it with `oh config set tracker.max_in_progress_per_member <n>` (for you) or `oh team config` (for the team). `max_ticket_wip` of the team policies, which also checks this command, is a separate setting.

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

Changes the owner of an existing claim without releasing it. Handing a claim in progress to yourself is one more ticket in progress: it is refused at the limit (see [`oh team claim`](#oh-team-claim)); handing it to someone else is not.

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
- Only the projects attached to the team (`team_id`) are linked to its tracker; a project without a team, or of another team, is not synchronized with it. With no project of the team, the command says so (“no hub project is linked to the tracker”).
- **Automatic sync** (`auto_sync`): the daemon and the opening of the board synchronize every `sync_interval_minutes`, one member per interval (`sync.lock` lease); the command does not wait for the lease. See [`config.en.md`](config.en.md#tracker-section).
- **GitLab token search order** (first hit wins; `GITLAB_TOKEN` always first):

  | # | Source | Keychain key |
  |---|--------|--------------|
  | 1 | Environment variable | `GITLAB_TOKEN` |
  | 2 | `tracker_token_key` of the tracker config | the key you set |
  | 3 | Token of the project | `openhub.mcp.gitlab.token.<project>` |
  | 4 | Token of the team | `openhub.team.<team>.gitlab.token` |
  | 5 | Hub MCP token | key of `[mcp.gitlab] token`, then `openhub.mcp.gitlab.token` (and its former name `gitlab-token`) |
  | 6 | Former tracker key | `openhub.tracker.gitlab.token` |

  **Every source is tied to a server and only answers for it** (v5.1): the token of the team repository is never sent to a tracker that is on another GitLab. The server of the environment variable is `OH_GITLAB_URL` (otherwise `GITLAB_URL`): with neither, `GITLAB_TOKEN` is used nowhere. That of `tracker_token_key` and of the former tracker key is `tracker_token_url`, otherwise (implicit) the tracker; that of the MCP tokens is `[mcp.gitlab] url`, otherwise (implicit) `https://gitlab.com`; that of the team token is the `gitlab_url` of the team when declared (repository reached through an SSH alias), otherwise the host of `state_repo`. With no token for the server being called, the command stops naming the server, the sources read and the command that declares the key; see [the server of each GitLab token](config.en.md#the-server-of-each-gitlab-token). A refusal by the server names the source, the key and the server: a 401 is a refused token (replace it; it stops the sync), a 403 is a refusal of access to a resource (scope of the token, role in the project; it only concerns that tracker project). `oh team config status` reads the same order (key and server of `hub.toml` included) and prints `token_key` and `token_url`.

  Jira keeps its own order (`JIRA_TOKEN`, then `tracker_token_key`).

  **What changes in v5.0** (breaking: the same order now applies to every command):

  | Command | Before | Now |
  |---------|--------|-----|
  | `oh team sync-tracker`, team screens, automatic sync | `GITLAB_TOKEN`, then the `tracker_token_key` only | the whole order above: a team or MCP token is now found without `tracker_token_key` |
  | `oh mr create` and the inputs of the `review-feedback` workflow | `GITLAB_TOKEN`, then the MCP key of `hub.toml`, then `openhub.mcp.gitlab.token` | the whole order above: **the team token (and the project token) now comes before the hub MCP token**, and the team of the project is found; the former tracker key is the last resort |
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

Refusing a proposal (`r`) asks for a **reason** (required: an empty one cancels the refusal). The proposal is removed from the team-state; the reason is kept in the subject of the commit (`wiki: rejected proposal <id>: <reason>`) and in the data of the `wiki.rejected` event, which `oh team activity`, the Activity view and the notification show.

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
| `--yes` | `-y` | bool | Remove without confirmation (required without a terminal). Former names `--force` and `-f`: removed in 5.1 |

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
```

## Lease and end of session

Reference summary; the full contract (statuses, lease, end of session, queue) is in the [Team Setup](../guides/team-setup.en.md#lease-and-end-of-session) guide.

**Statuses.** The REVIEW column is, on the Beads side (`oh board`, TUI Board), an `in_progress` ticket carrying the `ready-for-review` label (Beads 1.3 has no `review` status); on the claim side (team board), the `review` status. The two are not copied into each other; at the end of a session, either one keeps the claim. Details: [Status model](../guides/team-setup.en.md#status-model).

**Lease.** A claim being worked on (an owner and work started; not `planned`, not in the pool, not finished) has a lease. Settings of the `[claim]` section of the team-state `config.toml` (and of `team-detail`):

| Key | Default | Role |
|---|---|---|
| `lease_hours` | 24 (1 to 168) | lease of a claim worked on by a session |
| `heartbeat_minutes` | 60 (0 = off, 5 to 1440, at most half the lease) | signal of life of the daemon: renews the lease of the open sessions when less than half of it is left |
| `sleeping_renewal_days` | 3 (1 to 90) | ceiling: a session asleep for longer no longer renews |
| `remote_renewal_days` | 7 (1 to 90) | ceiling: a remote session launched longer ago no longer renews |
| `lease_hours_manual` | 72 (`lease_hours` to 720) | lease of a claim taken by hand (no session), which nothing renews on its own |
| `release_expired_on_sync` | `false` | the tracker synchronization releases the expired claims (a claim in review is never released) |

**Expiry and takeover.** A claim in review (`review` status or `review` / `ready-for-review` label) **never expires**: it stays reserved until the merge or its release, and `oh doctor` (`claims_review`, a warning) only reports it when it has been there for more than `takeover.stale_days` days. Any other expired claim (lease past; without a lease, inactive for more than `takeover.stale_days` days) is reported by `oh doctor` (`claims_expired`, a warning). `oh team claim <ticket>` renews your own claim (expired or not, without confirmation) and takes over the claim of another member when it has expired (common confirmation; `--yes` without a terminal, otherwise exit code 2).

**End of a session.** For each claim of the session: finished → kept; ticket closed → terminal status; in review (label `review` / `ready-for-review` or `review` status) → **kept**; a checkpoint waiting, `cp-2` put off with work left, state of the checkpoints unreadable → kept with the `session-ended` label; otherwise released. A kept claim is no longer renewed and expires when its lease ends, except a claim in review, which does not expire.

**Durable queue.** The end-of-session decisions and the lease renewals that were not pushed are written to `.git/oh-claims-queue.json` of the team-state clone, then applied in one commit as soon as the repository answers (end of the session, beat of the daemon, `claims-queue` task every 15 minutes). `oh doctor` (`claims_queue`) reports what has been waiting for more than 15 minutes; `oh doctor --fix` applies it after a confirmation.

**Forgotten claim:** `oh team release <ticket-id> -p <project>` frees it by hand. `oh doctor` also reports orphan claims (ticket closed, or every session that worked on it ended); the ones taken under the former ID of a recreated project are attached to the local project by `oh doctor --fix` (see [`oh doctor`](cli-infra.en.md#oh-doctor)).
