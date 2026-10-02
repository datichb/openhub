> [Lire en francais](cli-team.fr.md)

# CLI Reference — Team

## Team Analytics

## Analytics

### oh team claim

Claim a ticket and create a claim entry in the team database.

```
oh team claim <ticket-id> [options]
```

| Flag | Type | Description |
|------|------|-------------|
| `--planned` | bool | Create the claim in `planned` status (TODO column) instead of starting immediately in `in_progress` |
| `--project` | string | Target project |
| `--worktree` | bool | Create a worktree for the claimed ticket |

Without `--planned`, the claim is created directly in `in_progress` status.

```bash
oh team claim TICKET-123
oh team claim TICKET-123 --planned
```

### oh team release

Release a claimed ticket.

```
oh team release <ticket-id> [options]
```

| Flag | Type | Description |
|------|------|-------------|
| `--project` | string | Target project |

```bash
oh team release TICKET-123
```

### oh team claim transfer

Transfer a claim to another team member.

```
oh team claim transfer <ticket-id> --to <member-id>
```

```bash
oh team claim transfer TICKET-123 --to bob
```

---

### oh team status

Display team state: active claims, members, in-progress tickets.

```bash
oh team status
```

---

### oh team activity

Display team activity history.

```bash
oh team activity
```

---

### oh team board

Display the team kanban board (claims by status).

| Flag | Type | Description |
|------|------|-------------|
| `--watch` | bool | Auto-refresh every 5s |

```bash
oh team board
oh team board --watch
```

---

### oh team sync-tracker

Synchronize claims with the external tracker (GitLab/Jira).

```bash
oh team sync-tracker
```

Pulls issue states from the tracker, updates claim statuses, mirrors labels, and auto-creates planned claims for assigned issues not yet claimed. Uses configuration from `team-state/config.toml` and hub MCP config.

No flags. Configuration is read from `team-state/config.toml` and the hub MCP config.

---


## Team Management

## Team Management

### oh teams list

List all configured teams.

```bash
oh teams list
```

### oh teams add

Add a new team.

```bash
oh teams add --repo git@github.com:org/team-state.git --member-id alice
```

| Flag | Type | Description |
|------|------|-------------|
| `--repo` | string | Team-state Git repository URL |
| `--member-id` | string | Your member ID in the team |
| `--id` | string | Team identifier |
| `--name` | string | Team display name |

### oh teams remove

Remove a team.

```bash
oh teams remove <team-id>
```

### oh teams detach

Detach a project from its team.

```bash
oh teams detach <project-name>
```

### oh teams archive / restore

Archive or restore a team:

```bash
oh teams archive <team-id>
oh teams restore <team-id>
```

---

### oh team init

Initialize team features with an interactive wizard.

```bash
oh team init
```

### oh team config

Manage team configuration.

```bash
oh team config
oh team config status
```


### oh team notify test

Test notification delivery.

```bash
oh team notify test
```

### oh team sync-tracker

Sync claim ExternalIID links back to the external tracker (Jira, Linear, GitLab Issues).

```bash
oh team sync-tracker
```

---


---

### oh team wiki list

List team wiki pages and pending proposals.

```bash
oh team wiki list
```

### oh team wiki read

Read a specific wiki page.

```bash
oh team wiki read <page-name>
```

### oh team wiki review

Review pending wiki proposals interactively. Without an ID, lists all pending proposals.

```bash
oh team wiki review
oh team wiki review <proposal-id>
```

---

## Governance

### oh conventions check

Validate the current project against team conventions.

```bash
oh conventions check
```

### oh patterns

Manage the team patterns library.

```bash
oh patterns list               # list all patterns
oh patterns show <name>        # display a pattern
oh patterns add                # propose a new pattern
oh patterns validate           # validate all patterns
oh patterns remove <name>      # remove a pattern
```

### oh policies

Manage and check team policies.

```bash
oh policies list               # list active policies
oh policies check              # validate project against policies
oh policies add                # add a new policy
```

---

## Takeover Briefs

### oh takeover-brief

Manage takeover briefs for ticket handoffs. Alias: `tb`.

```bash
oh takeover-brief show <ticket-id>    # view takeover context
oh takeover-brief list                # list available briefs
oh takeover-brief enrich <ticket-id>  # enrich with code analysis
```

---

